package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
	"github.com/rivo/uniseg"
	"github.com/urfave/cli/v2"
)

// diffCommand prints the checkout's current change by default, read like a
// capture but never stored. --stored and explicit pairs print stored patches.
func diffCommand(state *invocation, ctx *cli.Context) error {
	if ctx.NArg() != 0 && ctx.NArg() != 2 {
		return invalidWithFix("diff takes no IDs or a BASE CANDIDATE snapshot pair", "try: after diff, after diff --stored, or after diff BASE CANDIDATE")
	}
	if ctx.Bool("json") {
		return invalidWithFix("diff writes a patch, not JSON", "omit --json; use --raw to write exact patch bytes to a file or pipe")
	}
	if ctx.Bool("raw") && ctx.Bool("stat") {
		return invalidWithFix("--raw and --stat cannot be combined", "choose one: after diff --raw or after diff --stat")
	}
	if ctx.Bool("raw") && state.stdoutTTY {
		return invalidWithFix("--raw requires non-terminal stdout", "redirect the patch to a file or pipe; terminal output is refused")
	}
	live := ctx.NArg() == 0 && !ctx.Bool("stored")
	if !live {
		for _, name := range []string{"staged", "base", "target", "include-untracked"} {
			if ctx.IsSet(name) {
				return invalidWithFix("--"+name+" reads the checkout, so it cannot be combined with --stored or snapshot IDs", "drop --"+name+" to print stored snapshots, or drop --stored and IDs to read the checkout")
			}
		}
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	if live {
		return liveDiff(state, ctx, cfg.Project)
	}
	s, err := store.Open(cfg.Project, false, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return missingStoredRecord("capture", "run after diff to print the current change, or after capture to store one")
		}
		return operational("cannot open private evidence store for reading")
	}
	defer s.Close()

	var baseID, candidateID evidence.Digest
	if ctx.NArg() == 0 {
		capture, err := newestCaptureForStore(s)
		if err != nil {
			return err
		}
		baseID, candidateID = capture.Base, capture.Candidate
	} else {
		baseID, err = resolveID(s, ctx.Args().Get(0), "snapshot")
		if err != nil {
			return err
		}
		candidateID, err = resolveID(s, ctx.Args().Get(1), "snapshot")
		if err != nil {
			return err
		}
	}
	base, err := store.Get[evidence.Snapshot](s, baseID)
	if err != nil {
		return operational("base snapshot is corrupt or unavailable")
	}
	candidate, err := store.Get[evidence.Snapshot](s, candidateID)
	if err != nil {
		return operational("candidate snapshot is corrupt or unavailable")
	}
	view, err := rawdiff.Open(s, base, candidate)
	if err != nil {
		return operational("snapshot inventory is corrupt or unavailable")
	}
	entries := view.Inventory()
	limits := view.Limits()
	origin := "captured patch"
	if _, err := view.Raw(0, rawdiff.MaxPageBytes); errors.Is(err, rawdiff.ErrDiff) {
		computed, err := view.Compute(state.ctx)
		if err != nil {
			return operational("cannot compute a bounded diff from captured sources")
		}
		entries, limits, origin = computed.Inventory, append(limits, computed.Limits...), rawdiff.ComputedOrigin
		if err := writeDiffReport(state, fmt.Sprintf("stored pair %s → %s", shortID(baseID), shortID(candidateID)), origin, entries, limits); err != nil {
			return operational("cannot write diff diagnostics")
		}
		return writePatch(state, ctx, bytes.NewReader(computed.Raw), computed.Files, entries)
	} else if err != nil {
		return operational("captured patch is unavailable")
	}
	if err := writeDiffReport(state, fmt.Sprintf("stored pair %s → %s", shortID(baseID), shortID(candidateID)), origin, entries, limits); err != nil {
		return operational("cannot write diff diagnostics")
	}
	return writePatch(state, ctx, &rawDiffReader{view: view, more: true}, view.Files(), entries)
}

// liveDiff reads the checkout with capture's consistency and safety rules and
// prints its patch. It never stores anything and never falls back to a stored
// patch: a failed read prints no patch.
func liveDiff(state *invocation, ctx *cli.Context, project string) error {
	options, err := captureOptionsFromFlags(ctx, "diff")
	if err != nil {
		return err
	}
	stopNotice := startElapsedNotice(state, "diff", time.Second)
	current, err := capture.ReadLive(state.ctx, project, options)
	stopNotice()
	if err != nil {
		return liveDiffFailure(err)
	}
	view := rawdiff.Unstored(current.Base, current.Candidate, current.Patch)
	entries := view.Inventory()
	selection := liveSelection(current.Base, current.Candidate)
	if err := writeDiffReport(state, selection, liveOrigin, entries, nil); err != nil {
		return operational("cannot write diff diagnostics")
	}
	return writePatch(state, ctx, bytes.NewReader(current.Patch), view.Files(), entries)
}

const liveOrigin = "current checkout; not stored"

// liveSelection names what a live diff compared, like git diff HEAD.
func liveSelection(base, candidate evidence.Snapshot) string {
	baseName := "empty repository (no commits)"
	if !base.Unborn {
		baseName = "commit " + shortCommit(base.Commit)
	}
	switch candidate.Source {
	case evidence.Index:
		return baseName + " → staged changes"
	case evidence.MergeBase:
		return fmt.Sprintf("merge base %s → commit %s", shortCommit(candidate.MergeBase), shortCommit(candidate.Commit))
	}
	return baseName + " → working tree"
}

// writePatch writes the selected patch as statistics, exact bytes or a
// sanitized terminal patch. Exact and sanitized output stream the patch.
func writePatch(state *invocation, ctx *cli.Context, patch io.Reader, files []rawdiff.PatchFile, entries []rawdiff.Entry) error {
	switch {
	case ctx.Bool("stat"):
		raw, err := io.ReadAll(patch)
		if err != nil {
			return operational("captured patch is unavailable")
		}
		if err := writeFull(state.stdout, []byte(diffStat(raw, files, entries))); err != nil {
			return operational("cannot write diff statistics")
		}
	case ctx.Bool("raw"):
		if _, err := io.Copy(state.stdout, patch); err != nil {
			return operational("cannot write exact patch bytes")
		}
	default:
		changed, err := terminal.WriteDiff(state.stdout, patch, state.theme, state.stdoutTTY)
		if err != nil {
			return operational("cannot write sanitized patch")
		}
		if changed {
			if _, err := fmt.Fprintln(state.stderr, "after diff: unsafe patch bytes were sanitized; use --raw to write exact bytes to a file or pipe"); err != nil {
				return operational("cannot write diff diagnostics")
			}
		}
	}
	return nil
}

// diffStat summarizes each patch section like git diff --stat: changed hunk
// lines per file, a bar scaled to at most 40 columns, and a total line.
func diffStat(raw []byte, files []rawdiff.PatchFile, inventory []rawdiff.Entry) string {
	binary := map[string]bool{}
	for _, entry := range inventory {
		binary[entry.Path] = entry.Binary
	}
	type row struct {
		path           string
		added, deleted int
		binary         bool
	}
	rows := make([]row, 0, len(files))
	pathWidth, countWidth, largest, added, deleted := 0, 1, 0, 0, 0
	for _, file := range files {
		item := row{path: terminal.Sanitize(file.Path), binary: binary[file.Path]}
		if item.path == "" {
			item.path = "(unmatched patch section)"
		}
		inHunk := false
		for _, line := range bytes.Split(raw[file.Start:min(file.End, len(raw))], []byte{'\n'}) {
			switch {
			case bytes.HasPrefix(line, []byte("@@")):
				inHunk = true
			case !inHunk || len(line) == 0:
			case line[0] == '+':
				item.added++
			case line[0] == '-':
				item.deleted++
			case line[0] != ' ' && line[0] != '\\':
				inHunk = false
			}
		}
		added, deleted = added+item.added, deleted+item.deleted
		largest = max(largest, item.added+item.deleted)
		pathWidth = max(pathWidth, uniseg.StringWidth(item.path))
		countWidth = max(countWidth, len(fmt.Sprint(item.added+item.deleted)))
		rows = append(rows, item)
	}
	var out strings.Builder
	for _, item := range rows {
		padding := strings.Repeat(" ", pathWidth-uniseg.StringWidth(item.path))
		if item.binary {
			fmt.Fprintf(&out, " %s%s | %*s\n", item.path, padding, countWidth, "Bin")
			continue
		}
		plus, minus := item.added, item.deleted
		if largest > 40 {
			plus, minus = scaledBar(item.added, largest), scaledBar(item.deleted, largest)
		}
		fmt.Fprintf(&out, " %s%s | %*d %s%s\n", item.path, padding, countWidth, item.added+item.deleted, strings.Repeat("+", plus), strings.Repeat("-", minus))
	}
	fmt.Fprintf(&out, " %d %s changed", len(rows), plural(len(rows), "file"))
	if added > 0 || deleted == 0 {
		fmt.Fprintf(&out, ", %d %s(+)", added, plural(added, "insertion"))
	}
	if deleted > 0 || added == 0 {
		fmt.Fprintf(&out, ", %d %s(-)", deleted, plural(deleted, "deletion"))
	}
	out.WriteByte('\n')
	return out.String()
}

// scaledBar keeps any nonzero count visible after scaling to 40 columns.
func scaledBar(count, largest int) int {
	if count == 0 {
		return 0
	}
	return max(1, count*40/largest)
}

// writeDiffReport names the selection and origin on stderr, followed by any
// uncovered inventory and limits. Empty sections are omitted.
func writeDiffReport(state *invocation, selection, origin string, inventory []rawdiff.Entry, limits []string) error {
	var report strings.Builder
	fmt.Fprintf(&report, "after diff: %s\n  origin: %s\n", selection, origin)
	untracked := false
	for _, item := range inventory {
		if item.Change != "unknown" && len(item.Limits) == 0 {
			continue
		}
		if !strings.Contains(report.String(), "  uncovered inventory:\n") {
			report.WriteString("  uncovered inventory:\n")
		}
		reason := strings.Join(uniqueDiffStrings(item.Limits), "; ")
		if reason == "" {
			reason = "unknown; original change is not fully captured"
		}
		untracked = untracked || excludedUntracked(item)
		fmt.Fprintf(&report, "    %s — %s\n", terminal.Sanitize(item.Path), terminal.Sanitize(reason))
	}
	if len(inventory) == 0 {
		report.WriteString("  no changes\n")
	}
	if untracked && origin == liveOrigin {
		report.WriteString("    include an untracked file with --include-untracked PATH\n")
	}
	if limits = uniqueDiffStrings(limits); len(limits) > 0 {
		report.WriteString("  limits:\n")
		for _, limit := range limits {
			fmt.Fprintf(&report, "    %s\n", terminal.Sanitize(limit))
		}
	}
	_, err := io.WriteString(state.stderr, report.String())
	return err
}

func uniqueDiffStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// rawDiffReader streams a stored patch through its bounded pages.
type rawDiffReader struct {
	view   *rawdiff.View
	page   []byte
	offset int
	next   int
	more   bool
}

func (r *rawDiffReader) Read(out []byte) (int, error) {
	if len(out) == 0 {
		return 0, nil
	}
	for len(r.page) == 0 {
		if !r.more {
			return 0, io.EOF
		}
		page, err := r.view.Raw(r.next, rawdiff.MaxPageBytes)
		if err != nil {
			return 0, err
		}
		r.page, r.next, r.more = page.Bytes, page.Next, page.More
		r.offset = 0
	}
	n := copy(out, r.page[r.offset:])
	r.offset += n
	if r.offset == len(r.page) {
		r.page = nil
		r.offset = 0
	}
	return n, nil
}

func writeFull(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

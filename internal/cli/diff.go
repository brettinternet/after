package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
	"github.com/urfave/cli/v2"
)

func diffCommand(state *invocation, ctx *cli.Context) error {
	if ctx.NArg() != 0 && ctx.NArg() != 2 {
		return invalidWithFix("diff takes no IDs or a BASE CANDIDATE snapshot pair", "try: after diff or after diff BASE CANDIDATE")
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
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	s, err := store.Open(cfg.Project, false, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return missingStoredRecord("capture", "run after capture to create one")
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
	patch, patchErr := view.Raw(0, rawdiff.MaxPageBytes)
	var computed *rawdiff.ComputedDiff
	if errors.Is(patchErr, rawdiff.ErrDiff) {
		computed, err = view.Compute(state.ctx)
		if err != nil {
			return operational("cannot compute a bounded diff from captured sources")
		}
		origin = rawdiff.ComputedOrigin
		entries = computed.Inventory
		limits = append(limits, computed.Limits...)
	} else if patchErr != nil {
		return operational("captured patch is unavailable")
	}
	limits = uniqueDiffStrings(limits)
	if err := writeDiffReport(state, baseID, candidateID, origin, entries, limits); err != nil {
		return operational("cannot write diff diagnostics")
	}
	if ctx.Bool("stat") {
		return writeResult(state, "capture", struct {
			Base      snapshotSummary `json:"base_snapshot"`
			Candidate snapshotSummary `json:"candidate_snapshot"`
		}{summarizeSnapshot(base), summarizeSnapshot(candidate)})
	}
	if ctx.Bool("raw") {
		if computed != nil {
			if err := writeFull(state.stdout, computed.Raw); err != nil {
				return operational("cannot write exact patch bytes")
			}
			return nil
		}
		if err := writeRawView(state.stdout, view, patch); err != nil {
			return operational("cannot write exact patch bytes")
		}
		return nil
	}
	var reader io.Reader
	if computed != nil {
		reader = bytes.NewReader(computed.Raw)
	} else {
		reader = &rawDiffReader{view: view, page: patch.Bytes, next: patch.Next, more: patch.More}
	}
	changed, err := terminal.WriteDiff(state.stdout, reader, state.theme, state.stdoutTTY)
	if err != nil {
		return operational("cannot write sanitized patch")
	}
	if changed {
		if _, err := fmt.Fprintln(state.stderr, "after diff: unsafe patch bytes were sanitized; use --raw to write exact bytes to a file or pipe"); err != nil {
			return operational("cannot write diff diagnostics")
		}
	}
	return nil
}

func summarizeSnapshot(snapshot evidence.Snapshot) snapshotSummary {
	return snapshotSummary{snapshot.ID, snapshot.Source, snapshot.Completeness, len(snapshot.Files), len(snapshot.Excluded), len(snapshot.Unsupported), snapshot.Diff, append([]string(nil), snapshot.Limits...)}
}

func writeDiffReport(state *invocation, base, candidate evidence.Digest, origin string, inventory []rawdiff.Entry, limits []string) error {
	if _, err := fmt.Fprintf(state.stderr, "after diff: pair %s → %s\n  origin: %s\n  uncovered inventory:\n", base, candidate, origin); err != nil {
		return err
	}
	uncovered := 0
	for _, item := range inventory {
		if item.Change != "unknown" && len(item.Limits) == 0 {
			continue
		}
		reason := strings.Join(uniqueDiffStrings(item.Limits), "; ")
		if reason == "" {
			reason = "unknown; original change is not fully captured"
		}
		if _, err := fmt.Fprintf(state.stderr, "    %s — %s\n", terminal.Sanitize(item.Path), terminal.Sanitize(reason)); err != nil {
			return err
		}
		uncovered++
	}
	if uncovered == 0 {
		if _, err := io.WriteString(state.stderr, "    none\n"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(state.stderr, "  limits:\n"); err != nil {
		return err
	}
	if len(limits) == 0 {
		_, err := io.WriteString(state.stderr, "    none\n")
		return err
	}
	for _, limit := range limits {
		if _, err := fmt.Fprintf(state.stderr, "    %s\n", terminal.Sanitize(limit)); err != nil {
			return err
		}
	}
	return nil
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

func writeRawView(writer io.Writer, view *rawdiff.View, first rawdiff.Page) error {
	page := first
	for {
		if err := writeFull(writer, page.Bytes); err != nil {
			return err
		}
		if !page.More {
			return nil
		}
		var err error
		page, err = view.Raw(page.Next, rawdiff.MaxPageBytes)
		if err != nil {
			return err
		}
	}
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

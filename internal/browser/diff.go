package browser

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/terminal"
)

type DiffRow struct {
	RawLine   int
	File      int
	Summary   string
	OldLine   int
	NewLine   int
	HasOld    bool
	HasNew    bool
	HasGutter bool
	Style     terminal.Style
	// Pair links a removed line to the added line that replaced it, so the
	// renderer can emphasize the changed span. Only set when Paired.
	Pair   int
	Paired bool
}

type DiffFile struct {
	Path            string
	Change          string
	Binary          bool
	PotentialOracle bool
	BaseMode        string
	CandidateMode   string
	Added           int
	Deleted         int
	Start, End      int
	StartRow        int
	Summary         string
	// Kinds are the trusted header words: added, deleted, binary, mode-only,
	// computed-diff limits, or the plain change kind.
	Kinds []string
}

type DiffView struct {
	Raw           []byte
	Files         []DiffFile
	VisibleFiles  int
	FileByPath    map[string]int
	Rows          []DiffRow
	HunkRows      []int
	Origin        string
	Added         int
	Deleted       int
	SourceLimited bool
	HunksComplete bool
	Limited       bool
}

type InventoryRow struct {
	EntryIndex int
	Heading    string
	Count      int
}

func readRawDiff(v *rawdiff.View) ([]byte, error) {
	var out []byte
	for offset := 0; ; {
		page, err := v.Raw(offset, rawdiff.MaxPageBytes)
		if err != nil {
			return nil, err
		}
		if out == nil {
			out = make([]byte, page.Total)
		}
		copy(out[page.Offset:page.Next], page.Bytes)
		if !page.More {
			return out, nil
		}
		offset = page.Next
	}
}

func buildDiff(raw []byte, patchFiles []rawdiff.PatchFile, hunks []rawdiff.Hunk, inventory []rawdiff.Entry) *DiffView {
	hunkOffsets := make([]int, len(hunks))
	for i, hunk := range hunks {
		hunkOffsets[i] = hunk.Start
	}
	return buildDiffAt(raw, patchFiles, hunkOffsets, inventory, "captured patch")
}

func buildComputedDiff(raw []byte, patchFiles []rawdiff.PatchFile, hunkOffsets []int, inventory []rawdiff.Entry) *DiffView {
	return buildDiffAt(raw, patchFiles, hunkOffsets, inventory, rawdiff.ComputedOrigin)
}

func buildDiffAt(raw []byte, patchFiles []rawdiff.PatchFile, hunkOffsets []int, inventory []rawdiff.Entry, origin string) *DiffView {
	byPath := make(map[string]rawdiff.Entry, len(inventory))
	for _, item := range inventory {
		byPath[item.Path] = item
	}
	view := &DiffView{Raw: raw, Origin: origin, Files: make([]DiffFile, len(patchFiles)), FileByPath: make(map[string]int, len(patchFiles)), HunkRows: make([]int, 0, len(hunkOffsets))}
	view.Rows = make([]DiffRow, 0, min(bytes.Count(raw, []byte("\n"))+1+len(patchFiles), terminal.MaxLines))
	for i, indexed := range patchFiles {
		file := DiffFile{Path: indexed.Path, Change: "unknown", Start: indexed.Start, End: indexed.End, StartRow: -1}
		if entry, ok := byPath[indexed.Path]; ok {
			file.Change = entry.Change
			file.Binary = entry.Binary
			file.PotentialOracle = entry.PotentialOracle
			if entry.Base != nil {
				file.BaseMode = entry.Base.Mode
			}
			if entry.Candidate != nil {
				file.CandidateMode = entry.Candidate.Mode
			}
			modeOnly := entry.Change == "modified" && entry.Base != nil && entry.Candidate != nil && entry.Base.Content == entry.Candidate.Content && entry.Base.Mode != entry.Candidate.Mode
			var summaries []string
			if file.Binary {
				summaries = append(summaries, "binary")
			}
			if modeOnly {
				summaries = append(summaries, "mode-only")
			}
			if entry.Change == "added" || entry.Change == "deleted" {
				summaries = append(summaries, entry.Change)
			}
			if origin == rawdiff.ComputedOrigin {
				switch {
				case entry.Change == "unknown":
					summaries = append(summaries, "unknown; no computed diff")
				case slices.Contains(entry.Limits, "too large to diff here — open both sources"):
					summaries = append(summaries, "too large to diff here — open both sources")
				case slices.Contains(entry.Limits, "captured source unavailable; no computed diff"):
					summaries = append(summaries, "captured source unavailable; no computed diff")
				}
			}
			file.Kinds = summaries
		}
		if len(file.Kinds) == 0 && file.Change != "unknown" {
			file.Kinds = []string{file.Change}
		}
		file.Summary = "── " + strings.Join(append([]string{file.Path}, file.Kinds...), " · ") + " ──"
		view.Files[i] = file
		if file.Path != "" {
			view.FileByPath[file.Path] = i
		}
	}

	hunkAt := make(map[int]struct{}, len(hunkOffsets))
	for _, offset := range hunkOffsets {
		hunkAt[offset] = struct{}{}
	}
	fileAt := make(map[int]int, len(patchFiles))
	for i, file := range patchFiles {
		fileAt[file.Start] = i
	}
	currentFile, oldLine, newLine, inHunk := -1, 0, 0, false
	// removed holds the row indexes of the current run of '-' lines; added
	// counts the '+' lines that followed it. Pairs are positional.
	var removed []int
	added, lastKind := 0, byte(0)
	resetRun := func() { removed, added, lastKind = removed[:0], 0, 0 }
	for rawLine, start := 0, 0; start < len(raw); rawLine++ {
		end := bytes.IndexByte(raw[start:], '\n')
		if end < 0 {
			end = len(raw)
		} else {
			end += start
		}
		line := raw[start:end]
		if fileIndex, ok := fileAt[start]; ok {
			currentFile, inHunk = fileIndex, false
			resetRun()
			if len(view.Rows) < terminal.MaxLines {
				view.Files[fileIndex].StartRow = len(view.Rows)
				view.Rows = append(view.Rows, DiffRow{File: fileIndex, Summary: view.Files[fileIndex].Summary, Style: terminal.Muted})
				view.VisibleFiles++
			} else {
				view.Limited = true
			}
		}
		row := DiffRow{RawLine: rawLine, File: currentFile, Style: patchLineStyle(line)}
		if _, ok := hunkAt[start]; ok {
			resetRun()
			if currentFile >= 0 {
				row.File = currentFile
			}
			if len(view.Rows) < terminal.MaxLines {
				view.HunkRows = append(view.HunkRows, len(view.Rows))
			} else {
				view.Limited = true
			}
			oldLine, newLine, inHunk = 0, 0, false
			if oldStart, newStart, valid := parseHunk(line); valid {
				oldLine, newLine, inHunk = oldStart, newStart, true
			}
		} else if inHunk {
			switch {
			case len(line) > 0 && line[0] == ' ':
				resetRun()
				row.OldLine, row.NewLine, row.HasOld, row.HasNew, row.HasGutter, row.Style = oldLine, newLine, true, true, true, terminal.Plain
				oldLine++
				newLine++
			case len(line) > 0 && line[0] == '-':
				if lastKind == '+' {
					resetRun()
				}
				removed, added, lastKind = append(removed, len(view.Rows)), 0, '-'
				row.OldLine, row.HasOld, row.HasGutter, row.Style = oldLine, true, true, terminal.Removed
				oldLine++
				if currentFile >= 0 {
					view.Files[currentFile].Deleted++
				}
			case len(line) > 0 && line[0] == '+':
				if added < len(removed) && removed[added] < len(view.Rows) {
					row.Pair, row.Paired = removed[added], true
					view.Rows[removed[added]].Pair, view.Rows[removed[added]].Paired = len(view.Rows), true
				}
				added, lastKind = added+1, '+'
				row.NewLine, row.HasNew, row.HasGutter, row.Style = newLine, true, true, terminal.Added
				newLine++
				if currentFile >= 0 {
					view.Files[currentFile].Added++
				}
			case len(line) > 0 && line[0] == '\\':
			default:
				resetRun()
				inHunk = false
			}
		}
		if len(view.Rows) < terminal.MaxLines {
			view.Rows = append(view.Rows, row)
		} else {
			view.Limited = true
		}
		if end == len(raw) {
			break
		}
		start = end + 1
	}
	for _, file := range view.Files {
		view.Added += file.Added
		view.Deleted += file.Deleted
	}
	return view
}

func parseHunk(line []byte) (int, int, bool) {
	if !bytes.HasPrefix(line, []byte("@@ -")) {
		return 0, 0, false
	}
	space := bytes.IndexByte(line[4:], ' ')
	if space < 0 {
		return 0, 0, false
	}
	oldRange := line[4 : 4+space]
	rest := line[4+space+1:]
	if len(rest) == 0 || rest[0] != '+' {
		return 0, 0, false
	}
	end := bytes.IndexByte(rest, ' ')
	if end < 0 {
		return 0, 0, false
	}
	oldStart, _, oldOK := parseRange(oldRange)
	newStart, _, newOK := parseRange(rest[1:end])
	return oldStart, newStart, oldOK && newOK
}

func parseRange(raw []byte) (int, int, bool) {
	start := bytes.IndexByte(raw, ',')
	if start < 0 {
		value, err := strconv.Atoi(string(raw))
		return value, 1, err == nil
	}
	first, errFirst := strconv.Atoi(string(raw[:start]))
	count, errCount := strconv.Atoi(string(raw[start+1:]))
	return first, count, errFirst == nil && errCount == nil
}

func inventoryRows(entries []Entry) ([]InventoryRow, []int) {
	groups := []struct {
		heading string
		match   func(Entry) bool
	}{
		{"POTENTIAL ORACLES", func(e Entry) bool { return e.PotentialOracle }},
		{"CHANGED", func(e Entry) bool { return !e.PotentialOracle && e.Change != "unknown" }},
		{"UNKNOWN — not fully captured", func(e Entry) bool { return !e.PotentialOracle && e.Change == "unknown" }},
	}
	rows := make([]InventoryRow, 0, len(entries)+len(groups))
	positions := make([]int, len(entries))
	for _, group := range groups {
		count := 0
		for _, entry := range entries {
			if group.match(entry) {
				count++
			}
		}
		if count == 0 {
			continue
		}
		rows = append(rows, InventoryRow{EntryIndex: -1, Heading: group.heading, Count: count})
		for index, entry := range entries {
			if group.match(entry) {
				positions[index] = len(rows)
				rows = append(rows, InventoryRow{EntryIndex: index})
			}
		}
	}
	return rows, positions
}

func pathSummary(item rawdiff.Entry, added, deleted int) string {
	parts := []string{}
	if item.PotentialOracle {
		parts = append(parts, "oracle")
	}
	if item.Binary {
		parts = append(parts, "binary")
	}
	baseMode, candidateMode := "", ""
	if item.Base != nil {
		baseMode = item.Base.Mode
	}
	if item.Candidate != nil {
		candidateMode = item.Candidate.Mode
	}
	if baseMode != "" && candidateMode != "" && baseMode != candidateMode {
		parts = append(parts, "mode "+baseMode+" → "+candidateMode)
	}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("+%d", added))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("−%d", deleted))
	}
	parts = append(parts, item.Limits...)
	return strings.Join(parts, " · ")
}

type inventoryRecord struct {
	Path            string   `json:"path"`
	Change          string   `json:"change"`
	PotentialOracle bool     `json:"potential_oracle"`
	Binary          bool     `json:"binary"`
	BaseMode        string   `json:"base_mode,omitempty"`
	CandidateMode   string   `json:"candidate_mode,omitempty"`
	DiffOrigin      string   `json:"diff_origin"`
	Added           int      `json:"added_lines_in_displayed_diff"`
	Deleted         int      `json:"deleted_lines_in_displayed_diff"`
	Limits          []string `json:"recorded_limitations,omitempty"`
}

func diffPosition(rows []DiffRow, row int) int {
	if len(rows) == 0 {
		return -1
	}
	row = min(max(row, 0), len(rows)-1)
	return rows[row].File
}

// patchMetadata prefixes are Git's per-file header lines. They stay visible
// but recede so the changed lines carry the eye.
var patchMetadata = [][]byte{
	[]byte("diff --git "), []byte("index "), []byte("--- "), []byte("+++ "), []byte("old mode "), []byte("new mode "),
	[]byte("new file mode "), []byte("deleted file mode "), []byte("similarity index "), []byte("rename from "),
	[]byte("rename to "), []byte("Binary files "), []byte("GIT binary patch"), []byte("literal "), []byte("delta "), []byte(`\ `),
}

// patchLineStyle classifies one raw patch line for the browser's Diff view.
func patchLineStyle(line []byte) terminal.Style {
	for _, prefix := range patchMetadata {
		if bytes.HasPrefix(line, prefix) {
			return terminal.Muted
		}
	}
	switch {
	case bytes.HasPrefix(line, []byte("@@")):
		return terminal.Hunk
	case len(line) > 0 && line[0] == '+':
		return terminal.Added
	case len(line) > 0 && line[0] == '-':
		return terminal.Removed
	default:
		return terminal.Plain
	}
}

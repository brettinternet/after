package rawdiff

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

type sourceFixture struct {
	path             string
	base, candidate  []byte
	baseMode         string
	candidateMode    string
	basePresent      bool
	candidatePresent bool
}

func computedView(t *testing.T, fixtures []sourceFixture, excluded map[string]string) (*View, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s := openStore(t, dir, nil)
	baseDiff, err := s.PutArtifact([]byte("base diff identity"), "base-diff", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	candidateDiff, err := s.PutArtifact([]byte("candidate diff identity"), "candidate-diff", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	base := evidence.Snapshot{SchemaVersion: 1, Source: evidence.Commit, Unborn: true, Completeness: evidence.Complete, Diff: baseDiff.Content}
	candidate := evidence.Snapshot{SchemaVersion: 1, Source: evidence.WorkingTree, Unborn: true, Completeness: evidence.Complete, Diff: candidateDiff.Content}
	for _, fixture := range fixtures {
		baseMode, candidateMode := fixture.baseMode, fixture.candidateMode
		if baseMode == "" {
			baseMode = "100644"
		}
		if candidateMode == "" {
			candidateMode = "100644"
		}
		if fixture.basePresent {
			blob, err := s.PutArtifact(fixture.base, "source", store.MaxBlobBytes)
			if err != nil {
				t.Fatal(err)
			}
			base.Files = append(base.Files, evidence.File{Path: fixture.path, Mode: baseMode, Content: blob.Content})
		}
		if fixture.candidatePresent {
			blob, err := s.PutArtifact(fixture.candidate, "source", store.MaxBlobBytes)
			if err != nil {
				t.Fatal(err)
			}
			candidate.Files = append(candidate.Files, evidence.File{Path: fixture.path, Mode: candidateMode, Content: blob.Content})
		}
	}
	for path, reason := range excluded {
		candidate.Excluded = append(candidate.Excluded, evidence.Limitation{Path: path, Reason: reason})
	}
	base, err = store.Put(s, base)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err = store.Put(s, candidate)
	if err != nil {
		t.Fatal(err)
	}
	v, err := Open(s, base, candidate)
	if err != nil {
		t.Fatal(err)
	}
	return v, s
}

func TestComputedDiffPreservesInventoryOrderAndLimitations(t *testing.T) {
	largeBase := bytes.Repeat([]byte("a"), MaxComputedFileBytes/2+1)
	largeCandidate := bytes.Repeat([]byte("b"), MaxComputedFileBytes/2+1)
	binaryBase := bytes.Repeat([]byte("a"), 8000)
	binaryCandidate := bytes.Repeat([]byte("b"), 8000)
	binaryCandidate[7999] = 0
	longPath := strings.Repeat("x", MaxComputedPathBytes+1)
	fixtures := []sourceFixture{
		{path: "z-last.go", base: []byte("old\n"), candidate: []byte("new\n"), basePresent: true, candidatePresent: true},
		{path: longPath, base: []byte("old\n"), candidate: []byte("new\n"), basePresent: true, candidatePresent: true},
		{path: "mode.sh", base: []byte("same\n"), candidate: []byte("same\n"), baseMode: "100644", candidateMode: "100755", basePresent: true, candidatePresent: true},
		{path: "binary.dat", base: binaryBase, candidate: binaryCandidate, basePresent: true, candidatePresent: true},
		{path: "large.txt", base: largeBase, candidate: largeCandidate, basePresent: true, candidatePresent: true},
	}
	v, _ := computedView(t, fixtures, map[string]string{"unknown.env": "excluded: untracked; not selected"})
	result, err := v.Compute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"binary.dat", "large.txt", "mode.sh", "unknown.env", "z-last.go"}
	var gotOrder []string
	for _, file := range result.Files {
		gotOrder = append(gotOrder, file.Path)
	}
	if strings.Join(gotOrder, "|") != strings.Join(wantOrder, "|") {
		t.Fatalf("computed file order %v want %v", gotOrder, wantOrder)
	}
	for i := 1; i < len(result.Files); i++ {
		if result.Files[i-1].Start >= result.Files[i].Start {
			t.Fatal("computed sections are not in inventory order")
		}
	}
	byPath := map[string]Entry{}
	for _, entry := range result.Inventory {
		byPath[entry.Path] = entry
	}
	if !byPath["binary.dat"].Binary || !bytes.Contains(result.Raw, []byte("Binary files a/binary.dat and b/binary.dat differ")) {
		t.Fatal("NUL in first 8000 bytes was not classified as binary")
	}
	if !strings.Contains(strings.Join(byPath["large.txt"].Limits, " "), "too large to diff here — open both sources") {
		t.Fatal("oversized file limitation was lost", byPath["large.txt"])
	}
	if !strings.Contains(strings.Join(byPath[longPath].Limits, " "), "too large to diff here — open both sources") || bytes.Contains(result.Raw, []byte(longPath)) {
		t.Fatal("oversized path was rendered instead of retained as a limitation")
	}
	modePatch := sectionFor(t, result.Files, result.Raw, "mode.sh")
	if !bytes.Contains(modePatch, []byte("old mode 100644\nnew mode 100755")) || bytes.Contains(modePatch, []byte("@@ ")) {
		t.Fatal("mode-only diff was not explicit", modePatch)
	}
	if byPath["unknown.env"].Change != "unknown" || !strings.Contains(strings.Join(byPath["unknown.env"].Limits, " "), "excluded: untracked; not selected") {
		t.Fatal("unknown limitation was altered", byPath["unknown.env"])
	}
	if bytes.Contains(sectionFor(t, result.Files, result.Raw, "unknown.env"), []byte("@@ ")) {
		t.Fatal("unknown path received a computed hunk")
	}
	if result.Added != 1 || result.Deleted != 1 {
		t.Fatalf("computed line counts = +%d -%d", result.Added, result.Deleted)
	}
	if len(v.Hunks()) != 0 {
		t.Fatal("computed hunks were added to captured hunk index")
	}
	counts, err := v.Count(nil, nil)
	if err != nil || counts.Total != 0 || counts.Complete {
		t.Fatalf("computed output changed captured Count semantics: %+v %v", counts, err)
	}
}

func TestComputedBinaryDetectionIsLimitedToFirstEightThousandBytes(t *testing.T) {
	before := bytes.Repeat([]byte{'a'}, 8001)
	after := bytes.Repeat([]byte{'b'}, 8001)
	after[8000] = 0
	entry := Entry{Path: "late-nul", Change: "modified"}
	patch, _, _, _, binary, err := ComputeSourceDiff(entry, before, after)
	if err != nil || binary || bytes.Contains(patch, []byte("Binary files")) {
		t.Fatalf("NUL after the first 8000 bytes was classified binary: %q %t %v", patch, binary, err)
	}
	before[0] = 0
	_, _, _, _, binary, err = ComputeSourceDiff(entry, before, after)
	if err != nil || !binary {
		t.Fatalf("NUL in first 8000 bytes was not classified binary: %t %v", binary, err)
	}
}

func TestComputedDiffWorkLimit(t *testing.T) {
	entry := Entry{Path: "work.txt", Change: "modified"}
	before := bytes.Repeat([]byte("a\n"), 1000)
	after := bytes.Repeat([]byte("b\n"), 1000)
	_, _, _, _, _, err := ComputeSourceDiff(entry, before, after)
	if err == nil || err.Error() != "too large to diff here — open both sources" {
		t.Fatalf("pathological edit walk did not stop at its work bound: %v", err)
	}
}

func TestComputedOutputBudget(t *testing.T) {
	fixtures := make([]sourceFixture, 300)
	for i := range fixtures {
		path := fmt.Sprintf("%03d", i) + strings.Repeat("界", (MaxComputedPathBytes-4)/3)
		fixtures[i] = sourceFixture{path: path, base: []byte("old\n"), candidate: []byte("new\n"), basePresent: true, candidatePresent: true}
	}
	v, _ := computedView(t, fixtures, nil)
	result, err := v.Compute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Raw) > MaxComputedOutputBytes || !bytes.Contains(result.Raw, []byte(computedOutputNotice)) || len(result.Files) >= len(fixtures) {
		t.Fatalf("computed output cap did not stop later inventory paths: output=%d files=%d limits=%v", len(result.Raw), len(result.Files), result.Limits)
	}
	if !strings.Contains(strings.Join(result.Inventory[len(result.Inventory)-1].Limits, " "), "too large to diff here — open both sources") {
		t.Fatal("path skipped at the output limit lacks its explicit limitation")
	}
}

func TestComputedTotalSourceBudget(t *testing.T) {
	fixtures := make([]sourceFixture, 11)
	before := bytes.Repeat([]byte("a"), 400<<10)
	after := bytes.Repeat([]byte("b"), 400<<10)
	for i := range fixtures {
		fixtures[i] = sourceFixture{path: fmt.Sprintf("file-%02d.txt", i), base: before, candidate: after, basePresent: true, candidatePresent: true}
	}
	v, _ := computedView(t, fixtures, nil)
	result, err := v.Compute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Raw), "too large to diff here — open both sources") {
		t.Fatal("total computed-source bound did not yield an explicit limit")
	}
	if !strings.Contains(strings.Join(result.Inventory[len(result.Inventory)-1].Limits, " "), "too large to diff here — open both sources") {
		t.Fatal("over-budget path lacks its explicit limitation")
	}
	if len(result.Raw) > MaxComputedOutputBytes {
		t.Fatalf("computed patch exceeded output budget: %d", len(result.Raw))
	}
}

func sectionFor(t *testing.T, files []PatchFile, raw []byte, path string) []byte {
	t.Helper()
	for _, file := range files {
		if file.Path == path {
			return raw[file.Start:file.End]
		}
	}
	t.Fatalf("missing computed section for %q", path)
	return nil
}

var unifiedHunk = regexp.MustCompile(`^@@ -(\d+),(\d+) \+(\d+),(\d+) @@$`)

func applyUnified(base, patch []byte) ([]byte, error) {
	oldLines := splitLines(base)
	lines := strings.Split(string(patch), "\n")
	var out bytes.Buffer
	oldAt := 0
	for i := 0; i < len(lines); i++ {
		match := unifiedHunk.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}
		oldStart, oldCount, newCount := 0, 0, 0
		if _, err := fmt.Sscanf(match[1], "%d", &oldStart); err != nil {
			return nil, err
		}
		if _, err := fmt.Sscanf(match[2], "%d", &oldCount); err != nil {
			return nil, err
		}
		if _, err := fmt.Sscanf(match[4], "%d", &newCount); err != nil {
			return nil, err
		}
		copyUntil := oldStart
		if oldCount > 0 {
			copyUntil--
		}
		if copyUntil < oldAt || copyUntil > len(oldLines) {
			return nil, fmt.Errorf("invalid old hunk start %d after %d", copyUntil, oldAt)
		}
		for oldAt < copyUntil {
			out.Write(oldLines[oldAt])
			oldAt++
		}
		oldUsed, newUsed := 0, 0
		for oldUsed < oldCount || newUsed < newCount {
			i++
			if i >= len(lines) {
				return nil, errors.New("truncated hunk")
			}
			line := lines[i]
			if line == "\\ No newline at end of file" {
				return nil, errors.New("orphan no-newline marker")
			}
			if len(line) == 0 {
				return nil, errors.New("empty hunk operation")
			}
			kind, payload := line[0], []byte(line[1:])
			noNewline := i+1 < len(lines) && lines[i+1] == "\\ No newline at end of file"
			switch kind {
			case ' ':
				if oldAt >= len(oldLines) || !bytes.Equal(oldLines[oldAt], appendNewline(payload, !noNewline)) {
					return nil, errors.New("context does not match base")
				}
				out.Write(oldLines[oldAt])
				oldAt++
				oldUsed++
				newUsed++
			case '-':
				if oldAt >= len(oldLines) || !bytes.Equal(oldLines[oldAt], appendNewline(payload, !noNewline)) {
					return nil, errors.New("deletion does not match base")
				}
				oldAt++
				oldUsed++
			case '+':
				out.Write(appendNewline(payload, !noNewline))
				newUsed++
			default:
				return nil, fmt.Errorf("unexpected hunk line %q", line)
			}
			if noNewline {
				i++
			}
		}
	}
	for oldAt < len(oldLines) {
		out.Write(oldLines[oldAt])
		oldAt++
	}
	return out.Bytes(), nil
}

func appendNewline(line []byte, newline bool) []byte {
	if newline {
		return append(append([]byte(nil), line...), '\n')
	}
	return append([]byte(nil), line...)
}

func FuzzUnifiedDiffAppliesAndIsDeterministic(f *testing.F) {
	for _, seed := range [][2]string{{"a\nb\nc\nd\ne\nold\ng\n", "a\nb\nc\nd\ne\nnew\ng\n"}, {"without newline", "also without newline"}, {"", "new\n"}, {"old\n", ""}, {"a\nb\nc\nd\ne\nf\ng\n", "A\nb\nc\nd\ne\nf\nG\n"}} {
		f.Add([]byte(seed[0]), []byte(seed[1]))
	}
	f.Fuzz(func(t *testing.T, base, candidate []byte) {
		if len(base)+len(candidate) > 64<<10 || hasNULPrefix(base) || hasNULPrefix(candidate) {
			t.Skip()
		}
		entry := Entry{Path: "src/file.go", Change: "modified", Base: &evidence.File{Path: "src/file.go", Mode: "100644"}, Candidate: &evidence.File{Path: "src/file.go", Mode: "100644"}}
		patch, _, _, _, _, err := ComputeSourceDiff(entry, base, candidate)
		if err != nil {
			t.Skip()
		}
		got, err := applyUnified(base, patch)
		if err != nil || !bytes.Equal(got, candidate) {
			t.Fatalf("computed patch failed to apply: err=%v\nbase=%q\ncandidate=%q\npatch=%q\ngot=%q", err, base, candidate, patch, got)
		}
		again, _, _, _, _, err := ComputeSourceDiff(entry, base, candidate)
		if err != nil || !bytes.Equal(patch, again) {
			t.Fatalf("same source pair produced a different diff: %v", err)
		}
	})
}

func TestComputedDiffInventoryOrderIsStable(t *testing.T) {
	fixtures := []sourceFixture{
		{path: "z", base: []byte("a\n"), candidate: []byte("b\n"), basePresent: true, candidatePresent: true},
		{path: "a", base: []byte("c\n"), candidate: []byte("d\n"), basePresent: true, candidatePresent: true},
	}
	v, _ := computedView(t, fixtures, nil)
	first, err := v.Compute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := v.Compute(context.Background())
	if err != nil || !bytes.Equal(first.Raw, second.Raw) {
		t.Fatalf("compute was not deterministic: %v", err)
	}
	paths := make([]string, len(first.Files))
	for i, file := range first.Files {
		paths[i] = file.Path
	}
	want := []string{"a", "z"}
	if !sort.StringsAreSorted(paths) || strings.Join(paths, "|") != strings.Join(want, "|") {
		t.Fatal(paths)
	}
}

func TestComputedDiffRunsWithoutExecutableSearchPath(t *testing.T) {
	v, _ := computedView(t, []sourceFixture{{path: "file", base: []byte("old\n"), candidate: []byte("new\n"), basePresent: true, candidatePresent: true}}, nil)
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty-path"))
	result, err := v.Compute(context.Background())
	t.Setenv("PATH", oldPath)
	if err != nil || !bytes.Contains(result.Raw, []byte("@@ -1,1 +1,1 @@")) {
		t.Fatalf("pure Go computed diff failed without executable search path: %v", err)
	}
}

package browser

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
)

func changesFixture(t *testing.T) (*store.Store, Selection) {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2025-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2025-01-01T00:00:00Z")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", err, out)
		}
	}
	write := func(name string, raw []byte) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "--template=", "-b", "main")
	write("fixture.expected.json", []byte("{\"value\":1}\n"))
	write("source.go", []byte("package fixture\n\nfunc Value() int { return 1 }\n"))
	write("mode.sh", []byte("#!/bin/sh\necho fixture\n"))
	write("removed.go", []byte("package fixture\n"))
	write("image.bin", []byte{0, 1, 2, 3})
	git("add", "--all")
	git("commit", "-qm", "base")
	write("fixture.expected.json", []byte("{\"value\":2}\n"))
	write("source.go", []byte("package fixture\n\nfunc Value() int { return 2 }\n"))
	write("image.bin", []byte{0, 1, 7, 9})
	if err := os.Chmod(filepath.Join(dir, "mode.sh"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "removed.go")); err != nil {
		t.Fatal(err)
	}
	write("added.go", []byte("package fixture\n"))
	write("excluded.txt", []byte("not selected\n"))
	if err := os.Symlink("source.go", filepath.Join(dir, "unsupported-link")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	result, err := capture.Capture(t.Context(), dir, s, capture.Options{IncludeUntracked: []string{"added.go", "unsupported-link"}})
	if err != nil {
		t.Fatal(err)
	}
	return s, Selection{Project: dir, Pair: evidence.SnapshotPair{Base: result.Base.ID, Candidate: result.Candidate.ID}}
}

func TestChangesGroupsFlagsCountsSectionsAndPreview(t *testing.T) {
	_, sel := changesFixture(t)
	d, err := Load(t.Context(), sel)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{"added.go", "excluded.txt", "fixture.expected.json", "image.bin", "mode.sh", "removed.go", "source.go", "unsupported-link"}
	if len(d.Inventory) != len(wantPaths) {
		t.Fatalf("inventory length %d: %+v", len(d.Inventory), d.Inventory)
	}
	byPath := make(map[string]Entry)
	for i, entry := range d.Inventory {
		byPath[entry.Name] = entry
		if entry.Name != wantPaths[i] {
			t.Fatalf("inventory order %q at %d, want %q", entry.Name, i, wantPaths[i])
		}
		if len(entry.Sections) != 4 || entry.Sections[0].Name != "Diff" || entry.Sections[1].Name != "Base source" || entry.Sections[2].Name != "Candidate source" || entry.Sections[3].Name != "Inventory record" {
			t.Fatalf("path sections: %+v", entry.Sections)
		}
	}
	for _, path := range []string{"fixture.expected.json", "image.bin", "mode.sh", "added.go", "removed.go", "source.go"} {
		if byPath[path].Change == "unknown" {
			t.Fatalf("known path was classified unknown: %+v", byPath[path])
		}
	}
	if !byPath["fixture.expected.json"].PotentialOracle || !strings.Contains(byPath["fixture.expected.json"].Summary, "oracle") || !strings.Contains(byPath["fixture.expected.json"].Summary, "+1") || !strings.Contains(byPath["fixture.expected.json"].Summary, "−1") {
		t.Fatal("oracle flags or patch counts missing", byPath["fixture.expected.json"])
	}
	if !byPath["image.bin"].Binary || !strings.Contains(byPath["image.bin"].Summary, "binary") {
		t.Fatal("binary flag missing", byPath["image.bin"])
	}
	if !strings.Contains(diffFileForPath(d.Diff, "image.bin").Summary, "binary") || !strings.Contains(diffFileForPath(d.Diff, "mode.sh").Summary, "mode-only") || !strings.Contains(diffFileForPath(d.Diff, "added.go").Summary, "added") || !strings.Contains(diffFileForPath(d.Diff, "removed.go").Summary, "deleted") {
		t.Fatal("typed diff summary dividers missing", d.Diff.Files)
	}
	if !bytes.Contains(d.Diff.Raw, []byte("old mode 100644\nnew mode 100755")) || !bytes.Contains(d.Diff.Raw, []byte("GIT binary patch")) {
		t.Fatal("mode or binary raw patch lines were hidden")
	}
	if byPath["mode.sh"].BaseMode != "100644" || byPath["mode.sh"].CandidateMode != "100755" || !strings.Contains(byPath["mode.sh"].Summary, "mode 100644 → 100755") {
		t.Fatal("mode flag missing", byPath["mode.sh"])
	}
	if byPath["added.go"].Change != "added" || byPath["removed.go"].Change != "deleted" || byPath["excluded.txt"].Change != "unknown" || byPath["unsupported-link"].Change != "unknown" {
		t.Fatal("A/D/M/? inventory types missing", byPath)
	}
	if !strings.Contains(byPath["excluded.txt"].Summary, "excluded: untracked; not selected") || !strings.Contains(byPath["unsupported-link"].Summary, "unsupported:") {
		t.Fatal("recorded limitations were not kept verbatim", byPath)
	}
	baseAdded, err := ReadSection(t.Context(), sel.Project, byPath["added.go"].Sections[1])
	if err != nil || !strings.Contains(string(baseAdded), "No base source") {
		t.Fatalf("added path missing-side explanation: %q %v", baseAdded, err)
	}
	candidateDeleted, err := ReadSection(t.Context(), sel.Project, byPath["removed.go"].Sections[2])
	if err != nil || !strings.Contains(string(candidateDeleted), "No candidate source") {
		t.Fatalf("deleted path missing-side explanation: %q %v", candidateDeleted, err)
	}
	unknownSide, err := ReadSection(t.Context(), sel.Project, byPath["excluded.txt"].Sections[1])
	if err != nil || !strings.Contains(string(unknownSide), "source unavailable") || !strings.Contains(string(unknownSide), "excluded: untracked; not selected") {
		t.Fatalf("unknown path missing-side explanation: %q %v", unknownSide, err)
	}
	if len(d.InventoryRows) == 0 || d.InventoryRows[0].Heading != "POTENTIAL ORACLES" {
		t.Fatalf("potential-oracle group is not first: %+v", d.InventoryRows)
	}
	var groupOrder []string
	for _, row := range d.InventoryRows {
		if row.Heading != "" {
			groupOrder = append(groupOrder, row.Heading)
		}
	}
	if strings.Join(groupOrder, "|") != "POTENTIAL ORACLES|CHANGED|UNKNOWN — not fully captured" {
		t.Fatal("inventory groups", groupOrder)
	}
	m := New(t.Context(), sel, Jobs{})
	defer m.Close()
	m.data, m.width, m.height = d, 120, 40
	m.screen = "inventory"
	m.theme.Color = false
	m.inventory = len(d.Inventory) - 1
	view := m.View()
	for _, expected := range []string{"POTENTIAL ORACLES", "CHANGED", "UNKNOWN — not fully captured", "SELECTED PATH", "unsupported:"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("Changes view missing %q:\n%s", expected, view)
		}
	}
	if !strings.Contains(view, "?  unsupported-link") {
		t.Fatalf("unknown path letter missing:\n%s", view)
	}
	if !strings.Contains(view, "A  added.go") || !strings.Contains(view, "D  removed.go") || !strings.Contains(view, "M  source.go") {
		t.Fatalf("change letters missing:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if strings.ContainsAny(line, "\x1b\r\a") {
			t.Fatalf("unsafe inventory line %q", line)
		}
	}
}

func TestChangesDiffNavigationAndCapturedBytes(t *testing.T) {
	_, sel := changesFixture(t)
	d, err := Load(t.Context(), sel)
	if err != nil {
		t.Fatal(err)
	}
	if d.Diff == nil || d.Diff.Origin != "captured patch" || len(d.Diff.Files) < 5 || len(d.Diff.HunkRows) < 2 {
		t.Fatalf("fixture lost captured patch/hunk accounting: %+v", d.Diff)
	}
	m := New(t.Context(), sel, Jobs{})
	defer m.Close()
	m.data = d
	m.theme.Color = false
	m.width, m.height = 120, 40
	m.screen = "inventory"
	m.inventory = 0
	step(m, key("3"))
	if m.screen != "patch" || m.doc == nil {
		t.Fatalf("Diff did not open: %s", m.View())
	}
	firstPath := d.Inventory[0].Name
	if fileIndex, ok := d.Diff.FileByPath[firstPath]; ok && m.top != d.Diff.Files[fileIndex].StartRow {
		t.Fatalf("Diff did not land on selected path %q: top=%d", firstPath, m.top)
	}
	if !bytes.Equal(m.doc.RawBytes(), d.Diff.Raw) {
		t.Fatal("gutter projection changed captured patch bytes")
	}
	if !strings.Contains(m.View(), "captured patch") || !strings.Contains(m.View(), "file 1 of") {
		t.Fatal("sticky origin/file header missing", m.View())
	}
	beforeFile := diffPosition(d.Diff.Rows, m.top)
	step(m, key("]"))
	if diffPosition(d.Diff.Rows, m.top) != beforeFile+1 {
		t.Fatal("] did not navigate through indexed files")
	}
	if !strings.Contains(m.diffStickyHeader(), d.Diff.Files[beforeFile+1].Path) {
		t.Fatal("sticky header did not follow file navigation", m.diffStickyHeader())
	}
	step(m, key("["))
	if diffPosition(d.Diff.Rows, m.top) != beforeFile {
		t.Fatal("[ did not return to prior indexed file")
	}
	step(m, key("}"))
	if m.top != d.Diff.HunkRows[0] {
		t.Fatalf("} did not navigate to first indexed hunk: %d", m.top)
	}
	step(m, key("}"))
	if m.top != d.Diff.HunkRows[1] {
		t.Fatalf("} did not navigate to next indexed hunk: %d", m.top)
	}
	step(m, key("{"))
	if m.top != d.Diff.HunkRows[0] {
		t.Fatalf("{ did not navigate to previous indexed hunk: %d", m.top)
	}
	foundAdded, foundDeleted := false, false
	for index, row := range d.Diff.Rows {
		if row.HasNew && row.Style == terminal.Observed {
			text := m.diffDocumentRow(index)
			foundAdded = strings.Contains(text, "+")
		}
		if row.HasOld && !row.HasNew && row.Style == terminal.Problem {
			text := m.diffDocumentRow(index)
			foundDeleted = strings.Contains(text, "-")
		}
		if foundAdded && foundDeleted {
			break
		}
	}
	if !foundAdded || !foundDeleted {
		t.Fatalf("plain Diff lost +/- meaning: added=%t deleted=%t", foundAdded, foundDeleted)
	}
	m.theme.Color = true
	colored := ""
	for index, row := range d.Diff.Rows {
		if row.HasNew && row.Style == terminal.Observed {
			colored = m.diffDocumentRow(index)
			break
		}
	}
	if !strings.Contains(colored, "\x1b[32m") || !strings.Contains(colored, "\x1b[0m") {
		t.Fatalf("added diff line is not colored: %q", colored)
	}

	m.screen, m.returnTo = "inventory", ""
	for i, entry := range d.Inventory {
		if entry.Name == "fixture.expected.json" {
			m.inventory = i
		}
	}
	step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != "inspector" || len(m.sections()) != 4 || m.sections()[0].Name != "Diff" {
		t.Fatalf("path detail sections/order: %+v", m.sections())
	}
	if !bytes.Contains(m.doc.RawBytes(), []byte("diff --git a/fixture.expected.json")) {
		t.Fatalf("path Diff section did not retain its own hunks: %q", m.doc.RawBytes())
	}
}

func TestPairWithoutSharedPatchNeverUsesAnotherPatch(t *testing.T) {
	s, first := setup(t, false)
	base, err := store.Get[evidence.Snapshot](s, first.Pair.Base)
	if err != nil {
		t.Fatal(err)
	}
	oldCandidate, err := store.Get[evidence.Snapshot](s, first.Pair.Candidate)
	if err != nil {
		t.Fatal(err)
	}
	foreignPatch, err := s.ReadBlob(oldCandidate.Diff)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first.Project, "app/config.go"), []byte("package main\nconst retentionSeconds = 45\n"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := capture.Capture(t.Context(), first.Project, s, capture.Options{})
	if err != nil {
		t.Fatal(err)
	}
	pair := Selection{Project: first.Project, Pair: evidence.SnapshotPair{Base: base.ID, Candidate: second.Candidate.ID}}
	d, err := Load(t.Context(), pair)
	if err != nil {
		t.Fatal(err)
	}
	if d.Diff == nil || d.Diff.Origin != rawdiff.ComputedOrigin {
		t.Fatal("cross-capture pair did not receive a computed source diff", d.Diff)
	}
	if !d.Diff.SourceLimited {
		t.Fatal("unknown inventory limitations were not marked in the computed summary")
	}
	if bytes.Contains(d.Patch.Content, foreignPatch) {
		t.Fatal("patch from another pair was shown")
	}
	if !strings.Contains(d.Patch.Name, rawdiff.ComputedOrigin) || !bytes.Contains(d.Patch.Content, []byte("@@ -")) || !bytes.Contains(d.Patch.Content, []byte("retentionSeconds = 45")) {
		t.Fatalf("computed retention diff missing: %s %s", d.Patch.Name, d.Patch.Content)
	}
	var config Entry
	for _, entry := range d.Inventory {
		if entry.Name == "app/config.go" {
			config = entry
		}
	}
	if config.Name == "" || !bytes.Contains(config.Sections[0].Content, []byte("retentionSeconds = 45")) {
		t.Fatal("path detail did not show the computed source diff", config)
	}
	baseSource, err := ReadSection(t.Context(), pair.Project, config.Sections[1])
	if err != nil || !bytes.Contains(baseSource, []byte("retentionSeconds int64 = 24 * 60 * 60")) {
		t.Fatalf("base source unavailable: %q %v", baseSource, err)
	}
	candidateSource, err := ReadSection(t.Context(), pair.Project, config.Sections[2])
	if err != nil || !bytes.Contains(candidateSource, []byte("retentionSeconds = 45")) {
		t.Fatalf("candidate source unavailable: %q %v", candidateSource, err)
	}
	m := New(t.Context(), pair, Jobs{})
	defer m.Close()
	m.data, m.width, m.height = d, 120, 40
	m.screen = "inventory"
	if !strings.Contains(m.View(), rawdiff.ComputedOrigin) || !strings.Contains(m.View(), "limited inventory") {
		t.Fatal("Changes counts did not label the computed origin and partial inventory", m.View())
	}
	step(m, key("1"))
	if !strings.Contains(m.View(), "CHANGES ") || !strings.Contains(m.View(), rawdiff.ComputedOrigin) || !strings.Contains(m.View(), "limited inventory") {
		t.Fatal("Overview CHANGES line did not label its computed origin and partial inventory", m.View())
	}
	step(m, key("2"))
	step(m, key("3"))
	if m.screen != "patch" || m.doc == nil || !strings.Contains(m.diffStickyHeader(), rawdiff.ComputedOrigin) || !strings.Contains(m.View(), "retentionSeconds = 45") {
		t.Fatal("Diff did not show the computed source diff", m.View())
	}
}

func diffFileForPath(diff *DiffView, path string) DiffFile {
	if diff != nil {
		if index, ok := diff.FileByPath[path]; ok {
			return diff.Files[index]
		}
	}
	return DiffFile{}
}

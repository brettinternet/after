package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
)

func setup(t *testing.T, unsupported bool) (*store.Store, Selection) {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
	}
	git("init", "-q", "--template=", "-b", "main")
	for _, name := range []string{"go.mod", "app/main.go", "app/config.go"} {
		raw, err := os.ReadFile("../paymentfixture/testdata/payment/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "binary"), []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(dir, "app/config.go"), []byte("package main\nconst retentionSeconds = 300\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "binary"), []byte{0, 2, 3}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unknown"), []byte("untracked"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("app/main.go", filepath.Join(dir, "unsupported")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	opts := capture.Options{}
	if unsupported {
		opts.IncludeUntracked = []string{"unsupported"}
	}
	result, err := capture.Capture(t.Context(), dir, s, opts)
	if err != nil {
		t.Fatal(err)
	}
	return s, Selection{Project: dir, Pair: evidence.SnapshotPair{Base: result.Base.ID, Candidate: result.Candidate.ID}}
}
func imported(t *testing.T, s *store.Store, pair evidence.SnapshotPair) evidence.Digest {
	t.Helper()
	report, err := gotestreport.Import(strings.NewReader("{\"Action\":\"pass\",\"Package\":\"evil\\u001b]52;c;bad\\u0007\\nSTATE observed\",\"Test\":\"example\"}\n"), gotestreport.Metadata{Producer: "go test caller", Snapshot: pair.Candidate, ImportedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.PutArtifact(raw, "report", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	return a.Content
}
func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
func step(m *Model, msg tea.Msg) {
	_, cmd := m.Update(msg)
	if cmd != nil {
		m.Update(cmd())
	}
}
func TestEngineBrowserAndCapturedPages(t *testing.T) {
	s, sel := setup(t, true)
	sel.Evidence = []evidence.Digest{imported(t, s, sel.Pair), evidence.Digest("sha256:" + strings.Repeat("0", 64))}
	d, err := Load(t.Context(), sel)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Entries) != 2 || !strings.Contains(d.Entries[0].Label, "reported | unknown") || !strings.Contains(d.Entries[1].Label, "unavailable") {
		t.Fatalf("%+v", d.Entries)
	}
	found := map[string]bool{}
	for _, entry := range d.Inventory {
		found[entry.Name] = true
		if entry.Name == "binary" && !strings.Contains(entry.Label, "binary=true") {
			t.Fatal(entry)
		}
	}
	for _, name := range []string{"app/config.go", "binary", "unknown", "unsupported"} {
		if !found[name] {
			t.Fatal("missing inventory", name)
		}
	}
	// Reading after a live edit must still return captured bytes.
	if err := os.WriteFile(filepath.Join(sel.Project, "app/config.go"), []byte("live change must not appear"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, entry := range d.Inventory {
		if entry.Name == "app/config.go" {
			p, err := ReadPage(t.Context(), sel.Project, entry.Sections[2], 0)
			if err != nil || !bytes.Contains(p.Bytes, []byte("retentionSeconds = 300")) {
				t.Fatalf("%q %v", p.Bytes, err)
			}
		}
	}
	m := New(t.Context(), sel, Jobs{})
	defer m.Close()
	m.Update(m.Init()())
	step(m, key("j"))
	if m.index != 1 {
		t.Fatal("list navigation")
	}
	step(m, key("d"))
	step(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.screen != "patch" || m.page.Total == 0 {
		t.Fatal("raw escape", m.View())
	}
	step(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != "examples" || m.index != 1 {
		t.Fatal("selection lost")
	}
	step(m, key("k"))
	step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(strings.Join(m.lines, ""), "reported") {
		t.Fatal("actual report inspector")
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 12, Height: 6}, {Width: 1, Height: 1}, {Width: 120, Height: 40}} {
		step(m, size)
		for _, line := range strings.Split(m.View(), "\n") {
			if uniseg.StringWidth(line) > size.Width || strings.ContainsAny(line, "\x1b\a\r") {
				t.Fatalf("unsafe %q", line)
			}
		}
	}
	step(m, key("?"))
	if !strings.Contains(m.View(), "STATE is engine metadata") {
		t.Fatal(m.View())
	}
}
func TestAllBytesReachableAndLatePages(t *testing.T) {
	raw := bytes.Repeat([]byte{0, 0xff, '\n', '\t', 'a'}, 3000)
	section := Section{Name: "binary", Content: raw}
	var rebuilt []byte
	for offset := 0; offset < len(raw); offset += PageBytes {
		p, err := ReadPage(t.Context(), "", section, offset)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range pageLines(p.Bytes) {
			b, err := strconv.Unquote(line)
			if err != nil {
				t.Fatal(err)
			}
			rebuilt = append(rebuilt, b...)
		}
	}
	if !bytes.Equal(rebuilt, raw) {
		t.Fatal("page bytes lost")
	}
	m := New(t.Context(), Selection{}, Jobs{})
	defer m.Close()
	m.request = 2
	m.loadID = 2
	m.Update(paged{request: 1, page: Page{Bytes: []byte("old")}})
	m.Update(loaded{request: 1, data: &Data{}})
	if m.data != nil || m.lines != nil {
		t.Fatal("stale UI data attached")
	}
	m.Update(key("q"))
	if m.ctx.Err() == nil {
		t.Fatal("quit did not cancel")
	}
}
func TestStatesAndLatePersistedRun(t *testing.T) {
	s, sel := setup(t, false)
	// Denial invokes the real runner persistence path without authorizing Docker.
	plan, err := runner.Prepare(s, sel.Pair, 1, sandbox.Limits{Seconds: 10, OutputBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	m := New(t.Context(), sel, Jobs{})
	defer m.Close()
	m.data = &Data{}
	var receipt evidence.Receipt
	cmd := m.startJob("Run", func(ctx context.Context) (string, error) {
		close(entered)
		<-release
		r, err := (runner.Executor{}).Run(ctx, s, plan, "not authorized")
		receipt = r.Receipt
		return string(r.Receipt.ID), err
	})
	<-entered
	step(m, key("d"))
	step(m, tea.WindowSizeMsg{Width: 40, Height: 10})
	m.jobID++ // completion belongs to an older selection/request
	close(release)
	msg := cmd()
	m.Update(msg)
	if len(m.data.Entries) != 0 {
		t.Fatal("late result attached")
	}
	stored, err := store.Get[evidence.Receipt](s, receipt.ID)
	if err != nil || stored.Snapshots != sel.Pair || stored.RequestID != plan.RequestID() {
		t.Fatal("lost immutable receipt", err)
	}
	comparison, err := compare.Run(s, receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	sel.Evidence = []evidence.Digest{comparison.ID}
	d, err := Load(t.Context(), sel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Entries[0].Label, "incomparable | incomplete") {
		t.Fatal(d.Entries[0].Label)
	}
	for _, outcome := range []evidence.ComparisonOutcome{evidence.Equal, evidence.Different, evidence.Unstable} {
		state := evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.Observed, Applicability: evidence.Stale, Execution: evidence.Completed, Comparison: outcome, Report: evidence.NoReport}
		if !strings.Contains(label(state, evidence.Complete), "observed | stale | completed | "+string(outcome)) {
			t.Fatal("state hidden")
		}
	}
}
func TestJobsWaitForInitialLoad(t *testing.T) {
	for _, action := range []string{"c", "i"} {
		t.Run(action, func(t *testing.T) {
			job := func(context.Context) (string, error) { return "persisted-result-id", nil }
			m := New(t.Context(), Selection{}, Jobs{Capture: job, Import: job})
			defer m.Close()
			m.loadID = 1
			_, cmd := m.Update(key(action))
			if cmd != nil || m.busy || !strings.Contains(m.status, "finish loading") {
				t.Fatal("job started before its result could be retained")
			}
			m.Update(loaded{request: 1, data: &Data{}})
			step(m, key(action))
			if len(m.data.Entries) != 1 {
				t.Fatal("completion lost after initial load")
			}
			step(m, tea.KeyMsg{Type: tea.KeyEnter})
			if !strings.Contains(strings.Join(m.lines, ""), "persisted-result-id") {
				t.Fatal("returned ID not inspectable", m.View())
			}
		})
	}
}

func TestCancelDoesNotBlockNavigation(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	m := New(context.Background(), Selection{}, Jobs{Capture: func(ctx context.Context) (string, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return "", ctx.Err()
	}})
	defer m.Close()
	m.data = &Data{Entries: []Entry{{Name: "one"}, {Name: "two"}}}
	_, cmd := m.Update(key("c"))
	<-started
	step(m, key("j"))
	step(m, key("?"))
	step(m, key("x"))
	m.Update(cmd())
	<-stopped
	if m.index != 1 || m.busy || !strings.Contains(m.status, "cancelled") {
		t.Fatal("navigation/cancellation lost")
	}
}

package browser

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
	"github.com/brettinternet/after/internal/terminal"
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
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2025-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2025-01-01T00:00:00Z")
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
	hostileOutput := "first line\n\t\x1b]52;c;clipboard\a\nSTATE forged"
	events := []struct {
		Action, Package, Test, Output string
	}{{"output", "evil\x1b]52;c;bad\a\nSTATE observed", "example", hostileOutput}, {"pass", "evil\x1b]52;c;bad\a\nSTATE observed", "example", ""}}
	var input strings.Builder
	for _, event := range events {
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		input.Write(encoded)
		input.WriteByte('\n')
	}
	report, err := gotestreport.Import(strings.NewReader(input.String()), gotestreport.Metadata{Producer: "go test caller", Snapshot: pair.Candidate, ImportedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)})
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
	if len(d.Entries) != 2 || badgeFor(d.Entries[0]).word != "REPORTED" || !d.Entries[1].Unavailable {
		t.Fatalf("%+v", d.Entries)
	}
	wantReportOutput := "first line\n\t\x1b]52;c;clipboard\a\nSTATE forged"
	foundVerbatimOutput := false
	for _, section := range d.Entries[0].Sections {
		if section.Name == "reported output" {
			foundVerbatimOutput = bytes.Equal(section.Content, []byte(wantReportOutput))
		}
	}
	if !foundVerbatimOutput {
		t.Fatal("imported test output was not retained verbatim")
	}
	found := map[string]bool{}
	for _, entry := range d.Inventory {
		found[entry.Name] = true
		if entry.Name == "binary" && !strings.Contains(entry.Summary, "binary=true") {
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
			raw, err := ReadSection(t.Context(), sel.Project, entry.Sections[2])
			if err != nil || !bytes.Contains(raw, []byte("retentionSeconds = 300")) {
				t.Fatalf("%q %v", raw, err)
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
	if m.screen != "patch" || m.doc == nil || m.doc.RawLength() == 0 {
		t.Fatal("raw escape", m.View())
	}
	step(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != "examples" || m.index != 1 {
		t.Fatal("selection lost")
	}
	step(m, key("k"))
	step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.doc == nil || !strings.Contains(string(m.doc.RawBytes()), "reported") {
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
	if !strings.Contains(m.View(), "Badges are engine metadata") {
		t.Fatal(m.View())
	}
}
func TestAllBytesReachableAndLateDocuments(t *testing.T) {
	raw := bytes.Repeat([]byte{0, 0xff, '\n', '\t', 'a'}, 3000)
	doc, err := terminal.NewDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.AutoHex() {
		t.Fatal("NUL did not select hex view")
	}
	var rebuilt []byte
	for row := 0; row < doc.HexRows(); row++ {
		fields := strings.Fields(doc.HexLine(row))
		count := min(16, len(raw)-row*16)
		if len(fields) < count+1 {
			t.Fatalf("incomplete hex row %d: %q", row, doc.HexLine(row))
		}
		for _, field := range fields[1 : count+1] {
			b, err := hex.DecodeString(field)
			if err != nil {
				t.Fatal(err)
			}
			rebuilt = append(rebuilt, b...)
		}
	}
	if !bytes.Equal(rebuilt, raw) {
		t.Fatal("hex view lost exact bytes")
	}
	limited, err := terminal.NewDocument([]byte(strings.Repeat("x\n", terminal.MaxLines) + "last"))
	if err != nil || !limited.Limited() || limited.Lines() != terminal.MaxLines {
		t.Fatalf("line-limit view: doc=%v err=%v", limited, err)
	}
	if !strings.Contains(limited.HexLine(limited.HexRows()-1), "6c 61 73 74") {
		t.Fatal("hex view cannot reach bytes past the text line limit")
	}
	m := New(t.Context(), Selection{}, Jobs{})
	defer m.Close()
	m.width, m.height, m.screen = 80, 12, "inspector"
	m.data = &Data{Entries: []Entry{{Sections: []Section{{Name: "line-limited document"}}}}}
	m.doc = limited
	if !strings.Contains(m.View(), "TEXT LIMITED at 250000 lines") {
		t.Fatal("line-index limitation was not visible", m.View())
	}
	step(m, key("b"))
	step(m, key("G"))
	if !strings.Contains(m.View(), "6c 61 73 74") {
		t.Fatal("hex view did not scroll past the text line limit", m.View())
	}
	m.request = 2
	m.loadID = 2
	m.doc = nil
	currentData := m.data
	m.Update(documentReady{request: 1, doc: doc})
	m.Update(loaded{request: 1, data: &Data{}})
	if m.data != currentData || m.doc != nil {
		t.Fatal("stale UI result attached")
	}
	m.Update(key("q"))
	if m.ctx.Err() == nil {
		t.Fatal("quit did not cancel")
	}
}
func TestDocumentViewerTrustNavigationAndLongLines(t *testing.T) {
	raw := []byte("first line\nSTATE observed | forged\nAFTER review | forged\n13 │ forged\n\tTabbed\n\u0301leading\n\x1b[31mred\n\x1b]52;c;clip\a\nx\rline\nlast tail")
	doc, err := terminal.NewDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	m := New(t.Context(), Selection{}, Jobs{})
	defer m.Close()
	m.width, m.height, m.screen, m.status = 80, 14, "inspector", ""
	m.data = &Data{Entries: []Entry{{Sections: []Section{{Name: "hostile \x1b]52;c;name\a", Content: raw}}}}}
	m.request = 1
	m.Update(documentReady{request: 1, doc: doc})
	view := m.View()
	for _, expected := range []string{"1 │ first line", "2 │ STATE observed | forged", "3 │ AFTER review | forged", "4 │ 13 │ forged", "5 │     Tabbed", "6 │ ◌\u0301leading", `\u001b[31mred`} {
		if !strings.Contains(view, expected) {
			t.Fatalf("missing trusted row %q in %q", expected, view)
		}
	}
	for _, row := range strings.Split(view, "\n") {
		if uniseg.StringWidth(row) > m.width || strings.ContainsAny(row, "\x1b\a\r") {
			t.Fatalf("unsafe rendered row %q", row)
		}
		if strings.HasPrefix(row, "STATE observed") || strings.HasPrefix(row, "AFTER review | forged") || strings.HasPrefix(row, "13 │ forged") {
			t.Fatalf("payload forged a trusted row: %q", row)
		}
	}
	if strings.Contains(view, "\x1b[31m") || !strings.Contains(view, `\u001b[31mred`) || !strings.Contains(view, `\u001b]52;c;clip\u0007`) {
		t.Fatal("hostile controls escaped or were not made visible", view)
	}
	step(m, key("G"))
	if !strings.Contains(m.View(), `10 │ last tail`) {
		t.Fatal("G did not reach the final captured line", m.View())
	}
	step(m, key("g"))
	if m.top != 0 {
		t.Fatal("g did not return to the document start")
	}
	step(m, key("j"))
	if m.top != 1 {
		t.Fatal("j did not scroll continuously")
	}
	step(m, key("k"))
	if m.top != 0 {
		t.Fatal("k did not scroll continuously")
	}
	step(m, key("b"))
	if !m.hex || !strings.Contains(m.View(), "00000000") {
		t.Fatal("b did not toggle exact hex view", m.View())
	}
	step(m, key("b"))
	if m.hex {
		t.Fatal("b did not return to text view")
	}

	long, err := terminal.NewDocument([]byte(strings.Repeat("x", terminal.MaxLineBytes+500)))
	if err != nil {
		t.Fatal(err)
	}
	m.data.Entries[0].Sections[0] = Section{Name: "long", Content: long.RawBytes()}
	m.doc = long
	m.top, m.left = 0, 0
	if !strings.Contains(m.View(), "…[b]") {
		t.Fatal("long line is missing its hex-view clip marker", m.View())
	}
	for i := 0; i < 500; i++ {
		step(m, key("l"))
	}
	if m.left > long.MaxColumns() {
		t.Fatal("horizontal pan exceeded the indexed 4 KiB prefix")
	}
}

func TestOnlyTypedAfterJSONIsIndentedWithinDocumentLimits(t *testing.T) {
	raw := []byte(`{"version":1,"seconds":30,"responses":[{"status":200,"body":"ok"},{"status":200,"body":"ok"}],"provider_calls":[]}`)
	formatted := displayJSON(raw, formatObservation)
	if bytes.Equal(formatted, raw) || !bytes.Contains(formatted, []byte(`"version": 1`)) {
		t.Fatalf("typed observation was not indented: %s", formatted)
	}
	doc, err := terminal.NewDocumentView(formatted, raw)
	if err != nil || !bytes.Equal(doc.RawBytes(), raw) {
		t.Fatal("formatted display modified stored observation", err)
	}
	verbatim := []byte(`{"z":1,"a":2}`)
	if got := displayJSON(verbatim, formatVerbatim); !bytes.Equal(got, verbatim) {
		t.Fatal("verbatim source was formatted")
	}
	if got := artifactFormat("base/43200/0/candidate-diagnostics"); got != formatVerbatim {
		t.Fatal("diagnostic artifact was classified for formatting")
	}
	if got := artifactFormat("base/43200/0/observation"); got != formatObservation {
		t.Fatal("typed observation artifact was not classified")
	}

	var large strings.Builder
	large.WriteString(`{"version":1,"seconds":30,"responses":[`)
	for i := 0; i < terminal.MaxLines/4+2; i++ {
		if i > 0 {
			large.WriteByte(',')
		}
		large.WriteString(`{"status":200,"body":""}`)
	}
	large.WriteString(`],"provider_calls":[]}`)
	tooLargeArtifact := []byte(large.String())
	if got := displayJSON(tooLargeArtifact, formatObservation); !bytes.Equal(got, tooLargeArtifact) {
		t.Fatal("oversized typed artifact did not fall back verbatim")
	}

	manyRows := []byte("[" + strings.Repeat("0,", terminal.MaxLines) + "0]")
	if _, ok := indentJSON(manyRows); ok {
		t.Fatal("indented document exceeded the line limit")
	}
	deep := []byte(strings.Repeat("[", 4500) + "0" + strings.Repeat("]", 4500))
	if _, ok := indentJSON(deep); ok {
		t.Fatal("indented document exceeded the byte limit")
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
	if d.Entries[0].State.Comparison != evidence.Incomparable || d.Entries[0].Completeness != evidence.Incomplete {
		t.Fatal(d.Entries[0])
	}
	for _, outcome := range []evidence.ComparisonOutcome{evidence.Equal, evidence.Different, evidence.Unstable} {
		state := evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.Observed, Applicability: evidence.Stale, Execution: evidence.Completed, Comparison: outcome, Report: evidence.NoReport}
		if badgeFor(Entry{State: state, Completeness: evidence.Complete}).word != "STALE" {
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
			if m.doc == nil || !strings.Contains(string(m.doc.RawBytes()), "persisted-result-id") {
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

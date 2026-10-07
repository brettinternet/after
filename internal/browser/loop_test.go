package browser

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

func drain(m *Model, cmd tea.Cmd) {
	for cmd != nil {
		_, cmd = m.Update(cmd())
	}
}
func press(m *Model, s string) { _, cmd := m.Update(key(s)); drain(m, cmd) }

func TestLoopConsentAndSnapshotBarrier(t *testing.T) {
	s, sel := setup(t, false)
	s.Close()
	a := &Actions{Project: sel.Project, Repetitions: 1, Limits: sandbox.Limits{Seconds: 10, OutputBytes: 4096}}
	m := New(t.Context(), sel, Jobs{Actions: a})
	defer m.Close()
	drain(m, m.Init())
	press(m, "r")
	if m.screen != "plan" || len(m.preview) == 0 {
		t.Fatal(m.status)
	}
	old := append([]byte(nil), m.preview...)
	digest := m.digest
	press(m, "n")
	press(m, "y")
	if m.running || m.digest != "" {
		t.Fatal("denial retained consent")
	}
	// Changed bytes are reconstructed before any Docker capability is consulted.
	bad := []byte(strings.Replace(string(old), `"concurrency": 1`, `"concurrency": 2`, 1))
	if _, err := a.Run(t.Context(), sel.Pair, bad, digest); err == nil {
		t.Fatal("changed plan accepted")
	}
	press(m, "r")
	// A deterministic barrier controls a real persisted denied runner completion.
	entered, release := make(chan struct{}), make(chan struct{})
	shared, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := runner.PrepareFromPreview(shared, m.preview)
	if err != nil {
		t.Fatal(err)
	}
	m.running = true
	cmd := m.spawn(func() tea.Msg {
		close(entered)
		<-release
		result, err := (runner.Executor{}).Run(context.Background(), shared, plan, "denied")
		comparison, compareErr := compare.Run(shared, result.Receipt.ID)
		if compareErr != nil {
			return ran{pair: sel.Pair, err: compareErr}
		}
		return ran{pair: sel.Pair, comparison: comparison.ID, err: err}
	})
	<-entered
	before := m.selected.Pair
	press(m, "n")
	m.index = 0
	if err := os.WriteFile(filepath.Join(sel.Project, "app/config.go"), []byte("package main\nconst retentionSeconds = 60\n"), 0600); err != nil {
		t.Fatal(err)
	}
	press(m, "c")
	if m.pending == nil || m.selected.Pair != before || m.index != 0 {
		t.Fatal("arrival moved selection")
	}
	press(m, "a")
	if m.selected.Pair != before || m.pending == nil {
		t.Fatal("a must not switch snapshots", m.status)
	}
	press(m, "u")
	if m.selected.Pair == before || m.index != 0 || m.digest != "" {
		t.Fatal("explicit selection failed", m.status)
	}
	m.Update(prepared{generation: 0, pair: before, raw: old, digest: digest})
	if len(m.preview) != 0 {
		t.Fatal("late preview restored consent")
	}
	close(release)
	m.Update(cmd())
	if !strings.Contains(m.status, "Late result") || m.selected.Pair == before {
		t.Fatal(m.status)
	}
	c, err := store.Get[evidence.Comparison](shared, m.results[0])
	if err != nil {
		t.Fatal(err)
	}
	r, err := store.Get[evidence.Receipt](shared, c.Receipt)
	if err != nil || r.Snapshots != before || r.RequestID != plan.RequestID() {
		t.Fatal("late receipt rebound", r, err)
	}
	if len(m.selected.Evidence) != 0 {
		t.Fatal("late receipt attached")
	}
}

func TestLoopNoImplicitWriterAndPasteConsent(t *testing.T) {
	s, sel := setup(t, false)
	s.Close()
	a := &Actions{Project: sel.Project, Repetitions: 1, Limits: sandbox.Limits{Seconds: 10, OutputBytes: 4096}}
	m := New(t.Context(), sel, Jobs{Actions: a})
	defer m.Close()
	drain(m, m.Init())
	if a.s != nil {
		t.Fatal("open acquired writer")
	}
	press(m, "r")
	preview, digest := append([]byte(nil), m.preview...), m.digest
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y"), Paste: true})
	if m.running || m.screen != "plan" || !bytes.Equal(m.preview, preview) || m.digest != digest {
		t.Fatal("paste changed or authorized the retained plan")
	}
	press(m, "a") // no pending snapshot does not change the selection
	press(m, "n")
	if m.preview != nil || m.running {
		t.Fatal("denial executed")
	}
}

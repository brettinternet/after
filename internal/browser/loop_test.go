package browser

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

func drain(m *Model, cmd tea.Cmd) {
	for cmd != nil {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for index, command := range batch {
				if m.clockScheduled && index == len(batch)-1 {
					m.clockScheduled = false
					continue
				}
				drain(m, command)
			}
			return
		}
		_, cmd = m.Update(msg)
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
	if m.prompt == nil || m.selected.Pair != before {
		t.Fatal("u did not request explicit snapshot confirmation", m.status)
	}
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	drain(m, command)
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

func TestFollowUpPreviewAndComputedDiffUseActivePair(t *testing.T) {
	s, selection := setup(t, false)
	if err := os.WriteFile(filepath.Join(selection.Project, "app/config.go"), []byte("package main\nconst retentionSeconds = 60\n"), 0600); err != nil {
		t.Fatal(err)
	}
	captured, err := capture.Capture(t.Context(), selection.Project, s, capture.Options{})
	if err != nil {
		t.Fatal(err)
	}
	pair := evidence.SnapshotPair{Base: selection.Pair.Candidate, Candidate: captured.Candidate.ID}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	actions := &Actions{Project: selection.Project, Repetitions: 1, Limits: sandbox.Limits{Seconds: 10, OutputBytes: 4096}}
	followUp := Selection{Project: selection.Project, Pair: pair, Mode: evidence.FollowUp, Baseline: selection.Pair.Base}
	model := New(t.Context(), followUp, Jobs{Actions: actions})
	defer model.Close()
	drain(model, model.Init())
	if model.selected.Pair != pair || model.data.Diff == nil || model.data.Diff.Origin != rawdiff.ComputedOrigin {
		t.Fatalf("follow-up pair was not used for its computed diff: %+v %+v", model.selected.Pair, model.data.Diff)
	}
	press(model, "r")
	if model.screen != "plan" || model.planPair != pair {
		t.Fatalf("rerun preview was not bound to the active follow-up pair: %+v", model.planPair)
	}
	for _, id := range []evidence.Digest{pair.Base, pair.Candidate} {
		if !strings.Contains(string(model.preview), string(id)) {
			t.Fatalf("exact plan omitted active pair ID %s", id)
		}
	}
	press(model, "n")
	if model.running || len(model.preview) != 0 || model.digest != "" {
		t.Fatal("denying the follow-up plan retained consent or authorized execution")
	}
}

func TestFollowUpLateResultCannotAttachToDifferentPair(t *testing.T) {
	s, selection := setup(t, false)
	receipt := comparisonReceipt(t, s, selection)
	pin, err := review.Create(s, receipt.ID, "finite expectation", evidence.FiniteExample, "initial pin")
	if err != nil {
		t.Fatal(err)
	}
	comparison := viewComparison(t, s, selection, false, evidence.Complete)
	if err := os.WriteFile(filepath.Join(selection.Project, "app/config.go"), []byte("package main\nconst retentionSeconds = 60\n"), 0600); err != nil {
		t.Fatal(err)
	}
	captured, err := capture.Capture(t.Context(), selection.Project, s, capture.Options{})
	if err != nil {
		t.Fatal(err)
	}
	followUp := evidence.SnapshotPair{Base: selection.Pair.Candidate, Candidate: captured.Candidate.ID}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	actions := &Actions{Project: selection.Project}
	_, err = actions.Attach(Selection{Project: selection.Project, Pair: followUp, Mode: evidence.FollowUp, Baseline: selection.Pair.Base, Evidence: []evidence.Digest{pin.ID}}, comparison)
	if err == nil || !strings.Contains(err.Error(), "late result retained for originating snapshots only") {
		t.Fatalf("late original-base result attached to follow-up: %v", err)
	}
	stored, err := store.Get[evidence.Pin](actionsStore(t, actions), pin.ID)
	if err != nil || len(stored.History) != 1 {
		t.Fatalf("late result changed the pin history: %+v %v", stored, err)
	}
	if _, err := store.Get[evidence.Comparison](actionsStore(t, actions), comparison); err != nil {
		t.Fatalf("late result was not retained as history: %v", err)
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

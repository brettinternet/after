package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

func completePromptFixture(t *testing.T) (*store.Store, Selection) {
	t.Helper()
	s, selection := setup(t, false)
	for _, name := range []string{"unknown", "unsupported"} {
		if err := os.Remove(filepath.Join(selection.Project, name)); err != nil {
			t.Fatal(err)
		}
	}
	captured, err := capture.Capture(t.Context(), selection.Project, s, capture.Options{})
	if err != nil {
		t.Fatal(err)
	}
	selection.Pair = evidence.SnapshotPair{Base: captured.Base.ID, Candidate: captured.Candidate.ID}
	return s, selection
}

func promptModel(t *testing.T, s *store.Store, selection Selection) (*Model, *Actions) {
	t.Helper()
	data, err := Load(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	actions := &Actions{Project: selection.Project, Repetitions: 1}
	model := New(t.Context(), selection, Jobs{Actions: actions})
	model.data, model.selected = data, data.Selection
	model.selectFirstOverviewRow()
	t.Cleanup(model.Close)
	return model, actions
}

func comparisonReceipt(t *testing.T, s *store.Store, selection Selection) evidence.Receipt {
	t.Helper()
	comparisonID := viewComparison(t, s, selection, false, evidence.Complete)
	comparison, err := store.Get[evidence.Comparison](s, comparisonID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.Get[evidence.Receipt](s, comparison.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func TestPinPromptReasonDuplicateAndCancellation(t *testing.T) {
	s, selection := completePromptFixture(t)
	receipt := comparisonReceipt(t, s, selection)
	comparisonIDs, err := s.List("comparison")
	if err != nil || len(comparisonIDs) == 0 {
		t.Fatalf("comparison missing: %v", err)
	}
	selection.Evidence = []evidence.Digest{comparisonIDs[len(comparisonIDs)-1].ID}
	model, actions := promptModel(t, s, selection)
	entryIndex := -1
	for i, entry := range model.data.Entries {
		if entry.Receipt == receipt.ID && entry.Expectation != "" {
			entryIndex = i
			break
		}
	}
	if entryIndex < 0 {
		t.Fatal("synthetic complete observation did not produce a pinnable case")
	}
	model.positionOverviewEntry(entryIndex)

	step(model, key("p"))
	if model.prompt == nil || !strings.Contains(model.View(), string(receipt.ID)) || !strings.Contains(model.View(), string(selection.Pair.Candidate)) {
		t.Fatal("pin confirmation did not name its exact targets", model.View())
	}
	if _, _, err := actions.FindDuplicatePin(*model.selectedOverviewEntry()); err != nil {
		t.Fatal(err)
	}
	step(model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.prompt != nil || model.status != "Cancelled; no changes" {
		t.Fatal("Esc did not cancel pin confirmation", model.status)
	}
	reader := readStore(t, selection.Project)
	pins, err := review.Heads(reader)
	reader.Close()
	if err != nil || len(pins) != 0 {
		t.Fatalf("cancelled prompt created a pin: %v %v", pins, err)
	}

	step(model, key("p"))
	step(model, tea.KeyMsg{Type: tea.KeyCtrlU})
	payload := "recorded reason\n\x1b]52;c;blocked\a"
	step(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(payload), Paste: true})
	if model.prompt == nil || strings.ContainsAny(model.View(), "\x1b\a\r") || !strings.Contains(model.View(), `\u001b]52;c;blocked\u0007`) {
		t.Fatal("bracketed paste was not safe inert reason text", model.View())
	}
	step(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.prompt != nil || !strings.Contains(model.status, "Pinned") {
		t.Fatal("Enter did not perform confirmed pin", model.status)
	}
	pins = currentPins(t, selection.Project)
	if len(pins) != 1 || pins[0].BasisReceipt != receipt.ID || pins[0].Expectation != model.data.Entries[model.index].Expectation {
		t.Fatalf("pin target differs from confirmed observation: %+v", pins)
	}
	gotReason := pins[0].History[len(pins[0].History)-1].Reason
	if gotReason != `recorded reason\u000a\u001b]52;c;blocked\u0007` {
		t.Fatalf("sanitized entered reason was not retained: %q", gotReason)
	}

	model.positionOverviewEntry(entryIndex)
	step(model, key("p"))
	if model.prompt != nil || !strings.Contains(model.status, shortID(pins[0].ID)) {
		t.Fatalf("duplicate pin was not refused with its existing ID: %q", model.status)
	}
	if after := currentPins(t, selection.Project); len(after) != 1 {
		t.Fatalf("duplicate attempt created another pin: %d", len(after))
	}
}

func TestSnapshotUsePromptRetainsBaseAndHistoryReason(t *testing.T) {
	s, selection := completePromptFixture(t)
	receipt := comparisonReceipt(t, s, selection)
	pin, err := review.Create(s, receipt.ID, "finite expectation", evidence.FiniteExample, "initial pin")
	if err != nil {
		t.Fatal(err)
	}
	selection.Evidence = []evidence.Digest{pin.ID}
	if err := os.WriteFile(filepath.Join(selection.Project, "app", "config.go"), []byte("package main\nconst retentionSeconds = 60\n"), 0600); err != nil {
		t.Fatal(err)
	}
	model, actions := promptModel(t, s, selection)
	pending, err := actions.Capture(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	model.pending = &pending
	count, err := actions.ChangedPathCount(selection.Pair.Candidate, pending.Candidate)
	if err != nil || count < 1 {
		t.Fatalf("changed paths not counted: %d %v", count, err)
	}
	step(model, key("u"))
	if model.prompt == nil {
		t.Fatal("u did not open confirmation", model.status)
	}
	for _, want := range []string{string(selection.Pair.Base), string(pending.Candidate), fmt.Sprintf("%d paths differ", count), "original base", "Pins may reopen", "history"} {
		if !strings.Contains(model.View(), want) {
			t.Fatalf("snapshot prompt missing %q:\n%s", want, model.View())
		}
	}
	step(model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.selected.Pair != selection.Pair || model.pending == nil {
		t.Fatal("cancelled snapshot prompt changed the selected pair")
	}
	step(model, key("u"))
	if model.prompt == nil {
		t.Fatal("second u did not reopen confirmation", model.status)
	}
	step(model, tea.KeyMsg{Type: tea.KeyCtrlU})
	step(model, key("reviewed candidate"))
	step(model, tea.KeyMsg{Type: tea.KeyEnter})
	drain(model, nil)
	if model.selected.Pair.Base != selection.Pair.Base || model.selected.Pair.Candidate != pending.Candidate || model.pending != nil {
		t.Fatalf("confirmed snapshot did not retain original base: %+v", model.selected.Pair)
	}
	if len(model.selected.Evidence) != 1 {
		t.Fatalf("pin selection lost: %+v", model.selected.Evidence)
	}
	updated, err := store.Get[evidence.Pin](actionsStore(t, actions), model.selected.Evidence[0])
	if err != nil || updated.History[len(updated.History)-1].Reason != "reviewed candidate" {
		t.Fatalf("snapshot reason did not reach pin history: %+v %v", updated, err)
	}
}

func TestAcceptPinPromptEngineAndCancellation(t *testing.T) {
	s, selection := completePromptFixture(t)
	receipt := comparisonReceipt(t, s, selection)
	pin, err := review.Create(s, receipt.ID, "finite expectation", evidence.FiniteExample, "initial pin")
	if err != nil {
		t.Fatal(err)
	}
	selection.Evidence = []evidence.Digest{pin.ID}
	model, actions := promptModel(t, s, selection)
	model.width = 120
	acceptBinding, _ := keyBindingFor("a")
	if reason := model.keyReason(acceptBinding, false); reason != "" {
		t.Fatalf("current pin acceptance disabled: %s; selected=%+v pair=%+v", reason, model.selectedOverviewEntry(), model.selected.Pair)
	}
	if !strings.Contains(model.keyHints(), "a accept pin") {
		t.Fatal("current complete pin did not appear in available hints", model.View())
	}
	step(model, key("a"))
	if model.prompt == nil || !strings.Contains(model.View(), string(pin.ID)) || !strings.Contains(model.View(), string(receipt.ID)) {
		t.Fatal("accept prompt did not name pin and current receipt", model.View())
	}
	step(model, tea.KeyMsg{Type: tea.KeyEsc})
	unchanged := currentPins(t, selection.Project)
	if len(unchanged) != 1 || unchanged[0].ID != pin.ID || unchanged[0].Decision != evidence.Pinned {
		t.Fatal("cancelled accept changed pin history", unchanged)
	}
	model.positionOverviewEntry(0)
	step(model, key("a"))
	step(model, tea.KeyMsg{Type: tea.KeyCtrlU})
	step(model, key("reviewed current result"))
	step(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.prompt != nil {
		t.Fatal("confirmed acceptance left prompt open")
	}
	accepted := false
	for _, entry := range model.data.Entries {
		if entry.Decision == evidence.Accepted {
			accepted = badgeFor(entry).word == "ACCEPTED" && entry.CurrentReceipt == receipt.ID
		}
	}
	if !accepted {
		t.Fatalf("confirmed current pin did not render accepted: %s", model.View())
	}
	pinID := model.selected.Evidence[0]
	updated, err := store.Get[evidence.Pin](actionsStore(t, actions), pinID)
	if err != nil || updated.Decision != evidence.Accepted || updated.History[len(updated.History)-1].Reason != "reviewed current result" {
		t.Fatalf("engine acceptance/reason missing: %+v %v", updated, err)
	}
}

func TestAcceptHintUnavailableWithoutCurrentPin(t *testing.T) {
	s, selection := completePromptFixture(t)
	comparisonIDs := []evidence.Digest{}
	comparisonID := viewComparison(t, s, selection, false, evidence.Complete)
	comparisonIDs = append(comparisonIDs, comparisonID)
	selection.Evidence = comparisonIDs
	model, _ := promptModel(t, s, selection)
	if strings.Contains(model.keyHints(), "a accept pin") {
		t.Fatal("accept hint appeared without a pin")
	}
	step(model, key("a"))
	if model.prompt != nil || !strings.Contains(model.status, "select a pin on Overview") {
		t.Fatalf("unavailable acceptance did not explain itself: %q", model.status)
	}
}

func TestPromptReasonBoundsAndEmptyRefusal(t *testing.T) {
	reason := appendPromptReason("", strings.Repeat("x", maxPromptReasonBytes+100))
	if len(reason) != maxPromptReasonBytes {
		t.Fatalf("reason exceeded byte bound: %d", len(reason))
	}
	if got := appendPromptReason(strings.Repeat("x", maxPromptReasonBytes-1), "é"); len(got) != maxPromptReasonBytes-1 {
		t.Fatalf("partial UTF-8 rune crossed byte bound: %d", len(got))
	}

	s, selection := completePromptFixture(t)
	receipt := comparisonReceipt(t, s, selection)
	comparisonIDs, err := s.List("comparison")
	if err != nil || len(comparisonIDs) == 0 {
		t.Fatalf("comparison missing: %v", err)
	}
	selection.Evidence = []evidence.Digest{comparisonIDs[len(comparisonIDs)-1].ID}
	model, _ := promptModel(t, s, selection)
	entryIndex := -1
	for i, entry := range model.data.Entries {
		if entry.Receipt == receipt.ID && entry.Expectation != "" {
			entryIndex = i
			break
		}
	}
	model.positionOverviewEntry(entryIndex)
	step(model, key("p"))
	step(model, tea.KeyMsg{Type: tea.KeyCtrlU})
	step(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.prompt == nil || model.status != "Reason cannot be empty" || len(currentPins(t, selection.Project)) != 0 {
		t.Fatal("empty reason was accepted or prompt dismissed", model.status)
	}
	step(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("x", maxPromptReasonBytes+100)), Paste: true})
	if model.prompt == nil || len(model.prompt.reason) != maxPromptReasonBytes || !strings.Contains(model.View(), "4096/4096 bytes") {
		t.Fatal("prompt did not enforce the 4,096-byte reason bound")
	}
	step(model, tea.KeyMsg{Type: tea.KeyEnter})
	pins := currentPins(t, selection.Project)
	if len(pins) != 1 || len(pins[0].History[len(pins[0].History)-1].Reason) != maxPromptReasonBytes {
		t.Fatalf("bounded reason was not recorded exactly: %+v", pins)
	}
}

func readStore(t *testing.T, project string) *store.Store {
	t.Helper()
	opened, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	return opened
}

func currentPins(t *testing.T, project string) []evidence.Pin {
	t.Helper()
	s := readStore(t, project)
	defer s.Close()
	pins, err := review.Heads(s)
	if err != nil {
		t.Fatal(err)
	}
	return pins
}

func actionsStore(t *testing.T, actions *Actions) *store.Store {
	t.Helper()
	s, err := actions.Store()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

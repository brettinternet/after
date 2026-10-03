package browser

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/brettinternet/after/internal/evidence"
	tea "github.com/charmbracelet/bubbletea"
)

type loopState struct {
	pending                        *evidence.SnapshotPair
	preview                        []byte
	digest                         string
	planPair                       evidence.SnapshotPair
	generation                     uint64
	actionBusy, capturing, running bool
	runCancel                      context.CancelFunc
	results                        []evidence.Digest
}
type snapshotReady struct {
	pair evidence.SnapshotPair
	err  error
}
type prepared struct {
	generation uint64
	pair       evidence.SnapshotPair
	raw        []byte
	digest     string
	err        error
}
type changed struct {
	selected Selection
	data     *Data
	status   string
	err      error
}
type ran struct {
	pair       evidence.SnapshotPair
	comparison evidence.Digest
	err        error
}

func (m *Model) change(work func() (Selection, error), status string) tea.Cmd {
	m.actionBusy = true
	return m.spawn(func() tea.Msg {
		sel, err := work()
		var d *Data
		if err == nil {
			d, err = Load(m.ctx, sel)
		}
		return changed{sel, d, status, err}
	})
}
func (m *Model) invalidatePlan() { m.generation++; m.preview = nil; m.digest = "" }
func (m *Model) updateLoop(msg tea.Msg) (tea.Cmd, bool) {
	a := m.jobs.Actions
	switch msg := msg.(type) {
	case snapshotReady:
		m.capturing = false
		if msg.err != nil {
			m.status = "Capture failed: " + msg.err.Error()
		} else {
			m.pending = &msg.pair
			m.status = "Job finished; stored result retained; a accepts new snapshot"
		}
		return nil, true
	case prepared:
		m.actionBusy = false
		if msg.generation != m.generation || msg.pair != m.selected.Pair {
			return nil, true
		}
		if msg.err != nil {
			m.status = "Preview unavailable: " + msg.err.Error()
			return nil, true
		}
		m.preview, m.digest, m.planPair = msg.raw, msg.digest, msg.pair
		m.screen = "plan"
		m.section = 0
		m.offset = 0
		m.status = "No execution yet. Read all byte pages; y approves this exact plan once; n denies"
		return m.loadPage(), true
	case changed:
		m.actionBusy = false
		if msg.err != nil {
			m.status = "Action failed: " + msg.err.Error()
			return nil, true
		}
		m.selected, m.data = msg.selected, msg.data
		m.loadID++
		m.request++
		m.index = min(m.index, max(0, len(m.data.Entries)-1))
		m.inventory = min(m.inventory, max(0, len(m.data.Inventory)-1))
		m.status = msg.status + "; s session IDs for restart"
		if m.screen == "inspector" || m.screen == "patch" {
			m.section = 0
			m.offset = 0
			return m.loadPage(), true
		}
		if m.screen == "plan" {
			m.screen = "examples"
		}
		return nil, true
	case ran:
		m.running = false
		m.runCancel = nil
		if msg.comparison != "" {
			m.results = append(m.results, msg.comparison)
		}
		if msg.err != nil {
			m.status = "Run failed/cancelled; retained result in s session IDs: " + msg.err.Error()
		}
		if msg.comparison == "" {
			return nil, true
		}
		if msg.pair != m.selected.Pair || m.actionBusy {
			m.status = "Late result retained for originating snapshot; selection unchanged; s result IDs"
			return nil, true
		}
		sel := m.selected
		status := "Measured result attached; expectation remains a separate human decision"
		if msg.err != nil {
			status = "Run failed/cancelled; incomplete result retained, not equality"
		}
		return m.change(func() (Selection, error) { return a.Attach(sel, msg.comparison) }, status), true
	case tea.KeyMsg:
		if msg.Paste || a == nil {
			return nil, false
		}
		switch msg.String() {
		case "x":
			if m.runCancel != nil {
				m.runCancel()
				m.status = "Cancellation requested; waiting for owned run cleanup"
			}
			return nil, false
		case "esc", "n":
			if m.screen == "plan" {
				m.invalidatePlan()
				m.request++
				m.screen = "examples"
				m.status = "Execution denied; no project execution"
				return nil, true
			}
		case "s":
			if m.data == nil {
				return nil, true
			}
			// Full IDs are data pages, not clipped-only notifications. Immutable pin
			// revisions are the restart contract, shared with the headless CLI.
			m.data.Entries = append(m.data.Entries, Entry{Label: "not checked | session references only", Name: "session IDs for restart", Sections: []Section{document("selected pair, evidence/pin revisions, retained run comparisons", struct {
				Selection Selection
				Results   []evidence.Digest
			}{m.selected, m.results})}})
			m.index = len(m.data.Entries) - 1
			m.returnTo = "examples"
			m.screen = "inspector"
			m.section = 0
			m.offset = 0
			return m.loadPage(), true
		case "c":
			if m.capturing || m.data == nil {
				return nil, true
			}
			m.capturing = true
			m.status = "Capture running; selected pair unchanged"
			return m.spawn(func() tea.Msg { pair, err := a.Capture(m.ctx); return snapshotReady{pair, err} }), true
		case "p":
			if m.actionBusy || m.data == nil || m.screen != "examples" || len(m.data.Entries) == 0 {
				return nil, true
			}
			entry, sel := m.data.Entries[m.index], m.selected
			return m.change(func() (Selection, error) {
				if len(sel.Evidence) >= MaxEvidence {
					return sel, errors.New("evidence limit reached")
				}
				id, err := a.Pin(entry, sel.Pair)
				if err != nil {
					return sel, err
				}
				sel.Evidence = append(append([]evidence.Digest(nil), sel.Evidence...), id)
				return sel, nil
			}, "Pinned selected finite provider-request expectation"), true
		case "a":
			if m.pending == nil || m.actionBusy || m.data == nil {
				return nil, true
			}
			sel := m.selected
			// Capture's new HEAD must not silently replace the original review base.
			pair := evidence.SnapshotPair{Base: sel.Pair.Base, Candidate: m.pending.Candidate}
			m.pending = nil
			m.invalidatePlan()
			return m.change(func() (Selection, error) { return a.Select(sel, pair) }, "Snapshot accepted; prior evidence is history, not a prediction"), true
		case "r":
			if m.actionBusy || m.running || m.data == nil {
				return nil, true
			}
			m.invalidatePlan()
			gen, pair := m.generation, m.selected.Pair
			m.actionBusy = true
			m.status = "Preparing exact offline plan; no execution"
			return m.spawn(func() tea.Msg { raw, digest, err := a.Prepare(pair); return prepared{gen, pair, raw, digest, err} }), true
		case "y":
			if m.screen != "plan" || len(m.preview) == 0 || m.digest == "" || m.planPair != m.selected.Pair || m.running || m.actionBusy {
				return nil, true
			}
			raw, digest, pair := append([]byte(nil), m.preview...), m.digest, m.planPair
			m.invalidatePlan()
			m.screen = "examples"
			m.request++
			ctx, cancel := context.WithCancel(m.ctx)
			m.runCancel = cancel
			m.running = true
			m.status = "Authorized run active; x cancel; navigation and capture remain available"
			return m.spawn(func() tea.Msg { defer cancel(); id, err := a.Run(ctx, pair, raw, digest); return ran{pair, id, err} }), true
		}
	}
	return nil, false
}

// SessionJSON is printed after terminal restoration so restart references are
// usable without copying wrapped/quoted terminal pages. It contains no source.
func (m *Model) SessionJSON() []byte {
	raw, _ := json.Marshal(struct {
		Selection Selection
		Results   []evidence.Digest
	}{m.selected, m.results})
	return raw
}

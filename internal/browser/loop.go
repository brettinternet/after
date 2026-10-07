package browser

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	tea "github.com/charmbracelet/bubbletea"
)

type loopState struct {
	pending                        *evidence.SnapshotPair
	preview                        []byte
	digest                         string
	summary                        []byte
	summaryUnavailable             bool
	planPair                       evidence.SnapshotPair
	generation                     uint64
	actionBusy, capturing, running bool
	runCancel                      context.CancelFunc
	runStarted                     time.Time
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
	summary    []byte
	summaryErr error
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
type runClockTick struct{}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func nextRunClockTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return runClockTick{} })
}

func (m *Model) activeClock() bool { return m.capturing || m.busy || m.running }

func (m *Model) scheduleClock() tea.Cmd {
	if m.clockScheduled || !m.activeClock() {
		return nil
	}
	m.clockScheduled = true
	return nextRunClockTick()
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
func (m *Model) invalidatePlan() {
	m.generation++
	m.preview = nil
	m.digest = ""
	m.summary = nil
	m.summaryUnavailable = false
}
func (m *Model) updateLoop(msg tea.Msg) (tea.Cmd, bool) {
	a := m.jobs.Actions
	switch msg := msg.(type) {
	case runClockTick:
		m.clockScheduled = false
		return m.scheduleClock(), true
	case snapshotReady:
		m.capturing = false
		if m.captureCancel != nil {
			m.captureCancel()
			m.captureCancel = nil
		}
		m.captureStarted = time.Time{}
		if msg.err != nil {
			m.status = "Capture failed: " + msg.err.Error()
			kind, summary := "capture failed", "capture failed; the selected pair is unchanged"
			if errors.Is(msg.err, context.Canceled) {
				kind, summary = "capture cancelled", "capture cancelled; the selected pair is unchanged"
			}
			m.recordActivity(kind, summary, []evidence.Digest{m.selected.Pair.Candidate}, msg.err.Error())
		} else if msg.pair == m.selected.Pair {
			m.pending = nil
			m.status = "No new capture; the selected pair is unchanged"
			m.recordActivity("capture finished", "no change; the selected pair is unchanged", []evidence.Digest{msg.pair.Base, msg.pair.Candidate}, "")
		} else {
			m.pending = &msg.pair
			m.status = "New capture " + shortID(msg.pair.Candidate) + " is ready; u reviews it"
			m.recordActivity("capture finished", "new candidate ready; u reviews it", []evidence.Digest{msg.pair.Base, msg.pair.Candidate}, "")
		}
		return nil, true
	case prepared:
		m.actionBusy = false
		if msg.generation != m.generation || msg.pair != m.selected.Pair {
			return nil, true
		}
		if msg.err != nil {
			m.status = "Preview unavailable: " + msg.err.Error()
			m.recordActivity("plan preparation failed", "run preview could not be prepared", []evidence.Digest{m.selected.Pair.Base, m.selected.Pair.Candidate}, msg.err.Error())
			return nil, true
		}
		m.preview = append([]byte(nil), msg.raw...)
		m.digest, m.planPair = msg.digest, msg.pair
		m.summary = append([]byte(nil), msg.summary...)
		m.summaryUnavailable = msg.summaryErr != nil
		m.screen = "plan"
		m.section = 0
		m.status = "Nothing has run. y runs this exact plan once · n denies"
		m.recordActivity("plan prepared", "exact run preview prepared; nothing has run", []evidence.Digest{msg.pair.Base, msg.pair.Candidate, evidence.Digest(msg.digest)}, "")
		return m.loadDocument(), true
	case changed:
		m.actionBusy = false
		if msg.err != nil {
			m.status = "Action failed: " + msg.err.Error()
			m.recordActivity("action failed", msg.status, []evidence.Digest{m.selected.Pair.Base, m.selected.Pair.Candidate}, msg.err.Error())
			return nil, true
		}
		previousSelection := m.selected
		previousPair := m.selected.Pair
		persistFailed := false
		m.selected = msg.selected
		if msg.data != nil && msg.selected.Discover {
			m.selected = msg.data.Selection
		}
		m.data = msg.data
		if previousPair != m.selected.Pair {
			m.session.Pair = m.selected.Pair
			if m.persistSession != nil {
				if err := m.persistSession(m.ReviewSession()); err != nil {
					persistFailed = true
				}
			}
		}
		m.loadID++
		m.request++
		m.index = min(m.index, max(0, len(m.data.Entries)-1))
		m.inventory = min(m.inventory, max(0, len(m.data.Inventory)-1))
		m.status = msg.status + " · s Activity"
		kind, summary := "selection changed", msg.status
		if strings.HasPrefix(msg.status, "Pinned") {
			kind, summary = "pin created", msg.status
		} else if strings.Contains(msg.status, "result attached") {
			kind, summary = "result attached", msg.status
		}
		ids := []evidence.Digest{m.selected.Pair.Base, m.selected.Pair.Candidate}
		for _, id := range m.selected.Evidence {
			if !containsDigest(previousSelection.Evidence, id) {
				ids = append(ids, id)
			}
		}
		m.recordActivity(kind, summary, ids, "")
		if persistFailed {
			m.status = "Review selection changed but its private session could not be saved"
		}
		if m.screen == "inspector" || m.screen == "patch" {
			m.section = 0
			return m.loadDocument(), true
		}
		if m.screen == "plan" {
			m.screen = "examples"
		}
		return nil, true
	case ran:
		m.running = false
		m.runCancel = nil
		m.runStarted = time.Time{}
		ids := []evidence.Digest{msg.pair.Base, msg.pair.Candidate}
		if msg.comparison != "" {
			m.results = append(m.results, msg.comparison)
			ids = append(ids, msg.comparison)
		}
		kind, summary := "run finished", "run completed"
		if msg.err != nil {
			kind, summary = "run failed", "run did not complete"
			if errors.Is(msg.err, context.Canceled) {
				kind, summary = "run cancelled", "incomplete result retained, not equality"
			}
			m.status = "Run failed/cancelled; retained result · s Activity"
		}
		m.recordActivity(kind, summary, ids, errorText(msg.err))
		if msg.comparison == "" {
			return nil, true
		}
		if msg.pair != m.selected.Pair || m.actionBusy || a == nil {
			m.status = "Late result retained for originating snapshot; selection unchanged · s Activity"
			return nil, true
		}
		sel := m.selected
		status := "Measured result attached; expectation remains a separate human decision"
		if msg.err != nil {
			status = "Run failed/cancelled; incomplete result retained, not equality"
		}
		return m.change(func() (Selection, error) { return a.Attach(sel, msg.comparison) }, status), true
	}
	return nil, false
}

func (m *Model) dispatchLoop(action keyAction) (tea.Cmd, bool) {
	a := m.jobs.Actions
	switch action {
	case keyCapture:
		if a == nil {
			return nil, false
		}
		m.capturing = true
		m.captureStarted = m.now()
		m.status = "Capture running; selected pair unchanged"
		m.recordActivity("capture started", "capturing the working tree", []evidence.Digest{m.selected.Pair.Base, m.selected.Pair.Candidate}, "")
		ctx, cancel := context.WithCancel(m.ctx)
		m.captureCancel = cancel
		cmd := m.spawn(func() tea.Msg { pair, err := a.Capture(ctx); return snapshotReady{pair, err} })
		if tick := m.scheduleClock(); tick != nil {
			return tea.Batch(cmd, tick), true
		}
		return cmd, true
	case keyUseCapture:
		if m.pending == nil || m.actionBusy || m.data == nil || a == nil {
			return nil, true
		}
		sel := m.selected
		pair := evidence.SnapshotPair{Base: sel.Pair.Base, Candidate: m.pending.Candidate}
		m.pending = nil
		m.invalidatePlan()
		return m.change(func() (Selection, error) { return a.Select(sel, pair) }, "Snapshot selected; prior evidence remains history"), true
	case keyPin:
		if a == nil || m.data == nil || m.actionBusy || m.busy || m.running || len(m.data.Entries) == 0 {
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
	case keyPreview:
		if a == nil || m.actionBusy || m.busy || m.running || m.data == nil {
			return nil, true
		}
		m.invalidatePlan()
		gen, pair := m.generation, m.selected.Pair
		m.actionBusy = true
		m.status = "Preparing exact offline plan; no execution"
		return m.spawn(func() tea.Msg {
			raw, digest, err := a.Prepare(pair)
			if err != nil {
				return prepared{generation: gen, pair: pair, err: err}
			}
			summary, summaryErr := consentSummary(raw)
			return prepared{generation: gen, pair: pair, raw: raw, digest: digest, summary: summary, summaryErr: summaryErr}
		}), true
	case keyApprove:
		if a == nil || m.screen != "plan" || len(m.preview) == 0 || m.digest == "" || m.planPair != m.selected.Pair || m.running || m.busy || m.actionBusy {
			return nil, true
		}
		raw, digest, pair := append([]byte(nil), m.preview...), m.digest, m.planPair
		m.invalidatePlan()
		m.screen = "examples"
		m.request++
		ctx, cancel := context.WithCancel(m.ctx)
		m.runCancel = cancel
		m.runStarted = m.now()
		m.running = true
		m.recordActivity("run approved", "exact approved plan started", []evidence.Digest{pair.Base, pair.Candidate, evidence.Digest(digest)}, "")
		m.status = "Authorized run active; x cancels; navigation remains available"
		cmd := m.spawn(func() tea.Msg { defer cancel(); id, err := a.Run(ctx, pair, raw, digest); return ran{pair, id, err} })
		if tick := m.scheduleClock(); tick != nil {
			return tea.Batch(cmd, tick), true
		}
		return cmd, true
	case keyDeny, keyBack:
		if m.screen != "plan" {
			return nil, false
		}
		m.invalidatePlan()
		m.request++
		m.screen = "examples"
		m.status = "Execution denied; no project execution"
		m.recordActivity("plan denied", "prepared plan denied; no project execution", []evidence.Digest{m.planPair.Base, m.planPair.Candidate}, "")
		return nil, true
	case keySession:
		if m.data == nil {
			return nil, true
		}
		detail := activitySessionDetail(m.selected, m.results, m.persistSession != nil)
		if detail != m.sessionActivityDetail {
			m.recordActivity("session opened", "opened Activity; selected session references", []evidence.Digest{m.selected.Pair.Base, m.selected.Pair.Candidate}, detail)
			m.sessionActivityDetail = detail
		}
		m.request++
		m.screen = "activity"
		m.doc = nil
		m.top = 0
		m.helpFrom = ""
		return nil, true
	case keyCancel:
		if m.jobCancel != nil {
			m.jobCancel()
		}
		if m.captureCancel != nil {
			m.captureCancel()
		}
		if m.runCancel != nil {
			m.runCancel()
		}
		m.status = "Cancellation requested; waiting for owned job cleanup"
		return nil, true
	}
	return nil, false
}

// SessionJSON is printed after terminal restoration so restart references are
// usable without copying wrapped/quoted terminal pages. It contains no source.
func (m *Model) SessionJSON() []byte {
	raw, _ := json.Marshal(m.ReviewSession())
	return raw
}

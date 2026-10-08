package browser

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
)

const maxPromptReasonBytes = 4096

type mutationPrompt struct {
	action      keyAction
	returnTo    string
	title       string
	description []string
	reason      string
	selection   Selection
	pair        evidence.SnapshotPair
	entry       Entry
	changed     int
}

type usePromptReady struct {
	selection Selection
	pair      evidence.SnapshotPair
	changed   int
	err       error
}

func (m *Model) openMutationPrompt(prompt mutationPrompt) {
	m.clearSearch()
	prompt.returnTo = m.screen
	m.prompt = &prompt
	m.screen = "prompt"
	m.status = ""
}

func (m *Model) startUseCapturePrompt() tea.Cmd {
	if m.jobs.Actions == nil || m.pending == nil {
		return nil
	}
	selection := m.selected
	selection.Evidence = append([]evidence.Digest(nil), m.selected.Evidence...)
	pair := evidence.SnapshotPair{Base: selection.Pair.Base, Candidate: m.pending.Candidate}
	m.actionBusy = true
	m.status = "Counting paths that differ from the candidate under review"
	return m.spawn(func() tea.Msg {
		count, err := m.jobs.Actions.ChangedPathCount(selection.Pair.Candidate, pair.Candidate)
		return usePromptReady{selection: selection, pair: pair, changed: count, err: err}
	})
}

func (m *Model) prepareUsePrompt(msg usePromptReady) {
	m.actionBusy = false
	if msg.selection.Pair != m.selected.Pair || m.pending == nil || m.pending.Candidate != msg.pair.Candidate {
		m.status = "Capture changed while preparing confirmation; selection is unchanged"
		return
	}
	if msg.err != nil {
		m.status = "Cannot count changed paths: " + msg.err.Error()
		return
	}
	m.openMutationPrompt(mutationPrompt{
		action:    keyUseCapture,
		selection: msg.selection,
		pair:      msg.pair,
		changed:   msg.changed,
		title:     "Use this captured candidate?",
		description: []string{
			"Use the new candidate; the original base is kept.",
			"Exact target IDs:",
			"original base " + string(msg.pair.Base),
			"new candidate " + string(msg.pair.Candidate),
			fmt.Sprintf("%d paths differ from candidate %s under review.", msg.changed, msg.selection.Pair.Candidate),
			"Pins may reopen; earlier results become history.",
		},
		reason: "Use this captured snapshot; keep the original base and retain earlier results as history",
	})
}

func (m *Model) startPinPrompt(entry Entry) {
	m.openMutationPrompt(mutationPrompt{
		action: keyPin,
		entry:  entry,
		title:  "Pin this finite expectation?",
		description: []string{
			"Pin this measured provider-request count; this is not whole-change approval.",
			"Exact target IDs:",
			"receipt " + string(entry.Receipt),
			"base " + string(m.selected.Pair.Base),
			"candidate " + string(m.selected.Pair.Candidate),
			"expectation " + entry.Expectation,
		},
		reason: "Preserve this finite provider-request count; not whole-change approval",
	})
}

func (m *Model) startAcceptPrompt(entry Entry) {
	m.openMutationPrompt(mutationPrompt{
		action: keyAcceptPin,
		entry:  entry,
		title:  "Accept this pin's current result?",
		description: []string{
			"Accept only this pin against its current complete result; not the whole change.",
			"Exact target IDs:",
			"pin " + string(entry.PinID),
			"receipt " + string(entry.CurrentReceipt),
			"base " + string(m.selected.Pair.Base),
			"candidate " + string(m.selected.Pair.Candidate),
		},
		reason: "Reviewed the current complete result for this selected pair",
	})
}

func sanitizeReasonRune(r rune) string {
	if r == '\t' {
		return `\u0009`
	}
	return terminal.Sanitize(string(r))
}

func appendPromptReason(reason, input string) string {
	for _, r := range input {
		if !utf8.ValidRune(r) {
			continue
		}
		safe := sanitizeReasonRune(r)
		if len(reason)+len(safe) > maxPromptReasonBytes {
			break
		}
		reason += safe
	}
	return reason
}

func removeLastReasonRune(reason string) string {
	if reason == "" {
		return ""
	}
	_, size := utf8.DecodeLastRuneInString(reason)
	return reason[:len(reason)-size]
}

func (m *Model) updatePromptKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	prompt := m.prompt
	if prompt == nil {
		return m, nil
	}
	m.status = ""
	if key.Paste {
		prompt.reason = appendPromptReason(prompt.reason, string(key.Runes))
		return m, nil
	}
	switch key.String() {
	case "esc":
		m.screen = prompt.returnTo
		m.prompt = nil
		m.status = "Cancelled; no changes"
		return m, nil
	case "enter":
		if strings.TrimSpace(prompt.reason) == "" {
			m.status = "Reason cannot be empty"
			return m, nil
		}
		m.prompt = nil
		m.screen = prompt.returnTo
		return m, m.confirmMutation(*prompt)
	case "backspace", "delete":
		prompt.reason = removeLastReasonRune(prompt.reason)
		return m, nil
	case "ctrl+u":
		prompt.reason = ""
		return m, nil
	case "ctrl+c":
		m.prompt = nil
		m.screen = prompt.returnTo
		m.quitNow()
		return m, tea.Quit
	}
	if len(key.Runes) > 0 {
		prompt.reason = appendPromptReason(prompt.reason, string(key.Runes))
	}
	return m, nil
}

func (m *Model) confirmMutation(prompt mutationPrompt) tea.Cmd {
	a := m.jobs.Actions
	if a == nil {
		m.status = "Mutation is unavailable in this review"
		return nil
	}
	reason := prompt.reason
	switch prompt.action {
	case keyPin:
		selection := m.selected
		selection.Evidence = append([]evidence.Digest(nil), m.selected.Evidence...)
		return m.change(func() (Selection, error) {
			if len(selection.Evidence) >= MaxEvidence {
				return selection, errors.New("evidence limit reached")
			}
			id, err := a.Pin(prompt.entry, selection.Pair, reason)
			if err != nil {
				return selection, err
			}
			selection.Evidence = append(selection.Evidence, id)
			return selection, nil
		}, "Pinned selected finite provider-request expectation")
	case keyUseCapture:
		selection := prompt.selection
		selection.Evidence = append([]evidence.Digest(nil), prompt.selection.Evidence...)
		m.invalidatePlan()
		return m.changeWith(func() (Selection, error) {
			return a.Select(selection, prompt.pair, reason)
		}, "Snapshot selected; prior evidence remains history", true)
	case keyAcceptPin:
		selection := m.selected
		selection.Evidence = append([]evidence.Digest(nil), m.selected.Evidence...)
		return m.change(func() (Selection, error) {
			index := -1
			for i, id := range selection.Evidence {
				if id == prompt.entry.PinID {
					index = i
					break
				}
			}
			if index < 0 {
				if len(selection.Evidence) >= MaxEvidence {
					return selection, errors.New("evidence limit reached")
				}
				selection.Evidence = append(selection.Evidence, prompt.entry.PinID)
				index = len(selection.Evidence) - 1
			}
			id, err := a.AcceptPin(prompt.entry.PinID, reason)
			if err != nil {
				return selection, err
			}
			selection.Evidence[index] = id
			return selection, nil
		}, "Pin accepted for this current complete result")
	default:
		m.status = "Unknown confirmation action"
		return nil
	}
}

func (m *Model) promptLines() []string {
	if m.prompt == nil {
		return nil
	}
	lines := []string{}
	for _, paragraph := range m.prompt.description {
		lines = append(lines, terminal.Wrap(paragraph, m.width)...)
	}
	lines = append(lines, "")
	lines = append(lines, terminal.Wrap("Reason · stored in pin history", m.width)...)
	fieldWidth := max(m.width-8, 1)
	safe := terminal.Sanitize(m.prompt.reason)
	left := max(uniseg.StringWidth(safe)-fieldWidth, 0)
	lines = append(lines, terminal.Line("Reason  "+terminal.LineAt(safe, left, fieldWidth), m.width))
	lines = append(lines, fmt.Sprintf("%d/%d bytes", len(m.prompt.reason), maxPromptReasonBytes))
	return lines
}

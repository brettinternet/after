package browser

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
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
	baseline    evidence.Digest
	mode        evidence.ReviewMode
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
	baseline := selection.Baseline
	if baseline == "" {
		baseline = selection.Pair.Base
	}
	selection.Baseline = baseline
	pair := evidence.SnapshotPair{Base: baseline, Candidate: m.pending.Candidate}
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
		baseline:  msg.selection.Baseline,
		mode:      evidence.OriginalBase,
		changed:   msg.changed,
		title:     "Use this captured candidate?",
		description: []string{
			fmt.Sprintf("%d paths differ from candidate %s under review.", msg.changed, msg.selection.Pair.Candidate),
			"Pins may reopen; earlier results become history.",
			"Choose what before means, then confirm with a non-empty reason.",
		},
		reason: "Use this captured candidate for the selected comparison; earlier results remain history",
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

// removeLastRune and removeLastWord edit single-line text fields
// (Backspace/Ctrl-H and Ctrl-W).
func removeLastRune(text string) string {
	_, size := utf8.DecodeLastRuneInString(text)
	return text[:len(text)-size]
}

func removeLastWord(text string) string {
	text = strings.TrimRightFunc(text, unicode.IsSpace)
	return strings.TrimRightFunc(text, func(r rune) bool { return !unicode.IsSpace(r) })
}

func replacePinRevision(ids []evidence.Digest, old, next evidence.Digest) []evidence.Digest {
	updated := append([]evidence.Digest(nil), ids...)
	for index, id := range updated {
		if id == old {
			updated[index] = next
			return updated
		}
	}
	if !containsDigest(updated, next) && len(updated) < MaxEvidence {
		updated = append(updated, next)
	}
	return updated
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
	case "left", "right":
		if prompt.action == keyUseCapture {
			if key.String() == "left" {
				prompt.mode = evidence.OriginalBase
				prompt.pair.Base = prompt.baseline
			} else {
				prompt.mode = evidence.FollowUp
				prompt.pair.Base = prompt.selection.Pair.Candidate
			}
			return m, nil
		}
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
	case "backspace", "delete", "ctrl+h":
		prompt.reason = removeLastRune(prompt.reason)
		return m, nil
	case "ctrl+w":
		prompt.reason = removeLastWord(prompt.reason)
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
			selection.PinRevisions = append(append([]evidence.Digest(nil), selection.PinRevisions...), id)
			return selection, nil
		}, "Pinned selected finite provider-request expectation")
	case keyUseCapture:
		selection := prompt.selection
		selection.Evidence = append([]evidence.Digest(nil), prompt.selection.Evidence...)
		selection.PinRevisions = append([]evidence.Digest(nil), prompt.selection.PinRevisions...)
		selection.Baseline = prompt.baseline
		selection.Mode = prompt.mode
		m.invalidatePlan()
		status := "Snapshot selected; prior evidence remains history"
		if prompt.mode == evidence.FollowUp {
			status = "Follow-up selected; prior evidence remains history"
		}
		return m.changeWith(func() (Selection, error) {
			return a.Select(selection, prompt.pair, prompt.mode, reason)
		}, status, true)
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
			selection.PinRevisions = replacePinRevision(selection.PinRevisions, prompt.entry.PinID, id)
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
	if m.prompt.action == keyUseCapture {
		lines = append(lines, terminal.Wrap("Before means (←/→ choose):", m.width)...)
		original := fmt.Sprintf("original base %s → candidate %s", m.prompt.baseline, m.prompt.pair.Candidate)
		followUp := fmt.Sprintf("last inspected %s → candidate %s", m.prompt.selection.Pair.Candidate, m.prompt.pair.Candidate)
		if m.prompt.mode == evidence.OriginalBase {
			lines = append(lines, terminal.Wrap("> "+original, m.width)...)
			lines = append(lines, terminal.Wrap("  "+followUp, m.width)...)
		} else {
			lines = append(lines, terminal.Wrap("  "+original, m.width)...)
			lines = append(lines, terminal.Wrap("> "+followUp, m.width)...)
		}
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

package browser

import (
	"strings"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
)

type badge struct {
	word  string
	style terminal.Style
}

// badgeFor is the sole mapping from typed engine fields to row badges.
// Names, summaries, report output and expectation prose are deliberately absent.
func badgeFor(e Entry) badge {
	b := func(word string, style terminal.Style) badge { return badge{word, style} }
	switch {
	case e.Unavailable:
		return b("UNAVAILABLE", terminal.Problem)
	case e.Decision == evidence.Reopened:
		return b("REOPENED", terminal.Attention)
	case e.State.Applicability == evidence.Stale:
		return b("STALE", terminal.Attention)
	case (e.Decision != "" || e.State.Kind == evidence.Observed) && e.State.Applicability != evidence.Current && e.State.Applicability != evidence.Stale:
		return b("UNKNOWN", terminal.Attention)
	}
	if e.Decision != "" {
		if e.Decision == evidence.Accepted {
			return b("ACCEPTED", terminal.Decision)
		}
		return b("PINNED", terminal.Decision)
	}
	if e.State.Kind == evidence.Reported {
		return b("REPORTED", terminal.Reported)
	}
	if e.State.Kind != evidence.Observed {
		return b("NOT CHECKED", terminal.Muted)
	}
	switch {
	case e.State.Execution == evidence.Failed:
		return b("FAILED", terminal.Problem)
	case e.State.Execution == evidence.Cancelled:
		return b("CANCELLED", terminal.Problem)
	case e.Completeness != evidence.Complete || e.State.Comparison == evidence.Incomparable:
		return b("INCOMPLETE", terminal.Problem)
	case e.State.Comparison == evidence.Unstable:
		return b("UNSTABLE", terminal.Attention)
	case e.State.Comparison == evidence.Different:
		return b("DIFFERENT", terminal.Changed)
	case e.State.Comparison == evidence.Equal:
		return b("EQUAL", terminal.Observed)
	default:
		return b("NOT COMPARED", terminal.Muted)
	}
}

func shortID(id evidence.Digest) string {
	text := strings.TrimPrefix(string(id), "sha256:")
	return text[:min(8, len(text))]
}

func (m *Model) trailing(e Entry) string {
	if e.Decision != "" {
		if e.MissingCurrentResult {
			return "no current result"
		}
		if e.Decision == evidence.Accepted {
			at, now := e.DecisionAt.In(m.zone), m.now().In(m.zone)
			format := "Jan 2 15:04"
			if at.Year() == now.Year() && at.YearDay() == now.YearDay() {
				format = "15:04:05"
			}
			return "accepted " + at.Format(format)
		}
		return "current result attached"
	}
	switch e.State.Kind {
	case evidence.Observed:
		if e.State.Applicability == evidence.Stale {
			return "ran on " + shortID(e.Candidate)
		}
		return "observed · " + string(e.State.Applicability)
	case evidence.Reported:
		return "reported · " + string(e.State.Report)
	default:
		return ""
	}
}

// Each column is separately sanitized before styling. No payload can consume
// the badge/selection column, even when it starts with combining marks or tabs.
func (m *Model) entryLine(e Entry, selected bool) string {
	return m.entryLineWidth(e, selected, m.width)
}

func (m *Model) changeEntryLine(e Entry, selected bool, width int) string {
	prefix, style := "  ", terminal.Plain
	if selected {
		prefix, style = "> ", terminal.Strong
	}
	letter := "?"
	switch e.Change {
	case "added":
		letter = "A"
	case "deleted":
		letter = "D"
	case "modified":
		letter = "M"
	}
	if width <= len(prefix) {
		return terminal.Line(prefix, width)
	}
	body := letter + "  " + e.Name
	if e.Summary != "" {
		body += "  " + e.Summary
	}
	return prefix + m.theme.Render(body, width-len(prefix), style, false)
}

func (m *Model) entryLineWidth(e Entry, selected bool, width int) string {
	prefix, nameStyle := "  ", terminal.Plain
	if selected {
		prefix, nameStyle = "> ", terminal.Strong
	}
	b := badgeFor(e)
	if width <= 2 {
		return terminal.Line(prefix, width)
	}
	out := prefix + m.theme.Render("["+b.word+"]", min(14, width-2), b.style, true)
	if width <= 16 {
		return out
	}
	available := width - 16
	tail := m.trailing(e)
	tailWidth := 0
	// Narrow terminals preserve the badge and name; details retain all fields.
	if width >= 80 && tail != "" {
		tailWidth = 23
	}
	bodyWidth := available - tailWidth
	body := e.Name
	if e.Summary != "" {
		body += " · " + e.Summary
	}
	if tailWidth > 0 {
		out += m.theme.Render(body, bodyWidth-1, nameStyle, true) + " "
		style := terminal.Muted
		if e.State.Kind == evidence.Reported && e.State.Report == evidence.ReportFail {
			style = terminal.Problem
		}
		out += m.theme.Render(tail, tailWidth, style, false)
	} else {
		out += m.theme.Render(body, bodyWidth, nameStyle, false)
	}
	return out
}

// Rendering time is injected for deterministic views; engine timestamps remain
// untouched in the stored record sections.
func (m *Model) setClock(now func() time.Time, zone *time.Location) {
	m.now, m.zone = now, zone
}

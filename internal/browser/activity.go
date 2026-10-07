package browser

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
)

const (
	MaxActivityEvents = 64
	maxActivityText   = 16 << 10
)

type ActivityEvent struct {
	At      time.Time         `json:"at"`
	Kind    string            `json:"kind"`
	IDs     []evidence.Digest `json:"ids,omitempty"`
	Summary string            `json:"summary"`
	Detail  string            `json:"detail,omitempty"`
}

func (m *Model) recordActivity(kind, summary string, ids []evidence.Digest, detail string) {
	if m.now == nil {
		m.now = time.Now
	}
	if m.zone == nil {
		m.zone = time.Local
	}
	copied := append([]evidence.Digest(nil), ids...)
	if len(copied) > MaxActivityEvents {
		copied = copied[:MaxActivityEvents]
	}
	event := ActivityEvent{
		At:      m.now().UTC(),
		Kind:    boundedActivityText(kind, 128),
		IDs:     copied,
		Summary: boundedActivityText(summary, 512),
		Detail:  boundedActivityText(detail, maxActivityText),
	}
	m.activity = append(m.activity, event)
	if len(m.activity) > MaxActivityEvents {
		copy(m.activity, m.activity[len(m.activity)-MaxActivityEvents:])
		m.activity = m.activity[:MaxActivityEvents]
		m.activityDropped++
	}
	m.activityIndex = 0
}

func boundedActivityText(text string, limit int) string {
	text = terminal.Sanitize(text)
	if len(text) <= limit {
		return text
	}
	const suffix = "… [truncated]"
	end := limit - len(suffix)
	for end > 0 && !utf8.ValidString(text[:end]) {
		end--
	}
	return text[:end] + suffix
}

func containsDigest(ids []evidence.Digest, target evidence.Digest) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func (m *Model) activityRows() int {
	reserved := len(m.activitySessionLines()) + 1 // session block and ACTIVITY heading
	if m.activityDropped > 0 {
		reserved++
	}
	return max(m.bodyRows()-reserved, 1)
}

func (m *Model) activitySessionLines() []string {
	sessionLine := "explicit IDs leave .after/session.json unchanged"
	if m.persistSession != nil {
		sessionLine = "saved in .after/session.json on pair change or quit · after review resumes it"
	}
	lines := []string{
		"SESSION",
		sessionLine,
		"pair       base " + shortID(m.selected.Pair.Base) + " → candidate " + shortID(m.selected.Pair.Candidate),
	}
	ids := m.selected.Evidence
	if len(ids) == 0 {
		lines = append(lines, "loaded     no evidence IDs")
	} else {
		limit := 5
		if m.width < 60 {
			limit = 2
		} else if m.width < 100 {
			limit = 3
		}
		shown := make([]string, 0, min(len(ids), limit))
		for _, id := range ids[:min(len(ids), limit)] {
			shown = append(shown, shortID(id))
		}
		loaded := "loaded     " + strings.Join(shown, " · ")
		if len(ids) > len(shown) {
			loaded += " · +" + itoa(len(ids)-len(shown))
		}
		lines = append(lines, loaded)
	}
	return lines
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func (m *Model) activityLine(index int, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "> "
	}
	eventIndex := len(m.activity) - 1 - index
	if eventIndex < 0 || eventIndex >= len(m.activity) {
		return terminal.Line(prefix, m.width)
	}
	event := m.activity[eventIndex]
	at := event.At.In(m.zone)
	format := "Jan 2 15:04"
	now := m.now().In(m.zone)
	if at.Year() == now.Year() && at.YearDay() == now.YearDay() {
		format = "15:04:05"
	}
	ids := make([]string, len(event.IDs))
	for i, id := range event.IDs {
		ids[i] = shortID(id)
	}
	line := at.Format(format) + "  " + event.Kind
	if len(ids) > 0 {
		line += "  " + strings.Join(ids, " · ")
	}
	if event.Summary != "" {
		line += " · " + event.Summary
	}
	return prefix + m.theme.Render(line, max(m.width-len(prefix), 0), terminal.Plain, false)
}

func activityDetail(event ActivityEvent) []Section {
	return []Section{document("full IDs and sanitized details", event)}
}

func activitySessionDetail(selection Selection, results []evidence.Digest, saved bool) string {
	savedReview := "explicit IDs leave .after/session.json unchanged"
	if saved {
		savedReview = "pair saved on selection change or quit; after review resumes it"
	}
	raw, _ := json.MarshalIndent(struct {
		SavedReview string            `json:"saved_review"`
		Selection   Selection         `json:"selection"`
		Results     []evidence.Digest `json:"retained_run_comparisons,omitempty"`
	}{SavedReview: savedReview, Selection: selection, Results: append([]evidence.Digest(nil), results...)}, "", "  ")
	return string(raw)
}

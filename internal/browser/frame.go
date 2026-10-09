package browser

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/brettinternet/after/internal/terminal"
	"github.com/rivo/uniseg"
)

// segment is one trusted, separately styled piece of a chrome row. Text is
// still sanitized and clipped by terminal.Theme.Render.
type segment struct {
	text  string
	style terminal.Style
}

// segments renders pieces left to right and stops at width without splitting
// a piece's sanitized graphemes.
func (m *Model) segments(width int, parts ...segment) string {
	var line strings.Builder
	remaining := width
	for _, part := range parts {
		if remaining <= 0 {
			break
		}
		safe := terminal.Line(part.text, remaining)
		used := uniseg.StringWidth(safe)
		line.WriteString(m.theme.Render(safe, used, part.style, false))
		remaining -= used
		if used < uniseg.StringWidth(terminal.Sanitize(part.text)) {
			break
		}
	}
	return line.String()
}

// spread places right after left, separated by at least two spaces, and drops
// right when both do not fit.
func spread(left, right string, width int) string {
	leftWidth := uniseg.StringWidth(terminal.VisibleText(left))
	rightWidth := uniseg.StringWidth(terminal.VisibleText(right))
	if right == "" || leftWidth+2+rightWidth > width {
		return left
	}
	return left + strings.Repeat(" ", width-leftWidth-rightWidth) + right
}

// margin is the blank gutter on each side of the body and chrome.
func (m *Model) margin() int {
	if m.width >= 60 {
		return 2
	}
	return 0
}

// inner is the usable width between the side margins.
func (m *Model) inner() int { return max(m.width-2*m.margin(), 1) }

func (m *Model) isTabScreen() bool {
	return m.screen == "examples" || m.screen == "inventory" || m.screen == "patch" || m.screen == "activity"
}

// hasSectionStrip reports whether a document screen shows its section tabs.
func (m *Model) hasSectionStrip() bool {
	return (m.screen == "inspector" || m.screen == "plan") && m.height >= 12
}

// spacer adds one blank row under the chrome when the terminal has room.
func (m *Model) spacer() bool { return m.secondaryRow() && m.height >= 20 }

func (m *Model) topRows() int {
	_, indicators := m.frameHeader()
	rows := 1 + len(indicators)
	if m.secondaryRow() {
		rows++ // tabs or breadcrumb
		if m.isTabScreen() || m.height >= 12 {
			rows++ // rule
		}
		if m.hasSectionStrip() {
			rows++
		}
		if m.spacer() {
			rows++
		}
	}
	return rows
}

func (m *Model) footerRows() int {
	switch {
	case m.height >= 12:
		return 2 // rule and status
	case m.height >= 2:
		return 1
	default:
		return 0
	}
}

func (m *Model) projectName() string {
	project := filepath.Base(filepath.Clean(m.selected.Project))
	if project == "." || project == string(filepath.Separator) || project == "" {
		project = "project"
	}
	return project
}

func (m *Model) headerSegments() []segment {
	baseID, candidateID := shortID(m.selected.Pair.Base), shortID(m.selected.Pair.Candidate)
	mode := reviewModeLabel(m.selected.Mode)
	brand := segment{" AFTER ", terminal.Brand}
	if m.width < 60 {
		return []segment{brand, {" " + mode + " ", terminal.Muted}, {baseID, terminal.Plain}, {" → ", terminal.Muted}, {candidateID, terminal.Plain}}
	}
	baseSource, candidateSource := "source loading", "source loading"
	if m.data != nil {
		baseSource = snapshotSource(m.data.BaseSnapshot)
		candidateSource = snapshotSource(m.data.CandidateSnapshot)
	}
	return []segment{
		brand, {"  ", terminal.Plain}, {m.projectName(), terminal.Strong}, {"   " + mode + " ", terminal.Muted},
		{baseID, terminal.Plain}, {" (" + baseSource + ")", terminal.Muted}, {"  →  ", terminal.Muted},
		{"candidate ", terminal.Muted}, {candidateID, terminal.Plain}, {" (" + candidateSource + ")", terminal.Muted},
	}
}

func (m *Model) headerLine() string {
	header, _ := m.frameHeader()
	width := m.inner()
	left := m.segments(width, m.headerSegments()...)
	extra := strings.TrimPrefix(header, m.headerText())
	extra = strings.TrimPrefix(extra, " · ")
	if extra == "" {
		return left
	}
	return spread(left, m.theme.Render(extra, width, terminal.Attention, false), width)
}

type tab struct {
	label  string
	active bool
}

func (m *Model) tabs() []tab {
	labels := []string{"Overview", fmt.Sprintf("Changes %d", lenInventory(m.data)), "Diff", "Activity"}
	if m.width < 40 {
		labels = []string{"Ov", fmt.Sprintf("Ch %d", lenInventory(m.data)), "Df", "Ac"}
	}
	active := map[string]int{"examples": 0, "inventory": 1, "patch": 2, "activity": 3}[m.screen]
	count := 1
	if m.data != nil {
		count = len(labels)
	}
	out := make([]tab, count)
	for index := range out {
		out[index] = tab{label: labels[index], active: index == active}
	}
	return out
}

// strip renders tab-like labels and the rule beneath them. The active label is
// bold (bracketed without color) and underlined by a heavy rule segment, so
// the selection survives NO_COLOR.
func (m *Model) strip(tabs []tab, right []string, width int) (string, string) {
	var line, rule strings.Builder
	used := 0
	for index, item := range tabs {
		label := item.label
		if item.active && !m.theme.Color {
			label = "[" + label + "]"
		}
		gap := 0
		if index > 0 {
			gap = 3
		}
		labelWidth := uniseg.StringWidth(terminal.Sanitize(label))
		if used+gap+labelWidth > width {
			break
		}
		line.WriteString(strings.Repeat(" ", gap))
		rule.WriteString(m.theme.Render(strings.Repeat("─", gap), gap, terminal.Rule, false))
		style, underline, underlineStyle := terminal.Muted, "─", terminal.Rule
		if item.active {
			style, underline, underlineStyle = terminal.Strong, "━", terminal.Accent
		}
		line.WriteString(m.theme.Render(label, labelWidth, style, false))
		rule.WriteString(m.theme.Render(strings.Repeat(underline, labelWidth), labelWidth, underlineStyle, false))
		used += gap + labelWidth
	}
	if fill := width - used; fill > 0 {
		rule.WriteString(m.theme.Render(strings.Repeat("─", fill), fill, terminal.Rule, false))
	}
	left := line.String()
	// Drop the least specific leading parts until the rest fits; the last
	// parts (hex, pan column) name the current mode.
	for start := range right {
		text := strings.Join(right[start:], " · ")
		if used+2+uniseg.StringWidth(text) <= width {
			left = spread(left, m.theme.Render(text, width, terminal.Muted, false), width)
			break
		}
	}
	return left, rule.String()
}

func (m *Model) rule(width int) string {
	return m.theme.Render(strings.Repeat("─", width), width, terminal.Rule, false)
}

func (m *Model) sectionTabs() []tab {
	sections := m.sections()
	out := make([]tab, len(sections))
	for index, section := range sections {
		label := section.Name
		if m.screen == "plan" && section.Name == "Exact plan" {
			label += " " + previewSize(len(m.preview))
		}
		out[index] = tab{label: label, active: index == m.section}
	}
	return out
}

// documentMeta is the muted right-hand summary of an open document, most
// general part first.
func (m *Model) documentMeta() []string {
	if m.doc == nil {
		return nil
	}
	parts := []string{previewSize(m.doc.RawLength())}
	if m.screen == "patch" && m.data != nil && m.data.Diff != nil {
		parts = append([]string{m.data.Diff.Origin}, parts...)
	}
	if sections := m.sections(); m.screen != "patch" && len(sections) > 1 {
		parts = append([]string{fmt.Sprintf("%d/%d", m.section+1, len(sections))}, parts...)
	}
	if m.hex {
		parts = append(parts, "hex")
	}
	if m.left > 0 {
		parts = append(parts, fmt.Sprintf("col %d", m.left+1))
	}
	return parts
}

func (m *Model) frameTop() []string {
	width := m.inner()
	pad := strings.Repeat(" ", m.margin())
	_, indicators := m.frameHeader()
	lines := []string{pad + m.headerLine()}
	for _, indicator := range indicators {
		lines = append(lines, pad+m.theme.Render(indicator, width, terminal.Attention, false))
	}
	if !m.secondaryRow() {
		return lines
	}
	if m.isTabScreen() {
		var right []string
		if m.screen == "patch" {
			right = m.documentMeta()
		}
		tabs, rule := m.strip(m.tabs(), right, width)
		lines = append(lines, pad+tabs, pad+rule)
	} else {
		lines = append(lines, pad+m.breadcrumb())
		if m.hasSectionStrip() {
			sections, rule := m.strip(m.sectionTabs(), m.documentMeta(), width)
			lines = append(lines, pad+sections, pad+rule)
		} else if m.height >= 12 {
			lines = append(lines, pad+m.rule(width))
		}
	}
	if m.spacer() {
		lines = append(lines, "")
	}
	return lines
}

// statusStyle chooses emphasis for the trusted status prefix only.
func (m *Model) statusStyle(text string) terminal.Style {
	switch {
	case text == "":
		return terminal.Plain
	case strings.HasPrefix(text, "Can't") || strings.Contains(text, "failed") || strings.HasPrefix(text, "Couldn't") || strings.HasPrefix(text, "Preview unavailable") || strings.HasPrefix(text, "Action failed"):
		return terminal.Problem
	case m.screen == "plan" || m.quitConfirm || m.running:
		return terminal.Attention
	case strings.Contains(text, " — ") || strings.HasPrefix(text, "New capture") || strings.HasPrefix(text, "Pin reopened") || strings.HasPrefix(text, "Current complete"):
		return terminal.Accent
	case strings.HasPrefix(text, "/") || strings.HasPrefix(text, "match ") || text == "no matches":
		return terminal.Strong
	default:
		return terminal.Muted
	}
}

func (m *Model) frameFooter() []string {
	width := m.inner()
	pad := strings.Repeat(" ", m.margin())
	// Shortcuts live in ? help; the footer only names editing keys while a
	// field has focus.
	hint := "? help"
	if m.screen == "prompt" || m.searchEditing || m.height < 7 {
		hint = m.keyHints()
	}
	if m.height < 7 {
		return []string{pad + m.theme.Render(hint, width, terminal.Muted, false)}
	}
	status, style := m.statusLine(), terminal.Muted
	if status == "" {
		status = m.idleHint()
	} else {
		style = m.statusStyle(status)
	}
	hintWidth := uniseg.StringWidth(hint)
	statusWidth := width
	if width >= hintWidth+12 {
		statusWidth = width - hintWidth - 2
	}
	left := m.theme.Render(status, statusWidth, style, false)
	if m.searchEditing {
		left = m.segments(statusWidth, segment{status, terminal.Strong}, segment{"▏", terminal.Accent})
	}
	line := spread(left, m.theme.Render(hint, width, terminal.Muted, false), width)
	if m.footerRows() == 2 {
		return []string{pad + m.rule(width), pad + line}
	}
	return []string{pad + line}
}

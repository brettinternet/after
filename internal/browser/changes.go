package browser

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/terminal"
	"github.com/rivo/uniseg"
)

func (m *Model) overviewChangesLine() string {
	if m.data == nil {
		return ""
	}
	paths := len(m.data.Inventory)
	oracles := 0
	for _, entry := range m.data.Inventory {
		if entry.PotentialOracle {
			oracles++
		}
	}
	return fmt.Sprintf("CHANGES  %d %s · %d captured %s · %d potential %s", paths, plural(paths, "path", "paths"), m.data.ChangeCounts.Total, plural(m.data.ChangeCounts.Total, "hunk", "hunks"), oracles, plural(oracles, "oracle", "oracles"))
}

func (m *Model) overviewChangeDetailLine() string {
	if m.data == nil || m.data.Diff == nil || m.data.Diff.Origin != rawdiff.ComputedOrigin {
		return ""
	}
	line := m.data.Diff.Origin
	if m.data.Diff.SourceLimited {
		line += " · limited inventory"
	}
	return line
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func (m *Model) inventorySummary() string {
	if m.data == nil {
		return "Loading the complete captured inventory"
	}
	paths := len(m.data.Inventory)
	if m.data.Diff == nil {
		return fmt.Sprintf("%d paths · no diff available · per-path patch counts unavailable", paths)
	}
	if m.data.Diff.Origin == rawdiff.ComputedOrigin {
		summary := fmt.Sprintf("%s · %d paths · +%d −%d lines", m.data.Diff.Origin, paths, m.data.Diff.Added, m.data.Diff.Deleted)
		if m.data.Diff.SourceLimited {
			summary += " · limited inventory"
		}
		return summary
	}
	summary := fmt.Sprintf("%d paths · %d indexed hunks, all unclassified", paths, len(m.data.Diff.HunkRows))
	if !m.data.Diff.HunksComplete {
		summary += " · hunk index incomplete"
	}
	return summary
}

// inventoryReserved counts the summary and spacer rows above the Changes list.
func (m *Model) inventoryReserved() int { return 2 }

func (m *Model) inventoryBody() []string {
	width := m.inner()
	if m.data == nil || len(m.data.Inventory) == 0 {
		return []string{m.theme.Render("No captured inventory entries", width, terminal.Muted, false)}
	}
	capacity := max(m.bodyRows()-m.inventoryReserved(), 1)
	selected := min(max(m.inventory, 0), len(m.data.Inventory)-1)
	// Visual rows add one spacer before every heading but the first.
	visual := make([]int, 0, len(m.data.InventoryRows)+3)
	selectedVisual := 0
	for rowIndex, row := range m.data.InventoryRows {
		if row.EntryIndex < 0 && rowIndex > 0 {
			visual = append(visual, -1)
		}
		if row.EntryIndex == selected {
			selectedVisual = len(visual)
		}
		visual = append(visual, rowIndex)
	}
	top := max(0, selectedVisual-capacity+1)
	end := min(top+capacity, len(visual))
	listWidth, previewWidth := width, 0
	if m.splitView() {
		listWidth, previewWidth = overviewListWidth(width), m.previewWidth()
	}
	var preview []string
	if previewWidth > 0 {
		preview = m.inventoryPreviewLines(m.data.Inventory[selected], capacity, previewWidth)
	}
	out := []string{m.theme.Render(m.inventorySummary(), width, terminal.Muted, false), ""}
	rows := end - top
	if previewWidth > 0 {
		rows = max(rows, min(len(preview), capacity))
	}
	for visualIndex := top; visualIndex < top+rows; visualIndex++ {
		left := ""
		if visualIndex >= end {
			// Past the list: only the preview continues.
		} else if rowIndex := visual[visualIndex]; rowIndex >= 0 {
			row := m.data.InventoryRows[rowIndex]
			if row.EntryIndex < 0 {
				left = m.inventoryHeading(row, listWidth)
			} else {
				left = m.changeEntryLine(m.data.Inventory[row.EntryIndex], row.EntryIndex == selected, listWidth)
			}
			if m.searchMatchesRow(rowIndex) {
				left = m.theme.Highlight(left, m.searchQuery, listWidth)
			}
			if row.EntryIndex == selected {
				left = m.theme.Bar(left, listWidth)
			}
		}
		if previewWidth == 0 {
			out = append(out, left)
			continue
		}
		line := ""
		if offset := visualIndex - top; offset < len(preview) {
			line = preview[offset]
		}
		out = append(out, padStyled(left, listWidth)+m.paneDivider()+line)
	}
	return out
}

func (m *Model) inventoryHeading(row InventoryRow, width int) string {
	name, note, _ := strings.Cut(row.Heading, " — ")
	style := terminal.Strong
	if row.Heading != "CHANGED" {
		style = terminal.Attention
	}
	parts := []segment{{"  ", terminal.Plain}, {name, style}, {fmt.Sprintf("  %d", row.Count), terminal.Muted}}
	if note != "" {
		parts = append(parts, segment{"  " + note, terminal.Muted})
	}
	return m.segments(width, parts...)
}

// inventoryPreviewLines renders the selected path's summary and the start of
// its patch, styled like Diff. Every value is sanitized by the theme.
func (m *Model) inventoryPreviewLines(entry Entry, limit, width int) []string {
	letter, letterStyle := changeLetter(entry.Change)
	lines := []string{
		m.segments(width, segment{letter, letterStyle}, segment{"  ", terminal.Plain}, segment{entry.Name, terminal.Strong}),
	}
	meta := []segment{{entry.Change, terminal.Muted}}
	if entry.PotentialOracle {
		meta = append(meta, segment{" · ", terminal.Rule}, segment{"potential oracle", terminal.Attention})
	}
	if entry.Binary {
		meta = append(meta, segment{" · binary", terminal.Muted})
	}
	if entry.BaseMode != "" && entry.CandidateMode != "" && entry.BaseMode != entry.CandidateMode {
		meta = append(meta, segment{" · mode " + entry.BaseMode + " → " + entry.CandidateMode, terminal.Muted})
	}
	if entry.Added > 0 {
		meta = append(meta, segment{fmt.Sprintf("  +%d", entry.Added), terminal.AddedText})
	}
	if entry.Deleted > 0 {
		meta = append(meta, segment{fmt.Sprintf("  −%d", entry.Deleted), terminal.RemovedText})
	}
	lines = append(lines, m.segments(width, meta...))
	for _, limitation := range entry.Limits {
		lines = append(lines, m.theme.Render(limitation, width, terminal.Attention, false))
	}
	lines = append(lines, "")
	patch := func(line string) {
		style := patchLineStyle([]byte(line))
		lines = append(lines, m.theme.Render(line, width, style, style == terminal.Added || style == terminal.Removed))
	}
	switch {
	case m.data.Diff == nil:
		lines = append(lines, m.theme.Render("No shared captured patch for this pair", width, terminal.Muted, false))
	case m.data.Diff.FileByPath != nil && hasPath(m.data.Diff.FileByPath, entry.Name):
		file := m.data.Diff.Files[m.data.Diff.FileByPath[entry.Name]]
		offset := file.Start
		for offset < file.End && len(lines) < max(limit, 0) {
			end := bytes.IndexByte(m.data.Diff.Raw[offset:file.End], '\n')
			if end < 0 {
				end = file.End
			} else {
				end += offset
			}
			patch(string(m.data.Diff.Raw[offset:end]))
			if end == file.End {
				break
			}
			offset = end + 1
		}
	case m.data.Diff.Origin == rawdiff.ComputedOrigin:
		for _, line := range strings.Split(string(pathDiff(m.data, entry.Name)), "\n") {
			if len(lines) >= limit {
				break
			}
			patch(line)
		}
	default:
		lines = append(lines, m.theme.Render("No captured patch lines for this path", width, terminal.Muted, false))
	}
	if len(lines) > limit {
		lines = lines[:max(limit, 0)]
	}
	return lines
}

func hasPath(paths map[string]int, path string) bool {
	_, ok := paths[path]
	return ok
}

// diffFileAt returns the file index shown at the top of the Diff viewport.
func (m *Model) diffFileAt() int {
	if m.hex {
		byteOffset := m.top * 16
		candidate := sort.Search(len(m.data.Diff.Files), func(i int) bool { return m.data.Diff.Files[i].End > byteOffset })
		if candidate < len(m.data.Diff.Files) && m.data.Diff.Files[candidate].Start <= byteOffset {
			return candidate
		}
		if m.top == 0 {
			return 0
		}
		return -1
	}
	return diffPosition(m.data.Diff.Rows, m.top)
}

// diffHeaderAtTop reports whether the viewport already starts on a file bar,
// so the sticky copy would only repeat it.
func (m *Model) diffHeaderAtTop() bool {
	if m.hex || m.data == nil || m.data.Diff == nil || m.top < 0 || m.top >= len(m.data.Diff.Rows) {
		return false
	}
	return m.data.Diff.Rows[m.top].Summary != ""
}

func (m *Model) diffStickyHeader() string {
	width := m.inner()
	if m.data == nil || m.data.Diff == nil || len(m.data.Diff.Files) == 0 {
		origin := "captured patch"
		if m.data != nil && m.data.Diff != nil {
			origin = m.data.Diff.Origin
		}
		return m.theme.Render(origin+" · file metadata unavailable", width, terminal.Muted, false)
	}
	fileIndex := m.diffFileAt()
	if fileIndex < 0 || fileIndex >= len(m.data.Diff.Files) {
		return m.theme.Render(m.data.Diff.Origin+" · file metadata unavailable", width, terminal.Muted, false)
	}
	return m.diffFileBar(fileIndex, width)
}

// diffFileBar is the trusted per-file divider: path, header words and line
// counts, filled with a rule and the file's position. Payload never forms it.
func (m *Model) diffFileBar(fileIndex, width int) string {
	file := m.data.Diff.Files[fileIndex]
	name := file.Path
	if name == "" {
		name = "unmatched captured patch path"
	}
	parts := []segment{{"── ", terminal.Rule}, {name, terminal.Strong}}
	for _, kind := range file.Kinds {
		style := terminal.Muted
		if kind == "added" {
			style = terminal.AddedText
		} else if kind == "deleted" {
			style = terminal.RemovedText
		}
		parts = append(parts, segment{" · ", terminal.Rule}, segment{kind, style})
	}
	if file.PotentialOracle {
		parts = append(parts, segment{" · ", terminal.Rule}, segment{"potential oracle", terminal.Attention})
	}
	if file.Added > 0 {
		parts = append(parts, segment{fmt.Sprintf("  +%d", file.Added), terminal.AddedText})
	}
	if file.Deleted > 0 {
		parts = append(parts, segment{fmt.Sprintf("  −%d", file.Deleted), terminal.RemovedText})
	}
	right := fmt.Sprintf(" %d/%d ──", fileIndex+1, len(m.data.Diff.Files))
	rightWidth := uniseg.StringWidth(right)
	left := m.segments(max(width-rightWidth-2, 0), parts...)
	used := uniseg.StringWidth(terminal.VisibleText(left))
	fill := width - used - rightWidth - 1
	if fill < 1 {
		return left
	}
	return left + " " + m.theme.Render(strings.Repeat("─", fill), fill, terminal.Rule, false) + m.theme.Render(right, rightWidth, terminal.Rule, false)
}

func (m *Model) diffDocumentRow(rowIndex int) string {
	width := m.inner()
	if m.data == nil || m.data.Diff == nil || m.doc == nil || rowIndex < 0 || rowIndex >= len(m.data.Diff.Rows) {
		return ""
	}
	row := m.data.Diff.Rows[rowIndex]
	highlight := func(line string, lineWidth int) string {
		if m.searchMatchesRow(rowIndex) {
			return m.theme.Highlight(line, m.searchQuery, lineWidth)
		}
		return line
	}
	if row.Summary != "" {
		if row.File >= 0 && row.File < len(m.data.Diff.Files) {
			return highlight(m.diffFileBar(row.File, width), width)
		}
		return highlight(m.theme.Render(terminal.LineAt(row.Summary, m.left, width), width, terminal.Muted, false), width)
	}
	gutter := strings.Repeat(" ", 11) + "│ "
	if row.HasGutter {
		oldLine, newLine := "", ""
		if row.HasOld {
			oldLine = strconv.Itoa(row.OldLine)
		}
		if row.HasNew {
			newLine = strconv.Itoa(row.NewLine)
		}
		gutter = fmt.Sprintf("%5s %5s│ ", oldLine, newLine)
	}
	gutterWidth := uniseg.StringWidth(gutter)
	if width <= gutterWidth {
		return terminal.Line(gutter, width)
	}
	styledGutter := m.theme.Render(gutter, gutterWidth, terminal.Rule, false)
	available := width - gutterWidth
	marker := ""
	if m.doc.LongLine(row.RawLine) && available >= 4 {
		marker = m.theme.Render("[b]", 3, terminal.Muted, false)
		available -= 3
	}
	text := m.doc.LineAt(row.RawLine, m.left, available)
	tinted := row.Style == terminal.Added || row.Style == terminal.Removed
	content := m.theme.Render(text, available, row.Style, tinted)
	// Span emphasis is purely color styling; avoid comparing entire paired
	// lines when the terminal cannot display it.
	if m.theme.Color && tinted && row.Paired && row.Pair >= 0 && row.Pair < len(m.data.Diff.Rows) {
		content = m.emphasizedRow(row, text, available)
	}
	content = highlight(content, available)
	if text == "" && marker == "" && !tinted {
		return m.theme.Render(strings.TrimRight(gutter, " "), gutterWidth, terminal.Rule, false)
	}
	return styledGutter + content + marker
}

// emphasizedRow tints a replaced line and marks the span that changed against
// its paired line. Both lines are compared as sanitized display text.
func (m *Model) emphasizedRow(row DiffRow, window string, width int) string {
	pair := m.data.Diff.Rows[row.Pair]
	own := m.doc.SearchableLine(row.RawLine)
	other := m.doc.SearchableLine(pair.RawLine)
	if own == "" || other == "" {
		return m.theme.Render(window, width, row.Style, true)
	}
	// Compare without the +/- marker so the shared prefix is the code.
	var start, end int
	var ok bool
	if row.Style == terminal.Removed {
		start, end, _, _, ok = terminal.ChangedSpan(own[1:], other[1:])
	} else {
		_, _, start, end, ok = terminal.ChangedSpan(other[1:], own[1:])
	}
	if !ok {
		return m.theme.Render(window, width, row.Style, true)
	}
	start, end = start+1-m.left, end+1-m.left
	emphasis := terminal.AddedEmphasis
	if row.Style == terminal.Removed {
		emphasis = terminal.RemovedEmphasis
	}
	visible := uniseg.StringWidth(window)
	start, end = min(max(start, 0), visible), min(max(end, 0), visible)
	before := terminal.Columns(window, 0, start)
	changed := terminal.Columns(window, start, end)
	after := terminal.Columns(window, end, visible)
	out := m.theme.Render(before, uniseg.StringWidth(before), row.Style, false) +
		m.theme.Render(changed, uniseg.StringWidth(changed), emphasis, false)
	rest := width - uniseg.StringWidth(before) - uniseg.StringWidth(changed)
	return out + m.theme.Render(after, max(rest, 0), row.Style, true)
}

func (m *Model) navigateFile(delta int) {
	if m.data == nil || m.data.Diff == nil || len(m.data.Diff.Files) == 0 {
		return
	}
	current := diffPosition(m.data.Diff.Rows, m.top)
	if current < 0 {
		current = 0
	}
	next := current + delta
	for next >= 0 && next < len(m.data.Diff.Files) && m.data.Diff.Files[next].StartRow < 0 {
		next += delta
	}
	if next < 0 || next >= len(m.data.Diff.Files) {
		return
	}
	m.top = m.data.Diff.Files[next].StartRow
	m.left = 0
}

func (m *Model) navigateHunk(delta int) {
	if m.data == nil || m.data.Diff == nil || len(m.data.Diff.HunkRows) == 0 {
		return
	}
	index := sort.Search(len(m.data.Diff.HunkRows), func(i int) bool { return m.data.Diff.HunkRows[i] > m.top }) - 1
	if delta > 0 {
		index++
	} else if index > 0 {
		index--
	}
	index = min(max(index, 0), len(m.data.Diff.HunkRows)-1)
	m.top = m.data.Diff.HunkRows[index]
	m.left = 0
}

func padStyled(text string, width int) string {
	return text + strings.Repeat(" ", max(width-uniseg.StringWidth(terminal.VisibleText(text)), 0))
}

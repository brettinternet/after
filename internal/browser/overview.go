package browser

import (
	"fmt"
	"sort"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
)

type overviewGroup uint8

const (
	groupNeedsAnotherLook overviewGroup = iota
	groupPinnedExpectations
	groupAgrees
	groupReported
	groupEarlierSnapshots
	groupOther
)

type overviewRowKind uint8

const (
	overviewMessage overviewRowKind = iota
	overviewGroupHeader
	overviewEvidence
	overviewReport
	overviewInventory
	overviewChanges
	overviewSpacer
)

// selectable rows can hold the cursor; spacers and messages cannot.
func (row overviewRow) selectable() bool {
	return row.kind == overviewGroupHeader || row.kind == overviewEvidence || row.kind == overviewInventory
}

type overviewRow struct {
	kind           overviewRowKind
	group          overviewGroup
	count          int
	entryIndex     int
	inventoryIndex int
	text           string
}

// overviewGroupLabel returns a group's heading, an optional qualifier and the
// heading style. Green stays reserved for current complete equality.
func overviewGroupLabel(group overviewGroup) (string, string, terminal.Style) {
	switch group {
	case groupNeedsAnotherLook:
		return "NEEDS ANOTHER LOOK", "", terminal.Attention
	case groupPinnedExpectations:
		return "PINNED EXPECTATIONS", "", terminal.Decision
	case groupAgrees:
		return "AGREES", "", terminal.Observed
	case groupReported:
		return "REPORTED", "imported, not observed by AFTER", terminal.Reported
	case groupEarlierSnapshots:
		return "EARLIER SNAPSHOTS", "", terminal.Muted
	default:
		return "OTHER", "", terminal.Muted
	}
}

func overviewClass(entry Entry, candidate evidence.Digest) overviewGroup {
	if entry.Unavailable {
		return groupNeedsAnotherLook
	}
	if entry.State.Kind == evidence.Reported {
		return groupReported
	}
	if entry.Decision != "" {
		if entry.Decision == evidence.Reopened || entry.State.Applicability == evidence.Stale || entry.State.Applicability == evidence.Unknown {
			return groupNeedsAnotherLook
		}
		return groupPinnedExpectations
	}
	if entry.State.Kind == evidence.Observed {
		if entry.State.Applicability == evidence.Stale || (entry.Candidate != "" && entry.Candidate != candidate) {
			return groupEarlierSnapshots
		}
		if entry.State.Applicability == evidence.Unknown || entry.State.Execution == evidence.Failed || entry.State.Execution == evidence.Cancelled || entry.Completeness != evidence.Complete || entry.State.Comparison == evidence.Incomparable || entry.State.Comparison == evidence.Unstable || entry.State.Comparison == evidence.Different {
			return groupNeedsAnotherLook
		}
		if entry.State.Applicability == evidence.Current && entry.State.Execution == evidence.Completed && entry.State.Comparison == evidence.Equal {
			return groupAgrees
		}
	}
	return groupOther
}

func needsPriority(entry Entry) int {
	switch {
	case entry.Decision == evidence.Reopened:
		return 0
	case entry.Decision != "":
		return 1
	case entry.Unavailable:
		return 2
	default:
		return 3
	}
}

func (m *Model) overviewCollapsed(group overviewGroup) bool {
	if m.overviewCollapsedGroups == nil {
		return group == groupEarlierSnapshots
	}
	collapsed, ok := m.overviewCollapsedGroups[group]
	return collapsed || (!ok && group == groupEarlierSnapshots)
}

func (m *Model) overviewRows() []overviewRow {
	if m.data == nil {
		if m.loadFailed {
			return []overviewRow{{kind: overviewMessage, text: "Stored records unavailable; check the IDs with after inspect"}}
		}
		return []overviewRow{{kind: overviewMessage, text: "Loading immutable records"}}
	}
	if len(m.data.Entries) == 0 {
		rows := []overviewRow{
			{kind: overviewMessage, text: "NOT CHECKED — no evidence was loaded for this change"},
			{kind: overviewMessage, text: "Nothing was run or imported for candidate " + shortID(m.data.Selection.Pair.Candidate) + ". The change is readable below."},
			{kind: overviewSpacer},
			{kind: overviewChanges},
		}
		if detail := m.overviewChangeDetailLine(); detail != "" {
			rows = append(rows, overviewRow{kind: overviewMessage, text: detail})
		}
		for i := range m.data.Inventory {
			rows = append(rows, overviewRow{kind: overviewInventory, inventoryIndex: i})
		}
		return rows
	}

	indices := m.uniqueOverviewEvidence()
	grouped := map[overviewGroup][]int{}
	for _, index := range indices {
		group := overviewClass(m.data.Entries[index], m.data.Selection.Pair.Candidate)
		grouped[group] = append(grouped[group], index)
	}
	sort.SliceStable(grouped[groupNeedsAnotherLook], func(i, j int) bool {
		return needsPriority(m.data.Entries[grouped[groupNeedsAnotherLook][i]]) < needsPriority(m.data.Entries[grouped[groupNeedsAnotherLook][j]])
	})

	rows := []overviewRow{}
	for group := groupNeedsAnotherLook; group <= groupOther; group++ {
		entries := grouped[group]
		if len(entries) == 0 {
			continue
		}
		if len(rows) > 0 {
			rows = append(rows, overviewRow{kind: overviewSpacer})
		}
		rows = append(rows, overviewRow{kind: overviewGroupHeader, group: group, count: len(entries)})
		if m.overviewCollapsed(group) {
			continue
		}
		if group != groupReported {
			for _, index := range entries {
				rows = append(rows, overviewRow{kind: overviewEvidence, group: group, entryIndex: index})
			}
			continue
		}
		seenReports := map[evidence.Digest]bool{}
		for _, index := range entries {
			entry := m.data.Entries[index]
			if !seenReports[entry.ReportID] {
				seenReports[entry.ReportID] = true
				rows = append(rows, overviewRow{kind: overviewReport, group: group, entryIndex: index})
			}
			rows = append(rows, overviewRow{kind: overviewEvidence, group: group, entryIndex: index})
		}
	}
	selection := m.data.Selection
	rows = append(rows, overviewRow{kind: overviewSpacer})
	if selection.OmittedEvidence > 0 {
		rows = append(rows, overviewRow{kind: overviewMessage, text: fmt.Sprintf("%d matching records not loaded · after log lists older records", selection.OmittedEvidence)})
	} else if selection.DiscoveryWarning {
		rows = append(rows, overviewRow{kind: overviewMessage, text: "Evidence discovery was limited; after log lists older records"})
	}
	if selection.OmittedEvidence > 0 || selection.DiscoveryWarning {
		rows = append(rows, overviewRow{kind: overviewSpacer})
	}
	rows = append(rows, overviewRow{kind: overviewChanges})
	if detail := m.overviewChangeDetailLine(); detail != "" {
		rows = append(rows, overviewRow{kind: overviewMessage, text: detail})
	}
	for index, entry := range m.data.Inventory {
		if entry.PotentialOracle {
			rows = append(rows, overviewRow{kind: overviewInventory, inventoryIndex: index})
		}
	}
	return rows
}

func (m *Model) uniqueOverviewEvidence() []int {
	indices := make([]int, 0, len(m.data.Entries))
	byObservation := map[string]int{}
	for index, entry := range m.data.Entries {
		if entry.State.Kind != evidence.Observed || entry.SourceReceipt == "" {
			indices = append(indices, index)
			continue
		}
		key := string(entry.SourceReceipt) + "\x00" + entry.Name
		position, exists := byObservation[key]
		if !exists {
			byObservation[key] = len(indices)
			indices = append(indices, index)
			continue
		}
		previous := indices[position]
		if entry.HasComparison && !m.data.Entries[previous].HasComparison {
			indices[position] = index
		}
	}
	return indices
}

func (m *Model) overviewRowText(row overviewRow, selected bool) string {
	return m.overviewRowTextWidth(row, selected, m.width)
}

func (m *Model) overviewRowTextWidth(row overviewRow, selected bool, width int) string {
	prefix := m.cursorPrefix(selected)
	switch row.kind {
	case overviewSpacer:
		return ""
	case overviewMessage:
		if rest, ok := strings.CutPrefix(row.text, "NOT CHECKED"); ok {
			return m.segments(width, segment{"  ", terminal.Plain}, segment{"NOT CHECKED", terminal.Attention}, segment{rest, terminal.Muted})
		}
		return m.segments(width, segment{"  ", terminal.Plain}, segment{row.text, terminal.Muted})
	case overviewGroupHeader:
		marker := "▾ "
		if m.overviewCollapsed(row.group) {
			marker = "▸ "
		}
		name, note, style := overviewGroupLabel(row.group)
		parts := []segment{{prefix, terminal.Accent}, {marker, terminal.Rule}, {name, style}, {fmt.Sprintf("  %d", row.count), terminal.Muted}}
		if note != "" {
			parts = append(parts, segment{"  " + note, terminal.Muted})
		}
		return m.segments(width, parts...)
	case overviewEvidence:
		if row.entryIndex < 0 || row.entryIndex >= len(m.data.Entries) {
			return ""
		}
		return m.entryLineWidth(m.data.Entries[row.entryIndex], selected, width)
	case overviewReport:
		if row.entryIndex < 0 || row.entryIndex >= len(m.data.Entries) {
			return ""
		}
		return "  " + m.reportLine(m.data.Entries[row.entryIndex], max(width-2, 0))
	case overviewInventory:
		if row.inventoryIndex < 0 || row.inventoryIndex >= len(m.data.Inventory) {
			return ""
		}
		return m.changeEntryLine(m.data.Inventory[row.inventoryIndex], selected, width)
	case overviewChanges:
		heading, counts, _ := strings.Cut(m.overviewChangesLine(), "  ")
		return m.segments(width, segment{"  ", terminal.Plain}, segment{heading, terminal.Strong}, segment{"  " + counts, terminal.Muted})
	default:
		return ""
	}
}

func (m *Model) reportLine(entry Entry, width int) string {
	binding := "unbound"
	if entry.ReportBinding != "" {
		binding = shortID(entry.ReportBinding)
	}
	imported := "unknown time"
	if !entry.ReportImportedAt.IsZero() {
		at := entry.ReportImportedAt.In(m.zone)
		now := m.now().In(m.zone)
		format := "Jan 2 15:04"
		if at.Year() == now.Year() && at.YearDay() == now.YearDay() {
			format = "15:04"
		}
		imported = at.Format(format)
	}
	outcomes := []string{}
	if entry.ReportCounts.Pass > 0 {
		outcomes = append(outcomes, fmt.Sprintf("%d pass", entry.ReportCounts.Pass))
	}
	if entry.ReportCounts.Fail > 0 {
		outcomes = append(outcomes, fmt.Sprintf("%d fail", entry.ReportCounts.Fail))
	}
	if entry.ReportCounts.Skip > 0 {
		outcomes = append(outcomes, fmt.Sprintf("%d skip", entry.ReportCounts.Skip))
	}
	if len(outcomes) == 0 {
		outcomes = append(outcomes, "no cases")
	}
	prefix := fmt.Sprintf("report %s · bound to %s · imported %s · %s · ", shortID(entry.ReportID), binding, imported, strings.Join(outcomes, " · "))
	prefixWidth := uniseg.StringWidth(terminal.Line(prefix, width))
	if prefixWidth >= width {
		return m.theme.Render(prefix, width, terminal.Reported, false)
	}
	producer := entry.ReportProducer
	if producer == "" {
		producer = "producer not recorded"
	}
	return m.theme.Render(prefix, prefixWidth, terminal.Reported, false) + m.theme.Render(producer, width-prefixWidth, terminal.Plain, false)
}

func (m *Model) currentOverviewRow() (overviewRow, bool) {
	rows := m.overviewRows()
	if m.overviewPosition < 0 || m.overviewPosition >= len(rows) {
		return overviewRow{}, false
	}
	return rows[m.overviewPosition], true
}

func overviewListWidth(width int) int { return width * 45 / 100 }

// splitView reports whether lists show a preview pane beside them.
func (m *Model) splitView() bool { return m.width >= 110 }

// previewWidth is the preview pane width beside a list of listWidth.
func (m *Model) previewWidth() int { return m.inner() - overviewListWidth(m.inner()) - 3 }

func (m *Model) paneDivider() string { return m.theme.Render(" \u2502 ", 3, terminal.Rule, false) }

func (m *Model) overviewBody(width int) []string {
	rows := m.overviewRows()
	position := min(max(m.overviewPosition, 0), max(len(rows)-1, 0))
	capacity := max(m.rows(), 1)
	top := max(0, position-capacity+1)
	line := func(index, lineWidth int) string {
		text := m.overviewRowTextWidth(rows[index], index == position, lineWidth)
		if m.searchMatchesRow(index) {
			text = m.theme.Highlight(text, m.searchQuery, lineWidth)
		}
		if index == position && rows[index].selectable() {
			return m.theme.Bar(text, lineWidth)
		}
		return text
	}
	body := []string{}
	if !m.splitView() {
		for index := top; index < min(len(rows), top+capacity); index++ {
			body = append(body, line(index, width))
		}
		return body
	}
	listWidth := overviewListWidth(width)
	previewWidth := m.previewWidth()
	for offset := 0; offset < capacity; offset++ {
		left := ""
		if index := top + offset; index < len(rows) {
			left = line(index, listWidth)
		}
		body = append(body, padStyled(left, listWidth)+m.paneDivider()+m.overviewPreviewLine(offset, previewWidth))
	}
	return body
}

func (m *Model) startOverviewPreview() tea.Cmd {
	m.previewRequest++
	request := m.previewRequest
	m.previewKey = ""
	m.previewDoc = nil
	m.previewDividers = nil
	if m.screen != "examples" || !m.splitView() {
		return nil
	}
	row, ok := m.currentOverviewRow()
	entry := m.selectedOverviewEntry()
	if !ok || entry == nil || len(entry.Sections) == 0 || entry.Sections[0].Name != "Card" {
		return nil
	}
	key := fmt.Sprintf("%d:%d:%s", row.kind, row.entryIndex, entry.Name)
	m.previewKey = key
	section := entry.Sections[0]
	project := m.selected.Project
	width := m.previewWidth()
	return m.spawn(func() tea.Msg {
		view, err := readSectionDocument(m.ctx, project, section, width)
		if err != nil {
			return previewReady{request: request, key: key, err: err}
		}
		doc, err := terminal.NewDocumentView(view.display, view.raw)
		return previewReady{request: request, key: key, doc: doc, dividers: view.dividers, err: err}
	})
}

func (m *Model) overviewPreviewLine(row int, width int) string {
	if m.previewDoc == nil {
		if row != 0 {
			return ""
		}
		text := "Select an evidence row to preview its Card"
		if m.previewKey != "" {
			text = "Loading the Card preview\u2026"
		}
		return m.theme.Render(text, width, terminal.Muted, false)
	}
	if m.previewDividers[row] {
		return m.theme.Render(m.previewDoc.LineAt(row, 0, width), width, terminal.Strong, false)
	}
	return m.previewDoc.LineAt(row, 0, width)
}

func (m *Model) selectedOverviewEntry() *Entry {
	row, ok := m.currentOverviewRow()
	if !ok || m.data == nil {
		return nil
	}
	switch row.kind {
	case overviewEvidence:
		if row.entryIndex >= 0 && row.entryIndex < len(m.data.Entries) {
			return &m.data.Entries[row.entryIndex]
		}
	case overviewInventory:
		if row.inventoryIndex >= 0 && row.inventoryIndex < len(m.data.Inventory) {
			return &m.data.Inventory[row.inventoryIndex]
		}
	}
	return nil
}

func (m *Model) selectOverviewPosition(position int) {
	rows := m.overviewRows()
	if len(rows) == 0 {
		m.overviewPosition = 0
		return
	}
	m.overviewPosition = min(max(position, 0), len(rows)-1)
	row := rows[m.overviewPosition]
	switch row.kind {
	case overviewEvidence:
		m.index = row.entryIndex
	case overviewInventory:
		m.inventory = row.inventoryIndex
	}
}

// nearestSelectable moves from position in the direction of delta past
// spacers and messages, then falls back the other way at either end.
func nearestSelectable(rows []overviewRow, position, delta int) int {
	if len(rows) == 0 {
		return 0
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	position = min(max(position, 0), len(rows)-1)
	for _, direction := range []int{step, -step} {
		for index := position; index >= 0 && index < len(rows); index += direction {
			if rows[index].selectable() {
				return index
			}
		}
	}
	return position
}

func (m *Model) selectFirstOverviewRow() {
	rows := m.overviewRows()
	for index, row := range rows {
		if row.kind == overviewEvidence || row.kind == overviewInventory {
			m.selectOverviewPosition(index)
			return
		}
	}
	m.overviewPosition = 0
}

func (m *Model) positionOverviewEntry(entryIndex int) {
	for index, row := range m.overviewRows() {
		if row.kind == overviewEvidence && row.entryIndex == entryIndex {
			m.selectOverviewPosition(index)
			return
		}
	}
	m.selectFirstOverviewRow()
}

func (m *Model) toggleOverviewGroup() {
	row, ok := m.currentOverviewRow()
	if !ok || row.kind != overviewGroupHeader {
		return
	}
	if m.overviewCollapsedGroups == nil {
		m.overviewCollapsedGroups = map[overviewGroup]bool{groupEarlierSnapshots: true}
	}
	m.overviewCollapsedGroups[row.group] = !m.overviewCollapsed(row.group)
	for index, candidate := range m.overviewRows() {
		if candidate.kind == overviewGroupHeader && candidate.group == row.group {
			m.overviewPosition = index
			return
		}
	}
}

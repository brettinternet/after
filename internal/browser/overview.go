package browser

import (
	"fmt"
	"sort"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
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
)

type overviewRow struct {
	kind           overviewRowKind
	group          overviewGroup
	count          int
	entryIndex     int
	inventoryIndex int
	text           string
}

func overviewGroupName(group overviewGroup) string {
	switch group {
	case groupNeedsAnotherLook:
		return "NEEDS ANOTHER LOOK"
	case groupPinnedExpectations:
		return "PINNED EXPECTATIONS"
	case groupAgrees:
		return "AGREES"
	case groupReported:
		return "REPORTED — imported, not observed by AFTER"
	case groupEarlierSnapshots:
		return "EARLIER SNAPSHOTS"
	default:
		return "OTHER"
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
	if selection.OmittedEvidence > 0 {
		rows = append(rows, overviewRow{kind: overviewMessage, text: fmt.Sprintf("%d matching records not loaded · after log lists older records", selection.OmittedEvidence)})
	} else if selection.DiscoveryWarning {
		rows = append(rows, overviewRow{kind: overviewMessage, text: "Evidence discovery was limited; after log lists older records"})
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
	prefix := "  "
	if selected {
		prefix = "> "
	}
	switch row.kind {
	case overviewMessage:
		style := terminal.Muted
		if strings.HasPrefix(row.text, "NOT CHECKED") {
			style = terminal.Strong
		}
		return prefix + m.theme.Render(row.text, max(m.width-2, 0), style, false)
	case overviewGroupHeader:
		marker := "▾"
		if m.overviewCollapsed(row.group) {
			marker = "▸"
		}
		text := fmt.Sprintf("%s %s %d", marker, overviewGroupName(row.group), row.count)
		return prefix + m.theme.Render(text, max(m.width-2, 0), terminal.Strong, false)
	case overviewEvidence:
		if row.entryIndex < 0 || row.entryIndex >= len(m.data.Entries) {
			return ""
		}
		return m.entryLine(m.data.Entries[row.entryIndex], selected)
	case overviewReport:
		if row.entryIndex < 0 || row.entryIndex >= len(m.data.Entries) {
			return ""
		}
		return prefix + m.reportLine(m.data.Entries[row.entryIndex], max(m.width-2, 0))
	case overviewInventory:
		if row.inventoryIndex < 0 || row.inventoryIndex >= len(m.data.Inventory) {
			return ""
		}
		return m.changeEntryLine(m.data.Inventory[row.inventoryIndex], selected, m.width)
	case overviewChanges:
		return m.theme.Render(m.overviewChangesLine(), m.width, terminal.Strong, false)
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

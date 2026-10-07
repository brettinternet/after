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
	if m.data == nil || m.data.Diff == nil || m.data.Diff.Origin != rawdiff.ComputedOrigin {
		return ""
	}
	line := fmt.Sprintf("CHANGES · %s · %d paths · +%d −%d lines", m.data.Diff.Origin, len(m.data.Inventory), m.data.Diff.Added, m.data.Diff.Deleted)
	if m.data.Diff.SourceLimited {
		line += " · limited inventory"
	}
	return line
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

func (m *Model) inventoryBody() []string {
	if m.data == nil || len(m.data.Inventory) == 0 {
		return []string{terminal.Line("No captured inventory entries", m.width)}
	}
	rowCapacity := max(m.bodyRows()-1, 1) // one row is reserved for inventorySummary
	selected := min(max(m.inventory, 0), len(m.data.Inventory)-1)
	selectedRow := m.data.InventoryPositions[selected]
	top := max(0, selectedRow-rowCapacity+1)
	end := min(top+rowCapacity, len(m.data.InventoryRows))
	listWidth := m.width
	previewWidth := 0
	if m.width >= 110 {
		listWidth = m.width * 45 / 100
		previewWidth = m.width - listWidth - 3
	}
	previewLines := []string{}
	if previewWidth > 0 {
		previewLines = m.inventoryPreviewLines(m.data.Inventory[selected], rowCapacity-1)
	}
	out := []string{terminal.Line(m.inventorySummary(), m.width)}
	for rowIndex := top; rowIndex < end; rowIndex++ {
		row := m.data.InventoryRows[rowIndex]
		var left string
		if row.EntryIndex < 0 {
			left = m.theme.Render(fmt.Sprintf("%s %d", row.Heading, row.Count), listWidth, terminal.Strong, false)
		} else {
			left = m.changeEntryLine(m.data.Inventory[row.EntryIndex], row.EntryIndex == selected, listWidth)
		}
		if previewWidth == 0 {
			out = append(out, left)
			continue
		}
		previewRow := rowIndex - top
		preview := ""
		if previewRow == 0 {
			preview = "SELECTED PATH"
		} else if line := previewRow - 1; line < len(previewLines) {
			preview = previewLines[line]
		}
		line := padStyled(left, listWidth) + " │ " + m.theme.Render(preview, previewWidth, terminal.Plain, false)
		out = append(out, line)
	}
	return out
}

func (m *Model) inventoryPreviewLines(entry Entry, limit int) []string {
	lines := []string{"PATH " + entry.Name, "CHANGE " + entry.Change, fmt.Sprintf("LINES +%d −%d", entry.Added, entry.Deleted)}
	flags := []string{}
	if entry.PotentialOracle {
		flags = append(flags, "oracle")
	}
	if entry.Binary {
		flags = append(flags, "binary")
	}
	if entry.BaseMode != "" && entry.CandidateMode != "" && entry.BaseMode != entry.CandidateMode {
		flags = append(flags, "mode "+entry.BaseMode+" → "+entry.CandidateMode)
	}
	if len(flags) > 0 {
		lines = append(lines, "FLAGS "+strings.Join(flags, " · "))
	}
	for _, limitation := range entry.Limits {
		lines = append(lines, "LIMIT "+limitation)
	}
	if m.data.Diff == nil {
		lines = append(lines, "DIFF unavailable: no shared captured patch")
	} else if fileIndex, ok := m.data.Diff.FileByPath[entry.Name]; ok {
		file := m.data.Diff.Files[fileIndex]
		offset := file.Start
		for offset < file.End && len(lines) < max(limit, 0) {
			end := bytes.IndexByte(m.data.Diff.Raw[offset:file.End], '\n')
			if end < 0 {
				end = file.End
			} else {
				end += offset
			}
			lines = append(lines, string(m.data.Diff.Raw[offset:end]))
			if end == file.End {
				break
			}
			offset = end + 1
		}
	} else if m.data.Diff.Origin == rawdiff.ComputedOrigin {
		lines = append(lines, string(pathDiff(m.data, entry.Name)))
	} else {
		lines = append(lines, "No captured patch lines for this path")
	}
	if len(lines) > limit {
		lines = lines[:max(limit, 0)]
	}
	return lines
}

func (m *Model) diffStickyHeader() string {
	if m.data == nil || m.data.Diff == nil || len(m.data.Diff.Files) == 0 {
		origin := "captured patch"
		if m.data != nil && m.data.Diff != nil {
			origin = m.data.Diff.Origin
		}
		return m.theme.Render(origin+" · file metadata unavailable", m.width, terminal.Muted, false)
	}
	fileIndex := -1
	if m.hex {
		byteOffset := m.top * 16
		candidate := sort.Search(len(m.data.Diff.Files), func(i int) bool { return m.data.Diff.Files[i].End > byteOffset })
		if candidate < len(m.data.Diff.Files) && m.data.Diff.Files[candidate].Start <= byteOffset {
			fileIndex = candidate
		}
		if fileIndex < 0 && m.top == 0 {
			fileIndex = 0
		}
	} else {
		fileIndex = diffPosition(m.data.Diff.Rows, m.top)
	}
	if fileIndex < 0 || fileIndex >= len(m.data.Diff.Files) {
		return m.theme.Render(m.data.Diff.Origin+" · file metadata unavailable", m.width, terminal.Muted, false)
	}
	return m.theme.Render(diffHeader(m.data.Diff.Files[fileIndex], fileIndex, len(m.data.Diff.Files), m.data.Diff.Origin), m.width, terminal.Strong, false)
}

func (m *Model) diffDocumentRow(rowIndex int) string {
	if m.data == nil || m.data.Diff == nil || m.doc == nil || rowIndex < 0 || rowIndex >= len(m.data.Diff.Rows) {
		return ""
	}
	row := m.data.Diff.Rows[rowIndex]
	if row.Summary != "" {
		return m.theme.Render(row.Summary, m.width, terminal.Muted, false)
	}
	gutter := strings.Repeat(" ", 12) + "│ "
	if row.HasGutter {
		oldLine, newLine := "", ""
		if row.HasOld {
			oldLine = strconv.Itoa(row.OldLine)
		}
		if row.HasNew {
			newLine = strconv.Itoa(row.NewLine)
		}
		gutter = fmt.Sprintf("%5s %5s │ ", oldLine, newLine)
	}
	gutterWidth := uniseg.StringWidth(gutter)
	if m.width <= gutterWidth {
		return terminal.Line(gutter, m.width)
	}
	available := m.width - gutterWidth
	marker := ""
	if m.doc.LongLine(row.RawLine) && available >= 4 {
		marker = "[b]"
		available -= len(marker)
	}
	content := m.doc.LineAt(row.RawLine, m.left, available)
	if content == "" && marker == "" {
		gutter = strings.TrimSuffix(gutter, " ")
	}
	return gutter + m.theme.Render(content, available, row.Style, false) + marker
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
	plain := strings.NewReplacer("\x1b[32m", "", "\x1b[1;35m", "", "\x1b[1;33m", "", "\x1b[1;31m", "", "\x1b[34m", "", "\x1b[36m", "", "\x1b[2m", "", "\x1b[1m", "", "\x1b[7m", "", "\x1b[0m", "").Replace(text)
	return text + strings.Repeat(" ", max(width-uniseg.StringWidth(plain), 0))
}

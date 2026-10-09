package browser

import (
	"context"
	"fmt"
	"sort"

	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
)

const maxSearchQueryBytes = 512

type searchMatch struct {
	row, order, cursor int
	column             int
}

type searchReady struct {
	request    uint64
	docRequest uint64
	screen     string
	query      string
	cursor     int
	matches    []searchMatch
	err        error
}

func canSearch(m *Model) string {
	switch keyScreen(m) {
	case "examples", "inventory", "activity":
		if m.data == nil {
			return "wait for stored records to load"
		}
		return ""
	case "inspector", "patch":
		if m.doc == nil {
			return "wait for the document to load"
		}
		return ""
	default:
		return "search is unavailable on this screen"
	}
}

func canSearchNext(m *Model) string {
	switch {
	case m.searchEditing:
		return "finish the query first"
	case m.searchQuery == "":
		return "enter a search query with /"
	case m.searchPending:
		return "search is still running"
	case len(m.searchMatches) == 0:
		return "there are no matches"
	default:
		return ""
	}
}

func (m *Model) resetSearchResults() {
	if m.searchCancel != nil {
		m.searchCancel()
		m.searchCancel = nil
	}
	m.searchRequest++
	m.searchPending = false
	m.searchMatches = nil
	m.searchCurrent = -1
}

func (m *Model) clearSearch() {
	m.searchEditing = false
	m.searchQuery = ""
	m.resetSearchResults()
}

func (m *Model) openSearch() {
	m.resetSearchResults()
	m.searchEditing = true
}

func (m *Model) updateSearchKey(key tea.KeyMsg) (tea.Cmd, bool) {
	if !m.searchEditing {
		return nil, false
	}
	if key.Paste {
		m.searchQuery = appendSearchQuery(m.searchQuery, key.Runes)
		m.resetSearchResults()
		m.searchEditing = true
		return nil, true
	}
	switch key.String() {
	case "esc":
		m.clearSearch()
		m.status = "Search cleared"
		return nil, true
	case "enter":
		m.searchEditing = false
		if m.searchQuery == "" {
			m.clearSearch()
			return nil, true
		}
		return m.startSearch(), true
	case "backspace", "delete", "ctrl+h", "ctrl+w":
		if key.String() == "ctrl+w" {
			m.searchQuery = removeLastWord(m.searchQuery)
		} else {
			m.searchQuery = removeLastRune(m.searchQuery)
		}
		m.resetSearchResults()
		m.searchEditing = true
		return nil, true
	case "ctrl+u":
		m.searchQuery = ""
		m.resetSearchResults()
		m.searchEditing = true
		return nil, true
	case "ctrl+c":
		m.clearSearch()
		m.quitNow()
		return tea.Quit, true
	}
	if len(key.Runes) > 0 {
		m.searchQuery = appendSearchQuery(m.searchQuery, key.Runes)
		m.resetSearchResults()
		m.searchEditing = true
	}
	return nil, true
}

func appendSearchQuery(query string, input []rune) string {
	for _, r := range input {
		if len(query) >= maxSearchQueryBytes {
			break
		}
		safe := terminal.Sanitize(string(r))
		if len(query)+len(safe) > maxSearchQueryBytes {
			break
		}
		query += safe
	}
	return query
}

func (m *Model) startSearch() tea.Cmd {
	m.resetSearchResults()
	if m.searchQuery == "" {
		return nil
	}
	request, docRequest := m.searchRequest, m.request
	screen, query, cursor := m.screen, m.searchQuery, m.searchCursorOrder()
	searchView := m.searchSnapshot()
	doc, data, hex := m.doc, m.data, m.hex
	ctx, cancel := context.WithCancel(m.ctx)
	m.searchCancel = cancel
	m.searchPending = true
	return m.spawn(func() tea.Msg {
		var matches []searchMatch
		var err error
		switch screen {
		case "examples", "inventory", "activity":
			matches, err = searchList(ctx, searchView, screen, query)
		case "inspector", "patch":
			matches, err = searchDocument(ctx, doc, data, screen, hex, query)
		default:
			err = fmt.Errorf("search is unavailable on this screen")
		}
		cancel()
		return searchReady{request: request, docRequest: docRequest, screen: screen, query: query, cursor: cursor, matches: matches, err: err}
	})
}

func (m *Model) searchSnapshot() *Model {
	collapsed := make(map[overviewGroup]bool, len(m.overviewCollapsedGroups))
	for group, value := range m.overviewCollapsedGroups {
		collapsed[group] = value
	}
	return &Model{
		data:                    m.data,
		selected:                m.selected,
		width:                   m.width,
		activity:                append([]ActivityEvent(nil), m.activity...),
		activityDropped:         m.activityDropped,
		theme:                   m.theme,
		now:                     m.now,
		zone:                    m.zone,
		overviewCollapsedGroups: collapsed,
	}
}

func searchList(ctx context.Context, view *Model, screen, query string) ([]searchMatch, error) {
	matches := []searchMatch{}
	add := func(row, order, cursor int, text string) bool {
		if err := ctx.Err(); err != nil {
			return false
		}
		if terminal.Contains(query, terminal.VisibleText(text)) {
			matches = append(matches, searchMatch{row: row, order: order, cursor: cursor})
		}
		return true
	}
	switch screen {
	case "examples":
		rows := view.overviewRows()
		for rowIndex, row := range rows {
			if !add(rowIndex, rowIndex, rowIndex, view.overviewRowTextWidth(row, false, view.width)) {
				return nil, ctx.Err()
			}
		}
	case "inventory":
		if view.data == nil {
			return nil, nil
		}
		for rowIndex, row := range view.data.InventoryRows {
			line, cursor := "", row.EntryIndex
			if row.EntryIndex < 0 {
				line = view.theme.Render(fmt.Sprintf("%s %d", row.Heading, row.Count), view.width, terminal.Strong, false)
				cursor = inventoryCursorForRow(view.data.InventoryRows, rowIndex)
			} else {
				line = view.changeEntryLine(view.data.Inventory[row.EntryIndex], false, view.width)
			}
			if !add(rowIndex, rowIndex, cursor, line) {
				return nil, ctx.Err()
			}
		}
	case "activity":
		for rowIndex := range view.activity {
			if !add(rowIndex, rowIndex, rowIndex, view.activityLine(rowIndex, false)) {
				return nil, ctx.Err()
			}
		}
	}
	return matches, nil
}

func inventoryCursorForRow(rows []InventoryRow, row int) int {
	for index := row + 1; index < len(rows); index++ {
		if rows[index].EntryIndex >= 0 {
			return rows[index].EntryIndex
		}
	}
	for index := row - 1; index >= 0; index-- {
		if rows[index].EntryIndex >= 0 {
			return rows[index].EntryIndex
		}
	}
	return 0
}

func searchDocument(ctx context.Context, doc *terminal.Document, data *Data, screen string, hex bool, query string) ([]searchMatch, error) {
	if doc == nil {
		return nil, nil
	}
	rows := doc.Lines()
	if hex {
		rows = doc.HexRows()
	} else if screen == "patch" && data != nil && data.Diff != nil {
		rows = len(data.Diff.Rows)
	}
	matches := make([]searchMatch, 0)
	for row := 0; row < rows; row++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := ""
		switch {
		case hex:
			line = doc.HexLine(row)
		case screen == "patch" && data != nil && data.Diff != nil:
			diffRow := data.Diff.Rows[row]
			if diffRow.Summary != "" {
				line = terminal.Sanitize(diffRow.Summary)
			} else {
				line = doc.SearchableLine(diffRow.RawLine)
			}
		default:
			line = doc.SearchableLine(row)
		}
		if terminal.Contains(query, line) {
			matches = append(matches, searchMatch{row: row, order: row, cursor: row, column: terminal.MatchColumn(line, query)})
		}
	}
	return matches, nil
}

func (m *Model) searchCursorOrder() int {
	switch m.screen {
	case "examples":
		return m.overviewPosition
	case "inventory":
		if m.data != nil && m.inventory >= 0 && m.inventory < len(m.data.InventoryPositions) {
			return m.data.InventoryPositions[m.inventory]
		}
		return 0
	case "activity":
		return m.activityIndex
	default:
		return m.top
	}
}

func (m *Model) applySearchMatch(index int) tea.Cmd {
	if index < 0 || index >= len(m.searchMatches) {
		return nil
	}
	match := m.searchMatches[index]
	m.searchCurrent = index
	switch m.screen {
	case "examples":
		m.selectOverviewPosition(match.cursor)
		return m.startOverviewPreview()
	case "inventory":
		m.inventory = min(max(match.cursor, 0), max(len(m.data.Inventory)-1, 0))
	case "activity":
		m.activityIndex = min(max(match.cursor, 0), max(len(m.activity)-1, 0))
	default:
		m.top = min(max(match.cursor, 0), max(m.documentRows()-1, 0))
		m.left = match.column
	}
	return nil
}

func (m *Model) moveSearch(delta int) tea.Cmd {
	if len(m.searchMatches) == 0 {
		return nil
	}
	index := m.searchCurrent + delta
	if index < 0 {
		index = len(m.searchMatches) - 1
	} else if index >= len(m.searchMatches) {
		index = 0
	}
	return m.applySearchMatch(index)
}

func (m *Model) searchMatchesRow(row int) bool {
	index := sort.Search(len(m.searchMatches), func(i int) bool { return m.searchMatches[i].row >= row })
	return index < len(m.searchMatches) && m.searchMatches[index].row == row
}

func (m *Model) searchStatus() string {
	if m.searchEditing {
		return "/" + m.searchQuery
	}
	if m.searchPending {
		return "Searching for /" + m.searchQuery
	}
	if m.searchQuery == "" {
		return ""
	}
	if len(m.searchMatches) == 0 {
		return "no matches"
	}
	return fmt.Sprintf("match %d of %d", m.searchCurrent+1, len(m.searchMatches))
}

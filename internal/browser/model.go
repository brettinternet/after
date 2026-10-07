package browser

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
)

// Job persists its own engine results before returning IDs. Cancellation or a
// stale UI completion must never erase those records. No job runs on selection.
type Job func(context.Context) (string, error)
type Jobs struct {
	Capture, Import Job
	Actions         *Actions
}
type loaded struct {
	request uint64
	data    *Data
	err     error
}
type documentReady struct {
	request uint64
	doc     *terminal.Document
	err     error
}
type finished struct {
	request uint64
	result  string
	err     error
}

type Model struct {
	loopState
	selected                             Selection
	data                                 *Data
	jobs                                 Jobs
	ctx, parent                          context.Context
	cancel                               context.CancelFunc
	workers                              sync.WaitGroup
	jobCancel                            context.CancelFunc
	jobID, request, loadID               uint64
	busy                                 bool
	width, height                        int
	screen                               string
	returnTo, helpFrom                   string
	targetDiffPath                       string
	index, inventory, section, top, left int
	doc                                  *terminal.Document
	hex                                  bool
	status                               string
	theme                                terminal.Theme
	now                                  func() time.Time
	zone                                 *time.Location
}

func New(ctx context.Context, selected Selection, jobs Jobs) *Model {
	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	selected.Evidence = append(selected.Evidence[:0:0], selected.Evidence...)
	return &Model{theme: terminal.DefaultTheme(), now: time.Now, zone: time.Local, selected: selected, jobs: jobs, ctx: ctx, parent: parent, cancel: cancel, width: 80, height: 24, screen: "examples", status: "Loading immutable records; no project execution"}
}

// spawn starts ownership before returning a Bubble Tea command, so even a quit
// before command dispatch cannot race Close's join.
func (m *Model) spawn(work func() tea.Msg) tea.Cmd {
	ch := make(chan tea.Msg, 1)
	m.workers.Add(1)
	go func() { defer m.workers.Done(); ch <- work() }()
	return func() tea.Msg { return <-ch }
}
func (m *Model) Close() {
	m.cancel()
	m.workers.Wait()
	if m.jobs.Actions != nil {
		m.jobs.Actions.Close()
	}
}
func (m *Model) Init() tea.Cmd {
	m.loadID++
	id := m.loadID
	selected := m.selected
	return m.spawn(func() tea.Msg { d, err := Load(m.ctx, selected); return loaded{id, d, err} })
}
func (m *Model) secondaryRow() bool {
	if m.height <= 2 {
		return false
	}
	return m.screen == "help" || m.screen == "inspector" || m.screen == "plan" || (m.height >= 12 && (m.screen == "examples" || m.screen == "inventory" || m.screen == "patch"))
}
func (m *Model) bodyRows() int {
	_, jobLines := m.frameHeader()
	reserved := 1 + len(jobLines) // header and overflow indicators
	if m.secondaryRow() {
		reserved++
	}
	reserved++ // key hints
	if m.height >= 7 {
		reserved++ // next/status line
	}
	return max(m.height-reserved, 0)
}
func (m *Model) rows() int {
	rows := m.bodyRows()
	if m.screen == "examples" || m.screen == "inventory" {
		rows -= 2 // explanatory rows above the list
	}
	return max(rows, 1)
}
func (m *Model) textLimited() bool {
	return m.doc != nil && m.doc.Limited() || m.screen == "patch" && m.section == 0 && m.data != nil && m.data.Diff != nil && m.data.Diff.Limited
}

func (m *Model) contentRows() int {
	reserved := 2 // section metadata and section name
	if m.screen == "patch" && m.section == 0 && m.data != nil && m.data.Diff != nil {
		reserved++ // sticky current-file metadata
	}
	if m.textLimited() && !m.hex {
		reserved++
	}
	return max(m.bodyRows()-reserved, 1)
}
func (m *Model) entries() []Entry {
	if m.data == nil {
		return nil
	}
	if m.screen == "inventory" || (m.screen == "inspector" && m.returnTo == "inventory") {
		return m.data.Inventory
	}
	return m.data.Entries
}
func (m *Model) cursor() *int {
	if m.screen == "inventory" || (m.screen == "inspector" && m.returnTo == "inventory") {
		return &m.inventory
	}
	return &m.index
}
func (m *Model) sections() []Section {
	if m.screen == "plan" {
		if m.summaryUnavailable {
			return []Section{{Name: "Exact plan", Content: m.preview}}
		}
		return []Section{{Name: "Summary", Content: m.summary}, {Name: "Exact plan", Content: m.preview}}
	}
	if m.data == nil {
		return nil
	}
	if m.screen == "patch" {
		return []Section{m.data.Patch, m.data.Limits}
	}
	entries := m.entries()
	i := *m.cursor()
	if i >= len(entries) {
		return nil
	}
	return entries[i].Sections
}
func (m *Model) loadDocument() tea.Cmd {
	m.request++
	id := m.request
	m.doc = nil
	m.top = 0
	m.left = 0
	m.hex = false
	sections := m.sections()
	if len(sections) == 0 {
		return nil
	}
	section := sections[m.section]
	project := m.selected.Project
	return m.spawn(func() tea.Msg {
		raw, err := ReadSection(m.ctx, project, section)
		if err != nil {
			return documentReady{request: id, err: err}
		}
		display := displayJSON(raw, section.format)
		doc, err := terminal.NewDocumentView(display, raw)
		return documentReady{request: id, doc: doc, err: err}
	})
}
func (m *Model) startJob(name string, job Job) tea.Cmd {
	if m.data == nil {
		m.status = "Action unavailable until immutable records finish loading"
		return nil
	}
	if m.busy || job == nil {
		m.status = "Action unavailable or another job is active"
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.jobCancel = cancel
	m.busy = true
	m.jobID++
	id := m.jobID
	m.status = name + " running for selected immutable basis | x cancel; navigation remains available"
	return m.spawn(func() tea.Msg { result, err := job(ctx); cancel(); return finished{id, result, err} })
}
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.Paste {
			return m, nil
		}
		binding, found := keyBindingFor(key.String())
		if !found {
			return m, nil
		}
		if reason := m.keyReason(binding, false); reason != "" {
			m.status = "Can't " + binding.label + ": " + reason
			return m, nil
		}
		return m, m.dispatch(binding)
	}
	if cmd, handled := m.updateLoop(msg); handled {
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = min(max(msg.Width, 1), 240)
		m.height = min(max(msg.Height, 1), 100)
	case loaded:
		if msg.request != m.loadID {
			return m, nil
		}
		if msg.err != nil {
			m.status = "Capture unavailable; use after capture/inspect"
		} else {
			m.data = msg.data
			m.status = "Stored records only; no project execution"
		}
	case documentReady:
		if msg.request != m.request {
			return m, nil
		}
		if msg.err != nil || msg.doc == nil {
			unavailable := "Artifact unavailable; no conclusion. Use after inspect for this stored ID."
			m.doc, _ = terminal.NewDocument([]byte(unavailable))
			m.hex = false
			m.status = unavailable
		} else {
			m.doc = msg.doc
			m.hex = msg.doc.AutoHex()
			if m.screen == "patch" && m.targetDiffPath != "" && m.data != nil && m.data.Diff != nil {
				if fileIndex, ok := m.data.Diff.FileByPath[m.targetDiffPath]; ok && m.data.Diff.Files[fileIndex].StartRow >= 0 {
					m.top = m.data.Diff.Files[fileIndex].StartRow
				}
				m.targetDiffPath = ""
			}
			if m.textLimited() {
				m.status = "Text index limited at 250000 lines; b opens exact hex for every stored byte"
			}
		}
	case finished:
		if msg.request != m.jobID {
			return m, nil
		}
		m.busy = false
		m.jobCancel = nil
		m.status = "Job finished; stored result retained, selection unchanged"
		result := msg.result
		if msg.err != nil {
			m.status = "Job failed/cancelled; stored results retained if published"
			result += "\n" + msg.err.Error()
		}
		// Keep full IDs and errors in a data view, never a clipped-only toast.
		if m.data != nil {
			m.data.Entries = append(m.data.Entries, Entry{Summary: "job completion, not evidence", Name: "background result", Sections: []Section{{Name: "stored result IDs / diagnostic", Content: []byte(result)}}})
		}
	}
	m.top = max(0, m.top)
	return m, nil
}

func (m *Model) dispatch(binding keyBinding) tea.Cmd {
	if cmd, handled := m.dispatchLoop(binding.action); handled {
		return cmd
	}
	switch binding.action {
	case keyQuit:
		if m.screen == "plan" || m.screen == "help" && m.helpFrom == "plan" {
			m.invalidatePlan()
			m.screen, m.helpFrom = "examples", ""
			m.status = "Execution denied; no project execution"
		}
		m.cancel()
		return tea.Quit
	case keyHelp:
		m.request++
		if m.screen == "help" {
			m.screen, m.helpFrom = m.helpFrom, ""
		} else {
			m.helpFrom, m.screen = m.screen, "help"
		}
		m.top = 0
	case keyBack:
		m.request++
		m.top = 0
		if m.screen == "help" {
			m.screen, m.helpFrom = m.helpFrom, ""
		} else if m.screen == "inspector" {
			m.screen = m.returnTo
		} else {
			m.screen = "examples"
		}
	case keyOverview:
		return m.switchView(0)
	case keyChanges:
		return m.switchView(1)
	case keyDiff:
		return m.switchView(2)
	case keyNextFile, keyPreviousFile:
		delta := 1
		if binding.action == keyPreviousFile {
			delta = -1
		}
		m.navigateFile(delta)
	case keyNextHunk, keyPreviousHunk:
		delta := 1
		if binding.action == keyPreviousHunk {
			delta = -1
		}
		m.navigateHunk(delta)
	case keyNext, keyPrevious:
		if m.screen == "inspector" || m.screen == "plan" {
			count := len(m.sections())
			delta := 1
			if binding.action == keyPrevious {
				delta = -1
			}
			m.section = (m.section + delta + count) % count
			return m.loadDocument()
		}
		current := map[string]int{"examples": 0, "inventory": 1, "patch": 2}[m.screen]
		count := 1
		if m.data != nil {
			count = 3
		}
		delta := 1
		if binding.action == keyPrevious {
			delta = -1
		}
		return m.switchView((current + delta + count) % count)
	case keyEnter:
		m.returnTo = m.screen
		m.screen = "inspector"
		m.section = 0
		return m.loadDocument()
	case keyPanLeft, keyPanRight:
		if m.isDocumentScreen() && m.doc != nil && !m.hex {
			delta := 16
			if binding.action == keyPanLeft {
				delta = -16
			}
			m.left = min(max(m.left+delta, 0), m.doc.MaxColumns())
		} else if binding.action == keyPanRight {
			m.left = min(m.left+16, 240)
		} else {
			m.left = max(m.left-16, 0)
		}
	case keyHex:
		m.hex = !m.hex
		m.top, m.left = 0, 0
	case keyDown, keyUp, keyPageDown, keyPageUp, keyStart, keyEnd:
		m.move(binding.action)
	case keyCapture:
		if m.jobs.Actions == nil {
			return m.startJob("Capture", m.jobs.Capture)
		}
	case keyImport:
		return m.startJob("Import", m.jobs.Import)
	}
	return nil
}

func (m *Model) switchView(view int) tea.Cmd {
	m.request++
	m.doc = nil
	m.top = 0
	m.targetDiffPath = ""
	if view == 2 && m.screen == "inventory" && m.data != nil && m.inventory >= 0 && m.inventory < len(m.data.Inventory) {
		m.targetDiffPath = m.data.Inventory[m.inventory].Name
	}
	switch view {
	case 0:
		m.screen = "examples"
		return nil
	case 1:
		m.screen = "inventory"
		return nil
	case 2:
		m.screen = "patch"
		m.section = 0
		if m.data != nil && m.data.Diff != nil && m.targetDiffPath != "" {
			if fileIndex, ok := m.data.Diff.FileByPath[m.targetDiffPath]; ok && m.data.Diff.Files[fileIndex].StartRow >= 0 {
				m.top = m.data.Diff.Files[fileIndex].StartRow
			}
		}
		return m.loadDocument()
	default:
		return nil
	}
}

func (m *Model) move(action keyAction) {
	delta := 1
	switch action {
	case keyUp:
		delta = -1
	case keyPageDown:
		delta = m.rows()
	case keyPageUp:
		delta = -m.rows()
	}
	switch m.screen {
	case "examples", "inventory":
		p := m.cursor()
		*p += delta
		if action == keyStart {
			*p = 0
		}
		if action == keyEnd {
			*p = len(m.entries()) - 1
		}
		*p = min(max(*p, 0), max(len(m.entries())-1, 0))
	case "help":
		lines := m.helpLines()
		m.top += delta
		if action == keyStart {
			m.top = 0
		}
		if action == keyEnd {
			m.top = len(lines) - 1
		}
		m.top = min(max(m.top, 0), max(len(lines)-m.bodyRows(), 0))
	default:
		if m.isDocumentScreen() && m.doc != nil {
			rows := m.contentRows()
			if action == keyPageDown {
				delta = rows
			} else if action == keyPageUp {
				delta = -rows
			}
			total := m.documentRows()
			m.top += delta
			if action == keyStart {
				m.top = 0
			}
			if action == keyEnd {
				m.top = max(total-rows, 0)
			}
			m.top = min(max(m.top, 0), max(total-rows, 0))
		}
	}
}

func (m *Model) isDocumentScreen() bool {
	return m.screen == "inspector" || m.screen == "patch" || m.screen == "plan"
}
func (m *Model) documentRows() int {
	if m.doc == nil {
		return 0
	}
	if m.hex {
		return m.doc.HexRows()
	}
	if m.screen == "patch" && m.section == 0 && m.data != nil && m.data.Diff != nil {
		return len(m.data.Diff.Rows)
	}
	return m.doc.Lines()
}
func window(raw string, left, width int) string { return terminal.LineAt(raw, left, width) }

func (m *Model) headerText() string {
	baseID, candidateID := shortID(m.selected.Pair.Base), shortID(m.selected.Pair.Candidate)
	if m.width < 60 {
		return fmt.Sprintf("AFTER · %s → %s", baseID, candidateID)
	}
	project := filepath.Base(filepath.Clean(m.selected.Project))
	if project == "." || project == string(filepath.Separator) || project == "" {
		project = "project"
	}
	baseSource, candidateSource := "source loading", "source loading"
	if m.data != nil {
		baseSource = snapshotSource(m.data.BaseSnapshot)
		candidateSource = snapshotSource(m.data.CandidateSnapshot)
	}
	return fmt.Sprintf("AFTER · %s · base %s (%s) → candidate %s (%s)", project, baseID, baseSource, candidateID, candidateSource)
}

func (m *Model) frameHeader() (string, []string) {
	header := m.headerText()
	if m.height <= 2 {
		return header, nil
	}
	indicators := []string{}
	if m.capturing {
		indicators = append(indicators, "capturing")
	}
	if m.pending != nil {
		indicators = append(indicators, "new capture "+shortID(m.pending.Candidate)+" · u")
	}
	if m.running {
		indicators = append(indicators, "running "+elapsed(m.now().Sub(m.runStarted))+" · x")
	}
	extra := []string{}
	for _, indicator := range indicators {
		candidate := header + " · " + indicator
		if uniseg.StringWidth(candidate) <= m.width {
			header = candidate
		} else {
			extra = append(extra, indicator)
		}
	}
	return header, extra
}

func snapshotSource(snapshot evidence.Snapshot) string {
	switch snapshot.Source {
	case evidence.Commit:
		if snapshot.Unborn || snapshot.Commit == "" {
			return "commit unborn"
		}
		return "commit " + commitShort(snapshot.Commit)
	case evidence.WorkingTree:
		return "working tree"
	case evidence.Index:
		return "staged"
	case evidence.MergeBase:
		return "merge base " + commitShort(snapshot.MergeBase)
	default:
		return "source unavailable"
	}
}

func commitShort(commit string) string {
	return commit[:min(7, len(commit))]
}

func elapsed(duration time.Duration) string {
	seconds := max(int(duration.Seconds()), 0)
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func (m *Model) tabBar() string {
	labels := []string{"1 Overview", fmt.Sprintf("2 Changes %d", lenInventory(m.data)), "3 Diff"}
	if m.width < 60 {
		labels = []string{"1 Ov", fmt.Sprintf("2 Ch %d", lenInventory(m.data)), "3 Df"}
	}
	active := map[string]int{"examples": 0, "inventory": 1, "patch": 2}[m.screen]
	count := 1
	if m.data != nil {
		count = len(labels)
	}
	var line strings.Builder
	used := 0
	for index := 0; index < count; index++ {
		label := labels[index]
		if index == active {
			label = "[" + label + "]"
		}
		separator := ""
		if used > 0 {
			separator = "   "
		}
		width := uniseg.StringWidth(label)
		sepWidth := uniseg.StringWidth(separator)
		if used+sepWidth+width > m.width {
			break
		}
		line.WriteString(separator)
		style := terminal.Plain
		if index == active {
			style = terminal.Reverse
		}
		line.WriteString(m.theme.Render(label, width, style, false))
		used += sepWidth + width
	}
	return line.String()
}

func (m *Model) planBreadcrumb() string {
	const title = "Run this exact plan?"
	var line strings.Builder
	remaining := m.width
	appendStyled := func(text string, style terminal.Style) bool {
		if remaining == 0 {
			return false
		}
		width := min(uniseg.StringWidth(text), remaining)
		line.WriteString(m.theme.Render(text, width, style, false))
		remaining -= width
		return width == uniseg.StringWidth(text)
	}
	if !appendStyled(title, terminal.Strong) {
		return line.String()
	}
	for index, section := range m.sections() {
		if !appendStyled("  ", terminal.Plain) {
			break
		}
		label := section.Name
		if section.Name == "Exact plan" {
			label += " " + previewSize(len(m.preview))
		}
		style := terminal.Plain
		if index == m.section {
			label = "[" + label + "]"
			style = terminal.Reverse
		}
		if !appendStyled(label, style) {
			break
		}
	}
	return line.String()
}

func (m *Model) breadcrumb() string {
	if m.screen == "help" {
		return m.theme.Render("Help", m.width, terminal.Strong, false)
	}
	if m.screen == "plan" {
		return m.planBreadcrumb()
	}
	if m.screen != "inspector" {
		return ""
	}
	root := "Overview"
	if m.returnTo == "inventory" {
		root = "Changes"
	}
	entries := m.entries()
	i := *m.cursor()
	if i >= len(entries) {
		return m.theme.Render(root, m.width, terminal.Strong, false)
	}
	entry := entries[i]
	prefix := root + " › " + entry.Name
	b := badgeFor(entry)
	badgeText := "[" + b.word + "]"
	badgeWidth := uniseg.StringWidth(badgeText)
	prefixWidth := max(m.width-badgeWidth-1, 0)
	if prefixWidth == 0 {
		return m.theme.Render(prefix, m.width, terminal.Strong, false)
	}
	safePrefix := terminal.Line(prefix, prefixWidth)
	used := uniseg.StringWidth(safePrefix)
	if used+1+badgeWidth > m.width {
		return m.theme.Render(prefix, m.width, terminal.Strong, false)
	}
	return m.theme.Render(safePrefix, prefixWidth, terminal.Strong, true) + " " + m.theme.Render(badgeText, badgeWidth, b.style, false)
}

func lenInventory(data *Data) int {
	if data == nil {
		return 0
	}
	return len(data.Inventory)
}

func (m *Model) statusLine() string {
	if m.pending != nil {
		if strings.HasPrefix(m.status, "New capture ") {
			return "New capture " + shortID(m.pending.Candidate) + " — u reviews it"
		}
		return m.status + " · new capture " + shortID(m.pending.Candidate) + " · u"
	}
	if m.running {
		return "Running the approved plan · " + elapsed(m.now().Sub(m.runStarted)) + " · x cancels"
	}
	return m.status
}

func (m *Model) View() string {
	if m.height == 1 {
		return terminal.Line(m.keyHints(), m.width)
	}
	header, indicators := m.frameHeader()
	lines := []string{m.theme.Render(header, m.width, terminal.Strong, false)}
	for _, indicator := range indicators {
		lines = append(lines, m.theme.Render(indicator, m.width, terminal.Attention, false))
	}
	if m.secondaryRow() {
		if m.screen == "examples" || m.screen == "inventory" || m.screen == "patch" {
			lines = append(lines, m.tabBar())
		} else {
			lines = append(lines, m.breadcrumb())
		}
	}
	body := []string{}
	add := func(s string) { body = append(body, terminal.Line(s, m.width)) }
	data := func(s string) {
		prefix := "data | "
		if m.width <= len(prefix) {
			add(prefix)
			return
		}
		add(prefix + window(s, m.left, m.width-len(prefix)))
	}
	switch m.screen {
	case "help":
		help := m.helpLines()
		top := min(m.top, max(len(help)-1, 0))
		for _, line := range help[top:min(top+m.bodyRows(), len(help))] {
			add(window(line, m.left, m.width))
		}
	case "examples":
		entries := m.entries()
		i := *m.cursor()
		if len(entries) == 0 {
			add("Not checked: no evidence loaded")
		} else {
			add("Badges: finite evidence only · Enter for full state and scope")
			top := max(0, i-m.rows()+1)
			for n := top; n < min(len(entries), top+m.rows()); n++ {
				body = append(body, m.entryLine(entries[n], n == i))
			}
		}
	case "inventory":
		body = append(body, m.inventoryBody()...)
	case "inspector", "patch", "plan":
		sections := m.sections()
		if len(sections) > 0 {
			section := sections[m.section]
			total := 0
			if m.doc != nil {
				total = m.doc.RawLength()
			}
			mode := "text"
			if m.hex {
				mode = "hex"
			}
			add(fmt.Sprintf("Section %d/%d | %d stored bytes | %s | pan %d", m.section+1, len(sections), total, mode, m.left))
			data(section.Name)
			if m.screen == "plan" && m.summaryUnavailable {
				add("Summary unavailable; exact preview bytes remain unchanged")
			}
			if m.screen == "patch" && m.section == 0 && m.data != nil && m.data.Diff != nil {
				body = append(body, m.diffStickyHeader())
			}
			if m.doc == nil {
				add("Loading complete document off the event loop")
			} else {
				if m.textLimited() && !m.hex {
					add("TEXT LIMITED at 250000 lines; b opens exact hex for every stored byte")
				}
				if m.doc.Lines() == 0 && m.doc.HexRows() == 0 {
					add("(empty captured bytes; not an equality claim)")
				} else {
					count := m.documentRows()
					for n := m.top; n < min(m.top+m.contentRows(), count); n++ {
						if m.hex {
							add(m.doc.HexLine(n))
							continue
						}
						if m.screen == "patch" && m.section == 0 && m.data != nil && m.data.Diff != nil {
							body = append(body, m.diffDocumentRow(n))
							continue
						}
						digits := len(strconv.Itoa(max(m.doc.Lines(), 1)))
						gutter := fmt.Sprintf("%*d │ ", digits, n+1)
						gutterWidth := uniseg.StringWidth(gutter)
						if m.width <= gutterWidth {
							body = append(body, terminal.Line(gutter, m.width))
							continue
						}
						available := m.width - gutterWidth
						marker := ""
						if m.doc.LongLine(n) && available >= 4 {
							marker = "[b]"
							available -= len(marker)
						}
						row := m.doc.LineAt(n, m.left, available) + marker
						body = append(body, gutter+row)
					}
				}
			}
		}
	}
	footer := []string{}
	if m.height >= 7 {
		footer = append(footer, terminal.Line(m.statusLine(), m.width))
	}
	footer = append(footer, terminal.Line(m.keyHints(), m.width))
	available := max(m.height-len(lines)-len(footer), 0)
	lines = append(lines, body[:min(len(body), available)]...)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}
func Run(m *Model, input io.Reader, output io.Writer) error {
	defer m.Close()
	_, err := tea.NewProgram(m, tea.WithInput(input), tea.WithOutput(output), tea.WithContext(m.parent), tea.WithAltScreen()).Run()
	return err
}

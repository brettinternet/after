package browser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
)

// Job persists its own engine results before returning IDs. Cancellation or a
// stale UI completion must never erase those records. No job runs on selection.
type Job func(context.Context, Selection) (string, error)
type Jobs struct {
	Capture, Import Job
	Actions         *Actions
	CaptureOnStart  bool
}
type loaded struct {
	request uint64
	data    *Data
	err     error
}
type documentReady struct {
	request  uint64
	doc      *terminal.Document
	dividers []int
	err      error
}
type previewReady struct {
	request  uint64
	key      string
	doc      *terminal.Document
	dividers []int
	err      error
}
type finished struct {
	request   uint64
	kind      string
	selection Selection
	result    string
	err       error
}
type importedLoaded struct {
	request   uint64
	selection Selection
	data      *Data
	err       error
}

type Model struct {
	loopState
	selected                             Selection
	session                              ReviewSession
	persistSession                       func(ReviewSession) error
	data                                 *Data
	jobs                                 Jobs
	ctx, parent                          context.Context
	cancel                               context.CancelFunc
	workers                              sync.WaitGroup
	jobCancel, captureCancel             context.CancelFunc
	searchCancel                         context.CancelFunc
	jobID, request, loadID               uint64
	captureStarted                       time.Time
	busy                                 bool
	busyKind                             string
	busyStarted                          time.Time
	clockScheduled                       bool
	activity                             []ActivityEvent
	activityDropped, activityIndex       int
	sessionActivityDetail                string
	quitConfirm                          bool
	loadFailed                           bool
	width, height                        int
	screen                               string
	returnTo, helpFrom                   string
	inspectInventory                     bool
	targetDiffPath                       string
	index, inventory, section, top, left int
	overviewPosition                     int
	overviewCollapsedGroups              map[overviewGroup]bool
	previewRequest                       uint64
	previewKey                           string
	previewDoc                           *terminal.Document
	previewDividers                      map[int]bool
	doc                                  *terminal.Document
	dividerRows                          map[int]bool
	hex                                  bool
	status                               string
	searchQuery                          string
	searchEditing, searchPending         bool
	searchRequest                        uint64
	searchCurrent                        int
	searchMatches                        []searchMatch
	prompt                               *mutationPrompt
	theme                                terminal.Theme
	now                                  func() time.Time
	zone                                 *time.Location
}

func New(ctx context.Context, selected Selection, jobs Jobs) *Model {
	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	selected.Evidence = append(selected.Evidence[:0:0], selected.Evidence...)
	selected.PinRevisions = append([]evidence.Digest(nil), selected.PinRevisions...)
	if selected.Mode != evidence.FollowUp {
		selected.Mode = evidence.OriginalBase
	}
	if selected.Baseline == "" {
		selected.Baseline = selected.Pair.Base
	}
	session := ReviewSession{SchemaVersion: ReviewSessionVersion, Pair: selected.Pair, Mode: selected.Mode, Baseline: selected.Baseline, PinRevisions: append([]evidence.Digest(nil), selected.PinRevisions...)}
	return &Model{theme: terminal.DefaultTheme(), now: time.Now, zone: time.Local, selected: selected, session: session, jobs: jobs, ctx: ctx, parent: parent, cancel: cancel, width: 80, height: 24, screen: "examples"}
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
	commands := []tea.Cmd{m.spawn(func() tea.Msg { d, err := Load(m.ctx, selected); return loaded{id, d, err} })}
	if m.jobs.CaptureOnStart && m.jobs.Actions != nil {
		m.capturing = true
		m.captureStarted = m.now()
		m.recordActivity("capture started", "capturing the working tree", []evidence.Digest{selected.Pair.Base, selected.Pair.Candidate}, "")
		ctx, cancel := context.WithCancel(m.ctx)
		m.captureCancel = cancel
		commands = append(commands, m.spawn(func() tea.Msg {
			pair, err := m.jobs.Actions.Capture(ctx)
			return snapshotReady{pair, err}
		}))
		if cmd := m.scheduleClock(); cmd != nil {
			commands = append(commands, cmd)
		}
	}
	if m.activeClock() {
		if cmd := m.scheduleClock(); cmd != nil {
			commands = append(commands, cmd)
		}
	}
	return tea.Batch(commands...)
}

// SetReviewSession supplies the validated persisted state and its writer. The
// callback is invoked only when the selected snapshot pair changes.
func (m *Model) SetReviewSession(session ReviewSession, persist func(ReviewSession) error) {
	if session.Baseline == "" && session.Mode == evidence.OriginalBase {
		session.Baseline = session.Pair.Base
	}
	m.session = session
	m.session.Capture.IncludeUntracked = append([]string(nil), session.Capture.IncludeUntracked...)
	m.session.PinRevisions = append([]evidence.Digest(nil), session.PinRevisions...)
	m.selected.Mode = session.Mode
	m.selected.Baseline = session.Baseline
	m.selected.PinRevisions = append([]evidence.Digest(nil), session.PinRevisions...)
	m.persistSession = persist
}

func (m *Model) ReviewSession() ReviewSession {
	session := m.session
	session.Capture.IncludeUntracked = append([]string(nil), m.session.Capture.IncludeUntracked...)
	session.PinRevisions = append([]evidence.Digest(nil), m.session.PinRevisions...)
	return session
}
func (m *Model) secondaryRow() bool {
	if m.height <= 2 {
		return false
	}
	return m.screen == "help" || m.screen == "inspector" || m.screen == "plan" || m.screen == "prompt" || (m.height >= 12 && (m.screen == "examples" || m.screen == "inventory" || m.screen == "patch" || m.screen == "activity"))
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
	if m.screen == "activity" {
		return m.activityRows()
	}
	if m.screen == "inventory" {
		rows -= 2 // inventory summary and one fixed heading row
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
	if m.screen == "inventory" || (m.screen == "inspector" && (m.returnTo == "inventory" || m.returnTo == "examples" && m.inspectInventory)) {
		return m.data.Inventory
	}
	return m.data.Entries
}
func (m *Model) cursor() *int {
	if m.screen == "inventory" || (m.screen == "inspector" && (m.returnTo == "inventory" || m.returnTo == "examples" && m.inspectInventory)) {
		return &m.inventory
	}
	return &m.index
}
func (m *Model) sections() []Section {
	if m.screen == "inspector" && m.returnTo == "activity" {
		index := len(m.activity) - 1 - m.activityIndex
		if index >= 0 && index < len(m.activity) {
			return activityDetail(m.activity[index])
		}
		return nil
	}
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
	m.clearSearch()
	m.request++
	id := m.request
	m.doc = nil
	m.top = 0
	m.left = 0
	m.hex = false
	m.dividerRows = nil
	sections := m.sections()
	if len(sections) == 0 {
		return nil
	}
	section := sections[m.section]
	project, width := m.selected.Project, m.width
	return m.spawn(func() tea.Msg {
		view, err := readSectionDocument(m.ctx, project, section, width)
		if err != nil {
			return documentReady{request: id, err: err}
		}
		doc, err := terminal.NewDocumentView(view.display, view.raw)
		return documentReady{request: id, doc: doc, dividers: view.dividers, err: err}
	})
}
func (m *Model) startJob(name, kind string, job Job) tea.Cmd {
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
	m.busyKind = kind
	m.busyStarted = m.now()
	m.jobID++
	id, selected := m.jobID, m.selected
	selected.Evidence = append([]evidence.Digest(nil), selected.Evidence...)
	m.recordActivity(kind+" started", strings.ToLower(name)+" started for selected candidate", []evidence.Digest{selected.Pair.Candidate}, "")
	m.status = name + " running for selected immutable basis | x cancel; navigation remains available"
	cmd := m.spawn(func() tea.Msg {
		result, err := job(ctx, selected)
		cancel()
		return finished{request: id, kind: kind, selection: selected, result: result, err: err}
	})
	if tick := m.scheduleClock(); tick != nil {
		return tea.Batch(cmd, tick)
	}
	return cmd
}
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if m.prompt != nil {
			return m.updatePromptKey(key)
		}
		m.status = ""
		if cmd, consumed := m.updateSearchKey(key); consumed {
			return m, cmd
		}
		if key.Paste {
			return m, nil
		}
		if key.String() == "esc" && m.searchQuery != "" {
			m.clearSearch()
			m.status = "Search cleared"
			return m, nil
		}
		if key.String() == "ctrl+c" {
			m.quitConfirm = false
			m.quitNow()
			return m, tea.Quit
		}
		if m.quitConfirm {
			switch key.String() {
			case "y", "enter":
				m.quitConfirm = false
				m.quitNow()
				return m, tea.Quit
			case "n", "esc":
				m.quitConfirm = false
				m.status = "Run continues; q asks again and Ctrl-C quits now"
			}
			return m, nil
		}
		binding, found := keyBindingForContext(key.String(), m.screen)
		if !found {
			return m, nil
		}
		if reason := m.keyReason(binding, false); reason != "" {
			m.status = "Can't " + binding.label + ": " + reason
			m.recordActivity("action unavailable", binding.label, []evidence.Digest{m.selected.Pair.Base, m.selected.Pair.Candidate}, m.status)
			return m, nil
		}
		return m, m.dispatch(binding)
	}
	if cmd, handled := m.updateLoop(msg); handled {
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		previousWidth := m.width
		m.width = min(max(msg.Width, 1), 240)
		m.height = min(max(msg.Height, 1), 100)
		if previousWidth != m.width && m.screen == "examples" {
			return m, m.startOverviewPreview()
		}
	case loaded:
		if msg.request != m.loadID {
			return m, nil
		}
		if msg.err != nil {
			m.loadFailed = true
			m.status = ""
		} else {
			m.loadFailed = false
			m.data = msg.data
			m.selected = msg.data.Selection
			m.status = ""
			m.selectFirstOverviewRow()
			return m, m.startOverviewPreview()
		}
	case searchReady:
		if msg.request != m.searchRequest || msg.docRequest != m.request || msg.screen != m.screen || msg.query != m.searchQuery {
			return m, nil
		}
		if m.searchCancel != nil {
			m.searchCancel()
			m.searchCancel = nil
		}
		m.searchPending = false
		if msg.err != nil {
			m.searchMatches = nil
			m.searchCurrent = -1
			m.status = "Search cancelled or unavailable"
			return m, nil
		}
		m.searchMatches = msg.matches
		if len(m.searchMatches) == 0 {
			m.searchCurrent = -1
			m.status = "no matches"
			return m, nil
		}
		m.searchCurrent = sort.Search(len(m.searchMatches), func(i int) bool { return m.searchMatches[i].order > msg.cursor })
		if m.searchCurrent >= len(m.searchMatches) {
			m.searchCurrent = 0
		}
		return m, m.applySearchMatch(m.searchCurrent)
	case previewReady:
		if msg.request != m.previewRequest || msg.key != m.previewKey {
			return m, nil
		}
		if msg.err != nil || msg.doc == nil {
			m.previewDoc = nil
			m.previewDividers = nil
		} else {
			m.previewDoc = msg.doc
			m.previewDividers = make(map[int]bool, len(msg.dividers))
			for _, row := range msg.dividers {
				m.previewDividers[row] = true
			}
		}
	case documentReady:
		if msg.request != m.request {
			return m, nil
		}
		if msg.err != nil || msg.doc == nil {
			unavailable := "Artifact unavailable; no conclusion. Use after inspect for this stored ID."
			m.doc, _ = terminal.NewDocument([]byte(unavailable))
			m.dividerRows = nil
			m.hex = false
			m.status = unavailable
		} else {
			m.doc = msg.doc
			m.dividerRows = make(map[int]bool, len(msg.dividers))
			for _, row := range msg.dividers {
				m.dividerRows[row] = true
			}
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
		m.busyKind = ""
		m.busyStarted = time.Time{}
		m.jobCancel = nil
		if msg.err != nil {
			m.status = "Job failed/cancelled; stored results retained if published"
			failure := msg.kind + " failed"
			summary := "operation failed; any published result remains stored"
			if errors.Is(msg.err, context.Canceled) {
				failure, summary = msg.kind+" cancelled", "operation cancelled; any published result remains stored"
			}
			m.recordActivity(failure, summary, []evidence.Digest{msg.selection.Pair.Candidate}, msg.err.Error())
			return m, nil
		}
		if msg.kind == "import" {
			id := evidence.Digest(msg.result)
			ids := []evidence.Digest{id, msg.selection.Pair.Candidate}
			if containsDigest(m.selected.Evidence, id) {
				m.recordActivity("import finished", "report stored and already loaded", ids, "")
				m.status = "Report stored and loaded · s Activity"
				return m, nil
			}
			if len(m.selected.Evidence) >= MaxEvidence {
				m.recordActivity("import finished", "report stored; 32 evidence IDs already loaded", ids, "report remains stored but was not added to the loaded evidence")
				m.status = "Report stored; 32 evidence IDs already loaded · s Activity"
				return m, nil
			}
			m.recordActivity("import finished", "report stored and added to loaded evidence", ids, "")
			selected := m.selected
			selected.Discover = false
			selected.Evidence = append(append([]evidence.Digest(nil), selected.Evidence...), id)
			m.loadID++
			request := m.loadID
			m.status = "Report stored; loading its reported cards"
			return m, m.spawn(func() tea.Msg {
				data, err := Load(m.ctx, selected)
				return importedLoaded{request: request, selection: selected, data: data, err: err}
			})
		}
		m.status = "Capture finished; selected pair unchanged · s Activity"
		if msg.kind == "capture" {
			m.recordActivity("capture finished", "capture completed; review the new candidate with u", []evidence.Digest{msg.selection.Pair.Candidate}, msg.result)
		} else {
			m.recordActivity(msg.kind+" finished", "operation finished; stored result retained", []evidence.Digest{msg.selection.Pair.Candidate, evidence.Digest(msg.result)}, "")
		}
	case importedLoaded:
		if msg.request != m.loadID {
			return m, nil
		}
		if msg.err != nil {
			m.status = "Report remains stored; its cards could not be loaded · s Activity"
			m.recordActivity("import load failed", "stored report cards could not be loaded", []evidence.Digest{msg.selection.Pair.Candidate}, msg.err.Error())
			return m, nil
		}
		m.clearSearch()
		m.selected, m.data = msg.data.Selection, msg.data
		m.index = max(len(m.data.Entries)-1, 0)
		m.positionOverviewEntry(m.index)
		m.status = "Imported report cards loaded without restart · s Activity"
		if m.screen == "inspector" {
			m.screen = "examples"
			m.returnTo = ""
			m.doc = nil
		}
		m.request++
		m.loadID++
		if m.screen == "patch" {
			return m, m.loadDocument()
		}
		return m, m.startOverviewPreview()
	}
	m.top = max(0, m.top)
	return m, nil
}

func (m *Model) quitNow() {
	if m.screen == "plan" || m.screen == "help" && m.helpFrom == "plan" {
		m.recordActivity("plan denied", "quitting denied the prepared plan; no project execution", []evidence.Digest{m.planPair.Base, m.planPair.Candidate}, "")
		m.invalidatePlan()
		m.screen, m.helpFrom = "examples", ""
		m.status = "Execution denied; no project execution"
	}
	m.cancel()
}

func (m *Model) dispatch(binding keyBinding) tea.Cmd {
	if cmd, handled := m.dispatchLoop(binding.action); handled {
		return cmd
	}
	switch binding.action {
	case keyQuit:
		if m.running {
			m.quitConfirm = true
			m.status = "Confirm quit? y cancels the run and quits · n keeps running"
			return nil
		}
		m.quitNow()
		return tea.Quit
	case keyHelp:
		m.clearSearch()
		m.request++
		if m.screen == "help" {
			m.screen, m.helpFrom = m.helpFrom, ""
		} else {
			m.helpFrom, m.screen = m.screen, "help"
		}
		m.top = 0
	case keyBack:
		m.clearSearch()
		m.request++
		m.top = 0
		if m.screen == "help" {
			m.screen, m.helpFrom = m.helpFrom, ""
		} else if m.screen == "inspector" {
			m.screen = m.returnTo
		} else {
			m.screen = "examples"
		}
		if m.screen == "examples" {
			return m.startOverviewPreview()
		}
	case keySearch:
		m.openSearch()
	case keyNextMatch:
		return m.moveSearch(1)
	case keyPreviousMatch:
		return m.moveSearch(-1)
	case keyOverview:
		return m.switchView(0)
	case keyChanges:
		return m.switchView(1)
	case keyDiff:
		return m.switchView(2)
	case keyActivity:
		return m.switchView(3)
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
		current := map[string]int{"examples": 0, "inventory": 1, "patch": 2, "activity": 3}[m.screen]
		count := 1
		if m.data != nil {
			count = 4
		}
		delta := 1
		if binding.action == keyPrevious {
			delta = -1
		}
		return m.switchView((current + delta + count) % count)
	case keyEnter:
		if m.screen == "help" {
			m.screen, m.helpFrom = m.helpFrom, ""
		}
		if m.screen == "examples" {
			row, ok := m.currentOverviewRow()
			if !ok {
				return nil
			}
			if row.kind == overviewGroupHeader {
				m.clearSearch()
				m.toggleOverviewGroup()
				return m.startOverviewPreview()
			}
			m.inspectInventory = row.kind == overviewInventory
		}
		m.returnTo = m.screen
		m.screen = "inspector"
		m.startOverviewPreview()
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
		m.clearSearch()
		m.hex = !m.hex
		m.top, m.left = 0, 0
	case keyDown, keyUp, keyPageDown, keyPageUp, keyStart, keyEnd:
		return m.move(binding.action)
	case keyCapture:
		if m.jobs.Actions == nil {
			return m.startJob("Capture", "capture", m.jobs.Capture)
		}
	case keyImport:
		return m.startJob("Import", "import", m.jobs.Import)
	}
	return nil
}

func (m *Model) switchView(view int) tea.Cmd {
	m.clearSearch()
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
		return m.startOverviewPreview()
	case 1:
		m.screen = "inventory"
		m.startOverviewPreview()
		return nil
	case 2:
		m.screen = "patch"
		m.startOverviewPreview()
		m.section = 0
		if m.data != nil && m.data.Diff != nil && m.targetDiffPath != "" {
			if fileIndex, ok := m.data.Diff.FileByPath[m.targetDiffPath]; ok && m.data.Diff.Files[fileIndex].StartRow >= 0 {
				m.top = m.data.Diff.Files[fileIndex].StartRow
			}
		}
		return m.loadDocument()
	case 3:
		m.screen = "activity"
		m.startOverviewPreview()
		return nil
	default:
		return nil
	}
}

func (m *Model) move(action keyAction) tea.Cmd {
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
	case "activity":
		m.activityIndex += delta
		if action == keyStart {
			m.activityIndex = 0
		}
		if action == keyEnd {
			m.activityIndex = len(m.activity) - 1
		}
		m.activityIndex = min(max(m.activityIndex, 0), max(len(m.activity)-1, 0))
	case "examples":
		position := m.overviewPosition + delta
		if action == keyStart {
			position = 0
		}
		if action == keyEnd {
			position = len(m.overviewRows()) - 1
		}
		m.selectOverviewPosition(position)
		return m.startOverviewPreview()
	case "inventory":
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
	return nil
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

func padStyledLine(raw string, width int) string {
	var visible strings.Builder
	for i := 0; i < len(raw); {
		if raw[i] == '\x1b' {
			if end := strings.IndexByte(raw[i:], 'm'); end >= 0 {
				i += end + 1
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(raw[i:])
		if size == 0 {
			break
		}
		visible.WriteString(raw[i : i+size])
		i += size
	}
	missing := width - uniseg.StringWidth(visible.String())
	if missing > 0 {
		return raw + strings.Repeat(" ", missing)
	}
	return raw
}

func (m *Model) headerText() string {
	baseID, candidateID := shortID(m.selected.Pair.Base), shortID(m.selected.Pair.Candidate)
	mode := reviewModeLabel(m.selected.Mode)
	if m.width < 60 {
		return fmt.Sprintf("AFTER · %s %s → %s", mode, baseID, candidateID)
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
	return fmt.Sprintf("AFTER · %s · %s %s (%s) → candidate %s (%s)", project, mode, baseID, baseSource, candidateID, candidateSource)
}

func (m *Model) frameHeader() (string, []string) {
	header := m.headerText()
	if m.height <= 2 {
		return header, nil
	}
	indicators := []string{}
	if m.capturing {
		indicators = append(indicators, "capturing "+elapsed(m.now().Sub(m.captureStarted)))
	}
	if m.busy {
		indicators = append(indicators, strings.ToLower(m.busyKind)+" "+elapsed(m.now().Sub(m.busyStarted)))
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
	labels := []string{"1 Overview", fmt.Sprintf("2 Changes %d", lenInventory(m.data)), "3 Diff", "4 Activity"}
	if m.width < 60 {
		labels = []string{"1 Ov", fmt.Sprintf("2 Ch %d", lenInventory(m.data)), "3 Df", "4 Ac"}
	}
	active := map[string]int{"examples": 0, "inventory": 1, "patch": 2, "activity": 3}[m.screen]
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
	if m.screen == "prompt" && m.prompt != nil {
		return m.theme.Render(m.prompt.title, m.width, terminal.Strong, false)
	}
	if m.screen != "inspector" {
		return ""
	}
	root := "Overview"
	if m.returnTo == "inventory" {
		root = "Changes"
	} else if m.returnTo == "activity" {
		index := len(m.activity) - 1 - m.activityIndex
		if index >= 0 && index < len(m.activity) {
			return m.theme.Render("Activity › "+m.activity[index].Kind, m.width, terminal.Strong, false)
		}
		return m.theme.Render("Activity", m.width, terminal.Strong, false)
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

func (m *Model) overviewIsSelected() bool {
	return m.screen == "examples" || m.screen == "inspector" && m.returnTo == "examples" || m.screen == "help" && m.helpFrom == "examples"
}

func lenInventory(data *Data) int {
	if data == nil {
		return 0
	}
	return len(data.Inventory)
}

func (m *Model) statusLine() string {
	if m.status != "" && (m.quitConfirm || m.data == nil) {
		return m.status
	}
	if m.loadFailed {
		return "Couldn't load this pair — check the IDs with after inspect"
	}
	if m.data == nil && m.status != "" {
		return m.status
	}
	if m.data == nil {
		return "Loading stored records — nothing runs on open"
	}
	if m.screen == "prompt" {
		if m.status != "" {
			return m.status
		}
		return "Reason is recorded in pin history; no project execution"
	}
	if m.screen == "plan" {
		return "Nothing has run. y runs this exact plan once · n denies"
	}
	if searchStatus := m.searchStatus(); searchStatus != "" {
		return searchStatus
	}
	if m.running {
		return "Running the approved plan · " + elapsed(m.now().Sub(m.runStarted)) + " · x cancels (the incomplete result is kept)"
	}
	if m.pending != nil {
		return "New capture " + shortID(m.pending.Candidate) + " — u reviews it"
	}
	if m.capturing || m.busy || m.actionBusy {
		return ""
	}
	if m.status != "" {
		return m.status
	}
	if m.overviewIsSelected() {
		if entry := m.selectedOverviewEntry(); entry != nil && entry.Decision == evidence.Reopened && entry.MissingCurrentResult {
			return "Pin reopened: no result for this candidate yet — r previews a rerun"
		}
		if entry := m.selectedOverviewEntry(); entry != nil && entry.Decision != "" && entry.State.Applicability == evidence.Current && !entry.MissingCurrentResult {
			return "Current complete result attached — a accepts this pin"
		}
	}
	if len(m.data.Entries) == 0 {
		return "Not checked — read the change, or c captures again after editing"
	}
	return ""
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
		if m.screen == "examples" || m.screen == "inventory" || m.screen == "patch" || m.screen == "activity" {
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
	case "prompt":
		body = append(body, m.promptLines()...)
	case "help":
		help := m.helpLines()
		top := min(m.top, max(len(help)-1, 0))
		for _, line := range help[top:min(top+m.bodyRows(), len(help))] {
			add(window(line, m.left, m.width))
		}
	case "activity":
		for _, line := range m.activitySessionLines() {
			add(line)
		}
		add("ACTIVITY")
		if m.activityDropped > 0 {
			add(fmt.Sprintf("%d older activity events dropped", m.activityDropped))
		}
		if len(m.activity) == 0 {
			add("No Activity events yet")
		} else {
			top := max(0, m.activityIndex-m.activityRows()+1)
			for index := top; index < min(len(m.activity), top+m.activityRows()); index++ {
				line := m.activityLine(index, index == m.activityIndex)
				if m.searchMatchesRow(index) {
					line = m.theme.Highlight(line, m.searchQuery, m.width)
				}
				body = append(body, line)
			}
		}
	case "examples":
		rows := m.overviewRows()
		position := min(max(m.overviewPosition, 0), max(len(rows)-1, 0))
		top := max(0, position-m.rows()+1)
		if m.width >= 110 {
			listWidth := overviewListWidth(m.width)
			previewWidth := m.width - listWidth - 1
			for offset := 0; offset < m.rows(); offset++ {
				left := strings.Repeat(" ", listWidth)
				if index := top + offset; index < len(rows) {
					left = m.overviewRowTextWidth(rows[index], index == position, listWidth)
					if m.searchMatchesRow(index) {
						left = m.theme.Highlight(left, m.searchQuery, listWidth)
					}
					left = padStyledLine(left, listWidth)
				}
				right := padStyledLine(m.overviewPreviewLine(offset, previewWidth), previewWidth)
				body = append(body, left+"│"+right)
			}
		} else {
			for n := top; n < min(len(rows), top+m.rows()); n++ {
				line := m.overviewRowText(rows[n], n == position)
				if m.searchMatchesRow(n) {
					line = m.theme.Highlight(line, m.searchQuery, m.width)
				}
				body = append(body, line)
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
							line := m.doc.HexLine(n)
							if m.searchMatchesRow(n) {
								line = m.theme.Highlight(line, m.searchQuery, m.width)
							}
							add(line)
							continue
						}
						if m.dividerRows[n] {
							line := m.theme.Render(m.doc.LineAt(n, m.left, m.width), m.width, terminal.Strong, false)
							if m.searchMatchesRow(n) {
								line = m.theme.Highlight(line, m.searchQuery, m.width)
							}
							body = append(body, line)
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
						if m.searchMatchesRow(n) {
							row = m.theme.Highlight(row, m.searchQuery, available)
							body = append(body, gutter+row)
						} else {
							body = append(body, gutter+row)
						}
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

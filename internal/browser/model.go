package browser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
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
	return max(m.height-m.topRows()-m.footerRows(), 0)
}
func (m *Model) rows() int {
	rows := m.bodyRows()
	if m.screen == "activity" {
		return m.activityRows()
	}
	if m.screen == "inventory" {
		rows -= m.inventoryReserved()
	}
	return max(rows, 1)
}
func (m *Model) textLimited() bool {
	return m.doc != nil && m.doc.Limited() || m.screen == "patch" && m.section == 0 && m.data != nil && m.data.Diff != nil && m.data.Diff.Limited
}

func (m *Model) contentRows() int {
	reserved := 0
	if m.screen == "patch" && m.section == 0 && m.data != nil && m.data.Diff != nil {
		reserved++ // sticky current-file metadata
	}
	if m.textLimited() && !m.hex {
		reserved++
	}
	if m.screen == "plan" && m.summaryUnavailable {
		reserved++
	}
	if !m.hasSectionStrip() && m.screen != "patch" {
		reserved++ // compact section label
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
		return []Section{{Name: "Summary", Content: m.summary, Wrap: commandPlanPreview(m.preview)}, {Name: "Exact plan", Content: m.preview}}
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
	project, width := m.selected.Project, m.inner()
	// Diff rows index raw patch lines directly, so the patch document has no
	// heading divider that would shift every row against its gutter.
	barePatch := m.screen == "patch" && m.section == 0 && m.data != nil && m.data.Diff != nil
	return m.spawn(func() tea.Msg {
		if barePatch {
			raw, err := ReadSection(m.ctx, project, section)
			if err != nil {
				return documentReady{request: id, err: err}
			}
			doc, err := terminal.NewDocument(raw)
			return documentReady{request: id, doc: doc, err: err}
		}
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
		rows := m.overviewRows()
		position := m.overviewPosition + delta
		if action == keyStart {
			position, delta = 0, 1
		}
		if action == keyEnd {
			position, delta = len(rows)-1, -1
		}
		m.selectOverviewPosition(nearestSelectable(rows, position, delta))
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

// headerText is the plain text of the header segments; indicator placement
// measures it so styled and unstyled frames wrap identically.
func (m *Model) headerText() string {
	var text strings.Builder
	for _, part := range m.headerSegments() {
		text.WriteString(part.text)
	}
	return text.String()
}

func (m *Model) frameHeader() (string, []string) {
	header := m.headerText()
	width := m.inner()
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
		if uniseg.StringWidth(candidate) <= width {
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

func (m *Model) breadcrumb() string {
	width := m.inner()
	switch m.screen {
	case "help":
		return spread(m.theme.Render("Keys", width, terminal.Strong, false), m.theme.Render("Esc closes", width, terminal.Muted, false), width)
	case "plan":
		return m.theme.Render("Run this exact plan?", width, terminal.Attention, false)
	case "prompt":
		if m.prompt != nil {
			return m.theme.Render(m.prompt.title, width, terminal.Attention, false)
		}
		return ""
	case "inspector":
	default:
		return ""
	}
	root := "Overview"
	if m.returnTo == "inventory" {
		root = "Changes"
	} else if m.returnTo == "activity" {
		index := len(m.activity) - 1 - m.activityIndex
		if index >= 0 && index < len(m.activity) {
			return m.segments(width, segment{"Activity", terminal.Muted}, segment{"  ›  ", terminal.Rule}, segment{m.activity[index].Kind, terminal.Strong})
		}
		return m.theme.Render("Activity", width, terminal.Strong, false)
	}
	entries := m.entries()
	i := *m.cursor()
	if i >= len(entries) {
		return m.theme.Render(root, width, terminal.Strong, false)
	}
	entry := entries[i]
	b := badgeFor(entry)
	badgeText := m.badgeText(b)
	badgeWidth := uniseg.StringWidth(badgeText)
	nameWidth := max(width-badgeWidth-2, 0)
	if nameWidth < 12 {
		return m.segments(width, segment{root, terminal.Muted}, segment{"  ›  ", terminal.Rule}, segment{entry.Name, terminal.Strong})
	}
	left := m.segments(nameWidth, segment{root, terminal.Muted}, segment{"  ›  ", terminal.Rule}, segment{entry.Name, terminal.Strong})
	return spread(left, m.theme.Render(badgeText, badgeWidth, b.style, false), width)
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
	top := m.frameTop()
	footer := m.frameFooter()
	available := max(m.height-len(top)-len(footer), 0)
	body := m.body()
	pad := strings.Repeat(" ", m.margin())
	lines := append([]string(nil), top...)
	for row := 0; row < available; row++ {
		if row < len(body) && body[row] != "" {
			lines = append(lines, pad+body[row])
		} else {
			lines = append(lines, "")
		}
	}
	lines = append(lines, footer...)
	if len(lines) > m.height {
		lines = append(lines[:max(m.height-len(footer), 0)], footer...)
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

// body renders the screen content at the inner width; View adds margins and
// pads it to the full height so the footer stays on the last rows.
func (m *Model) body() []string {
	width := m.inner()
	body := []string{}
	add := func(s string, style terminal.Style) { body = append(body, m.theme.Render(s, width, style, false)) }
	switch m.screen {
	case "prompt":
		body = append(body, m.promptLines()...)
	case "help":
		body = append(body, m.helpView(width)...)
	case "activity":
		body = append(body, m.activityBody(width)...)
	case "examples":
		body = append(body, m.overviewBody(width)...)
	case "inventory":
		body = append(body, m.inventoryBody()...)
	case "inspector", "patch", "plan":
		sections := m.sections()
		if len(sections) == 0 {
			break
		}
		section := sections[m.section]
		if !m.hasSectionStrip() && m.screen != "patch" {
			label := section.Name
			for _, part := range m.documentMeta() {
				label += " · " + part
			}
			add(label, terminal.Muted)
		}
		if m.screen == "plan" && m.summaryUnavailable {
			add("Summary unavailable; exact preview bytes remain unchanged", terminal.Attention)
		}
		extra := 0
		if m.screen == "patch" && m.section == 0 && m.data != nil && m.data.Diff != nil {
			if m.doc != nil && m.diffHeaderAtTop() {
				extra = 1 // the file bar is already the first row
			} else {
				body = append(body, m.diffStickyHeader())
			}
		}
		if m.doc == nil {
			add("Loading the complete document…", terminal.Muted)
			break
		}
		if m.textLimited() && !m.hex {
			add("Text limited at 250000 lines; b opens exact hex for every stored byte", terminal.Attention)
		}
		if m.doc.Lines() == 0 && m.doc.HexRows() == 0 {
			add("(empty captured bytes; not an equality claim)", terminal.Muted)
			break
		}
		count := m.documentRows()
		for n := m.top; n < min(m.top+m.contentRows()+extra, count); n++ {
			body = append(body, m.documentRow(n, section, width))
		}
	}
	return body
}

// documentRow renders one trusted row of an open document. Content always
// follows a renderer-owned gutter; only typed dividers start at the margin.
func (m *Model) documentRow(n int, section Section, width int) string {
	highlight := func(line string, lineWidth int) string {
		if m.searchMatchesRow(n) {
			return m.theme.Highlight(line, m.searchQuery, lineWidth)
		}
		return line
	}
	if m.hex {
		return highlight(m.theme.Render(m.doc.HexLine(n), width, terminal.Plain, false), width)
	}
	if m.screen == "patch" && m.section == 0 && m.data != nil && m.data.Diff != nil {
		return m.diffDocumentRow(n)
	}
	if m.dividerRows[n] {
		return highlight(m.theme.Render(m.doc.LineAt(n, m.left, width), width, terminal.Strong, false), width)
	}
	if m.dividerRows[n+1] && m.doc.LineAt(n, 0, 1) == "" {
		return "" // display-only spacing before the next part
	}
	gutter := m.theme.Render("│ ", 2, terminal.Rule, false)
	gutterWidth := 2
	if !section.Wrap {
		digits := len(strconv.Itoa(max(m.doc.Lines(), 1)))
		number := fmt.Sprintf("%*d │ ", digits, n+1)
		gutterWidth = uniseg.StringWidth(number)
		gutter = m.theme.Render(number, gutterWidth, terminal.Rule, false)
	}
	if width <= gutterWidth {
		return terminal.Line(terminal.VisibleText(gutter), width)
	}
	available := width - gutterWidth
	marker := ""
	if m.doc.LongLine(n) && available >= 4 {
		marker = m.theme.Render("[b]", 3, terminal.Muted, false)
		available -= 3
	}
	text, style, tint := m.doc.LineAt(n, m.left, available), terminal.Plain, false
	if section.format == formatPatch {
		style = patchLineStyle([]byte(m.doc.SearchableLine(n)))
		tint = style == terminal.Added || style == terminal.Removed
	}
	if style == terminal.Plain {
		return gutter + highlight(text, available) + marker
	}
	return gutter + highlight(m.theme.Render(text, available, style, tint), available) + marker
}

// idleHint gently names the primary next step when no status applies.
func (m *Model) idleHint() string {
	switch m.screen {
	case "examples":
		row, ok := m.currentOverviewRow()
		switch {
		case !ok:
			return ""
		case row.kind == overviewGroupHeader && m.overviewCollapsed(row.group):
			return "Enter expands this group"
		case row.kind == overviewGroupHeader:
			return "Enter collapses this group"
		case row.kind == overviewInventory:
			return "Enter opens this path"
		case row.kind == overviewEvidence:
			return "Enter opens the full record"
		}
	case "inventory":
		return "Enter opens this path · 3 shows the whole diff"
	case "patch":
		if canNavigateFiles(m) == "" {
			return "] and [ jump between files · } and { between hunks"
		}
	case "activity":
		if len(m.activity) > 0 {
			return "Enter shows full IDs and details"
		}
	case "inspector":
		if len(m.sections()) > 1 {
			return "Tab shows the next section · Esc goes back"
		}
		return "Esc goes back"
	}
	return ""
}
func Run(m *Model, input io.Reader, output io.Writer) error {
	defer m.Close()
	return terminal.RunProgram(m.parent, m, input, output)
}

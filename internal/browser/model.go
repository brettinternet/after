package browser

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
)

// Job persists its own engine results before returning IDs. Cancellation or a
// stale UI completion must never erase those records. No job runs on selection.
type Job func(context.Context) (string, error)
type Jobs struct{ Capture, Import Job }
type loaded struct {
	request uint64
	data    *Data
	err     error
}
type paged struct {
	request uint64
	page    Page
	err     error
}
type finished struct {
	request uint64
	result  string
	err     error
}

type Model struct {
	selected                                     Selection
	data                                         *Data
	jobs                                         Jobs
	ctx, parent                                  context.Context
	cancel                                       context.CancelFunc
	workers                                      sync.WaitGroup
	jobCancel                                    context.CancelFunc
	jobID, request, loadID                       uint64
	busy                                         bool
	width, height                                int
	screen                                       string
	returnTo                                     string
	index, inventory, section, offset, top, left int
	page                                         Page
	lines                                        []string
	status                                       string
}

func New(ctx context.Context, selected Selection, jobs Jobs) *Model {
	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	selected.Evidence = append(selected.Evidence[:0:0], selected.Evidence...)
	return &Model{selected: selected, jobs: jobs, ctx: ctx, parent: parent, cancel: cancel, width: 80, height: 24, screen: "examples", status: "Loading immutable records; no project execution"}
}

// spawn starts ownership before returning a Bubble Tea command, so even a quit
// before command dispatch cannot race Close's join.
func (m *Model) spawn(work func() tea.Msg) tea.Cmd {
	ch := make(chan tea.Msg, 1)
	m.workers.Add(1)
	go func() { defer m.workers.Done(); ch <- work() }()
	return func() tea.Msg { return <-ch }
}
func (m *Model) Close() { m.cancel(); m.workers.Wait() }
func (m *Model) Init() tea.Cmd {
	m.loadID++
	id := m.loadID
	return m.spawn(func() tea.Msg { d, err := Load(m.ctx, m.selected); return loaded{id, d, err} })
}
func (m *Model) rows() int { return max(m.height-5, 1) }
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
func (m *Model) loadPage() tea.Cmd {
	m.request++
	id := m.request
	m.lines = nil
	m.page = Page{}
	m.top = 0
	m.left = 0
	sections := m.sections()
	if len(sections) == 0 {
		return nil
	}
	section := sections[m.section]
	offset := m.offset
	project := m.selected.Project
	return m.spawn(func() tea.Msg { p, err := ReadPage(m.ctx, project, section, offset); return paged{id, p, err} })
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
			m.status = "Stored records only; no new observation. c capture | i import configured file"
		}
	case paged:
		if msg.request != m.request {
			return m, nil
		}
		if msg.err != nil {
			m.lines = []string{"Artifact unavailable; no conclusion. Use after inspect for this stored ID."}
		} else {
			m.page = msg.page
			m.offset = msg.page.Offset
			m.lines = pageLines(msg.page.Bytes)
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
		// Keep full IDs and errors in a paged data view, never a clipped-only toast.
		if m.data != nil {
			m.data.Entries = append(m.data.Entries, Entry{Label: "not checked | job completion, not evidence", Name: "background result", Sections: []Section{{Name: "stored result IDs / diagnostic", Content: []byte(result)}}})
		}
	case tea.KeyMsg:
		if msg.Paste {
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "x":
			if m.jobCancel != nil {
				m.jobCancel()
				m.status = "Cancellation requested; waiting for owned job cleanup"
			}
		case "c":
			return m, m.startJob("Capture", m.jobs.Capture)
		case "i":
			return m, m.startJob("Import", m.jobs.Import)
		case "?":
			m.request++
			m.screen = "help"
			m.top = 0
		case "esc":
			m.request++
			m.top = 0
			if m.screen == "inspector" {
				m.screen = m.returnTo
			} else {
				m.screen = "examples"
			}
		case "d":
			m.request++
			m.screen = "inventory"
			m.top = 0
		case "enter":
			if m.screen == "examples" || m.screen == "inventory" {
				if len(m.entries()) == 0 {
					return m, nil
				}
				m.returnTo = m.screen
				m.screen = "inspector"
				m.section = 0
				m.offset = 0
				return m, m.loadPage()
			}
		case "tab", "shift+tab":
			if m.screen == "inventory" {
				m.screen = "patch"
				m.section = 0
				m.offset = 0
				return m, m.loadPage()
			}
			if m.screen == "patch" || m.screen == "inspector" {
				count := len(m.sections())
				if count == 0 {
					return m, nil
				}
				delta := 1
				if msg.String() == "shift+tab" {
					delta = -1
				}
				m.section = (m.section + delta + count) % count
				m.offset = 0
				return m, m.loadPage()
			}
		case "]", "[":
			if m.screen == "patch" || m.screen == "inspector" {
				next := m.offset + PageBytes
				if msg.String() == "[" {
					next = max(0, m.offset-PageBytes)
				}
				if next < m.page.Total && next != m.offset {
					m.offset = next
					return m, m.loadPage()
				}
			}
		case "right", "l":
			m.left = min(m.left+16, 240)
		case "left", "h":
			m.left = max(m.left-16, 0)
		case "j", "down", "k", "up", "pgdown", "pgup", "home", "end":
			delta := 1
			switch msg.String() {
			case "k", "up":
				delta = -1
			case "pgdown":
				delta = m.rows()
			case "pgup":
				delta = -m.rows()
			}
			if m.screen == "examples" || m.screen == "inventory" {
				p := m.cursor()
				*p += delta
				if msg.String() == "home" {
					*p = 0
				}
				if msg.String() == "end" {
					*p = len(m.entries()) - 1
				}
				*p = min(max(*p, 0), max(len(m.entries())-1, 0))
			} else {
				m.top += delta
				if msg.String() == "home" {
					m.top = 0
				}
				if msg.String() == "end" {
					m.top = len(m.lines) - 1
					if m.screen == "help" {
						m.top = len(helpLines) - 1
					}
				}
			}
		}
	}
	m.top = max(0, m.top)
	return m, nil
}

// Every data line is JSON-quoted in <=32-byte chunks. This makes binary/control
// bytes inspectable, bounds expensive grapheme work, and stops a payload newline
// from forging a trusted STATE or chrome row. Left/right reveals clipped text.
func pageLines(raw []byte) []string {
	out := []string{}
	for len(raw) > 0 {
		end := min(32, len(raw))
		if n := strings.IndexByte(string(raw[:end]), '\n'); n >= 0 {
			end = n + 1
		}
		out = append(out, strconv.Quote(string(raw[:end])))
		raw = raw[end:]
	}
	if len(out) == 0 {
		out = append(out, "(empty captured bytes; not an equality claim)")
	}
	return out
}
func window(raw string, left, width int) string {
	safe := []rune(terminal.Line(raw, 240))
	left = min(left, len(safe))
	return terminal.Line(string(safe[left:]), width)
}

var helpLines = []string{"Enter inspect | Esc back | d complete inventory", "Inventory: Tab raw patch; Enter captured source", "Inspector/patch: Tab/Shift+Tab section; [ ] byte page", "Up/down j/k scroll | PgUp/PgDn | Home/End", "Left/right h/l pan clipped text | ? help", "c capture | i import configured file | x cancel job", "q/Ctrl-C quit and cancel/join owned jobs", "STATE is engine metadata; data | rows are untrusted", "No execution on open. Pin/authorized rerun: headless CLI", "Evidence is finite measured inputs/channels, not safety"}

func (m *Model) View() string {
	lines := []string{terminal.Line("AFTER review | "+m.screen, m.width)}
	add := func(s string) { lines = append(lines, terminal.Line(s, m.width)) }
	data := func(s string) {
		prefix := "data | "
		if m.width <= len(prefix) {
			add(prefix)
			return
		}
		lines = append(lines, prefix+window(s, m.left, m.width-len(prefix)))
	}
	switch m.screen {
	case "help":
		top := min(m.top, len(helpLines)-1)
		for _, s := range helpLines[top:min(top+m.rows(), len(helpLines))] {
			add(window(s, m.left, m.width))
		}
	case "examples", "inventory":
		entries := m.entries()
		i := *m.cursor()
		if len(entries) == 0 {
			add("Not checked: no evidence / no inventory entries")
			add("d raw inventory remains available when evidence fails")
		} else {
			add("STATE " + entries[i].Label)
			top := max(0, i-m.rows()+1)
			for n := top; n < min(len(entries), top+m.rows()); n++ {
				prefix := "  "
				if n == i {
					prefix = "> "
				}
				data(prefix + strconv.Itoa(n+1) + " " + strconv.Quote(entries[n].Name))
			}
		}
	case "inspector", "patch":
		sections := m.sections()
		if len(sections) > 0 {
			add(fmt.Sprintf("Section %d/%d | bytes %d..%d/%d | pan %d", m.section+1, len(sections), m.page.Offset, m.page.Offset+len(m.page.Bytes), m.page.Total, m.left))
			data(strconv.Quote(sections[m.section].Name))
			top := min(m.top, max(len(m.lines)-1, 0))
			for n := top; n < min(top+m.rows()-1, len(m.lines)); n++ {
				data(m.lines[n])
			}
		}
	}
	add(m.status)
	add("Enter inspect | d diff | ? help | Esc back | q quit")
	return strings.Join(lines[:min(len(lines), m.height)], "\n")
}
func Run(m *Model, input io.Reader, output io.Writer) error {
	defer m.Close()
	_, err := tea.NewProgram(m, tea.WithInput(input), tea.WithOutput(output), tea.WithContext(m.parent), tea.WithAltScreen()).Run()
	return err
}

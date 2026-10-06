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
	returnTo                             string
	index, inventory, section, top, left int
	doc                                  *terminal.Document
	hex                                  bool
	status                               string
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
func (m *Model) rows() int { return max(m.height-5, 1) }
func (m *Model) contentRows() int {
	reserved := 5 // header, document details, section name, status and key hints
	if m.doc != nil && m.doc.Limited() && !m.hex {
		reserved++
	}
	return max(m.height-reserved, 1)
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
		return []Section{{Name: "exact execution preview; y approve once / n deny", Content: m.preview}}
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
			m.status = "Stored records only; no new observation. c capture | i import configured file"
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
			if msg.doc.Limited() {
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
				return m, m.loadDocument()
			}
		case "tab", "shift+tab":
			if m.screen == "inventory" {
				m.screen = "patch"
				m.section = 0
				return m, m.loadDocument()
			}
			if m.screen == "patch" || m.screen == "inspector" || m.screen == "plan" {
				count := len(m.sections())
				if count == 0 {
					return m, nil
				}
				delta := 1
				if msg.String() == "shift+tab" {
					delta = -1
				}
				m.section = (m.section + delta + count) % count
				return m, m.loadDocument()
			}
		case "b":
			if m.isDocumentScreen() && m.doc != nil {
				m.hex = !m.hex
				m.top = 0
				m.left = 0
			}
		case "right", "l":
			if m.isDocumentScreen() && m.doc != nil && !m.hex {
				m.left = min(m.left+16, m.doc.MaxColumns())
			} else if m.screen == "examples" || m.screen == "inventory" || m.screen == "help" {
				m.left = min(m.left+16, 240)
			}
		case "left", "h":
			if (m.isDocumentScreen() && !m.hex) || m.screen == "examples" || m.screen == "inventory" || m.screen == "help" {
				m.left = max(m.left-16, 0)
			}
		case "j", "down", "k", "up", "pgdown", "pgup", "home", "end", "g", "G":
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
				if msg.String() == "home" || msg.String() == "g" {
					*p = 0
				}
				if msg.String() == "end" || msg.String() == "G" {
					*p = len(m.entries()) - 1
				}
				*p = min(max(*p, 0), max(len(m.entries())-1, 0))
			} else if m.screen == "help" {
				m.top += delta
				if msg.String() == "home" || msg.String() == "g" {
					m.top = 0
				}
				if msg.String() == "end" || msg.String() == "G" {
					m.top = len(helpLines) - 1
				}
				m.top = min(max(m.top, 0), max(len(helpLines)-m.rows(), 0))
			} else if m.isDocumentScreen() && m.doc != nil {
				rows := m.contentRows()
				if msg.String() == "pgdown" {
					delta = rows
				} else if msg.String() == "pgup" {
					delta = -rows
				}
				total := m.documentRows()
				m.top += delta
				if msg.String() == "home" || msg.String() == "g" {
					m.top = 0
				}
				if msg.String() == "end" || msg.String() == "G" {
					m.top = max(total-rows, 0)
				}
				m.top = min(max(m.top, 0), max(total-rows, 0))
			}
		}
	}
	m.top = max(0, m.top)
	return m, nil
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
	return m.doc.Lines()
}
func window(raw string, left, width int) string { return terminal.LineAt(raw, left, width) }

var helpLines = []string{
	"Enter inspect | Esc back | d complete inventory",
	"Inventory: Tab raw patch; Enter captured source",
	"Inspector/patch: Tab/Shift+Tab section; b text/hex",
	"j/k move | PgUp/PgDn | Home/End | g/G document start/end",
	"h/l pan safely through first 4 KiB; [b] links to hex view",
	"Documents: NUL in first 8000 bytes starts in hex view",
	"Hex: offset rows keep every original byte reachable",
	"c capture | i import configured file | x cancel job",
	"q/Ctrl-C quit and cancel/join owned jobs",
	"STATE is engine metadata; payload lines cannot forge it",
	"p pin selected measured count | c capture | a accept snapshot",
	"r exact preview | y approve once | n deny | x cancel",
	"Resume using pin revision IDs shown in session details (s)",
	"Evidence is finite measured inputs/channels, not safety",
}

func (m *Model) View() string {
	lines := []string{terminal.Line("AFTER review | "+m.screen, m.width)}
	add := func(s string) { lines = append(lines, terminal.Line(s, m.width)) }
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
			if m.doc == nil {
				add("Loading complete document off the event loop")
			} else {
				if m.doc.Limited() && !m.hex {
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
						digits := len(strconv.Itoa(max(m.doc.Lines(), 1)))
						gutter := fmt.Sprintf("%*d │ ", digits, n+1)
						gutterWidth := uniseg.StringWidth(gutter)
						if m.width <= gutterWidth {
							lines = append(lines, terminal.Line(gutter, m.width))
							continue
						}
						available := m.width - gutterWidth
						marker := ""
						if m.doc.LongLine(n) && available >= 4 {
							marker = "[b]"
							available -= len(marker)
						}
						row := m.doc.LineAt(n, m.left, available) + marker
						lines = append(lines, gutter+row)
					}
				}
			}
		}
	}
	if m.pending != nil {
		add("New captured snapshot available; a explicitly accepts; selection unchanged")
	}
	add(m.status)
	if m.isDocumentScreen() {
		add("j/k move | PgUp/PgDn | Home/End | g/G start/end | h/l pan | b text/hex | Tab section | Esc back")
	} else {
		add("Enter inspect | d diff | ? help | Esc back | q quit")
	}
	return strings.Join(lines[:min(len(lines), m.height)], "\n")
}
func Run(m *Model, input io.Reader, output io.Writer) error {
	defer m.Close()
	_, err := tea.NewProgram(m, tea.WithInput(input), tea.WithOutput(output), tea.WithContext(m.parent), tea.WithAltScreen()).Run()
	return err
}

package terminal

import (
	"context"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Binding keeps late job results from updating a different selected basis.
type Binding struct{ SnapshotID, RequestID string }

// Result reports job completion, not evidence applicability or acceptance.
// Jobs must prepare documents outside Update/View and retain their own receipts.
type Result struct {
	Binding Binding
	Err     error
}

type Model struct {
	document      *Document
	binding       Binding
	title         string
	width, height int
	selected, top int
	status        string
	ctx, parent   context.Context
	cancel        context.CancelFunc
}

// New does not read files, run Git, or start project work. Background commands
// must use Context and remain bounded/cooperatively cancellable. Their owner is
// responsible for joining them after Run returns (Bubble Tea does not do so).
func New(ctx context.Context, doc *Document, binding Binding, title string) *Model {
	jobs, cancel := context.WithCancel(ctx)
	return &Model{document: doc, binding: binding, title: Line(title, maxWidth), width: 80, height: 24, status: "No evidence loaded", ctx: jobs, parent: ctx, cancel: cancel}
}

func (m *Model) Context() context.Context { return m.ctx }
func (m *Model) Close()                   { m.cancel() }
func (m *Model) Init() tea.Cmd            { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = min(max(msg.Width, 1), maxWidth)
		m.height = min(max(msg.Height, 1), maxHeight)
	case tea.KeyMsg:
		// Bracketed paste is data, not a navigation/quit command.
		if msg.Paste {
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.cancel()
			return m, tea.Quit
		case "j", "down":
			m.selected++
		case "k", "up":
			m.selected--
		case "pgdown":
			m.selected += m.rows()
		case "pgup":
			m.selected -= m.rows()
		case "home":
			m.selected = 0
		case "end":
			m.selected = m.document.Lines() - 1
		}
	case Result:
		if msg.Binding != m.binding {
			return m, nil
		}
		m.status = "Job finished; inspect evidence separately"
		if msg.Err != nil {
			m.status = "Job failed: " + Line(msg.Err.Error(), maxWidth)
		}
	}
	m.selected = min(max(m.selected, 0), max(m.document.Lines()-1, 0))
	if m.selected < m.top {
		m.top = m.selected
	}
	if m.selected >= m.top+m.rows() {
		m.top = m.selected - m.rows() + 1
	}
	m.top = min(max(m.top, 0), max(m.document.Lines()-m.rows(), 0))
	return m, nil
}

func (m *Model) rows() int { return max(m.height-3, 1) }

func (m *Model) View() string {
	// All interpolated content goes through Line, even stored titles/errors.
	lines := []string{Line("AFTER | "+m.title, m.width)}
	if m.document.Lines() == 0 {
		lines = append(lines, Line("No captured text / no evidence", m.width))
	} else {
		for n := m.top; n < min(m.top+m.rows(), m.document.Lines()); n++ {
			prefix := "  "
			if n == m.selected {
				prefix = "> "
			}
			if m.width <= 2 {
				lines = append(lines, Line(prefix, m.width))
				continue
			}
			lines = append(lines, prefix+Line(m.document.line(n), m.width-2))
		}
	}
	lines = append(lines, Line(m.status, m.width))
	lines = append(lines, Line(fmt.Sprintf("%d/%d | ↑↓ move | q quit | … clipped", min(m.selected+1, m.document.Lines()), m.document.Lines()), m.width))
	return strings.Join(lines[:min(len(lines), m.height)], "\n")
}

// Run restores the terminal before returning errors, and cancels the model's
// context on every exit. It never prints an untrusted error after restoration.
func Run(m *Model, input io.Reader, output io.Writer) error {
	defer m.Close()
	return RunProgram(m.parent, m, input, output)
}

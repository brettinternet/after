package terminal

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func model(t testing.TB, raw string) *Model {
	t.Helper()
	d, err := NewDocument([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	m := New(context.Background(), d, Binding{"snapshot-a", "request-1"}, "captured diff")
	t.Cleanup(m.Close)
	return m
}
func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestViewportSelection(t *testing.T) {
	m := model(t, strings.Repeat("界e\u0301\t"+strings.Repeat("x", 300)+"\n", 100))
	original := m.document
	for _, size := range []tea.WindowSizeMsg{{Width: 40, Height: 8}, {Width: 120, Height: 24}, {Width: 1, Height: 1}, {Width: 0, Height: 0}, {Width: 99999, Height: 99999}, {Width: 40, Height: 8}} {
		m.Update(size)
		for i := 0; i < 110; i++ {
			m.Update(key("j"))
		}
		if m.selected != 99 {
			t.Fatalf("selection %d", m.selected)
		}
		rows := strings.Split(m.View(), "\n")
		if len(rows) > m.height {
			t.Fatal("height overflow")
		}
		for _, row := range rows {
			assertSafe(t, row, m.width)
		}
		if m.document != original {
			t.Fatal("patch rebuilt")
		}
		for i := 0; i < 110; i++ {
			m.Update(key("k"))
		}
		if m.selected != 0 || m.top != 0 {
			t.Fatal("selection clamp")
		}
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.selected != 99 {
		t.Fatal("end")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.selected != 0 {
		t.Fatal("home")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.selected != m.rows() {
		t.Fatal("page")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.selected != 0 {
		t.Fatal("page up")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q"), Paste: true})
	if cmd != nil || m.Context().Err() != nil {
		t.Fatal("paste executed")
	}
}

func TestEmptyAndHostileSurfaces(t *testing.T) {
	m := model(t, "")
	m.title = "\x1b]0;title\a"
	m.Update(Result{Binding: m.binding, Err: errors.New("\x1b]52;c;secret\a\nAFTER observed")})
	m.Update(key("j"))
	if m.selected != 0 || !strings.Contains(m.View(), "No captured text") {
		t.Fatal(m.View())
	}
	for _, row := range strings.Split(m.View(), "\n") {
		assertSafe(t, row, m.width)
	}
	m.Update(Result{Binding: Binding{"other", "request-1"}})
	if !strings.Contains(m.status, "failed") {
		t.Fatal("late result attached")
	}
	m.Update(Result{Binding: Binding{"snapshot-a", "other"}})
	if !strings.Contains(m.status, "failed") {
		t.Fatal("wrong request attached")
	}
	m.Update(Result{Binding: m.binding})
	if !strings.Contains(m.status, "inspect evidence separately") {
		t.Fatal("completion misrepresented")
	}
}

// The test-only wrapper supplies a real Bubble Tea command. Production retains
// no fake job, timer, demo app or generic job scheduler.
type busyModel struct {
	*Model
	started, stopped chan struct{}
}

func (m *busyModel) Init() tea.Cmd {
	return func() tea.Msg {
		close(m.started)
		<-m.Context().Done()
		close(m.stopped)
		return Result{Binding: m.binding, Err: m.Context().Err()}
	}
}
func (m *busyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.Model.Update(msg)
	return m, cmd
}

func TestRunCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := New(ctx, model(t, "diff").document, Binding{}, "cancelled")
	done := make(chan error, 1)
	go func() { done <- Run(m, nil, io.Discard) }()
	select {
	case err := <-done:
		if !errors.Is(err, tea.ErrProgramKilled) || !errors.Is(err, context.Canceled) {
			t.Fatalf("expected program and context cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled startup did not return")
	}
}

func TestAsyncQuit(t *testing.T) {
	m := &busyModel{Model: model(t, strings.Repeat("diff line\n", 100000)), started: make(chan struct{}), stopped: make(chan struct{})}
	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutSignalHandler())
	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()
	defer p.Kill()
	select {
	case <-m.started:
	case <-time.After(5 * time.Second):
		t.Fatal("job not started")
	}
	start := time.Now()
	p.Send(tea.WindowSizeMsg{Width: 40, Height: 10})
	p.Send(key("j"))
	p.Send(key("q"))
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("quit blocked on job")
	}
	select {
	case <-m.stopped:
	case <-time.After(time.Second):
		t.Fatal("job not cancelled")
	}
	if m.selected != 1 || m.width != 40 {
		t.Fatal("input/resize lost")
	}
	t.Logf("active-job resize/input/quit: %s", time.Since(start))
}

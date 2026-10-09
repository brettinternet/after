package terminal

import (
	"context"
	"errors"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// RunProgram restores the terminal before returning, including on cancellation.
// The model's own context remains responsible for cancelling its background work.
func RunProgram(ctx context.Context, model tea.Model, input io.Reader, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(tea.ErrProgramKilled, err)
	}

	// Bubble Tea v1.3.10 skips joining its input loop on forced shutdown,
	// racing cancelreader.Close against Read on macOS. Translate parent
	// cancellation into Quit so shutdown joins the reader before closing it.
	p := tea.NewProgram(model, tea.WithInput(input), tea.WithOutput(output), tea.WithContext(context.WithoutCancel(ctx)), tea.WithAltScreen())
	finished := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			p.Quit()
		case <-finished:
		}
	}()
	_, err := p.Run()
	close(finished)
	<-watcherDone
	if ctx.Err() != nil {
		return errors.Join(err, tea.ErrProgramKilled, ctx.Err())
	}
	return err
}

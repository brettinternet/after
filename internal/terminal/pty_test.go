//go:build darwin || linux

package terminal

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"golang.org/x/term"
)

func TestPTYRestoration(t *testing.T) {
	for _, mode := range []string{"quit", "ctrl-c", "context-error", "job-error"} {
		t.Run(mode, func(t *testing.T) {
			master, slave, err := pty.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer master.Close()
			defer slave.Close()
			if err := pty.Setsize(master, &pty.Winsize{Rows: 12, Cols: 40}); err != nil {
				t.Fatal(err)
			}
			before, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			doc, err := NewDocument([]byte("\x1b]52;c;Y2xpcA==\a\n\x1b]0;forged\a\n界e\u0301\tvalue"))
			if err != nil {
				t.Fatal(err)
			}
			m := New(ctx, doc, Binding{"snapshot", "pty"}, "PTY-ready")
			if mode == "job-error" {
				m.Update(Result{Binding: m.binding, Err: errors.New("injected \x1b]8;;https://evil.invalid\a")})
			}
			stopped := make(chan struct{})
			go func() { <-m.Context().Done(); close(stopped) }()
			ready := make(chan struct{})
			output := make(chan []byte, 1)
			go func() {
				var out bytes.Buffer
				buf := make([]byte, 4096)
				seen := false
				for {
					n, err := master.Read(buf)
					out.Write(buf[:n])
					if !seen && strings.Contains(out.String(), "PTY-ready") {
						close(ready)
						seen = true
					}
					if err != nil || out.Len() > 1<<20 {
						break
					}
				}
				output <- out.Bytes()
			}()
			done := make(chan error, 1)
			go func() { done <- Run(m, slave, slave) }()
			select {
			case <-ready:
			case err := <-done:
				t.Fatalf("early exit: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("no PTY frame")
			}
			during, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			if reflect.DeepEqual(before, during) {
				t.Fatal("PTY did not enter raw mode")
			}
			start := time.Now()
			switch mode {
			case "context-error":
				cancel()
			case "ctrl-c":
				if _, err := master.Write([]byte{3}); err != nil {
					t.Fatal(err)
				}
			default:
				if _, err := master.Write([]byte("q")); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				if mode == "context-error" {
					if !errors.Is(err, tea.ErrProgramKilled) || !errors.Is(err, context.Canceled) {
						t.Fatalf("expected cancellation error: %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("PTY quit exceeded 1s")
			}
			t.Logf("PTY %s exit: %s", mode, time.Since(start))
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("owned job not cancelled")
			}
			after, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("termios not restored")
			}
			// Close the slave to finish reading the actual terminal output on macOS/Linux.
			slave.Close()
			select {
			case out := <-output:
				for _, want := range []string{"\x1b[?1049h", "\x1b[?1049l", "\x1b[?25h"} {
					if !bytes.Contains(out, []byte(want)) {
						t.Fatalf("missing terminal restoration sequence %q", want)
					}
				}
				for _, bad := range []string{"\x1b]52;", "\x1b]0;forged", "\x1b]8;", "\u009b", "\u009d"} {
					if bytes.Contains(out, []byte(bad)) {
						t.Fatalf("payload escaped boundary: %q", bad)
					}
				}
			case <-time.After(time.Second):
				master.Close()
				t.Fatal("PTY output did not finish")
			}
		})
	}
}

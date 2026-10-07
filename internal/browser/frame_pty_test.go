//go:build darwin || linux

package browser

import (
	"context"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/creack/pty"
	"golang.org/x/term"
)

func TestResponsiveFramePTY(t *testing.T) {
	s, sel := setup(t, false)
	s.Close()
	stripCSI := regexp.MustCompile("\\x1b\\[[0-?]*[ -/]*[@-~]")
	stripSGR := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	for _, size := range []struct{ width, height int }{{80, 24}, {120, 40}} {
		for _, noColor := range []bool{false, true} {
			name := "color"
			if noColor {
				name = "NO_COLOR"
			}
			t.Run(name+"-"+strconv.Itoa(size.width)+"x"+strconv.Itoa(size.height), func(t *testing.T) {
				if noColor {
					t.Setenv("NO_COLOR", "1")
				} else {
					t.Setenv("NO_COLOR", "")
				}
				t.Setenv("TERM", "xterm-256color")
				m := New(context.Background(), sel, Jobs{})
				master, slave, err := pty.Open()
				if err != nil {
					t.Fatal(err)
				}
				defer master.Close()
				defer slave.Close()
				if err := pty.Setsize(master, &pty.Winsize{Rows: uint16(size.height), Cols: uint16(size.width)}); err != nil {
					t.Fatal(err)
				}
				before, err := term.GetState(int(slave.Fd()))
				if err != nil {
					t.Fatal(err)
				}
				chunks := make(chan string, 256)
				go func() {
					defer close(chunks)
					buf := make([]byte, 4096)
					for {
						n, err := master.Read(buf)
						if n > 0 {
							chunks <- string(buf[:n])
						}
						if err != nil {
							return
						}
					}
				}()
				var transcript strings.Builder
				done := make(chan error, 1)
				go func() { done <- Run(m, slave, slave) }()
				ready := false
				deadline := time.After(10 * time.Second)
				for !ready {
					select {
					case chunk, ok := <-chunks:
						if !ok {
							t.Fatal("PTY closed before drawing the captured frame")
						}
						transcript.WriteString(chunk)
						plain := stripCSI.ReplaceAllString(transcript.String(), "")
						ready = strings.Contains(plain, shortID(evidence.Digest(sel.Pair.Candidate))) && strings.Contains(plain, "working tree") && strings.Contains(plain, "1 Overview")
					case err := <-done:
						t.Fatalf("TUI exited before frame: %v", err)
					case <-deadline:
						t.Fatalf("missing responsive frame in PTY output: %q", stripCSI.ReplaceAllString(transcript.String(), ""))
					}
				}
				if _, err := master.Write([]byte("q")); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("q did not quit the real PTY")
				}
				after, err := term.GetState(int(slave.Fd()))
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("PTY terminal state was not restored", err)
				}
				slave.Close()
				for chunk := range chunks {
					transcript.WriteString(chunk)
				}
				raw := transcript.String()
				if noColor && stripSGR.MatchString(raw) {
					t.Fatal("NO_COLOR PTY emitted SGR styling")
				}
				if !noColor && !strings.Contains(raw, "\x1b[7m[1 Overview]") {
					t.Fatal("color PTY did not mark the active tab with reverse video")
				}
				plain := stripCSI.ReplaceAllString(raw, "")
				last := strings.LastIndex(plain, "AFTER ·")
				if last < 0 {
					t.Fatal("frame header missing from final PTY transcript")
				}
				lines := strings.Split(plain[last:], "\n")
				excerpt := strings.Join(lines[:min(2, len(lines))], "\n")
				t.Logf("real PTY %dx%d %s excerpt:\n%s", size.width, size.height, name, excerpt)
			})
		}
	}
}

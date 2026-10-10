//go:build darwin || linux

package browser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"golang.org/x/term"
)

func TestResponsiveFramePTY(t *testing.T) {
	sel := paymentLoopSelection(t)
	configPath := filepath.Join(sel.Project, "app/config.go")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(config), "5 * 60", "45", 1)
	if updated == string(config) || os.WriteFile(configPath, []byte(updated), 0600) != nil {
		t.Fatal("could not prepare the post-capture retention edit")
	}
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
				actions := &Actions{Project: sel.Project}
				m := New(context.Background(), sel, Jobs{Actions: actions})
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
				runFinished := false
				defer func() {
					if runFinished {
						return
					}
					_, _ = master.Write([]byte("\x03"))
					select {
					case <-done:
						return
					case <-time.After(5 * time.Second):
						_ = master.Close()
						_ = slave.Close()
						t.Error("PTY model did not stop after a failed assertion")
					}
				}()
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
						ready = strings.Contains(plain, shortID(evidence.Digest(sel.Pair.Candidate))) && strings.Contains(plain, "NOT CHECKED") && strings.Contains(plain, "Overview")
					case err := <-done:
						t.Fatalf("TUI exited before frame: %v", err)
					case <-deadline:
						t.Fatalf("missing responsive frame in PTY output: %q", stripCSI.ReplaceAllString(transcript.String(), ""))
					}
				}
				waitForSince := func(offset int, wants ...string) {
					t.Helper()
					deadline := time.After(5 * time.Second)
					for {
						raw := transcript.String()
						offset = min(offset, len(raw))
						plain := stripCSI.ReplaceAllString(raw[offset:], "")
						found := true
						for _, want := range wants {
							found = found && strings.Contains(plain, want)
						}
						if found {
							return
						}
						select {
						case chunk, ok := <-chunks:
							if !ok {
								t.Fatal("PTY closed before requested view")
							}
							transcript.WriteString(chunk)
						case err := <-done:
							t.Fatalf("TUI exited before requested view: %v", err)
						case <-deadline:
							t.Fatalf("missing requested PTY view %v: %q", wants, plain)
						}
					}
				}
				waitFor := func(wants ...string) { waitForSince(0, wants...) }
				overviewExcerpt := ptyExcerpt(stripCSI.ReplaceAllString(transcript.String(), ""), "NOT CHECKED")
				if _, err := master.Write([]byte("2")); err != nil {
					t.Fatal(err)
				}
				waitFor("CHANGED", "app/config.go")
				if _, err := master.Write([]byte("/")); err != nil {
					t.Fatal(err)
				}
				waitFor("Enter search")
				if _, err := master.Write([]byte("config.go\r")); err != nil {
					t.Fatal(err)
				}
				waitFor("match 1 of 1")
				searchExcerpt := ptyExcerpt(stripCSI.ReplaceAllString(transcript.String(), ""), "match 1 of 1")
				if noColor && !strings.Contains(stripCSI.ReplaceAllString(transcript.String(), ""), "⟦config.go⟧") {
					t.Fatal("NO_COLOR PTY did not visibly delimit its list match")
				}
				if !noColor && !strings.Contains(transcript.String(), "\x1b[7mconfig.go\x1b[0m") {
					t.Fatal("color PTY did not reverse-highlight its list match")
				}
				if _, err := master.Write([]byte("\x1b")); err != nil {
					t.Fatal(err)
				}
				waitFor("Search cleared")
				if _, err := master.Write([]byte("c")); err != nil {
					t.Fatal(err)
				}
				waitFor("New capture ", "u reviews it")
				if _, err := master.Write([]byte("u")); err != nil {
					t.Fatal(err)
				}
				waitFor("Use this captured candidate?", "paths differ from candidate", "Pins may reopen")
				selectionStart := transcript.Len()
				if _, err := master.Write([]byte("\r")); err != nil {
					t.Fatal(err)
				}
				// Confirmation returns to Changes. Sending a redundant '2'
				// and matching its old output could batch it with the next '3'.
				waitForSince(selectionStart, "Snapshot selected", "base "+shortID(evidence.Digest(sel.Pair.Base)), "CHANGED", "app/config.go", "computed from captured sources — not Git's patch")
				changeExcerpt := stripCSI.ReplaceAllString(transcript.String(), "")
				if _, err := master.Write([]byte("3")); err != nil {
					t.Fatal(err)
				}
				waitFor("app/config.go · modified", "1/1 ──", "@@ -", "retentionSeconds int64 = 45")
				searchStart := transcript.Len()
				if _, err := master.Write([]byte("/")); err != nil {
					t.Fatal(err)
				}
				waitForSince(searchStart, "Enter search")
				searchStart = transcript.Len()
				if _, err := master.Write([]byte("retentionSeconds\r")); err != nil {
					t.Fatal(err)
				}
				waitForSince(searchStart, "match 1 of")
				diffSearchPlain := stripCSI.ReplaceAllString(transcript.String(), "")
				if noColor && !strings.Contains(diffSearchPlain, "⟦retentionSeconds⟧") {
					t.Fatalf("NO_COLOR PTY did not visibly delimit its Diff match: %q", diffSearchPlain)
				}
				diffSearchExcerpt := ptyExcerpt(diffSearchPlain, "match 1 of")
				clearStart := transcript.Len()
				if _, err := master.Write([]byte("\x1b")); err != nil {
					t.Fatal(err)
				}
				// The list search already emitted this message. Wait for this
				// Escape to finish before sending a key it could consume as Alt.
				waitForSince(clearStart, "Search cleared")
				if _, err := master.Write([]byte("}")); err != nil {
					t.Fatal(err)
				}
				waitFor("Can't Go to the next indexed hunk")
				if _, err := master.Write([]byte("s")); err != nil {
					t.Fatal(err)
				}
				waitFor("Activity", "SESSION", "ACTIVITY", "capture finished")
				if _, err := master.Write([]byte("q")); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-done:
					runFinished = true
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
				if !noColor {
					if !strings.Contains(raw, "\x1b[1mOverview\x1b[0m") {
						t.Fatal("color PTY did not mark the active tab in bold")
					}
					// xterm-256color tints added/removed lines with backgrounds.
					for _, style := range []string{"\x1b[48;5;22m", "\x1b[48;5;52m", "\x1b[36m"} {
						if !strings.Contains(raw, style) {
							t.Fatalf("color PTY missing diff style %q", style)
						}
					}
				}
				plain := stripCSI.ReplaceAllString(raw, "")
				last := strings.LastIndex(plain, " AFTER   ")
				if last < 0 {
					t.Fatal("frame header missing from final PTY transcript")
				}
				lines := strings.Split(plain[last:], "\n")
				excerpt := strings.Join(lines[:min(2, len(lines))], "\n")
				t.Logf("real PTY %dx%d %s excerpt:\n%s", size.width, size.height, name, excerpt)
				t.Logf("real PTY Overview excerpt: %s", overviewExcerpt)
				t.Logf("real PTY Search excerpt: %s", searchExcerpt)
				t.Logf("real PTY Changes excerpt: %s", ptyExcerpt(changeExcerpt, "computed from captured sources — not Git's patch"))
				t.Logf("real PTY Diff search excerpt: %s", diffSearchExcerpt)
				t.Logf("real PTY Diff excerpt: %s", ptyExcerpt(plain, "computed from captured sources — not Git's patch"))
				t.Logf("real PTY Activity excerpt: %s", ptyExcerpt(plain, "ACTIVITY"))
			})
		}
	}
}

func TestOverviewCardPreviewPTY(t *testing.T) {
	s, selection := setup(t, false)
	selection.Evidence = []evidence.Digest{viewComparison(t, s, selection, false, evidence.Complete)}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := Load(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []struct{ width, height int }{{80, 24}, {120, 40}} {
		for _, noColor := range []bool{false, true} {
			name := fmt.Sprintf("%dx%d/no-color=%t", size.width, size.height, noColor)
			t.Run(name, func(t *testing.T) {
				if noColor {
					t.Setenv("NO_COLOR", "1")
				} else {
					t.Setenv("NO_COLOR", "")
				}
				t.Setenv("TERM", "xterm-256color")
				model := New(context.Background(), selection, Jobs{})
				model.data, model.selected = data, selection
				model.selectFirstOverviewRow()
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
				go func() { done <- Run(model, slave, slave) }()
				waitFor := func(want string) {
					t.Helper()
					deadline := time.After(5 * time.Second)
					for {
						plain := stripPTYControls(transcript.String())
						if strings.Contains(plain, want) {
							return
						}
						select {
						case chunk, ok := <-chunks:
							if !ok {
								t.Fatalf("PTY closed before %q: %s", want, plain)
							}
							transcript.WriteString(chunk)
						case err := <-done:
							t.Fatalf("TUI exited before %q: %v", want, err)
						case <-deadline:
							t.Fatalf("missing PTY text %q: %s", want, plain)
						}
					}
				}
				waitFor("12h same-key retry")
				if size.width >= 110 {
					waitFor("── Payment case")
					waitFor("candidate 2")
					waitFor("── Input")
					if !strings.Contains(stripPTYControls(transcript.String()), "30s same-key retry") {
						t.Fatal("wide PTY Card preview lost its content")
					}
				} else if strings.Contains(stripPTYControls(transcript.String()), "── Payment case") {
					t.Fatal("narrow PTY unexpectedly rendered the side preview")
				}
				raw := transcript.String()
				if noColor && regexp.MustCompile("\\x1b\\[[0-9;]*m").MatchString(raw) {
					t.Fatal("NO_COLOR Card preview emitted SGR")
				}
				if !noColor && !strings.Contains(raw, "\x1b[1m") {
					t.Fatal("color Card preview omitted typed section-title styling")
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
					t.Fatal("q did not quit the Card preview PTY")
				}
				after, err := term.GetState(int(slave.Fd()))
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("Card preview PTY did not restore terminal state", err)
				}
				t.Logf("real PTY %dx%d no-color=%t Card excerpt: %s", size.width, size.height, noColor, ptyExcerpt(stripPTYControls(raw), "Payment case"))
			})
		}
	}
}

func TestRunQuitPTYConfirmationAndCtrlC(t *testing.T) {
	for _, size := range []struct{ width, height int }{{80, 24}, {120, 40}} {
		for _, noColor := range []bool{false, true} {
			for _, quit := range []string{"q-confirm", "ctrl-c"} {
				name := fmt.Sprintf("%dx%d/no-color=%t/%s", size.width, size.height, noColor, quit)
				t.Run(name, func(t *testing.T) {
					if noColor {
						t.Setenv("NO_COLOR", "1")
					} else {
						t.Setenv("NO_COLOR", "")
					}
					t.Setenv("TERM", "xterm-256color")
					pair := evidence.SnapshotPair{Base: evidence.Digest("sha256:" + strings.Repeat("a", 64)), Candidate: evidence.Digest("sha256:" + strings.Repeat("b", 64))}
					m := New(context.Background(), Selection{Project: "pty-run", Pair: pair}, Jobs{})
					m.data = &Data{}
					m.running, m.runStarted = true, time.Now().Add(-65*time.Second)
					runCtx, cancel := context.WithCancel(m.ctx)
					m.runCancel = cancel
					joined := make(chan struct{})
					m.spawn(func() tea.Msg {
						<-runCtx.Done()
						close(joined)
						return ran{pair: pair, err: runCtx.Err()}
					})
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
					waitFor := func(wants ...string) {
						t.Helper()
						deadline := time.After(5 * time.Second)
						for {
							plain := stripPTYControls(transcript.String())
							found := true
							for _, want := range wants {
								found = found && strings.Contains(plain, want)
							}
							if found {
								return
							}
							select {
							case chunk, ok := <-chunks:
								if !ok {
									t.Fatalf("PTY closed before %v: %s", wants, plain)
								}
								transcript.WriteString(chunk)
							case err := <-done:
								t.Fatalf("TUI exited before %v: %v", wants, err)
							case <-deadline:
								t.Fatalf("missing PTY text %v: %s", wants, plain)
							}
						}
					}
					waitForAfter := func(previous string, wants ...string) {
						t.Helper()
						previousCount := strings.Count(stripPTYControls(previous), "Confirm quit?")
						deadline := time.After(5 * time.Second)
						for {
							plain := stripPTYControls(transcript.String())
							found := strings.Count(plain, "Confirm quit?") > previousCount
							for _, want := range wants {
								found = found && strings.Contains(plain, want)
							}
							if found {
								return
							}
							select {
							case chunk, ok := <-chunks:
								if !ok {
									t.Fatalf("PTY closed before new %v: %s", wants, plain)
								}
								transcript.WriteString(chunk)
							case err := <-done:
								t.Fatalf("TUI exited before new %v: %v", wants, err)
							case <-deadline:
								t.Fatalf("missing new PTY text %v: %s", wants, plain)
							}
						}
					}
					waitFor("running 1:05 · x")
					if quit == "q-confirm" {
						if _, err := master.Write([]byte("q")); err != nil {
							t.Fatal(err)
						}
						waitFor("Confirm quit?", "y cancels the run and quits")
						select {
						case err := <-done:
							t.Fatalf("q quit without confirmation: %v", err)
						case <-time.After(50 * time.Millisecond):
						}
						if _, err := master.Write([]byte("n")); err != nil {
							t.Fatal(err)
						}
						waitFor("running 1:06 · x")
						previous := transcript.String()
						if _, err := master.Write([]byte("q")); err != nil {
							t.Fatal(err)
						}
						waitForAfter(previous, "Confirm quit?")
						if _, err := master.Write([]byte("y")); err != nil {
							t.Fatal(err)
						}
					} else {
						if _, err := master.Write([]byte("\x03")); err != nil {
							t.Fatal(err)
						}
					}
					select {
					case err := <-done:
						if err != nil {
							t.Fatal(err)
						}
					case <-time.After(5 * time.Second):
						t.Fatal("quit did not join the owned run")
					}
					select {
					case <-joined:
					case <-time.After(time.Second):
						t.Fatal("owned run did not observe cancellation before terminal restoration")
					}
					after, err := term.GetState(int(slave.Fd()))
					if err != nil || !reflect.DeepEqual(before, after) {
						t.Fatal("quit path did not restore terminal state", err)
					}
					if quit == "ctrl-c" && strings.Contains(stripPTYControls(transcript.String()), "Confirm quit?") {
						t.Fatal("Ctrl-C incorrectly asked for confirmation")
					}
					m.Close()
				})
			}
		}
	}
}

func stripPTYControls(raw string) string {
	return regexp.MustCompile("\\x1b\\[[0-?]*[ -/]*[@-~]").ReplaceAllString(raw, "")
}

func paymentLoopSelection(t *testing.T) Selection {
	t.Helper()
	project := filepath.Join(t.TempDir(), "payment")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = project
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + project, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2025-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2025-01-01T00:00:00Z"}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", err, output)
		}
	}
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	git("init", "-q", "--template=", "-b", "main")
	for _, name := range []string{"go.mod", "app/main.go", "app/config.go", "driver/main.go"} {
		raw, err := os.ReadFile(filepath.Join("..", "paymentfixture", "testdata", "payment", name))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(project, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", "--all")
	git("commit", "-qm", "payment base")
	configPath := filepath.Join(project, "app/config.go")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	candidate := strings.Replace(string(config), "24 * 60 * 60", "5 * 60", 1)
	if candidate == string(config) || os.WriteFile(configPath, []byte(candidate), 0600) != nil {
		t.Fatal("could not prepare the initial payment review candidate")
	}
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	captured, err := capture.Capture(t.Context(), project, s, capture.Options{})
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return Selection{Project: project, Pair: evidence.SnapshotPair{Base: captured.Base.ID, Candidate: captured.Candidate.ID}}
}

func ptyExcerpt(raw, marker string) string {
	index := strings.LastIndex(raw, marker)
	if index < 0 {
		return "<not found>"
	}
	start, end := max(index-60, 0), min(index+180, len(raw))
	return strings.ReplaceAll(raw[start:end], "\r", "")
}

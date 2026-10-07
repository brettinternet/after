//go:build darwin || linux

package browser

import (
	"context"
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
						ready = strings.Contains(plain, shortID(evidence.Digest(sel.Pair.Candidate))) && strings.Contains(plain, "Stored records only") && strings.Contains(plain, "1 Overview")
					case err := <-done:
						t.Fatalf("TUI exited before frame: %v", err)
					case <-deadline:
						t.Fatalf("missing responsive frame in PTY output: %q", stripCSI.ReplaceAllString(transcript.String(), ""))
					}
				}
				waitFor := func(wants ...string) {
					t.Helper()
					deadline := time.After(5 * time.Second)
					for {
						plain := stripCSI.ReplaceAllString(transcript.String(), "")
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
							t.Fatalf("missing requested PTY view %v: %q", wants, stripCSI.ReplaceAllString(transcript.String(), ""))
						}
					}
				}
				if _, err := master.Write([]byte("2")); err != nil {
					t.Fatal(err)
				}
				waitFor("CHANGED", "app/config.go")
				if _, err := master.Write([]byte("c")); err != nil {
					t.Fatal(err)
				}
				waitFor("New capture ", "u reviews it")
				if _, err := master.Write([]byte("u")); err != nil {
					t.Fatal(err)
				}
				waitFor("Snapshot selected", "base "+shortID(evidence.Digest(sel.Pair.Base)), "computed from captured sources — not Git's patch")
				if _, err := master.Write([]byte("2")); err != nil {
					t.Fatal(err)
				}
				waitFor("CHANGED", "app/config.go", "computed from captured sources — not Git's patch")
				changeExcerpt := stripCSI.ReplaceAllString(transcript.String(), "")
				if _, err := master.Write([]byte("3")); err != nil {
					t.Fatal(err)
				}
				waitFor("app/config.go · file", "file 1 of", "@@ -", "retentionSeconds int64 = 45")
				if _, err := master.Write([]byte("}")); err != nil {
					t.Fatal(err)
				}
				waitFor("Can't Go to the next indexed hunk")
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
				if !noColor {
					if !strings.Contains(raw, "\x1b[7m[1 Overview]") {
						t.Fatal("color PTY did not mark the active tab with reverse video")
					}
					for _, style := range []string{"\x1b[32m", "\x1b[1;31m", "\x1b[36m"} {
						if !strings.Contains(raw, style) {
							t.Fatalf("color PTY missing diff style %q", style)
						}
					}
				}
				plain := stripCSI.ReplaceAllString(raw, "")
				last := strings.LastIndex(plain, "AFTER ·")
				if last < 0 {
					t.Fatal("frame header missing from final PTY transcript")
				}
				lines := strings.Split(plain[last:], "\n")
				excerpt := strings.Join(lines[:min(2, len(lines))], "\n")
				t.Logf("real PTY %dx%d %s excerpt:\n%s", size.width, size.height, name, excerpt)
				t.Logf("real PTY Changes excerpt: %s", ptyExcerpt(changeExcerpt, "computed from captured sources — not Git's patch"))
				t.Logf("real PTY Diff excerpt: %s", ptyExcerpt(plain, "computed from captured sources — not Git's patch"))
			})
		}
	}
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

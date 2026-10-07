//go:build darwin || linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	"github.com/creack/pty"
	"golang.org/x/term"
)

type loopPTY struct {
	t                  *testing.T
	master, slave      *os.File
	chunks             chan string
	done               chan int
	stderr             bytes.Buffer
	stdout             bytes.Buffer
	transcript, unread strings.Builder
	before             *term.State
}

func startLoopPTY(t *testing.T, args []string) *loopPTY {
	return startLoopPTYSize(t, args, 160, 30)
}

func startLoopPTYSize(t *testing.T, args []string, width, height int) *loopPTY {
	t.Helper()
	p := &loopPTY{t: t, chunks: make(chan string, 512), done: make(chan int, 1)}
	var err error
	p.master, p.slave, err = pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err = pty.Setsize(p.master, &pty.Winsize{Rows: uint16(height), Cols: uint16(width)}); err != nil {
		t.Fatal(err)
	}
	p.before, err = term.GetState(int(p.slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-stopped:
		default:
			_, _ = p.master.Write([]byte("q"))
			select {
			case <-stopped:
			case <-time.After(30 * time.Second):
				cancel()
				<-stopped
			}
		}
		cancel()
		p.master.Close()
		p.slave.Close()
	})
	go func() {
		defer close(p.chunks)
		buf := make([]byte, 8192)
		for {
			n, err := p.master.Read(buf)
			if n > 0 {
				p.chunks <- string(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { defer close(stopped); p.done <- run(ctx, args, &p.stdout, p.slave, p.slave, true) }()
	return p
}
func (p *loopPTY) send(keys string) {
	p.t.Helper()
	p.unread.Reset()
	if _, err := p.master.Write([]byte(keys)); err != nil {
		p.t.Fatal(err)
	}
}
func (p *loopPTY) expect(want string) {
	p.t.Helper()
	deadline := time.After(4 * time.Minute)
	for {
		if index := strings.Index(p.unread.String(), want); index >= 0 {
			remaining := p.unread.String()[index+len(want):]
			p.unread.Reset()
			p.unread.WriteString(remaining)
			return
		}
		select {
		case chunk, ok := <-p.chunks:
			if !ok {
				p.t.Fatal("PTY closed", want)
			}
			p.transcript.WriteString(chunk)
			p.unread.WriteString(chunk)
		case <-deadline:
			p.t.Fatalf("missing %q: %q", want, p.transcript.String())
		}
	}
}
func latestPinID(t *testing.T, project string) evidence.Digest {
	t.Helper()
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	pins, err := review.Heads(s)
	if err != nil || len(pins) == 0 {
		t.Fatalf("missing saved review pin: %v", err)
	}
	return pins[len(pins)-1].ID
}

func latestComparisonsForPair(s *store.Store, pair evidence.SnapshotPair) ([]evidence.Comparison, error) {
	ids, err := s.List("comparison")
	if err != nil {
		return nil, err
	}
	type result struct {
		comparison evidence.Comparison
		finished   time.Time
	}
	var matches []result
	for _, entry := range ids {
		comparison, err := store.Get[evidence.Comparison](s, entry.ID)
		if err != nil {
			return nil, err
		}
		receipt, err := store.Get[evidence.Receipt](s, comparison.Receipt)
		if err != nil {
			return nil, err
		}
		if receipt.Snapshots == pair {
			matches = append(matches, result{comparison: comparison, finished: receipt.FinishedAt})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].finished.After(matches[j].finished) })
	comparisons := make([]evidence.Comparison, len(matches))
	for i, match := range matches {
		comparisons[i] = match.comparison
	}
	return comparisons, nil
}

func (p *loopPTY) finish() (browser.Selection, []evidence.Digest) {
	p.t.Helper()
	p.send("q")
	select {
	case code := <-p.done:
		if code != 0 {
			p.t.Fatalf("exit %d: %s", code, p.stderr.String())
		}
	case <-time.After(30 * time.Second):
		p.t.Fatal("PTY quit blocked")
	}
	after, err := term.GetState(int(p.slave.Fd()))
	if err != nil || !reflect.DeepEqual(p.before, after) {
		p.t.Fatal("terminal not restored", err)
	}
	p.slave.Close()
	for chunk := range p.chunks {
		p.transcript.WriteString(chunk)
	}
	var output struct {
		Data browser.ReviewSession `json:"data"`
	}
	if err := json.NewDecoder(strings.NewReader(p.stdout.String())).Decode(&output); err != nil {
		p.t.Fatalf("no valid review session on stdout: %v; output=%q", err, p.stdout.String())
	}
	return browser.Selection{Pair: output.Data.Pair}, nil
}

func TestReviewLaunchResumePTY(t *testing.T) {
	for _, variant := range []struct {
		width, height int
		noColor       bool
	}{{80, 24, false}, {80, 24, true}, {120, 40, false}, {120, 40, true}} {
		t.Run(fmt.Sprintf("%dx%d-NO_COLOR=%t", variant.width, variant.height, variant.noColor), func(t *testing.T) {
			project := filepath.Join(t.TempDir(), "project")
			makeProject(t, project)
			firstSource := "package main\nfunc main() { println(1) }\n"
			writeProjectFile(t, project, "app/main.go", firstSource)
			t.Setenv("TERM", "xterm-256color")
			if variant.noColor {
				t.Setenv("NO_COLOR", "1")
			} else {
				t.Setenv("NO_COLOR", "")
			}
			statePath := filepath.Join(project, ".after", "session.json")
			readSession := func() browser.ReviewSession {
				t.Helper()
				raw, err := os.ReadFile(statePath)
				if err != nil {
					t.Fatal(err)
				}
				session, err := browser.DecodeReviewSession(raw)
				if err != nil {
					t.Fatal(err)
				}
				return session
			}
			var allTranscript strings.Builder

			p := startLoopPTYSize(t, []string{"review", "--project", project, "--json"}, variant.width, variant.height)
			p.expect("AFTER · project")
			p.expect("Stored records only")
			firstSelection, _ := p.finish()
			allTranscript.WriteString(p.transcript.String())
			first := readSession()
			if first.Pair != firstSelection.Pair || first.Capture.Staged || first.Capture.Base != "" {
				t.Fatalf("initial capture session mismatch: stored=%+v stdout=%+v", first, firstSelection)
			}
			if info, err := os.Stat(statePath); err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("saved session mode: info=%v err=%v", info, err)
			}
			firstBytes, err := os.ReadFile(statePath)
			if err != nil || strings.Contains(string(firstBytes), "println(1)") {
				t.Fatalf("session contains source or is unreadable: %v", err)
			}
			if !strings.Contains(p.transcript.String(), fmt.Sprintf("Saved review %s → %s · after review resumes it", shortID(first.Pair.Base), shortID(first.Pair.Candidate))) {
				t.Fatal("quit did not announce the saved review")
			}

			writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(2) }\n")
			p = startLoopPTYSize(t, []string{"review", "--project", project, "--json"}, variant.width, variant.height)
			p.expect("AFTER · project")
			p.expect("Stored records only")
			p.expect("New capture ")
			p.send("u")
			p.expect("Snapshot selected; prior evidence remains history")
			selectedBeforeQuit := readSession()
			if selectedBeforeQuit.Pair.Base != first.Pair.Base || selectedBeforeQuit.Pair.Candidate == first.Pair.Candidate {
				t.Fatalf("selection was not saved immediately: first=%+v selected=%+v", first.Pair, selectedBeforeQuit.Pair)
			}
			resumedSelection, _ := p.finish()
			allTranscript.WriteString(p.transcript.String())
			resumed := readSession()
			if resumed.Pair != resumedSelection.Pair || resumed.Pair.Base != first.Pair.Base || resumed.Pair.Candidate == first.Pair.Candidate {
				t.Fatalf("pending capture did not remain explicit until u: first=%+v resumed=%+v", first.Pair, resumed.Pair)
			}

			gitRun(t, project, "add", "app/main.go")
			writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(3) }\n")
			p = startLoopPTYSize(t, []string{"review", "--staged", "--project", project, "--json"}, variant.width, variant.height)
			p.expect("Replaced saved review ")
			p.expect("AFTER · project")
			p.send("c")
			p.expect("Capture running; selected pair unchanged")
			p.expect("No new capture; the selected pair is unchanged")
			replacedSelection, _ := p.finish()
			allTranscript.WriteString(p.transcript.String())
			replaced := readSession()
			if replaced.Pair != replacedSelection.Pair || replaced.Pair == resumed.Pair || !replaced.Capture.Staged {
				t.Fatalf("different capture flags did not replace the saved review: %+v", replaced)
			}

			beforeExplicit, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			p = startLoopPTYSize(t, []string{"review", string(first.Pair.Candidate), "--project", project, "--json"}, variant.width, variant.height)
			p.expect("AFTER · project")
			explicitSelection, _ := p.finish()
			allTranscript.WriteString(p.transcript.String())
			if explicitSelection.Pair != first.Pair {
				t.Fatalf("explicit ID opened another selection: %+v, want %+v", explicitSelection.Pair, first.Pair)
			}
			afterExplicit, err := os.ReadFile(statePath)
			if err != nil || !bytes.Equal(beforeExplicit, afterExplicit) {
				t.Fatalf("explicit ID changed saved session: %v", err)
			}

			transcript := allTranscript.String()
			if variant.noColor {
				if regexp.MustCompile(`\x1b\[[0-9;]*m`).MatchString(transcript) {
					t.Fatal("NO_COLOR PTY emitted SGR")
				}
			} else if !strings.Contains(transcript, "\x1b[7m[1 Overview]") {
				t.Fatal("color PTY did not style the active tab")
			}
			t.Logf("PTY %dx%d NO_COLOR=%t excerpts: Stored records only · New capture %s — u reviews it · Snapshot selected; prior evidence remains history · c: Capture running; selected pair unchanged · No new capture; the selected pair is unchanged · Saved review %s → %s · after review resumes it", variant.width, variant.height, variant.noColor, shortID(resumed.Pair.Candidate), shortID(first.Pair.Base), shortID(first.Pair.Candidate))
		})
	}
}

func TestReviewInvalidSessionIsReplacedPTY(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(1) }\n")
	t.Setenv("TERM", "xterm-256color")
	p := startLoopPTYSize(t, []string{"review", "--project", project, "--json"}, 120, 40)
	p.expect("AFTER · project")
	p.finish()
	statePath := filepath.Join(project, ".after", "session.json")
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(statePath, 0600); err != nil {
		t.Fatal(err)
	}
	p = startLoopPTYSize(t, []string{"review", "--project", project, "--json"}, 120, 40)
	p.expect("saved review is invalid; it will be replaced at the next save")
	p.expect("AFTER · project")
	selection, _ := p.finish()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	session, err := browser.DecodeReviewSession(raw)
	if err != nil || session.Pair != selection.Pair {
		t.Fatalf("invalid session was not replaced with the opened review: %+v %v", session, err)
	}
	info, err := os.Stat(statePath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		t.Fatalf("session replacement is not a private regular file: %v %v", info, err)
	}
	t.Log("PTY invalid private state: non-regular session reported, replaced with valid 0600 saved review")
}

func TestExplicitOlderPinRevisionPTY(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(1) }\n")
	t.Setenv("TERM", "xterm-256color")
	p := startLoopPTYSize(t, []string{"review", "--project", project, "--json"}, 120, 40)
	p.expect("AFTER · project")
	pair, _ := p.finish()
	statePath := filepath.Join(project, ".after", "session.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	receiptID, _ := seedCLIProcessRecords(t, project, string(pair.Pair.Base), string(pair.Pair.Candidate))
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.Get[evidence.Receipt](s, receiptID)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	older, err := review.Create(s, receipt.ID, "older pin revision", evidence.HumanIntent, "explicit older revision fixture")
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	newer, err := review.Select(s, older.ID, evidence.BasisOf(receipt), evidence.OriginalBase, "later revision fixture")
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if older.ID == newer.ID {
		t.Fatal("fixture did not create a newer pin revision")
	}

	p = startLoopPTYSize(t, []string{"review", string(older.ID), "--project", project, "--json"}, 120, 40)
	p.expect("older pin revision")
	p.send("\r")
	p.expect(string(older.ID))
	selection, _ := p.finish()
	if selection.Pair != pair.Pair {
		t.Fatalf("older pin revision opened another pair: %+v want %+v", selection.Pair, pair.Pair)
	}
	after, err := os.ReadFile(statePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("explicit older revision changed saved review: %v", err)
	}
	t.Logf("PTY explicit revision excerpt: older pin revision %s remained selected after newer revision %s was created", shortID(older.ID), shortID(newer.ID))
}

func TestReviewConsentPTY(t *testing.T) {
	project := filepath.Join(t.TempDir(), "payment")
	fixtureCommits(t, project)
	actions := &browser.Actions{Project: project, Repetitions: 1, Limits: sandbox.Limits{Seconds: 180, OutputBytes: 65536}}
	pair, err := actions.Capture(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	actions.Close()
	config := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(config, []byte("repetitions: 1\nrun_seconds: 180\noutput_bytes: 65536\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"review", string(pair.Base), string(pair.Candidate), "--project", project, "--config", config, "--json"}
	for _, variant := range []struct {
		width, height int
		noColor       bool
	}{{120, 40, false}, {120, 40, true}, {80, 24, false}, {80, 24, true}} {
		name := fmt.Sprintf("%dx%d-no-color-%t", variant.width, variant.height, variant.noColor)
		t.Run(name, func(t *testing.T) {
			t.Setenv("TERM", "xterm-256color")
			if variant.noColor {
				t.Setenv("NO_COLOR", "1")
			} else {
				t.Setenv("NO_COLOR", "")
			}
			p := startLoopPTYSize(t, args, variant.width, variant.height)
			p.expect("AFTER · payment")
			p.send("r")
			p.expect("Run this exact plan?")
			p.expect("Runs: 2 sides × 2 cases × 1 repetition = 4 runs · concurrency 1")
			p.send("G")
			p.expect("Container limits")
			p.send("g")
			p.send("\t")
			p.expect("Exact plan")
			p.expect("version")
			transcript := p.transcript.String()
			if variant.noColor {
				if strings.Contains(transcript, "\x1b[7m[Summary]") {
					t.Fatal("NO_COLOR PTY emitted theme reverse video")
				}
			} else if !strings.Contains(transcript, "\x1b[7m[Summary]") {
				t.Fatal("color PTY did not style the active Summary section")
			}
			_, results := p.finish()
			if len(results) != 0 {
				t.Fatalf("quitting consent unexpectedly attached run results: %v", results)
			}
			t.Logf("PTY %dx%d NO_COLOR=%t: Run this exact plan? · Summary · Runs 2×2×1=4; Tab Exact plan · q denied; terminal restored", variant.width, variant.height, variant.noColor)
		})
	}
}

// This opt-in test uses real Docker observations, not manufactured receipt flags.
func TestReviewLoopPTYProof(t *testing.T) {
	if os.Getenv("AFTER_TUI_PROOF") != "1" {
		t.Skip("task tui:proof authorizes real offline payment execution")
	}
	project := filepath.Join(t.TempDir(), "payment")
	base, _ := fixtureCommits(t, project)
	gitRun(t, project, "checkout", "--detach", base)
	a := &browser.Actions{Project: project, Repetitions: 1, Limits: sandbox.Limits{Seconds: 180, OutputBytes: 65536}, Docker: sandbox.Docker{Binary: os.Getenv("AFTER_DOCKER_BINARY"), Host: os.Getenv("AFTER_DOCKER_HOST")}}
	pair, err := a.Capture(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	preview, digest, err := a.Prepare(pair)
	if err != nil {
		t.Fatal(err)
	}
	id, err := a.Run(t.Context(), pair, preview, digest)
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte("repetitions: 1\nrun_seconds: 180\noutput_bytes: 65536\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := func(sel browser.Selection) []string {
		out := []string{"review", string(sel.Pair.Base), string(sel.Pair.Candidate), "--project", project, "--config", cfg, "--json"}
		for _, id := range sel.Evidence {
			out = append(out, string(id))
		}
		return out
	}
	sel := browser.Selection{Project: project, Pair: pair, Evidence: []evidence.Digest{id}}
	p := startLoopPTY(t, args(sel))
	p.expect("[EQUAL]")
	p.send("\r")
	p.expect("measured provider-request counts")
	p.send("d")
	p.expect("2 Changes")
	p.send("\t")
	p.expect("captured raw diff")
	p.send("\x1b")
	p.expect("1 Overview")
	p.send("p")
	p.expect("Pinned selected finite provider-request expectation")
	sel, _ = p.finish()
	sel.Project = project
	pinID := latestPinID(t, project)
	sel.Evidence = []evidence.Digest{pinID}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := review.Inspect(s, pinID)
	s.Close()
	if err != nil || !strings.Contains(v.Pin.Expectation, "expect 1 provider") {
		t.Fatal(v, err)
	}
	// Restart before editing proves the selected expectation is a durable record.
	p = startLoopPTY(t, args(sel))
	p.expect("observed · current")
	configBytes, err := os.ReadFile(filepath.Join(project, "app/config.go"))
	if err != nil {
		t.Fatal(err)
	}
	writeProjectFile(t, project, "app/config.go", strings.Replace(string(configBytes), "24 * 60 * 60", "5 * 60", 1))
	p.send("c")
	p.expect("new capture")
	p.send("u")
	p.expect("Snapshot selected; prior evidence remains history")
	p.send("\x1b[F")
	p.expect("not run / missing current evidence")
	p.send("\r")
	p.expect("exact reopening reason")
	sel, _ = p.finish()
	sel.Project = project
	pinID = latestPinID(t, project)
	sel.Evidence = []evidence.Digest{pinID}
	s, err = store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err = review.Inspect(s, pinID)
	s.Close()
	if err != nil || !v.MissingCurrentResult || v.CurrentReceipt != nil || v.Applicability != evidence.Stale || !strings.Contains(v.Reason, "whole-project snapshot identity changed") {
		t.Fatal("reopening invented evidence or lost reason", v, err)
	}
	p = startLoopPTY(t, args(sel))
	p.expect("[STALE]")
	p.send("r")
	p.expect("Run this exact plan?")
	p.send("n")
	p.expect("Execution denied; no project execution")
	// A second preview needs new consent; n did not execute or attach anything.
	p.send("r")
	p.expect("Run this exact plan?")
	p.send("y")
	p.expect("Authorized run active")
	responsive := time.Now()
	p.send("?")
	p.expect("Help")
	latency := time.Since(responsive)
	t.Logf("real authorized fixture active: PTY help input-to-render=%s (warm UI, includes scheduling)", latency)
	if latency > time.Second {
		t.Fatal("active fixture blocked PTY navigation budget")
	}
	p.send("\x1b")
	p.expect("Measured result attached")
	p.send("\x1b[F")
	p.expect("30s same-key retry · provider requests 1 → 1")
	p.send("k")
	p.expect("12h same-key retry · provider requests 1 → 2")
	// Real authorized cancellation persists an incomplete result, not equality.
	p.send("r")
	p.expect("Run this exact plan?")
	p.send("y")
	p.expect("Authorized run active")
	p.send("x")
	p.expect("Run failed/cancelled; incomplete result retained")
	sel, _ = p.finish()
	sel.Project = project
	pinID = latestPinID(t, project)
	sel.Evidence = []evidence.Digest{pinID}
	s, err = store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	v, err = review.Inspect(s, pinID)
	if err != nil || v.Pin.Decision != evidence.Reopened {
		t.Fatal(v, err)
	}
	foundSelect := false
	for _, event := range v.Pin.History {
		if event.Review.Action == "select" {
			foundSelect = true
			if event.Review.Receipt != "" {
				t.Fatal("snapshot invented evidence")
			}
		}
	}
	if !foundSelect {
		t.Fatal("missing selection history")
	}
	comparisons, err := latestComparisonsForPair(s, sel.Pair)
	if err != nil || len(comparisons) < 2 {
		t.Fatalf("missing real run/cancellation comparisons: %v %v", comparisons, err)
	}
	cancelled, good := comparisons[0], comparisons[1]
	if cancelled.Outcome != evidence.Incomparable || good.Outcome != evidence.Different {
		t.Fatalf("unexpected recent comparison outcomes: newest=%s prior=%s", cancelled.Outcome, good.Outcome)
	}
	p = startLoopPTY(t, args(sel))
	p.expect("[STALE]")
	p.send("\x1b[B\x1b[B")
	p.expect("[REOPENED]")
	p.send("\r")
	p.expect("exact reopening reason")
	p.finish()
	t.Log("Real PTY: inspect/raw diff, pin one request, restart, edit retention, capture notification/explicit acceptance, missing current evidence, deny preview, authorize 1->2 witness and 1->1 control, cancel, reopen after restart; terminal restored")
}

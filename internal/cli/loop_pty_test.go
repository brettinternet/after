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
	"github.com/brettinternet/after/internal/capture"
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
	want = strings.Join(strings.Fields(want), " ")
	deadline := time.After(4 * time.Minute)
	for {
		visible := strings.Join(strings.Fields(stripThemeSGR(p.unread.String())), " ")
		if index := strings.Index(visible, want); index >= 0 {
			remaining := visible[index+len(want):]
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
			p.expect("AFTER   project")
			p.expect("NOT CHECKED — no evidence was loaded")
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
			p.expect("AFTER   project")
			// The transient loaded status can be replaced before the next frame.
			// The pending-capture header survives either completion order.
			p.expect("new capture ")
			p.send("u")
			p.expect("Use this captured candidate?")
			p.expect("Pins may reopen; earlier results become history")
			p.expect("original base")
			p.expect(shortID(first.Pair.Base))
			p.expect("last inspected")
			p.expect(shortID(first.Pair.Candidate))
			p.send("\x1b[C")
			p.expect("> last inspected")
			p.send("\r")
			p.expect("Follow-up selected; prior evidence remains history")
			selectedBeforeQuit := readSession()
			if selectedBeforeQuit.Mode != evidence.FollowUp || selectedBeforeQuit.Baseline != first.Pair.Base || selectedBeforeQuit.Pair.Base != first.Pair.Candidate || selectedBeforeQuit.Pair.Candidate == first.Pair.Candidate {
				t.Fatalf("follow-up selection was not saved immediately: first=%+v selected=%+v", first, selectedBeforeQuit)
			}
			resumedSelection, _ := p.finish()
			allTranscript.WriteString(p.transcript.String())
			resumed := readSession()
			if resumed.Pair != resumedSelection.Pair || resumed.Mode != evidence.FollowUp || resumed.Baseline != first.Pair.Base || resumed.Pair.Base != first.Pair.Candidate || resumed.Pair.Candidate == first.Pair.Candidate {
				t.Fatalf("pending follow-up capture did not remain explicit until u: first=%+v resumed=%+v", first, resumed)
			}

			gitRun(t, project, "add", "app/main.go")
			writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(3) }\n")
			p = startLoopPTYSize(t, []string{"review", "--project", project, "--json"}, variant.width, variant.height)
			p.expect("AFTER   project")
			p.expect("last inspected " + shortID(resumed.Pair.Base))
			p.expect("candidate " + shortID(resumed.Pair.Candidate))
			resumedAgain, _ := p.finish()
			allTranscript.WriteString(p.transcript.String())
			if resumedAgain.Pair != resumed.Pair {
				t.Fatalf("resumed session opened another pair: got=%+v want=%+v", resumedAgain.Pair, resumed.Pair)
			}
			p = startLoopPTYSize(t, []string{"review", "--staged", "--project", project, "--json"}, variant.width, variant.height)
			p.expect("Replaced saved review ")
			p.expect("AFTER   project")
			// The initial header renders before stored records enable capture.
			p.expect("NOT CHECKED — no evidence was loaded")
			p.send("c")
			// A fast capture may finish between renderer frames.
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
			p.expect("AFTER   project")
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
			} else if !strings.Contains(transcript, "\x1b[1mOverview\x1b[0m") {
				t.Fatal("color PTY did not style the active tab")
			}
			t.Logf("PTY %dx%d NO_COLOR=%t excerpts: NOT CHECKED — no evidence was loaded · New capture %s — u reviews it · Snapshot selected; prior evidence remains history · c: Capture running; selected pair unchanged · No new capture; the selected pair is unchanged · Saved review %s → %s · after review resumes it", variant.width, variant.height, variant.noColor, shortID(resumed.Pair.Candidate), shortID(first.Pair.Base), shortID(first.Pair.Candidate))
		})
	}
}

func TestReviewResumeRestoresFollowUpPairAndPinRevisionPTY(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(1) }\n")
	actions := &browser.Actions{Project: project}
	initial, err := actions.Capture(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	actions.Close()
	receiptID, _ := seedCLIProcessRecords(t, project, string(initial.Base), string(initial.Candidate))
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	initialPin, err := review.Create(s, receiptID, "resume follow-up pin", evidence.HumanIntent, "resume fixture")
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(2) }\n")
	actions = &browser.Actions{Project: project}
	captured, err := actions.Capture(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	actions.Close()
	pair := evidence.SnapshotPair{Base: initial.Candidate, Candidate: captured.Candidate}
	s, err = store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	selectedPin, err := review.Select(s, initialPin.ID, evidence.ReviewBasis{Snapshots: pair}, evidence.FollowUp, "selected follow-up")
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	session := browser.NewReviewSession(pair, capture.Options{Mode: evidence.WorkingTree})
	session.Mode = evidence.FollowUp
	session.Baseline = initial.Base
	session.PinRevisions = []evidence.Digest{selectedPin.ID}
	if err := saveReviewSessionFile(project, session); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "1")
	p := startLoopPTYSize(t, []string{"review", "--project", project, "--json"}, 120, 40)
	p.expect("last inspected " + shortID(pair.Base))
	p.expect("[REOPENED]")
	p.expect("No new capture; the selected pair is unchanged")
	p.send("s")
	p.expect("SESSION")
	p.expect("loaded     " + shortID(selectedPin.ID))
	selection, _ := p.finish()
	if selection.Pair != pair {
		t.Fatalf("resumed another snapshot pair: %+v want %+v", selection.Pair, pair)
	}
	raw, err := os.ReadFile(filepath.Join(project, ".after", "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := browser.DecodeReviewSession(raw)
	if err != nil || resumed.Mode != evidence.FollowUp || resumed.Baseline != initial.Base || resumed.Pair != pair || !reflect.DeepEqual(resumed.PinRevisions, []evidence.Digest{selectedPin.ID}) {
		t.Fatalf("CLI resume did not restore mode, baseline, pair and pin revision: %+v %v", resumed, err)
	}
}

func TestReviewInvalidSessionIsReplacedPTY(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(1) }\n")
	t.Setenv("TERM", "xterm-256color")
	p := startLoopPTYSize(t, []string{"review", "--project", project, "--json"}, 120, 40)
	p.expect("AFTER   project")
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
	p.expect("AFTER   project")
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
	p.expect("AFTER   project")
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
	p.expect("1/5 ·")
	for range 4 {
		p.send("\t")
	}
	p.expect("5/5 ·")
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
			p.expect("AFTER   payment")
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
				if !strings.Contains(transcript, "[Summary]") || strings.Contains(transcript, "\x1b[1mSummary") {
					t.Fatal("NO_COLOR PTY did not bracket the active Summary section")
				}
			} else if !strings.Contains(transcript, "\x1b[1mSummary\x1b[0m") {
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
	// Keep the initial observations equal while providing a real captured diff
	// for the inventory/raw-diff navigation proof below.
	configPath := filepath.Join(project, "app", "config.go")
	configSource, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, append(configSource, []byte("\n// Synthetic review candidate; behavior unchanged.\n")...), 0600); err != nil {
		t.Fatal(err)
	}
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
	initialStore, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	initialComparison, err := store.Get[evidence.Comparison](initialStore, id)
	initialStore.Close()
	if err != nil {
		t.Fatal(err)
	}
	receiptID := initialComparison.Receipt
	variants := []struct {
		width, height int
		noColor       bool
	}{{80, 24, false}, {120, 40, false}, {80, 24, true}, {120, 40, true}}
	setTerminal := func(variant struct {
		width, height int
		noColor       bool
	}) {
		t.Setenv("TERM", "xterm-256color")
		if variant.noColor {
			t.Setenv("NO_COLOR", "1")
		} else {
			t.Setenv("NO_COLOR", "")
		}
	}
	verifyPromptTerminal := func(p *loopPTY, variant struct {
		width, height int
		noColor       bool
	}) {
		transcript := p.transcript.String()
		if variant.noColor {
			if regexp.MustCompile(`\x1b\[[0-9;]*m`).MatchString(transcript) {
				t.Fatal("NO_COLOR mutation prompt emitted SGR")
			}
		} else if !strings.Contains(transcript, "\x1b[1mOverview\x1b[0m") {
			t.Fatal("color mutation prompt did not retain the active tab style")
		}
	}
	var duplicateTargetID evidence.Digest
	for index, variant := range variants {
		setTerminal(variant)
		p := startLoopPTYSize(t, args(sel), variant.width, variant.height)
		p.expect("AFTER   payment")
		p.expect("12h same-key retry")
		p.send("p")
		p.expect("Pin this finite expectation?")
		p.expect(string(receiptID))
		if index < len(variants)-1 {
			p.send("\x1b")
			p.expect("Cancelled; no changes")
			unchanged, _ := p.finish()
			if unchanged.Pair != pair {
				t.Fatal("cancelled pin prompt changed the selected pair")
			}
			verifyPromptTerminal(p, variant)
			continue
		}
		p.send("\x15")
		p.send("\x1b[200~PTY pin reason\n\x1b]52;c;not-a-command\a\x1b[201~")
		p.send("\r")
		p.expect("Pinned selected finite provider-request expectation")
		duplicateTargetID = latestPinID(t, project)
		p.send("p")
		p.expect("Matching pin already exists: " + shortID(duplicateTargetID))
		p.send("d")
		p.expect("CHANGED")
		p.send("\t")
		p.expect("diff --git")
		p.send("\x1b")
		p.expect("Diff   Activity")
		p.finish()
		verifyPromptTerminal(p, variant)
	}
	pinID := latestPinID(t, project)
	if pinID != duplicateTargetID {
		t.Fatalf("duplicate pin refusal created a new pin: existing=%s latest=%s", duplicateTargetID, pinID)
	}
	sel.Evidence = []evidence.Digest{pinID, id}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := review.Inspect(s, pinID)
	s.Close()
	if err != nil || !strings.Contains(v.Pin.Expectation, "expect 1 provider") || v.Pin.History[len(v.Pin.History)-1].Reason != `PTY pin reason\u000a\u001b]52;c;not-a-command\u0007` {
		t.Fatal(v, err)
	}
	// Restart before editing proves the selected expectation is a durable record.
	p := startLoopPTY(t, args(sel))
	p.expect("[PINNED]")
	configBytes, err := os.ReadFile(filepath.Join(project, "app/config.go"))
	if err != nil {
		t.Fatal(err)
	}
	writeProjectFile(t, project, "app/config.go", strings.Replace(string(configBytes), "24 * 60 * 60", "5 * 60", 1))
	p.finish()
	for index, variant := range variants {
		setTerminal(variant)
		p = startLoopPTYSize(t, args(sel), variant.width, variant.height)
		p.expect("AFTER   payment")
		p.expect("Current complete result attached")
		p.send("c")
		p.expect("New capture")
		p.send("u")
		p.expect("Use this captured candidate?")
		p.expect("paths differ from candidate")
		p.expect("Pins may reopen; earlier results become history")
		if index < len(variants)-1 {
			p.send("\x1b")
			p.expect("u reviews it")
			unchanged, _ := p.finish()
			if unchanged.Pair != pair || latestPinID(t, project) != pinID {
				t.Fatal("cancelled snapshot prompt changed the selected pair or pin history")
			}
			verifyPromptTerminal(p, variant)
			continue
		}
		p.send("\x15TUI snapshot reason\r")
		p.expect("Snapshot selected; prior evidence remains history")
		selected, _ := p.finish()
		if selected.Pair.Base != pair.Base || selected.Pair.Candidate == pair.Candidate {
			t.Fatalf("confirmed selection did not keep original base: %+v", selected.Pair)
		}
		verifyPromptTerminal(p, variant)
		sel.Pair = selected.Pair
	}
	sel.Project = project
	pinID = latestPinID(t, project)
	sel.Evidence = []evidence.Digest{pinID, id}
	s, err = store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err = review.Inspect(s, pinID)
	s.Close()
	if err != nil || !v.MissingCurrentResult || v.CurrentReceipt != nil || v.Applicability != evidence.Stale || !strings.Contains(v.Reason, "whole-project snapshot identity changed") || v.Pin.History[len(v.Pin.History)-1].Reason != "TUI snapshot reason" {
		t.Fatal("reopening invented evidence or lost reason", v, err)
	}
	t.Log("mutation PTY excerpt (80x24/120x40, color/NO_COLOR): Pin this finite expectation? · Matching pin already exists · Use this captured candidate? · original base is kept · 1 paths differ · Pins may reopen; earlier results become history")
	p = startLoopPTY(t, args(sel))
	p.expect("[REOPENED]")
	p.send("r")
	p.expect("Run this exact plan?")
	p.send("n")
	p.expect("Execution denied; no project execution")
	// A second preview needs new consent; n did not execute or attach anything.
	p.send("r")
	p.expect("Run this exact plan?")
	p.send("y")
	p.expect("Running the approved plan")
	responsive := time.Now()
	p.send("?")
	p.expect("Keys")
	latency := time.Since(responsive)
	t.Logf("real authorized fixture active: PTY help input-to-render=%s (warm UI, includes scheduling)", latency)
	if latency > time.Second {
		t.Fatal("active fixture blocked PTY navigation budget")
	}
	p.send("\x1b")
	p.expect("Measured result attached")
	p.send("?")
	p.expect("Keys")
	p.send("\x1b")
	p.expect("NEEDS ANOTHER LOOK 2")
	p.expect("[REOPENED]")
	p.expect("[DIFFERENT]")
	p.expect("12h same-key retry · provider requests 1 → 2")
	p.expect("AGREES 1")
	p.expect("[EQUAL]")
	p.expect("30s same-key retry · provider requests 1 → 1")
	p.expect("▸ EARLIER SNAPSHOTS 2")
	p.send("\x1b[H" + strings.Repeat("\x1b[B", 5) + "\r") // Expand the history header after the attention rows and control.
	p.expect("▾ EARLIER SNAPSHOTS 2")
	p.expect("[STALE]")
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
	if err != nil || v.CurrentReceipt == nil || v.Applicability != evidence.Current || v.MissingCurrentResult {
		t.Fatal("rerun did not attach a current complete result", v, err)
	}
	currentReceiptID := v.CurrentReceipt.ID
	for index, variant := range variants {
		setTerminal(variant)
		p = startLoopPTYSize(t, args(sel), variant.width, variant.height)
		p.expect("[REOPENED]")
		p.send("a")
		p.expect("Accept this pin's current result?")
		p.expect(string(pinID))
		p.expect(string(currentReceiptID))
		if index < len(variants)-1 {
			p.send("\x1b")
			p.expect("Cancelled; no changes")
			unchanged, _ := p.finish()
			if unchanged.Pair != sel.Pair {
				t.Fatal("cancelled acceptance changed the selected pair")
			}
			verifyPromptTerminal(p, variant)
			continue
		}
		p.send("\x15TUI acceptance reason\r")
		p.expect("[ACCEPTED]")
		p.expect("Pin accepted for this current complete result")
		p.finish()
		verifyPromptTerminal(p, variant)
	}
	pinID = latestPinID(t, project)
	sel.Evidence = []evidence.Digest{pinID}
	s, err = store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err = review.Inspect(s, pinID)
	s.Close()
	if err != nil || v.Pin.Decision != evidence.Accepted || v.Pin.History[len(v.Pin.History)-1].Reason != "TUI acceptance reason" {
		t.Fatal("confirmed acceptance was not recorded", v, err)
	}
	// Real authorized cancellation persists an incomplete result, not equality.
	setTerminal(variants[len(variants)-1])
	p = startLoopPTY(t, args(sel))
	p.expect("[ACCEPTED]")
	p.send("r")
	p.expect("Run this exact plan?")
	p.send("y")
	p.expect("Running the approved plan")
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
	p.expect("[REOPENED]")
	p.send("\r")
	p.expect("receipt is not a complete observed execution")
	p.finish()

	setTerminal(variants[3])
	configBytes, err = os.ReadFile(filepath.Join(project, "app/config.go"))
	if err != nil {
		t.Fatal(err)
	}
	followUpSource := strings.Replace(string(configBytes), "5 * 60", "2 * 60", 1)
	if followUpSource == string(configBytes) {
		t.Fatal("could not prepare a second follow-up edit")
	}
	writeProjectFile(t, project, "app/config.go", followUpSource)
	p = startLoopPTYSize(t, args(sel), 120, 40)
	p.expect("AFTER   payment")
	p.expect("original base")
	p.expect(shortID(sel.Pair.Base))
	p.expect("[REOPENED]")
	p.send("c")
	p.expect("New capture ")
	p.send("u")
	p.expect("Use this captured candidate?")
	p.expect("last inspected")
	p.expect(shortID(sel.Pair.Candidate))
	p.send("\x1b[C")
	p.send("\r")
	p.expect("last inspected " + shortID(sel.Pair.Candidate))
	p.expect("Follow-up selected; prior evidence remains history")
	followUpSelection, _ := p.finish()
	if followUpSelection.Pair.Base != sel.Pair.Candidate || followUpSelection.Pair.Candidate == sel.Pair.Candidate {
		t.Fatalf("TUI did not select the follow-up pair: %+v prior=%+v", followUpSelection.Pair, sel.Pair)
	}
	sel.Pair = followUpSelection.Pair
	pinID = latestPinID(t, project)
	sel.Evidence = []evidence.Digest{pinID}
	// Reopen the pin's recorded mode, not a new explicit-pair review whose
	// supplied base intentionally becomes its original baseline.
	p = startLoopPTYSize(t, []string{"review", string(pinID), "--project", project, "--config", cfg, "--json"}, 120, 40)
	p.expect("AFTER   payment")
	p.expect("last inspected " + shortID(sel.Pair.Base))
	p.expect("[REOPENED]")
	p.send("r")
	p.expect("Run this exact plan?")
	p.expect(shortID(sel.Pair.Base))
	// The full-digest summary line exceeds 120 columns; pan to its candidate.
	p.send("\x1b[C\x1b[C\x1b[C\x1b[C")
	p.expect(shortID(sel.Pair.Candidate))
	p.send("y")
	p.expect("Running the approved plan")
	p.expect("Measured result attached")
	p.send("3")
	p.expect("computed from captured sources — not Git's patch")
	p.finish()
	sel.Project = project
	pinID = latestPinID(t, project)
	followStore, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err = review.Inspect(followStore, pinID)
	if err != nil || v.CurrentReceipt == nil || v.CurrentReceipt.Snapshots != sel.Pair || v.Applicability != evidence.Current || v.MissingCurrentResult {
		followStore.Close()
		t.Fatal("follow-up rerun did not attach to its exact pair", v, err)
	}
	if _, err := store.Get[evidence.Receipt](followStore, receiptID); err != nil {
		followStore.Close()
		t.Fatalf("original receipt was not retained as history: %v", err)
	}
	if err := followStore.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("Real PTY: original-base and last-inspected choices; follow-up preview/rerun attached only to its selected pair; computed follow-up diff; terminal restored")
}

//go:build darwin || linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	transcript, unread strings.Builder
	before             *term.State
}

func startLoopPTY(t *testing.T, args []string) *loopPTY {
	t.Helper()
	p := &loopPTY{t: t, chunks: make(chan string, 512), done: make(chan int, 1)}
	var err error
	p.master, p.slave, err = pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err = pty.Setsize(p.master, &pty.Winsize{Rows: 30, Cols: 160}); err != nil {
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
	go func() { defer close(stopped); p.done <- run(ctx, args, p.slave, &p.stderr, p.slave, true) }()
	return p
}
func (p *loopPTY) send(keys string) {
	p.t.Helper()
	if _, err := p.master.Write([]byte(keys)); err != nil {
		p.t.Fatal(err)
	}
}
func (p *loopPTY) expect(want string) {
	p.t.Helper()
	deadline := time.After(4 * time.Minute)
	for {
		if strings.Contains(p.unread.String(), want) {
			p.unread.Reset()
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
	raw := p.transcript.String()
	index := strings.LastIndex(raw, `{"Selection":`)
	if index < 0 {
		p.t.Fatal("no restart references", raw)
	}
	var session struct {
		Selection browser.Selection
		Results   []evidence.Digest
	}
	if err := json.NewDecoder(strings.NewReader(raw[index:])).Decode(&session); err != nil {
		p.t.Fatal(err)
	}
	return session.Selection, session.Results
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
		out := []string{"review", string(sel.Pair.Candidate), "--tui", "--base", string(sel.Pair.Base), "--project", project, "--config", cfg}
		for _, id := range sel.Evidence {
			out = append(out, "--evidence", string(id))
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
	if len(sel.Evidence) != 2 {
		t.Fatal("pin revision missing", sel)
	}
	pinID := sel.Evidence[1]
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
	s, err = store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err = review.Inspect(s, sel.Evidence[1])
	s.Close()
	if err != nil || !v.MissingCurrentResult || v.CurrentReceipt != nil || v.Applicability != evidence.Stale || !strings.Contains(v.Reason, "whole-project snapshot identity changed") {
		t.Fatal("reopening invented evidence or lost reason", v, err)
	}
	p = startLoopPTY(t, args(sel))
	p.expect("[STALE]")
	p.send("r")
	p.expect("exact execution preview")
	p.send("n")
	p.expect("Execution denied; no project execution")
	// A second preview needs new consent; n did not execute or attach anything.
	p.send("r")
	p.expect("exact execution preview")
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
	p.expect("exact execution preview")
	p.send("y")
	p.expect("Authorized run active")
	p.send("x")
	p.expect("Run failed/cancelled; incomplete result retained")
	sel, results := p.finish()
	if len(results) != 2 {
		t.Fatal("missing real run/cancellation", results)
	}
	s, err = store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	v, err = review.Inspect(s, sel.Evidence[1])
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
	good, err := store.Get[evidence.Comparison](s, results[0])
	if err != nil || good.Outcome != evidence.Different {
		t.Fatal(good, err)
	}
	cancelled, err := store.Get[evidence.Comparison](s, results[1])
	if err != nil || cancelled.Outcome != evidence.Incomparable {
		t.Fatal(cancelled, err)
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

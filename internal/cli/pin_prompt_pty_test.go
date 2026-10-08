//go:build darwin || linux

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/store"
)

// Synthetic artifacts exercise decision persistence, not actual execution.
func pinPromptFixture(t *testing.T) (string, evidence.Receipt, evidence.Digest) {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() {}\n// changed\n")
	code, _, diagnostic := invoke([]string{"capture", "--project", project}, false, "")
	if code != 0 {
		t.Fatal(diagnostic)
	}
	reader, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := newestCaptureForStore(reader)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	incomplete, _ := seedCLIProcessRecords(t, project, string(capture.Base), string(capture.Candidate))
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := store.Get[evidence.Receipt](s, incomplete)
	if err != nil {
		t.Fatal(err)
	}
	r.ID = ""
	r.State.Kind, r.State.Execution, r.State.Applicability = evidence.Observed, evidence.Completed, evidence.Current
	r.Completeness = evidence.Complete
	r.StartedAt = r.FinishedAt
	r.Limits = []string{"synthetic prompt test; not a real run"}
	artifact := func(value any, channel string) evidence.Artifact {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		a, err := s.PutArtifact(raw, channel, 64<<10)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	for _, seconds := range []int64{43200, 30} {
		for _, side := range []string{"base", "candidate"} {
			o := runner.Observation{Version: 1, Seconds: seconds, Responses: []runner.Response{{Status: 200, Body: "ok"}, {Status: 200, Body: "ok"}}, Calls: []runner.Call{{Method: "POST", Path: "/payments"}}}
			a := artifact(o, fmt.Sprintf("%s/%d/0/observation", side, seconds))
			sample := runner.Sample{RequestID: r.RequestID, Snapshots: r.Snapshots, Side: side, CaseSeconds: seconds, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Status: "completed", Artifacts: []evidence.Artifact{a}}
			r.Artifacts = append(r.Artifacts, a, artifact(sample, fmt.Sprintf("%s/%d/0/sample", side, seconds)))
		}
	}
	r, err = store.Put(s, r)
	if err != nil {
		t.Fatal(err)
	}
	return project, r, incomplete
}

func TestPinExpectationPTY(t *testing.T) {
	for _, noColor := range []bool{false, true} {
		t.Run(fmt.Sprintf("no-color-%t", noColor), func(t *testing.T) {
			project, receipt, incomplete := pinPromptFixture(t)
			home := t.TempDir()
			heads := func() []evidence.Pin {
				t.Helper()
				s, err := store.Open(project, false, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				pins, err := review.Heads(s)
				if err != nil {
					t.Fatal(err)
				}
				return pins
			}
			for _, mode := range []string{"pipe", "pty"} {
				run := runCLIPipe
				if mode == "pty" {
					run = runCLIPTY
				}
				code, out, stderr, err := run(project, home, noColor, []string{"pin"})
				if err != nil || code != 0 || stderr != "" || !strings.Contains(out, "after pin "+shortID(receipt.ID)) {
					t.Fatalf("bare pin: %d %s %s %v", code, out, stderr, err)
				}
				t.Logf("bare pin %s NO_COLOR=%t: %s", mode, noColor, out)
			}
			args := []string{"pin", string(receipt.ID)}
			code, out, diagnostic, err := runCLIPipe(project, home, noColor, args)
			if err != nil || code != ExitInvalid || out != "" || !strings.Contains(diagnostic, "--expectation '") {
				t.Fatalf("pipe: %d %q %q %v", code, out, diagnostic, err)
			}
			t.Logf("pipe NO_COLOR=%t: %s", noColor, diagnostic)
			for _, answer := range []string{"\n", "\x04"} {
				code, out, err := runCLIPTYAnswer(project, home, noColor, args, "(empty line cancels): ", answer)
				if err != nil || code != 0 || len(heads()) != 0 {
					t.Fatalf("cancel: %d %s %v", code, out, err)
				}
			}
			for _, answer := range []string{"1", "  Preserve this exact text  ", "2"} {
				before := len(heads())
				code, out, err := runCLIPTYAnswer(project, home, noColor, args, "(empty line cancels): ", answer+"\n")
				if err != nil || code != 0 || len(heads()) != before+1 {
					t.Fatalf("create: %d %s %v", code, out, err)
				}
				for _, want := range []string{"Scope: finite_example", string(receipt.ID), string(receipt.Snapshots.Base), string(receipt.Snapshots.Candidate), "1. At 43200s", "2. At 30s"} {
					if !strings.Contains(out, want) {
						t.Fatalf("prompt omitted %q: %s", want, out)
					}
				}
				want := answer
				if answer == "1" || answer == "2" {
					s, _ := store.Open(project, false, nil)
					suggestions, err := browser.PinSuggestions(s, receipt)
					s.Close()
					if err != nil {
						t.Fatal(err)
					}
					want = suggestions[int(answer[0]-'1')]
				}
				found := false
				for _, pin := range heads() {
					if pin.Expectation == want && pin.BasisReceipt == receipt.ID && pin.BasisSnapshots == receipt.Snapshots && pin.Scope == evidence.FiniteExample && pin.History[0].Reason == "Pinned from the command line" {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing exact expectation/defaults: %+v", heads())
				}
				prompt, _, _ := strings.Cut(out, "Card ·")
				t.Logf("80-column pin NO_COLOR=%t answer=%q: %s", noColor, answer, prompt)
			}
			code, out, diagnostic, err = runCLIPipe(project, home, noColor, []string{"pin"})
			if err != nil || code != 0 || diagnostic != "" || strings.Contains(out, "after pin "+string(receipt.ID)) {
				t.Fatalf("fully pinned run still suggested: %d %s %s %v", code, out, diagnostic, err)
			}
			code, out, err = runCLIPTYAnswer(project, home, noColor, []string{"pin", string(incomplete), "--scope", "human_intent"}, "Expectation (empty line cancels): ", "human intent\n")
			if err != nil || code != 0 || strings.Contains(out, "1. At") || !strings.Contains(out, "Scope: human_intent") {
				t.Fatalf("text only: %d %s %v", code, out, err)
			}
		})
	}
}

func TestPinPromptInputBoundaries(t *testing.T) {
	project, receipt, _ := pinPromptFixture(t)
	args := []string{"pin", string(receipt.ID), "--project", project, "--json"}
	reader := &readCountReader{remaining: 100}
	var stdout, stderr bytes.Buffer
	if code := run(t.Context(), args, &stdout, &stderr, reader, false); code != ExitInvalid || reader.read != 0 || stdout.Len() != 0 {
		t.Fatalf("pipe read input: %d %+v %s %s", code, reader, &stdout, &stderr)
	}
	if code := run(t.Context(), append(args, "--interactive=false"), &stdout, &stderr, reader, true); code != ExitInvalid || reader.read != 0 {
		t.Fatalf("disabled interaction read input: %d %+v", code, reader)
	}
	for _, input := range []string{"", "partial EOF", strings.Repeat("x", 4097) + "\n"} {
		code, out, diagnostic := invoke(args, true, input)
		wantCode := ExitOK
		if len(input) > 4096 {
			wantCode = ExitInvalid
		}
		if code != wantCode || out != "" {
			t.Fatalf("incomplete/oversized input: %d %q %q", code, out, diagnostic)
		}
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	pins, err := review.Heads(s)
	if err != nil || len(pins) != 0 {
		t.Fatalf("incomplete input created pins: %+v %v", pins, err)
	}
}

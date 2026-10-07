package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/store"
)

func TestPinAndReviewInputSafety(t *testing.T) {
	project := t.TempDir()
	t.Setenv("AFTER_PROJECT", project)
	id := "sha256:" + strings.Repeat("a", 64)
	for _, args := range [][]string{
		{"pin", id},
		{"pin", id, "--expectation", "x", "--scope", "universal"},
		{"pin", id, "--expectation", "x", "--reason", strings.Repeat("x", 4097)},
		{"pin", id, "--select", id, "--reason", "x"},
		{"pin", id, "--mode", "original_base"},
		{"pin", id, "--accept", "--attach", id, "--reason", "x"},
		{"pin", id, "--accept=false", "--reason", "x"},
		{"pin", id, "--attach", "bad\x1b]52;c;clipboard\a"},
		{"pin", id, "--reason", "no mutation"},
		{"pin", id, "--accept"},
	} {
		code, out, stderr := invoke(args, false, "")
		if code != ExitInvalid || out != "" || strings.ContainsAny(stderr, "\x1b\a") || !strings.Contains(stderr, " — ") {
			t.Fatalf("%v: %d %q %q", args, code, out, stderr)
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"review", id, "--select", id, "--mode", "original_base"}, "after pin PIN --select SNAPSHOT"},
		{[]string{"review", id, "--receipt", id}, "after pin PIN --attach RECEIPT"},
		{[]string{"review", id, "--accept"}, "after pin PIN --accept"},
		{[]string{"review", "--tui", id, "--base", id}, "after review BASE CANDIDATE"},
		{[]string{"review", id, "--evidence", id}, "after review BASE CANDIDATE"},
		{[]string{"inspect", id, "--base", id}, "after inspect BASE CANDIDATE"},
	} {
		code, out, stderr := invoke(tc.args, false, "")
		if code != ExitInvalid || out != "" || !strings.Contains(stderr, tc.want) || !strings.Contains(stderr, " — ") {
			t.Fatalf("removed form %v: %d %q %q", tc.args, code, out, stderr)
		}
	}
	for _, args := range [][]string{{"pin", "--help"}, {"review", "--help"}} {
		code, _, _ := invoke(args, false, "")
		if code != 0 {
			t.Fatal(code)
		}
	}
	entries, err := os.ReadDir(project)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid input/help wrote project: %v %v", entries, err)
	}
}

func TestExplicitReviewIDRequiresTerminalWithoutCreatingStore(t *testing.T) {
	project := filepath.Join(t.TempDir(), "checkout")
	makeProject(t, project)
	id := "sha256:" + strings.Repeat("a", 64)
	code, out, stderr := invoke([]string{"review", id, "--project", project}, false, "")
	if code != ExitInvalid || out != "" || !strings.Contains(stderr, "run after review ID in a terminal") {
		t.Fatalf("nonterminal explicit-ID review: %d %q %q", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(project, ".after")); !os.IsNotExist(err) {
		t.Fatalf("nonterminal review created private store: %v", err)
	}
	code, out, stderr = invoke([]string{"review", "--project", project}, false, "")
	if code != ExitInvalid || out != "" || !strings.Contains(stderr, "after status --json") {
		t.Fatalf("nonterminal implicit review: %d %q %q", code, out, stderr)
	}
}

func TestPinDefaultsScopeModeAndVerbatimHistory(t *testing.T) {
	project := filepath.Join(t.TempDir(), "checkout")
	baseCommit, candidateCommit := fixtureCommits(t, project)
	code, output, stderr := invoke([]string{"capture", "--project", project, "--base", baseCommit, "--target", candidateCommit, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("capture: %d %q", code, stderr)
	}
	var captured struct {
		Data struct {
			Base      snapshotSummary `json:"base_snapshot"`
			Candidate snapshotSummary `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &captured); err != nil {
		t.Fatal(err)
	}
	receiptID, _ := seedCLIProcessRecords(t, project, string(captured.Data.Base.ID), string(captured.Data.Candidate.ID))
	code, out, diagnostic := invoke([]string{"pin", string(receiptID), "--expectation", "finite case", "--project", project}, false, "")
	if code != ExitInvalid || out != "" || !strings.Contains(diagnostic, "--scope human_intent") || !strings.Contains(diagnostic, " — ") {
		t.Fatalf("unsupported finite default: %d %q %q", code, out, diagnostic)
	}
	code, out, diagnostic = invoke([]string{"pin", string(receiptID), "--expectation", "broader intent", "--scope", "human_intent", "--project", project, "--json"}, false, "")
	if code != ExitOK || diagnostic != "" {
		t.Fatalf("pin create: %d %q", code, diagnostic)
	}
	var envelope struct {
		Data review.View `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatal(err)
	}
	pin := envelope.Data.Pin
	if pin.Scope != evidence.HumanIntent || len(pin.History) != 1 || pin.History[0].Reason != "Pinned from the command line" {
		t.Fatalf("default scope/reason not retained: %+v", pin)
	}
	reason := "  reason kept verbatim  "
	code, out, diagnostic = invoke([]string{"pin", string(pin.ID), "--select", string(captured.Data.Candidate.ID), "--reason", reason, "--project", project, "--json"}, false, "")
	if code != ExitOK || diagnostic != "" {
		t.Fatalf("pin select: %d %q", code, diagnostic)
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatal(err)
	}
	last := envelope.Data.Pin.History[len(envelope.Data.Pin.History)-1]
	if last.Reason != reason || last.Review.Mode != evidence.OriginalBase {
		t.Fatalf("default mode or verbatim reason changed: %+v", last)
	}
	code, out, diagnostic = invoke([]string{"pin", string(envelope.Data.Pin.ID), "--project", project, "--json"}, false, "")
	if code != ExitOK || diagnostic != "" || !strings.Contains(out, `"scope":"human_intent"`) {
		t.Fatalf("pin inspection: %d %q %q", code, out, diagnostic)
	}
	evidenceStore, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer evidenceStore.Close()
	if _, err := store.Get[evidence.Pin](evidenceStore, envelope.Data.Pin.ID); err != nil {
		t.Fatalf("pin was not persisted: %v", err)
	}
}

func TestCommandLineDiagnosticsAndSuggestions(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"captrue"}, `after: unknown command "captrue" — did you mean capture?`},
		{[]string{"capture", "--stagd"}, `after: unknown flag "--stagd" — did you mean --staged?`},
		{[]string{"mystery"}, `after: unknown command "mystery" — run after --help to list available commands`},
		{[]string{"capture", "--unrelated"}, `after: unknown flag "--unrelated" — run after capture --help to list this command's options`},
		{[]string{"compare"}, `after: no stored capture is available — run after capture to create one`},
	} {
		code, stdout, stderr := invoke(tc.args, false, "")
		if code != ExitInvalid || stdout != "" || strings.TrimSuffix(stderr, "\n") != tc.want {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q; want %q", tc.args, code, stdout, stderr, tc.want)
		}
	}
	code, stdout, stderr := invoke([]string{"captrue\x1b]52;c;clipboard\a"}, false, "")
	if code != ExitInvalid || stdout != "" || strings.ContainsAny(stderr, "\x1b\a") || !strings.Contains(stderr, "unknown command") || !strings.Contains(stderr, " — ") {
		t.Fatalf("untrusted command diagnostic: %d %q %q", code, stdout, stderr)
	}
}

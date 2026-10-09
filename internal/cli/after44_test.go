//go:build darwin || linux

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

func guidanceExcerpt(output string) string {
	if at := strings.Index(output, "Nothing to review:"); at >= 0 {
		return trimExcerpt(output[at:])
	}
	return trimExcerpt(output)
}

func TestEmptyReviewGuidanceForCommittedBranchAt80Columns(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	home := t.TempDir()
	makeProject(t, project)
	gitRun(t, project, "switch", "-q", "-c", "feature")
	for i := 1; i <= 3; i++ {
		writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println("+string(rune('0'+i))+" ) }\n")
		gitRun(t, project, "add", "app/main.go")
		gitRun(t, project, "commit", "-qm", "feature change")
	}

	for _, noColor := range []bool{false, true} {
		pipeCode, pipeOutput, pipeError, pipeErr := runCLIPipe(project, home, noColor, []string{"review"})
		if pipeErr != nil || pipeCode != ExitInvalid || pipeOutput != "" || !strings.Contains(pipeError, "review requires a terminal") {
			t.Fatalf("non-TTY review NO_COLOR=%t: exit=%d stdout=%q stderr=%q err=%v", noColor, pipeCode, pipeOutput, pipeError, pipeErr)
		}
		code, transcript, _, err := runCLIPTY(project, home, noColor, []string{"review"})
		if err != nil || code != ExitOK {
			t.Fatalf("empty review PTY NO_COLOR=%t: exit=%d transcript=%q err=%v", noColor, code, transcript, err)
		}
		plain := stripThemeSGR(transcript)
		for _, want := range []string{"working tree matches HEAD (commit", "HEAD is 3 commits ahead of main", "after review --base main"} {
			if !strings.Contains(plain, want) {
				t.Errorf("empty review omitted %q (NO_COLOR=%t): %q", want, noColor, plain)
			}
		}
		if strings.Contains(plain, "Diff   Activity") || strings.Contains(plain, "AFTER   project") {
			t.Fatalf("empty review launched the TUI: %q", plain)
		}
		if strings.ContainsAny(plain, "\x1b\a") || (noColor && strings.Contains(transcript, "\x1b[")) {
			t.Fatalf("empty review emitted unexpected terminal controls: %q", transcript)
		}
		t.Logf("80-column PTY review NO_COLOR=%t: %s", noColor, trimExcerpt(plain))
	}
	if _, found, _, err := readReviewSession(project); err != nil || found {
		t.Fatalf("empty review saved a session: found=%v err=%v", found, err)
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	captures, err := s.List("capture")
	s.Close()
	if err != nil || len(captures) != 2 {
		t.Fatalf("empty captures were not persisted: records=%d err=%v", len(captures), err)
	}
}

func TestEmptyCaptureGuidanceForUntrackedIndexAndMergeBase(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "0 space.txt", "not selected\n")
	for _, args := range [][]string{{"capture", "--staged"}, {"capture", "--base", "main"}} {
		args = append(args, "--project", project)
		code, output, stderr := invoke(args, false, "")
		if code != ExitOK || stderr != "" {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", args, code, output, stderr)
		}
		want := "index matches HEAD"
		if strings.Contains(strings.Join(args, " "), "--base") {
			want = "merge-base comparison with main"
		}
		if !strings.Contains(output, want) {
			t.Errorf("%v omitted match explanation: %q", args, output)
		}
		if want == "index matches HEAD" && (!strings.Contains(output, "0 space.txt") || !strings.Contains(output, "after review --include-untracked='0 space.txt'")) {
			t.Errorf("%v omitted excluded-untracked suggestions: %q", args, output)
		}
	}
	for _, noColor := range []bool{false, true} {
		for _, tc := range []struct {
			args []string
			want string
		}{{[]string{"review", "--staged"}, "index matches HEAD"}, {[]string{"review", "--base", "main"}, "merge-base comparison with main"}} {
			code, transcript, _, err := runCLIPTY(project, t.TempDir(), noColor, append(tc.args, "--project", project))
			plain := stripThemeSGR(transcript)
			if err != nil || code != ExitOK || !strings.Contains(plain, tc.want) || strings.Contains(plain, "Diff   Activity") || strings.ContainsAny(plain, "\x1b\a") {
				t.Errorf("%v PTY NO_COLOR=%t: exit=%d output=%q err=%v", tc.args, noColor, code, transcript, err)
			}
		}
	}
	code, output, stderr := invoke([]string{"capture", "--json", "--project", project}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("capture JSON: exit=%d stdout=%q stderr=%q", code, output, stderr)
	}
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 3 || envelope.Data["base_snapshot"] == nil || envelope.Data["candidate_snapshot"] == nil || envelope.Data["index_snapshot"] == nil {
		t.Fatalf("capture JSON shape changed: %s", output)
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	captures, err := s.List("capture")
	s.Close()
	if err != nil || len(captures) < 3 {
		t.Fatalf("empty after capture did not persist each capture record: records=%d err=%v", len(captures), err)
	}
}

func TestEmptyReviewGuidanceNamesBoundedSanitizedUntrackedPathsInPTYAndPipe(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	home := t.TempDir()
	makeProject(t, project)
	gitRun(t, project, "switch", "-q", "-c", "feature")
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(2) }\n")
	gitRun(t, project, "add", "app/main.go")
	gitRun(t, project, "commit", "-qm", "feature change")
	writeProjectFile(t, project, "0 space.txt", "untracked\n")
	writeProjectFile(t, project, "a.txt", "untracked\n")
	writeProjectFile(t, project, "evil\x1b]52;c;clipboard\a.txt", "untracked\n")
	writeProjectFile(t, project, "z-omitted.txt", "untracked\n")

	for _, noColor := range []bool{false, true} {
		for _, mode := range []string{"pipe", "pty"} {
			var code int
			var stdout, stderr string
			var err error
			args := []string{"capture", "--project", project}
			if mode == "pty" {
				code, stdout, stderr, err = runCLIPTY(project, home, noColor, args)
			} else {
				code, stdout, stderr, err = runCLIPipe(project, home, noColor, args)
			}
			transcript := stdout + stderr
			if err != nil || code != ExitOK {
				t.Fatalf("empty capture %s NO_COLOR=%t: exit=%d output=%q err=%v", mode, noColor, code, transcript, err)
			}
			plain := stripThemeSGR(transcript)
			for _, want := range []string{"working tree matches HEAD", "0 space.txt", "a.txt", "evil", "after review --include-untracked='0 space.txt'", "after review --base main"} {
				if !strings.Contains(plain, want) {
					t.Errorf("%s NO_COLOR=%t omitted %q: %q", mode, noColor, want, plain)
				}
			}
			if strings.Contains(plain, "z-omitted.txt") || strings.ContainsAny(plain, "\x1b\a") || (noColor && strings.Contains(transcript, "\x1b[")) {
				t.Fatalf("%s NO_COLOR=%t did not bound/sanitize untracked inventory: %q", mode, noColor, transcript)
			}
			if mode == "pty" {
				t.Logf("80-column PTY capture NO_COLOR=%t: %s", noColor, guidanceExcerpt(plain))
			} else {
				t.Logf("pipe capture NO_COLOR=%t: %s", noColor, guidanceExcerpt(plain))
			}
		}
		code, transcript, _, err := runCLIPTY(project, home, noColor, []string{"review", "--project", project})
		plain := stripThemeSGR(transcript)
		if err != nil || code != ExitOK || !strings.Contains(plain, "after review --base main") || !strings.Contains(plain, "after review --include-untracked='0 space.txt'") || strings.Contains(plain, "Diff   Activity") || strings.ContainsAny(plain, "\x1b\a") {
			t.Fatalf("empty review with excluded paths PTY NO_COLOR=%t: exit=%d output=%q err=%v", noColor, code, transcript, err)
		}
		t.Logf("80-column PTY review with excluded paths NO_COLOR=%t: %s", noColor, guidanceExcerpt(plain))
	}
}

func TestEmptyCaptureOnDefaultBranchSaysNothingIsAhead(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	home := t.TempDir()
	makeProject(t, project)
	code, transcript, _, err := runCLIPTY(project, home, true, []string{"review"})
	if err != nil || code != ExitOK || !strings.Contains(transcript, "Nothing to review") || !strings.Contains(transcript, "HEAD has no commits ahead of main") || strings.Contains(transcript, "--base main") {
		t.Fatalf("default-branch empty review: exit=%d output=%q err=%v", code, transcript, err)
	}
	code, transcript, err = runCLIPTYAnswer(project, home, true, []string{"review", "--new"}, "Diff   Activity", "q")
	if err != nil || code != ExitOK || !strings.Contains(transcript, "AFTER   project") || !strings.Contains(transcript, "Saved review") || strings.Contains(transcript, "Nothing to review") {
		t.Fatalf("--new no longer opens and saves an empty review: exit=%d output=%q err=%v", code, transcript, err)
	}
	if _, found, invalid, err := readReviewSession(project); err != nil || !found || invalid {
		t.Fatalf("--new did not save the empty review: found=%v invalid=%v err=%v", found, invalid, err)
	}
	code, transcript, err = runCLIPTYAnswer(project, home, true, []string{"review"}, "Diff   Activity", "q")
	if err != nil || code != ExitOK || !strings.Contains(transcript, "AFTER   project") || strings.Contains(transcript, "Nothing to review") {
		t.Fatalf("saved empty review did not resume: exit=%d output=%q err=%v", code, transcript, err)
	}
}

func TestUnknownCaptureInventoryStillRequiresReview(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	if err := os.Remove(filepath.Join(project, "app", "main.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing-target", filepath.Join(project, "app", "main.go")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := capture.Capture(context.Background(), project, s, capture.Options{Mode: evidence.WorkingTree})
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	_, _, _, empty, err := capturedPairState(s, evidence.SnapshotPair{Base: result.Base.ID, Candidate: result.Candidate.ID})
	s.Close()
	if err != nil || empty {
		t.Fatalf("unsupported inventory was treated as an empty review: empty=%v err=%v", empty, err)
	}
}

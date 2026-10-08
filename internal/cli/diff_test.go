package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/store"
)

func TestDiffCapturedPatchIsExactAndApplies(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	baseRef, candidateRef := fixtureCommits(t, project)
	code, capturedText, stderr := invoke([]string{"capture", "--base", baseRef, "--target", candidateRef, "--project", project, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("capture: exit=%d stderr=%q", code, stderr)
	}
	var captured struct {
		Data struct {
			Base struct {
				ID evidence.Digest `json:"id"`
			} `json:"base_snapshot"`
			Candidate struct {
				ID evidence.Digest `json:"id"`
			} `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(capturedText), &captured); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	base, err := store.Get[evidence.Snapshot](s, captured.Data.Base.ID)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	candidate, err := store.Get[evidence.Snapshot](s, captured.Data.Candidate.ID)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	stored, err := s.ReadBlob(candidate.Diff)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	code, safeOutput, stderr := invoke([]string{"diff", "--stored", "--project", project}, false, "")
	if code != ExitOK || !bytes.Equal([]byte(safeOutput), stored) || !strings.Contains(stderr, shortID(base.ID)+" → "+shortID(candidate.ID)) || !strings.Contains(stderr, "captured patch") {
		t.Fatalf("stored diff: exit=%d stdout=%q stderr=%q", code, safeOutput, stderr)
	}
	code, exactOutput, stderr := invoke([]string{"diff", "--stored", "--raw", "--project", project}, false, "")
	if code != ExitOK || !bytes.Equal([]byte(exactOutput), stored) || !strings.Contains(stderr, "captured patch") {
		t.Fatalf("raw diff: exit=%d bytes-match=%t stderr=%q", code, bytes.Equal([]byte(exactOutput), stored), stderr)
	}
	// A live read of the same selection generates the same Git patch.
	pipeCode, pipeOutput, pipeStderr, pipeErr := runCLIPipe(project, t.TempDir(), false, []string{"diff", "--raw", "--base", baseRef, "--target", candidateRef})
	if pipeErr != nil || pipeCode != ExitOK || !bytes.Equal([]byte(pipeOutput), stored) || !strings.Contains(pipeStderr, "not stored") {
		t.Fatalf("raw pipe live diff: exit=%d bytes-match=%t stderr=%q err=%v", pipeCode, bytes.Equal([]byte(pipeOutput), stored), pipeStderr, pipeErr)
	}
	code, pairOutput, stderr := invoke([]string{"diff", prefixFor(base.ID), prefixFor(candidate.ID), "--project", project}, false, "")
	if code != ExitOK || !bytes.Equal([]byte(pairOutput), stored) || !strings.Contains(stderr, shortID(base.ID)+" → "+shortID(candidate.ID)) {
		t.Fatalf("explicit captured pair: exit=%d stdout=%q stderr=%q", code, pairOutput, stderr)
	}

	gitRun(t, project, "checkout", "--detach", baseRef)
	apply := exec.Command("git", "apply")
	apply.Dir = project
	apply.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + project, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	apply.Stdin = bytes.NewReader([]byte(exactOutput))
	if output, err := apply.CombinedOutput(); err != nil {
		t.Fatalf("git apply captured patch: %v: %s", err, output)
	}
	if output := gitRun(t, project, "diff", "--quiet", candidateRef, "--"); output != "" {
		t.Fatalf("applied patch does not reproduce candidate: %q", output)
	}
}

func TestDiffComputesAnySnapshotPairAndStatCountsLines(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(1) }\n")
	first := captureForDiff(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(2) }\n")
	second := captureForDiff(t, project)

	code, output, stderr := invoke([]string{"diff", prefixFor(first.Candidate), prefixFor(second.Candidate), "--project", project}, false, "")
	if code != ExitOK || !strings.HasPrefix(output, "diff --git ") || !strings.Contains(output, "-func main() { println(1) }") || !strings.Contains(output, "+func main() { println(2) }") || !strings.Contains(stderr, rawdiff.ComputedOrigin) {
		t.Fatalf("computed pair: exit=%d stdout=%q stderr=%q", code, output, stderr)
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	base, err := store.Get[evidence.Snapshot](s, first.Candidate)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	candidate, err := store.Get[evidence.Snapshot](s, second.Candidate)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	view, err := rawdiff.Open(s, base, candidate)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	computed, err := view.Compute(t.Context())
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	code, rawOutput, stderr := invoke([]string{"diff", prefixFor(first.Candidate), prefixFor(second.Candidate), "--raw", "--project", project}, false, "")
	if code != ExitOK || !bytes.Equal([]byte(rawOutput), computed.Raw) || !strings.Contains(stderr, rawdiff.ComputedOrigin) {
		t.Fatalf("computed raw pair: exit=%d exact=%t stderr=%q", code, bytes.Equal([]byte(rawOutput), computed.Raw), stderr)
	}
	code, stat, stderr := invoke([]string{"diff", prefixFor(first.Candidate), prefixFor(second.Candidate), "--stat", "--project", project}, false, "")
	if code != ExitOK || stat != " app/main.go | 2 +-\n 1 file changed, 1 insertion(+), 1 deletion(-)\n" || !strings.Contains(stderr, rawdiff.ComputedOrigin) {
		t.Fatalf("computed stat: exit=%d stdout=%q stderr=%q", code, stat, stderr)
	}
}

func TestDiffSanitizesHostileStoredPatchAndReportsUncoveredInventory(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	patch := []byte("diff --git a/file b/file\n--- a/file\n+++ b/file\n@@ -1 +1 @@\n-old\n+safe\x1b]52;c;clipboard\a\u009b31m\u202e\xff\n")
	base, candidate := seedHostileDiffPair(t, project, patch)
	code, output, stderr := invoke([]string{"diff", string(base.ID), string(candidate.ID), "--project", project}, false, "")
	if code != ExitOK || !strings.Contains(output, `\u001b]52;c;clipboard\u0007\u009b31m\u202e�`) || strings.ContainsAny(output, "\x1b\a") || bytes.Contains([]byte(output), []byte{0xc2, 0x9b}) {
		t.Fatalf("hostile patch escaped terminal boundary: exit=%d stdout=%q stderr=%q", code, output, stderr)
	}
	for _, want := range []string{shortID(base.ID), shortID(candidate.ID), "captured patch", "uncovered inventory", "untracked; not selected", "unsupported", "unsafe patch bytes were sanitized", "use --raw"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q: %q", want, stderr)
		}
	}
	for _, unsafe := range []string{"\x1b]52", "\a", "\u202e"} {
		if strings.Contains(stderr, unsafe) {
			t.Fatalf("hostile repository content reached stderr: %q", stderr)
		}
	}

	code, stdout, _, err := runCLIPTY(project, t.TempDir(), true, []string{"diff", "--raw", string(base.ID), string(candidate.ID)})
	if err != nil || code != ExitInvalid || !strings.Contains(stdout, "requires non-terminal stdout") {
		t.Fatalf("raw PTY refusal: exit=%d transcript=%q err=%v", code, stdout, err)
	}
}

func captureForDiff(t *testing.T, project string) evidence.Capture {
	t.Helper()
	code, output, stderr := invoke([]string{"capture", "--project", project, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("capture: exit=%d stderr=%q", code, stderr)
	}
	var result struct {
		Data struct {
			Base struct {
				ID evidence.Digest `json:"id"`
			} `json:"base_snapshot"`
			Candidate struct {
				ID evidence.Digest `json:"id"`
			} `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	return evidence.Capture{Base: result.Data.Base.ID, Candidate: result.Data.Candidate.ID}
}

func seedHostileDiffPair(t *testing.T, project string, patch []byte) (evidence.Snapshot, evidence.Snapshot) {
	t.Helper()
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	artifact, err := s.PutArtifact(patch, "diff", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	base, err := store.Put(s, evidence.Snapshot{SchemaVersion: 1, Source: evidence.Commit, Commit: strings.Repeat("a", 40), Diff: artifact.Content, Completeness: evidence.Complete, Files: []evidence.File{}, Excluded: []evidence.Limitation{{Path: "excluded.txt", Reason: "untracked; not selected"}}, Unsupported: []evidence.Limitation{}})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := store.Put(s, evidence.Snapshot{SchemaVersion: 1, Source: evidence.WorkingTree, Commit: strings.Repeat("a", 40), Diff: artifact.Content, Completeness: evidence.Incomplete, Files: []evidence.File{}, Excluded: []evidence.Limitation{}, Unsupported: []evidence.Limitation{{Path: "odd\x1b]52;c;payload\a.bin", Reason: "unsupported Git mode 120000"}}, Limits: []string{"unsupported captured content"}})
	if err != nil {
		t.Fatal(err)
	}
	return base, candidate
}

func TestDiffLargeCapturedPatchStreamsWithinMemoryBudget(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	const lines = 100250
	patch := []byte("diff --git a/large.txt b/large.txt\n" + strings.Repeat(" "+strings.Repeat("x", 73)+"\n", lines-1))
	base, candidate := seedHostileDiffPair(t, project, patch)
	args := []string{"diff", string(base.ID), string(candidate.ID), "--project", project}
	for _, raw := range []bool{false, true} {
		command := append([]string(nil), args...)
		if raw {
			command = append(command, "--raw")
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		var stderr bytes.Buffer
		outputHash := sha256.New()
		code := run(context.Background(), command, outputHash, &stderr, strings.NewReader(""), false)
		runtime.ReadMemStats(&after)
		allocated := after.TotalAlloc - before.TotalAlloc
		if code != ExitOK || allocated > 64<<20 {
			t.Fatalf("large diff raw=%t: exit=%d allocations=%d stderr=%q", raw, code, allocated, stderr.String())
		}
		expectedHash := sha256.Sum256(patch)
		if !bytes.Equal(outputHash.Sum(nil), expectedHash[:]) {
			t.Fatalf("large diff raw=%t did not emit the entire patch", raw)
		}
		t.Logf("100250-line diff raw=%t: allocated %d bytes (64 MiB budget)", raw, allocated)
	}
}

func TestCaptureAndStatusNextSuggestDiff(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(1) }\n")
	code, output, stderr := invoke([]string{"capture", "--project", project}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(output, "after diff --stored") {
		t.Fatalf("capture Next omitted after diff: exit=%d stdout=%q stderr=%q", code, output, stderr)
	}
	code, output, stderr = invoke([]string{"status", "--project", project}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(output, "after diff --stored") {
		t.Fatalf("status Next omitted after diff: exit=%d stdout=%q stderr=%q", code, output, stderr)
	}
	code, _, stderr = invoke([]string{"diff", "--raw", "--stat", "--project", project}, false, "")
	if code != ExitInvalid || !strings.Contains(stderr, "cannot be combined") {
		t.Fatalf("conflicting diff modes were accepted: exit=%d stderr=%q", code, stderr)
	}
	code, _, stderr = invoke([]string{"diff", "--json", "--project", project}, false, "")
	if code != ExitInvalid || !strings.Contains(stderr, "not JSON") {
		t.Fatalf("diff unexpectedly accepted JSON mode: exit=%d stderr=%q", code, stderr)
	}
}

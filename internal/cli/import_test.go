package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/store"
)

func runImportWithReader(args []string, tty bool, input io.Reader) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, &stdout, &stderr, input, tty)
	return code, stdout.String(), stderr.String()
}

func TestImportAcceptsFileDashAndPipedInput(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	code, captured, stderr := invoke([]string{"capture", "--project", project, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("capture: exit=%d stderr=%q", code, stderr)
	}
	var captureResult struct {
		Data struct {
			Candidate struct {
				ID evidence.Digest `json:"id"`
			} `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(captured), &captureResult); err != nil {
		t.Fatal(err)
	}

	input := []byte("{\"Action\":\"run\",\"Package\":\"example.test\",\"Test\":\"TestImport\"}\n" +
		"{\"Action\":\"pass\",\"Package\":\"example.test\",\"Test\":\"TestImport\"}\n")
	reportPath := filepath.Join(t.TempDir(), "report.jsonl")
	if err := os.WriteFile(reportPath, input, 0600); err != nil {
		t.Fatal(err)
	}
	common := []string{"--project", project, "--snapshot", string(captureResult.Data.Candidate.ID), "--producer", "go test caller", "--captured-at", "2026-01-02T03:04:05Z", "--json"}
	cases := []struct {
		name  string
		args  []string
		input io.Reader
	}{
		{name: "file", args: append([]string{"import", reportPath}, common...)},
		{name: "dash", args: append([]string{"import", "-"}, common...), input: bytes.NewReader(input)},
		{name: "implicit-pipe", args: append([]string{"import"}, common...), input: bytes.NewReader(input)},
	}
	var want []byte
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, output, diagnostic := runImportWithReader(tc.args, false, tc.input)
			if code != ExitOK || diagnostic != "" {
				t.Fatalf("import: exit=%d diagnostic=%q output=%q", code, diagnostic, output)
			}
			var result struct {
				Kind string `json:"kind"`
				Data struct {
					ID       evidence.Digest       `json:"id"`
					Metadata gotestreport.Metadata `json:"metadata"`
					Digest   evidence.Digest       `json:"original_digest"`
					Cards    []gotestreport.Card   `json:"cards"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatal(err)
			}
			if result.Kind != "import" || result.Data.Digest == "" || result.Data.Metadata.Snapshot != captureResult.Data.Candidate.ID || result.Data.Metadata.Producer != "go test caller" || len(result.Data.Cards) != 1 {
				t.Fatalf("unexpected imported report: %s", output)
			}
			// Import time is intentionally per invocation; all report facts and
			// the selected binding must otherwise be identical for equal bytes.
			result.Data.ID = ""
			result.Data.Metadata.ImportedAt = time.Time{}
			encoded, err := json.Marshal(result.Data)
			if err != nil {
				t.Fatal(err)
			}
			if want == nil {
				want = encoded
			} else if !bytes.Equal(want, encoded) {
				t.Fatalf("input mode changed report facts:\n%s\n%s", want, encoded)
			}
		})
	}
}

func TestImportDefaultsToRecordedWorkingTreeBindingAndWarnsAboutUntracked(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "notes.txt", "potential test input\n")
	reportPath := filepath.Join(t.TempDir(), "report.jsonl")
	if err := os.WriteFile(reportPath, []byte("{\"Action\":\"pass\",\"Package\":\"example.test\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	code, output, diagnostic := invoke([]string{"import", reportPath, "--project", project, "--json"}, false, "")
	if code != ExitOK || diagnostic != "" {
		t.Fatalf("import: exit=%d diagnostic=%q", code, diagnostic)
	}
	var result struct {
		Data struct {
			ID       evidence.Digest            `json:"id"`
			Metadata map[string]json.RawMessage `json:"metadata"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	if !validDigest(string(result.Data.ID)) || result.Data.Metadata["producer"] != nil || result.Data.Metadata["snapshot"] == nil {
		t.Fatalf("missing optional producer or default candidate binding: %s", output)
	}
	var candidate evidence.Digest
	if err := json.Unmarshal(result.Data.Metadata["snapshot"], &candidate); err != nil || !validDigest(string(candidate)) {
		t.Fatalf("invalid candidate binding: %s %v", result.Data.Metadata["snapshot"], err)
	}

	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := store.Get[evidence.Snapshot](s, candidate)
	if err != nil || bound.Source != evidence.WorkingTree || len(bound.Excluded) != 1 || bound.Excluded[0].Path != "notes.txt" {
		s.Close()
		t.Fatalf("default binding did not use the working tree policy: %+v %v", bound, err)
	}
	history, err := store.CapturesForSnapshot(s, candidate)
	if err != nil || len(history.Records) == 0 || history.Records[0].Candidate != candidate {
		s.Close()
		t.Fatalf("default binding has no capture history: %+v %v", history, err)
	}
	s.Close()

	code, readable, diagnostic := invoke([]string{"import", reportPath, "--project", project}, false, "")
	if code != ExitOK || diagnostic != "" {
		t.Fatalf("readable import: exit=%d diagnostic=%q", code, diagnostic)
	}
	for _, text := range []string{"Producer     not stated", "--producer TEXT", "caller claim", "Capture", "tests may have used them", "AFTER did not run or observe these tests"} {
		if !strings.Contains(readable, text) {
			t.Errorf("readable import omitted %q: %s", text, readable)
		}
	}

	code, boundReadable, diagnostic := invoke([]string{"import", reportPath, "--project", project, "--snapshot", string(candidate)}, false, "")
	if code != ExitOK || diagnostic != "" || !strings.Contains(boundReadable, "tests may have used them") || strings.Contains(boundReadable, "captured at import") {
		t.Fatalf("explicit snapshot binding omitted its exclusion warning or claimed a new capture: exit=%d diagnostic=%q output=%s", code, diagnostic, boundReadable)
	}
}

func TestImportWithoutTerminalInputDoesNotReadAndShowsForms(t *testing.T) {
	reader := &readCountReader{}
	code, output, diagnostic := runImportWithReader([]string{"import"}, true, reader)
	if code != ExitInvalid || output != "" || reader.read != 0 || !strings.Contains(diagnostic, "after import FILE") || !strings.Contains(diagnostic, "go test -json ./... | after import") {
		t.Fatalf("missing terminal input: exit=%d output=%q diagnostic=%q reads=%d", code, output, diagnostic, reader.read)
	}
}

func TestImportStdinRemainsBounded(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	reader := &readCountReader{remaining: gotestreport.MaxBytes + 99}
	code, output, diagnostic := runImportWithReader([]string{"import", "-", "--project", project}, false, reader)
	if code != ExitInvalid || output != "" || reader.read != gotestreport.MaxBytes+1 || !strings.Contains(diagnostic, "8 MiB") {
		t.Fatalf("stdin input was not bounded: exit=%d reads=%d diagnostic=%q", code, reader.read, diagnostic)
	}
}

type readCountReader struct {
	remaining int
	read      int
}

func (r *readCountReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.remaining)
	clear(p[:n])
	r.remaining -= n
	r.read += n
	return n, nil
}

func TestImportBindingCaptureFailureSuggestsSnapshotAndStoresNoReport(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	gitRun(t, project, "checkout", "-qb", "other")
	writeProjectFile(t, project, "app/main.go", "package main\nconst branch = 1\n")
	gitRun(t, project, "add", "app/main.go")
	gitRun(t, project, "commit", "-qm", "other branch")
	gitRun(t, project, "checkout", "-q", "main")
	writeProjectFile(t, project, "app/main.go", "package main\nconst mainLine = 1\n")
	gitRun(t, project, "add", "app/main.go")
	gitRun(t, project, "commit", "-qm", "main branch")
	merge := exec.Command("git", "merge", "other")
	merge.Dir = project
	merge.Env = []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + project, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=AFTER fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=AFTER fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
	}
	if output, err := merge.CombinedOutput(); err == nil {
		t.Fatalf("expected a conflicted merge, got %s", output)
	}

	reportPath := filepath.Join(t.TempDir(), "report.jsonl")
	if err := os.WriteFile(reportPath, []byte("{\"Action\":\"pass\",\"Package\":\"example.test\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, output, diagnostic := invoke([]string{"import", reportPath, "--project", project}, false, "")
	if code != ExitOperational || output != "" || !strings.Contains(diagnostic, "capture failed: unmerged index is unsupported") || !strings.Contains(diagnostic, "--snapshot ID") {
		t.Fatalf("capture failure was hidden or report emitted: exit=%d output=%q diagnostic=%q", code, output, diagnostic)
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	artifacts, err := s.List("artifact")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range artifacts {
		artifact, err := s.ReadArtifactMetadata(entry.ID)
		if err == nil && artifact.Channel == "go-test-report-v1" {
			t.Fatal("report was persisted after default binding capture failed")
		}
	}
}

func TestReviewImportFileAcceptsOmittedProducer(t *testing.T) {
	code, output, diagnostic := invoke([]string{"review", "--import-file", "report.jsonl"}, false, "")
	if code != ExitInvalid || output != "" || strings.Contains(diagnostic, "producer") || !strings.Contains(diagnostic, "review requires a terminal") {
		t.Fatalf("review rejected an optional producer before its terminal check: exit=%d output=%q diagnostic=%q", code, output, diagnostic)
	}
}

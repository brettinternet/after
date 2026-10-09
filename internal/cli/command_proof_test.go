package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/store"
)

const commandFixtureSource = `package main

import (
	"fmt"
	"io"
	"os"
)

const changedValue = "before"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "changed" {
		fmt.Println(changedValue)
		return
	}
	fmt.Print("stable control: ")
	io.Copy(os.Stdout, os.Stdin)
}
`

const commandFixtureStdin = "frozen stdin bytes\n"

func TestCommandCLIProof(t *testing.T) {
	if os.Getenv("AFTER_COMMAND_PROOF") != "1" {
		t.Skip("task command:proof authorizes the provisioned Go image")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Fatal("command proof requires a supported Linux image platform")
	}
	docker, host := os.Getenv("AFTER_DOCKER_BINARY"), os.Getenv("AFTER_DOCKER_HOST")
	if docker == "" || host == "" {
		t.Fatal("command proof requires explicit AFTER_DOCKER_BINARY and AFTER_DOCKER_HOST")
	}
	root := filepath.Join(t.TempDir(), "command-fixture")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	writeProjectFile(t, root, "main.go", commandFixtureSource)
	rawDefinition, err := os.ReadFile("../runner/testdata/command-proof.json")
	if err != nil {
		t.Fatal(err)
	}
	definition, err := runner.ParseCommandDefinition(rawDefinition)
	if err != nil || string(definition.Cases[1].Stdin) != commandFixtureStdin {
		t.Fatalf("invalid command proof definition: %v", err)
	}
	if definition.Platform != "linux/"+runtime.GOARCH {
		definition.Platform = "linux/" + runtime.GOARCH
		rawDefinition, err = json.Marshal(definition)
		if err != nil {
			t.Fatal(err)
		}
	}
	writeProjectFile(t, root, "command.json", string(rawDefinition))
	gitRun(t, root, "init", "-q", "--template=", "-b", "main")
	gitRun(t, root, "add", "main.go", "command.json")
	gitRun(t, root, "commit", "-qm", "synthetic command base")
	writeProjectFile(t, root, "main.go", strings.Replace(commandFixtureSource, `"before"`, `"after"`, 1))

	projectConfig := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(projectConfig, []byte("interactive: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	captureCode, captureRaw, captureErr := commandCLI(t, []string{"capture", "--project", root, "--config", projectConfig, "--json"})
	if captureCode != ExitOK || captureErr != "" {
		t.Fatalf("capture exit=%d stderr=%q stdout=%q", captureCode, captureErr, captureRaw)
	}
	var captureResult struct {
		Data struct {
			Base      snapshotSummary `json:"base_snapshot"`
			Candidate snapshotSummary `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(captureRaw), &captureResult); err != nil || captureResult.Data.Base.ID == "" || captureResult.Data.Candidate.ID == "" {
		t.Fatalf("capture did not return a snapshot pair: %v %s", err, captureRaw)
	}
	planPath := filepath.Join(t.TempDir(), "command-plan.json")
	common := []string{"--project", root, "--config", projectConfig, "--json"}
	dockerOptions := []string{"--docker-binary", docker, "--docker-host", host}
	previewArgs := []string{"run", string(captureResult.Data.Base.ID), string(captureResult.Data.Candidate.ID), "--definition", filepath.Join(root, "command.json"), "--plan-out", planPath}
	previewArgs = append(previewArgs, common...)
	previewArgs = append(previewArgs, dockerOptions...)
	previewCode, previewRaw, previewErr := commandCLI(t, previewArgs)
	if previewCode != ExitDenied || previewErr != "" {
		t.Fatalf("consent preview exit=%d stderr=%q stdout=%q args=%q base=%q candidate=%q", previewCode, previewErr, previewRaw, previewArgs, captureResult.Data.Base.ID, captureResult.Data.Candidate.ID)
	}
	var preview struct {
		Data struct {
			Authorization string `json:"authorization_digest"`
			Consent       string `json:"consent_summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(previewRaw), &preview); err != nil || preview.Data.Authorization == "" || !strings.Contains(preview.Data.Consent, "changed") || !strings.Contains(preview.Data.Consent, "control") {
		t.Fatalf("consent preview did not disclose command cases: %v %s", err, previewRaw)
	}
	runArgs := append([]string{"run", "--plan-file", planPath, "--approve", preview.Data.Authorization}, common...)
	runArgs = append(runArgs, dockerOptions...)
	runCode, runRaw, runErr := commandCLI(t, runArgs)
	if runErr != "" || (runCode != ExitFinding && runCode != ExitOK) {
		t.Fatalf("approved command run exit=%d stderr=%q stdout=%q", runCode, runErr, runRaw)
	}
	var runResult struct {
		Data struct {
			Status     string              `json:"status"`
			Receipt    evidence.Receipt    `json:"receipt"`
			Comparison evidence.Comparison `json:"comparison"`
			Details    compare.Report      `json:"details"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(runRaw), &runResult); err != nil || runResult.Data.Status != "completed" || runResult.Data.Receipt.State.Execution != evidence.Completed || runResult.Data.Comparison.Completeness != evidence.Complete || runResult.Data.Comparison.Outcome != evidence.Different {
		t.Fatalf("changed command case did not produce a complete difference: %v %+v", err, runResult.Data)
	}
	changed, control := false, false
	for _, witness := range runResult.Data.Details.Witnesses {
		if witness.Relation != "paired" || witness.Channel != "stdout" {
			continue
		}
		if witness.Before.CaseID == "changed" && witness.Outcome == evidence.Different {
			changed = true
		}
		if witness.Before.CaseID == "control" && witness.Outcome == evidence.Equal {
			control = true
		}
	}
	if !changed || !control {
		t.Fatalf("changed/control output witnesses missing: changed=%t control=%t", changed, control)
	}
	// Equal control witnesses would also pass if both sides read EOF; the
	// container-boundary stdout must contain the frozen stdin bytes.
	s, err := store.Open(root, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	controlStreams := 0
	for _, artifact := range runResult.Data.Receipt.Artifacts {
		if !strings.HasSuffix(artifact.Channel, "/control/0/stdout") {
			continue
		}
		controlStdout, err := s.ReadBlob(artifact.Content)
		if err != nil || string(controlStdout) != "stable control: "+commandFixtureStdin {
			t.Fatalf("%s did not receive frozen stdin: %q %v", artifact.Channel, controlStdout, err)
		}
		controlStreams++
	}
	if controlStreams != 2 {
		t.Fatalf("want base and candidate control stdout, got %d", controlStreams)
	}

	pinCode, pinRaw, pinErr := commandCLI(t, append([]string{"pin", string(runResult.Data.Receipt.ID), "--expectation", "Changed command output is intentional; control output stays fixed", "--scope", "finite_example", "--reason", "synthetic command proof"}, common...))
	if pinCode != ExitOK || pinErr != "" {
		t.Fatalf("pin exit=%d stderr=%q stdout=%q", pinCode, pinErr, pinRaw)
	}
	var pinned struct {
		Data review.View `json:"data"`
	}
	if err := json.Unmarshal([]byte(pinRaw), &pinned); err != nil || pinned.Data.Pin.Decision != evidence.Pinned || pinned.Data.CurrentReceipt.ID != runResult.Data.Receipt.ID {
		t.Fatalf("command receipt was not pinned: %v %+v", err, pinned.Data.Pin)
	}
	reopenCode, reopenRaw, reopenErr := commandCLI(t, append([]string{"pin", string(pinned.Data.Pin.ID), "--select", string(captureResult.Data.Base.ID), "--mode", "original_base", "--reason", "reinspect original command implementation"}, common...))
	if reopenCode != ExitOK || reopenErr != "" {
		t.Fatalf("reopen exit=%d stderr=%q stdout=%q", reopenCode, reopenErr, reopenRaw)
	}
	var reopened struct {
		Data review.View `json:"data"`
	}
	if err := json.Unmarshal([]byte(reopenRaw), &reopened); err != nil || reopened.Data.Pin.Decision != evidence.Reopened || reopened.Data.Applicability != evidence.Stale || !reopened.Data.MissingCurrentResult || reopened.Data.CurrentReceipt != nil {
		t.Fatalf("command pin did not reopen on original snapshot selection: %v %+v", err, reopened.Data.Pin)
	}
	t.Logf("Captured changed/control command cases, approved exact plan %s, compared exit/stdout/stderr, pinned the result, and reopened it on explicit original-base selection", preview.Data.Authorization)
}

func commandCLI(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(t.Context(), args, &stdout, &stderr, bytes.NewReader(nil), false)
	return code, stdout.String(), stderr.String()
}

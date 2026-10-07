//go:build darwin || linux

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
	"github.com/creack/pty"
)

const cliProcessHelperEnv = "AFTER_CLI_PROCESS_HELPER"

// TestCLIProcessHelper lets the test binary serve as an actual AFTER process.
func TestCLIProcessHelper(t *testing.T) {
	if os.Getenv(cliProcessHelperEnv) != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Exit(Run(os.Args[i+1:], os.Stdout, os.Stderr))
		}
	}
	os.Exit(125)
}

func TestProjectCommandProcessModes(t *testing.T) {
	project := t.TempDir()
	baseRef, candidateRef := fixtureCommits(t, project)
	nested := filepath.Join(project, "internal", "deep")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(project, "report.jsonl")
	if err := os.WriteFile(report, []byte("{\"Action\":\"pass\",\"Package\":\"fixture\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	captureArgs := []string{"capture", "--base", baseRef, "--target", candidateRef}
	code, output, stderr, err := runCLIPipe(nested, home, false, append(append([]string(nil), captureArgs...), "--json"))
	if err != nil || code != ExitOK || stderr != "" {
		t.Fatalf("initial pipe capture: exit=%d stdout=%q stderr=%q err=%v", code, output, stderr, err)
	}
	var captured struct {
		Data struct {
			Base struct {
				ID string `json:"id"`
			} `json:"base_snapshot"`
			Candidate struct {
				ID string `json:"id"`
			} `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &captured); err != nil {
		t.Fatal(err)
	}
	baseID, candidateID := captured.Data.Base.ID, captured.Data.Candidate.ID
	if !validDigest(baseID) || !validDigest(candidateID) {
		t.Fatalf("capture IDs are invalid: %s", output)
	}
	receiptID, artifactID := seedCLIProcessRecords(t, project, baseID, candidateID)
	importedCode, importedOutput, importedError, err := runCLIPipe(nested, home, true, []string{"import", report, "--producer", "fixture", "--json"})
	if err != nil || importedCode != ExitOK || importedError != "" {
		t.Fatalf("JSON import setup: exit=%d output=%q stderr=%q err=%v", importedCode, importedOutput, importedError, err)
	}
	var imported struct {
		Data struct {
			ID evidence.Digest `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(importedOutput), &imported); err != nil {
		t.Fatal(err)
	}
	comparisonCode, comparisonOutput, comparisonError, err := runCLIPipe(nested, home, true, []string{"compare", string(receiptID), "--json"})
	if err != nil || comparisonCode != ExitOperational || comparisonError != "" {
		t.Fatalf("JSON compare setup: exit=%d output=%q stderr=%q err=%v", comparisonCode, comparisonOutput, comparisonError, err)
	}
	var compared struct {
		Data struct {
			Comparison evidence.Comparison `json:"comparison"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(comparisonOutput), &compared); err != nil {
		t.Fatal(err)
	}
	pinCode, pinOutput, pinError, err := runCLIPipe(nested, home, true, []string{"pin", string(receiptID), "--scope", "human_intent", "--expectation", "finite synthetic review", "--reason", "process-mode fixture", "--json"})
	if err != nil || pinCode != ExitOK || pinError != "" {
		t.Fatalf("JSON pin setup: exit=%d output=%q stderr=%q err=%v", pinCode, pinOutput, pinError, err)
	}
	var pinned struct {
		Data struct {
			Pin evidence.Pin `json:"pin"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(pinOutput), &pinned); err != nil {
		t.Fatal(err)
	}
	commands := []struct {
		name string
		args []string
		code int
	}{
		{"capture", captureArgs, ExitOK},
		{"import", []string{"import", report, "--producer", "fixture", "--snapshot", candidateID}, ExitOK},
		{"inspect-pair", []string{"inspect", candidateID, "--base", baseID}, ExitOK},
		{"inspect-snapshot", []string{"inspect", candidateID}, ExitOK},
		{"inspect-receipt", []string{"inspect", string(receiptID)}, ExitOK},
		{"inspect-comparison", []string{"inspect", string(compared.Data.Comparison.ID)}, ExitOperational},
		{"inspect-report", []string{"inspect", string(imported.Data.ID)}, ExitOK},
		{"inspect-artifact", []string{"inspect", string(artifactID)}, ExitOK},
		{"compare", []string{"compare", string(receiptID)}, ExitOperational},
		{"export", []string{"export", string(compared.Data.Comparison.ID)}, ExitOperational},
		{"run", []string{"run", baseID, candidateID, "--interactive=false"}, ExitDenied},
		{"pin", []string{"pin", string(receiptID), "--expectation", "finite synthetic review", "--scope", "human_intent", "--reason", "process-mode fixture"}, ExitOK},
		{"review", []string{"review", string(pinned.Data.Pin.ID)}, ExitOK},
		{"review-select", []string{"review", string(pinned.Data.Pin.ID), "--select", candidateID, "--mode", "original_base", "--reason", "prefix selection"}, ExitOK},
		{"review-receipt", []string{"review", string(pinned.Data.Pin.ID), "--receipt", string(receiptID), "--reason", "prefix attachment"}, ExitOK},
		{"run-short-approval", []string{"run", "--plan-file", "not-read", "--approve", "abcd"}, ExitInvalid},
		{"inspect-no-match", []string{"inspect", strings.Repeat("0", 64)}, ExitInvalid},
		{"config", []string{"config"}, ExitOK},
	}
	for _, command := range commands {
		// Exercise every ID argument/flag with uppercase optional-scheme prefixes
		// in real 80-column terminals and OS pipes, not just in-memory writers.
		for i, arg := range command.args {
			if validDigest(arg) {
				command.args[i] = "SHA256:" + prefixFor(evidence.Digest(arg))
			}
		}
		for _, noColor := range []bool{false, true} {
			for _, mode := range []string{"pipe", "pty"} {
				t.Run(command.name+"/no-color="+map[bool]string{true: "yes", false: "no"}[noColor]+"/"+mode, func(t *testing.T) {
					var code int
					var stdout, stderr string
					var err error
					if mode == "pty" {
						code, stdout, stderr, err = runCLIPTY(nested, home, noColor, command.args)
					} else {
						code, stdout, stderr, err = runCLIPipe(nested, home, noColor, command.args)
					}
					if err != nil || code != command.code {
						t.Fatalf("exit=%d want=%d stdout=%q stderr=%q err=%v", code, command.code, stdout, stderr, err)
					}
					transcript := stdout + stderr
					if strings.Contains(transcript, outsideRepositoryDiagnostic) || strings.Contains(transcript, "cannot open private evidence store") {
						t.Fatalf("command did not use the checkout store: %q", transcript)
					}
					if strings.ContainsAny(transcript, "\x1b\a") {
						if mode != "pty" || noColor || !themeSGROnly(transcript) {
							t.Fatalf("unexpected terminal control in %s output: %q", mode, transcript)
						}
					}
					if command.name == "capture" && noColor && mode == "pty" {
						t.Logf("80-column PTY capture excerpt: %s", trimExcerpt(transcript))
					}
					if command.name == "inspect-snapshot" && noColor && mode == "pty" {
						t.Logf("80-column PTY snapshot inspect excerpt: %s", trimExcerpt(transcript))
					}
					if (command.name == "compare" || command.name == "review" || command.name == "run-short-approval" || command.name == "inspect-no-match") && !noColor && mode == "pipe" {
						t.Logf("pipe diagnostic excerpt: %s", trimExcerpt(transcript))
					}
					if _, err := os.Stat(filepath.Join(project, ".after", ".gitignore")); err != nil {
						t.Fatalf("checkout-root store missing: %v", err)
					}
					if _, err := os.Stat(filepath.Join(nested, ".after")); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("nested store was created: %v", err)
					}
				})
			}
		}
	}

	outside := t.TempDir()
	for _, command := range []struct {
		name string
		args []string
		want string
	}{
		{"help", []string{"help"}, "COMMANDS:"},
		{"version", []string{"version"}, Version},
		{"config", []string{"config"}, "configuration"},
	} {
		for _, noColor := range []bool{false, true} {
			for _, mode := range []string{"pipe", "pty"} {
				var code int
				var stdout, stderr string
				var err error
				if mode == "pty" {
					code, stdout, stderr, err = runCLIPTY(outside, home, noColor, command.args)
				} else {
					code, stdout, stderr, err = runCLIPipe(outside, home, noColor, command.args)
				}
				if err != nil || code != ExitOK || stderr != "" || !strings.Contains(stdout, command.want) {
					t.Errorf("%s outside Git via %s (NO_COLOR=%v): exit=%d stdout=%q stderr=%q err=%v", command.name, mode, noColor, code, stdout, stderr, err)
				}
			}
		}
	}
	if _, err := os.Stat(filepath.Join(outside, ".after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("help/version/config created private storage: %v", err)
	}
}

func seedCLIProcessRecords(t *testing.T, project string, base, candidate string) (evidence.Digest, evidence.Digest) {
	t.Helper()
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	input, err := s.PutArtifact([]byte(`{"fixture":true}`), "fixture-input", 1024)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := store.Put(s, evidence.Scenario{SchemaVersion: 1, Input: input.Content, Driver: processDigest("1"), Observer: processDigest("2"), Rules: processDigest("3"), Boundary: "synthetic process proof", Author: "test fixture", Limits: []string{"finite synthetic input"}})
	if err != nil {
		t.Fatal(err)
	}
	bindings := &evidence.Bindings{Scenario: scenario.ID, Input: scenario.Input, Driver: scenario.Driver, Observer: scenario.Observer, Rules: scenario.Rules}
	environment := func(char string) *evidence.Environment {
		return &evidence.Environment{Environment: processDigest(char), Toolchain: processDigest(char), Dependencies: processDigest(char), Argv: []string{"fixture"}}
	}
	receipt, err := store.Put(s, evidence.Receipt{
		SchemaVersion: 1,
		State:         evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.NoEvidence, Applicability: evidence.Unknown, Execution: evidence.Failed, Comparison: evidence.Incomparable, Report: evidence.NoReport},
		Snapshots:     evidence.SnapshotPair{Base: evidence.Digest(base), Candidate: evidence.Digest(candidate)},
		Bindings:      bindings, BaseEnvironment: environment("4"), CandidateEnvironment: environment("5"), Authorization: processDigest("6"),
		StartedAt: time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC), FinishedAt: time.Date(2026, time.January, 2, 10, 1, 0, 0, time.UTC),
		Completeness: evidence.Incomplete, Artifacts: []evidence.Artifact{}, Limits: []string{"synthetic incomplete run; no observed values"},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := s.PutArtifact([]byte("fixture artifact\n"), "fixture", 1024)
	if err != nil {
		t.Fatal(err)
	}
	return receipt.ID, artifact.Content
}

func processDigest(char string) evidence.Digest {
	return evidence.Digest("sha256:" + strings.Repeat(char, 64))
}

func runCLIPipe(dir, home string, noColor bool, args []string) (int, string, string, error) {
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestCLIProcessHelper$", "--"}, args...)...)
	cmd.Dir = dir
	cmd.Env = cliProcessEnv(home, noColor)
	cmd.Stdin = strings.NewReader("")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code, err := processExitCode(err)
	return code, stdout.String(), stderr.String(), err
}

func runCLIPTY(dir, home string, noColor bool, args []string) (int, string, string, error) {
	master, slave, err := pty.Open()
	if err != nil {
		return -1, "", "", err
	}
	defer master.Close()
	if err := pty.Setsize(master, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		slave.Close()
		return -1, "", "", err
	}
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestCLIProcessHelper$", "--"}, args...)...)
	cmd.Dir = dir
	cmd.Env = cliProcessEnv(home, noColor)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	if err := cmd.Start(); err != nil {
		slave.Close()
		return -1, "", "", err
	}
	_ = slave.Close()
	var output strings.Builder
	readDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(&output, master)
		readDone <- err
	}()
	waitErr := cmd.Wait()
	readErr := <-readDone
	if readErr != nil && !errors.Is(readErr, os.ErrClosed) && !errors.Is(readErr, syscall.EIO) {
		return -1, output.String(), "", readErr
	}
	code, err := processExitCode(waitErr)
	return code, output.String(), "", err
}

func cliProcessEnv(home string, noColor bool) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		cliProcessHelperEnv + "=1",
	}
	if noColor {
		env = append(env, "NO_COLOR=1")
	}
	return env
}

func processExitCode(err error) (int, error) {
	if err == nil {
		return ExitOK, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return -1, err
}

func stripThemeSGR(value string) string {
	for _, sequence := range []string{"\x1b[0m", "\x1b[32m", "\x1b[1;35m", "\x1b[1;33m", "\x1b[1;31m", "\x1b[34m", "\x1b[36m", "\x1b[2m", "\x1b[1m", "\x1b[7m"} {
		value = strings.ReplaceAll(value, sequence, "")
	}
	return value
}

func themeSGROnly(value string) bool {
	return !strings.Contains(stripThemeSGR(value), "\x1b")
}

func trimExcerpt(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.TrimSpace(value)
	if len(value) > 180 {
		return value[:180] + "…"
	}
	return value
}

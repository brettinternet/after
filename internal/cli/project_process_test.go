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
	code, output, stderr, err := runCLIPipe(nested, home, false, captureArgs)
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
	fakeID := "sha256:" + strings.Repeat("f", 64)
	commands := []struct {
		name string
		args []string
		code int
	}{
		{"capture", captureArgs, ExitOK},
		{"import", []string{"import", report, "--producer", "fixture"}, ExitOK},
		{"inspect", []string{"inspect", candidateID}, ExitOK},
		{"compare", []string{"compare", fakeID}, ExitInvalid},
		{"export", []string{"export", candidateID}, ExitInvalid},
		{"run", []string{"run", baseID, candidateID, "--interactive=false"}, ExitDenied},
		{"pin", []string{"pin", fakeID, "--expectation", "fixture", "--scope", "finite_example", "--reason", "fixture"}, ExitInvalid},
		{"review", []string{"review", fakeID}, ExitInvalid},
	}
	for _, command := range commands {
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
						t.Fatalf("unexpected terminal control in %s output: %q", mode, transcript)
					}
					if command.name == "capture" && noColor && mode == "pty" {
						t.Logf("80-column PTY capture excerpt: %s", trimExcerpt(transcript))
					}
					if command.name == "compare" && !noColor && mode == "pipe" {
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

func trimExcerpt(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.TrimSpace(value)
	if len(value) > 180 {
		return value[:180] + "…"
	}
	return value
}

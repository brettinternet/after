package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/config"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

func TestBareRunStoresExactPlanAndInspectSharesConsentDecoder(t *testing.T) {
	project := filepath.Join(t.TempDir(), "payment")
	t.Setenv("AFTER_DOCKER_BINARY", "")
	t.Setenv("AFTER_DOCKER_HOST", "")
	base, candidate := fixtureCommits(t, project)
	code, _, stderr := invoke([]string{"capture", "--base", base, "--target", candidate, "--project", project, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("capture: %d %q", code, stderr)
	}

	code, output, stderr := invoke([]string{"run", "--project", project, "--json"}, false, "")
	if code != ExitDenied || stderr != "" {
		t.Fatalf("bare run preview: %d %q", code, stderr)
	}
	var result struct {
		Data executionPreview `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	preview := result.Data
	if preview.Status != "authorization_required" || !validDigest(preview.Authorization) || preview.PlanBytes <= 0 || preview.Using == nil || preview.Using.Capture == "" {
		t.Fatalf("bare run did not resolve and name the newest capture: %+v", preview)
	}
	var pair struct {
		Snapshots evidence.SnapshotPair `json:"snapshots"`
	}
	if err := json.Unmarshal(preview.Plan, &pair); err != nil || pair.Snapshots.Base == "" || pair.Snapshots.Candidate == "" {
		t.Fatalf("preview lacks a complete plan pair: %s %v", preview.Plan, err)
	}
	storePath := filepath.Join(project, ".after", "plan-"+preview.Authorization[7:])
	info, err := os.Stat(storePath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("stored plan is not private: %v %v", info, err)
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	exact, err := s.ReadPlan(evidence.Digest(preview.Authorization))
	s.Close()
	if err != nil || len(exact) != preview.PlanBytes {
		t.Fatalf("stored plan bytes/size: %d %v", len(exact), err)
	}
	summary, err := browser.ConsentSummary(exact)
	if err != nil || strings.TrimSuffix(string(summary), "\n") != preview.Consent {
		t.Fatalf("CLI did not share the strict consent summary: %q %v", preview.Consent, err)
	}
	if !strings.Contains(output, "after run --approve "+preview.Authorization) {
		t.Fatalf("non-TTY preview lacks the exact stored-plan command: %s", output)
	}

	code, readable, stderr := invoke([]string{"inspect", preview.Authorization, "--project", project}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("inspect plan: %d %q", code, stderr)
	}
	if summaryAt, planAt := strings.Index(readable, "Consent summary"), strings.Index(readable, "Plan\n"); summaryAt < 0 || planAt < summaryAt || !strings.Contains(readable, formatPlanSize(len(exact))) || !strings.Contains(readable, "after run --approve "+preview.Authorization) {
		t.Fatalf("inspect plan omitted ordered summary, exact plan, size or Next: %s", readable)
	}
	code, machine, stderr := invoke([]string{"inspect", preview.Authorization, "--project", project, "--json"}, false, "")
	var inspection struct {
		Data struct {
			Base64 string `json:"base64"`
		} `json:"data"`
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(func() string {
		if json.Unmarshal([]byte(machine), &inspection) != nil {
			return ""
		}
		return inspection.Data.Base64
	}())
	if code != ExitOK || stderr != "" || decodeErr != nil || !bytes.Equal(decoded, exact) {
		t.Fatalf("JSON plan inspection did not preserve exact bytes: %d %q %q %v", code, machine, stderr, decodeErr)
	}

	code, _, stderr = invoke([]string{"run", "--approve", "abcd", "--project", project}, false, "")
	if code != ExitInvalid || !strings.Contains(stderr, "full digest") {
		t.Fatalf("approval prefix was not refused: %d %q", code, stderr)
	}
	missing := "sha256:" + strings.Repeat("f", 64)
	code, stdout, stderr := invoke([]string{"run", "--approve", missing, "--project", project}, false, "")
	if code != ExitInvalid || stdout != "" || !strings.Contains(stderr, "no stored execution plan") {
		t.Fatalf("unknown authorization digest was not refused: %d %q %q", code, stdout, stderr)
	}
	code, stdout, stderr = invoke([]string{"run", "--approve", preview.Authorization, "--project", project}, false, "")
	if code != ExitOperational || stdout != "" || !strings.Contains(stderr, "docker_binary") || !strings.Contains(stderr, "docker_host:") || !strings.Contains(stderr, "not contacted") {
		t.Fatalf("exact stored approval did not stop at passive setup guidance: %d %q %q", code, stdout, stderr)
	}
	s, err = store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	receipts, err := s.List("receipt")
	s.Close()
	if err != nil || len(receipts) != 0 {
		t.Fatalf("refusal contacted execution path or persisted a receipt: %d %v", len(receipts), err)
	}
}

func TestInvalidStoredPlanSummaryStillPrintsSanitizedBytes(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("{\"untrusted\":\"\x1b]52;c;clipboard\"}\n")
	id := evidence.Digest("sha256:" + fmt.Sprintf("%x", sha256Bytes(raw)))
	if err := s.PutPlan(id, raw); err != nil {
		t.Fatal(err)
	}
	s.Close()
	code, output, stderr := invoke([]string{"inspect", string(id), "--project", project}, false, "")
	if code != ExitOK || stderr != "" || strings.ContainsAny(output, "\x1b\a") || !strings.Contains(output, "Consent summary unavailable") || !strings.Contains(output, "Plan\n") {
		t.Fatalf("malformed plan did not retain safe inspection: %d %q %q", code, output, stderr)
	}
}

func TestDockerSetupIsPassiveAndConfigReportsSameProblems(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "docker-called")
	docker := filepath.Join(bin, "docker")
	script := "#!/bin/sh\nprintf called > " + shellQuote(marker) + "\nexit 99\n"
	if err := os.WriteFile(docker, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	t.Setenv("PATH", bin)
	t.Setenv("AFTER_DOCKER_BINARY", "")
	t.Setenv("AFTER_DOCKER_HOST", "")

	status := dockerSetupStatus(config.Config{})
	if len(status.Problems) == 0 || len(status.Suggestions) != 2 || !strings.Contains(strings.Join(status.Suggestions, "\n"), docker) || !strings.Contains(strings.Join(status.Suggestions, "\n"), "docker_host:") {
		t.Fatalf("setup probe did not suggest passive CLI/socket candidates: %+v", status)
	}
	code, output, stderr := invoke([]string{"config", "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("config: %d %q", code, stderr)
	}
	var configResult struct {
		Data struct {
			Settings []struct {
				Name   string `json:"name"`
				Value  any    `json:"value"`
				Source string `json:"source"`
			} `json:"settings"`
			Setup dockerSetup `json:"docker_setup"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &configResult); err != nil || strings.Join(configResult.Data.Setup.Problems, "\n") != strings.Join(status.Problems, "\n") || !strings.Contains(output, "not selected") && !strings.Contains(output, "found on PATH") {
		t.Fatalf("config did not show shared passive diagnostics: %s %v", output, err)
	}
	dockerSettings := map[string]bool{}
	for _, setting := range configResult.Data.Settings {
		if setting.Name == "docker_binary" || setting.Name == "docker_host" {
			dockerSettings[setting.Name] = true
			if setting.Value != "not configured (value hidden)" || setting.Source != "default" {
				t.Fatalf("passive setup selected a Docker setting: %+v", setting)
			}
		}
	}
	if len(dockerSettings) != 2 {
		t.Fatalf("config omitted explicit unselected Docker settings: %+v", configResult.Data.Settings)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("setup checks executed the Docker CLI: %v", err)
	}
}

func TestDockerConfigErrorsGiveConfigurationLines(t *testing.T) {
	t.Setenv("AFTER_DOCKER_BINARY", "relative/docker")
	t.Setenv("AFTER_DOCKER_HOST", "tcp://not-local")
	code, stdout, stderr := invoke([]string{"config"}, false, "")
	if code != ExitInvalid || !strings.Contains(stdout, "Docker setup") || !strings.Contains(stdout, "docker_binary") || !strings.Contains(stderr, "docker_binary") || !strings.Contains(stderr, "docker_binary:") || !strings.Contains(stderr, "docker_host:") || !strings.Contains(stderr, "AFTER_DOCKER_BINARY") || !strings.Contains(stderr, "AFTER_DOCKER_HOST") {
		t.Fatalf("invalid Docker settings lack actionable setup guidance: %d %q %q", code, stdout, stderr)
	}
}

func TestRunConsentPTYAndPipeRefusalsAt80Columns(t *testing.T) {
	project := filepath.Join(t.TempDir(), "payment")
	base, candidate := fixtureCommits(t, project)
	home := t.TempDir()
	code, _, stderr, err := runCLIPipe(project, home, false, []string{"capture", "--base", base, "--target", candidate, "--json"})
	if err != nil || code != ExitOK || stderr != "" {
		t.Fatalf("capture setup: %d %q %v", code, stderr, err)
	}
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "docker-called")
	docker := filepath.Join(bin, "docker")
	if err := os.WriteFile(docker, []byte("#!/bin/sh\nprintf called > "+shellQuote(marker)+"\nexit 99\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	for _, noColor := range []bool{false, true} {
		code, transcript, err := runCLIPTYAnswer(project, home, noColor, []string{"run"}, "Type yes to run exactly these plan bytes:", "no\n")
		if err != nil || code != ExitDenied || !strings.Contains(transcript, "Consent summary") || !strings.Contains(transcript, "Nothing has run") || !strings.Contains(transcript, "Using the newest capture") || strings.ContainsAny(stripThemeSGR(transcript), "\x1b\a") {
			t.Fatalf("80-column PTY refusal NO_COLOR=%t: exit=%d output=%q err=%v", noColor, code, transcript, err)
		}
		if noColor && strings.ContainsAny(transcript, "\x1b") || !noColor && !themeSGROnly(transcript) {
			t.Fatalf("80-column PTY style contract NO_COLOR=%t: %q", noColor, transcript)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("answer other than yes contacted the Docker CLI: %v", err)
		}
		code, preview, stderr, err := runCLIPipe(project, home, noColor, []string{"run"})
		if err != nil || code != ExitDenied || stderr != "" || !strings.Contains(preview, "after run --approve sha256:") || !strings.Contains(preview, "Consent summary") {
			t.Fatalf("pipe preview NO_COLOR=%t: exit=%d output=%q stderr=%q err=%v", noColor, code, preview, stderr, err)
		}
		if strings.ContainsAny(preview+stderr, "\x1b\a") {
			t.Fatalf("pipe included terminal styling: %q", preview+stderr)
		}
		var parsed struct {
			Data executionPreview `json:"data"`
		}
		code, machine, stderr, err := runCLIPipe(project, home, noColor, []string{"run", "--json"})
		if err != nil || code != ExitDenied || stderr != "" || json.Unmarshal([]byte(machine), &parsed) != nil {
			t.Fatalf("JSON preview NO_COLOR=%t: exit=%d output=%q stderr=%q err=%v", noColor, code, machine, stderr, err)
		}
		code, approved, err := runCLIPTYAnswer(project, home, noColor, []string{"run", "--approve", parsed.Data.Authorization}, "", "")
		if err != nil || code != ExitOperational || !strings.Contains(approved, "Docker execution is not configured") || !strings.Contains(approved, "docker_host:") || !strings.Contains(approved, "not contacted") {
			t.Fatalf("exact PTY approval without setup NO_COLOR=%t: exit=%d output=%q err=%v", noColor, code, approved, err)
		}
		code, approved, stderr, err = runCLIPipe(project, home, noColor, []string{"run", "--approve", parsed.Data.Authorization})
		if err != nil || code != ExitOperational || !strings.Contains(stderr, "Docker execution is not configured") || !strings.Contains(stderr, "docker_binary:") || !strings.Contains(stderr, "not contacted") {
			t.Fatalf("exact pipe approval without setup NO_COLOR=%t: exit=%d stdout=%q stderr=%q err=%v", noColor, code, approved, stderr, err)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("setup diagnostics contacted the Docker CLI: %v", err)
		}
	}
}

func TestConsentPromptShowsOnlySummaryAndKeepsExactDigestOnDecodeFailure(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	var output bytes.Buffer
	accepted := prompt(&invocation{stderr: &output, reader: strings.NewReader("no\n"), columns: 80}, digest, []byte("Snapshots: base → candidate\nRuns: 4 runs\n"), nil, 2048, nil)
	if accepted || !strings.Contains(output.String(), digest) || !strings.Contains(output.String(), "2.0 KiB") || !strings.Contains(output.String(), "Snapshots: base") || strings.Contains(output.String(), `{"version"`) {
		t.Fatalf("refusal prompt did not show a bounded summary of exact bytes: accepted=%t %q", accepted, output.String())
	}
	output.Reset()
	accepted = prompt(&invocation{stderr: &output, reader: strings.NewReader("yes\n"), columns: 80}, digest, nil, fmt.Errorf("strict decoder rejected preview"), 12, nil)
	if !accepted || !strings.Contains(output.String(), "Consent summary unavailable") || !strings.Contains(output.String(), digest) || !strings.Contains(output.String(), "Type yes to run exactly these plan bytes") {
		t.Fatalf("summary failure changed exact-byte consent: accepted=%t %q", accepted, output.String())
	}
}

func sha256Bytes(raw []byte) [32]byte {
	return sha256.Sum256(raw)
}

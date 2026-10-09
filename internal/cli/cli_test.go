package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

func invoke(args []string, tty bool, input string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, &stdout, &stderr, strings.NewReader(input), tty)
	return code, stdout.String(), stderr.String()
}

func TestTTYDetectionRejectsNonTerminalCharacterDevices(t *testing.T) {
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if terminalInput(file) || terminalInput(strings.NewReader("")) {
		t.Fatal("non-terminal input was accepted for an interactive prompt")
	}
}

func TestUrfaveHelpVersionAndInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		code int
		text string
	}{
		{"empty", nil, ExitOK, "capture"},
		{"help-command", []string{"help"}, ExitOK, "Everyday"},
		{"help-flag", []string{"--help"}, ExitOK, "Everyday"},
		{"short-help", []string{"-h"}, ExitOK, "import"},
		{"version-command", []string{"version"}, ExitOK, "after " + Version},
		{"version-flag", []string{"--version"}, ExitOK, Version},
		{"unknown-command", []string{"untrusted\x1b]52;c;bad\a"}, ExitInvalid, "unknown command"},
		{"extra-command-argument", []string{"capture", "unexpected"}, ExitInvalid, "unexpected arguments"},
		{"bad-flag", []string{"config", "--unknown-option"}, ExitInvalid, "unknown flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := invoke(tc.args, false, "")
			if code != tc.code || !strings.Contains(stdout+stderr, tc.text) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if tc.code == ExitOK && stderr != "" {
				t.Fatalf("unexpected diagnostics: %q", stderr)
			}
			if tc.code != ExitOK && stdout != "" {
				t.Fatalf("invalid command wrote stdout: %q", stdout)
			}
			if strings.ContainsAny(stderr, "\x1b\a") {
				t.Fatalf("untrusted argument reached terminal: %q", stderr)
			}
		})
	}
}

func TestSnapshotInspectionShowsRecordedCaptureTimeAndKeepsJSON(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	t.Chdir(project)
	type captureEnvelope struct {
		Data struct {
			Base struct {
				ID evidence.Digest `json:"id"`
			} `json:"base_snapshot"`
			Candidate struct {
				ID evidence.Digest `json:"id"`
			} `json:"candidate_snapshot"`
		} `json:"data"`
	}
	captureOnce := func() captureEnvelope {
		t.Helper()
		code, output, stderr := invoke([]string{"capture", "--json"}, false, "")
		if code != ExitOK || stderr != "" {
			t.Fatalf("capture: exit=%d stderr=%q", code, stderr)
		}
		var result captureEnvelope
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	first, second := captureOnce(), captureOnce()
	if first.Data.Base.ID != second.Data.Base.ID || first.Data.Candidate.ID != second.Data.Candidate.ID {
		t.Fatal("unchanged capture changed snapshot IDs")
	}
	code, readable, stderr := invoke([]string{"inspect", string(first.Data.Candidate.ID)}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(readable, "Captured") || !strings.Contains(readable, "working_tree") || !strings.Contains(readable, "record ") {
		t.Fatalf("readable snapshot inspection did not show capture history: exit=%d stdout=%q stderr=%q", code, readable, stderr)
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	history, err := store.CapturesForSnapshot(s, first.Data.Candidate.ID)
	if err != nil || len(history.Records) != 2 {
		t.Fatalf("capture history: %+v %v", history, err)
	}
	snapshot, err := store.Get[evidence.Snapshot](s, first.Data.Candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	expected := append([]byte(`{"schema_version":1,"kind":"snapshot","data":`), data...)
	expected = append(expected, '}', '\n')
	code, output, stderr := invoke([]string{"inspect", string(first.Data.Candidate.ID), "--json"}, false, "")
	if code != ExitOK || stderr != "" || !bytes.Equal([]byte(output), expected) {
		t.Fatalf("snapshot JSON changed: exit=%d stderr=%q\n got %s\nwant %s", code, stderr, output, expected)
	}
}

func TestConfigCLIUsesExplicitFalseAndRedactsRuntimeValues(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "empty config.yaml")
	if err := os.WriteFile(configFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(dir, "project with spaces")
	t.Setenv("AFTER_DOCKER_BINARY", "/private/DO_NOT_PRINT/docker")
	t.Setenv("AFTER_DOCKER_HOST", "unix:///private/DO_NOT_PRINT.sock")
	t.Setenv("AFTER_INTERACTIVE", "false")
	t.Setenv("AFTER_RAW_DIFF", "false")
	t.Setenv("AFTER_DIFF_BYTES", "0")
	code, stdout, stderr := invoke([]string{"config", "--config", configFile, "--project", project, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var result struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Data          struct {
			Settings []struct {
				Name   string `json:"name"`
				Value  any    `json:"value"`
				Source string `json:"source"`
			} `json:"settings"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 1 || result.Kind != "configuration" {
		t.Fatalf("unexpected envelope: %+v", result)
	}
	settings := map[string]struct {
		value  any
		source string
	}{}
	for _, item := range result.Data.Settings {
		settings[item.Name] = struct {
			value  any
			source string
		}{item.Value, item.Source}
	}
	if settings["interactive"].value != false || settings["interactive"].source != "env" {
		t.Fatalf("explicit false lost: %+v", settings["interactive"])
	}
	if settings["raw_diff"].value != false || settings["raw_diff"].source != "env" || settings["diff_bytes"].value != float64(0) || settings["diff_bytes"].source != "env" {
		t.Fatalf("false/zero CLI settings lost: %+v %+v", settings["raw_diff"], settings["diff_bytes"])
	}
	if settings["project"].value != "selected path hidden" || settings["project"].source != "flag" || strings.Contains(stdout, project) {
		t.Fatalf("project path or provenance was exposed: %+v", settings["project"])
	}
	if !strings.Contains(stdout, "configured (value hidden)") || strings.Contains(stdout, "DO_NOT_PRINT") {
		t.Fatalf("runtime setting leaked: %s", stdout)
	}
}

func TestCaptureImportInspectExportAndSafeFailures(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project with spaces")
	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	baseID, _ := makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() {}\n// changed\n")
	gitRun(t, project, "add", "app/main.go")
	gitRun(t, project, "commit", "-qm", "candidate")
	candidateCommit := strings.TrimSpace(gitRun(t, project, "rev-parse", "HEAD"))
	code, captured, stderr := invoke([]string{"capture", "--project", project, "--config", configFile, "--base", baseID, "--target", candidateCommit, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("capture exit=%d stderr=%q", code, stderr)
	}
	var captureEnvelope struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Data          struct {
			Base struct {
				ID string `json:"id"`
			} `json:"base_snapshot"`
			Candidate struct {
				ID string `json:"id"`
			} `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(captured), &captureEnvelope); err != nil {
		t.Fatal(err)
	}
	base, candidate := captureEnvelope.Data.Base.ID, captureEnvelope.Data.Candidate.ID
	if captureEnvelope.SchemaVersion != 1 || captureEnvelope.Kind != "capture" || !validDigest(base) || !validDigest(candidate) {
		t.Fatalf("bad capture result: %s", captured)
	}
	args := []string{"inspect", base, candidate, "--project", project, "--config", configFile, "--json"}
	code, inspected, stderr := invoke(args, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("inspect exit=%d stderr=%q", code, stderr)
	}
	var snapshotView struct {
		Kind string `json:"kind"`
		Data struct {
			Inventory []struct {
				Path   string `json:"path"`
				Change string `json:"change"`
			} `json:"inventory"`
			Diff struct {
				Base64 string `json:"base64"`
				More   bool   `json:"more"`
			} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(inspected), &snapshotView); err != nil {
		t.Fatal(err)
	}
	if snapshotView.Kind != "snapshot" || len(snapshotView.Data.Inventory) != 1 || snapshotView.Data.Inventory[0].Path != "app/main.go" || snapshotView.Data.Inventory[0].Change != "modified" {
		t.Fatalf("missing diff inventory: %s", inspected)
	}
	diff, err := base64.StdEncoding.DecodeString(snapshotView.Data.Diff.Base64)
	if err != nil || !bytes.Contains(diff, []byte("changed")) {
		t.Fatalf("raw diff unavailable: %v %q", err, diff)
	}
	code, exported, stderr := invoke([]string{"export", base, candidate, "--project", project, "--config", configFile}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(exported, `"kind":"export"`) {
		t.Fatalf("export exit=%d stderr=%q output=%q", code, stderr, exported)
	}

	reportPath := filepath.Join(dir, "report with spaces.jsonl")
	report := strings.Join([]string{
		`{"Action":"run","Package":"example.test","Test":"TestPass"}`,
		`{"Action":"pass","Package":"example.test","Test":"TestPass"}`,
		`{"Action":"run","Package":"example.test","Test":"TestFail"}`,
		`{"Action":"fail","Package":"example.test","Test":"TestFail"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(reportPath, []byte(report), 0600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := invoke([]string{"import", reportPath, "--project", project, "--config", configFile}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(stdout, "Producer     not stated") || !strings.Contains(stdout, "--producer TEXT") {
		t.Fatalf("import without a producer claim failed or omitted its hint: %d %q %q", code, stdout, stderr)
	}
	code, stdout, stderr = invoke([]string{"import", reportPath, "--producer", "go1.27.1", "--captured-at", "not-a-time"}, false, "")
	if code != ExitInvalid || stdout != "" || !strings.Contains(stderr, "RFC3339") {
		t.Fatalf("invalid capture time was accepted: %d %q %q", code, stdout, stderr)
	}
	code, imported, stderr := invoke([]string{"import", reportPath, "--project", project, "--config", configFile, "--producer", "go1.27.1", "--captured-at", "2026-01-02T03:04:05Z", "--snapshot", candidate, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("import exit=%d stderr=%q", code, stderr)
	}
	var reportEnvelope struct {
		Data struct {
			ID       string `json:"id"`
			Metadata struct {
				Producer   string `json:"producer"`
				Snapshot   string `json:"snapshot"`
				CapturedAt string `json:"captured_at"`
			} `json:"metadata"`
			Cards []struct {
				State evidence.EvidenceState `json:"state"`
			} `json:"cards"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(imported), &reportEnvelope); err != nil {
		t.Fatal(err)
	}
	if !validDigest(reportEnvelope.Data.ID) || len(reportEnvelope.Data.Cards) != 2 || reportEnvelope.Data.Metadata.Producer != "go1.27.1" || reportEnvelope.Data.Metadata.Snapshot != candidate || reportEnvelope.Data.Metadata.CapturedAt != "2026-01-02T03:04:05Z" || strings.Contains(imported, reportPath) {
		t.Fatalf("bad imported report: %s", imported)
	}
	if reportEnvelope.Data.Cards[0].State.Producer != evidence.Importer || reportEnvelope.Data.Cards[0].State.Kind != evidence.Reported || reportEnvelope.Data.Cards[0].State.Execution != evidence.NotRun || reportEnvelope.Data.Cards[0].State.Report != evidence.ReportPass || reportEnvelope.Data.Cards[1].State.Report != evidence.ReportFail {
		t.Fatalf("reported outcomes were mislabeled: %+v", reportEnvelope.Data.Cards)
	}
	code, inspectedReport, stderr := invoke([]string{"inspect", reportEnvelope.Data.ID, "--project", project, "--config", configFile, "--json"}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(inspectedReport, `"report":"fail"`) {
		t.Fatalf("report inspection: code=%d stderr=%q output=%q", code, stderr, inspectedReport)
	}

	code, stdout, stderr = invoke([]string{"import", filepath.Join(dir, "missing report.jsonl"), "--project", project, "--config", configFile, "--producer", "go1.27.1"}, false, "")
	if code != ExitOperational || stdout != "" || strings.Contains(stderr, "missing report") {
		t.Fatalf("unsafe unreadable-file failure: %d %q %q", code, stdout, stderr)
	}
	link := filepath.Join(dir, "report symlink.jsonl")
	if err := os.Symlink(reportPath, link); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = invoke([]string{"import", link, "--project", project, "--config", configFile, "--producer", "go1.27.1"}, false, "")
	if code != ExitOperational || stdout != "" || strings.Contains(stderr, link) {
		t.Fatalf("symlink input was followed or leaked: %d %q %q", code, stdout, stderr)
	}
	code, stdout, stderr = invoke([]string{"inspect", "not-a-digest", "--project", project, "--config", configFile}, false, "")
	if code != ExitInvalid || stdout != "" || !strings.Contains(stderr, "invalid input") {
		t.Fatalf("bad ID status: %d %q %q", code, stdout, stderr)
	}
	// A well-formed ID that is not a stored receipt is invalid input, not an
	// operational persistence failure.
	for _, id := range []string{candidate, "sha256:" + strings.Repeat("0", 64)} {
		code, stdout, stderr = invoke([]string{"compare", id, "--project", project, "--config", configFile}, false, "")
		if code != ExitInvalid || stdout != "" || !strings.Contains(stderr, "no receipt matches") || !strings.Contains(stderr, "after run BASE CANDIDATE") {
			t.Fatalf("unknown receipt status: %d %q %q", code, stdout, stderr)
		}
	}
	hostile := "\x1b]52;c;clipboard\a"
	code, stdout, stderr = invoke([]string{"inspect", hostile, "--project", project, "--config", configFile}, false, "")
	if code != ExitInvalid || stdout != "" || strings.ContainsAny(stderr, "\x1b\a") {
		t.Fatalf("terminal injection: %d %q %q", code, stdout, stderr)
	}
	maliciousConfig := filepath.Join(dir, "hostile config.yaml")
	if err := os.WriteFile(maliciousConfig, []byte(hostile+": true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = invoke([]string{"config", "--config", maliciousConfig}, false, "")
	if code != ExitInvalid || stdout != "" || strings.ContainsAny(stderr, "\x1b\a") {
		t.Fatalf("configuration key reached terminal: %d %q %q", code, stdout, stderr)
	}
}

func TestBoundedJSONAndPages(t *testing.T) {
	var stdout, stderr bytes.Buffer
	state := &invocation{stdout: &stdout, stderr: &stderr}
	if err := writeJSON(state, "large", strings.Repeat("x", MaxCLIOutput)); err == nil || stdout.Len() != 0 {
		t.Fatal("oversized output was written")
	}
	code, output, diagnostic := invoke([]string{"inspect", strings.Repeat("a", 71), "--diff-size", "65537"}, false, "")
	if code != ExitInvalid || output != "" || diagnostic == "" {
		t.Fatalf("unbounded page accepted: %d %q %q", code, output, diagnostic)
	}
}

func TestRunPreviewReconstructionDoesNotExecuteWithoutExactApproval(t *testing.T) {
	// The POC gate authorizes live proofs by exporting these settings; this unit
	// test specifically proves approval cannot execute without local Docker setup.
	t.Setenv("AFTER_DOCKER_BINARY", "")
	t.Setenv("AFTER_DOCKER_HOST", "")
	dir := t.TempDir()
	project := filepath.Join(dir, "payment fixture with spaces")
	configFile := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(configFile, []byte("repetitions: 2\nrun_seconds: 180\noutput_bytes: 65536\ninteractive: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	baseCommit, candidateCommit := fixtureCommits(t, project)
	code, captured, stderr := invoke([]string{"capture", "--project", project, "--config", configFile, "--base", baseCommit, "--target", candidateCommit, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("capture: %d %q", code, stderr)
	}
	var captureResult struct {
		Data struct {
			Base struct {
				ID string `json:"id"`
			} `json:"base_snapshot"`
			Candidate struct {
				ID string `json:"id"`
			} `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(captured), &captureResult); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(dir, "saved preview.json")
	code, output, stderr := invoke([]string{"run", captureResult.Data.Base.ID, captureResult.Data.Candidate.ID, "--project", project, "--config", configFile, "--plan-out", planPath, "--json"}, false, "")
	if code != ExitDenied || stderr != "" {
		t.Fatalf("preview should require explicit approval: %d %q", code, stderr)
	}
	var previewResult struct {
		Data struct {
			Authorization string `json:"authorization_digest"`
			Status        string `json:"status"`
			Plan          struct {
				Repetitions int `json:"repetitions"`
			} `json:"plan"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &previewResult); err != nil {
		t.Fatal(err)
	}
	if previewResult.Data.Status != "authorization_required" || previewResult.Data.Plan.Repetitions != 2 || !validDigest(previewResult.Data.Authorization) {
		t.Fatalf("default flag presence or plan digest wrong: %s", output)
	}
	info, err := os.Stat(planPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("saved plan is not private: %v %v", info, err)
	}
	wrong := "sha256:" + strings.Repeat("0", 64)
	code, output, stderr = invoke([]string{"run", "--plan-file", planPath, "--approve", wrong, "--project", project, "--config", configFile, "--json"}, false, "")
	if code != ExitDenied || stderr != "" || !strings.Contains(output, "authorization_mismatch") {
		t.Fatalf("wrong digest was not refused: %d %q %q", code, output, stderr)
	}
	content, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	code, output, stderr = invoke([]string{"run", captureResult.Data.Base.ID, captureResult.Data.Candidate.ID, "--project", project, "--config", configFile, "--plan-out", planPath, "--json"}, false, "")
	if code != ExitOperational || output != "" || !strings.Contains(stderr, "without overwriting") {
		t.Fatalf("--plan-out overwrote an existing preview: %d %q %q", code, output, stderr)
	}
	if saved, err := os.ReadFile(planPath); err != nil || !bytes.Equal(saved, content) {
		t.Fatalf("existing --plan-out bytes changed: %v", err)
	}
	code, output, stderr = invoke([]string{"run", "--plan-file", planPath, "--approve", previewResult.Data.Authorization, "--project", project, "--config", configFile, "--json"}, false, "")
	if code != ExitOperational || output != "" || !strings.Contains(stderr, "docker_binary") || !strings.Contains(stderr, "docker_host:") {
		t.Fatalf("exact explicit-file approval did not preserve the plan-file flow: %d %q %q", code, output, stderr)
	}
	if _, err := os.Stat(filepath.Join(project, ".after")); err != nil {
		t.Fatal("preview unexpectedly removed or replaced store")
	}
	badPlan := filepath.Join(dir, "edited preview.json")
	content = bytes.Replace(content, []byte(`"build_argv"`), []byte(`"repository_command"`), 1)
	if err := os.WriteFile(badPlan, content, 0600); err != nil {
		t.Fatal(err)
	}
	code, output, stderr = invoke([]string{"run", "--plan-file", badPlan, "--project", project, "--config", configFile}, false, "")
	if code != ExitInvalid || output != "" || stderr == "" {
		t.Fatalf("modified plan was accepted: %d %q %q", code, output, stderr)
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	receipts, err := s.List("receipt")
	s.Close()
	if err != nil || len(receipts) != 0 {
		t.Fatalf("refused approvals persisted a run receipt: %d %v", len(receipts), err)
	}
}

func makeProject(t *testing.T, root string) (string, string) {
	t.Helper()
	gitRun(t, root, "init", "-q", "--template=", "-b", "main")
	writeProjectFile(t, root, "go.mod", "module fixture.example/payment\n\ngo 1.27.1\n")
	writeProjectFile(t, root, "app/main.go", "package main\nfunc main() {}\n")
	gitRun(t, root, "add", "go.mod", "app/main.go")
	gitRun(t, root, "commit", "-qm", "base")
	base := strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD"))
	return base, ""
}

func fixtureCommits(t *testing.T, root string) (string, string) {
	t.Helper()
	gitRun(t, root, "init", "-q", "--template=", "-b", "main")
	for _, name := range []string{"go.mod", "app/main.go", "app/config.go", "driver/main.go"} {
		data, err := os.ReadFile(filepath.Join("..", "paymentfixture", "testdata", "payment", name))
		if err != nil {
			t.Fatal(err)
		}
		writeProjectFile(t, root, name, string(data))
	}
	gitRun(t, root, "add", "go.mod", "app", "driver")
	gitRun(t, root, "commit", "-qm", "synthetic base")
	base := strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD"))
	configPath := filepath.Join(root, "app/config.go")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "24 * 60 * 60") != 1 {
		t.Fatal("payment fixture mutation anchor changed")
	}
	writeProjectFile(t, root, "app/config.go", strings.Replace(string(data), "24 * 60 * 60", "5 * 60", 1))
	gitRun(t, root, "add", "app/config.go")
	gitRun(t, root, "commit", "-qm", "synthetic candidate")
	candidate := strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD"))
	return base, candidate
}

func writeProjectFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=AFTER fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=AFTER fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git command failed: %v", err)
	}
	return string(output)
}

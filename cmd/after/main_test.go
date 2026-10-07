package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
)

const (
	entryEnv = "AFTER_TEST_ENTRY"
	argsEnv  = "AFTER_TEST_ARGS"
)

// TestNativeCLI also acts as the child process entry point. It invokes the
// compiled package main in an isolated environment, preserving real argv and
// distinct stdout/stderr streams without invoking a shell.
func TestNativeCLI(t *testing.T) {
	if os.Getenv(entryEnv) == "1" {
		encoded, err := base64.RawURLEncoding.DecodeString(os.Getenv(argsEnv))
		if err != nil {
			os.Exit(2)
		}
		var args []string
		if err := json.Unmarshal(encoded, &args); err != nil {
			os.Exit(2)
		}
		os.Args = append([]string{"after"}, args...)
		main()
		return
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cwd := filepath.Join(root, "empty working directory")
	home := filepath.Join(root, "isolated home")
	if err := os.MkdirAll(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, text string
		args       []string
		code       int
	}{
		{"help", "Everyday", []string{"--help"}, 0},
		{"version", "0.1.0-dev", []string{"--version"}, 0},
		{"version-command", "after 0.1.0-dev", []string{"version"}, 0},
		{"invalid-arguments", "unexpected arguments", []string{"capture", "unexpected"}, 2},
		{"unknown-command", "unknown command", []string{"untrusted\x1b]52;c;clipboard\a"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := native(exe, cwd, home, tc.args, nil)
			if code != tc.code || !strings.Contains(stdout+stderr, tc.text) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if strings.ContainsAny(stderr, "\x1b\a") {
				t.Fatalf("terminal controls reached stderr: %q", stderr)
			}
			if tc.code == 0 && stderr != "" {
				t.Fatalf("unexpected diagnostics: %q", stderr)
			}
			if tc.code != 0 && stdout != "" {
				t.Fatalf("invalid command wrote stdout: %q", stdout)
			}
		})
	}
	entries, err := os.ReadDir(cwd)
	if err != nil || len(entries) != 0 {
		t.Fatalf("help/version/invalid arguments touched project state: %v %v", entries, err)
	}
}

func TestCLIWorkflowSubprocess(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	project := filepath.Join(root, "project with spaces")
	home := filepath.Join(root, "isolated home")
	configPath := filepath.Join(root, "settings.yaml")
	reportPath := filepath.Join(root, "go report with spaces.jsonl")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("interactive: false\nraw_diff: true\ndiff_bytes: 32768\n"), 0600); err != nil {
		t.Fatal(err)
	}
	baseCommit, candidateCommit := simpleCommits(t, project)
	captureCode, captureOut, captureErr := native(exe, root, home, []string{
		"capture", "--project", project, "--config", configPath, "--base", baseCommit, "--target", candidateCommit,
	}, nil)
	if captureCode != 0 || captureErr != "" {
		t.Fatalf("capture exit=%d stderr=%q stdout=%q", captureCode, captureErr, captureOut)
	}
	var captured struct {
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
	if err := json.Unmarshal([]byte(captureOut), &captured); err != nil || captured.SchemaVersion != 1 || captured.Kind != "capture" || captured.Data.Base.ID == "" || captured.Data.Candidate.ID == "" {
		t.Fatalf("invalid capture response: %q %v", captureOut, err)
	}
	inspectCode, inspectOut, inspectErr := native(exe, root, home, []string{
		"inspect", captured.Data.Base.ID, captured.Data.Candidate.ID, "--project", project, "--config", configPath,
	}, nil)
	if inspectCode != 0 || inspectErr != "" || !strings.Contains(inspectOut, `"path":"app/main.go"`) || !strings.Contains(inspectOut, `"base64":`) {
		t.Fatalf("raw diff inspection failed: %d %q %q", inspectCode, inspectErr, inspectOut)
	}

	report := strings.Join([]string{
		`{"Action":"run","Package":"example.invalid/cli","Test":"TestPass"}`,
		`{"Action":"pass","Package":"example.invalid/cli","Test":"TestPass"}`,
		`{"Action":"run","Package":"example.invalid/cli","Test":"TestFail"}`,
		`{"Action":"fail","Package":"example.invalid/cli","Test":"TestFail"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(reportPath, []byte(report), 0600); err != nil {
		t.Fatal(err)
	}
	importCode, importOut, importErr := native(exe, root, home, []string{
		"import", reportPath, "--project", project, "--config", configPath, "--producer", "synthetic-go-test-v1", "--snapshot", captured.Data.Candidate.ID,
	}, nil)
	if importCode != 0 || importErr != "" {
		t.Fatalf("import exit=%d stderr=%q stdout=%q", importCode, importErr, importOut)
	}
	var imported struct {
		Data struct {
			ID    string `json:"id"`
			Cards []struct {
				State evidence.EvidenceState `json:"state"`
			} `json:"cards"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(importOut), &imported); err != nil || len(imported.Data.Cards) != 2 || imported.Data.ID == "" {
		t.Fatalf("invalid report response: %q %v", importOut, err)
	}
	for i, want := range []evidence.ReportOutcome{evidence.ReportPass, evidence.ReportFail} {
		state := imported.Data.Cards[i].State
		if state.Kind != evidence.Reported || state.Producer != evidence.Importer || state.Applicability != evidence.Unknown || state.Execution != evidence.NotRun || state.Comparison != evidence.NotCompared || state.Report != want {
			t.Fatalf("import promoted reported result: %+v", state)
		}
	}
	inspectReportCode, inspectReportOut, inspectReportErr := native(exe, root, home, []string{
		"inspect", imported.Data.ID, "--project", project, "--config", configPath,
	}, nil)
	if inspectReportCode != 0 || inspectReportErr != "" || !strings.Contains(inspectReportOut, `"report":"pass"`) || !strings.Contains(inspectReportOut, `"report":"fail"`) || !strings.Contains(inspectReportOut, `"kind":"reported"`) {
		t.Fatalf("report provenance was not visible: %d %q %q", inspectReportCode, inspectReportErr, inspectReportOut)
	}
}

func TestPaymentCLIProof(t *testing.T) {
	if os.Getenv("AFTER_CLI_PROOF") != "1" {
		t.Skip("task cli:proof authorizes the real synthetic payment comparison")
	}
	dockerBinary, dockerHost := os.Getenv("AFTER_DOCKER_BINARY"), os.Getenv("AFTER_DOCKER_HOST")
	if !filepath.IsAbs(dockerBinary) || !strings.HasPrefix(dockerHost, "unix:///") {
		t.Fatal("cli:proof requires explicit absolute AFTER_DOCKER_BINARY and local Unix AFTER_DOCKER_HOST; see docs/SANDBOX.md")
	}
	if _, err := os.Stat(dockerBinary); err != nil {
		t.Fatalf("configured Docker CLI is unavailable: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	project := filepath.Join(root, "payment fixture with spaces")
	home := filepath.Join(root, "isolated home")
	configPath := filepath.Join(root, "settings.yaml")
	planPath := filepath.Join(root, "approved preview.json")
	reportPath := filepath.Join(root, "reported test output.jsonl")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("repetitions: 2\nrun_seconds: 180\noutput_bytes: 65536\n"), 0600); err != nil {
		t.Fatal(err)
	}
	baseCommit, candidateCommit := paymentCommits(t, project)
	dockerEnv := []string{"AFTER_DOCKER_BINARY=" + dockerBinary, "AFTER_DOCKER_HOST=" + dockerHost}
	captureCode, captureOut, captureErr := native(exe, root, home, []string{
		"capture", "--project", project, "--config", configPath, "--base", baseCommit, "--target", candidateCommit,
	}, nil)
	if captureCode != 0 || captureErr != "" {
		t.Fatalf("payment capture exit=%d stderr=%q stdout=%q", captureCode, captureErr, captureOut)
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
	if err := json.Unmarshal([]byte(captureOut), &captured); err != nil || captured.Data.Base.ID == "" || captured.Data.Candidate.ID == "" {
		t.Fatalf("invalid payment capture response: %q %v", captureOut, err)
	}
	previewCode, previewOut, previewErr := native(exe, root, home, []string{
		"run", captured.Data.Base.ID, captured.Data.Candidate.ID, "--project", project,
		"--config", configPath, "--plan-out", planPath,
	}, nil)
	if previewCode != 3 || previewErr != "" {
		t.Fatalf("preview must require exact authorization: %d %q %q", previewCode, previewErr, previewOut)
	}
	var preview struct {
		Data struct {
			Authorization string          `json:"authorization_digest"`
			Status        string          `json:"status"`
			Plan          json.RawMessage `json:"plan"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(previewOut), &preview); err != nil || preview.Data.Status != "authorization_required" {
		t.Fatalf("invalid preview response: %q %v", previewOut, err)
	}
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(planBytes)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	if preview.Data.Authorization != digest || !json.Valid(preview.Data.Plan) {
		t.Fatalf("preview is not bound to exact saved plan: %q", previewOut)
	}
	t.Logf("Authorizing real synthetic payment CLI plan %s: %s", digest, planBytes)

	runCode, runOut, runErr := native(exe, root, home, []string{
		"run", "--plan-file", planPath, "--approve", digest, "--project", project, "--config", configPath,
	}, dockerEnv)
	if runCode != 4 || runErr != "" {
		t.Fatalf("payment run exit=%d stderr=%q stdout=%q", runCode, runErr, runOut)
	}
	var executed struct {
		Data struct {
			Status        string              `json:"status"`
			Authorization string              `json:"authorization_digest"`
			Receipt       evidence.Receipt    `json:"receipt"`
			Comparison    evidence.Comparison `json:"comparison"`
			Details       compare.Report      `json:"details"`
			Samples       []runner.Sample     `json:"samples"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(runOut), &executed); err != nil {
		t.Fatalf("invalid payment result: %v %q", err, runOut)
	}
	r := executed.Data.Receipt
	if executed.Data.Status != "completed" || executed.Data.Authorization != digest || r.State.Kind != evidence.Observed || r.State.Execution != evidence.Completed || r.State.Applicability != evidence.Current || r.Authorization != evidence.Digest(digest) || r.RequestID == "" || r.Bindings == nil || r.BaseEnvironment == nil || r.CandidateEnvironment == nil || len(executed.Data.Samples) != 8 {
		t.Fatalf("missing execution provenance: %+v", executed.Data)
	}
	if executed.Data.Comparison.Outcome != evidence.Different || executed.Data.Details.Receipt != r.ID || len(executed.Data.Details.Witnesses) != 16 {
		t.Fatalf("unexpected comparison result: %+v", executed.Data.Comparison)
	}
	for _, environment := range []*evidence.Environment{r.BaseEnvironment, r.CandidateEnvironment} {
		if !strings.HasPrefix(string(environment.Toolchain), "sha256:") || len(environment.Argv) == 0 {
			t.Fatalf("incomplete environment provenance: %+v", environment)
		}
	}

	observations := map[string]runner.Observation{}
	for _, sample := range executed.Data.Samples {
		if sample.Status != "completed" || sample.RequestID != r.RequestID || sample.Snapshots != r.Snapshots || !sample.Execution.App.Cleaned || !sample.Execution.Observer.Cleaned {
			t.Fatalf("sample lost its execution binding: %+v", sample)
		}
		var observationArtifact *evidence.Artifact
		for i := range sample.Artifacts {
			if strings.HasSuffix(sample.Artifacts[i].Channel, "/observation") {
				observationArtifact = &sample.Artifacts[i]
				break
			}
		}
		if observationArtifact == nil {
			t.Fatalf("sample has no observer-owned response/effects artifact: %+v", sample)
		}
		code, output, stderr := native(exe, root, home, []string{
			"inspect", string(observationArtifact.Content), "--project", project, "--config", configPath,
		}, nil)
		if code != 0 || stderr != "" {
			t.Fatalf("observation inspection: %d %q %q", code, stderr, output)
		}
		var inspected struct {
			Data struct {
				Document json.RawMessage `json:"document"`
			} `json:"data"`
		}
		var observation runner.Observation
		if err := json.Unmarshal([]byte(output), &inspected); err != nil {
			t.Fatalf("invalid observation inspection: %q %v", output, err)
		}
		if err := json.Unmarshal(inspected.Data.Document, &observation); err != nil {
			t.Fatalf("invalid observation artifact: %q %v", output, err)
		}
		key := sample.Side + "/" + strconv.FormatInt(sample.CaseSeconds, 10) + "/" + strconv.Itoa(sample.Repetition)
		observations[key] = observation
	}
	for _, seconds := range []int64{43200, 30} {
		base := observations["base/"+strconv.FormatInt(seconds, 10)+"/0"]
		candidate := observations["candidate/"+strconv.FormatInt(seconds, 10)+"/0"]
		if len(base.Responses) != 2 || len(candidate.Responses) != 2 || !reflect.DeepEqual(base.Responses, candidate.Responses) {
			t.Fatalf("paired responses differ at %ds: base=%+v candidate=%+v", seconds, base.Responses, candidate.Responses)
		}
		wantBase, wantCandidate := 1, 1
		if seconds == 43200 {
			wantCandidate = 2
		}
		if len(base.Calls) != wantBase || len(candidate.Calls) != wantCandidate {
			t.Fatalf("provider effects at %ds: base=%d candidate=%d", seconds, len(base.Calls), len(candidate.Calls))
		}
	}

	paymentBrowserProof(t, exe, root, home, project, configPath, r, executed.Data.Comparison)
	paymentReviewProof(t, exe, root, home, project, configPath, r, dockerEnv)

	// The same process boundary also proves imported outcomes stay reported.
	report := `{"Action":"run","Package":"example.invalid/import","Test":"TestPass"}
{"Action":"pass","Package":"example.invalid/import","Test":"TestPass"}
{"Action":"run","Package":"example.invalid/import","Test":"TestFail"}
{"Action":"fail","Package":"example.invalid/import","Test":"TestFail"}
`
	if err := os.WriteFile(reportPath, []byte(report), 0600); err != nil {
		t.Fatal(err)
	}
	importCode, importOut, importErr := native(exe, root, home, []string{
		"import", reportPath, "--project", project, "--config", configPath, "--producer", "synthetic-go-test-v1",
	}, nil)
	if importCode != 0 || importErr != "" || !strings.Contains(importOut, `"kind":"reported"`) || !strings.Contains(importOut, `"execution":"not_run"`) || !strings.Contains(importOut, `"report":"pass"`) || !strings.Contains(importOut, `"report":"fail"`) {
		t.Fatalf("imported pass/fail was not visibly reported: %d %q %q", importCode, importErr, importOut)
	}
}

func native(exe, dir, home string, args, extraEnv []string) (int, string, string) {
	if len(args) > 0 {
		switch args[0] {
		case "capture", "import", "inspect", "compare", "export", "run", "pin", "review", "config":
			args = append(args, "--json")
		}
	}
	encoded, _ := json.Marshal(args)
	cmd := exec.Command(exe, "-test.run=^TestNativeCLI$")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("")
	cmd.Env = append([]string{
		entryEnv + "=1", argsEnv + "=" + base64.RawURLEncoding.EncodeToString(encoded),
		"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, "config"),
		"PATH=" + filepath.Join(home, "no-tools"),
	}, extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			return -1, stdout.String(), stderr.String()
		}
		code = exit.ExitCode()
	}
	return code, stdout.String(), stderr.String()
}

func simpleCommits(t *testing.T, root string) (string, string) {
	t.Helper()
	gitRun(t, root, "init", "-q", "--template=", "-b", "main")
	writeFile(t, root, "go.mod", "module example.invalid/cli\n\ngo 1.27.1\n")
	writeFile(t, root, "app/main.go", "package main\nfunc main() {}\n")
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-qm", "base")
	base := strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD"))
	writeFile(t, root, "app/main.go", "package main\nfunc main() {}\n// changed\n")
	gitRun(t, root, "add", "app/main.go")
	gitRun(t, root, "commit", "-qm", "candidate")
	candidate := strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD"))
	return base, candidate
}

func paymentCommits(t *testing.T, root string) (string, string) {
	t.Helper()
	gitRun(t, root, "init", "-q", "--template=", "-b", "main")
	for _, name := range []string{"go.mod", "app/main.go", "app/config.go", "driver/main.go"} {
		path := filepath.Join("..", "..", "internal", "paymentfixture", "testdata", "payment", name)
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, name, string(contents))
	}
	gitRun(t, root, "add", "go.mod", "app", "driver")
	gitRun(t, root, "commit", "-qm", "synthetic base")
	base := strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD"))
	configFile := filepath.Join(root, "app/config.go")
	contents, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(contents), "24 * 60 * 60") != 1 {
		t.Fatal("payment fixture mutation anchor changed")
	}
	writeFile(t, root, "app/config.go", strings.Replace(string(contents), "24 * 60 * 60", "5 * 60", 1))
	gitRun(t, root, "add", "app/config.go")
	gitRun(t, root, "commit", "-qm", "synthetic candidate")
	candidate := strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD"))
	return base, candidate
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/usr/bin/git", args...)
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=/usr/bin:/bin", "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=AFTER fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=AFTER fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git command failed: %v (%s)", err, output)
	}
	return string(output)
}

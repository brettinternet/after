package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const outsideRepositoryDiagnostic = "after: not inside a Git repository — run AFTER in a checkout, or pass --project DIR"

func TestProjectRootFindsNearestGitDirectoryOrFile(t *testing.T) {
	for _, marker := range []string{"directory", "file"} {
		t.Run(marker, func(t *testing.T) {
			root := t.TempDir()
			gitEntry := filepath.Join(root, ".git")
			if marker == "directory" {
				if err := os.Mkdir(gitEntry, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(gitEntry, []byte("gitdir: elsewhere\n"), 0600); err != nil {
				t.Fatal(err)
			}
			nested := filepath.Join(root, "one", "two")
			if err := os.MkdirAll(nested, 0700); err != nil {
				t.Fatal(err)
			}
			got, ok := projectRoot(nested)
			if !ok || got != root {
				t.Fatalf("root=%q ok=%v; want %q", got, ok, root)
			}
		})
	}
	if _, ok := projectRoot(t.TempDir()); ok {
		t.Fatal("found a checkout outside Git")
	}
}

func TestCaptureFromNestedDirectoryUsesCheckoutRootAndIgnoresStore(t *testing.T) {
	project := filepath.Join(t.TempDir(), "checkout")
	makeProject(t, project)
	writeProjectFile(t, project, ".gitignore", "*.local-secret\n")
	gitRun(t, project, "add", ".gitignore")
	gitRun(t, project, "commit", "-qm", "user ignore rules")
	gitignoreBefore, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	excludePath := filepath.Join(project, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte("*.private\n"), 0600); err != nil {
		t.Fatal(err)
	}
	excludeBefore, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(project, "app", "nested")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	code, fromNested, stderr := invoke([]string{"capture", "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("nested capture: exit=%d stderr=%q", code, stderr)
	}
	var nestedResult struct {
		Data struct {
			Base struct {
				ID string `json:"id"`
			} `json:"base_snapshot"`
			Candidate struct {
				ID string `json:"id"`
			} `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(fromNested), &nestedResult); err != nil {
		t.Fatal(err)
	}
	if !validDigest(nestedResult.Data.Base.ID) || !validDigest(nestedResult.Data.Candidate.ID) {
		t.Fatalf("invalid nested capture result: %s", fromNested)
	}
	if _, err := os.Stat(filepath.Join(project, ".after", "writer.lock")); err != nil {
		t.Fatalf("store was not created at checkout root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, ".after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("nested store was created: %v", err)
	}
	ignore, err := os.ReadFile(filepath.Join(project, ".after", ".gitignore"))
	if err != nil || string(ignore) != "*\n" {
		t.Fatalf("store ignore file=%q err=%v", ignore, err)
	}
	if status := gitRun(t, project, "status", "--porcelain"); status != "" {
		t.Fatalf("capture exposed private storage to Git: %q", status)
	}
	gitignoreAfter, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil || !bytes.Equal(gitignoreBefore, gitignoreAfter) {
		t.Fatalf("user .gitignore changed: %q %v", gitignoreAfter, err)
	}
	excludeAfter, err := os.ReadFile(excludePath)
	if err != nil || !bytes.Equal(excludeBefore, excludeAfter) {
		t.Fatalf("Git exclude file changed: %q %v", excludeAfter, err)
	}

	t.Chdir(project)
	code, fromRoot, stderr := invoke([]string{"capture", "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("root capture: exit=%d stderr=%q", code, stderr)
	}
	var rootResult struct {
		Data struct {
			Base struct {
				ID string `json:"id"`
			} `json:"base_snapshot"`
			Candidate struct {
				ID string `json:"id"`
			} `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(fromRoot), &rootResult); err != nil {
		t.Fatal(err)
	}
	if rootResult.Data.Base.ID != nestedResult.Data.Base.ID || rootResult.Data.Candidate.ID != nestedResult.Data.Candidate.ID {
		t.Fatalf("nested capture differs from root: nested=%+v root=%+v", nestedResult.Data, rootResult.Data)
	}
	code, explicit, stderr := invoke([]string{"capture", "--project", nested, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("explicit nested project capture: exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(explicit, rootResult.Data.Candidate.ID) {
		t.Fatalf("--project did not resolve the checkout root: %s", explicit)
	}
}

func TestProjectCommandsOutsideGitFailBeforeCreatingStorage(t *testing.T) {
	outside := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AFTER_PROJECT", "")
	t.Setenv("AFTER_CONFIG", "")
	t.Chdir(outside)
	id := "sha256:" + strings.Repeat("a", 64)
	commands := []struct {
		name string
		args []string
	}{
		{"capture", []string{"capture"}},
		{"import", []string{"import", "missing-report.jsonl", "--producer", "fixture"}},
		{"inspect", []string{"inspect", id}},
		{"compare", []string{"compare", id}},
		{"export", []string{"export", id}},
		{"run", []string{"run"}},
		{"pin", []string{"pin", id, "--expectation", "test", "--scope", "finite_example", "--reason", "test"}},
		{"review", []string{"review", id}},
	}
	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			code, stdout, stderr := invoke(command.args, false, "")
			if code != ExitInvalid || stdout != "" || strings.TrimSuffix(stderr, "\n") != outsideRepositoryDiagnostic {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if _, err := os.Stat(filepath.Join(outside, ".after")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("project command created .after: %v", err)
			}
			if _, err := os.Stat(filepath.Join(outside, ".after", "writer.lock")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("project command created writer.lock: %v", err)
			}
		})
	}

	project := filepath.Join(outside, "checkout")
	makeProject(t, project)
	t.Chdir(project)
	code, stdout, stderr := invoke([]string{"capture", "--project", outside}, false, "")
	if code != ExitInvalid || stdout != "" || strings.TrimSuffix(stderr, "\n") != outsideRepositoryDiagnostic {
		t.Fatalf("external --project exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(outside, ".after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("external --project created storage: %v", err)
	}
}

func TestReadOnlyProjectCommandsDoNotCreateStore(t *testing.T) {
	project := filepath.Join(t.TempDir(), "checkout")
	makeProject(t, project)
	id := "sha256:" + strings.Repeat("a", 64)
	for _, args := range [][]string{{"inspect", id, "--project", project}, {"export", id, "--project", project}, {"review", id, "--project", project}} {
		code, stdout, stderr := invoke(args, false, "")
		if code != ExitOperational || stdout != "" || !strings.Contains(stderr, "cannot open private evidence store") {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
		if _, err := os.Stat(filepath.Join(project, ".after")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read-only command created .after: %v", err)
		}
	}
}

func TestHelpVersionAndConfigWorkOutsideGit(t *testing.T) {
	outside := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AFTER_PROJECT", "")
	t.Setenv("AFTER_CONFIG", "")
	t.Chdir(outside)
	for _, tc := range []struct {
		args []string
		want string
	}{{[]string{"help"}, "COMMANDS:"}, {[]string{"--help"}, "capture"}, {[]string{"version"}, Version}, {[]string{"--version"}, Version}, {[]string{"config"}, "configuration"}} {
		code, stdout, stderr := invoke(tc.args, false, "")
		if code != ExitOK || stderr != "" || !strings.Contains(stdout, tc.want) {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q", tc.args, code, stdout, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, ".after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("exempt command created storage: %v", err)
	}
}

func TestCaptureErrorsAreFixedAndActionable(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		fix    string
		setup  func(*testing.T, string) []string
	}{
		{
			name: "unmerged index", reason: "unmerged index is unsupported",
			fix: "resolve the index conflicts, then retry capture",
			setup: func(t *testing.T, project string) []string {
				makeProject(t, project)
				writeProjectFile(t, project, "conflict.txt", "base\n")
				gitRun(t, project, "add", "conflict.txt")
				gitRun(t, project, "commit", "-qm", "base conflict fixture")
				gitRun(t, project, "checkout", "-qb", "side")
				writeProjectFile(t, project, "conflict.txt", "REPOSITORY_CONTROLLED_side\n")
				gitRun(t, project, "add", "conflict.txt")
				gitRun(t, project, "commit", "-qm", "side conflict fixture")
				gitRun(t, project, "checkout", "-q", "main")
				writeProjectFile(t, project, "conflict.txt", "REPOSITORY_CONTROLLED_main\n")
				gitRun(t, project, "add", "conflict.txt")
				gitRun(t, project, "commit", "-qm", "main conflict fixture")
				cmd := exec.Command("git", "merge", "side")
				cmd.Dir = project
				cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + project, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=AFTER fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=AFTER fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid"}
				if output, err := cmd.CombinedOutput(); err == nil {
					t.Fatalf("conflict fixture merged cleanly: %s", output)
				}
				return []string{"capture", "--project", project}
			},
		},
		{
			name: "shallow repository", reason: "shallow repositories are unsupported",
			fix: "use a complete local clone, then retry capture",
			setup: func(t *testing.T, project string) []string {
				makeProject(t, project)
				head := strings.TrimSpace(gitRun(t, project, "rev-parse", "HEAD"))
				writeProjectFile(t, project, ".git/shallow", head+"\n")
				return []string{"capture", "--project", project}
			},
		},
		{
			name: "sparse repository", reason: "sparse or partial repositories are unsupported",
			fix: "use a complete local checkout, then retry capture",
			setup: func(t *testing.T, project string) []string {
				makeProject(t, project)
				gitRun(t, project, "config", "core.sparseCheckout", "true")
				return []string{"capture", "--project", project}
			},
		},
		{
			name: "ambiguous merge base", reason: "comparison needs exactly one merge base",
			fix: "choose refs with one merge base, then retry capture",
			setup: func(t *testing.T, project string) []string {
				base, _ := makeProject(t, project)
				gitRun(t, project, "checkout", "-qb", "left", base)
				writeProjectFile(t, project, "left.txt", "left\n")
				gitRun(t, project, "add", "left.txt")
				gitRun(t, project, "commit", "-qm", "left side")
				left := strings.TrimSpace(gitRun(t, project, "rev-parse", "HEAD"))
				gitRun(t, project, "checkout", "-qb", "right", base)
				writeProjectFile(t, project, "right.txt", "right\n")
				gitRun(t, project, "add", "right.txt")
				gitRun(t, project, "commit", "-qm", "right side")
				gitRun(t, project, "checkout", "-q", "left")
				gitRun(t, project, "merge", "--no-ff", "-m", "left merge", "right")
				gitRun(t, project, "checkout", "-q", "right")
				gitRun(t, project, "merge", "--no-ff", "-m", "right merge", left)
				bases := strings.Split(strings.TrimSpace(gitRun(t, project, "merge-base", "--all", "left", "right")), "\n")
				if len(bases) != 2 {
					t.Fatalf("fixture has %d merge bases, want 2: %v", len(bases), bases)
				}
				return []string{"capture", "--project", project, "--base", "left", "--target", "right"}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project := filepath.Join(t.TempDir(), "REPOSITORY_CONTROLLED_path")
			args := tc.setup(t, project)
			code, stdout, stderr := invoke(args, false, "")
			want := "after: capture failed: " + tc.reason + " — " + tc.fix
			if code != ExitOperational || stdout != "" || strings.TrimSuffix(stderr, "\n") != want {
				t.Fatalf("exit=%d stdout=%q stderr=%q; want %q", code, stdout, stderr, want)
			}
			if strings.Contains(stdout+stderr, project) || strings.Contains(stdout+stderr, "REPOSITORY_CONTROLLED") {
				t.Fatalf("repository data leaked: %q", stdout+stderr)
			}
		})
	}
}

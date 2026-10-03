package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The subprocess exercises the real entry point in an empty non-repository
// directory with no PATH, HOME, Docker, model credentials or stdin.
func TestNativeCLI(t *testing.T) {
	if os.Getenv("AFTER_TEST_ENTRY") == "1" {
		os.Args = append([]string{"after"}, strings.Split(os.Getenv("AFTER_TEST_ARGS"), "\n")...)
		main()
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, tc := range []struct {
		arg  string
		code int
		text string
	}{
		{"--help", 0, "No evidence loaded"},
		{"--version", 0, "after 0.1.0-dev"},
		{"capture", 2, "unknown argument"},
		{"--version\nextra", 2, "expected help or version"},
	} {
		cmd := exec.Command(exe, "-test.run=^TestNativeCLI$")
		cmd.Dir = dir
		cmd.Env = []string{"AFTER_TEST_ENTRY=1", "AFTER_TEST_ARGS=" + tc.arg, "PATH=" + filepath.Join(dir, "no-tools")}
		out, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		if code != tc.code || !strings.Contains(string(out), tc.text) {
			t.Fatalf("%q: exit %d, output %q", tc.arg, code, out)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("CLI wrote files: %v %v", entries, err)
	}
}

package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnedCleanup(t *testing.T) {
	w, err := createWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(t.TempDir(), "keep")
	if err = os.WriteFile(sentinel, []byte("unrelated"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(filepath.Dir(sentinel), filepath.Join(w.root, "external")); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(w.root, ".owner"), []byte("wrong"), 0600); err != nil {
		t.Fatal(err)
	}
	if w.cleanup() == nil {
		t.Fatal("accepted changed owner")
	}
	if err = os.WriteFile(filepath.Join(w.root, ".owner"), []byte(w.token), 0600); err != nil {
		t.Fatal(err)
	}
	if err = w.cleanup(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "unrelated" {
		t.Fatal("touched unrelated symlink target")
	}
	if _, err = os.Stat(w.root); !os.IsNotExist(err) {
		t.Fatal("owned directory remained")
	}
}
func TestReplacedWorkspaceRejected(t *testing.T) {
	w, err := createWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	moved := w.root + "-moved"
	if err = os.Rename(w.root, moved); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(moved, w.root); err != nil {
		t.Fatal(err)
	}
	if w.cleanup() == nil {
		t.Fatal("accepted replaced workspace")
	}
	// Remove link, restore original, and use the original verified owner.
	if err = os.Remove(w.root); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(moved, w.root); err != nil {
		t.Fatal(err)
	}
	if err = w.cleanup(); err != nil {
		t.Fatal(err)
	}
}
func TestInspectionWalk(t *testing.T) {
	t.Chdir("../..")
	binary := filepath.Join(t.TempDir(), "after")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/after")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	w, err := createWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := w.cleanup(); err != nil {
			t.Error(err)
		}
	}()
	d := demo{root: w.root, project: filepath.Join(w.root, "payment"), binary: binary, env: []string{"PATH=/usr/bin:/bin", "HOME=" + w.root, "XDG_CONFIG_HOME=" + w.root, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=demo", "GIT_AUTHOR_EMAIL=demo@example.invalid", "GIT_COMMITTER_NAME=demo", "GIT_COMMITTER_EMAIL=demo@example.invalid"}}
	if err = d.walk(false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(d.project, "app/config.go"))
	if err != nil || !strings.Contains(string(data), "5 * 60") {
		t.Fatal("fixture not edited")
	}
	// Denial must stop at preview, even without a Docker endpoint.
	var c capture
	if err = d.cli(0, &c, "capture"); err != nil {
		t.Fatal(err)
	}
	d.input = bufio.NewReader(strings.NewReader("no\n"))
	if _, err = d.execute(c, "denied", true); err == nil || !strings.Contains(err.Error(), "declined") {
		t.Fatalf("denial: %v", err)
	}
}

package capture

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func defaultBranchFixture(t *testing.T, branch string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	command(t, dir, "init", "-q", "--template=", "-b", branch)
	put(t, dir, "tracked", []byte("base\n"))
	base := commit(t, dir)
	command(t, dir, "switch", "-q", "-c", "feature")
	for _, content := range []string{"one\n", "two\n"} {
		put(t, dir, "tracked", []byte(content))
		commit(t, dir)
	}
	return dir, base
}

func TestFindDefaultBranchResolutionAndAheadCount(t *testing.T) {
	t.Run("origin HEAD", func(t *testing.T) {
		dir, base := defaultBranchFixture(t, "main")
		command(t, dir, "branch", "-D", "main")
		command(t, dir, "update-ref", "refs/remotes/origin/main", base)
		command(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
		branch, found, err := FindDefaultBranch(context.Background(), dir)
		if err != nil || !found || branch.Reference != "origin/main" || branch.Name != "main" || branch.Ahead != 2 {
			t.Fatalf("origin default branch: %+v found=%v err=%v", branch, found, err)
		}
	})

	t.Run("local main fallback", func(t *testing.T) {
		dir, _ := defaultBranchFixture(t, "main")
		branch, found, err := FindDefaultBranch(context.Background(), dir)
		if err != nil || !found || branch.Reference != "main" || branch.Name != "main" || branch.Ahead != 2 {
			t.Fatalf("main fallback: %+v found=%v err=%v", branch, found, err)
		}
	})

	t.Run("local master fallback", func(t *testing.T) {
		dir, _ := defaultBranchFixture(t, "master")
		branch, found, err := FindDefaultBranch(context.Background(), dir)
		if err != nil || !found || branch.Reference != "master" || branch.Name != "master" || branch.Ahead != 2 {
			t.Fatalf("master fallback: %+v found=%v err=%v", branch, found, err)
		}
	})

	t.Run("detached HEAD", func(t *testing.T) {
		dir, _ := defaultBranchFixture(t, "main")
		command(t, dir, "switch", "--detach", "-q", "HEAD")
		branch, found, err := FindDefaultBranch(context.Background(), dir)
		if err != nil || !found || branch.Reference != "main" || branch.Ahead != 2 {
			t.Fatalf("detached HEAD: %+v found=%v err=%v", branch, found, err)
		}
	})

	t.Run("unborn", func(t *testing.T) {
		dir := t.TempDir()
		command(t, dir, "init", "-q", "--template=", "-b", "main")
		branch, found, err := FindDefaultBranch(context.Background(), dir)
		if err != nil || found {
			t.Fatalf("unborn repository found a comparison: %+v found=%v err=%v", branch, found, err)
		}
	})

	t.Run("no candidate branch", func(t *testing.T) {
		dir, _ := defaultBranchFixture(t, "main")
		command(t, dir, "branch", "-D", "main")
		branch, found, err := FindDefaultBranch(context.Background(), dir)
		if err != nil || found {
			t.Fatalf("repository without a candidate default: %+v found=%v err=%v", branch, found, err)
		}
	})
}

func TestFindDefaultBranchIgnoresHostileGitConfiguration(t *testing.T) {
	dir, _ := defaultBranchFixture(t, "main")
	marker := filepath.Join(dir, "project-code-ran")
	command(t, dir, "config", "core.fsmonitor", "!touch "+marker)
	command(t, dir, "config", "core.hooksPath", filepath.Join(dir, ".git", "hooks"))
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(hook), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch \""+marker+"\"\n"), 0700); err != nil {
		t.Fatal(err)
	}

	branch, found, err := FindDefaultBranch(context.Background(), dir)
	if err != nil || !found || branch.Reference != "main" || branch.Ahead != 2 {
		t.Fatalf("hostile-config lookup: %+v found=%v err=%v", branch, found, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("project-configured command ran: %v", err)
	}
}

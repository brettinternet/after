package capture

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

func command(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid"}
	b, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}
func repo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	command(t, d, "init", "-q", "--template=", "-b", "main")
	return d
}
func put(t *testing.T, d, p string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(d, p)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, p), b, 0600); err != nil {
		t.Fatal(err)
	}
}
func commit(t *testing.T, d string) string {
	t.Helper()
	command(t, d, "add", "--all")
	command(t, d, "commit", "-qm", "fixture")
	return command(t, d, "rev-parse", "HEAD")
}
func openStore(t *testing.T, d string) *store.Store {
	t.Helper()
	s, err := store.Open(d, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func take(t *testing.T, d string, s *store.Store, o Options) Result {
	t.Helper()
	r, err := Capture(context.Background(), d, s, o)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func content(t *testing.T, s *store.Store, snap evidence.Snapshot, p string) []byte {
	t.Helper()
	for _, f := range snap.Files {
		if f.Path == p {
			b, err := s.ReadBlob(f.Content)
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
	}
	t.Fatalf("missing file %q", p)
	return nil
}
func listed(es []evidence.Limitation, p string) bool {
	for _, e := range es {
		if e.Path == p {
			return true
		}
	}
	return false
}
func treeState(t *testing.T, d string) map[string]string {
	t.Helper()
	m := map[string]string{}
	err := filepath.WalkDir(d, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(d, p)
		if rel == ".after" {
			return filepath.SkipDir
		}
		if e.IsDir() {
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			v, err := os.Readlink(p)
			m[rel] = info.Mode().String() + v
			return err
		}
		b, err := os.ReadFile(p)
		m[rel] = info.Mode().String() + string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestModesPathsAndNoSourceMutation(t *testing.T) {
	d := repo(t)
	put(t, d, "staged", []byte("base\n"))
	put(t, d, "delete", []byte("gone\n"))
	put(t, d, "old name", []byte("rename\n"))
	head := commit(t, d)
	put(t, d, "staged", []byte("index\n"))
	command(t, d, "add", "staged")
	put(t, d, "staged", []byte("work\n"))
	command(t, d, "mv", "old name", "renamed")
	if err := os.Remove(filepath.Join(d, "delete")); err != nil {
		t.Fatal(err)
	}
	paths := []string{"space name", "line\nbreak", "日本語", "-leading", "binary", "executable"}
	for _, p := range paths {
		put(t, d, p, []byte("new\x00bytes\n"))
		command(t, d, "add", "--", p)
	}
	if err := os.Chmod(filepath.Join(d, "executable"), 0700); err != nil {
		t.Fatal(err)
	}
	command(t, d, "add", "executable")
	put(t, d, "untracked", []byte("leave alone"))
	put(t, d, ".gitignore", []byte("secret\n.after/\n"))
	put(t, d, "secret", []byte("private"))
	before := treeState(t, d)
	s := openStore(t, d)
	r := take(t, d, s, Options{})
	if r.Base.Commit != head || r.Candidate.Source != evidence.WorkingTree || r.Index == nil || r.Candidate.IndexSnapshot != r.Index.ID {
		t.Fatalf("wrong identities: %+v", r)
	}
	for _, pair := range []struct {
		s    evidence.Snapshot
		want string
	}{{r.Base, "base\n"}, {*r.Index, "index\n"}, {r.Candidate, "work\n"}} {
		if got := string(content(t, s, pair.s, "staged")); got != pair.want {
			t.Fatalf("got %q", got)
		}
	}
	for _, p := range paths {
		if !bytes.Equal(content(t, s, r.Candidate, p), []byte("new\x00bytes\n")) {
			t.Fatal(p)
		}
	}
	if !listed(r.Candidate.Excluded, "untracked") || listed(r.Candidate.Excluded, "secret") {
		t.Fatal("untracked policy")
	}
	for _, f := range r.Candidate.Files {
		if f.Path == "delete" || f.Path == "old name" {
			t.Fatal("deleted path captured")
		}
		if f.Path == "executable" && f.Mode != "100755" {
			t.Fatal("lost executable mode")
		}
	}
	diff, err := s.ReadBlob(r.Candidate.Diff)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"GIT binary patch", "deleted file mode", "new file mode 100755", "-base", "+work"} {
		if !bytes.Contains(diff, []byte(want)) {
			t.Errorf("diff missing %q: %s", want, diff)
		}
	}
	staged := take(t, d, s, Options{Mode: evidence.Index})
	if !listed(staged.Candidate.Excluded, "untracked") {
		t.Fatal("index omitted untracked inventory")
	}
	if string(content(t, s, staged.Candidate, "staged")) != "index\n" {
		t.Fatal("staged captured working tree")
	}
	included := take(t, d, s, Options{IncludeUntracked: []string{"untracked"}})
	if string(content(t, s, included.Candidate, "untracked")) != "leave alone" {
		t.Fatal("explicit selection")
	}
	for _, p := range []string{"secret", ".after/writer.lock", "../outside", "missing"} {
		if _, err := Capture(context.Background(), d, s, Options{IncludeUntracked: []string{p}}); err == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	again := take(t, d, s, Options{})
	if r.Candidate.ID != again.Candidate.ID {
		t.Fatal("unstable identity")
	}
	if _, err := store.Get[evidence.Snapshot](s, r.Candidate.ID); err != nil {
		t.Fatal(err)
	}
	after := treeState(t, d)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("capture changed source repository, index, refs or files")
	}
}
func TestMergeBaseAndUnborn(t *testing.T) {
	t.Run("unborn", func(t *testing.T) {
		d := repo(t)
		put(t, d, "new", []byte("index"))
		command(t, d, "add", "new")
		put(t, d, "new", []byte("work"))
		s := openStore(t, d)
		r := take(t, d, s, Options{})
		if !r.Base.Unborn || r.Base.Commit != "" || len(r.Base.Files) != 0 || !r.Candidate.Unborn {
			t.Fatal("invented unborn commit")
		}
		if string(content(t, s, r.Candidate, "new")) != "work" {
			t.Fatal("unborn content")
		}
		take(t, d, s, Options{Mode: evidence.Index})
		again := take(t, d, s, Options{})
		if r.Candidate.ID != again.Candidate.ID {
			t.Fatal("private store changed capture identity")
		}
	})
	t.Run("merge-base", func(t *testing.T) {
		d := repo(t)
		put(t, d, "file", []byte("common"))
		common := commit(t, d)
		command(t, d, "checkout", "-qb", "feature")
		put(t, d, "file", []byte("candidate"))
		target := commit(t, d)
		command(t, d, "checkout", "-q", "main")
		put(t, d, "other", []byte("base tip"))
		base := commit(t, d)
		put(t, d, "file", []byte("dirty must not capture"))
		s := openStore(t, d)
		r := take(t, d, s, Options{Mode: evidence.MergeBase, Base: "main", Target: "feature"})
		if r.Base.Commit != common || r.Candidate.Commit != target || r.Candidate.MergeBase != common || r.Candidate.BaseCommit != base {
			t.Fatal("wrong resolved pair")
		}
		if string(content(t, s, r.Candidate, "file")) != "candidate" {
			t.Fatal("branch content")
		}
		if r.Index != nil {
			t.Fatal("branch captured index")
		}
	})
}
func TestConcurrentEdits(t *testing.T) {
	for _, retry := range []bool{true, false} {
		t.Run(map[bool]string{true: "retry", false: "fail"}[retry], func(t *testing.T) {
			d := repo(t)
			put(t, d, "a", []byte("original"))
			commit(t, d)
			s := openStore(t, d)
			calls := 0
			r, err := capture(context.Background(), d, s, Options{}, func(attempt int) {
				calls++
				if !retry || attempt == 0 {
					put(t, d, "a", []byte(strings.Repeat("x", attempt+1)))
				}
			})
			if retry {
				if err != nil || calls != 2 {
					t.Fatalf("retry: %v %d", err, calls)
				}
				if string(content(t, s, r.Candidate, "a")) != "x" {
					t.Fatal("mixed content")
				}
			} else {
				if !errors.Is(err, ErrInconsistent) || calls != MaxAttempts {
					t.Fatalf("failure: %v %d", err, calls)
				}
				entries, _ := os.ReadDir(filepath.Join(d, ".after"))
				if len(entries) != 1 {
					t.Fatal("published failed capture")
				}
			}
		})
	}
}
func TestUnsupportedInventory(t *testing.T) {
	d := repo(t)
	put(t, d, "normal", []byte("ok"))
	put(t, d, "lfs", []byte("version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 20\n"))
	put(t, d, "large", make([]byte, MaxFileBytes+1))
	put(t, d, ".after/tracked", []byte("must not capture"))
	outside := t.TempDir()
	put(t, outside, "private", []byte("outside"))
	if err := os.Symlink(filepath.Join(outside, "private"), filepath.Join(d, "link")); err != nil {
		t.Fatal(err)
	}
	head := commit(t, d)
	command(t, d, "update-index", "--add", "--cacheinfo", "160000,"+head+",module")
	s := openStore(t, d)
	r := take(t, d, s, Options{})
	for _, p := range []string{"lfs", "large", "link", "module"} {
		if !listed(r.Candidate.Unsupported, p) {
			t.Errorf("missing limitation for %s", p)
		}
	}
	if r.Candidate.Completeness != evidence.Incomplete || !listed(r.Candidate.Excluded, ".after/tracked") {
		t.Fatal("false completeness/private content")
	}
	// An untracked symlink directory is inventoried, never followed.
	if err := os.Symlink(outside, filepath.Join(d, "external")); err != nil {
		t.Fatal(err)
	}
	r = take(t, d, s, Options{IncludeUntracked: []string{"external"}})
	if !listed(r.Candidate.Unsupported, "external") {
		t.Fatal("symlink not limited")
	}
}
func TestRejectUnsupportedRepositoryState(t *testing.T) {
	for _, kind := range []string{"conflict", "sparse", "skip", "assume", "missing", "partial", "shallow"} {
		t.Run(kind, func(t *testing.T) {
			d := repo(t)
			put(t, d, "file", []byte("base"))
			head := commit(t, d)
			switch kind {
			case "conflict":
				command(t, d, "checkout", "-qb", "side")
				put(t, d, "file", []byte("side"))
				commit(t, d)
				command(t, d, "checkout", "-q", "main")
				put(t, d, "file", []byte("main"))
				commit(t, d)
				c := exec.Command("git", "merge", "side")
				c.Dir = d
				c.Run()
			case "sparse":
				command(t, d, "config", "core.sparseCheckout", "true")
			case "skip":
				command(t, d, "update-index", "--skip-worktree", "file")
			case "assume":
				command(t, d, "update-index", "--assume-unchanged", "file")
			case "missing":
				oid := command(t, d, "rev-parse", "HEAD:file")
				if err := os.Remove(filepath.Join(d, ".git/objects", oid[:2], oid[2:])); err != nil {
					t.Fatal(err)
				}
			case "partial":
				command(t, d, "config", "extensions.partialClone", "origin")
			case "shallow":
				put(t, d, ".git/shallow", []byte(head+"\n"))
			}
			s := openStore(t, d)
			if _, err := Capture(context.Background(), d, s, Options{}); err == nil {
				t.Fatal("unsupported repository accepted")
			}
		})
	}
}
func TestNonRegularAndParentSymlink(t *testing.T) {
	d := repo(t)
	put(t, d, "dir/file", []byte("base"))
	put(t, d, "fifo", []byte("base"))
	commit(t, d)
	if err := os.Rename(filepath.Join(d, "dir"), filepath.Join(d, "saved")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	put(t, outside, "file", []byte("outside"))
	if err := os.Symlink(outside, filepath.Join(d, "dir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(d, "fifo")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(d, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	s := openStore(t, d)
	r := take(t, d, s, Options{})
	if !listed(r.Candidate.Unsupported, "dir/file") || !listed(r.Candidate.Unsupported, "fifo") {
		t.Fatal("followed unsupported filesystem content")
	}
}
func TestHostileGitConfigurationAndEnvironment(t *testing.T) {
	d := repo(t)
	put(t, d, "file", []byte("base\n"))
	put(t, d, ".gitattributes", []byte("* filter=hostile diff=hostile\n"))
	commit(t, d)
	marker := filepath.Join(t.TempDir(), "executed")
	helper := filepath.Join(t.TempDir(), "helper")
	put(t, filepath.Dir(helper), filepath.Base(helper), []byte("#!/bin/sh\nprintf invoked >> '"+marker+"'\nexit 1\n"))
	if err := os.Chmod(helper, 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"core.fsmonitor", "diff.external", "diff.hostile.command", "diff.hostile.textconv", "filter.hostile.clean", "filter.hostile.smudge", "filter.hostile.process", "core.sshCommand", "core.pager"} {
		command(t, d, "config", key, helper)
	}
	command(t, d, "config", "filter.hostile.required", "true")
	hooks := t.TempDir()
	for _, name := range []string{"post-index-change", "post-checkout", "pre-commit"} {
		put(t, hooks, name, []byte("#!/bin/sh\n'"+helper+"'\n"))
		os.Chmod(filepath.Join(hooks, name), 0700)
	}
	command(t, d, "config", "core.hooksPath", hooks)
	command(t, d, "config", "remote.origin.url", "ext::"+helper)
	put(t, d, "file", []byte("candidate\n"))
	s := openStore(t, d)
	for _, key := range []string{"GIT_EXTERNAL_DIFF", "GIT_SSH", "GIT_SSH_COMMAND", "GIT_PAGER", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG", "GIT_EXEC_PATH", "GIT_TEMPLATE_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_DIR", "GIT_WORK_TREE"} {
		t.Setenv(key, helper)
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
	t.Setenv("GIT_CONFIG_VALUE_0", helper)
	t.Setenv("GIT_TRACE", marker)
	t.Setenv("GIT_TRACE_SETUP", marker)
	fakeBin := t.TempDir()
	put(t, fakeBin, "git", []byte("#!/bin/sh\nprintf invoked >> '"+marker+"'\nexit 1\n"))
	if err := os.Chmod(filepath.Join(fakeBin, "git"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin)
	r := take(t, d, s, Options{})
	if string(content(t, s, r.Candidate, "file")) != "candidate\n" {
		t.Fatal("helper transformed content")
	}
	take(t, d, s, Options{Mode: evidence.Index})
	take(t, d, s, Options{Mode: evidence.MergeBase, Base: "HEAD", Target: "HEAD"})
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("repository helper or tracing executed")
	}
	if _, err := Capture(context.Background(), d, s, Options{Mode: evidence.MergeBase, Base: "--upload-pack=" + helper, Target: "HEAD"}); err == nil {
		t.Fatal("option injection accepted")
	}
}
func TestBoundsCancellationAndRedaction(t *testing.T) {
	t.Run("entries", func(t *testing.T) {
		var raw strings.Builder
		for i := 0; i <= MaxFiles; i++ {
			raw.WriteString("100644 blob " + strings.Repeat("a", 40) + "\tx\x00")
		}
		if _, err := parseEntries([]byte(raw.String()), false); !errors.Is(err, ErrBudget) {
			t.Fatal(err)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		d := repo(t)
		s := openStore(t, d)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := Capture(ctx, d, s, Options{}); err == nil {
			t.Fatal("cancel ignored")
		}
	})
	t.Run("redact", func(t *testing.T) {
		d := repo(t)
		put(t, d, "file", []byte("private-token"))
		commit(t, d)
		s, err := store.Open(d, true, []string{"private-token"})
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		r := take(t, d, s, Options{})
		if r.Candidate.Completeness != evidence.Incomplete || string(content(t, s, r.Candidate, "file")) != "[REDACTED]" {
			t.Fatal("redaction masquerades as complete source")
		}
	})
}

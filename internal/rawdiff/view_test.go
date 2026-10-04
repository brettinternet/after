package rawdiff

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/store"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("/usr/bin/git", args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid"}
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git: %v: %s", err, b)
	}
}
func put(t *testing.T, dir, path string, data []byte) {
	t.Helper()
	p := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func openStore(t *testing.T, dir string, secrets []string) *store.Store {
	t.Helper()
	s, err := store.Open(dir, true, secrets)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func view(t *testing.T, s *store.Store, r capture.Result) *View {
	t.Helper()
	v, err := Open(s, r.Base, r.Candidate)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func allRaw(t *testing.T, v *View, size int) []byte {
	t.Helper()
	var out []byte
	for offset := 0; ; {
		p, err := v.Raw(offset, size)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Bytes) > size || p.Offset != offset || p.Next != offset+len(p.Bytes) {
			t.Fatal("invalid window")
		}
		out = append(out, p.Bytes...)
		if !p.More {
			if len(out) != p.Total {
				t.Fatal("lost bytes")
			}
			return out
		}
		offset = p.Next
	}
}

func TestCapturedModesInventoryAndFrozenContext(t *testing.T) {
	for _, mode := range []evidence.SourceMode{evidence.WorkingTree, evidence.Index, evidence.MergeBase} {
		t.Run(string(mode), func(t *testing.T) {
			d := t.TempDir()
			git(t, d, "init", "-q", "--template=", "-b", "main")
			paths := []string{"prod.go", "unit_test.go", "fixtures/request.json", "masks.json", "expected.json", "reply.golden", "comparison-policy.json", "space name", "line\nbreak", "tab\tname", "日本語", "-leading", "quote\"name", "bell\aname", "a b/ambiguous", "mode"}
			for _, p := range paths {
				put(t, d, p, []byte("before\n"))
			}
			put(t, d, "deleted", []byte("gone\n"))
			put(t, d, "binary", []byte("a\x00b"))
			git(t, d, "add", "--all")
			git(t, d, "commit", "-qm", "base")
			for _, p := range paths {
				if p != "mode" {
					put(t, d, p, []byte("after\n"))
				}
			}
			if err := os.Chmod(filepath.Join(d, "mode"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(d, "deleted")); err != nil {
				t.Fatal(err)
			}
			put(t, d, "added", []byte("new\n"))
			put(t, d, "binary", []byte("c\x00d"))
			put(t, d, "large", make([]byte, capture.MaxFileBytes+1))
			if err := os.Symlink("prod.go", filepath.Join(d, "link")); err != nil {
				t.Fatal(err)
			}
			git(t, d, "add", "--all")
			opts := capture.Options{Mode: mode}
			if mode == evidence.MergeBase {
				git(t, d, "checkout", "-qb", "feature")
				git(t, d, "commit", "-qm", "candidate")
				opts.Base = "main"
				opts.Target = "feature"
			} else if mode == evidence.Index {
				put(t, d, "prod.go", []byte("unstaged must not appear\n"))
			}
			put(t, d, "untracked", []byte("not selected\n"))
			s := openStore(t, d, nil)
			r, err := capture.Capture(context.Background(), d, s, opts)
			if err != nil {
				t.Fatal(err)
			}
			v := view(t, s, r)
			entries := map[string]Entry{}
			for _, e := range v.Inventory() {
				entries[e.Path] = e
			}
			for _, p := range append(paths, "added", "deleted", "binary", "large", "link") {
				if _, ok := entries[p]; !ok {
					t.Errorf("missing %q", p)
				}
			}
			wantCount := len(paths) + 5
			if mode != evidence.MergeBase {
				wantCount++
			}
			if len(entries) != wantCount {
				t.Fatalf("inventory count %d want %d", len(entries), wantCount)
			}
			if entries["added"].Change != "added" || entries["deleted"].Change != "deleted" || !entries["binary"].Binary || entries["large"].Change != "unknown" || len(entries["link"].Limits) == 0 {
				t.Fatalf("bad inventory: %+v", entries)
			}
			for _, p := range paths[1:7] {
				if !entries[p].PotentialOracle {
					t.Error("oracle not highlighted", p)
				}
			}
			if entries["prod.go"].PotentialOracle {
				t.Fatal("production labeled oracle")
			}
			if mode != evidence.MergeBase && entries["untracked"].Change != "unknown" {
				t.Fatal("untracked hidden")
			}
			want, err := s.ReadBlob(r.Candidate.Diff)
			if err != nil {
				t.Fatal(err)
			}
			if got := allRaw(t, v, 37); !bytes.Equal(got, want) {
				t.Fatal("raw bytes changed")
			}
			if !bytes.Contains(want, []byte("old mode 100644")) {
				t.Fatal("mode change hidden")
			}
			seen := map[string]bool{}
			for _, h := range v.Hunks() {
				if h.Path == "" || !bytes.HasPrefix(want[h.Start:h.End], []byte("@@ ")) {
					t.Fatalf("bad hunk: %+v", h)
				}
				seen[h.Path] = true
			}
			for _, p := range paths {
				if p != "mode" && !seen[p] {
					t.Errorf("unindexed path %q", p)
				}
			}
			if len(v.Hunks()) != len(paths)+1 {
				t.Fatalf("hunks: %d", len(v.Hunks()))
			}
			// Live edits and malicious helper configuration cannot affect access.
			put(t, d, "prod.go", []byte("live changed again\n"))
			marker := filepath.Join(t.TempDir(), "called")
			helper := filepath.Join(t.TempDir(), "helper")
			put(t, filepath.Dir(helper), filepath.Base(helper), []byte("#!/bin/sh\nprintf called > '"+marker+"'\n"))
			if err := os.Chmod(helper, 0700); err != nil {
				t.Fatal(err)
			}
			git(t, d, "config", "diff.external", helper)
			t.Setenv("GIT_EXTERNAL_DIFF", helper)
			t.Setenv("PATH", "/nonexistent")
			for _, side := range []Side{Base, Candidate} {
				p, err := v.Context(side, "prod.go", 0, MaxPageBytes)
				if err != nil {
					t.Fatal(err)
				}
				w := "before\n"
				if side == Candidate {
					w = "after\n"
				}
				if string(p.Bytes) != w {
					t.Fatalf("context %q", p.Bytes)
				}
			}
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("helper executed")
			}
			if !bytes.Equal(allRaw(t, v, MaxPageBytes), want) {
				t.Fatal("live edit changed raw diff")
			}
			for _, p := range []string{"../prod.go", "/prod.go", "a/../prod.go", "large", "link", "missing"} {
				if _, err := v.Context(Candidate, p, 0, 10); !errors.Is(err, ErrContext) {
					t.Errorf("context accepted %q: %v", p, err)
				}
			}
			if _, err := v.Context(Base, "added", 0, 10); !errors.Is(err, ErrContext) {
				t.Fatal("invented added base")
			}
			if _, err := v.Context(Candidate, "deleted", 0, 10); !errors.Is(err, ErrContext) {
				t.Fatal("invented deleted context")
			}
		})
	}
}

func synthetic(t *testing.T, raw, source []byte, secrets []string) (*View, *store.Store, capture.Result, string) {
	t.Helper()
	d := t.TempDir()
	s := openStore(t, d, secrets)
	a, err := s.PutArtifact(raw, "git-diff", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.PutArtifact(source, "source", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	base := evidence.Snapshot{SchemaVersion: 1, Source: evidence.Commit, Unborn: true, Completeness: evidence.Complete, Diff: a.Content}
	candidate := base
	candidate.Source = evidence.WorkingTree
	candidate.Files = []evidence.File{{Path: "file", Mode: "100644", Content: b.Content}}
	if a.Redacted || b.Redacted {
		base.Completeness = evidence.Incomplete
		candidate.Completeness = evidence.Incomplete
		base.Limits = []string{"redacted"}
		candidate.Limits = []string{"redacted"}
	}
	base, err = store.Put(s, base)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err = store.Put(s, candidate)
	if err != nil {
		t.Fatal(err)
	}
	r := capture.Result{Base: base, Candidate: candidate}
	return view(t, s, r), s, r, d
}

const patch = "diff --git a/file b/file\nnew file mode 100644\n--- /dev/null\n+++ b/file\n@@ -0,0 +1 @@\n+hello\n"

func TestHunkIdentityAccountingAndCopies(t *testing.T) {
	v, s, r, _ := synthetic(t, []byte(patch+"@@ -10 +10 @@\n-old\n+new\n"), []byte("hello\n"), nil)
	h := v.Hunks()
	if len(h) != 2 || h[0].ID == h[1].ID {
		t.Fatal(h)
	}
	if again := view(t, s, r).Hunks(); !reflect.DeepEqual(again, h) {
		t.Fatal("unstable hunk identities")
	}
	c, err := v.Count(nil, nil)
	if err != nil || c.Total != 2 || c.Unclassified != 2 || !c.Complete {
		t.Fatal(c, err)
	}
	c, err = v.Count([][]evidence.Digest{{h[0].ID, h[0].ID}, {h[0].ID, h[1].ID}}, []evidence.Digest{h[0].ID, h[0].ID})
	if err != nil || c.Mapped != 1 || c.Folded != 1 || c.Unclassified != 0 || c.Total != c.Mapped+c.Folded+c.Unclassified {
		t.Fatal(c, err)
	}
	if _, err := v.Count([][]evidence.Digest{{"unknown"}}, nil); err == nil {
		t.Fatal("unknown mapping accepted")
	}
	if _, err := v.Count(nil, []evidence.Digest{h[0].ID}); err == nil {
		t.Fatal("unmapped fold accepted")
	}
	h[0].Path = "mutated"
	entries := v.Inventory()
	entries[0].Candidate.Path = "mutated"
	p, _ := v.Raw(0, 10)
	p.Bytes[0] = 'X'
	if v.Hunks()[0].Path != "file" || v.Inventory()[0].Candidate.Path != "file" || !bytes.Equal(allRaw(t, v, 10), []byte(patch+"@@ -10 +10 @@\n-old\n+new\n")) {
		t.Fatal("consumer mutated view")
	}
}

func TestBoundsLargePatchAndPartialIndex(t *testing.T) {
	// A line longer than bufio.Scanner's default limit must remain accessible.
	raw := []byte(patch + "+" + strings.Repeat("x", 3*MaxPageBytes) + "\n")
	v, _, _, _ := synthetic(t, raw, raw, nil)
	if !bytes.Equal(allRaw(t, v, MaxPageBytes), raw) {
		t.Fatal("long line truncated")
	}
	for _, w := range [][2]int{{-1, 1}, {len(raw) + 1, 1}, {0, 0}, {0, -1}, {0, MaxPageBytes + 1}, {int(^uint(0) >> 1), 1}} {
		if _, err := v.Raw(w[0], w[1]); !errors.Is(err, ErrWindow) {
			t.Fatal(w, err)
		}
		if _, err := v.Context(Candidate, "file", w[0], w[1]); !errors.Is(err, ErrWindow) {
			t.Fatal(w, err)
		}
	}
	p, err := v.Raw(len(raw), 1)
	if err != nil || p.More || len(p.Bytes) != 0 {
		t.Fatal(p, err)
	}
	many := []byte("diff --git a/file b/file\n" + strings.Repeat("@@ -0,0 +1 @@\n+x\n", MaxHunks+1))
	v, _, _, _ = synthetic(t, many, []byte("x"), nil)
	c, err := v.Count(nil, nil)
	if err != nil || c.Total != MaxHunks || c.Complete || len(v.Limits()) == 0 {
		t.Fatal(c, err)
	}
	if !bytes.Equal(allRaw(t, v, MaxPageBytes), many) {
		t.Fatal("index cap discarded raw bytes")
	}
}

func TestMissingArtifactsRedactionAndInvalidPair(t *testing.T) {
	v, s, r, d := synthetic(t, []byte(patch), []byte("hello\n"), nil)
	// Loss of context after opening cannot disable the retained ordinary diff.
	blobPath := func(id evidence.Digest) string {
		return filepath.Join(d, ".after", "blob-"+strings.TrimPrefix(string(id), "sha256:"))
	}
	if err := os.Remove(blobPath(r.Candidate.Files[0].Content)); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Context(Candidate, "file", 0, 10); !errors.Is(err, ErrContext) {
		t.Fatal(err)
	}
	if !bytes.Equal(allRaw(t, v, 10), []byte(patch)) {
		t.Fatal("missing source hid diff")
	}
	if err := os.Remove(blobPath(r.Candidate.Diff)); err != nil {
		t.Fatal(err)
	}
	v = view(t, s, r)
	if len(v.Inventory()) != 1 || len(v.Limits()) == 0 {
		t.Fatal("missing diff hid inventory")
	}
	if _, err := v.Raw(0, 10); !errors.Is(err, ErrDiff) {
		t.Fatal(err)
	}
	if c, _ := v.Count(nil, nil); c.Complete {
		t.Fatal("missing diff is complete")
	}
	r.Candidate.Diff = evidence.Digest("sha256:" + strings.Repeat("a", 64))
	v, err := Open(s, r.Base, r.Candidate)
	if err != nil || len(v.Inventory()) != 1 || !strings.Contains(strings.Join(v.Limits(), " "), "not a captured base/candidate pair") {
		t.Fatal("cross-capture inventory unavailable", err)
	}
	if _, err := v.Raw(0, 10); !errors.Is(err, ErrDiff) {
		t.Fatal("unrelated patch exposed", err)
	}
	if count, _ := v.Count(nil, nil); count.Complete {
		t.Fatal("unrelated patch claimed complete")
	}
	r.Candidate = r.Base
	r.Candidate.Files = []evidence.File{{Path: "../outside", Content: r.Base.Diff, Mode: "100644"}}
	if _, err := Open(s, r.Base, r.Candidate); err == nil {
		t.Fatal("traversal manifest accepted")
	}
	v, _, _, _ = synthetic(t, []byte(patch), []byte("hello\n"), []string{"hello"})
	p, err := v.Context(Candidate, "file", 0, 100)
	if err != nil || string(p.Bytes) != "[REDACTED]\n" || len(p.Limits) == 0 {
		t.Fatal(p, err)
	}
	if !bytes.Contains(allRaw(t, v, 10), []byte("[REDACTED]")) {
		t.Fatal("redaction lost")
	}
}

func TestFallbackAndEmptyDiff(t *testing.T) {
	v, s, r, _ := synthetic(t, []byte(patch), []byte("hello\n"), nil)
	// Malformed and missing adapters cannot hide the ordinary view.
	report, err := gotestreport.Import(strings.NewReader("not JSON\n"), gotestreport.Metadata{Producer: "fixture", Snapshot: r.Candidate.ID, ImportedAt: time.Unix(1, 0)})
	if err != nil || report.Completeness != evidence.Incomplete {
		t.Fatal(report, err)
	}
	if _, err := gotestreport.Import(nil, gotestreport.Metadata{}); err == nil {
		t.Fatal("missing report accepted")
	}
	frozen, err := store.Put(s, evidence.Scenario{SchemaVersion: 1, Input: r.Candidate.Files[0].Content, Driver: r.Candidate.Diff, Observer: r.Candidate.Diff, Rules: r.Candidate.Diff, Boundary: "fixture", Author: "test", Limits: []string{"synthetic"}})
	if err != nil {
		t.Fatal(err)
	}
	v = view(t, s, r)
	after, err := store.Get[evidence.Scenario](s, frozen.ID)
	if err != nil || !reflect.DeepEqual(after, frozen) {
		t.Fatal("view replaced frozen scenario", err)
	}
	if !bytes.Equal(allRaw(t, v, 100), []byte(patch)) {
		t.Fatal("raw fallback unavailable")
	}
	v, _, _, _ = synthetic(t, nil, nil, nil)
	if len(v.Hunks()) != 0 {
		t.Fatal("empty patch invented hunks")
	}
	c, err := v.Count(nil, nil)
	if err != nil || c.Total != 0 || !c.Complete {
		t.Fatal(c, err)
	}
	p, err := v.Raw(0, 1)
	if err != nil || p.More || p.Total != 0 {
		t.Fatal(p, err)
	}
}

func TestRedactionCollisionRetainsUnknownPath(t *testing.T) {
	d := t.TempDir()
	git(t, d, "init", "-q", "--template=", "-b", "main")
	put(t, d, "file", []byte("alpha\n"))
	git(t, d, "add", "--all")
	git(t, d, "commit", "-qm", "base")
	put(t, d, "file", []byte("beta\n"))
	s := openStore(t, d, []string{"alpha", "beta"})
	r, err := capture.Capture(context.Background(), d, s, capture.Options{Mode: evidence.WorkingTree})
	if err != nil {
		t.Fatal(err)
	}
	if r.Base.Completeness != evidence.Incomplete || r.Candidate.Completeness != evidence.Incomplete || !reflect.DeepEqual(r.Base.Files, r.Candidate.Files) {
		t.Fatal("fixture must collapse different originals into equal incomplete manifests")
	}
	v := view(t, s, r)
	entries := v.Inventory()
	if len(entries) != 1 || entries[0].Path != "file" || entries[0].Change != "unknown" || len(entries[0].Limits) == 0 {
		t.Fatalf("redaction collision hid changed path: %+v", entries)
	}
	if !bytes.Contains(allRaw(t, v, MaxPageBytes), []byte("diff --git a/file b/file")) {
		t.Fatal("raw patch lost changed path")
	}
	if c, err := v.Count(nil, nil); err != nil || c.Complete {
		t.Fatal(c, err)
	}
}

func TestPotentialOracleHeuristic(t *testing.T) {
	for _, p := range []string{"Test/thing", "foo.spec.ts", "fixture.json", "MASK", "value.golden", "expected-output.txt", "comparison-rules.json"} {
		if !potentialOracle(p) {
			t.Fatal(p)
		}
	}
	if potentialOracle("src/payment.go") {
		t.Fatal("production")
	}
	// This intentionally admits false positives: labels are hints, not approvals.
	if !potentialOracle("contest.go") {
		t.Fatal("heuristic changed")
	}
}

func TestPatchOnlyForCapturedPair(t *testing.T) {
	d := t.TempDir()
	git(t, d, "init", "-q", "--template=", "-b", "main")
	put(t, d, "prod.go", []byte("before\n"))
	git(t, d, "add", "--all")
	git(t, d, "commit", "-qm", "base")
	put(t, d, "prod.go", []byte("staged\n"))
	git(t, d, "add", "--all")
	put(t, d, "prod.go", []byte("unstaged\n"))
	s := openStore(t, d, nil)
	r, err := capture.Capture(context.Background(), d, s, capture.Options{})
	if err != nil || r.Index == nil {
		t.Fatal(err)
	}
	if raw := allRaw(t, view(t, s, r), MaxPageBytes); !bytes.Contains(raw, []byte("+unstaged")) {
		t.Fatalf("captured pair lost its patch: %q", raw)
	}
	if staged, err := s.ReadBlob(r.Index.Diff); err != nil || !bytes.Contains(staged, []byte("+staged")) || bytes.Contains(staged, []byte("unstaged")) {
		t.Fatalf("index snapshot diff is not the staged patch: %q %v", staged, err)
	}
	for name, pair := range map[string][2]evidence.Snapshot{
		"base-index":          {r.Base, *r.Index},
		"index-candidate":     {*r.Index, r.Candidate},
		"reversed":            {r.Candidate, r.Base},
		"candidate-candidate": {r.Candidate, r.Candidate},
	} {
		v, err := Open(s, pair[0], pair[1])
		if err != nil {
			t.Fatal(name, err)
		}
		if p, err := v.Raw(0, MaxPageBytes); err == nil {
			t.Fatalf("%s: patch for another pair exposed: %q", name, p.Bytes)
		}
		if len(v.Inventory()) == 0 && name != "candidate-candidate" {
			t.Fatal(name, "inventory hidden")
		}
	}
}

package browser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
)

func TestComputedBrowserDiffEventBudget(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2025-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2025-01-01T00:00:00Z"}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", err, output)
		}
	}
	write := func(replacement string) {
		for file := 0; file < 10; file++ {
			var body strings.Builder
			for line := 0; line < 1000; line++ {
				prefix := "before"
				if line == 500 {
					prefix = replacement
				}
				fmt.Fprintf(&body, "%s file=%02d line=%04d %s\n", prefix, file, line, strings.Repeat("x", 48))
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("file-%02d.txt", file)), []byte(body.String()), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	git("init", "-q", "--template=", "-b", "main")
	write("before")
	git("add", ".")
	git("commit", "-qm", "bounded computed-diff base")
	s, err := store.Open(dir, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := capture.Capture(context.Background(), dir, s, capture.Options{})
	if err != nil {
		t.Fatal(err)
	}
	write("after!")
	second, err := capture.Capture(context.Background(), dir, s, capture.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	selected := Selection{Project: dir, Pair: evidence.SnapshotPair{Base: first.Base.ID, Candidate: second.Candidate.ID}}
	probeStore, err := store.Open(dir, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := rawdiff.Open(probeStore, first.Base, second.Candidate)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var computeBefore, computeAfter runtime.MemStats
	runtime.ReadMemStats(&computeBefore)
	if _, err := probe.Compute(context.Background()); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&computeAfter)
	computeAllocated := computeAfter.TotalAlloc - computeBefore.TotalAlloc
	if err := probeStore.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	data, err := Load(context.Background(), selected)
	loadTime := time.Since(started)
	runtime.ReadMemStats(&after)
	loadAllocated := after.TotalAlloc - before.TotalAlloc
	if err != nil {
		t.Fatal(err)
	}
	if data.Diff == nil || data.Diff.Origin != rawdiff.ComputedOrigin || len(data.Diff.Files) != 10 || data.Diff.Added != 10 || data.Diff.Deleted != 10 {
		t.Fatalf("computed workload incomplete: origin=%q files=%d +%d -%d", data.Diff.Origin, len(data.Diff.Files), data.Diff.Added, data.Diff.Deleted)
	}
	started = time.Now()
	doc, err := terminal.NewDocument(data.Patch.Content)
	if err != nil {
		t.Fatal(err)
	}
	documentPrepare := time.Since(started)
	runtime.ReadMemStats(&after)
	documentAllocated := after.TotalAlloc - before.TotalAlloc - loadAllocated
	if doc.Lines() == 0 || len(data.Diff.HunkRows) != 10 {
		t.Fatalf("computed patch lost display rows: document lines=%d hunks=%d", doc.Lines(), len(data.Diff.HunkRows))
	}
	m := New(context.Background(), selected, Jobs{})
	defer m.Close()
	m.data, m.doc, m.screen = data, doc, "patch"
	m.width, m.height, m.status = 120, 40, ""
	m.theme.Color = false
	var eventBefore, eventAfter runtime.MemStats
	runtime.ReadMemStats(&eventBefore)
	var inputMax, resizeMax time.Duration
	for i := 0; i < 1000; i++ {
		event := time.Now()
		m.Update(key("j"))
		_ = m.View()
		inputMax = max(inputMax, time.Since(event))
		event = time.Now()
		m.Update(tea.WindowSizeMsg{Width: 80 + i%2*40, Height: 24 + i%2*16})
		_ = m.View()
		resizeMax = max(resizeMax, time.Since(event))
	}
	runtime.ReadMemStats(&eventAfter)
	perEvent := (eventAfter.TotalAlloc - eventBefore.TotalAlloc) / 2000
	started = time.Now()
	m.Update(key("q"))
	quit := time.Since(started)
	t.Logf("computed browser %s/%s CPUs=%d source-pair-bytes<=%d raw=%d lines=%d files=%d hunks=%d compute-alloc=%d load=%s load-alloc=%d document-prepare=%s document-alloc=%d input+view max=%s resize+view max=%s quit=%s alloc/event=%d", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), rawdiff.MaxComputedTotalBytes, len(data.Patch.Content), doc.Lines(), len(data.Diff.Files), len(data.Diff.HunkRows), computeAllocated, loadTime, loadAllocated, documentPrepare, documentAllocated, inputMax, resizeMax, quit, perEvent)
	if loadTime > time.Second || documentPrepare > time.Second || computeAllocated > 64<<20 || loadAllocated > 64<<20 || documentAllocated > 64<<20 || inputMax > 100*time.Millisecond || resizeMax > 100*time.Millisecond || quit > 100*time.Millisecond || perEvent > 256<<10 {
		t.Fatal("computed browser diff performance budget exceeded")
	}
}

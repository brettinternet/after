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
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
)

func TestCapturedBrowserDiffEventBudget(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2025-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2025-01-01T00:00:00Z"}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", err, out)
		}
	}
	write := func(prefix string) {
		for file := 0; file < 50; file++ {
			var body strings.Builder
			for line := 0; line < 1000; line++ {
				fmt.Fprintf(&body, "%s file=%02d line=%04d %s\n", prefix, file, line, strings.Repeat("x", 48))
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("file-%02d.txt", file)), []byte(body.String()), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	git("init", "-q", "--template=", "-b", "main")
	write("before")
	git("add", ".")
	git("commit", "-qm", "captured 100k-line workload")
	write("after!")
	s, err := store.Open(dir, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	captured, err := capture.Capture(context.Background(), dir, s, capture.Options{})
	if err != nil {
		t.Fatal(err)
	}
	selected := Selection{Project: dir, Pair: evidence.SnapshotPair{Base: captured.Base.ID, Candidate: captured.Candidate.ID}}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	data, err := Load(context.Background(), selected)
	if err != nil {
		t.Fatal(err)
	}
	loadTime := time.Since(start)
	runtime.ReadMemStats(&after)
	loadAllocated := after.TotalAlloc - before.TotalAlloc
	beforeDocument := after.TotalAlloc
	startDocument := time.Now()
	doc, err := terminal.NewDocument(data.Patch.Content)
	if err != nil {
		t.Fatal(err)
	}
	documentPrepare := time.Since(startDocument)
	runtime.ReadMemStats(&after)
	documentAllocated := after.TotalAlloc - beforeDocument
	if data.Diff == nil || len(data.Diff.Rows) < 100000 || doc.Lines() < 100000 {
		t.Fatalf("captured workload was incomplete: rows=%d lines=%d", len(data.Diff.Rows), doc.Lines())
	}

	m := New(context.Background(), selected, Jobs{})
	defer m.Close()
	m.data, m.doc, m.screen = data, doc, "patch"
	m.width, m.height, m.status = 120, 40, ""
	m.theme.Color = false
	t.Setenv("PATH", t.TempDir())
	runtime.ReadMemStats(&before)
	var inputMax, resizeMax, navigationMax time.Duration
	for i := 0; i < 1000; i++ {
		start = time.Now()
		m.Update(key("j"))
		_ = m.View()
		inputMax = max(inputMax, time.Since(start))
		start = time.Now()
		m.Update(tea.WindowSizeMsg{Width: 80 + i%2*40, Height: 24 + i%2*16})
		_ = m.View()
		resizeMax = max(resizeMax, time.Since(start))
	}
	for i := 0; i < 1000; i++ {
		start = time.Now()
		m.Update(key("}"))
		_ = m.View()
		m.Update(key("]"))
		_ = m.View()
		navigationMax = max(navigationMax, time.Since(start))
	}
	runtime.ReadMemStats(&after)
	perEvent := (after.TotalAlloc - before.TotalAlloc) / 4000
	start = time.Now()
	m.Update(key("q"))
	quit := time.Since(start)
	t.Logf("browser %s/%s CPUs=%d raw=%d indexed-lines=%d rows=%d files=%d hunks=%d load=%s load-alloc=%d document-prepare=%s document-alloc=%d input+view max=%s resize+view max=%s navigation+view max=%s quit=%s alloc/event=%d", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), len(data.Patch.Content), doc.Lines(), len(data.Diff.Rows), len(data.Diff.Files), len(data.Diff.HunkRows), loadTime, loadAllocated, documentPrepare, documentAllocated, inputMax, resizeMax, navigationMax, quit, perEvent)
	if documentPrepare > time.Second || documentAllocated > 64<<20 || inputMax > 100*time.Millisecond || resizeMax > 100*time.Millisecond || navigationMax > 100*time.Millisecond || quit > 100*time.Millisecond || perEvent > 256<<10 {
		t.Fatal("browser captured-diff per-event budget exceeded")
	}
}

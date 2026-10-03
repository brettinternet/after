package terminal

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
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// Real Git capture, not an invented diff. All repository/process work completes
// before any model timing. Fixed synthetic inputs contain no participant data.
func capturedDiff(t testing.TB, fileCount int) []byte {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid"}
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	git("init", "-q", "--template=", "-b", "main")
	write := func(side string) {
		for file := 0; file < fileCount; file++ {
			var b strings.Builder
			for line := 0; line < 1000; line++ {
				fmt.Fprintf(&b, "%s file=%02d line=%04d %s\n", side, file, line, strings.Repeat("x", 48))
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("file-%02d.txt", file)), []byte(b.String()), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	write("before")
	git("add", ".")
	git("commit", "-qm", "synthetic workload")
	write("after!")
	s, err := store.Open(dir, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	captureStart := time.Now()
	captured, err := capture.Capture(context.Background(), dir, s, capture.Options{})
	if err != nil {
		t.Fatal(err)
	}
	captureTime := time.Since(captureStart)
	start := time.Now()
	v, err := rawdiff.Open(s, captured.Base, captured.Candidate)
	if err != nil {
		t.Fatal(err)
	}
	coldOpen := time.Since(start)
	counts, err := v.Count(nil, nil)
	if err != nil || !counts.Complete || counts.Total != fileCount || len(v.Inventory()) != fileCount {
		t.Fatalf("dropped or incomplete inventory: %d/%d hunks=%+v limits=%v error=%v", len(v.Inventory()), fileCount, counts, v.Limits(), err)
	}
	t.Logf("coverage: %d/%d paths, %d/%d unclassified hunks; limits=%v", len(v.Inventory()), fileCount, counts.Unclassified, counts.Total, v.Limits())
	start = time.Now()
	v, err = rawdiff.Open(s, captured.Base, captured.Candidate)
	if err != nil {
		t.Fatal(err)
	}
	warmOpen := time.Since(start)
	start = time.Now()
	for i := 0; i < 1000; i++ {
		if len(v.Inventory()) != fileCount {
			t.Fatal("cached inventory lost paths")
		}
	}
	listAverage := time.Since(start) / 1000
	t.Logf("files=%d capture=%s raw first-open=%s warm-open=%s cached-list/iteration=%s; cold application view, OS file cache warm from capture (not dropped)", fileCount, captureTime, coldOpen, warmOpen, listAverage)
	if warmOpen > time.Second || listAverage > 2*time.Second {
		t.Fatal("raw/list evaluation budget exceeded")
	}
	var raw []byte
	for offset := 0; ; {
		page, err := v.Raw(offset, rawdiff.MaxPageBytes)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, page.Bytes...)
		if !page.More {
			break
		}
		offset = page.Next
	}
	return raw
}

func TestCapturedDiffBudget(t *testing.T) {
	for _, size := range []struct {
		name  string
		files int
	}{{"medium", 10}, {"large", 50}} {
		t.Run(size.name, func(t *testing.T) { capturedDiffBudget(t, size.files) })
	}
}

func capturedDiffBudget(t *testing.T, files int) {
	raw := capturedDiff(t, files)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	doc, err := NewDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	prep := time.Since(start)
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	m := New(context.Background(), doc, Binding{"captured", "budget"}, "large captured diff")
	defer m.Close()
	// No executable can be found during Update/View; capture is already frozen.
	t.Setenv("PATH", t.TempDir())
	var inputMax, resizeMax time.Duration
	runtime.ReadMemStats(&before)
	for i := 0; i < 1000; i++ {
		start = time.Now()
		m.Update(key("j"))
		_ = m.View()
		inputMax = max(inputMax, time.Since(start))
		start = time.Now()
		m.Update(tea.WindowSizeMsg{Width: 40 + (i%2)*80, Height: 24})
		_ = m.View()
		resizeMax = max(resizeMax, time.Since(start))
	}
	runtime.ReadMemStats(&after)
	perEvent := (after.TotalAlloc - before.TotalAlloc) / 2000
	start = time.Now()
	_, cmd := m.Update(key("q"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("quit")
	}
	quit := time.Since(start)
	t.Logf("%s/%s CPUs=%d raw=%d lines=%d prepare=%s allocated=%d input+view max=%s resize+view max=%s quit=%s alloc/event=%d", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), len(raw), doc.Lines(), prep, allocated, inputMax, resizeMax, quit, perEvent)
	if prep > time.Second || allocated > 64<<20 || inputMax > 100*time.Millisecond || resizeMax > 100*time.Millisecond || quit > 100*time.Millisecond || perEvent > 256<<10 {
		t.Fatal("terminal evaluation budget exceeded")
	}
}

func BenchmarkCapturedViewport(b *testing.B) {
	raw := capturedDiff(b, 50)
	m := model(b, string(raw))
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(key("j"))
		_ = m.View()
	}
}

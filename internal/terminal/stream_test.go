package terminal

import (
	"bytes"
	"io"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWriteDiffSanitizesHostileTextAndPreservesPipeTabs(t *testing.T) {
	input := []byte("diff --git a/file b/file\n+safe\x1b]52;c;clipboard\a\u009b31m\u202e\xff\tend\n")
	var output bytes.Buffer
	changed, err := WriteDiff(&output, bytes.NewReader(input), Theme{}, false)
	if err != nil || !changed {
		t.Fatalf("WriteDiff: changed=%t err=%v", changed, err)
	}
	got := output.Bytes()
	if !utf8.Valid(got) || bytes.Contains(got, []byte{0x1b}) || bytes.Contains(got, []byte{0x07}) || bytes.Contains(got, []byte{0xc2, 0x9b}) || bytes.Contains(got, []byte("\u202e")) || !bytes.Contains(got, []byte("\\u001b]52;c;clipboard\\u0007\\u009b31m\\u202e�\tend")) {
		t.Fatalf("unsafe or altered pipe output: %q", got)
	}
}

func TestWriteDiffTTYUsesThemeAndExpandsTabs(t *testing.T) {
	input := []byte("diff --git a/file b/file\n+one\ttwo\n-old\n")
	for _, tc := range []struct {
		name  string
		theme Theme
		want  string
		color bool
	}{{"color", Theme{Color: true}, "\x1b[1m", true}, {"NO_COLOR", Theme{Color: false}, "\t", false}} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			changed, err := WriteDiff(&output, bytes.NewReader(input), tc.theme, true)
			if err != nil || changed {
				t.Fatalf("WriteDiff: changed=%t err=%v", changed, err)
			}
			got := output.String()
			if strings.Contains(got, "\t") || strings.Contains(got, "+one    two") != true {
				t.Fatalf("TTY tabs were not expanded: %q", got)
			}
			if strings.Contains(got, tc.want) != tc.color {
				t.Fatalf("color contract mismatch: %q", got)
			}
			if tc.color && (!strings.Contains(got, "\x1b[32m+one") || !strings.Contains(got, "\x1b[1;31m-old")) {
				t.Fatalf("unexpected diff styles: %q", got)
			}
		})
	}
}

func TestWriteDiffHandlesUTF8AcrossReadBoundary(t *testing.T) {
	prefix := strings.Repeat("x", (64<<10)-1)
	input := prefix + "界\nsecond\nthird\n"
	var output bytes.Buffer
	changed, err := WriteDiff(&output, strings.NewReader(input), Theme{}, false)
	if err != nil || changed || output.String() != input {
		t.Fatalf("UTF-8 boundary: changed=%t err=%v exact=%t", changed, err, output.String() == input)
	}
}

func TestWriteDiffStreams100000LinesWithinMemoryBudget(t *testing.T) {
	const lineCount = 100250
	patch := strings.Repeat(strings.Repeat("x", 73)+"\n", lineCount)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	changed, err := WriteDiff(io.Discard, strings.NewReader(patch), Theme{}, false)
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	if err != nil || changed {
		t.Fatalf("WriteDiff: changed=%t err=%v", changed, err)
	}
	if allocated > 64<<20 {
		t.Fatalf("100000-line streaming allocated %d bytes; budget is 64 MiB", allocated)
	}
}

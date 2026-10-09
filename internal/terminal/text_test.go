package terminal

import (
	"fmt"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

func assertSafe(t *testing.T, s string, width int) {
	t.Helper()
	if !utf8.ValidString(s) {
		t.Fatalf("invalid UTF-8: %q", s)
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			t.Fatalf("unsafe rune %U in %q", r, s)
		}
	}
	if uniseg.StringWidth(s) > width {
		t.Fatalf("width %d > %d: %q", uniseg.StringWidth(s), width, s)
	}
}

func TestLine(t *testing.T) {
	for _, tc := range []struct {
		in    string
		width int
		want  string
	}{
		{"界e\u0301", 3, "界e\u0301"},
		{"\u0301x", 3, "◌\u0301x"},
		{"界\tx", 8, "界  x"},
		{"a\tb", 8, "a   b"},
		{"abcdef", 4, "abc…"},
		{"界界界", 4, "界…"},
		{"abc", 0, ""},
		{"\xff", 3, "�"},
		{"x\r\ny", 40, `x\u000d\u000ay`},
	} {
		got := Line(tc.in, tc.width)
		if got != tc.want {
			t.Errorf("Line(%q,%d)=%q want %q", tc.in, tc.width, got, tc.want)
		}
		assertSafe(t, got, tc.width)
	}
	for _, payload := range []string{
		"\x1b[31mforged\x1b[0m", "\x1b]52;c;Y2xpcA==\a", "\x1b]0;title\x1b\\", "\x1b]8;;https://evil.invalid\x1b\\click\x1b]8;;\x1b\\",
		"\u009b2J\u009d52;c;Y2xpcA==\u009c", "\r\b\x7f\x00", "a\u202eb\u2066c", "a\u2028b\u2029c",
		strings.Repeat("\u0301", 100000), strings.Repeat("x", MaxTextBytes),
	} {
		for _, w := range []int{1, 2, 3, 40, 80, 240} {
			assertSafe(t, Line(payload, w), w)
		}
	}
}

func TestWrapSanitizesAndWrapsGraphemes(t *testing.T) {
	for _, tc := range []struct {
		in    string
		width int
		want  []string
	}{
		{"hello 界e\u0301 there", 7, []string{"hello ", "界e\u0301 ", "there"}},
		{"first\nsecond", 4, []string{"firs", "t", "seco", "nd"}},
		{"a\tb", 5, []string{"a   b"}},
		{"\u0301x", 2, []string{"◌\u0301x"}},
		{"界", 1, []string{"�"}},
	} {
		got := Wrap(tc.in, tc.width)
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("Wrap(%q,%d)=%q want %q", tc.in, tc.width, got, tc.want)
		}
		for _, line := range got {
			assertSafe(t, line, tc.width)
		}
	}
	for _, hostile := range []string{"\x1b]52;c;clipboard\a\u202eabc", strings.Repeat("x", MaxLineBytes+500)} {
		lines := Wrap(hostile, 8)
		if len(lines) == 0 || len(lines) > MaxLineBytes*6 || lines[len(lines)-1] == "" {
			t.Fatalf("unbounded or empty wrapped content: %d rows", len(lines))
		}
		for _, line := range lines {
			assertSafe(t, line, 8)
		}
	}
}

func TestLinePanIsColumnAndGraphemeSafe(t *testing.T) {
	for _, tc := range []struct {
		line string
		left int
		want string
	}{
		{"界e\u0301x", 1, "e\u0301x"},
		{"a\tb", 1, "   b"},
		{"界\tx", 3, " x"},
		{"\u0301x", 1, "x"},
	} {
		if got := LineAt(tc.line, tc.left, 8); got != tc.want {
			t.Errorf("LineAt(%q,%d)=%q want %q", tc.line, tc.left, got, tc.want)
		}
		assertSafe(t, LineAt(tc.line, tc.left, 8), 8)
	}
	long := strings.Repeat("x", MaxLineBytes+1)
	if got := LineAt(long, 0, 10); got != "xxxxxxxxx…" {
		t.Fatalf("bounded long line: %q", got)
	}
	if got := LineAt(long, MaxLineBytes-6, 10); got != "xxxxxx…" {
		t.Fatalf("long-line pan: %q", got)
	}
}

func TestDocument(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		lines int
	}{{"", 0}, {"a", 1}, {"a\n", 1}, {"\n\n", 2}, {"a\nb", 2}} {
		raw := []byte(tc.raw)
		d, err := NewDocument(raw)
		if err != nil {
			t.Fatal(err)
		}
		if d.Lines() != tc.lines {
			t.Fatal(tc, d.Lines())
		}
		if len(raw) > 0 {
			raw[0] = 'z'
			if d.text != tc.raw || string(d.RawBytes()) != tc.raw {
				t.Fatal("document aliases input")
			}
		}
	}
	if _, err := NewDocument(make([]byte, MaxTextBytes+1)); err == nil {
		t.Fatal("oversize bytes accepted")
	}
	limitedRaw := []byte(strings.Repeat("a\n", MaxLines) + "final")
	limited, err := NewDocument(limitedRaw)
	if err != nil || !limited.Limited() || limited.Lines() != MaxLines {
		t.Fatalf("line limit not explicit: lines=%d limited=%t err=%v", limited.Lines(), limited.Limited(), err)
	}
	if got := limited.LineAt(MaxLines-1, 0, 20); got != "a" || limited.LongLine(MaxLines-1) {
		t.Fatalf("text beyond the final indexed row leaked into it: %q", got)
	}
	if !strings.Contains(limited.HexLine(limited.HexRows()-1), "final") {
		t.Fatal("line-limited document lost exact trailing bytes")
	}
}

func TestDocumentViewRetainsExactBytesAndHexRows(t *testing.T) {
	raw := []byte("{\"a\":1}\r\n\xef\xbb\xbf \t")
	display := []byte("{\n  \"a\": 1\n}\r\n\xef\xbb\xbf \t")
	doc, err := NewDocumentView(display, raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.LineAt(1, 0, 30); got != `  "a": 1` {
		t.Fatalf("display text %q", got)
	}
	if string(doc.RawBytes()) != string(raw) {
		t.Fatal("formatted display changed exact bytes")
	}
	var recovered []byte
	for row := 0; row < doc.HexRows(); row++ {
		line := doc.HexLine(row)
		start := row * 16
		end := min(start+16, len(raw))
		if !strings.HasPrefix(line, fmt.Sprintf("%08x  ", start)) {
			t.Fatalf("bad hex offset: %q", line)
		}
		for i := start; i < end; i++ {
			var got byte
			if _, err := fmt.Sscanf(line[10+(i-start)*3:12+(i-start)*3], "%02x", &got); err != nil || got != raw[i] {
				t.Fatalf("hex byte %d: %q got=%02x want=%02x err=%v", i, line, got, raw[i], err)
			}
			recovered = append(recovered, got)
		}
	}
	if string(recovered) != string(raw) {
		t.Fatal("hex rows did not reach every original byte")
	}
}

func TestDocumentAutoHexNULBoundary(t *testing.T) {
	for _, tc := range []struct {
		at   int
		want bool
	}{{7999, true}, {8000, false}} {
		raw := []byte(strings.Repeat("x", 8001))
		raw[tc.at] = 0
		doc, err := NewDocument(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := doc.AutoHex(); got != tc.want {
			t.Fatalf("NUL at %d: autohex=%t want=%t", tc.at, got, tc.want)
		}
	}
}

func FuzzLine(f *testing.F) {
	for _, s := range []string{"\x1b]52;c;x\a", "界e\u0301\t", "\xff\u009b2J", strings.Repeat("\u0301", 100)} {
		f.Add(s, uint8(40))
	}
	f.Fuzz(func(t *testing.T, s string, w uint8) { assertSafe(t, Line(s, int(w)), min(int(w), maxWidth)) })
}

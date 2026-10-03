package terminal

import (
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
			if d.text != tc.raw {
				t.Fatal("document aliases input")
			}
		}
	}
	for _, raw := range [][]byte{make([]byte, MaxTextBytes+1), []byte(strings.Repeat("\n", MaxLines+1))} {
		if _, err := NewDocument(raw); err == nil {
			t.Fatal("oversize accepted")
		}
	}
}

func FuzzLine(f *testing.F) {
	for _, s := range []string{"\x1b]52;c;x\a", "界e\u0301\t", "\xff\u009b2J", strings.Repeat("\u0301", 100)} {
		f.Add(s, uint8(40))
	}
	f.Fuzz(func(t *testing.T, s string, w uint8) { assertSafe(t, Line(s, int(w)), min(int(w), maxWidth)) })
}

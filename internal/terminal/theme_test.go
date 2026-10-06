package terminal

import (
	"strings"
	"testing"
)

func TestThemeBoundary(t *testing.T) {
	for _, tc := range []struct {
		noColor, term string
		color         bool
	}{
		{"", "xterm-256color", true}, {"1", "xterm", false}, {"0", "xterm", false}, {"", "dumb", false},
	} {
		t.Setenv("NO_COLOR", tc.noColor)
		t.Setenv("TERM", tc.term)
		theme := DefaultTheme()
		if theme.Color != tc.color {
			t.Fatal(tc)
		}
		for style := Plain; style <= Reverse+1; style++ {
			raw := "\u0301\x1b[32m[EQUAL]\r\n\x1b]52;c;bad\a\t界"
			got := theme.Render(raw, 120, style, true)
			plain := (Theme{}).Render(raw, 120, style, true)
			if !strings.HasPrefix(plain, "◌") || strings.ContainsAny(plain, "\x1b\r\n\a") {
				t.Fatal("unsanitized text", plain)
			}
			if !tc.color && got != plain {
				t.Fatal("color disabled", got)
			}
			if tc.color && style > Plain && style <= Reverse && !strings.HasSuffix(got, "\x1b[0m") {
				t.Fatal("style leaked", got)
			}
		}
	}
}

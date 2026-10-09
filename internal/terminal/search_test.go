package terminal

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestDocumentSearchSmartCaseSanitizationAndCancellation(t *testing.T) {
	doc, err := NewDocument([]byte("alpha\nAlpha\nALPHA\nÉtage\néTAGE\n\x1b]52;c;blocked\a\na\tb\n"))
	if err != nil {
		t.Fatal(err)
	}
	matches, err := doc.Search(context.Background(), "alpha")
	if err != nil || !reflect.DeepEqual(matches, []int{0, 1, 2}) {
		t.Fatalf("case-insensitive matches = %v, %v", matches, err)
	}
	matches, err = doc.Search(context.Background(), "Alpha")
	if err != nil || len(matches) != 1 || matches[0] != 1 {
		t.Fatalf("smart-case match = %v, %v", matches, err)
	}
	matches, err = doc.Search(context.Background(), "étage")
	if err != nil || !reflect.DeepEqual(matches, []int{3, 4}) {
		t.Fatalf("lowercase Unicode query did not ignore case: %v, %v", matches, err)
	}
	matches, err = doc.Search(context.Background(), "Étage")
	if err != nil || !reflect.DeepEqual(matches, []int{3}) {
		t.Fatalf("uppercase query was not case-sensitive: %v, %v", matches, err)
	}
	matches, err = doc.Search(context.Background(), `\u001b]52`)
	if err != nil || len(matches) != 1 || matches[0] != 5 {
		t.Fatalf("sanitized control text not searchable: %v, %v", matches, err)
	}
	matches, err = doc.Search(context.Background(), "a   b")
	if err != nil || !reflect.DeepEqual(matches, []int{6}) {
		t.Fatalf("displayed tab expansion was not searchable: %v, %v", matches, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := doc.Search(ctx, "alpha"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled document search error = %v", err)
	}
}

func TestThemeHighlightIsSafeAndKeepsRowsBounded(t *testing.T) {
	line := "before \x1b]52;c;clipboard\a after"
	for _, color := range []bool{false, true} {
		theme := Theme{Color: color}
		rendered := theme.Render(line, 40, Changed, false)
		highlighted := theme.Highlight(rendered, `\u001b]52`, 40)
		if strings.Count(highlighted, "\n") != strings.Count(rendered, "\n") {
			t.Fatalf("highlight changed row count: %q", highlighted)
		}
		plain := regexp.MustCompile("\\x1b\\[(?:0|1|2|7|1;7|31|32|34|36|90|1;31|1;32|1;33|1;35|1;36|48;5;(?:22|52|237)|1;48;5;(?:28|124))m").ReplaceAllString(highlighted, "")
		if strings.ContainsAny(plain, "\x1b\a\r") {
			t.Fatalf("unsafe control escaped in highlight: %q", highlighted)
		}
		if color && !strings.Contains(plain, `\u001b]52`) || !color && !strings.Contains(plain, `⟦\u001b]52⟧`) {
			t.Fatalf("safe match highlight missing: %q", highlighted)
		}
		if color && !strings.Contains(highlighted, "\x1b[7m") {
			t.Fatal("color match lacks fixed reverse-video highlight")
		}
		if !color && (strings.Contains(highlighted, "\x1b[") || uniseg.StringWidth(highlighted) > 40) {
			t.Fatalf("monochrome match is styled or over-width: %q", highlighted)
		}
	}
}

package terminal

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// Columns cuts display text (the output of LineAt or DisplayText) to the
// columns [from, to) without splitting a grapheme or adding an ellipsis.
func Columns(display string, from, to int) string {
	if to <= from {
		return ""
	}
	var out strings.Builder
	column, state := 0, -1
	for len(display) > 0 && column < to {
		var cluster string
		var width int
		cluster, display, width, state = uniseg.FirstGraphemeClusterInString(display, state)
		if column >= from && column+width <= to {
			out.WriteString(cluster)
		}
		column += width
	}
	return out.String()
}

// maxSpanBytes bounds emphasis work per line; longer lines keep plain tints.
const maxSpanBytes = 4096

// ChangedSpan finds the differing middle of two display lines after their
// common prefix and suffix, widened to grapheme and word boundaries. It
// returns column ranges for each line and false when the lines share too
// little for emphasis to help (a rewrite rather than an edit). It does not
// allocate.
func ChangedSpan(a, b string) (aStart, aEnd, bStart, bEnd int, ok bool) {
	if len(a) > maxSpanBytes || len(b) > maxSpanBytes {
		return 0, 0, 0, 0, false
	}
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	prefix = min(boundaryAtOrBefore(a, prefix), boundaryAtOrBefore(b, prefix))
	// Never split a word: a partial word reads worse than a whole one.
	for prefix > 0 && (splitsWord(a, prefix) || splitsWord(b, prefix)) {
		_, size := utf8.DecodeLastRuneInString(a[:prefix])
		prefix = min(boundaryAtOrBefore(a, prefix-size), boundaryAtOrBefore(b, prefix-size))
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	for suffix > 0 {
		aCut, bCut := boundaryAtOrAfter(a, len(a)-suffix), boundaryAtOrAfter(b, len(b)-suffix)
		next := min(len(a)-aCut, len(b)-bCut)
		if next == suffix && !splitsWord(a, len(a)-suffix) && !splitsWord(b, len(b)-suffix) {
			break
		}
		if next == suffix {
			_, size := utf8.DecodeRuneInString(a[len(a)-suffix:])
			next = suffix - size
		}
		suffix = max(next, 0)
	}
	aWidth, bWidth := uniseg.StringWidth(a), uniseg.StringWidth(b)
	aStart, bStart = uniseg.StringWidth(a[:prefix]), uniseg.StringWidth(b[:prefix])
	aEnd, bEnd = aWidth-uniseg.StringWidth(a[len(a)-suffix:]), bWidth-uniseg.StringWidth(b[len(b)-suffix:])
	shared := aStart + aWidth - aEnd
	if aStart == aEnd && bStart == bEnd || shared < 3 || shared*4 < max(aWidth, bWidth) {
		return 0, 0, 0, 0, false
	}
	return aStart, aEnd, bStart, bEnd, true
}

// boundaryAtOrBefore and boundaryAtOrAfter move a byte offset to the nearest
// grapheme cluster boundary in text.
func boundaryAtOrBefore(text string, offset int) int {
	start, state := 0, -1
	for start < len(text) {
		var cluster string
		cluster, _, _, state = uniseg.FirstGraphemeClusterInString(text[start:], state)
		if start+len(cluster) > offset {
			return start
		}
		start += len(cluster)
	}
	return len(text)
}

func boundaryAtOrAfter(text string, offset int) int {
	start, state := 0, -1
	for start < len(text) && start < offset {
		var cluster string
		cluster, _, _, state = uniseg.FirstGraphemeClusterInString(text[start:], state)
		start += len(cluster)
	}
	return start
}

func splitsWord(text string, offset int) bool {
	if offset <= 0 || offset >= len(text) {
		return false
	}
	before, _ := utf8.DecodeLastRuneInString(text[:offset])
	after, _ := utf8.DecodeRuneInString(text[offset:])
	return isWordRune(before) && isWordRune(after)
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

package terminal

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// SmartCase reports whether a query contains an upper-case letter.
func SmartCase(query string) bool {
	return strings.IndexFunc(query, unicode.IsUpper) >= 0
}

// Contains searches displayed, sanitized text using smart-case substring rules.
func Contains(query, text string) bool {
	return hasMatch(text, query)
}

// VisibleText removes only AFTER's fixed theme SGR sequences and sanitizes all
// other controls before search compares rendered rows.
func VisibleText(raw string) string {
	visible, _ := splitStyled(raw)
	return visible
}

func hasMatch(text, query string) bool {
	query = DisplayText(query)
	if query == "" || text == "" {
		return false
	}
	if SmartCase(query) {
		return strings.Contains(text, query)
	}
	if isASCII(text) && isASCII(query) {
		if len(query) > len(text) {
			return false
		}
		for start := 0; start <= len(text)-len(query); start++ {
			matched := true
			for index := range query {
				left, right := text[start+index], query[index]
				if left >= 'A' && left <= 'Z' {
					left += 'a' - 'A'
				}
				if right >= 'A' && right <= 'Z' {
					right += 'a' - 'A'
				}
				if left != right {
					matched = false
					break
				}
			}
			if matched {
				return true
			}
		}
		return false
	}
	queryRunes := []rune(query)
	if len(queryRunes) == 0 {
		return false
	}
	starts := make([]int, 0, utf8.RuneCountInString(text)+1)
	for offset := range text {
		starts = append(starts, offset)
	}
	starts = append(starts, len(text))
	for runeIndex := 0; runeIndex+len(queryRunes) < len(starts); runeIndex++ {
		if strings.EqualFold(text[starts[runeIndex]:starts[runeIndex+len(queryRunes)]], query) {
			return true
		}
	}
	return false
}

// Search scans a document's bounded displayed-text prefix. Call it from a
// cancellable background job, never from a terminal event update.
func (d *Document) Search(ctx context.Context, query string) ([]int, error) {
	if d == nil || query == "" {
		return nil, nil
	}
	matches := make([]int, 0)
	for line := 0; line < d.Lines(); line++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if Contains(query, d.SearchableLine(line)) {
			matches = append(matches, line)
		}
	}
	return matches, nil
}

// SearchableLine returns the sanitized text prefix the text viewer can display.
// Long lines obey the same 4 KiB source-work bound as LineAt.
func (d *Document) SearchableLine(line int) string {
	if d == nil || line < 0 || line >= d.Lines() {
		return ""
	}
	raw := d.line(line)
	if len(raw) > MaxLineBytes {
		raw = raw[:MaxLineBytes]
	}
	return DisplayText(raw)
}

// MatchColumn returns the display column of the first match in sanitized text.
// Callers use it to reveal a hit beyond the initial horizontal viewport.
func MatchColumn(text, query string) int {
	matches := matchRanges(text, query)
	if len(matches) == 0 {
		return 0
	}
	return uniseg.StringWidth(text[:matches[0].start])
}

type matchRange struct{ start, end int }

func matchRanges(text, query string) []matchRange {
	query = DisplayText(query)
	if query == "" || text == "" {
		return nil
	}
	if SmartCase(query) {
		var matches []matchRange
		for offset := 0; offset <= len(text)-len(query); {
			index := strings.Index(text[offset:], query)
			if index < 0 {
				break
			}
			start := offset + index
			matches = append(matches, matchRange{start: start, end: start + len(query)})
			offset = start + len(query)
		}
		return matches
	}
	queryRunes := []rune(query)
	if len(queryRunes) == 0 {
		return nil
	}
	starts := make([]int, 0, utf8.RuneCountInString(text))
	for offset := range text {
		starts = append(starts, offset)
	}
	starts = append(starts, len(text))
	if len(starts)-1 < len(queryRunes) {
		return nil
	}
	matches := make([]matchRange, 0)
	for runeIndex := 0; runeIndex+len(queryRunes) < len(starts); {
		start, end := starts[runeIndex], starts[runeIndex+len(queryRunes)]
		if strings.EqualFold(text[start:end], query) {
			matches = append(matches, matchRange{start: start, end: end})
			runeIndex += len(queryRunes)
		} else {
			runeIndex++
		}
	}
	return matches
}

// Highlight marks every match without interpreting payload controls. Color
// mode uses the closed reverse-video theme style; monochrome mode uses visible
// delimiters and clips back to the original row width.
func (t Theme) Highlight(raw, query string, width int) string {
	visible, tokens := splitStyled(raw)
	matches := matchRanges(visible, query)
	if len(matches) == 0 {
		if t.Color {
			return raw
		}
		return Line(visible, width)
	}
	if !t.Color {
		var out strings.Builder
		position := 0
		for _, match := range matches {
			out.WriteString(visible[position:match.start])
			out.WriteRune('⟦')
			out.WriteString(visible[match.start:match.end])
			out.WriteRune('⟧')
			position = match.end
		}
		out.WriteString(visible[position:])
		return Line(out.String(), width)
	}

	starts, ends := make(map[int]bool, len(matches)), make(map[int]bool, len(matches))
	for _, match := range matches {
		starts[match.start], ends[match.end] = true, true
	}
	var out strings.Builder
	position, activeStyle, highlighted := 0, "", false
	for _, token := range tokens {
		if token.sgr != "" {
			out.WriteString(token.sgr)
			if token.sgr == "\x1b[0m" {
				activeStyle = ""
			} else {
				activeStyle = token.sgr
			}
			if highlighted {
				out.WriteString("\x1b[7m")
			}
			continue
		}
		for i := 0; i < len(token.text); i++ {
			if starts[position] && !highlighted {
				out.WriteString("\x1b[7m")
				highlighted = true
			}
			out.WriteByte(token.text[i])
			position++
			if ends[position] && highlighted {
				out.WriteString("\x1b[0m")
				if activeStyle != "" {
					out.WriteString(activeStyle)
				}
				highlighted = false
			}
		}
	}
	return out.String()
}

type styledToken struct{ text, sgr string }

func splitStyled(raw string) (string, []styledToken) {
	var visible strings.Builder
	tokens := make([]styledToken, 0, len(raw))
	for offset := 0; offset < len(raw); {
		if sgr, end := themeSGR(raw, offset); end > offset {
			tokens = append(tokens, styledToken{sgr: sgr})
			offset = end
			continue
		}
		r, size := utf8.DecodeRuneInString(raw[offset:])
		if size == 0 {
			break
		}
		if r == utf8.RuneError && size == 1 {
			r = utf8.RuneError
		}
		text := sanitized(string(r))
		visible.WriteString(text)
		tokens = append(tokens, styledToken{text: text})
		offset += size
	}
	return visible.String(), tokens
}

// DisplayText returns the complete bounded-width-safe text representation
// used for matching before a viewport clips it. Controls and invalid UTF-8 are
// made visible, tabs expand to four-column stops, and leading combining marks
// receive the same dotted-circle base as LineAt.
func DisplayText(raw string) string {
	safe := sanitized(raw)
	var out strings.Builder
	column, state := 0, -1
	for len(safe) > 0 {
		cluster, rest, cells, nextState := uniseg.FirstGraphemeClusterInString(safe, state)
		safe, state = rest, nextState
		if cluster == "\t" {
			cluster = strings.Repeat(" ", 4-column%4)
			cells = len(cluster)
		}
		if column == 0 && cells == 0 {
			cluster, cells = "◌"+cluster, 1
		}
		if cells > maxWidth {
			cluster, cells = "�", 1
		}
		out.WriteString(cluster)
		column += cells
	}
	return out.String()
}

func isASCII(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func themeSGR(raw string, offset int) (string, int) {
	if offset+3 > len(raw) || raw[offset] != '\x1b' || raw[offset+1] != '[' {
		return "", offset
	}
	end := strings.IndexByte(raw[offset+2:], 'm')
	if end < 0 || end > 8 {
		return "", offset
	}
	end += offset + 2
	candidate := raw[offset : end+1]
	switch candidate {
	case "\x1b[0m", "\x1b[32m", "\x1b[1;35m", "\x1b[1;33m", "\x1b[1;31m", "\x1b[34m", "\x1b[36m", "\x1b[2m", "\x1b[1m", "\x1b[7m":
		return candidate, end + 1
	default:
		return "", offset
	}
}

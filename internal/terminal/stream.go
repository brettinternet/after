package terminal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// DiffStyle returns the same closed style used by the captured Diff view.
func DiffStyle(line []byte) Style {
	switch {
	case bytes.HasPrefix(line, []byte("diff --git ")):
		return Strong
	case bytes.HasPrefix(line, []byte("@@ ")):
		return Decision
	case len(line) > 0 && line[0] == '+' && !bytes.HasPrefix(line, []byte("+++ ")):
		return Observed
	case len(line) > 0 && line[0] == '-' && !bytes.HasPrefix(line, []byte("--- ")):
		return Problem
	default:
		return Plain
	}
}

// WriteDiff streams a patch without retaining lines or output. TTY rendering
// expands tabs and applies only fixed theme SGR; pipes retain tab bytes and
// contain no styling. Input is bounded by the caller's stored artifact limit;
// every line is emitted, using fixed-size buffers rather than a line index.
func WriteDiff(w io.Writer, r io.Reader, theme Theme, tty bool) (changed bool, err error) {
	out := bufio.NewWriterSize(w, 64<<10)
	in := bufio.NewReaderSize(r, 64<<10)
	state := diffStream{out: out, tty: tty, color: tty && theme.Color, atLineStart: true}
	for {
		fragment, readErr := in.ReadSlice('\n')
		newline := len(fragment) > 0 && fragment[len(fragment)-1] == '\n'
		content := fragment
		if newline {
			content = content[:len(content)-1]
		}
		if state.atLineStart && (len(content) > 0 || newline) {
			state.beginLine(content)
		}
		if len(content) > 0 {
			state.changed = state.writeSafe(content, newline || !errors.Is(readErr, bufio.ErrBufferFull)) || state.changed
		}
		if newline {
			if err := state.endLine(true); err != nil {
				return state.changed, err
			}
			continue
		}
		if errors.Is(readErr, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			if !state.atLineStart {
				state.changed = state.writeSafe(nil, true) || state.changed
				if err := state.endLine(false); err != nil {
					return state.changed, err
				}
			}
			return state.changed, out.Flush()
		}
		if readErr != nil {
			return state.changed, readErr
		}
	}
}

type diffStream struct {
	out         *bufio.Writer
	tty         bool
	color       bool
	atLineStart bool
	styled      bool
	column      int
	pending     []byte
	changed     bool
}

func (s *diffStream) beginLine(prefix []byte) {
	s.atLineStart = false
	code := styleCode(DiffStyle(prefix))
	if s.color && code != "" {
		_, _ = io.WriteString(s.out, "\x1b["+code+"m")
		s.styled = true
	}
}

func (s *diffStream) endLine(newline bool) error {
	if s.styled {
		if _, err := io.WriteString(s.out, "\x1b[0m"); err != nil {
			return err
		}
	}
	s.styled = false
	if newline {
		if err := s.out.WriteByte('\n'); err != nil {
			return err
		}
	}
	s.atLineStart = true
	s.column = 0
	s.pending = s.pending[:0]
	return nil
}

// writeSafe retains only an incomplete UTF-8 suffix (at most three bytes) from
// one read fragment to the next. Expanded control text is emitted directly.
func (s *diffStream) writeSafe(raw []byte, final bool) bool {
	changed := false
	if len(s.pending) != 0 {
		var prefix [utf8.UTFMax]byte
		n := copy(prefix[:], s.pending)
		oldPending := n
		s.pending = s.pending[:0]
		for n < len(prefix) && n-oldPending < len(raw) && !utf8.FullRune(prefix[:n]) {
			prefix[n] = raw[n-oldPending]
			n++
		}
		if !utf8.FullRune(prefix[:n]) && !final {
			s.pending = append(s.pending, prefix[:n]...)
			return false
		}
		if !utf8.FullRune(prefix[:n]) {
			for range prefix[:n] {
				if s.writeRune(utf8.RuneError, true, nil) {
					changed = true
				}
			}
			raw = raw[min(n-oldPending, len(raw)):]
		} else {
			r, size := utf8.DecodeRune(prefix[:n])
			if s.writeRune(r, size == 1 && r == utf8.RuneError, prefix[:size]) {
				changed = true
			}
			consumed := max(size-oldPending, 0)
			raw = raw[min(consumed, len(raw)):]
		}
	}
	for len(raw) > 0 {
		if raw[0] >= 0x20 && raw[0] < 0x7f {
			end := 1
			for end < len(raw) && raw[end] >= 0x20 && raw[end] < 0x7f {
				end++
			}
			_, _ = s.out.Write(raw[:end])
			s.column += end
			raw = raw[end:]
			continue
		}
		if !utf8.FullRune(raw) && !final {
			s.pending = append(s.pending, raw...)
			break
		}
		r, size := utf8.DecodeRune(raw)
		if s.writeRune(r, size == 1 && r == utf8.RuneError, raw[:size]) {
			changed = true
		}
		raw = raw[size:]
	}
	return changed
}

func (s *diffStream) writeRune(r rune, invalid bool, original []byte) bool {
	if invalid {
		_, _ = io.WriteString(s.out, "�")
		s.column++
		return true
	}
	if r == '\t' {
		if s.tty {
			spaces := 4 - s.column%4
			for range spaces {
				_ = s.out.WriteByte(' ')
			}
			s.column += spaces
		} else {
			_, _ = s.out.Write(original)
		}
		return false
	}
	if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
		escaped := fmt.Sprintf("\\u%04x", r)
		_, _ = io.WriteString(s.out, escaped)
		s.column += len(escaped)
		return true
	}
	_, _ = s.out.Write(original)
	if r < utf8.RuneSelf {
		s.column++
	} else {
		s.column += uniseg.StringWidth(string(r))
	}
	return false
}

func styleCode(style Style) string {
	switch style {
	case Observed:
		return "32"
	case Changed:
		return "1;35"
	case Attention:
		return "1;33"
	case Problem:
		return "1;31"
	case Reported:
		return "34"
	case Decision:
		return "36"
	case Muted:
		return "2"
	case Strong:
		return "1"
	case Reverse:
		return "7"
	case Brand:
		return "1;7"
	case Rule:
		return "90"
	case Accent:
		return "1;36"
	case Added:
		return "32"
	case Removed:
		return "31"
	case AddedEmphasis:
		return "1;32"
	case RemovedEmphasis:
		return "1;31"
	case Hunk:
		return "36"
	case AddedText:
		return "32"
	case RemovedText:
		return "31"
	default:
		return ""
	}
}

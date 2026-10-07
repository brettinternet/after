// Package terminal provides the bounded text/view foundation for the local TUI.
// Repository content is data, never terminal markup or evidence labels.
package terminal

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

const (
	MaxTextBytes = 16 << 20
	MaxLines     = 250000
	MaxLineBytes = 4096
	maxWidth     = 240
	maxHeight    = 100
)

// Document owns exact captured bytes and a bounded index of their display text.
// A line-limit overflow keeps the raw bytes and first MaxLines rows available;
// callers can always render the exact bytes through HexLine without an index.
// Construct documents during background preparation, not Update or View.
type Document struct {
	text      string
	raw       string
	starts    []int
	textEnd   int
	maxWidth  int
	lineLimit bool
}

// NewDocument indexes raw bytes verbatim for display and exact hex access.
func NewDocument(raw []byte) (*Document, error) {
	return NewDocumentView(raw, raw)
}

// NewDocumentView indexes display bytes while retaining raw as the exact-byte
// source for hex view. It is used only for explicitly typed, indented JSON; all
// other content passes the same bytes for display and exact access.
func NewDocumentView(display, raw []byte) (*Document, error) {
	if len(raw) > MaxTextBytes || len(display) > MaxTextBytes {
		return nil, fmt.Errorf("terminal document exceeds %d MiB", MaxTextBytes>>20)
	}
	text := string(display)
	rawText := text
	if !bytes.Equal(display, raw) {
		rawText = string(raw)
	}
	d := &Document{text: text, raw: rawText, textEnd: len(display)}
	if len(display) == 0 {
		return d, nil
	}
	d.starts = []int{0}
	for i, b := range display {
		if b != '\n' || i+1 == len(display) {
			continue
		}
		if len(d.starts) == MaxLines {
			d.lineLimit = true
			d.textEnd = i
			break
		}
		d.starts = append(d.starts, i+1)
	}
	for i := range d.starts {
		d.maxWidth = max(d.maxWidth, displayWidth(d.line(i)))
	}
	return d, nil
}

func (d *Document) Lines() int {
	if d == nil {
		return 0
	}
	return len(d.starts)
}

func (d *Document) RawLength() int {
	if d == nil {
		return 0
	}
	return len(d.raw)
}

// Limited reports that the document has more than MaxLines and text view shows
// only the indexed prefix. HexRows/HexLine still reach the entire raw artifact.
func (d *Document) Limited() bool { return d != nil && d.lineLimit }

// MaxColumns is the largest safe display width through the first MaxLineBytes
// of any indexed line. Horizontal pan is bounded by this precomputed value.
func (d *Document) MaxColumns() int {
	if d == nil {
		return 0
	}
	return d.maxWidth
}

func (d *Document) line(n int) string {
	if n < 0 || n >= d.Lines() {
		return ""
	}
	end := d.textEnd
	if n+1 < len(d.starts) {
		end = d.starts[n+1]
	}
	return strings.TrimSuffix(d.text[d.starts[n]:end], "\n")
}

// LongLine reports whether the indexed display line extends past the bounded
// 4 KiB text prefix. The exact suffix remains available in hex view.
func (d *Document) LongLine(n int) bool { return len(d.line(n)) > MaxLineBytes }

// LineAt sanitizes one captured line and pans by display columns without ever
// splitting a grapheme cluster. Work is capped at MaxLineBytes of source text.
func (d *Document) LineAt(n, left, width int) string {
	if d == nil || n < 0 || n >= d.Lines() {
		return ""
	}
	return LineAt(d.line(n), left, width)
}

// AutoHex selects the readable hex representation when a NUL occurs in the
// first 8,000 exact stored bytes.
func (d *Document) AutoHex() bool {
	if d == nil {
		return false
	}
	return strings.IndexByte(d.raw[:min(len(d.raw), 8000)], 0) >= 0
}

func (d *Document) RawBytes() []byte {
	if d == nil {
		return nil
	}
	return []byte(d.raw)
}

func (d *Document) HexRows() int {
	if d == nil {
		return 0
	}
	return (len(d.raw) + 15) / 16
}

// HexLine formats one offset-addressed row directly from raw bytes; no line
// index is required, including when the text index reached its line limit.
func (d *Document) HexLine(row int) string {
	if d == nil || row < 0 || row >= d.HexRows() {
		return ""
	}
	start := row * 16
	end := min(start+16, len(d.raw))
	var out strings.Builder
	fmt.Fprintf(&out, "%08x  ", start)
	for i := start; i < start+16; i++ {
		if i < end {
			fmt.Fprintf(&out, "%02x ", d.raw[i])
		} else {
			out.WriteString("   ")
		}
	}
	out.WriteString(" |")
	for i := start; i < end; i++ {
		b := d.raw[i]
		if b >= 0x20 && b <= 0x7e {
			out.WriteByte(b)
		} else {
			out.WriteByte('.')
		}
	}
	out.WriteByte('|')
	return out.String()
}

// Sanitize escapes terminal controls and format characters without clipping.
// It is used for non-terminal output where clipping would lose information.
func Sanitize(raw string) string { return sanitized(raw) }

// Line is the untrusted-text boundary at the left edge of a line.
func Line(raw string, width int) string { return LineAt(raw, 0, width) }

// LineAt sanitizes, clips and horizontally pans untrusted text. It never emits
// controls, including ESC, C1 CSI/OSC, CR, bidi overrides or embedded newlines.
// Tabs use absolute four-column stops. Invalid UTF-8 becomes U+FFFD, and a
// leading combining cluster gets a dotted-circle base. Both byte and cell work
// are bounded, and clipping never splits a display grapheme.
func LineAt(raw string, left, width int) string {
	width = min(max(width, 0), maxWidth)
	left = max(left, 0)
	if width == 0 {
		return ""
	}
	clippedBytes := len(raw) > MaxLineBytes
	if clippedBytes {
		raw = raw[:MaxLineBytes]
	}
	safe := sanitized(raw)
	var out strings.Builder
	col, used, state := 0, 0, -1
	clipped := false
	for len(safe) > 0 {
		var cluster string
		var cells int
		cluster, safe, cells, state = uniseg.FirstGraphemeClusterInString(safe, state)
		isTab := cluster == "\t"
		if isTab {
			cells = 4 - col%4
		}
		if col == 0 && cells == 0 {
			cluster = "◌" + cluster
			cells = 1
		}
		if col < left {
			if isTab && col+cells > left {
				partial := min(col+cells-left, width-used)
				if partial > 0 {
					out.WriteString(strings.Repeat(" ", partial))
					used += partial
				}
				if partial < col+cells-left {
					clipped = true
					break
				}
			}
			col += cells
			continue
		}
		remaining := len(safe) > 0 || clippedBytes
		limit := width
		if remaining {
			limit = max(width-1, 0)
		}
		if used+cells > limit {
			clipped = true
			break
		}
		if isTab {
			out.WriteString(strings.Repeat(" ", cells))
		} else {
			out.WriteString(cluster)
		}
		used += cells
		col += cells
	}
	if len(safe) > 0 || clippedBytes {
		clipped = true
	}
	if clipped {
		if used < width {
			out.WriteRune('…')
		} else if used > 0 {
			// The last cluster is already committed. Keep a safe width rather than
			// cutting it to squeeze in a marker; the caller also exposes hex mode.
		}
	}
	return out.String()
}

func sanitized(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		switch {
		case r == '\t':
			b.WriteRune(r)
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029':
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func displayWidth(raw string) int {
	if len(raw) > MaxLineBytes {
		raw = raw[:MaxLineBytes]
	}
	ascii := true
	for i := 0; i < len(raw); i++ {
		if raw[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		col := 0
		for i := 0; i < len(raw); i++ {
			switch b := raw[i]; {
			case b == '\t':
				col += 4 - col%4
			case b < 0x20 || b == 0x7f:
				col += 6 // visible \\uNNNN escape
			default:
				col++
			}
		}
		return col
	}
	safe := sanitized(raw)
	col, state := 0, -1
	for len(safe) > 0 {
		var cluster string
		var cells int
		cluster, safe, cells, state = uniseg.FirstGraphemeClusterInString(safe, state)
		if cluster == "\t" {
			cells = 4 - col%4
		}
		if col == 0 && cells == 0 {
			cells = 1
		}
		col += cells
	}
	return col
}

// Package terminal provides the bounded text/view foundation for the local TUI.
// Repository content is data, never terminal markup or evidence labels.
package terminal

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

const (
	MaxTextBytes = 16 << 20
	MaxLines     = 250000
	maxLineBytes = 4096
	maxWidth     = 240
	maxHeight    = 100
)

// Document owns a copy of captured bytes and indexes lines once, off the event
// loop. Oversize inputs fail explicitly; the original raw artifact is unchanged.
// A Document is immutable and can be shared by successive model states.
type Document struct {
	text   string
	starts []int
}

func NewDocument(raw []byte) (*Document, error) {
	if len(raw) > MaxTextBytes {
		return nil, errors.New("terminal text exceeds 16 MiB; use raw artifact export")
	}
	d := &Document{text: string(raw)}
	if len(raw) == 0 {
		return d, nil
	}
	d.starts = []int{0}
	for i, b := range raw {
		if b == '\n' && i+1 < len(raw) {
			if len(d.starts) == MaxLines {
				return nil, errors.New("terminal text exceeds 250000 lines; use raw artifact export")
			}
			d.starts = append(d.starts, i+1)
		}
	}
	return d, nil
}

func (d *Document) Lines() int {
	if d == nil {
		return 0
	}
	return len(d.starts)
}

func (d *Document) line(n int) string {
	if n < 0 || n >= d.Lines() {
		return ""
	}
	end := len(d.text)
	if n+1 < len(d.starts) {
		end = d.starts[n+1]
	}
	return strings.TrimSuffix(d.text[d.starts[n]:end], "\n")
}

// Line is the only untrusted-text rendering boundary. It never emits controls,
// including ESC, C1 CSI/OSC, CR, bidi overrides or embedded newlines. Tabs become
// four-column stops; other controls/format characters become visible escapes.
// Limit work even on a single enormous line or combining-character cluster.
// Clipping is explicit and never changes the underlying captured artifact.
func Line(raw string, width int) string {
	width = min(max(width, 0), maxWidth)
	if width == 0 {
		return ""
	}
	clipped := len(raw) > maxLineBytes
	if clipped {
		raw = raw[:maxLineBytes]
	}
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r == '\t':
			// Tabs expand below, after grapheme widths are known.
			b.WriteRune(r)
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029':
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			b.WriteRune(r)
		}
	}
	safe := b.String()
	var out strings.Builder
	col, state := 0, -1
	for len(safe) > 0 {
		var cluster string
		var cells int
		cluster, safe, cells, state = uniseg.FirstGraphemeClusterInString(safe, state)
		if cluster == "\t" {
			cells = 4 - col%4
			cluster = strings.Repeat(" ", cells)
		}
		// A leading combining cluster must not attach to the trusted row prefix.
		if col == 0 && cells == 0 {
			cluster = "◌" + cluster
			cells = 1
		}
		if col+cells > width || ((len(safe) > 0 || clipped) && col+cells > width-1) {
			clipped = true
			break
		}
		out.WriteString(cluster)
		col += cells
	}
	if clipped {
		out.WriteRune('…')
	}
	return out.String()
}

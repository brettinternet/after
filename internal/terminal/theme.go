package terminal

import (
	"os"
	"strings"

	"github.com/rivo/uniseg"
)

// Style is a closed vocabulary; callers cannot supply terminal markup.
type Style uint8

const (
	Plain Style = iota
	Observed
	Changed
	Attention
	Problem
	Reported
	Decision
	Muted
	Strong
	Reverse
	// Chrome and Diff styles. They never carry evidence meaning.
	Brand
	Rule
	Accent
	Added
	Removed
	AddedEmphasis
	RemovedEmphasis
	Hunk
	Selected
	AddedText
	RemovedText
	lastStyle
)

// Theme enables fixed SGR. Rich adds fixed xterm-256 backgrounds (Diff line
// tints and the selection bar); without it the same styles use 16 colors.
type Theme struct{ Color, Rich bool }

func DefaultTheme() Theme {
	color := os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	colorterm := os.Getenv("COLORTERM")
	rich := color && (strings.Contains(os.Getenv("TERM"), "256color") || colorterm == "truecolor" || colorterm == "24bit")
	return Theme{Color: color, Rich: rich}
}

// Render is the only styling boundary. Even trusted-looking strings are
// sanitized and clipped before fixed SGR is applied. A cell may be padded for
// column layout; padding is measured on safe text, never on markup.
func (t Theme) Render(raw string, width int, style Style, pad bool) string {
	width = min(max(width, 0), maxWidth)
	safe := Line(raw, width)
	if pad {
		safe += strings.Repeat(" ", max(0, width-uniseg.StringWidth(safe)))
	}
	if !t.Color || safe == "" {
		return safe
	}
	sgr := t.code(style)
	if sgr == "" {
		return safe
	}
	return "\x1b[" + sgr + "m" + safe + "\x1b[0m"
}

// Bar pads an already rendered row to width and, in rich color mode, keeps the
// selection background behind every styled span. Only theme SGR is emitted:
// each trusted reset is followed by the closed background code again.
func (t Theme) Bar(rendered string, width int) string {
	visible := uniseg.StringWidth(VisibleText(rendered))
	padded := rendered + strings.Repeat(" ", max(width-visible, 0))
	if !t.Color || !t.Rich {
		return padded
	}
	background := "\x1b[" + t.code(Selected) + "m"
	return background + strings.ReplaceAll(padded, "\x1b[0m", "\x1b[0m"+background) + "\x1b[0m"
}

func (t Theme) code(style Style) string {
	if t.Rich {
		switch style {
		case Added:
			return "48;5;22"
		case Removed:
			return "48;5;52"
		case AddedEmphasis:
			return "1;48;5;28"
		case RemovedEmphasis:
			return "1;48;5;124"
		case Selected:
			return "48;5;237"
		}
	}
	return styleCode(style)
}

// themeCodes is the complete set of SGR sequences Render may emit.
var themeCodes = func() map[string]bool {
	codes := map[string]bool{"\x1b[0m": true}
	for _, rich := range []bool{false, true} {
		theme := Theme{Color: true, Rich: rich}
		for style := Plain; style < lastStyle; style++ {
			if code := theme.code(style); code != "" {
				codes["\x1b["+code+"m"] = true
			}
		}
	}
	return codes
}()

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
)

type Theme struct{ Color bool }

func DefaultTheme() Theme {
	return Theme{Color: os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"}
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
	var sgr string
	switch style {
	case Observed:
		sgr = "32"
	case Changed:
		sgr = "1;35"
	case Attention:
		sgr = "1;33"
	case Problem:
		sgr = "1;31"
	case Reported:
		sgr = "34"
	case Decision:
		sgr = "36"
	case Muted:
		sgr = "2"
	case Strong:
		sgr = "1"
	case Reverse:
		sgr = "7"
	default:
		return safe
	}
	return "\x1b[" + sgr + "m" + safe + "\x1b[0m"
}

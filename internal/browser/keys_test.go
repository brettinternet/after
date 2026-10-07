package browser

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
)

func TestEveryDispatchedKeyHasOneKeyMapEntry(t *testing.T) {
	dispatched := []string{
		"q", "ctrl+c", "x", "c", "i", "?", "esc", "d", "enter", "tab", "shift+tab",
		"b", "right", "l", "left", "h", "j", "down", "k", "up", "pgdown", "pgup",
		"home", "end", "g", "G", "s", "p", "u", "r", "y", "n", "a", "1", "2", "3", "]", "[", "}", "{",
	}
	seen := make(map[string]bool)
	for _, key := range dispatched {
		if _, ok := keyBindingFor(key); !ok {
			t.Errorf("dispatched key %q has no key-map entry", key)
		}
		if seen[key] {
			t.Errorf("dispatched key %q appears more than once", key)
		}
		seen[key] = true
	}
	for _, binding := range keyMap {
		for _, key := range binding.keys {
			if seen[key] {
				continue
			}
			seen[key] = true
			if _, ok := keyBindingFor(key); !ok {
				t.Errorf("key-map alias %q cannot be dispatched", key)
			}
		}
	}
}

func TestContextualHintsAndGroupedHelp(t *testing.T) {
	sel := viewSelection(t)
	m := New(t.Context(), sel, Jobs{})
	defer m.Close()
	m.theme.Color = false
	drain(m, m.Init())
	m.width, m.height = 120, 100
	hints := m.keyHints()
	for _, want := range []string{"q quit", "? help", "1 Overview", "2 Changes", "3 Diff", "Enter open"} {
		if !strings.Contains(hints, want) {
			t.Fatalf("missing enabled hint %q in %q", want, hints)
		}
	}
	for _, unavailable := range []string{"u use capture", "p pin", "a accept pin", "y approve once", "n deny"} {
		if strings.Contains(hints, unavailable) {
			t.Fatalf("disabled action leaked into hints: %q", hints)
		}
	}
	step(m, key("?"))
	help := m.View()
	for _, want := range []string{"Navigation", "Views", "Review", "Consent", "u  Use the pending capture", "[unavailable: no new capture is waiting]", "a  Accept a pin, not a snapshot", "pin acceptance is not available"} {
		if !strings.Contains(help, want) {
			t.Fatalf("grouped help missing %q:\n%s", want, help)
		}
	}
	if strings.Index(help, "Views") > strings.Index(help, "Review") || strings.Index(help, "Review") > strings.Index(help, "Consent") {
		t.Fatalf("help groups are not ordered:\n%s", help)
	}
}

func TestFrameTabsHeaderAndSmallTerminalRules(t *testing.T) {
	s, sel := setup(t, false)
	s.Close()
	m := New(t.Context(), sel, Jobs{})
	defer m.Close()
	m.theme.Color = false
	drain(m, m.Init())
	clock := time.Date(2025, 1, 2, 20, 0, 0, 0, time.UTC)
	m.setClock(func() time.Time { return clock }, time.UTC)
	m.pending = &evidence.SnapshotPair{Base: sel.Pair.Base, Candidate: evidence.Digest("sha256:" + strings.Repeat("a", 64))}
	m.running, m.runStarted = true, clock.Add(-72*time.Second)
	m.selected.Project = filepath.Join(t.TempDir(), "unsafe\x1b]52;c;name\a")
	m.width, m.height = 120, 40
	header, headerLines := m.frameHeader()
	header += "\n" + strings.Join(headerLines, "\n")
	for _, want := range []string{
		"AFTER · unsafe",
		"base " + shortID(sel.Pair.Base) + " (commit ",
		"candidate " + shortID(sel.Pair.Candidate) + " (working tree)",
		"new capture aaaaaaaa · u",
		"running 1:12 · x",
	} {
		if !strings.Contains(header, want) {
			t.Fatalf("header missing %q: %q", want, header)
		}
	}
	if strings.Contains(m.View(), "\x1b]52;") || strings.Contains(m.View(), "\a") {
		t.Fatal("project name escaped the terminal boundary")
	}
	for _, source := range []struct {
		snapshot evidence.Snapshot
		want     string
	}{
		{evidence.Snapshot{Source: evidence.Index}, "staged"},
		{evidence.Snapshot{Source: evidence.MergeBase, MergeBase: strings.Repeat("b", 40)}, "merge base bbbbbbb"},
		{evidence.Snapshot{Source: evidence.WorkingTree}, "working tree"},
	} {
		if got := snapshotSource(source.snapshot); got != source.want {
			t.Fatalf("snapshot source = %q, want %q", got, source.want)
		}
	}

	m.running = false
	m.width, m.height = 40, 12
	if !strings.Contains(m.View(), "new capture aaaaaaaa · u") {
		t.Fatalf("compact frame hid the pending capture indicator: %q", m.View())
	}
	m.pending = nil
	m.selected.Project = sel.Project
	view := m.View()
	lines := strings.Split(view, "\n")
	if !strings.Contains(lines[0], shortID(sel.Pair.Base)+" → "+shortID(sel.Pair.Candidate)) || strings.Contains(lines[0], "project") || strings.Contains(lines[0], "working tree") {
		t.Fatalf("compact header retained project/source: %q", lines[0])
	}
	if !strings.Contains(lines[1], "[1 Ov]") {
		t.Fatalf("small terminal tab did not use compact active label: %q", lines[1])
	}
	m.height = 11
	lines = strings.Split(m.View(), "\n")
	if strings.Contains(lines[1], "[1 Ov]") {
		t.Fatalf("tab bar was not hidden below 12 rows: %q", lines[1])
	}
	m.height = 6
	lines = strings.Split(m.View(), "\n")
	if lines[len(lines)-1] != "? help · q quit" {
		t.Fatalf("short-terminal hints: %q", lines[len(lines)-1])
	}
	for _, line := range lines {
		if strings.Contains(line, m.status) && m.status != "" {
			t.Fatalf("status line visible below 7 rows: %q", line)
		}
	}
	m.screen, m.width, m.height = "help", 40, 2
	if got := len(strings.Split(m.View(), "\n")); got > 2 {
		t.Fatalf("frame exceeded 2-row terminal: %q", m.View())
	}
	m.width, m.height = 1, 1
	if uniseg.StringWidth(m.View()) != 1 {
		t.Fatalf("1x1 frame did not clip to one cell: %q", m.View())
	}
	_, quit := m.Update(key("q"))
	if quit == nil || m.ctx.Err() == nil {
		t.Fatal("quit did not work at 1x1")
	}
}

func TestNumberAndTabNavigationAndColorlessActiveTab(t *testing.T) {
	m := New(t.Context(), viewSelection(t), Jobs{})
	defer m.Close()
	m.theme.Color = false
	drain(m, m.Init())
	m.width, m.height = 120, 40
	step(m, key("2"))
	if m.screen != "inventory" {
		t.Fatal("2 did not switch to Changes")
	}
	step(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.screen != "patch" || m.doc == nil {
		t.Fatal("Tab did not switch to Diff and load its captured patch")
	}
	step(m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.screen != "inventory" {
		t.Fatal("Shift+Tab did not return to Changes")
	}
	step(m, key("1"))
	if m.screen != "examples" || !strings.Contains(m.View(), "[1 Overview]") {
		t.Fatal("number navigation or no-color active tab marker failed", m.View())
	}
	m.theme.Color = true
	view := m.View()
	plain := regexp.MustCompile("\\x1b\\[(?:0|32|1;35|1;33|1;31|34|36|2|1|7)m").ReplaceAllString(view, "")
	if !strings.Contains(view, "\x1b[7m[1 Overview]") || strings.Contains(plain, "\x1b[") {
		t.Fatalf("color active-tab reverse style missing or unexpected escape: %q", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "\x1b]52;") || strings.Contains(line, "\a") {
			t.Fatalf("unsafe frame line %q", line)
		}
	}
}

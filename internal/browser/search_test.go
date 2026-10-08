package browser

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
)

func submitSearch(m *Model, query string) {
	step(m, key("/"))
	step(m, key(query))
	step(m, tea.KeyMsg{Type: tea.KeyEnter})
}

func TestListSearchSmartCaseWrapHighlightAndPasteSafety(t *testing.T) {
	selection := viewSelection(t)
	m := New(t.Context(), selection, Jobs{})
	defer m.Close()
	m.theme.Color = false
	m.width, m.height = 80, 24
	drain(m, m.Init())

	submitSearch(m, "retry")
	if len(m.searchMatches) != 2 || m.searchCurrent != 0 || !strings.Contains(m.statusLine(), "match 1 of 2") {
		t.Fatalf("initial list search result: matches=%+v current=%d status=%q", m.searchMatches, m.searchCurrent, m.statusLine())
	}
	if !strings.Contains(m.View(), "⟦retry⟧") {
		t.Fatalf("list match was not highlighted in monochrome:\n%s", m.View())
	}
	step(m, key("n"))
	if m.searchCurrent != 1 || !strings.Contains(m.statusLine(), "match 2 of 2") {
		t.Fatalf("next match did not advance: %d %s", m.searchCurrent, m.statusLine())
	}
	step(m, key("n"))
	if m.searchCurrent != 0 || !strings.Contains(m.statusLine(), "match 1 of 2") {
		t.Fatalf("forward search did not wrap: %d %s", m.searchCurrent, m.statusLine())
	}
	step(m, key("N"))
	if m.searchCurrent != 1 || !strings.Contains(m.statusLine(), "match 2 of 2") {
		t.Fatalf("reverse search did not wrap: %d %s", m.searchCurrent, m.statusLine())
	}
	step(m, key("esc"))
	if m.searchQuery != "" || m.searchEditing || m.screen != "examples" {
		t.Fatal("Escape did not clear the list search without changing views")
	}

	submitSearch(m, "Retry")
	if len(m.searchMatches) != 0 || m.statusLine() != "no matches" {
		t.Fatalf("uppercase query did not use smart case: %v %q", m.searchMatches, m.statusLine())
	}
	step(m, key("esc"))

	step(m, key("/"))
	paste := "retry\nq\x1b]52;c;blocked\a"
	step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(paste), Paste: true})
	if m.screen != "examples" || !strings.Contains(m.searchQuery, `\u000a`) || !strings.Contains(m.searchQuery, `\u001b]52`) {
		t.Fatalf("pasted controls were not inert sanitized query data: %q", m.searchQuery)
	}
	if strings.ContainsAny(m.View(), "\x1b\a\r") {
		t.Fatal("hostile search paste escaped the terminal boundary")
	}
	step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.statusLine() != "no matches" || m.screen != "examples" {
		t.Fatalf("paste triggered an action or unsafe search: %s %s", m.screen, m.statusLine())
	}
	step(m, key("esc"))

	step(m, key("/"))
	step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("x", 2048)), Paste: true})
	if len(m.searchQuery) != maxSearchQueryBytes {
		t.Fatalf("pasted query length %d, want bound %d", len(m.searchQuery), maxSearchQueryBytes)
	}
	step(m, key("esc"))
}

func TestChangesActivityAndDocumentSearch(t *testing.T) {
	_, selection := changesFixture(t)
	data, err := Load(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	m := New(t.Context(), selection, Jobs{})
	defer m.Close()
	m.theme.Color = false
	m.width, m.height, m.data = 80, 24, data
	m.screen = "inventory"
	for index, entry := range data.Inventory {
		if entry.Name == "source.go" {
			m.inventory = index
			m.section = 1
			break
		}
	}
	m.returnTo = "inventory"
	m.screen = "inspector"
	drain(m, m.loadDocument())
	submitSearch(m, "func Value")
	if len(m.searchMatches) != 1 || !strings.Contains(m.View(), "⟦func Value⟧") {
		t.Fatalf("source document search failed: %+v\n%s", m.searchMatches, m.View())
	}
	step(m, key("esc"))

	m.clearSearch()
	m.screen = "inventory"
	m.recordActivity("capture finished", "needle in activity summary", nil, "")
	m.screen = "activity"
	submitSearch(m, "needle")
	if len(m.searchMatches) != 1 || !strings.Contains(m.View(), "⟦needle⟧") {
		t.Fatalf("Activity search failed: %+v\n%s", m.searchMatches, m.View())
	}
	step(m, key("esc"))

	m.screen = "patch"
	m.section = 0
	m.data = data
	drain(m, m.loadDocument())
	submitSearch(m, "Value")
	if len(m.searchMatches) == 0 || !strings.Contains(m.View(), "⟦Value⟧") {
		t.Fatalf("Diff search failed: %+v\n%s", m.searchMatches, m.View())
	}
	m.theme.Color = true
	if !strings.Contains(m.View(), "\x1b[7mValue\x1b[0m") {
		t.Fatalf("color Diff match was not reverse-highlighted: %q", m.View())
	}
	m.theme.Color = false
	first := m.searchCurrent
	step(m, key("n"))
	want := (first + 1) % len(m.searchMatches)
	if m.searchCurrent != want {
		t.Fatalf("n did not wrap through Diff matches: current=%d want=%d", m.searchCurrent, want)
	}
}

func TestDocumentSearchRevealsOffscreenMatch(t *testing.T) {
	for _, screen := range []string{"inspector", "patch"} {
		for _, color := range []bool{false, true} {
			m := New(t.Context(), Selection{}, Jobs{})
			t.Cleanup(m.Close)
			m.theme.Color = color
			m.width, m.height = 80, 24
			// Wide characters and tabs ensure offsets are display columns, not bytes.
			raw := []byte(strings.Repeat("界\t", 40) + "Needle\n")
			doc, err := terminal.NewDocument(raw)
			if err != nil {
				t.Fatal(err)
			}
			m.data = &Data{Entries: []Entry{{Sections: []Section{{Name: "Source", Content: raw}}}}, Diff: &DiffView{Rows: []DiffRow{{RawLine: 0}}}}
			m.screen, m.returnTo, m.doc = screen, "examples", doc
			submitSearch(m, "needle")
			if len(m.searchMatches) != 1 || m.left != 160 {
				t.Fatalf("%s color=%t: match was not revealed: %+v pan=%d", screen, color, m.searchMatches, m.left)
			}
			want := "⟦Needle⟧"
			if color {
				want = "\x1b[7mNeedle\x1b[0m"
			}
			if !strings.Contains(m.View(), want) {
				t.Fatalf("%s color=%t: offscreen match not highlighted: %q", screen, color, m.View())
			}
			step(m, key("n"))
			step(m, key("N"))
			if m.left != 160 {
				t.Fatal("wraparound lost the match column")
			}
		}
	}
}

func TestSearchCancellationOnQueryAndViewChanges(t *testing.T) {
	m := New(t.Context(), viewSelection(t), Jobs{})
	defer m.Close()
	drain(m, m.Init())

	ctx, cancel := context.WithCancel(m.ctx)
	m.searchCancel = cancel
	m.searchPending = true
	m.searchQuery = "old"
	m.openSearch()
	if !errors.Is(ctx.Err(), context.Canceled) || m.searchPending || len(m.searchMatches) != 0 {
		t.Fatal("changing the query did not cancel and invalidate the prior search")
	}

	ctx, cancel = context.WithCancel(m.ctx)
	m.searchCancel = cancel
	m.searchPending = true
	m.searchQuery = "old"
	m.switchView(1)
	if !errors.Is(ctx.Err(), context.Canceled) || m.searchQuery != "" || m.searchPending {
		t.Fatal("changing the view did not cancel and clear search")
	}
}

func TestSearch100000LineEventBudgetAndStaleCancellation(t *testing.T) {
	const lines = 100000
	line := strings.Repeat("a", 128) + "\n"
	raw := []byte(strings.Repeat(line, lines))
	doc, err := terminal.NewDocument(raw)
	if err != nil || doc.Lines() != lines {
		t.Fatalf("large document setup lines=%d err=%v", doc.Lines(), err)
	}
	selection := viewRawSelection(t)
	m := New(t.Context(), selection, Jobs{})
	defer m.Close()
	m.theme.Color = false
	m.width, m.height = 120, 40
	m.data = &Data{Entries: []Entry{{Sections: []Section{{Name: "Source", Content: []byte("large")}}}}}
	m.screen, m.returnTo, m.doc = "inspector", "examples", doc
	m.searchEditing = true
	m.searchQuery = strings.Repeat("a", 127) + "z"
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil || !m.searchPending {
		t.Fatal("document search did not start as a background job")
	}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	m.Update(key("j"))
	_ = m.View()
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	if elapsed > 100*time.Millisecond || allocated > 256<<10 {
		t.Fatalf("100000-line search blocked an input event: %s / %d bytes", elapsed, allocated)
	}

	oldRequest := m.searchRequest
	m.switchView(1)
	if m.searchRequest == oldRequest || m.searchPending || m.searchQuery != "" {
		t.Fatal("view change failed to invalidate the active search")
	}
	message := command()
	ready := message.(searchReady)
	if ready.request == m.searchRequest {
		t.Fatal("late search result retained the current request identity")
	}
	m.Update(ready)
	if m.searchQuery != "" || len(m.searchMatches) != 0 || m.screen != "inventory" {
		t.Fatal("cancelled late search changed the new view")
	}
	t.Logf("100000-line search event: %s, %d allocated bytes; view change cancelled request %d", elapsed, allocated, oldRequest)
}

func TestSearchConsentKeyContextKeepsDenial(t *testing.T) {
	if binding, ok := keyBindingForContext("/", "plan"); ok {
		t.Fatalf("search key is available on consent: %+v", binding)
	}
	binding, ok := keyBindingForContext("n", "plan")
	if !ok || binding.action != keyDeny {
		t.Fatalf("consent n no longer denies: %+v, %t", binding, ok)
	}
	binding, ok = keyBindingForContext("n", "examples")
	if !ok || binding.action != keyNextMatch {
		t.Fatalf("list n no longer advances search: %+v, %t", binding, ok)
	}
	if !reflect.DeepEqual(searchKeyContexts, []keyContext{contextOverview, contextChanges, contextDiff, contextActivity, contextInspector}) {
		t.Fatal("search escaped its explicit non-consent contexts")
	}
}

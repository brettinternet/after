package browser

import (
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
)

func overviewModel(t *testing.T, selection Selection) *Model {
	t.Helper()
	data, err := Load(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	m := New(t.Context(), selection, Jobs{})
	t.Cleanup(m.Close)
	m.data, m.selected = data, selection
	m.width, m.height = 120, 40
	m.theme = terminal.Theme{}
	m.setClock(func() time.Time { return viewTime }, time.UTC)
	m.selectFirstOverviewRow()
	return m
}

func TestOverviewTypedGroupsAndHistorySelection(t *testing.T) {
	m := overviewModel(t, viewPaymentLoopSelection(t))
	rows := m.overviewRows()
	var groups []overviewGroup
	for _, row := range rows {
		if row.kind == overviewGroupHeader {
			groups = append(groups, row.group)
		}
	}
	wantGroups := []overviewGroup{groupNeedsAnotherLook, groupAgrees, groupEarlierSnapshots}
	if len(groups) != len(wantGroups) {
		t.Fatalf("groups = %v, want %v", groups, wantGroups)
	}
	for i := range wantGroups {
		if groups[i] != wantGroups[i] {
			t.Fatalf("group order = %v, want %v", groups, wantGroups)
		}
	}
	needs, agrees, history := rows[0], overviewRow{}, overviewRow{}
	for _, row := range rows {
		if row.kind != overviewGroupHeader {
			continue
		}
		switch row.group {
		case groupNeedsAnotherLook:
			needs = row
		case groupAgrees:
			agrees = row
		case groupEarlierSnapshots:
			history = row
		}
	}
	if needs.count != 2 || agrees.count != 1 || history.count != 2 || !m.overviewCollapsed(groupEarlierSnapshots) {
		t.Fatalf("group counts/collapse: needs=%d agrees=%d history=%d collapsed=%t", needs.count, agrees.count, history.count, m.overviewCollapsed(groupEarlierSnapshots))
	}
	var historyPosition int = -1
	for index, row := range rows {
		if row.kind == overviewGroupHeader && row.group == groupEarlierSnapshots {
			historyPosition = index
		}
		if row.kind == overviewEvidence {
			m.selectOverviewPosition(index)
			if m.index != row.entryIndex || m.selectedOverviewEntry() != &m.data.Entries[row.entryIndex] {
				t.Fatalf("selection lost row action target at %d: row=%+v selected=%+v", index, row, m.selectedOverviewEntry())
			}
		}
	}
	if historyPosition < 0 {
		t.Fatal("collapsed history header missing")
	}
	m.selectOverviewPosition(historyPosition)
	step(m, tea.KeyMsg{Type: tea.KeyEnter})
	var oldCandidate evidence.Digest
	found := map[string]bool{}
	for index, row := range m.overviewRows() {
		if row.kind != overviewEvidence || row.group != groupEarlierSnapshots {
			continue
		}
		entry := m.data.Entries[row.entryIndex]
		if oldCandidate == "" {
			oldCandidate = entry.Candidate
		}
		if entry.Candidate != oldCandidate || entry.Candidate == m.data.Selection.Pair.Candidate {
			t.Fatalf("history row has wrong candidate: %+v current=%s", entry, m.data.Selection.Pair.Candidate)
		}
		line := m.overviewRowText(row, false)
		if !strings.Contains(line, "ran on "+shortID(oldCandidate)) {
			t.Fatalf("history row lacks its candidate label: %q", line)
		}
		found[entry.Name] = true
		m.selectOverviewPosition(index)
		if m.index != row.entryIndex || m.selectedOverviewEntry() != &m.data.Entries[row.entryIndex] {
			t.Fatalf("expanded history selection lost action target at %d", index)
		}
	}
	if !found["12h same-key retry"] || !found["30s same-key retry"] {
		t.Fatalf("earlier run rows not preserved: %v", found)
	}
}

func TestOverviewRawdiffAndFullNoEvidenceInventory(t *testing.T) {
	m := overviewModel(t, viewRawSelection(t))
	rows := m.overviewRows()
	if !strings.Contains(m.View(), "NOT CHECKED") || !strings.Contains(m.View(), "Nothing was run or imported") {
		t.Fatal("empty-evidence Overview is not explicitly unchecked:\n", m.View())
	}
	changesFound := false
	inventory := map[string]bool{}
	for _, row := range rows {
		switch row.kind {
		case overviewChanges:
			line := m.overviewRowText(row, false)
			changesFound = strings.Contains(line, "CHANGES 6 paths") && strings.Contains(line, "1 captured hunk") && strings.Contains(line, "0 potential oracles")
		case overviewInventory:
			entry := m.data.Inventory[row.inventoryIndex]
			inventory[entry.Name] = true
			m.selectOverviewPosition(indexOfOverviewRow(rows, row))
			if m.inventory != row.inventoryIndex || m.selectedOverviewEntry() != &m.data.Inventory[row.inventoryIndex] {
				t.Fatalf("inventory selection lost path action target: %+v", row)
			}
		}
	}
	if !changesFound {
		t.Fatalf("rawdiff path/hunk/oracle counts missing: %q", m.overviewChangesLine())
	}
	for _, path := range []string{"app/config.go", "app/main.go", "binary", "go.mod", "unknown", "unsupported"} {
		if !inventory[path] {
			t.Fatalf("no-evidence Overview omitted %q: %v", path, inventory)
		}
	}
	if !strings.Contains(m.View(), "excluded: untracked") || !strings.Contains(m.View(), "unsupported: symlink") {
		t.Fatalf("excluded/unsupported inventory details were hidden:\n%s", m.View())
	}
}

func indexOfOverviewRow(rows []overviewRow, target overviewRow) int {
	for index, row := range rows {
		if row.kind == target.kind && row.entryIndex == target.entryIndex && row.inventoryIndex == target.inventoryIndex && row.group == target.group {
			return index
		}
	}
	return -1
}

func TestOverviewReportProvenanceAndOutcomeLines(t *testing.T) {
	m := overviewModel(t, viewReportSelection(t))
	var overviewLines []string
	for _, row := range m.overviewRows() {
		if row.kind == overviewReport || row.kind == overviewEvidence {
			overviewLines = append(overviewLines, m.overviewRowTextWidth(row, false, 120))
		}
	}
	view := strings.Join(overviewLines, "\n")
	if strings.Count(view, "report ") != 2 {
		t.Fatalf("each report needs its own provenance line:\n%s", view)
	}
	for _, want := range []string{
		shortID(m.data.Entries[0].ReportID),
		"bound to " + shortID(m.data.Selection.Pair.Candidate),
		"bound to " + shortID(m.data.Selection.Pair.Base),
		"imported 19:17",
		"imported 19:18",
		"1 pass",
		"1 fail",
		"candidate suite",
		"base tests on candi…",
		"TestFreeShippingThreshold",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing report outcome/provenance %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "[ACCEPTED]") || strings.Contains(view, "\x1b[32m") {
		t.Fatal("report prose or reported pass acquired trusted/observed styling", view)
	}
	m.theme = terminal.Theme{Color: true}
	styled := m.View()
	if !strings.Contains(styled, "\x1b[34m[REPORTED]") || strings.Contains(styled, "\x1b[32m") {
		t.Fatal("reported outcomes were not styled separately from observations", styled)
	}
	m.theme = terminal.Theme{}
	m.width = 240
	m.data.Entries[0].ReportProducer = "\x1b]52;c;clipboard\a\n[ACCEPTED]"
	untrusted := m.reportLine(m.data.Entries[0], 240)
	if strings.ContainsAny(untrusted, "\x1b\a\r") || !strings.Contains(untrusted, "[ACCEPTED]") || badgeFor(m.data.Entries[0]).word != "REPORTED" || strings.Contains(m.statusLine(), "ACCEPTED") {
		t.Fatal("producer text escaped the data column or changed trusted state", untrusted)
	}
}

func TestNextLinePriority(t *testing.T) {
	now := viewTime
	currentPair := evidence.SnapshotPair{Base: evidence.Digest("sha256:" + strings.Repeat("a", 64)), Candidate: evidence.Digest("sha256:" + strings.Repeat("b", 64))}
	pin := Entry{Decision: evidence.Pinned, State: evidence.EvidenceState{Kind: evidence.Observed, Applicability: evidence.Current}}
	reopened := Entry{Decision: evidence.Reopened, State: evidence.EvidenceState{Kind: evidence.Observed, Applicability: evidence.Current}, MissingCurrentResult: true}
	cases := []struct {
		name  string
		setup func(*Model)
		want  string
	}{
		{"loading", func(*Model) {}, "Loading stored records — nothing runs on open"},
		{"load failed", func(m *Model) { m.loadFailed = true }, "Couldn't load this pair — check the IDs with after inspect"},
		{"consent", func(m *Model) { m.data = &Data{}; m.screen = "plan" }, "Nothing has run. y runs this exact plan once · n denies"},
		{"active run", func(m *Model) { m.data = &Data{}; m.running = true; m.runStarted = now.Add(-65 * time.Second) }, "Running the approved plan · 1:05 · x cancels (the incomplete result is kept)"},
		{"pending capture", func(m *Model) { m.data = &Data{}; m.pending = &evidence.SnapshotPair{Candidate: currentPair.Candidate} }, "New capture bbbbbbbb — u reviews it"},
		{"reopened pin", func(m *Model) {
			m.data = &Data{Selection: Selection{Pair: currentPair}, Entries: []Entry{reopened}}
			m.selectFirstOverviewRow()
		}, "Pin reopened: no result for this candidate yet — r previews a rerun"},
		{"current pin", func(m *Model) {
			m.data = &Data{Selection: Selection{Pair: currentPair}, Entries: []Entry{pin}}
			m.selectFirstOverviewRow()
		}, "Current result attached — accept it with after pin PIN --accept"},
		{"no evidence", func(m *Model) { m.data = &Data{Selection: Selection{Pair: currentPair}} }, "Not checked — read the change, or c captures again after editing"},
		{"otherwise blank", func(m *Model) {
			m.data = &Data{Selection: Selection{Pair: currentPair}, Entries: []Entry{{State: evidence.EvidenceState{Kind: evidence.Reported, Applicability: evidence.Current}}}}
			m.selectFirstOverviewRow()
		}, ""},
		{"transient action result", func(m *Model) { m.data = &Data{}; m.status = "Directory opened" }, "Directory opened"},
		{"load failure precedes consent", func(m *Model) { m.loadFailed = true; m.screen = "plan" }, "Couldn't load this pair — check the IDs with after inspect"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(t.Context(), Selection{Pair: currentPair}, Jobs{})
			defer m.Close()
			m.setClock(func() time.Time { return now }, time.UTC)
			tc.setup(m)
			if got := m.statusLine(); got != tc.want {
				t.Fatalf("Next line = %q, want %q", got, tc.want)
			}
		})
	}
}

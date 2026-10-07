package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
)

func viewCardScenarios(t *testing.T) []struct {
	name, entryName, section string
	selection                Selection
} {
	t.Helper()
	s, base := setup(t, false)
	comparisonID := viewComparison(t, s, base, false, evidence.Complete)
	comparison, err := store.Get[evidence.Comparison](s, comparisonID)
	if err != nil {
		t.Fatal(err)
	}
	return []struct {
		name, entryName, section string
		selection                Selection
	}{
		{"payment-case", "12h same-key retry", "Card", Selection{Project: base.Project, Pair: base.Pair, Evidence: []evidence.Digest{comparisonID}}},
		{"receipt", "12h same-key retry", "Receipt Card", Selection{Project: base.Project, Pair: base.Pair, Evidence: []evidence.Digest{comparison.Receipt}}},
		{"comparison", "12h same-key retry", "Receipt Card", Selection{Project: base.Project, Pair: base.Pair, Evidence: []evidence.Digest{comparisonID}}},
		{"pin", "", "Card", viewPaymentLoopSelection(t)},
		{"report", "", "Card", viewReportSelection(t)},
		{"unavailable", "", "Card", viewUnavailableCardSelection(t)},
	}
}

func viewUnavailableCardSelection(t *testing.T) Selection {
	t.Helper()
	selection := viewRawSelection(t)
	selection.Evidence = []evidence.Digest{evidence.Digest("sha256:" + strings.Repeat("0", 64))}
	return selection
}

func TestGoldenCardViews(t *testing.T) {
	for _, scenario := range viewCardScenarios(t) {
		t.Run(scenario.name, func(t *testing.T) {
			data, err := Load(t.Context(), scenario.selection)
			if err != nil {
				t.Fatal(err)
			}
			entryIndex := -1
			sectionIndex := -1
			for i, entry := range data.Entries {
				if scenario.entryName != "" && entry.Name != scenario.entryName {
					continue
				}
				if scenario.name == "pin" && entry.Decision == "" {
					continue
				}
				if scenario.name == "report" && entry.ReportID == "" {
					continue
				}
				if scenario.name == "unavailable" && !entry.Unavailable {
					continue
				}
				for j, section := range entry.Sections {
					if section.Name == scenario.section {
						entryIndex, sectionIndex = i, j
						break
					}
				}
				if sectionIndex >= 0 {
					break
				}
			}
			if entryIndex < 0 || sectionIndex < 0 {
				t.Fatalf("card %q/%q not found in %+v", scenario.entryName, scenario.section, data.Entries)
			}
			model := New(t.Context(), scenario.selection, Jobs{})
			t.Cleanup(model.Close)
			model.data, model.selected = data, scenario.selection
			model.screen, model.returnTo = "inspector", "examples"
			model.width, model.height, model.index, model.section = 120, 40, entryIndex, sectionIndex
			model.theme = terminal.Theme{}
			model.setClock(func() time.Time { return viewTime }, time.UTC)
			drain(model, model.loadDocument())
			got := model.View() + "\n"
			path := filepath.Join("testdata", "cards", scenario.name+".txt")
			if *updateViews {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden %s; regenerate explicitly with -update\n%s", path, got)
			}
			if string(want) != got {
				t.Fatalf("card view differs: %s; regenerate explicitly with -update\n%s", path, got)
			}
		})
	}
}

func TestPaymentCaseCardUsesBaseCandidateColumns(t *testing.T) {
	s, selection := setup(t, false)
	comparisonID := viewComparison(t, s, selection, false, evidence.Complete)
	data, err := Load(t.Context(), Selection{Project: selection.Project, Pair: selection.Pair, Evidence: []evidence.Digest{comparisonID}})
	if err != nil {
		t.Fatal(err)
	}
	card := sectionByName(t, data.Entries[0], "Card")
	document, err := readSectionDocument(t.Context(), selection.Project, card, 120)
	if err != nil {
		t.Fatal(err)
	}
	assertColumnPair := func(left, right string) {
		t.Helper()
		for _, line := range strings.Split(string(document.display), "\n") {
			leftIndex, rightIndex := strings.Index(line, left), strings.Index(line, right)
			if leftIndex >= 0 && rightIndex > leftIndex {
				return
			}
		}
		t.Fatalf("base/candidate values are not aligned in columns: %q", document.display)
	}
	for _, pair := range [][2]string{{"base " + shortID(selection.Pair.Base), "candidate " + shortID(selection.Pair.Candidate)}, {"base 1", "candidate 2"}, {"base 200 ok", "candidate 200 ok"}} {
		assertColumnPair(pair[0], pair[1])
	}
}

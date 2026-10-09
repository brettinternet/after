package browser

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
)

func viewCommandComparison(t *testing.T, s *store.Store, selection Selection) evidence.Digest {
	t.Helper()
	definition := runner.CommandDefinition{
		Version: 1, Kind: "command", Name: "command-fixture", Platform: "linux/amd64", Image: sandbox.Image,
		Cases: []runner.CommandCase{
			{ID: "typo", Title: "Command changes", Argv: []string{"/work/fixture"}, Stdin: []byte{}, Environment: []string{}},
			{ID: "control", Title: "Unaffected control", Argv: []string{"/work/fixture"}, Stdin: []byte{}, Environment: []string{}},
		},
		Repetitions: 2, Limits: runner.DefinitionLimits{Seconds: 30, OutputBytes: 65536, PreparationSeconds: 90},
		Comparison: runner.CommandComparison{Stdout: "json", Stderr: "text"},
	}
	input := viewArtifact(t, s, definition, "command-definition")
	scenario, err := store.Put(s, evidence.Scenario{
		SchemaVersion: evidence.SchemaVersion, Input: input.Content, Driver: input.Content, Observer: input.Content, Rules: input.Content,
		Boundary: "synthetic command boundary", Author: "AFTER operator-selected command v1", Limits: []string{"synthetic finite command example"},
	})
	if err != nil {
		t.Fatal(err)
	}
	env := &evidence.Environment{Environment: input.Content, Toolchain: input.Content, Dependencies: input.Content, Argv: []string{"synthetic command"}}
	receipt := evidence.Receipt{
		RequestID: input.Content, SchemaVersion: evidence.SchemaVersion,
		State:           evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.Observed, Applicability: evidence.Current, Execution: evidence.Completed, Comparison: evidence.Unstable, Report: evidence.NoReport},
		Snapshots:       selection.Pair,
		Bindings:        &evidence.Bindings{Scenario: scenario.ID, Input: input.Content, Driver: input.Content, Observer: input.Content, Rules: input.Content},
		BaseEnvironment: env, CandidateEnvironment: env, Authorization: input.Content,
		StartedAt: viewTime, FinishedAt: viewTime, Completeness: evidence.Complete, Artifacts: []evidence.Artifact{input}, Limits: []string{"synthetic; not a real command run"},
	}
	ref := func(side, caseID string, repetition int) compare.SampleRef {
		return compare.SampleRef{Side: side, CaseID: caseID, Repetition: repetition, Metadata: input.Content, Observation: input.Content}
	}
	textChange := func(before, after string) compare.Change {
		beforeRaw, _ := json.Marshal(struct {
			Base64 string `json:"base64"`
		}{base64.StdEncoding.EncodeToString([]byte(before))})
		afterRaw, _ := json.Marshal(struct {
			Base64 string `json:"base64"`
		}{base64.StdEncoding.EncodeToString([]byte(after))})
		return compare.Change{Path: "/base64", Kind: "changed", Before: beforeRaw, After: afterRaw}
	}
	witnesses := []compare.Witness{}
	for _, caseID := range []string{"typo", "control"} {
		for repetition := 0; repetition < 2; repetition++ {
			for _, channel := range []string{"exit_status", "stdout", "stderr"} {
				outcome := evidence.Equal
				var changes []compare.Change
				if caseID == "typo" {
					outcome = evidence.Different
					switch channel {
					case "exit_status":
						changes = []compare.Change{{Path: "", Kind: "changed", Before: json.RawMessage("2"), After: json.RawMessage("0")}}
					case "stdout":
						changes = []compare.Change{{Path: "/error", Kind: "removed", Before: json.RawMessage(`"invalid request"`)}, {Path: "/total_cents", Kind: "added", After: json.RawMessage("0")}}
					case "stderr":
						changes = []compare.Change{textChange("invalid \x1b]52;c;clipboard\a request", "handled safely")}
					}
				}
				witnesses = append(witnesses, compare.Witness{Relation: "paired", Channel: channel, Before: ref("base", caseID, repetition), After: ref("candidate", caseID, repetition), Outcome: outcome, Changes: changes})
			}
		}
		for _, channel := range []string{"exit_status", "stdout", "stderr"} {
			for _, side := range []string{"base", "candidate"} {
				outcome := evidence.Equal
				var changes []compare.Change
				if caseID == "typo" && channel == "stderr" && side == "base" {
					outcome = evidence.Different
					changes = []compare.Change{textChange("invalid \x1b]52;c;clipboard\a request", "retrying safely")}
				}
				witnesses = append(witnesses, compare.Witness{Relation: "repetition", Channel: channel, Before: ref(side, caseID, 0), After: ref(side, caseID, 1), Outcome: outcome, Changes: changes})
			}
		}
	}
	report := compare.Report{
		Version: 1, Snapshots: selection.Pair, DefinitionName: definition.Name,
		Cases: []string{"typo", "control"}, CaseTitles: []string{"Command changes", "Unaffected control"},
		Channels: []string{"exit_status", "stdout", "stderr"}, Outcome: evidence.Unstable,
		Artifacts: []evidence.Artifact{input}, Witnesses: witnesses, Limits: []string{"synthetic finite command comparison"},
	}
	receipt, err = store.Put(s, receipt)
	if err != nil {
		t.Fatal(err)
	}
	report.Receipt = receipt.ID
	detail := viewArtifact(t, s, report, "comparison-details-v1")
	comparison, err := store.Put(s, evidence.Comparison{SchemaVersion: evidence.SchemaVersion, Receipt: receipt.ID, Outcome: evidence.Unstable, Completeness: evidence.Complete, Details: &detail, Limits: report.Limits})
	if err != nil {
		t.Fatal(err)
	}
	return comparison.ID
}

func viewCardScenarios(t *testing.T) []struct {
	name, entryName, section string
	selection                Selection
} {
	t.Helper()
	s, base := setup(t, false)
	comparisonID := viewComparison(t, s, base, false, evidence.Complete)
	commandComparisonID := viewCommandComparison(t, s, base)
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
		{"command", "command-fixture command", "Card", Selection{Project: base.Project, Pair: base.Pair, Evidence: []evidence.Digest{commandComparisonID}}},
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

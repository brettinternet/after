package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
)

var viewTime = time.Date(2025, 1, 2, 20, 0, 0, 0, time.UTC)

func viewArtifact(t *testing.T, s *store.Store, value any, channel string) evidence.Artifact {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.PutArtifact(raw, channel, store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// Deliberately synthetic protocol records, not execution evidence. This path
// exercises real store decoding and browser rows without Docker or wall clocks.
func viewComparison(t *testing.T, s *store.Store, sel Selection, unstable bool, complete evidence.Completeness) evidence.Digest {
	t.Helper()
	a := viewArtifact(t, s, "synthetic fixture only", "input")
	scenario, err := store.Put(s, evidence.Scenario{SchemaVersion: 1, Input: a.Content, Driver: a.Content, Observer: a.Content, Rules: a.Content, Boundary: "synthetic", Author: "test", Limits: []string{"not execution"}})
	if err != nil {
		t.Fatal(err)
	}
	env := &evidence.Environment{Environment: a.Content, Toolchain: a.Content, Dependencies: a.Content, Argv: []string{"synthetic"}}
	r := evidence.Receipt{SchemaVersion: 1, State: evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.Observed, Applicability: evidence.Current, Execution: evidence.Completed, Comparison: evidence.NotCompared, Report: evidence.NoReport}, Snapshots: sel.Pair, Bindings: &evidence.Bindings{Scenario: scenario.ID, Input: a.Content, Driver: a.Content, Observer: a.Content, Rules: a.Content}, BaseEnvironment: env, CandidateEnvironment: env, Authorization: a.Content, StartedAt: viewTime, FinishedAt: viewTime, Completeness: evidence.Complete, Limits: []string{"synthetic; not a real run"}}
	report := compare.Report{Version: 1, Snapshots: sel.Pair, Outcome: evidence.Different, Limits: []string{"synthetic"}}
	for _, sec := range []int64{43200, 30} {
		for rep := 0; rep < 2; rep++ {
			for _, side := range []string{"base", "candidate"} {
				count := 1
				if side == "candidate" && sec == 43200 {
					count = 2
				}
				if unstable && side == "candidate" && sec == 43200 && rep == 1 {
					count = 1
				}
				o := runner.Observation{Version: 1, Seconds: sec, Responses: []runner.Response{{Status: 200, Body: "ok"}, {Status: 200, Body: "ok"}}, Calls: make([]runner.Call, count)}
				r.Artifacts = append(r.Artifacts, viewArtifact(t, s, o, fmt.Sprintf("%s/%d/%d/observation", side, sec, rep)))
			}
			for _, channel := range []string{"provider", "responses"} {
				outcome := evidence.Equal
				if sec == 43200 && channel == "provider" {
					outcome = evidence.Different
				}
				report.Witnesses = append(report.Witnesses, compare.Witness{Relation: "paired", Channel: channel, Before: compare.SampleRef{Side: "base", Seconds: sec, Repetition: rep}, After: compare.SampleRef{Side: "candidate", Seconds: sec, Repetition: rep}, Outcome: outcome})
			}
		}
	}
	if unstable {
		report.Outcome = evidence.Unstable
		report.Witnesses = append(report.Witnesses, compare.Witness{Relation: "repetition", Channel: "provider", Before: compare.SampleRef{Side: "candidate", Seconds: 43200, Repetition: 0}, After: compare.SampleRef{Side: "candidate", Seconds: 43200, Repetition: 1}, Outcome: evidence.Different})
	}
	r, err = store.Put(s, r)
	if err != nil {
		t.Fatal(err)
	}
	report.Receipt = r.ID
	if complete != evidence.Complete {
		report.Outcome = evidence.Incomparable
	}
	detail := viewArtifact(t, s, report, "comparison-details-v1")
	c, err := store.Put(s, evidence.Comparison{SchemaVersion: 1, Receipt: r.ID, Outcome: report.Outcome, Completeness: complete, Details: &detail, Limits: report.Limits})
	if err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func viewSelection(t *testing.T) Selection {
	t.Helper()
	s, sel := setup(t, true)
	comparison := viewComparison(t, s, sel, false, evidence.Complete)
	report, err := gotestreport.Import(strings.NewReader("{\"Action\":\"pass\",\"Package\":\"example.com/cart\"}\n{\"Action\":\"fail\",\"Package\":\"example.com/cart\",\"Test\":\"TestThreshold\"}\n"), gotestreport.Metadata{Producer: "synthetic test", ImportedAt: viewTime})
	if err != nil {
		t.Fatal(err)
	}
	sel.Evidence = []evidence.Digest{comparison, viewArtifact(t, s, report, "report").Content, imported(t, s, sel.Pair), evidence.Digest("sha256:" + strings.Repeat("0", 64))}
	return sel
}

func TestCaseRowsFromStoredWitnesses(t *testing.T) {
	s, sel := setup(t, false)
	for _, tc := range []struct {
		unstable      bool
		complete      evidence.Completeness
		badge, counts string
	}{
		{false, evidence.Complete, "DIFFERENT", "1 → 2"},
		{true, evidence.Complete, "UNSTABLE", "1 → 2,1"},
		{false, evidence.Incomplete, "INCOMPLETE", "1 → 2"},
	} {
		sel.Evidence = []evidence.Digest{viewComparison(t, s, sel, tc.unstable, tc.complete)}
		d, err := Load(t.Context(), sel)
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Entries) != 2 || badgeFor(d.Entries[0]).word != tc.badge || !strings.Contains(d.Entries[0].Summary, tc.counts) {
			t.Fatal(d.Entries)
		}
		want := "EQUAL"
		if tc.complete != evidence.Complete {
			want = "INCOMPLETE"
		}
		if badgeFor(d.Entries[1]).word != want {
			t.Fatal(d.Entries[1])
		}
		if tc.complete == evidence.Complete && !strings.HasSuffix(d.Entries[0].Summary, "responses same") {
			t.Fatal(d.Entries[0])
		}
	}
}

func TestGoldenViews(t *testing.T) {
	sel := viewSelection(t)
	for _, size := range []tea.WindowSizeMsg{{Width: 120, Height: 40}, {Width: 80, Height: 24}, {Width: 40, Height: 12}} {
		for _, screen := range []string{"examples", "inventory", "patch"} {
			name := fmt.Sprintf("%s-%dx%d", screen, size.Width, size.Height)
			t.Run(name, func(t *testing.T) {
				m := New(t.Context(), sel, Jobs{})
				defer m.Close()
				m.setClock(func() time.Time { return viewTime }, time.UTC)
				m.theme.Color = false
				drain(m, m.Init())
				step(m, size)
				if screen == "patch" {
					step(m, key("3"))
				} else {
					m.screen = screen
				}
				got := m.View() + "\n"
				path := filepath.Join("testdata", "views", name+".txt")
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
					t.Fatal(err)
				}
				if string(want) != got {
					t.Fatalf("view differs: %s; regenerate explicitly with -update\n%s", path, got)
				}
				for _, line := range strings.Split(got, "\n") {
					if uniseg.StringWidth(line) > size.Width {
						t.Fatal("width", line)
					}
				}
			})
		}
	}
}

func TestFrameThemeSequences(t *testing.T) {
	sel := viewSelection(t)
	for _, env := range []struct {
		color, term string
		styled      bool
	}{{"1", "xterm", false}, {"", "dumb", false}, {"", "xterm-256color", true}} {
		t.Setenv("NO_COLOR", env.color)
		t.Setenv("TERM", env.term)
		m := New(t.Context(), sel, Jobs{})
		drain(m, m.Init())
		m.width = 120
		hostile := "\u0301\x1b[32m[EQUAL]\r\n\x1b]52;c;clipboard\a\u009b2J"
		m.data.Entries[0].Name = hostile
		m.data.Inventory[0].Name = hostile
		for _, screen := range []string{"examples", "inventory", "inspector", "patch", "plan", "help"} {
			m.screen = screen
			m.doc, _ = terminal.NewDocument([]byte(hostile))
			frame := m.View()
			re := regexp.MustCompile("\x1b\\[(?:0|32|1;35|1;33|1;31|34|36|2|1|7)m")
			plain := re.ReplaceAllString(frame, "")
			if strings.ContainsAny(plain, "\x1b\a\r") || strings.ContainsAny(plain, "\u009b\u009d") {
				t.Fatal("non-theme control", screen, frame)
			}
			if env.styled && frame == plain {
				t.Fatal("missing theme", screen)
			}
			if !env.styled && frame != plain {
				t.Fatal("unexpected color", screen)
			}
		}
		m.Close()
	}
}

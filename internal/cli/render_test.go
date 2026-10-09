package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/config"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
	"github.com/rivo/uniseg"
)

var updateReadable = flag.Bool("update", false, "regenerate deterministic readable CLI output goldens")

func goldenDigest(char string) evidence.Digest {
	return evidence.Digest("sha256:" + strings.Repeat(char, 64))
}

func goldenState() *invocation {
	location := time.FixedZone("golden", 2*60*60)
	return &invocation{
		stdout: &bytes.Buffer{}, stdoutTTY: true, columns: 80,
		theme: terminal.Theme{Color: false}, location: location,
		now: func() time.Time { return time.Date(2026, time.January, 2, 12, 0, 0, 0, location) },
	}
}

func goldenReceipt() evidence.Receipt {
	return evidence.Receipt{
		SchemaVersion: 1, ID: goldenDigest("d"),
		State:        evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.NoEvidence, Applicability: evidence.Unknown, Execution: evidence.Failed, Comparison: evidence.Incomparable, Report: evidence.NoReport},
		Snapshots:    evidence.SnapshotPair{Base: goldenDigest("a"), Candidate: goldenDigest("b")},
		StartedAt:    time.Date(2026, time.January, 2, 10, 30, 0, 0, time.UTC),
		FinishedAt:   time.Date(2026, time.January, 2, 10, 31, 0, 0, time.UTC),
		Completeness: evidence.Incomplete,
		Artifacts:    []evidence.Artifact{{Content: goldenDigest("e"), Channel: "sample", Bytes: 10, MaxBytes: 100, Completeness: evidence.Complete}},
		Limits:       []string{"synthetic, finite example"},
	}
}

func goldenComparison() (evidence.Comparison, compare.Report) {
	comparison := evidence.Comparison{SchemaVersion: 1, ID: goldenDigest("c"), Receipt: goldenDigest("d"), Outcome: evidence.Different, Completeness: evidence.Complete, Limits: []string{"recorded samples only"}}
	report := compare.Report{Version: 1, Receipt: comparison.Receipt, Outcome: evidence.Different, Limits: []string{"recorded samples only"}, Witnesses: []compare.Witness{
		{Relation: "paired", Channel: "responses", Before: compare.SampleRef{Seconds: 43200}, After: compare.SampleRef{Seconds: 43200}, Outcome: evidence.Equal},
		{Relation: "paired", Channel: "provider", Before: compare.SampleRef{Seconds: 43200}, After: compare.SampleRef{Seconds: 43200}, Outcome: evidence.Different, Changes: []compare.Change{{Path: "/count", Kind: "replace", Before: json.RawMessage("1"), After: json.RawMessage("2")}}},
		{Relation: "paired", Channel: "responses", Before: compare.SampleRef{Seconds: 30}, After: compare.SampleRef{Seconds: 30}, Outcome: evidence.Equal},
		{Relation: "paired", Channel: "provider", Before: compare.SampleRef{Seconds: 30}, After: compare.SampleRef{Seconds: 30}, Outcome: evidence.Equal},
	}}
	return comparison, report
}

type readableGoldenCase struct {
	name string
	kind string
	data any
}

type readableRunOutput struct {
	Status        string              `json:"status"`
	Authorization string              `json:"authorization_digest"`
	Plan          json.RawMessage     `json:"plan"`
	Receipt       evidence.Receipt    `json:"receipt"`
	Comparison    evidence.Comparison `json:"comparison"`
	Details       compare.Report      `json:"details"`
	Samples       []runner.Sample     `json:"samples"`
}

func goldenCommandPresentation() (runner.CommandDefinition, compare.Report) {
	definition := runner.CommandDefinition{
		Version: 1, Kind: "command", Name: "command-fixture", Platform: "linux/amd64", Image: sandbox.Image,
		Cases: []runner.CommandCase{
			{ID: "typo", Title: "Command changes", Argv: []string{"/work/fixture"}, Stdin: []byte{}, Environment: []string{}},
			{ID: "control", Title: "Unaffected control", Argv: []string{"/work/fixture"}, Stdin: []byte{}, Environment: []string{}},
		},
		Repetitions: 2, Limits: runner.DefinitionLimits{Seconds: 30, OutputBytes: 65536, PreparationSeconds: 90},
		Comparison: runner.CommandComparison{Stdout: "json", Stderr: "text"},
	}
	ref := func(side, caseID string, repetition int) compare.SampleRef {
		return compare.SampleRef{Side: side, CaseID: caseID, Repetition: repetition, Metadata: goldenDigest("e"), Observation: goldenDigest("f")}
	}
	witness := func(relation, channel, caseID string, repetition int, outcome evidence.ComparisonOutcome, changes ...compare.Change) compare.Witness {
		before, after := ref("base", caseID, repetition), ref("candidate", caseID, repetition)
		if relation == "repetition" {
			after = ref("base", caseID, repetition+1)
		}
		return compare.Witness{Relation: relation, Channel: channel, Before: before, After: after, Outcome: outcome, Changes: changes}
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
	var witnesses []compare.Witness
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
				witnesses = append(witnesses, witness("paired", channel, caseID, repetition, outcome, changes...))
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
				before := ref(side, caseID, 0)
				after := ref(side, caseID, 1)
				witnesses = append(witnesses, compare.Witness{Relation: "repetition", Channel: channel, Before: before, After: after, Outcome: outcome, Changes: changes})
			}
		}
	}
	report := compare.Report{
		Version: 1, Receipt: goldenReceipt().ID, Snapshots: goldenReceipt().Snapshots, DefinitionName: definition.Name,
		Cases: []string{"typo", "control"}, CaseTitles: []string{"Command changes", "Unaffected control"},
		Channels: []string{"exit_status", "stdout", "stderr"}, Outcome: evidence.Unstable,
		Artifacts: []evidence.Artifact{}, Witnesses: witnesses, Limits: []string{"synthetic finite command comparison"},
	}
	return definition, report
}

func readableGoldenCases() []readableGoldenCase {
	idA, idB, idC, idD, idE := goldenDigest("a"), goldenDigest("b"), goldenDigest("c"), goldenDigest("d"), goldenDigest("e")
	baseSnapshot := evidence.Snapshot{SchemaVersion: 1, ID: idA, Source: evidence.Commit, Commit: strings.Repeat("a", 40), Files: []evidence.File{{Path: "internal/cli/base.go", Content: idE, Mode: "100644"}}, Completeness: evidence.Complete, Diff: idE, Limits: []string{"two matching reads; not an atomic filesystem snapshot"}}
	snapshot := evidence.Snapshot{SchemaVersion: 1, ID: idB, Source: evidence.WorkingTree, Commit: strings.Repeat("1", 40), Files: []evidence.File{{Path: "internal/cli/main.go", Content: idE, Mode: "100644"}}, Completeness: evidence.Complete, Diff: idE, Limits: []string{"two matching reads; not an atomic filesystem snapshot"}}
	report := gotestreport.Report{SchemaVersion: 1, Dialect: gotestreport.Dialect, Metadata: gotestreport.Metadata{Producer: "go1.27.1 · linux/amd64", Snapshot: idB, ImportedAt: time.Date(2026, time.January, 2, 10, 31, 0, 0, time.UTC)}, Cards: []gotestreport.Card{{Package: "example.invalid/cart", Test: "TestRetry", Scope: "test", Attempt: 1, State: evidence.EvidenceState{Producer: evidence.Importer, Kind: evidence.Reported, Applicability: evidence.Unknown, Execution: evidence.NotRun, Comparison: evidence.NotCompared, Report: evidence.ReportFail}}}, Completeness: evidence.Complete}
	counts := map[evidence.ReportOutcome]int{evidence.ReportPass: 2, evidence.ReportFail: 1, evidence.ReportSkip: 0}
	reportView := reportViewData{ID: idC, SchemaVersion: 1, Dialect: gotestreport.Dialect, Metadata: report.Metadata, Completeness: evidence.Complete, ReportedOutcomes: counts, Cards: report.Cards, CardTotal: 1}
	defaultBindingView := reportView
	defaultBindingView.Metadata.Producer = ""
	defaultBindingView.Binding = &reportBindingView{Source: evidence.WorkingTree, CapturedNow: true, UntrackedExcluded: 1}
	captureHistory := snapshotCaptureHistory{Records: []store.CaptureSummary{{ID: idC, CapturedAt: time.Date(2026, time.January, 2, 8, 30, 0, 0, time.UTC), Mode: evidence.WorkingTree, Base: idA, Candidate: idB, Index: idE, SelectedUntracked: 1}}}
	pair := snapshotView{Base: idA, Candidate: idB, BaseRecord: baseSnapshot, CandidateRecord: snapshot, BaseCaptureHistory: captureHistory, CandidateCaptureHistory: captureHistory, Inventory: []inventoryItem{{Path: "internal/cli/main.go", Change: "modified", PotentialOracle: true, Limits: []string{}}}, InventoryTotal: 1, Diff: struct {
		Content   evidence.Digest `json:"content"`
		Offset    int             `json:"offset"`
		Next      int             `json:"next"`
		Total     int             `json:"total"`
		More      bool            `json:"more"`
		Base64    string          `json:"base64"`
		Available bool            `json:"available"`
		Limits    []string        `json:"limits"`
	}{Content: idE, Offset: 0, Next: 18, Total: 18, Base64: "ZGlmZg==", Available: true}, Limits: []string{"raw diff is a captured finite patch"}}
	comparison, details := goldenComparison()
	receipt := goldenReceipt()
	pin := evidence.Pin{SchemaVersion: 1, ID: idD, Scenario: idE, Expectation: "At 12h, retries make one provider request", BasisReceipt: receipt.ID, BasisSnapshots: receipt.Snapshots, Decision: evidence.Reopened, Scope: evidence.FiniteExample, History: []evidence.DecisionEvent{{Decision: evidence.Pinned, At: time.Date(2026, time.January, 2, 9, 0, 0, 0, time.UTC), Reason: "initial"}, {Decision: evidence.Reopened, At: time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC), Reason: "selected new snapshot"}}}
	view := review.View{Pin: pin, Applicability: evidence.Stale, Reason: "whole-project snapshot identity changed", MissingCurrentResult: true, Limits: []string{review.ReuseLimit}}
	status := statusView{
		Capture:      &statusCapture{ID: idC, CapturedAt: time.Date(2026, time.January, 2, 10, 30, 0, 0, time.UTC), Mode: evidence.WorkingTree, Base: idA, Candidate: idB, SelectedUntracked: 1},
		Base:         &statusSnapshot{ID: idA, Source: evidence.Commit, Commit: strings.Repeat("a", 40), Completeness: evidence.Complete, Files: 1},
		Candidate:    &statusSnapshot{ID: idB, Source: evidence.WorkingTree, Commit: strings.Repeat("1", 40), Completeness: evidence.Complete, Files: 1, Excluded: 1},
		ChangedPaths: 1, UntrackedExcluded: true, Receipt: &receipt, Comparison: &comparison, Details: &details,
		Freshness: &freshness{State: freshChanged, Scope: evidence.WorkingTree},
		Pins:      []statusPin{{ID: idD, Decision: evidence.Reopened, Applicability: evidence.Stale, Expectation: pin.Expectation, MissingCurrentResult: true}},
		Reports:   []statusReport{{ID: idE, Imported: time.Date(2026, time.January, 2, 10, 31, 0, 0, time.UTC), Producer: "fixture report", Pass: 2, Fail: 1}},
		Next:      []nextCommand{next("after review "+string(idA)+" "+string(idB), "review this capture")},
	}
	log := logView{Total: 4, Shown: 4, Rows: []logRow{
		{Kind: "pin", ID: idD, At: time.Date(2026, time.January, 2, 11, 0, 0, 0, time.UTC), Base: idA, Candidate: idB, Receipt: idD, Decision: evidence.Reopened, Action: "attach", Expectation: pin.Expectation},
		{Kind: "report", ID: idC, At: time.Date(2026, time.January, 2, 10, 31, 0, 0, time.UTC), Candidate: idB, Producer: "fixture report", Pass: 2, Fail: 1},
		{Kind: "run", ID: idD, At: time.Date(2026, time.January, 2, 10, 30, 0, 0, time.UTC), Base: idA, Candidate: idB, Outcome: evidence.Different, Summary: "12h [DIFFERENT] 1 → 2 · 30s [EQUAL] 1 → 1"},
		{Kind: "capture", ID: idC, At: time.Date(2026, time.January, 2, 8, 30, 0, 0, time.UTC), Base: idA, Candidate: idB, Mode: evidence.WorkingTree, Paths: 1},
	}, Next: []nextCommand{next("after status", "show the current capture and review state")}}
	pinHeads := pinListView{Pins: []review.View{view}, Next: []nextCommand{next("after status", "show the current capture and review state")}}
	plan, _ := json.Marshal(struct {
		Snapshots   evidence.SnapshotPair `json:"snapshots"`
		Repetitions int                   `json:"repetitions"`
		Concurrency int                   `json:"concurrency"`
		Limits      sandbox.Limits        `json:"limits"`
	}{evidence.SnapshotPair{Base: idA, Candidate: idB}, 2, 1, sandbox.Limits{Seconds: 180, OutputBytes: 65536}})
	artifact := struct {
		ID       evidence.Digest `json:"id"`
		Offset   int             `json:"offset"`
		Next     int             `json:"next"`
		Total    int             `json:"total"`
		More     bool            `json:"more"`
		Base64   string          `json:"base64"`
		Document json.RawMessage `json:"document,omitempty"`
	}{ID: idE, Offset: 0, Next: 17, Total: 17, Base64: "eyJrZXkiOiJ2YWx1ZSJ9", Document: json.RawMessage(`{"key":"value"}`)}
	_, commandDetails := goldenCommandPresentation()
	commandReceipt := receipt
	commandReceipt.State.Kind = evidence.Observed
	commandReceipt.State.Execution = evidence.Completed
	commandReceipt.State.Comparison = evidence.Unstable
	commandReceipt.Completeness = evidence.Complete
	return []readableGoldenCase{
		{"status", "status", status},
		{"log", "log", log},
		{"pin-heads", "pins", pinHeads},
		{"config", "configuration", configurationResult{
			Settings: []configSetting{{"repetitions", float64(2), "default"}, {"docker_host", "not configured (value hidden)", "default"}},
			Setup: dockerSetup{
				Problems:    []string{"Docker execution is not configured; set docker_binary and docker_host as a pair."},
				Suggestions: []string{`docker_binary: "/usr/local/bin/docker"  # found on PATH; not executed`, `docker_host: "unix:///var/run/docker.sock"  # existing socket; not contacted`},
				Apply:       "export AFTER_DOCKER_BINARY=/usr/local/bin/docker AFTER_DOCKER_HOST=unix:///var/run/docker.sock",
			}}},
		{"capture", "capture", captureOutput{
			Base:      snapshotSummary{ID: idA, Source: evidence.Commit, Completeness: evidence.Complete, Files: 3, Limits: []string{"two matching reads; not an atomic filesystem snapshot"}},
			Candidate: snapshotSummary{ID: idB, Source: evidence.WorkingTree, Completeness: evidence.Complete, Files: 4, Excluded: 1, Limits: []string{"two matching reads; not an atomic filesystem snapshot"}},
			Index:     &snapshotSummary{ID: idC, Source: evidence.Index, Completeness: evidence.Complete, Files: 3},
			Next:      []nextCommand{next("after review", "open a review of this change"), next("after diff --stored", "print this captured patch")},
		}},
		{"import", "import", reportView},
		{"import-default-binding", "import", defaultBindingView},
		{"inspect-pair", "snapshot", pair},
		{"inspect-snapshot", "snapshot", snapshotInspection{Snapshot: snapshot, CaptureHistory: captureHistory}},
		{"inspect-receipt", "receipt", receipt},
		{"inspect-report", "inspection", reportView},
		{"inspect-artifact", "artifact", artifact},
		{"compare", "comparison", struct {
			Comparison evidence.Comparison `json:"comparison"`
			Details    compare.Report      `json:"details"`
		}{comparison, details}},
		{"inspect-comparison", "comparison", struct {
			Comparison evidence.Comparison `json:"comparison"`
			Receipt    evidence.Receipt    `json:"receipt"`
			Details    *compare.Report     `json:"details,omitempty"`
		}{comparison, receipt, &details}},
		{"compare-command", "comparison", comparisonResult{Comparison: evidence.Comparison{SchemaVersion: 1, ID: goldenDigest("c"), Receipt: commandReceipt.ID, Outcome: evidence.Unstable, Completeness: evidence.Complete, Limits: []string{"recorded command samples only"}}, Receipt: commandReceipt, Details: &commandDetails}},
		{"run-command", "run", readableRunOutput{Status: "completed", Receipt: commandReceipt, Comparison: evidence.Comparison{SchemaVersion: 1, ID: goldenDigest("c"), Receipt: commandReceipt.ID, Outcome: evidence.Unstable, Completeness: evidence.Complete, Limits: []string{"recorded command samples only"}}, Details: commandDetails}},
		{"pin", "review", view},
		{"review", "review", view},
		{"run-preview", "execution_preview", executionPreview{
			Authorization: string(idD), Status: "authorization_required", Plan: plan, PlanBytes: len(plan),
			Consent: "Snapshots: base → candidate\nRuns: 2 sides × 2 cases × 1 repetition = 4 runs · concurrency 1",
			DockerSetup: dockerSetup{
				Problems:    []string{"Docker execution is not configured; set docker_binary and docker_host as a pair."},
				Suggestions: []string{`docker_binary: "/absolute/path/to/docker"`, `docker_host: "unix:///absolute/path/to/local/docker.sock"`},
			},
			Next: executionPreviewNext("authorization_required", string(idD)),
		}},
		{"run", "run", struct {
			Status        string              `json:"status"`
			Authorization string              `json:"authorization_digest"`
			Plan          json.RawMessage     `json:"plan"`
			Receipt       evidence.Receipt    `json:"receipt"`
			Comparison    evidence.Comparison `json:"comparison"`
			Details       compare.Report      `json:"details"`
			Samples       []runner.Sample     `json:"samples"`
		}{"incomplete", string(idD), plan, receipt, comparison, details, []runner.Sample{}}},
	}
}

func TestLegacySnapshotCaptureTimeRemainsUnavailable(t *testing.T) {
	state := goldenState()
	snapshot := evidence.Snapshot{SchemaVersion: 1, ID: goldenDigest("f"), Source: evidence.WorkingTree, Commit: strings.Repeat("f", 40), Files: []evidence.File{}, Completeness: evidence.Complete, Diff: goldenDigest("e"), Limits: []string{"safe"}}
	if err := writeReadable(state, "snapshot", snapshot); err != nil {
		t.Fatal(err)
	}
	if output := state.stdout.(*bytes.Buffer).String(); !strings.Contains(output, "Captured") || !strings.Contains(output, "legacy snapshot; no capture record") {
		t.Fatalf("legacy snapshot time was not labeled honestly: %q", output)
	}
}

func TestReadableCommandWitnessOverflowPointsToJSON(t *testing.T) {
	definition, report := goldenCommandPresentation()
	for index := 0; index < 100; index++ {
		report.Witnesses[0].Changes = append(report.Witnesses[0].Changes, compare.Change{Path: "/many", Kind: "changed", Before: json.RawMessage("1"), After: json.RawMessage("2")})
	}
	project := t.TempDir()
	receipt := commandGoldenReceipt(t, project, goldenReceipt(), definition)
	state := goldenState()
	state.project = project
	lines := comparisonDetailLines(state, nil, report, receipt)
	var output strings.Builder
	for _, line := range lines {
		output.WriteString(line.text)
		output.WriteByte('\n')
	}
	if !strings.Contains(output.String(), "More command witnesses omitted; use --json for the complete report.") {
		t.Fatalf("bounded command output did not point to its complete JSON report: %q", output.String())
	}
}

func TestReadableCommandEnumeration(t *testing.T) {
	state := &invocation{}
	got := map[string]bool{}
	for _, command := range commands(state) {
		got[command.Name] = true
	}
	want := map[string]bool{"capture": true, "diff": true, "import": true, "inspect": true, "compare": true, "export": true, "run": true, "pin": true, "review": true, "status": true, "log": true, "config": true, "version": true, "completion": true}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("command coverage changed: got %v want %v", got, want)
	}
	for _, command := range commands(state) {
		if command.Name == "version" || command.Name == "completion" {
			continue
		}
		if command.Before == nil {
			t.Errorf("%s does not select readable/JSON output", command.Name)
		}
		found := false
		for _, flag := range command.Flags {
			if flag.Names()[0] == "json" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s has no --json flag", command.Name)
		}
	}
}

func commandGoldenReceipt(t *testing.T, project string, receipt evidence.Receipt, definition runner.CommandDefinition) evidence.Receipt {
	t.Helper()
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	input, err := s.PutArtifact(raw, "command-definition", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := store.Put(s, evidence.Scenario{
		SchemaVersion: evidence.SchemaVersion, Input: input.Content, Driver: input.Content, Observer: input.Content, Rules: input.Content,
		Boundary: "synthetic command boundary", Author: "AFTER operator-selected command v1", Limits: []string{"synthetic, finite command example"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	receipt.Bindings = &evidence.Bindings{Scenario: scenario.ID, Input: input.Content, Driver: input.Content, Observer: input.Content, Rules: input.Content}
	return receipt
}

func withCommandGoldenReceipt(data any, receipt evidence.Receipt) any {
	switch value := data.(type) {
	case comparisonResult:
		value.Receipt = receipt
		return value
	case readableRunOutput:
		value.Receipt = receipt
		return value
	default:
		panic("command golden has an unsupported result type")
	}
}

func TestReadableOutputGoldens(t *testing.T) {
	for _, tc := range readableGoldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			state := goldenState()
			if strings.HasSuffix(tc.name, "-command") {
				definition, _ := goldenCommandPresentation()
				state.project = t.TempDir()
				tc.data = withCommandGoldenReceipt(tc.data, commandGoldenReceipt(t, state.project, goldenReceipt(), definition))
			}
			if err := writeReadable(state, tc.kind, tc.data); err != nil {
				t.Fatal(err)
			}
			got := state.stdout.(*bytes.Buffer).String()
			path := filepath.Join("testdata", tc.name+".golden")
			if *updateReadable {
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
				t.Fatalf("readable output differs: %s; regenerate explicitly with -update\n%s", path, got)
			}
			for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
				// Copyable Next commands are never clipped.
				if !strings.HasPrefix(line, "sha256:") && uniseg.StringWidth(line) > 80 && !strings.HasPrefix(line, "  after ") && !strings.HasPrefix(line, "  export ") {
					t.Errorf("golden exceeds 80 columns: %q", line)
				}
			}
		})
	}
}

func TestCommandHelpGoldens(t *testing.T) {
	for _, name := range helpNames() {
		t.Run(map[bool]string{true: "top-level", false: name}[name == ""], func(t *testing.T) {
			got := commandHelp(name)
			file := "help-" + name + ".golden"
			if name == "" {
				file = "help.golden"
			}
			path := filepath.Join("testdata", file)
			if *updateReadable {
				if err := os.WriteFile(path, []byte(got), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden %s; regenerate explicitly with -update\\n%s", path, got)
			}
			if string(want) != got {
				t.Fatalf("help differs from %s; regenerate explicitly with -update\\n%s", path, got)
			}
			for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
				if width := uniseg.StringWidth(line); width > 80 {
					t.Errorf("help exceeds 80 columns (%d): %q", width, line)
				}
			}
		})
	}
}

func TestHelpMatchesAvailableFlagsAndConfigurationDefaults(t *testing.T) {
	for _, command := range commands(&invocation{}) {
		if command.Name == "version" {
			continue
		}
		text := commandHelp(command.Name)
		for _, flag := range command.Flags {
			for _, name := range flag.Names() {
				spelling := "--" + name
				if len(name) == 1 {
					spelling = "-" + name
				}
				if !strings.Contains(text, spelling) {
					t.Errorf("%s flag %s is missing from help", command.Name, spelling)
				}
			}
		}
		if strings.Count(text, "Global\n") != 1 {
			t.Errorf("%s help must have one global options group", command.Name)
		}
		if command.Name != "run" && (strings.Contains(text, "--docker-") || strings.Contains(text, "--repetitions") || strings.Contains(text, "--run-seconds") || strings.Contains(text, "--output-bytes") || strings.Contains(text, "--interactive")) {
			t.Errorf("%s help advertises run-only configuration", command.Name)
		}
	}
	defaults := config.Defaults()
	runHelp := commandHelp("run")
	for _, value := range []any{defaults.Repetitions, defaults.RunSeconds, defaults.OutputBytes, defaults.Interactive} {
		if !strings.Contains(runHelp, "default: "+fmt.Sprint(value)) {
			t.Errorf("run help does not use configuration default %v", value)
		}
	}
	for _, name := range []string{"inspect", "export"} {
		text := commandHelp(name)
		for _, value := range []any{defaults.RawDiff, defaults.DiffBytes} {
			if !strings.Contains(text, "default: "+fmt.Sprint(value)) {
				t.Errorf("%s help does not use configuration default %v", name, value)
			}
		}
	}
	if text := topLevelHelp(); strings.Count(text, "Global options:") != 1 || !strings.Contains(text, "after status") || !strings.Contains(text, "after log") || !strings.Contains(text, "after diff") || !strings.Contains(text, "after completion") {
		t.Fatalf("top-level help omits current commands or repeats globals: %q", text)
	}
}

func TestVersionAndExportGoldens(t *testing.T) {
	cases := []struct {
		name  string
		write func(io.Writer) error
	}{
		{name: "version", write: func(writer io.Writer) error { _, err := fmt.Fprintf(writer, "after %s\n", Version); return err }},
		{name: "export", write: func(writer io.Writer) error {
			return writeJSON(&invocation{stdout: writer}, "export", struct {
				ID string `json:"id"`
			}{"fixture"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := tc.write(&output); err != nil {
				t.Fatal(err)
			}
			got := output.String()
			path := filepath.Join("testdata", tc.name+".golden")
			if *updateReadable {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden %s; regenerate explicitly with -update: %v", path, err)
			}
			if string(want) != got {
				t.Fatalf("output differs from %s: got %q", path, got)
			}
		})
	}
}

func TestReadableHostileContentColorClippingAndInspectIDs(t *testing.T) {
	longPath := "internal/" + strings.Repeat("hostile/", 16) + "main\x1b]52;c;clip\a.go"
	data := struct {
		Base      snapshotSummary  `json:"base_snapshot"`
		Candidate snapshotSummary  `json:"candidate_snapshot"`
		Index     *snapshotSummary `json:"index_snapshot,omitempty"`
	}{Base: snapshotSummary{ID: goldenDigest("a"), Source: evidence.Commit, Completeness: evidence.Complete, Files: 1, Limits: []string{"hostile\x1b[31m\nline"}}, Candidate: snapshotSummary{ID: goldenDigest("b"), Source: evidence.WorkingTree, Completeness: evidence.Complete, Files: 1, Limits: []string{}}}
	terminalState := goldenState()
	terminalState.theme.Color = true
	if err := writeReadable(terminalState, "capture", data); err != nil {
		t.Fatal(err)
	}
	colored := terminalState.stdout.(*bytes.Buffer).String()
	if !themeSGROnly(colored) || strings.ContainsAny(stripThemeSGR(colored), "\x1b\a") {
		t.Fatalf("untrusted terminal sequence leaked or non-theme escape emitted: %q", colored)
	}
	pipeState := &invocation{stdout: &bytes.Buffer{}, columns: 80, location: time.UTC, now: time.Now}
	if err := writeReadable(pipeState, "capture", data); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(pipeState.stdout.(*bytes.Buffer).String(), "\x1b\a") {
		t.Fatalf("pipe output contains terminal controls: %q", pipeState.stdout.(*bytes.Buffer).String())
	}
	pair := snapshotView{Base: goldenDigest("a"), Candidate: goldenDigest("b"), Inventory: []inventoryItem{{Path: longPath, Change: "modified"}}, InventoryTotal: 1, Limits: []string{}}
	hostilePipe := &invocation{stdout: &bytes.Buffer{}}
	if err := writeReadable(hostilePipe, "snapshot", pair); err != nil {
		t.Fatal(err)
	}
	hostileOutput := hostilePipe.stdout.(*bytes.Buffer).String()
	if strings.ContainsAny(hostileOutput, "\x1b\a") || !strings.Contains(hostileOutput, `\u001b]52;c;clip\u0007`) {
		t.Fatalf("hostile path was not rendered as safe text: %q", hostileOutput)
	}

	id := goldenDigest("f")
	snapshot := evidence.Snapshot{SchemaVersion: 1, ID: id, Source: evidence.WorkingTree, Commit: strings.Repeat("f", 40), Files: []evidence.File{}, Completeness: evidence.Complete, Diff: goldenDigest("e"), Limits: []string{"safe"}}
	wide := goldenState()
	wide.columns = 30
	if err := writeReadable(wide, "snapshot", snapshot); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wide.stdout.(*bytes.Buffer).String(), string(id)) {
		t.Fatalf("full inspection ID was clipped: %q", wide.stdout.(*bytes.Buffer).String())
	}

	long := strings.Repeat("x", 300)
	clipData := struct {
		Base      snapshotSummary  `json:"base_snapshot"`
		Candidate snapshotSummary  `json:"candidate_snapshot"`
		Index     *snapshotSummary `json:"index_snapshot,omitempty"`
	}{Base: snapshotSummary{ID: goldenDigest("a"), Source: evidence.Commit, Completeness: evidence.Complete, Limits: []string{}}, Candidate: snapshotSummary{ID: goldenDigest("b"), Source: evidence.WorkingTree, Completeness: evidence.Complete, Limits: []string{long}}}
	clipped := goldenState()
	if err := writeReadable(clipped, "capture", clipData); err != nil {
		t.Fatal(err)
	}
	pipe := &invocation{stdout: &bytes.Buffer{}}
	if err := writeReadable(pipe, "capture", clipData); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(clipped.stdout.(*bytes.Buffer).String(), long) || !strings.Contains(pipe.stdout.(*bytes.Buffer).String(), long) {
		t.Fatal("readable output clipping did not depend on stdout terminal state")
	}
}

func TestReadableSanitizesReportProducerExpectationAndErrorText(t *testing.T) {
	hostile := "hostile\x1b]52;c;payload\a\nnext"
	assertSafe := func(kind string, value any, visible string) {
		t.Helper()
		state := &invocation{stdout: &bytes.Buffer{}}
		if err := writeReadable(state, kind, value); err != nil {
			t.Fatal(err)
		}
		output := state.stdout.(*bytes.Buffer).String()
		if strings.ContainsAny(output, "\x1b\a\r") || !strings.Contains(output, visible) {
			t.Fatalf("%s did not safely retain hostile text: %q", kind, output)
		}
	}
	report := reportViewData{ID: goldenDigest("c"), Metadata: gotestreport.Metadata{Producer: hostile, ImportedAt: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)}, Cards: []gotestreport.Card{{Package: "example.invalid", Test: hostile, Scope: "test", State: evidence.EvidenceState{Report: evidence.ReportFail}}}}
	assertSafe("import", report, `\u001b]52;c;payload\u0007\u000anext`)
	pin := evidence.Pin{ID: goldenDigest("d"), Scenario: goldenDigest("e"), Expectation: hostile, BasisReceipt: goldenDigest("f"), BasisSnapshots: evidence.SnapshotPair{Base: goldenDigest("a"), Candidate: goldenDigest("b")}, Decision: evidence.Reopened, History: []evidence.DecisionEvent{{Decision: evidence.Reopened, At: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC), Reason: hostile}}}
	assertSafe("review", review.View{Pin: pin, Applicability: evidence.Stale, Reason: hostile, MissingCurrentResult: true}, `\u001b]52;c;payload\u0007\u000anext`)
	comparison, details := goldenComparison()
	comparison.Limits = []string{hostile}
	assertSafe("comparison", struct {
		Comparison evidence.Comparison `json:"comparison"`
		Details    compare.Report      `json:"details"`
	}{comparison, details}, `\u001b]52;c;payload\u0007\u000anext`)
	content := []byte(hostile)
	artifact := struct {
		ID     evidence.Digest `json:"id"`
		Offset int             `json:"offset"`
		Next   int             `json:"next"`
		Total  int             `json:"total"`
		More   bool            `json:"more"`
		Base64 string          `json:"base64"`
	}{ID: goldenDigest("e"), Next: len(content), Total: len(content), Base64: base64.StdEncoding.EncodeToString(content)}
	assertSafe("artifact", artifact, `\u001b]52;c;payload\u0007`)
}

func TestElapsedNoticeIsTTYStderrOnlyAndIndependentOfInputConsent(t *testing.T) {
	var terminalError bytes.Buffer
	state := &invocation{stderr: &terminalError, stderrTTY: true, tty: false}
	noticed := make(chan struct{}, 1)
	stop := startElapsedNoticeWith(state, "run", time.Millisecond, noticed)
	select {
	case <-noticed:
	case <-time.After(time.Second):
		t.Fatal("terminal elapsed notice did not fire")
	}
	stop()
	if output := terminalError.String(); !strings.Contains(output, "run still running") || strings.Contains(output, "%") {
		t.Fatalf("terminal elapsed notice is missing or percentage-like: %q", output)
	}
	var pipeError bytes.Buffer
	pipe := &invocation{stderr: &pipeError, stderrTTY: false, tty: true}
	stopPipe := startElapsedNotice(pipe, "run", time.Millisecond)
	stopPipe()
	if pipeError.Len() != 0 {
		t.Fatalf("pipe received a progress notice: %q", pipeError.String())
	}
}

func TestSnapshotCaptureMetadataDoesNotChangeJSONOutput(t *testing.T) {
	snapshot := evidence.Snapshot{SchemaVersion: 1, ID: goldenDigest("f"), Source: evidence.WorkingTree, Commit: strings.Repeat("f", 40), Files: []evidence.File{}, Completeness: evidence.Complete, Diff: goldenDigest("e"), Limits: []string{"safe"}}
	plain := &invocation{stdout: &bytes.Buffer{}, jsonOutput: true}
	if err := writeResult(plain, "snapshot", snapshot); err != nil {
		t.Fatal(err)
	}
	enriched := &invocation{stdout: &bytes.Buffer{}, jsonOutput: true}
	inspection := snapshotInspection{Snapshot: snapshot, CaptureHistory: snapshotCaptureHistory{Records: []store.CaptureSummary{{ID: goldenDigest("a"), CapturedAt: time.Date(2026, time.January, 2, 10, 30, 0, 0, time.UTC), Mode: evidence.WorkingTree}}}}
	if err := writeResult(enriched, "snapshot", inspection); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plain.stdout.(*bytes.Buffer).Bytes(), enriched.stdout.(*bytes.Buffer).Bytes()) {
		t.Fatalf("capture history changed the versioned JSON payload:\nplain %s\nenriched %s", plain.stdout.(*bytes.Buffer).Bytes(), enriched.stdout.(*bytes.Buffer).Bytes())
	}
}

func TestVersionedJSONRemainsExactAndExportAlwaysJSON(t *testing.T) {
	data := struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}{"fixture", 2}
	for _, tc := range []struct {
		name       string
		kind       string
		jsonOutput bool
		forceJSON  bool
	}{{"explicit", "capture", true, false}, {"export", "export", false, false}, {"export-record-kind", "artifact", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			state := &invocation{stdout: &bytes.Buffer{}, jsonOutput: tc.jsonOutput, forceJSON: tc.forceJSON}
			kind := tc.kind
			if err := writeResult(state, kind, data); err != nil {
				t.Fatal(err)
			}
			want := "{\"schema_version\":1,\"kind\":\"" + kind + "\",\"data\":{\"name\":\"fixture\",\"count\":2}}\n"
			if got := state.stdout.(*bytes.Buffer).String(); got != want {
				t.Fatalf("JSON envelope changed: got %q want %q", got, want)
			}
		})
	}
}

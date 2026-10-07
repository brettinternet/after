package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/store"
)

type cardCLIRecords struct {
	project         string
	receipt         evidence.Receipt
	comparison      evidence.Comparison
	pin             evidence.Pin
	report          evidence.Digest
	malformedReport evidence.Digest
	missing         evidence.Digest
}

func setupCardCLIRecords(t *testing.T) cardCLIRecords {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	put := func(channel, text string) evidence.Artifact {
		t.Helper()
		artifact, err := s.PutArtifact([]byte(text), channel, 4096)
		if err != nil {
			t.Fatal(err)
		}
		return artifact
	}
	baseSource := put("fixture", "base snapshot")
	candidateSource := put("fixture", "candidate snapshot")
	input := put("fixture", `{"epoch":1735689600,"seconds":[43200,30],"key":"key","body":{"amount_cents":1200,"currency":"USD"}}`)
	driver := put("fixture", "synthetic driver")
	observer := put("fixture", "synthetic observer")
	rules := put("fixture", "synthetic rules")
	authorization := put("fixture", "synthetic authorization")
	sample, err := s.PutArtifact([]byte("synthetic partial sample"), "sample", 5)
	if err != nil {
		t.Fatal(err)
	}
	base, err := store.Put(s, evidence.Snapshot{SchemaVersion: 1, Source: evidence.Commit, Commit: strings.Repeat("a", 40), Files: []evidence.File{}, Completeness: evidence.Complete, Diff: baseSource.Content, Limits: []string{"synthetic fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := store.Put(s, evidence.Snapshot{SchemaVersion: 1, Source: evidence.WorkingTree, Commit: strings.Repeat("b", 40), Files: []evidence.File{}, Completeness: evidence.Complete, Diff: candidateSource.Content, Limits: []string{"synthetic fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := store.Put(s, evidence.Scenario{SchemaVersion: 1, Input: input.Content, Driver: driver.Content, Observer: observer.Content, Rules: rules.Content, Boundary: "synthetic offline fixture", Author: "CLI test", Limits: []string{"fixture only; no project execution"}})
	if err != nil {
		t.Fatal(err)
	}
	environment := &evidence.Environment{Environment: input.Content, Toolchain: driver.Content, Dependencies: rules.Content, Argv: []string{"synthetic"}}
	at := time.Date(2026, time.January, 2, 10, 30, 0, 0, time.UTC)
	receipt, err := store.Put(s, evidence.Receipt{
		SchemaVersion:   1,
		State:           evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.Observed, Applicability: evidence.Current, Execution: evidence.Completed, Comparison: evidence.Incomparable, Report: evidence.NoReport},
		Snapshots:       evidence.SnapshotPair{Base: base.ID, Candidate: candidate.ID},
		Bindings:        &evidence.Bindings{Scenario: scenario.ID, Input: input.Content, Driver: driver.Content, Observer: observer.Content, Rules: rules.Content},
		BaseEnvironment: environment, CandidateEnvironment: environment, Authorization: authorization.Content,
		StartedAt: at, FinishedAt: at.Add(time.Minute), Completeness: evidence.Incomplete,
		Artifacts: []evidence.Artifact{sample}, Limits: []string{"synthetic; no project execution"},
	})
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := store.Put(s, evidence.Comparison{SchemaVersion: 1, Receipt: receipt.ID, Outcome: evidence.Incomparable, Completeness: evidence.Incomplete, Limits: []string{"synthetic; incomplete fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	pin, err := store.Put(s, evidence.Pin{
		SchemaVersion: 1, Scenario: scenario.ID, Expectation: "Preserve one finite example", BasisReceipt: receipt.ID,
		BasisSnapshots: receipt.Snapshots, Decision: evidence.Pinned, Scope: evidence.HumanIntent,
		History: []evidence.DecisionEvent{{Decision: evidence.Pinned, At: at, Reason: "synthetic fixture", Review: &evidence.ReviewContext{Action: "pin", Mode: evidence.OriginalBase, PriorCandidate: candidate.ID, Target: evidence.BasisOf(receipt), Receipt: receipt.ID}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	rawReport := "{\"Time\":\"2026-01-02T10:30:00Z\",\"Action\":\"run\",\"Package\":\"example.invalid/cart\",\"Test\":\"TestRetry\"}\n" +
		"{\"Time\":\"2026-01-02T10:30:01Z\",\"Action\":\"output\",\"Package\":\"example.invalid/cart\",\"Test\":\"TestRetry\",\"Output\":\"synthetic output\\n\"}\n" +
		"{\"Time\":\"2026-01-02T10:30:02Z\",\"Action\":\"pass\",\"Package\":\"example.invalid/cart\",\"Test\":\"TestRetry\"}\n"
	report, err := gotestreport.Import(strings.NewReader(rawReport), gotestreport.Metadata{Producer: "go test JSON fixture", Snapshot: candidate.ID, ImportedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	reportBytes, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	reportArtifact, err := s.PutArtifact(reportBytes, "report", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	malformedReport, err := s.PutArtifact([]byte(`{"schema_version":1,"unexpected":"raw report bytes"}`), "go-test-report-v1", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return cardCLIRecords{project: project, receipt: receipt, comparison: comparison, pin: pin, report: reportArtifact.Content, malformedReport: malformedReport.Content, missing: evidence.Digest("sha256:" + strings.Repeat("f", 64))}
}

func TestCLIStoredCardOutputGoldens(t *testing.T) {
	records := setupCardCLIRecords(t)
	cases := []struct {
		name string
		args []string
		exit int
	}{
		{"receipt", []string{"inspect", string(records.receipt.ID), "--project", records.project}, ExitOK},
		{"comparison", []string{"inspect", string(records.comparison.ID), "--project", records.project}, ExitOperational},
		{"pin", []string{"pin", string(records.pin.ID), "--project", records.project}, ExitOK},
		{"report", []string{"inspect", string(records.report), "--project", records.project}, ExitOK},
		{"unavailable", []string{"inspect", string(records.missing), "--project", records.project}, ExitOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, got, stderr := invoke(tc.args, false, "")
			got = strings.ReplaceAll(got, records.project, "<project>")
			if code != tc.exit || stderr != "" {
				t.Fatalf("CLI Card inspection failed: exit=%d want=%d stderr=%q stdout=%q", code, tc.exit, stderr, got)
			}
			path := filepath.Join("testdata", "cards", tc.name+".golden")
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
				t.Fatalf("missing Card golden %s; regenerate explicitly with -update\n%s", path, got)
			}
			if string(want) != got {
				t.Fatalf("CLI Card output differs: %s; regenerate explicitly with -update\n%s", path, got)
			}
		})
	}
}

func TestCLIInspectMalformedReportUsesRawCard(t *testing.T) {
	records := setupCardCLIRecords(t)
	code, output, stderr := invoke([]string{"inspect", string(records.malformedReport), "--project", records.project}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(output, "Raw content\n") || !strings.Contains(output, `"unexpected":"raw report bytes"`) {
		t.Fatalf("malformed report did not retain the full raw Card: exit=%d stderr=%q output=%q", code, stderr, output)
	}
}

func TestCLIInspectMalformedComparisonDetailUsesRawCard(t *testing.T) {
	records := setupCardCLIRecords(t)
	s, err := store.Open(records.project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := store.Get[evidence.Comparison](s, records.comparison.ID)
	if err != nil {
		t.Fatal(err)
	}
	badRaw := []byte(`{"version":1,"unexpected":"raw comparison detail bytes"}`)
	badDetails, err := s.PutArtifact(badRaw, "comparison-details-v1", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	comparison.ID = ""
	comparison.Details = &badDetails
	comparison, err = store.Put(s, comparison)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	code, output, stderr := invoke([]string{"inspect", string(comparison.ID), "--project", records.project}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(output, "Raw content\n") || !strings.Contains(output, "raw comparison detail bytes") {
		t.Fatalf("malformed comparison did not retain full raw Card content: exit=%d stderr=%q output=%q", code, stderr, output)
	}
}

func TestCLIInspectJSONContractDoesNotExposeCardRenderer(t *testing.T) {
	records := setupCardCLIRecords(t)
	code, got, stderr := invoke([]string{"inspect", string(records.receipt.ID), "--project", records.project, "--json"}, false, "")
	data, err := json.Marshal(records.receipt)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte(`{"schema_version":1,"kind":"receipt","data":`), data...)
	want = append(want, '}', '\n')
	if code != ExitOK || stderr != "" || !bytes.Equal([]byte(got), want) {
		t.Fatalf("receipt JSON inspection contract changed:\n got %s\nwant %s\nerr %s", got, want, stderr)
	}
	code, got, stderr = invoke([]string{"inspect", string(records.comparison.ID), "--project", records.project, "--json"}, false, "")
	if code != ExitOperational || stderr != "" || bytes.Contains([]byte(got), []byte("inspect_card")) {
		t.Fatalf("comparison JSON inspection contract changed: %d %q %q", code, got, stderr)
	}
	var envelope struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Data          struct {
			Comparison evidence.Comparison `json:"comparison"`
			Receipt    evidence.Receipt    `json:"receipt"`
			Snapshots  json.RawMessage     `json:"snapshots"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(got), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != 1 || envelope.Kind != "comparison" || !reflect.DeepEqual(envelope.Data.Comparison, records.comparison) || !reflect.DeepEqual(envelope.Data.Receipt, records.receipt) || len(envelope.Data.Snapshots) == 0 {
		t.Fatalf("comparison JSON fields changed: %+v", envelope)
	}
}

func TestCLIStoredCardOutputContainsFullIDs(t *testing.T) {
	records := setupCardCLIRecords(t)
	for _, args := range [][]string{{"inspect", string(records.receipt.ID), "--project", records.project}, {"pin", string(records.pin.ID), "--project", records.project}} {
		code, output, stderr := invoke(args, false, "")
		if code != ExitOK || stderr != "" || !strings.Contains(output, "IDs\n") || !strings.Contains(output, string(records.receipt.ID)) {
			t.Fatalf("Card inspection omitted receipt IDs: %d %q %q", code, output, stderr)
		}
	}
}

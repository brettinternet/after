package compare

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/store"
)

func TestPolicyDigestAndReviewableDiff(t *testing.T) {
	s, _ := setup(t)
	original, err := s.PutArtifact([]byte(runner.ComparisonRules), "comparison-rules", 4096)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{
		strings.Replace(runner.ComparisonRules, `"masks":[]`, `"masks":["/**"]`, 1),
		strings.Replace(runner.ComparisonRules, `"normalization":[]`, `"normalization":["remove-provider-calls"]`, 1),
	} {
		changed, err := s.PutArtifact([]byte(policy), "comparison-rules", 4096)
		if err != nil {
			t.Fatal(err)
		}
		if changed.Content == original.Content {
			t.Fatal("changed policy reused identity")
		}
		before, err := s.ReadBlob(original.Content)
		if err != nil {
			t.Fatal(err)
		}
		after, err := s.ReadBlob(changed.Content)
		if err != nil {
			t.Fatal(err)
		}
		diffs, err := JSON(before, after)
		if err != nil || len(diffs) != 1 || diffs[0].Kind != "added" {
			t.Fatalf("policy change hidden: %+v %v", diffs, err)
		}
	}
}
func TestNewRedactionPolicyCannotPublishEquality(t *testing.T) {
	dir := t.TempDir()
	s, pair := setupAt(t, dir)
	r := receipt(t, s, pair, func(o *runner.Observation, side string, _ int) {
		if side == "candidate" {
			o.Responses[0].Body = `{"private":"private-value"}`
		}
	}, nil)
	r = put(t, s, r)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir, true, []string{"private-value"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := Run(s, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Outcome != evidence.Incomparable || c.Completeness != evidence.Incomplete || !c.Details.Redacted {
		t.Fatal("redacted detail promoted", c)
	}
	b, err := s.ReadBlob(c.Details.Content)
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err = json.Unmarshal(b, &report); err != nil {
		t.Fatal(err)
	}
	if report.Outcome != evidence.Incomparable || strings.Contains(string(b), "private-value") {
		t.Fatal("misleading or unredacted detail")
	}
}
func TestReportedEvidenceRemainsNotObserved(t *testing.T) {
	s, pair := setup(t)
	r := receipt(t, s, pair, nil, nil)
	r.State = evidence.EvidenceState{Producer: evidence.Importer, Kind: evidence.Reported, Applicability: evidence.Unknown, Execution: evidence.NotRun, Comparison: evidence.NotCompared, Report: evidence.ReportPass}
	r.RequestID = ""
	r.Bindings = nil
	r.BaseEnvironment = nil
	r.CandidateEnvironment = nil
	r.Authorization = ""
	c, _ := result(t, s, r)
	if c.Outcome != evidence.Incomparable {
		t.Fatal("report promoted")
	}
}

package compare

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

func put[T store.Record](t *testing.T, s *store.Store, r T) T {
	t.Helper()
	got, err := store.Put(s, r)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
func artifact(t *testing.T, s *store.Store, v any, channel string) evidence.Artifact {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.PutArtifact(b, channel, maxJSON)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func setup(t *testing.T) (*store.Store, evidence.SnapshotPair) {
	return setupAt(t, t.TempDir())
}

func setupAt(t *testing.T, dir string) (*store.Store, evidence.SnapshotPair) {
	t.Helper()
	s, err := store.Open(dir, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	diff, err := s.PutArtifact([]byte("synthetic source inventory diff"), "raw-diff", 4096)
	if err != nil {
		t.Fatal(err)
	}
	var pair evidence.SnapshotPair
	for _, side := range []string{"base", "candidate"} {
		files := []evidence.File{}
		for _, name := range []string{"go.mod", "app/main.go", "app/config.go"} {
			b, err := os.ReadFile("../paymentfixture/testdata/payment/" + name)
			if err != nil {
				t.Fatal(err)
			}
			if side == "candidate" && name == "app/config.go" {
				b = []byte(strings.Replace(string(b), "24 * 60 * 60", "5 * 60", 1))
			}
			a, err := s.PutArtifact(b, "source", maxJSON)
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, evidence.File{Path: name, Content: a.Content, Mode: "100644"})
		}
		snap := put(t, s, evidence.Snapshot{SchemaVersion: 1, Source: evidence.WorkingTree, Unborn: true, Files: files, Completeness: evidence.Complete, Diff: diff.Content})
		if side == "base" {
			pair.Base = snap.ID
		} else {
			pair.Candidate = snap.ID
		}
	}
	return s, pair
}

// Synthetic protocol records exercise the comparator without claiming actual
// execution. TestComparisonProof below separately uses actual Docker observations.
func receipt(t *testing.T, s *store.Store, pair evidence.SnapshotPair, change func(*runner.Observation, string, int), mutate func(*runner.Sample)) evidence.Receipt {
	t.Helper()
	p, err := runner.Prepare(s, pair, 2, sandbox.Limits{Seconds: 180, OutputBytes: 65536})
	if err != nil {
		t.Fatal(err)
	}
	preview, _ := p.Preview()
	var plans struct {
		Experiments [2][2]struct{ App, Observer json.RawMessage }
	}
	if err := json.Unmarshal(preview, &plans); err != nil {
		t.Fatal(err)
	}
	planID := func(raw json.RawMessage) string {
		b, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("sha256:%x", sha256.Sum256(b))
	}
	denied, err := (runner.Executor{}).Run(t.Context(), s, p, "not approved")
	if err == nil {
		t.Fatal("expected denial")
	}
	r := denied.Receipt
	r.ID = ""
	r.Completeness = evidence.Complete
	r.State = evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.Observed, Execution: evidence.Completed, Applicability: evidence.Current, Comparison: evidence.NotCompared, Report: evidence.NoReport}
	r.Artifacts = r.Artifacts[:2]
	for rep := 0; rep < 2; rep++ {
		for sideIndex, side := range []string{"base", "candidate"} {
			for caseIndex, sec := range []int64{43200, 30} {
				prefix := fmt.Sprintf("%s/%d/%d/", side, sec, rep)
				o := runner.Observation{Version: 1, Seconds: sec, Responses: []runner.Response{{Status: 200, Body: `{"payment":"synthetic-accepted"}`}, {Status: 200, Body: `{"payment":"synthetic-accepted"}`}}, Calls: []runner.Call{{At: 1735689600, Method: "POST", Path: "/charges", Key: "synthetic-key-a", Body: `{"amount_cents":1200,"currency":"USD"}`}}}
				if side == "candidate" && sec == 43200 {
					call := o.Calls[0]
					call.At += sec
					o.Calls = append(o.Calls, call)
				}
				if change != nil {
					change(&o, side, rep)
				}
				a := artifact(t, s, o, prefix+"observation")
				m := runner.Sample{RequestID: r.RequestID, Snapshots: pair, Side: side, CaseSeconds: sec, Repetition: rep, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Status: "completed", Execution: sandbox.ExperimentResult{App: sandbox.Result{Cleaned: true}, Observer: sandbox.Result{Cleaned: true}}, Artifacts: []evidence.Artifact{a}}
				m.Execution.App.Plan = planID(plans.Experiments[sideIndex][caseIndex].App)
				m.Execution.Observer.Plan = planID(plans.Experiments[sideIndex][caseIndex].Observer)
				if mutate != nil {
					mutate(&m)
				}
				r.Artifacts = append(r.Artifacts, a, artifact(t, s, m, prefix+"sample"))
			}
		}
	}
	return r
}
func result(t *testing.T, s *store.Store, r evidence.Receipt) (evidence.Comparison, Report) {
	t.Helper()
	r.ID = ""
	r = put(t, s, r)
	c, err := Run(s, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Get[evidence.Comparison](s, c.ID)
	if err != nil || got.ID != c.ID {
		t.Fatal("comparison persistence", err)
	}
	b, err := s.ReadBlob(c.Details.Content)
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err = json.Unmarshal(b, &report); err != nil {
		t.Fatal(err)
	}
	if report.Receipt != r.ID || report.Snapshots != r.Snapshots {
		t.Fatal("lost source inventory bindings")
	}
	return c, report
}
func checkPayment(t *testing.T, c evidence.Comparison, r Report) {
	t.Helper()
	if c.Outcome != evidence.Different || c.Completeness != evidence.Complete {
		t.Fatalf("payment: %+v", c)
	}
	changed := 0
	for _, w := range r.Witnesses {
		if w.Before.Metadata == "" || w.Before.Observation == "" || w.After.Observation == "" {
			t.Fatal("unlinked witness")
		}
		if w.Outcome == evidence.Different {
			changed++
			if w.Relation != "paired" || w.Channel != "provider" || w.Before.Seconds != 43200 {
				t.Fatalf("wrong changed channel: %+v", w)
			}
			found := false
			for _, d := range w.Changes {
				if d.Path == "/count" && string(d.Before) == "1" && string(d.After) == "2" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing exact count witness")
			}
		}
	}
	if changed != 2 || len(r.Witnesses) != 16 {
		t.Fatalf("lost samples: %d %d", changed, len(r.Witnesses))
	}
}
func TestPaymentAndDeterminism(t *testing.T) {
	s, pair := setup(t)
	r := receipt(t, s, pair, nil, nil)
	c, report := result(t, s, r)
	checkPayment(t, c, report)
	again, err := Run(s, c.Receipt)
	if err != nil || again.ID != c.ID {
		t.Fatal("nondeterministic persisted comparison", err)
	}
	for _, w := range report.Witnesses {
		for _, id := range []evidence.Digest{w.Before.Metadata, w.After.Metadata, w.Before.Observation, w.After.Observation} {
			if _, err := s.ReadBlob(id); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, id := range []evidence.Digest{report.Snapshots.Base, report.Snapshots.Candidate} {
		snap, err := store.Get[evidence.Snapshot](s, id)
		if err != nil || len(snap.Files) != 3 {
			t.Fatal("missing inventory", err)
		}
		if _, err := s.ReadBlob(snap.Diff); err != nil {
			t.Fatal(err)
		}
	}
}
func TestRepetitionsAndExactPayloads(t *testing.T) {
	for _, mode := range []string{"equal", "unstable", "key-order", "numeric", "operation", "destination", "payload", "order"} {
		t.Run(mode, func(t *testing.T) {
			s, pair := setup(t)
			r := receipt(t, s, pair, func(o *runner.Observation, side string, rep int) {
				o.Calls = o.Calls[:1]
				switch mode {
				case "unstable":
					if rep == 1 && side == "candidate" {
						o.Responses[0].Body = `{"changed":true}`
					}
				case "key-order":
					o.Responses[0].Body = `{"a":1,"b":2}`
					if side == "candidate" {
						o.Responses[0].Body = `{"b":2.0,"a":1e0}`
					}
				case "numeric":
					o.Responses[0].Body = `{"n":9007199254740992}`
					if side == "candidate" {
						o.Responses[0].Body = `{"n":9007199254740993}`
					}
				case "operation":
					if side == "candidate" {
						o.Calls[0].Method = "DELETE"
					}
				case "destination":
					if side == "candidate" {
						o.Calls[0].Path = "/other"
					}
				case "payload":
					if side == "candidate" {
						o.Calls[0].Body = `{"amount_cents":9007199254740993}`
					}
				case "order":
					call := o.Calls[0]
					call.Key = "second"
					o.Calls = append(o.Calls, call)
					if side == "candidate" {
						o.Calls[0], o.Calls[1] = o.Calls[1], o.Calls[0]
					}
				}
			}, nil)
			c, report := result(t, s, r)
			want := evidence.Different
			if mode == "equal" || mode == "key-order" {
				want = evidence.Equal
			}
			if mode == "unstable" {
				want = evidence.Unstable
			}
			if c.Outcome != want || len(report.Witnesses) != 16 {
				t.Fatalf("%s: %+v", mode, c)
			}
			if mode == "numeric" {
				b, _ := s.ReadBlob(c.Details.Content)
				if !strings.Contains(string(b), `"after":9007199254740993`) {
					t.Fatal("precision lost in persisted witness")
				}
			}
		})
	}
}
func TestIncomparable(t *testing.T) {
	for _, mode := range []string{"no-data", "failed", "missing", "missing-metadata", "redacted", "truncated", "environment", "toolchain", "dependencies", "argv", "mask", "normalization", "observer", "input", "driver", "new-channel", "duplicate", "protocol", "duplicate-json", "bad-status", "late-binding", "metadata-artifact", "case", "time", "plan", "missing-policy"} {
		t.Run(mode, func(t *testing.T) {
			s, pair := setup(t)
			r := receipt(t, s, pair, func(o *runner.Observation, side string, rep int) {
				if side != "candidate" {
					return
				}
				switch mode {
				case "protocol":
					o.Version = 2
				case "duplicate-json":
					o.Responses[0].Body = `{"a":1,"a":2}`
				case "case":
					o.Seconds = 99
				}
			}, func(m *runner.Sample) {
				if m.Side != "candidate" {
					return
				}
				switch mode {
				case "bad-status":
					m.Status = "timeout"
				case "late-binding":
					m.RequestID = pair.Base
				case "metadata-artifact":
					m.Artifacts = nil
				case "time":
					m.FinishedAt = m.FinishedAt.Add(time.Hour)
				}
			})
			switch mode {
			case "no-data":
				r.Artifacts = nil
				r.Completeness = evidence.Incomplete
				r.State.Kind = evidence.NoEvidence
				r.State.Execution = evidence.NotRun
				r.State.Applicability = evidence.Unknown
			case "failed":
				r.Completeness = evidence.Incomplete
				r.State.Kind = evidence.NoEvidence
				r.State.Execution = evidence.Failed
				r.State.Applicability = evidence.Unknown
			case "missing":
				r.Artifacts = append(r.Artifacts[:2], r.Artifacts[3:]...)
			case "missing-metadata":
				r.Artifacts = append(r.Artifacts[:3], r.Artifacts[4:]...)
			case "redacted", "truncated":
				r.Completeness = evidence.Incomplete
				if mode == "redacted" {
					r.Redacted = true
					r.RedactionPolicy = "literal-v1"
				} else {
					a, err := s.PutArtifact([]byte("too long"), "truncated", 2)
					if err != nil {
						t.Fatal(err)
					}
					r.Artifacts = append(r.Artifacts, a)
				}
			case "environment":
				r.CandidateEnvironment.Environment = pair.Base
			case "toolchain":
				r.CandidateEnvironment.Toolchain = pair.Base
			case "dependencies":
				r.CandidateEnvironment.Dependencies = pair.Base
			case "argv":
				r.CandidateEnvironment.Argv = []string{"/different"}
			case "mask", "normalization", "observer", "input", "driver":
				scenario, err := store.Get[evidence.Scenario](s, r.Bindings.Scenario)
				if err != nil {
					t.Fatal(err)
				}
				scenario.ID = ""
				switch mode {
				case "mask", "normalization":
					text := strings.Replace(runner.ComparisonRules, `"`+mode+`s":[]`, `"`+mode+`s":["/**"]`, 1)
					if mode == "normalization" {
						text = strings.Replace(runner.ComparisonRules, `"normalization":[]`, `"normalization":["drop-all"]`, 1)
					}
					a, err := s.PutArtifact([]byte(text), "comparison-rules", 4096)
					if err != nil {
						t.Fatal(err)
					}
					scenario.Rules = a.Content
					r.Artifacts[1] = a
				case "observer":
					scenario.Observer = pair.Base
				case "input":
					scenario.Input = r.Artifacts[0].Content
				case "driver":
					scenario.Driver = pair.Base
				}
				scenario = put(t, s, scenario)
				r.Bindings = &evidence.Bindings{Scenario: scenario.ID, Input: scenario.Input, Driver: scenario.Driver, Observer: scenario.Observer, Rules: scenario.Rules}
			case "new-channel":
				r.Artifacts = append(r.Artifacts, artifact(t, s, []string{}, "new-capability"))
			case "duplicate":
				r.Artifacts = append(r.Artifacts, r.Artifacts[2])
			case "plan":
				r.Authorization = pair.Base
			case "missing-policy":
				r.Artifacts = append(r.Artifacts[:1], r.Artifacts[2:]...)
			}
			c, report := result(t, s, r)
			if c.Outcome != evidence.Incomparable || c.Completeness != evidence.Incomplete || len(report.Limits) <= len(scope) {
				t.Fatalf("%s promoted: %+v", mode, c)
			}
		})
	}
}
func TestComparisonProof(t *testing.T) {
	if os.Getenv("AFTER_COMPARISON_PROOF") != "1" {
		t.Skip("task comparison:proof authorizes synthetic Docker execution")
	}
	s, pair := setup(t)
	p, err := runner.Prepare(s, pair, 2, sandbox.Limits{Seconds: 180, OutputBytes: 65536})
	if err != nil {
		t.Fatal(err)
	}
	_, id := p.Preview()
	run, err := (runner.Executor{Docker: sandbox.Docker{Binary: os.Getenv("AFTER_DOCKER_BINARY"), Host: os.Getenv("AFTER_DOCKER_HOST")}}).Run(t.Context(), s, p, id)
	if err != nil {
		t.Fatal(err)
	}
	c, report := result(t, s, run.Receipt)
	checkPayment(t, c, report)
	t.Logf("real paired responses equal; 12h provider 1/2; 30s provider 1/1; two repetitions; %d artifact-linked channel witnesses", len(report.Witnesses))
}

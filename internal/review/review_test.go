package review

import (
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

func put[T store.Record](t *testing.T, s *store.Store, r T) T {
	t.Helper()
	v, err := store.Put(s, r)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func fixture(t *testing.T) (*store.Store, string, evidence.Receipt) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(dir, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a, err := s.PutArtifact([]byte("synthetic unit-test observation, not real execution evidence"), "synthetic", 1024)
	if err != nil {
		t.Fatal(err)
	}
	snap := put(t, s, evidence.Snapshot{SchemaVersion: 1, Source: evidence.WorkingTree, Commit: strings.Repeat("a", 40), Files: []evidence.File{{Path: "app.go", Mode: "100644", Content: a.Content}}, Completeness: evidence.Complete, Diff: a.Content})
	scenario := put(t, s, evidence.Scenario{SchemaVersion: 1, Input: a.Content, Driver: a.Content, Observer: a.Content, Rules: a.Content, Boundary: "synthetic finite case", Author: "unit test", Limits: []string{"synthetic only"}})
	env := &evidence.Environment{Environment: a.Content, Toolchain: a.Content, Dependencies: a.Content, Argv: []string{"synthetic"}}
	r := put(t, s, evidence.Receipt{SchemaVersion: 1, State: evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.Observed, Applicability: evidence.Current, Execution: evidence.Completed, Comparison: evidence.NotCompared, Report: evidence.NoReport}, Snapshots: evidence.SnapshotPair{Base: snap.ID, Candidate: snap.ID}, Bindings: &evidence.Bindings{Scenario: scenario.ID, Input: scenario.Input, Driver: scenario.Driver, Observer: scenario.Observer, Rules: scenario.Rules}, BaseEnvironment: env, CandidateEnvironment: env, Authorization: a.Content, StartedAt: time.Unix(1, 0).UTC(), FinishedAt: time.Unix(2, 0).UTC(), Completeness: evidence.Complete, Artifacts: []evidence.Artifact{a}, Limits: []string{"synthetic only"}})
	return s, dir, r
}
func pin(t *testing.T, s *store.Store, r evidence.Receipt, scope evidence.PinScope) evidence.Pin {
	t.Helper()
	p, err := Create(s, r.ID, "same response and at most one provider call", scope, "human expectation")
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func view(t *testing.T, s *store.Store, p evidence.Pin) View {
	t.Helper()
	v, err := Inspect(s, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func changedSnapshot(t *testing.T, s *store.Store, id evidence.Digest, kind string) evidence.Digest {
	t.Helper()
	snap, err := store.Get[evidence.Snapshot](s, id)
	if err != nil {
		t.Fatal(err)
	}
	snap.ID = ""
	switch kind {
	case "incomplete":
		snap.Completeness = evidence.Incomplete
		snap.Limits = []string{"unknown footprint"}
	case "excluded":
		snap.Excluded = []evidence.Limitation{{Path: "excluded.go", Reason: "not captured"}}
	default:
		a, err := s.PutArtifact([]byte(kind), "synthetic-change", 1024)
		if err != nil {
			t.Fatal(err)
		}
		snap.Files = append(snap.Files, evidence.File{Path: kind, Content: a.Content, Mode: "100644"})
	}
	return put(t, s, snap).ID
}
func cloneBasis(b evidence.ReviewBasis) evidence.ReviewBasis {
	raw, _ := json.Marshal(b)
	var out evidence.ReviewBasis
	_ = json.Unmarshal(raw, &out)
	return out
}

func TestInvalidationMatrix(t *testing.T) {
	for _, kind := range []string{"code", "harmless-readme", "fixture", "driver", "observer", "runtime", "dependency", "environment", "argv", "rules", "mask", "scenario-title", "renamed-test", "incomplete", "excluded", "unknown-dependency"} {
		t.Run(kind, func(t *testing.T) {
			s, _, r := fixture(t)
			p := pin(t, s, r, evidence.FiniteExample)
			p, err := Accept(s, p.ID, "reviewed example")
			if err != nil {
				t.Fatal(err)
			}
			target := cloneBasis(evidence.BasisOf(r))
			alt, err := s.PutArtifact([]byte("different"), "synthetic", 1024)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "code", "harmless-readme", "renamed-test", "incomplete", "excluded":
				target.Snapshots.Candidate = changedSnapshot(t, s, target.Snapshots.Candidate, kind)
			case "runtime":
				target.CandidateEnvironment.Toolchain = alt.Content
			case "dependency":
				target.CandidateEnvironment.Dependencies = alt.Content
			case "environment":
				target.BaseEnvironment.Environment = alt.Content
			case "argv":
				target.BaseEnvironment.Argv = []string{"changed"}
			case "unknown-dependency":
				target.CandidateEnvironment = nil
			default:
				scenario, err := store.Get[evidence.Scenario](s, r.Bindings.Scenario)
				if err != nil {
					t.Fatal(err)
				}
				scenario.ID = ""
				switch kind {
				case "fixture":
					scenario.Input = alt.Content
				case "driver":
					scenario.Driver = alt.Content
				case "observer":
					scenario.Observer = alt.Content
				case "rules", "mask":
					scenario.Rules = alt.Content
				case "scenario-title":
					scenario.Boundary = "renamed scenario title"
				}
				scenario = put(t, s, scenario)
				target.Bindings = &evidence.Bindings{Scenario: scenario.ID, Input: scenario.Input, Driver: scenario.Driver, Observer: scenario.Observer, Rules: scenario.Rules}
			}
			next, err := Select(s, p.ID, target, evidence.OriginalBase, "explicitly inspect new basis")
			if err != nil {
				t.Fatal(err)
			}
			v := view(t, s, next)
			want := evidence.Stale
			if kind == "incomplete" || kind == "excluded" || kind == "unknown-dependency" {
				want = evidence.Unknown
			}
			if v.Applicability != want || !v.MissingCurrentResult || v.CurrentReceipt != nil || next.Decision != evidence.Reopened || next.Expectation != p.Expectation || next.BasisReceipt != p.BasisReceipt || len(next.History) != len(p.History)+1 || v.Reason == "" {
				t.Fatalf("bad reopening: %+v", v)
			}
			if _, err := Accept(s, next.ID, "blind acceptance"); err == nil {
				t.Fatal("accepted missing result")
			}
			if _, err := Attach(s, next.ID, r.ID, "late old result"); err == nil {
				t.Fatal("late result migrated")
			}
			old, err := store.Get[evidence.Receipt](s, r.ID)
			if err != nil || !reflect.DeepEqual(old, r) {
				t.Fatal("receipt changed", err)
			}
			if kind == "harmless-readme" && !strings.Contains(v.Reason, "harmless") {
				t.Fatal(v.Reason)
			}
		})
	}
}

func TestRerunHistoryAndSeparateAcceptance(t *testing.T) {
	s, _, r := fixture(t)
	p := pin(t, s, r, evidence.HumanIntent)
	p, err := Accept(s, p.ID, "human intent, not a proof")
	if err != nil {
		t.Fatal(err)
	}
	target := cloneBasis(evidence.BasisOf(r))
	target.Snapshots.Candidate = changedSnapshot(t, s, r.Snapshots.Candidate, "new.go")
	next, err := Select(s, p.ID, target, evidence.OriginalBase, "new snapshot")
	if err != nil {
		t.Fatal(err)
	}
	rerun := r
	rerun.ID = ""
	rerun.Snapshots = target.Snapshots
	rerun.FinishedAt = rerun.FinishedAt.Add(time.Second)
	rerun = put(t, s, rerun)
	attached, err := Attach(s, next.ID, rerun.ID, "separately authorized synthetic test receipt")
	if err != nil {
		t.Fatal(err)
	}
	v := view(t, s, attached)
	if attached.Decision != evidence.Reopened || v.Applicability != evidence.Current || v.MissingCurrentResult || v.CurrentReceipt.ID != rerun.ID || attached.BasisReceipt != r.ID || attached.Expectation != p.Expectation {
		t.Fatalf("rerun accepted or migrated basis: %+v", v)
	}
	accepted, err := Accept(s, attached.ID, "human explicitly reviewed again")
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Decision != evidence.Accepted || len(accepted.History) != 5 || view(t, s, next).CurrentReceipt != nil || view(t, s, p).CurrentReceipt.ID != r.ID {
		t.Fatal("history overwritten")
	}
	same, err := Select(s, accepted.ID, target, evidence.OriginalBase, "identical complete selection")
	if err != nil {
		t.Fatal(err)
	}
	if same.Decision != evidence.Accepted || view(t, s, same).Applicability != evidence.Current {
		t.Fatal("bounded reuse failed")
	}
	if again, err := Attach(s, same.ID, rerun.ID, "same reviewed receipt"); err != nil || again.Decision != evidence.Accepted {
		t.Fatal("reattaching the accepted receipt changed the decision", err)
	}
	fresh := rerun
	fresh.ID = ""
	fresh.FinishedAt = fresh.FinishedAt.Add(time.Second)
	fresh = put(t, s, fresh)
	unreviewed, err := Attach(s, same.ID, fresh.ID, "fresh identical-basis rerun")
	if err != nil {
		t.Fatal(err)
	}
	if unreviewed.Decision != evidence.Reopened || view(t, s, unreviewed).CurrentReceipt.ID != fresh.ID {
		t.Fatal("fresh execution inherited acceptance of a different receipt")
	}
	follow := cloneBasis(target)
	follow.Snapshots.Base = target.Snapshots.Candidate
	follow.Snapshots.Candidate = changedSnapshot(t, s, target.Snapshots.Candidate, "latest.go")
	f, err := Select(s, same.ID, follow, evidence.FollowUp, "compare last inspected to latest")
	if err != nil {
		t.Fatal(err)
	}
	if f.BasisSnapshots != p.BasisSnapshots || f.History[len(f.History)-1].Review.Mode != evidence.FollowUp || view(t, s, f).CurrentReceipt != nil {
		t.Fatal("follow-up erased original base")
	}
	if _, err := Select(s, same.ID, follow, evidence.OriginalBase, "wrong base"); err == nil {
		t.Fatal("changed original base accepted")
	}
	if _, err := Select(s, f.ID, follow, evidence.FollowUp, "stale follow-up base"); err == nil {
		t.Fatal("not last inspected base")
	}
}

func TestRestart(t *testing.T) {
	if os.Getenv("AFTER_REVIEW_CHILD") == "1" {
		s, err := store.Open(os.Getenv("AFTER_REVIEW_PROJECT"), false, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		v, err := Inspect(s, evidence.Digest(os.Getenv("AFTER_REVIEW_PIN")))
		if err != nil {
			t.Fatal(err)
		}
		if v.Pin.Decision != evidence.Reopened || len(v.Pin.History) != 3 || !v.MissingCurrentResult || v.CurrentReceipt != nil || v.Pin.Expectation != "same response and at most one provider call" {
			t.Fatalf("lost history %+v", v)
		}
		return
	}
	s, dir, r := fixture(t)
	p := pin(t, s, r, evidence.FiniteExample)
	p, err := Accept(s, p.ID, "accepted")
	if err != nil {
		t.Fatal(err)
	}
	b := evidence.BasisOf(r)
	b.Snapshots.Candidate = changedSnapshot(t, s, r.Snapshots.Candidate, "new.go")
	p, err = Select(s, p.ID, b, evidence.OriginalBase, "explicit new selection")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRestart$")
	cmd.Env = append(os.Environ(), "AFTER_REVIEW_CHILD=1", "AFTER_REVIEW_PROJECT="+dir, "AFTER_REVIEW_PIN="+string(p.ID))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restart %v: %s", err, out)
	}
}

func TestUnknownAndLegacyFailClosed(t *testing.T) {
	s, _, r := fixture(t)
	r.ID = ""
	r.Snapshots.Candidate = changedSnapshot(t, s, r.Snapshots.Candidate, "incomplete")
	r = put(t, s, r)
	p := pin(t, s, r, evidence.FiniteExample)
	if v := view(t, s, p); v.Applicability != evidence.Unknown {
		t.Fatal(v)
	}
	if _, err := Accept(s, p.ID, "cannot establish footprint"); err == nil {
		t.Fatal("unknown accepted")
	}
	p, err := Select(s, p.ID, evidence.BasisOf(r), evidence.OriginalBase, "same incomplete basis")
	if err != nil {
		t.Fatal(err)
	}
	if p.Decision != evidence.Reopened {
		t.Fatal("unknown footprint did not reopen")
	}
	p.ID = ""
	p.Scope = ""
	for i := range p.History {
		p.History[i].Review = nil
	}
	p = put(t, s, p)
	if view(t, s, p).Applicability != evidence.Unknown {
		t.Fatal("legacy freshness inferred")
	}
	if _, err := Attach(s, p.ID, r.ID, "legacy"); err == nil {
		t.Fatal("legacy migrated")
	}
}

func TestInvalidHistoryAndReceiptReferences(t *testing.T) {
	for _, kind := range []string{"scope", "missing-context", "auto-accept", "changed-result", "forged-bindings", "empty-reason", "history-limit"} {
		t.Run(kind, func(t *testing.T) {
			s, _, r := fixture(t)
			p := pin(t, s, r, evidence.FiniteExample)
			if kind == "empty-reason" {
				if _, err := Accept(s, p.ID, ""); err == nil {
					t.Fatal("empty reason")
				}
				return
			}
			p.ID = ""
			switch kind {
			case "scope":
				p.Scope = "universal-proof"
			case "missing-context":
				p.History[0].Review = nil
			case "auto-accept":
				e := p.History[0]
				c := *e.Review
				c.Action = "attach"
				e.Review = &c
				e.Decision = evidence.Accepted
				p.Decision = evidence.Accepted
				p.History = append(p.History, e)
			case "changed-result":
				p.History[0].Review.Receipt = p.Scenario
			case "forged-bindings":
				p.History[0].Review.Target.Bindings.Input = p.Scenario
			case "history-limit":
				for len(p.History) < 257 {
					p.History = append(p.History, p.History[0])
				}
			}
			if _, err := store.Put(s, p); err == nil {
				t.Fatal("invalid history accepted")
			}
		})
	}
}

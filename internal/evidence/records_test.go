package evidence

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// All identities and observations in these tests are synthetic, not run evidence.
var syntheticDigest = Digest("sha256:" + strings.Repeat("a", 64))
var syntheticTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func snapshot() Snapshot {
	return Snapshot{SchemaVersion: SchemaVersion, ID: syntheticDigest, Source: WorkingTree, Commit: strings.Repeat("a", 40), Files: []File{{"main.go", syntheticDigest, "100644"}}, Completeness: Complete, Diff: syntheticDigest}
}
func scenario() Scenario {
	return Scenario{SchemaVersion, syntheticDigest, syntheticDigest, syntheticDigest, syntheticDigest, syntheticDigest, "synthetic HTTP sequence", "synthetic test", []string{"no production evidence"}}
}
func receipt() Receipt {
	e := Environment{syntheticDigest, syntheticDigest, syntheticDigest, []string{"synthetic-driver", "--case", "control"}}
	return Receipt{
		SchemaVersion: SchemaVersion, ID: syntheticDigest,
		State:           EvidenceState{Runner, Observed, Current, Completed, Equal, NoReport},
		Snapshots:       SnapshotPair{syntheticDigest, syntheticDigest},
		Bindings:        &Bindings{syntheticDigest, syntheticDigest, syntheticDigest, syntheticDigest, syntheticDigest},
		BaseEnvironment: &e, CandidateEnvironment: &e, Authorization: syntheticDigest,
		StartedAt: syntheticTime, FinishedAt: syntheticTime.Add(time.Second), Completeness: Complete,
		Artifacts: []Artifact{{syntheticDigest, "synthetic-response", 2, 1024, Complete, false, false}},
		Limits:    []string{"synthetic; no real execution"},
	}
}
func pin() Pin {
	return Pin{SchemaVersion, syntheticDigest, syntheticDigest, "synthetic: one request", syntheticDigest, SnapshotPair{syntheticDigest, syntheticDigest}, Pinned, []DecisionEvent{{Pinned, syntheticTime, "synthetic human decision"}}}
}
func reported() Receipt {
	r := receipt()
	r.State = EvidenceState{Importer, Reported, Unknown, NotRun, NotCompared, ReportPass}
	r.Bindings, r.BaseEnvironment, r.CandidateEnvironment, r.Authorization = nil, nil, nil, ""
	r.Snapshots.Base = ""
	return r
}

func roundTrip[T interface {
	Snapshot | Scenario | Receipt | Pin
	Validate() error
}](t *testing.T, value T) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode[T](bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, decoded) {
		t.Fatalf("round trip mismatch: %#v", decoded)
	}
}

func TestSyntheticDocumentationExample(t *testing.T) {
	f, err := os.Open("../../docs/schema-example.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, err := Decode[Scenario](f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.Author, "SYNTHETIC") {
		t.Fatal("example must be labeled synthetic")
	}
}

func TestRoundTrips(t *testing.T) {
	roundTrip(t, snapshot())
	roundTrip(t, scenario())
	roundTrip(t, receipt())
	roundTrip(t, reported())
	roundTrip(t, pin())
	r := receipt()
	r.State.Applicability = Stale
	roundTrip(t, r)
	r.State.Kind, r.State.Execution, r.State.Comparison = NoEvidence, Failed, Incomparable
	r.Completeness, r.Artifacts = Incomplete, nil
	roundTrip(t, r)
}

func TestRejectReceipt(t *testing.T) {
	for name, change := range map[string]func(*Receipt){
		"version":               func(r *Receipt) { r.SchemaVersion++ },
		"identity":              func(r *Receipt) { r.ID = "branch-name" },
		"snapshot":              func(r *Receipt) { r.Snapshots.Candidate = "" },
		"base":                  func(r *Receipt) { r.Snapshots.Base = "" },
		"bindings":              func(r *Receipt) { r.Bindings = nil },
		"scenario":              func(r *Receipt) { r.Bindings.Scenario = "" },
		"input":                 func(r *Receipt) { r.Bindings.Input = "" },
		"driver":                func(r *Receipt) { r.Bindings.Driver = "" },
		"observer":              func(r *Receipt) { r.Bindings.Observer = "" },
		"rules":                 func(r *Receipt) { r.Bindings.Rules = "" },
		"environment":           func(r *Receipt) { r.BaseEnvironment = nil },
		"toolchain":             func(r *Receipt) { r.CandidateEnvironment.Toolchain = "" },
		"dependencies":          func(r *Receipt) { r.CandidateEnvironment.Dependencies = "" },
		"argv":                  func(r *Receipt) { r.BaseEnvironment.Argv = nil },
		"authorization":         func(r *Receipt) { r.Authorization = "" },
		"time":                  func(r *Receipt) { r.FinishedAt = r.StartedAt.Add(-time.Second) },
		"no time":               func(r *Receipt) { r.StartedAt = time.Time{} },
		"unknown kind":          func(r *Receipt) { r.State.Kind = "approved" },
		"unknown producer":      func(r *Receipt) { r.State.Producer = "human" },
		"unknown applicability": func(r *Receipt) { r.State.Applicability = "fresh-ish" },
		"unknown execution":     func(r *Receipt) { r.State.Execution = "pass" },
		"unknown comparison":    func(r *Receipt) { r.State.Comparison = "probably_equal" },
		"unknown report":        func(r *Receipt) { r.State.Report = "green" },
		"not run observed":      func(r *Receipt) { r.State.Execution = NotRun },
		"failed observed":       func(r *Receipt) { r.State.Execution = Failed },
		"cancelled observed":    func(r *Receipt) { r.State.Execution = Cancelled },
		"absent completed":      func(r *Receipt) { r.State.Kind = NoEvidence },
		"runner report":         func(r *Receipt) { r.State.Report = ReportPass },
		"missing artifacts":     func(r *Receipt) { r.Artifacts = nil },
		"truncated equality":    func(r *Receipt) { r.Artifacts[0].Truncated = true },
		"redacted equality":     func(r *Receipt) { r.Artifacts[0].Redacted = true },
		"incomplete equality":   func(r *Receipt) { r.Completeness = Incomplete },
		"artifact incomplete":   func(r *Receipt) { r.Artifacts[0].Completeness = Incomplete },
		"oversized":             func(r *Receipt) { r.Artifacts[0].Bytes = 1025 },
		"negative size":         func(r *Receipt) { r.Artifacts[0].Bytes = -1 },
		"unbounded":             func(r *Receipt) { r.Artifacts[0].MaxBytes = 0 },
		"artifact path":         func(r *Receipt) { r.Artifacts[0].Content = "../../secret" },
		"limits":                func(r *Receipt) { r.Limits = nil },
	} {
		t.Run(name, func(t *testing.T) {
			r := receipt()
			change(&r)
			data, _ := json.Marshal(r)
			if _, err := Decode[Receipt](bytes.NewReader(data)); err == nil {
				t.Fatal("accepted invalid receipt")
			}
		})
	}
}

func TestImportedPassAndPinCannotPromoteEvidence(t *testing.T) {
	for _, change := range []func(*Receipt){
		func(r *Receipt) { r.State.Kind = Observed },
		func(r *Receipt) { r.State.Applicability = Current },
		func(r *Receipt) { r.State.Execution = Completed },
		func(r *Receipt) { r.State.Comparison = Equal },
		func(r *Receipt) { r.Authorization = syntheticDigest },
	} {
		r := reported()
		change(&r)
		if r.Validate() == nil {
			t.Fatal("import promoted itself")
		}
	}
	r := reported()
	before, _ := json.Marshal(r)
	p := pin()
	p.Decision = Accepted
	p.History = append(p.History, DecisionEvent{Accepted, syntheticTime.Add(time.Second), "synthetic acceptance"})
	roundTrip(t, p)
	after, _ := json.Marshal(r)
	if !bytes.Equal(before, after) {
		t.Fatal("pin mutated evidence")
	}
	data, _ := json.Marshal(p)
	data = append(data[:len(data)-1], []byte(",\"state\":{\"kind\":\"observed\",\"applicability\":\"current\"}}")...)
	if _, err := Decode[Pin](bytes.NewReader(data)); err == nil {
		t.Fatal("pin accepted evidence state")
	}
}

func TestInvalidRecords(t *testing.T) {
	for _, change := range []func(*Snapshot){
		func(s *Snapshot) { s.SchemaVersion = 0 },
		func(s *Snapshot) { s.Commit = "main" },
		func(s *Snapshot) { s.Source = MergeBase },
		func(s *Snapshot) { s.Files[0].Path = "../outside" },
		func(s *Snapshot) { s.Files[0].Mode = "120000" },
		func(s *Snapshot) { s.Files = append(s.Files, s.Files[0]) },
		func(s *Snapshot) { s.Unsupported = []Limitation{{"link", "symlink"}} },
	} {
		s := snapshot()
		change(&s)
		if s.Validate() == nil {
			t.Fatal("invalid snapshot accepted")
		}
	}
	for _, change := range []func(*Scenario){
		func(s *Scenario) { s.SchemaVersion++ },
		func(s *Scenario) { s.Input = "" },
		func(s *Scenario) { s.Observer = "" },
		func(s *Scenario) { s.Rules = "" },
	} {
		s := scenario()
		change(&s)
		if s.Validate() == nil {
			t.Fatal("invalid scenario accepted")
		}
	}
	for _, change := range []func(*Pin){
		func(p *Pin) { p.SchemaVersion++ },
		func(p *Pin) { p.Scenario = "" },
		func(p *Pin) { p.BasisReceipt = "" },
		func(p *Pin) { p.Expectation = "" },
		func(p *Pin) { p.History = nil },
		func(p *Pin) { p.Decision = Accepted },
	} {
		p := pin()
		change(&p)
		if p.Validate() == nil {
			t.Fatal("invalid pin accepted")
		}
	}
}

func TestDecodeBoundsAndShape(t *testing.T) {
	data, _ := json.Marshal(snapshot())
	for _, input := range []string{"null", "{}", "[]", "", string(data) + "{}", string(data) + "garbage", strings.Repeat(" ", MaxRecordBytes+1), strings.Replace(string(data), "\"source\"", "\"unknown_field\"", 1)} {
		if _, err := Decode[Snapshot](strings.NewReader(input)); err == nil {
			t.Fatal("accepted invalid JSON record")
		}
	}
}

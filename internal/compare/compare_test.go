package compare

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
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
// execution. They include complete preparation bindings and generated-plan identities.
func receipt(t *testing.T, s *store.Store, pair evidence.SnapshotPair, change func(*runner.Observation, string, int), mutate func(*runner.Sample)) evidence.Receipt {
	t.Helper()
	p, err := runner.Prepare(s, pair, 2, sandbox.Limits{Seconds: 180, OutputBytes: 65536})
	if err != nil {
		t.Fatal(err)
	}
	preview, _ := p.Preview()
	var frozen struct {
		Definition       runner.Definition `json:"definition"`
		DefinitionDigest evidence.Digest   `json:"definition_digest"`
		Preparation      json.RawMessage   `json:"preparation"`
		ServiceConfig    json.RawMessage   `json:"service_config"`
		Experiments      [][]struct {
			App      json.RawMessage `json:"App"`
			Observer json.RawMessage `json:"Observer"`
		} `json:"experiments"`
	}
	if err := json.Unmarshal(preview, &frozen); err != nil {
		t.Fatal(err)
	}
	planID := func(raw json.RawMessage) string {
		b, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("sha256:%x", sha256.Sum256(b))
	}
	var serviceConfig bytes.Buffer
	if err := json.Compact(&serviceConfig, frozen.ServiceConfig); err != nil {
		t.Fatal(err)
	}
	launcher := syntheticStaticELF()
	launcherArtifact, err := s.PutArtifact(launcher, "launcher-executable", 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := s.PutArtifact(nil, "preparation-diagnostics", 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	var preparation struct {
		Version          int             `json:"version"`
		Status           string          `json:"status"`
		Plan             string          `json:"plan"`
		DefinitionDigest evidence.Digest `json:"definition_digest"`
		Platform         string          `json:"platform"`
		Image            string          `json:"image"`
		ExitCode         int             `json:"exit_code"`
		Cleaned          bool            `json:"cleaned"`
		Truncated        bool            `json:"truncated"`
		Launcher         evidence.Digest `json:"launcher_digest"`
		Bytes            int             `json:"launcher_bytes"`
	}
	var preparationTemplate struct {
		Image    string `json:"image"`
		Platform string `json:"platform"`
	}
	if err := json.Unmarshal(frozen.Preparation, &preparationTemplate); err != nil {
		t.Fatal(err)
	}
	preparation = struct {
		Version          int             `json:"version"`
		Status           string          `json:"status"`
		Plan             string          `json:"plan"`
		DefinitionDigest evidence.Digest `json:"definition_digest"`
		Platform         string          `json:"platform"`
		Image            string          `json:"image"`
		ExitCode         int             `json:"exit_code"`
		Cleaned          bool            `json:"cleaned"`
		Truncated        bool            `json:"truncated"`
		Launcher         evidence.Digest `json:"launcher_digest"`
		Bytes            int             `json:"launcher_bytes"`
	}{1, "completed", planID(frozen.Preparation), frozen.DefinitionDigest, preparationTemplate.Platform, preparationTemplate.Image, 0, true, false, launcherArtifact.Content, len(launcher)}
	preparationArtifact := artifact(t, s, preparation, "preparation-result")
	denied, err := (runner.Executor{}).Run(t.Context(), s, p, "not approved")
	if err == nil {
		t.Fatal("expected denial")
	}
	r := denied.Receipt
	r.ID = ""
	r.Completeness = evidence.Complete
	r.State = evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.Observed, Execution: evidence.Completed, Applicability: evidence.Current, Comparison: evidence.NotCompared, Report: evidence.NoReport}
	r.Artifacts = append(r.Artifacts[:2], diagnostics, launcherArtifact, preparationArtifact)
	var appPlans [2][]string
	var observerPlans [2][]string
	for sideIndex, side := range []string{"base", "candidate"} {
		snapshotID := pair.Base
		if side == "candidate" {
			snapshotID = pair.Candidate
		}
		snapshot, err := store.Get[evidence.Snapshot](s, snapshotID)
		if err != nil {
			t.Fatal(err)
		}
		files := make(map[string][]byte, len(snapshot.Files))
		for _, file := range snapshot.Files {
			files[file.Path], err = s.ReadBlob(file.Content)
			if err != nil {
				t.Fatal(err)
			}
		}
		for caseIndex := range frozen.Definition.Cases {
			var appTemplate struct {
				Argv           []string                `json:"argv"`
				Image          string                  `json:"image"`
				Platform       string                  `json:"platform"`
				Limits         sandbox.Limits          `json:"limits"`
				GeneratedSlots []sandbox.GeneratedFile `json:"generated_slots"`
			}
			if err := json.Unmarshal(frozen.Experiments[sideIndex][caseIndex].App, &appTemplate); err != nil {
				t.Fatal(err)
			}
			plan, err := sandbox.PrepareTemplate(string(snapshotID), files, appTemplate.Argv, appTemplate.Limits, appTemplate.Image, appTemplate.Platform, appTemplate.GeneratedSlots)
			if err != nil {
				t.Fatal(err)
			}
			_, templateID := plan.Preview()
			if templateID != planID(frozen.Experiments[sideIndex][caseIndex].App) {
				t.Fatalf("reconstructed template mismatch: got %s want %s", templateID, planID(frozen.Experiments[sideIndex][caseIndex].App))
			}
			materialized, err := plan.Materialize(map[string][]byte{"after/launcher": launcher, "after/service.json": serviceConfig.Bytes()})
			if err != nil {
				t.Fatal(err)
			}
			_, appPlanID := materialized.Preview()
			appPlans[sideIndex] = append(appPlans[sideIndex], appPlanID)
			observerPlans[sideIndex] = append(observerPlans[sideIndex], planID(frozen.Experiments[sideIndex][caseIndex].Observer))
		}
	}
	for rep := 0; rep < 2; rep++ {
		for sideIndex, side := range []string{"base", "candidate"} {
			for caseIndex, scenarioCase := range frozen.Definition.Cases {
				seconds := runner.CaseDuration(scenarioCase)
				prefix := fmt.Sprintf("%s/%s/%d/", side, scenarioCase.ID, rep)
				o := runner.Observation{Version: 1, CaseID: scenarioCase.ID, Seconds: seconds, Responses: []runner.Response{{Status: 200, Body: `{"payment":"synthetic-accepted"}`}, {Status: 200, Body: `{"payment":"synthetic-accepted"}`}}, Calls: []runner.Call{{At: 1735689600, Endpoint: "provider", Destination: "127.0.0.1:18082", Method: "POST", Path: "/charges", Key: "synthetic-key-a", Body: `{"amount_cents":1200,"currency":"USD"}`}}}
				if side == "candidate" && scenarioCase.ID == "43200" {
					call := o.Calls[0]
					call.At += seconds
					o.Calls = append(o.Calls, call)
				}
				if change != nil {
					change(&o, side, rep)
				}
				a := artifact(t, s, o, prefix+"observation")
				m := runner.Sample{RequestID: r.RequestID, Snapshots: pair, Side: side, CaseID: scenarioCase.ID, CaseSeconds: seconds, Repetition: rep, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Status: "completed", Execution: sandbox.ExperimentResult{App: sandbox.Result{Cleaned: true}, Observer: sandbox.Result{Cleaned: true}}, Artifacts: []evidence.Artifact{a}}
				m.Execution.App.Plan = appPlans[sideIndex][caseIndex]
				m.Execution.Observer.Plan = observerPlans[sideIndex][caseIndex]
				if mutate != nil {
					mutate(&m)
				}
				r.Artifacts = append(r.Artifacts, a, artifact(t, s, m, prefix+"sample"))
			}
		}
	}
	return r
}

func syntheticStaticELF() []byte {
	data := make([]byte, 120)
	copy(data[:4], []byte{0x7f, 'E', 'L', 'F'})
	data[4], data[5], data[6] = 2, 1, 1
	binary.LittleEndian.PutUint16(data[16:18], 2)
	machine := uint16(62)
	if runtime.GOARCH == "arm64" {
		machine = 183
	}
	binary.LittleEndian.PutUint16(data[18:20], machine)
	binary.LittleEndian.PutUint32(data[20:24], 1)
	binary.LittleEndian.PutUint64(data[32:40], 64)
	binary.LittleEndian.PutUint16(data[52:54], 64)
	binary.LittleEndian.PutUint16(data[54:56], 56)
	binary.LittleEndian.PutUint16(data[56:58], 1)
	binary.LittleEndian.PutUint32(data[64:68], 1)
	binary.LittleEndian.PutUint32(data[68:72], 5)
	binary.LittleEndian.PutUint64(data[96:104], uint64(len(data)))
	binary.LittleEndian.PutUint64(data[104:112], uint64(len(data)))
	binary.LittleEndian.PutUint64(data[112:120], 4096)
	return data
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
			if w.Relation != "paired" || w.Channel != "provider_calls" || w.Before.Seconds != 43200 {
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
func pythonServiceSetup(t *testing.T) (*store.Store, evidence.SnapshotPair) {
	t.Helper()
	s, err := store.Open(t.TempDir(), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	diff, err := s.PutArtifact([]byte("synthetic Python service source change"), "raw-diff", 4096)
	if err != nil {
		t.Fatal(err)
	}
	baseSource, err := os.ReadFile("../runner/testdata/python-service/app.py")
	if err != nil {
		t.Fatal(err)
	}
	pair := evidence.SnapshotPair{}
	for _, side := range []string{"base", "candidate"} {
		source := append([]byte(nil), baseSource...)
		if side == "candidate" {
			source = append(source, []byte("\n# candidate snapshot\n")...)
		}
		blob, err := s.PutArtifact(source, "source", maxJSON)
		if err != nil {
			t.Fatal(err)
		}
		snapshot := put(t, s, evidence.Snapshot{SchemaVersion: 1, Source: evidence.WorkingTree, Unborn: true, Files: []evidence.File{{Path: "app.py", Content: blob.Content, Mode: "100644"}}, Completeness: evidence.Complete, Diff: diff.Content})
		if side == "base" {
			pair.Base = snapshot.ID
		} else {
			pair.Candidate = snapshot.ID
		}
	}
	return s, pair
}

// Validate the fixture before consent or Docker access. The sandbox separately
// checks daemon isolation and the approved image platform after consent;
// run this proof process on a matching native Docker host.
func pythonProofDefinition(platform string) ([]byte, error) {
	raw, err := os.ReadFile("../runner/testdata/python-service/http-service.json")
	if err != nil {
		return nil, err
	}
	definition, err := runner.ParseDefinition(raw)
	if err != nil {
		return nil, err
	}
	if definition.Platform != platform {
		return nil, fmt.Errorf("Python proof fixture is provisioned for %s, not %s: separately authorize and provision a digest-pinned Python image for %s, then update internal/runner/testdata/python-service/http-service.json platform and image together and record the digest in docs/SANDBOX.md; rerun task http-service:proof and task test:poc on a matching native Docker host (no automatic pulls or emulation)", definition.Platform, platform, platform)
	}
	return raw, nil
}

func TestPythonProofPlatformProvisioning(t *testing.T) {
	if _, err := pythonProofDefinition("linux/arm64"); err != nil {
		t.Fatal(err)
	}
	if raw, err := pythonProofDefinition("linux/amd64"); err == nil || raw != nil || !strings.Contains(err.Error(), "separately authorize and provision") || !strings.Contains(err.Error(), "platform and image together") {
		t.Fatalf("unprovisioned platform must fail with a provisioning fix before execution: %s %v", raw, err)
	}
}

func TestPythonHTTPServiceProof(t *testing.T) {
	if os.Getenv("AFTER_HTTP_PROOF") != "1" {
		t.Skip("task http-service:proof authorizes the separate Python image")
	}
	definition, err := pythonProofDefinition("linux/" + runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	s, pair := pythonServiceSetup(t)
	plan, err := runner.PrepareDefinition(s, pair, definition, runner.DefinitionSource{Kind: "operator-selected-file"})
	if err != nil {
		t.Fatal(err)
	}
	preview, approval := plan.Preview()
	t.Logf("Approved synthetic Python HTTP-service plan %s: %s", approval, preview)
	run, err := (runner.Executor{Docker: sandbox.Docker{Binary: os.Getenv("AFTER_DOCKER_BINARY"), Host: os.Getenv("AFTER_DOCKER_HOST")}}).Run(t.Context(), s, plan, approval)
	if err != nil || run.Receipt.Completeness != evidence.Complete || len(run.Samples) != 2 {
		t.Fatalf("Python service run incomplete: %+v %v", run.Receipt, err)
	}
	comparison, report := result(t, s, run.Receipt)
	if comparison.Outcome != evidence.Equal || comparison.Completeness != evidence.Complete || report.DefinitionName != "python-stdlib-echo" || report.BuiltInPayment || len(report.Cases) != 1 || report.Cases[0] != "echo" || len(report.Channels) != 2 || len(report.Witnesses) != 2 {
		t.Fatalf("Python service comparison incorrect: %+v %+v", comparison, report)
	}
	for _, witness := range report.Witnesses {
		if witness.Relation != "paired" || witness.Outcome != evidence.Equal || witness.Before.CaseID != "echo" || witness.Channel != "responses" && witness.Channel != "provider_calls" {
			t.Fatalf("Python witness mismatch: %+v", witness)
		}
	}
	t.Logf("real Python stdlib service on its pinned image: equal responses and one recorded fake-upstream call per side")
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

package compare

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

func TestCommandComparisonPreservesBinaryBytesAndDetectsInstabilityAcrossAllRepetitions(t *testing.T) {
	s, pair := setup(t)
	definition := testCommandDefinition("text", "text", []runner.CommandCase{
		commandCase("binary", []string{"/work/fixture", "binary"}),
		commandCase("unstable", []string{"/work/fixture", "unstable"}),
	})
	receipt := commandReceipt(t, s, pair, definition, func(side, caseID string, repetition int) []byte {
		switch caseID {
		case "binary":
			if side == "base" {
				return []byte{0xff, 0, 0x1b, ']', '5', '2', ';', 'c', ';', 'x', 0x07, 'A'}
			}
			return []byte{0xff, 0, 0x1b, ']', '5', '2', ';', 'c', ';', 'x', 0x07, 'B'}
		case "unstable":
			if side == "base" && repetition == 1 {
				return []byte("drift\n")
			}
			return []byte("stable\n")
		default:
			t.Fatalf("unexpected command case %q", caseID)
			return nil
		}
	})
	comparison, report := result(t, s, receipt)
	if comparison.Outcome != evidence.Unstable || comparison.Completeness != evidence.Complete {
		t.Fatalf("repetition drift was not reported unstable: %+v", comparison)
	}
	binaryWitness, unstableWitness := false, false
	for _, witness := range report.Witnesses {
		if witness.Channel != "stdout" {
			continue
		}
		if witness.Relation == "paired" && witness.Before.CaseID == "binary" && witness.Outcome == evidence.Different {
			binaryWitness = len(witness.Changes) == 1 && bytes.Contains(witness.Changes[0].Before, []byte(base64.StdEncoding.EncodeToString([]byte{0xff, 0, 0x1b, ']', '5', '2', ';', 'c', ';', 'x', 0x07, 'A'}))) && bytes.Contains(witness.Changes[0].After, []byte(base64.StdEncoding.EncodeToString([]byte{0xff, 0, 0x1b, ']', '5', '2', ';', 'c', ';', 'x', 0x07, 'B'}))) && !bytes.Contains(witness.Changes[0].Before, []byte{0x1b}) && !bytes.Contains(witness.Changes[0].After, []byte{0x1b})
		}
		if witness.Relation == "repetition" && witness.Before.CaseID == "unstable" && witness.Before.Side == "base" && witness.Outcome == evidence.Different {
			unstableWitness = true
		}
	}
	if !binaryWitness || !unstableWitness {
		t.Fatalf("binary witness or all-repetition instability witness missing: binary=%t unstable=%t witnesses=%+v", binaryWitness, unstableWitness, report.Witnesses)
	}
	if len(report.Witnesses) != 24 {
		t.Fatalf("all 2 cases × 2 repetitions × paired/repetition × 3 channels must be witnessed: %d", len(report.Witnesses))
	}
}

func TestCommandComparisonUsesFrozenStructuralJSONPolicy(t *testing.T) {
	s, pair := setup(t)
	definition := testCommandDefinition("json", "text", []runner.CommandCase{commandCase("json", []string{"/work/fixture", "json"})})
	receipt := commandReceipt(t, s, pair, definition, func(side, _ string, _ int) []byte {
		if side == "base" {
			return []byte("{\"amount\":1.000,\"nested\":{\"a\":true,\"b\":null}}\n")
		}
		return []byte("{\"nested\":{\"b\":null,\"a\":true},\"amount\":1.0}\n")
	})
	comparison, report := result(t, s, receipt)
	if comparison.Outcome != evidence.Equal || comparison.Completeness != evidence.Complete {
		t.Fatalf("definition-declared structural JSON did not compare equal: %+v", comparison)
	}
	if len(report.Witnesses) != 12 {
		t.Fatalf("status and both streams must be witnessed for paired and both sides' repetition samples: %d", len(report.Witnesses))
	}
	for _, witness := range report.Witnesses {
		if witness.Outcome != evidence.Equal {
			t.Fatalf("structurally equal command JSON changed: %+v", witness)
		}
	}
}

func TestCommandComparisonFailsClosedForMissingRedactedInvalidExitAndChangedRules(t *testing.T) {
	for _, mode := range []string{"missing stdout", "redacted stdout", "truncated stderr", "reserved exit", "changed rules"} {
		t.Run(mode, func(t *testing.T) {
			s, pair := setup(t)
			definition := testCommandDefinition("text", "text", []runner.CommandCase{commandCase("case", []string{"/work/fixture"})})
			receipt := commandReceipt(t, s, pair, definition, func(string, string, int) []byte { return []byte("same\n") })
			switch mode {
			case "missing stdout":
				for index, artifact := range receipt.Artifacts {
					if artifact.Channel == "base/case/0/stdout" {
						receipt.Artifacts = append(receipt.Artifacts[:index], receipt.Artifacts[index+1:]...)
						break
					}
				}
			case "redacted stdout":
				for index := range receipt.Artifacts {
					if receipt.Artifacts[index].Channel == "base/case/0/stdout" {
						receipt.Artifacts[index].Redacted = true
						receipt.Artifacts[index].Completeness = evidence.Incomplete
					}
				}
			case "truncated stderr":
				for index := range receipt.Artifacts {
					if receipt.Artifacts[index].Channel == "candidate/case/0/stderr" {
						receipt.Artifacts[index].Truncated = true
						receipt.Artifacts[index].Completeness = evidence.Incomplete
					}
				}
			case "reserved exit":
				for index := range receipt.Artifacts {
					artifact := &receipt.Artifacts[index]
					if artifact.Channel != "base/case/0/sample" {
						continue
					}
					var sample runner.Sample
					data, err := s.ReadBlob(artifact.Content)
					if err != nil || json.Unmarshal(data, &sample) != nil {
						t.Fatalf("read sample metadata: %v", err)
					}
					sample.Execution.App.ExitCode = 125
					updated, err := s.PutArtifact(mustJSON(t, sample), artifact.Channel, 1<<20)
					if err != nil {
						t.Fatal(err)
					}
					*artifact = updated
				}
			case "changed rules":
				policy, err := s.PutArtifact([]byte(`{"version":1,"masks":["/**"]}`), "comparison-rules", 4096)
				if err != nil {
					t.Fatal(err)
				}
				for index := range receipt.Artifacts {
					if receipt.Artifacts[index].Channel == "comparison-rules" {
						receipt.Artifacts[index] = policy
					}
				}
			}
			if mode == "redacted stdout" || mode == "truncated stderr" {
				basis, err := runner.ValidateComparisonBasis(s, receipt)
				if err != nil {
					t.Fatal(err)
				}
				if err := compareCommandReceipt(s, receipt, &Report{}, basis); err == nil {
					t.Fatalf("%s command stream was accepted", mode)
				}
				return
			}
			comparison, report := result(t, s, receipt)
			if comparison.Outcome != evidence.Incomparable || comparison.Completeness != evidence.Incomplete || len(report.Limits) == 0 {
				t.Fatalf("%s did not fail closed: comparison=%+v report=%+v", mode, comparison, report)
			}
		})
	}
}

func testCommandDefinition(stdout, stderr string, cases []runner.CommandCase) runner.CommandDefinition {
	return runner.CommandDefinition{
		Version: 1, Kind: "command", Name: "compare-fixture", Platform: "linux/" + runtime.GOARCH, Image: sandbox.Image,
		Cases: cases, Repetitions: 2,
		Limits:     runner.DefinitionLimits{Seconds: 30, OutputBytes: 65536, PreparationSeconds: 90},
		Comparison: runner.CommandComparison{Stdout: stdout, Stderr: stderr},
	}
}

func commandCase(id string, argv []string) runner.CommandCase {
	return runner.CommandCase{ID: id, Title: "Synthetic " + id + " output", Argv: argv, Stdin: []byte{}, Environment: []string{}}
}

func commandReceipt(t *testing.T, s *store.Store, pair evidence.SnapshotPair, definition runner.CommandDefinition, output func(side, caseID string, repetition int) []byte) evidence.Receipt {
	t.Helper()
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := runner.PrepareCommandDefinition(s, pair, raw, runner.DefinitionSource{Kind: "operator-selected-file"})
	if err != nil {
		t.Fatal(err)
	}
	preview, planID := plan.Preview()
	var frozen struct {
		Request          evidence.Digest          `json:"request"`
		Snapshots        evidence.SnapshotPair    `json:"snapshots"`
		Scenario         evidence.Scenario        `json:"scenario"`
		DefinitionDigest evidence.Digest          `json:"definition_digest"`
		Definition       runner.CommandDefinition `json:"definition"`
		CommandConfigs   []json.RawMessage        `json:"command_configs"`
		Preparation      json.RawMessage          `json:"preparation"`
	}
	if err := json.Unmarshal(preview, &frozen); err != nil {
		t.Fatal(err)
	}
	launcher := syntheticStaticELF()
	launcherArtifact, err := s.PutArtifact(launcher, "launcher-executable", 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	preparationID := commandPreparationID(t, frozen.Preparation)
	preparationResult := struct {
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
	}{1, "completed", preparationID, frozen.DefinitionDigest, frozen.Definition.Platform, sandbox.Image, 0, true, false, launcherArtifact.Content, len(launcher)}
	preparationArtifact := artifact(t, s, preparationResult, "preparation-result")
	diagnostics, err := s.PutArtifact(nil, "preparation-diagnostics", 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	planArtifact, err := s.PutArtifact(preview, "execution-plan", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := s.PutArtifact([]byte(runner.CommandComparisonRules), "comparison-rules", 4096)
	if err != nil {
		t.Fatal(err)
	}
	artifacts := []evidence.Artifact{planArtifact, policy, preparationArtifact, diagnostics, launcherArtifact}
	basis := plan.ReviewBasis()
	started := time.Now().UTC()
	finished := started.Add(time.Second)
	requestID := plan.RequestID()
	planIDs := commandMaterializedIDs(t, s, frozen, launcher)
	for repetition := 0; repetition < frozen.Definition.Repetitions; repetition++ {
		for sideIndex, side := range []string{"base", "candidate"} {
			for caseIndex, scenarioCase := range frozen.Definition.Cases {
				stdout := output(side, scenarioCase.ID, repetition)
				stderr := []byte{}
				stdoutArtifact, err := s.PutArtifact(stdout, fmt.Sprintf("%s/%s/%d/stdout", side, scenarioCase.ID, repetition), int64(frozen.Definition.Limits.OutputBytes))
				if err != nil {
					t.Fatal(err)
				}
				stderrArtifact, err := s.PutArtifact(stderr, fmt.Sprintf("%s/%s/%d/stderr", side, scenarioCase.ID, repetition), int64(frozen.Definition.Limits.OutputBytes))
				if err != nil {
					t.Fatal(err)
				}
				status := 0
				sample := runner.Sample{
					RequestID: requestID, Snapshots: pair, Side: side, CaseID: scenarioCase.ID, Repetition: repetition,
					StartedAt: started, FinishedAt: finished, Status: "completed",
					Execution: sandbox.ExperimentResult{App: sandbox.Result{Plan: planIDs[sideIndex][caseIndex], Container: fmt.Sprintf("synthetic-%s-%s-%d", side, scenarioCase.ID, repetition), ExitCode: status, Cleaned: true}},
					Artifacts: []evidence.Artifact{stdoutArtifact, stderrArtifact},
				}
				artifacts = append(artifacts, stdoutArtifact, stderrArtifact, artifact(t, s, sample, fmt.Sprintf("%s/%s/%d/sample", side, scenarioCase.ID, repetition)))
			}
		}
	}
	return evidence.Receipt{
		RequestID: requestID, SchemaVersion: 1,
		State:     evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.Observed, Applicability: evidence.Current, Execution: evidence.Completed, Comparison: evidence.NotCompared, Report: evidence.NoReport},
		Snapshots: pair, Bindings: basis.Bindings, BaseEnvironment: basis.BaseEnvironment, CandidateEnvironment: basis.CandidateEnvironment,
		Authorization: evidence.Digest(planID), StartedAt: started, FinishedAt: finished, Completeness: evidence.Complete, Artifacts: artifacts, Limits: append([]string(nil), frozen.Scenario.Limits...),
	}
}

func commandPreparationID(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var spec struct {
		Version        int                     `json:"version"`
		Preparation    string                  `json:"preparation"`
		Mounts         string                  `json:"mounts"`
		Image          string                  `json:"image"`
		Platform       string                  `json:"platform"`
		Snapshot       string                  `json:"snapshot"`
		Input          string                  `json:"input_archive"`
		StdinDigest    string                  `json:"stdin_digest,omitempty"`
		GeneratedSlots []sandbox.GeneratedFile `json:"generated_slots,omitempty"`
		Materialized   []sandbox.GeneratedFile `json:"materialized_files,omitempty"`
		Argv           []string                `json:"argv"`
		Environment    []string                `json:"environment"`
		Policy         []string                `json:"docker_policy"`
		Limits         sandbox.Limits          `json:"limits"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(canonical)
	return fmt.Sprintf("sha256:%x", sum)
}

func commandMaterializedIDs(t *testing.T, s *store.Store, frozen struct {
	Request          evidence.Digest          `json:"request"`
	Snapshots        evidence.SnapshotPair    `json:"snapshots"`
	Scenario         evidence.Scenario        `json:"scenario"`
	DefinitionDigest evidence.Digest          `json:"definition_digest"`
	Definition       runner.CommandDefinition `json:"definition"`
	CommandConfigs   []json.RawMessage        `json:"command_configs"`
	Preparation      json.RawMessage          `json:"preparation"`
}, launcher []byte) [2][]string {
	t.Helper()
	preparationID := commandPreparationID(t, frozen.Preparation)
	var ids [2][]string
	for sideIndex, snapshotID := range []evidence.Digest{frozen.Snapshots.Base, frozen.Snapshots.Candidate} {
		snapshot, err := store.Get[evidence.Snapshot](s, snapshotID)
		if err != nil {
			t.Fatal(err)
		}
		baseFiles := make(map[string][]byte, len(snapshot.Files))
		for _, file := range snapshot.Files {
			content, err := s.ReadBlob(file.Content)
			if err != nil {
				t.Fatal(err)
			}
			baseFiles[file.Path] = content
		}
		ids[sideIndex] = make([]string, len(frozen.Definition.Cases))
		for caseIndex, scenarioCase := range frozen.Definition.Cases {
			files := make(map[string][]byte, len(baseFiles)+len(scenarioCase.InputFiles))
			for path, content := range baseFiles {
				files[path] = content
			}
			for _, input := range scenarioCase.InputFiles {
				files[input.Path] = input.Content
			}
			var config struct {
				Version     int      `json:"version"`
				BuildArgv   []string `json:"build_argv,omitempty"`
				Argv        []string `json:"argv"`
				Environment []string `json:"environment"`
			}
			if err := json.Unmarshal(frozen.CommandConfigs[caseIndex], &config); err != nil {
				t.Fatal(err)
			}
			configBytes, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			template, err := sandbox.PrepareTemplate(string(snapshotID), files, []string{"/input/after/launcher"}, sandbox.Limits{Seconds: frozen.Definition.Limits.Seconds, OutputBytes: frozen.Definition.Limits.OutputBytes}, frozen.Definition.Image, frozen.Definition.Platform, []sandbox.GeneratedFile{
				{Path: "after/launcher", Producer: preparationID, MaxBytes: 8 << 20, Mode: 0555},
				{Path: "after/service.json", Producer: string(frozen.DefinitionDigest), MaxBytes: 64 << 10, Mode: 0444},
			})
			if err != nil {
				t.Fatal(err)
			}
			template, err = template.WithCommandInput(scenarioCase.Stdin)
			if err != nil {
				t.Fatal(err)
			}
			concrete, err := template.Materialize(map[string][]byte{"after/launcher": launcher, "after/service.json": configBytes})
			if err != nil {
				t.Fatal(err)
			}
			_, ids[sideIndex][caseIndex] = concrete.Preview()
		}
	}
	return ids
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

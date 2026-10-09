package runner

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

func TestCommandPlanBindsSelectedDefinitionOracleAndBinaryStdin(t *testing.T) {
	s, pair := captured(t, "")
	pair.Base = addSnapshotFiles(t, s, pair.Base, map[string][]byte{"scenario.json": []byte(`{"snapshot":"base"}`)})
	pair.Candidate = addSnapshotFiles(t, s, pair.Candidate, map[string][]byte{"scenario.json": []byte(`{"snapshot":"candidate"}`)})

	definition := validCommandDefinition()
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	selected := DefinitionSource{Kind: "operator-selected-file", RepositoryPath: "scenario.json"}
	plan, err := PrepareCommandDefinition(s, pair, raw, selected)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.definitionSource.ChangedOracle || plan.definitionDigest != hash(raw) || string(plan.command.Cases[0].Stdin) != string(definition.Cases[0].Stdin) {
		t.Fatalf("selected definition/oracle/stdin was not frozen: %+v", plan.definitionSource)
	}
	var preview struct {
		Definition       CommandDefinition `json:"definition"`
		DefinitionDigest evidence.Digest   `json:"definition_digest"`
		Source           DefinitionSource  `json:"definition_source"`
	}
	previewBytes, id := plan.Preview()
	if err := json.Unmarshal(previewBytes, &preview); err != nil {
		t.Fatal(err)
	}
	if preview.DefinitionDigest != hash(raw) || !preview.Source.ChangedOracle || preview.Source.RepositoryPath != "scenario.json" || len(preview.Definition.Cases[0].Stdin) == 0 {
		t.Fatalf("consent preview omitted selected definition inputs: %+v", preview)
	}

	replayed, err := PrepareFromPreview(s, previewBytes)
	if err != nil {
		t.Fatal(err)
	}
	_, replayedID := replayed.Preview()
	if replayedID != id {
		t.Fatalf("exact command preview did not reconstruct: got %s want %s", replayedID, id)
	}
	definition.Cases[0].Stdin[1] ^= 0x10
	changedRaw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := PrepareCommandDefinition(s, pair, changedRaw, selected)
	if err != nil {
		t.Fatal(err)
	}
	_, changedID := changed.Preview()
	if changedID == id {
		t.Fatal("changing frozen binary stdin did not change the consent-bound plan")
	}
}

func TestCommandPlanRejectsInputFileCollidingWithCapturedSnapshot(t *testing.T) {
	s, pair := captured(t, "")
	pair.Base = addSnapshotFiles(t, s, pair.Base, map[string][]byte{
		"scenario.json": []byte(`{"snapshot":"base"}`),
		"fixtures":      []byte("captured"),
	})
	pair.Candidate = addSnapshotFiles(t, s, pair.Candidate, map[string][]byte{
		"scenario.json": []byte(`{"snapshot":"candidate"}`),
		"fixtures":      []byte("captured"),
	})
	definition := validCommandDefinition()
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareCommandDefinition(s, pair, raw, DefinitionSource{Kind: "operator-selected-file", RepositoryPath: "scenario.json"}); err == nil {
		t.Fatal("additive command input shadowing a captured snapshot path was accepted")
	}
}

func TestCommandLauncherPreparationFailureLeavesSamplesIncomplete(t *testing.T) {
	s, pair := captured(t, "")
	definition := validCommandDefinition()
	definition.Repetitions = 2
	definition.Limits.OutputBytes = 4096
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PrepareCommandDefinition(s, pair, raw, DefinitionSource{Kind: "operator-selected-file"})
	if err != nil {
		t.Fatal(err)
	}
	_, approval := plan.Preview()
	executed := 0
	executor := Executor{
		prepareLauncher: func(_ context.Context, preparation *sandbox.Plan) (sandbox.Result, []byte, error) {
			_, id := preparation.Preview()
			return sandbox.Result{Plan: id, ExitCode: 2, Cleaned: true, Stderr: "synthetic compiler failure"}, nil, errors.New("compiler failed")
		},
		executeCommand: func(context.Context, *sandbox.Plan, string) (sandbox.Result, error) {
			executed++
			return sandbox.Result{}, nil
		},
	}
	result, err := executor.Run(t.Context(), s, plan, approval)
	if err == nil || executed != 0 || result.Receipt.Completeness != evidence.Incomplete || result.Receipt.State.Execution != evidence.Failed || result.Receipt.State.Kind == evidence.Observed {
		t.Fatalf("compiler failure was promoted or samples started: receipt=%+v executed=%d error=%v", result.Receipt, executed, err)
	}
	if len(result.Samples) != 4 {
		t.Fatalf("all two sides × two repetitions must retain incomplete samples: %d", len(result.Samples))
	}
	for _, sample := range result.Samples {
		if sample.Status != "preparation_failed" || sample.Execution.App.Container != "" {
			t.Fatalf("preparation failure fabricated command execution: %+v", sample)
		}
	}
}

func addSnapshotFiles(t *testing.T, s *store.Store, id evidence.Digest, files map[string][]byte) evidence.Digest {
	t.Helper()
	snapshot, err := store.Get[evidence.Snapshot](s, id)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ID = ""
	for path, content := range files {
		blob, err := s.PutArtifact(content, "source", MaxDefinitionBytes)
		if err != nil {
			t.Fatal(err)
		}
		snapshot.Files = append(snapshot.Files, evidence.File{Path: path, Content: blob.Content, Mode: "100644"})
	}
	stored, err := store.Put(s, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return stored.ID
}

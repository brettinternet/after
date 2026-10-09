package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

// ComparisonBasis retains the expected concrete execution identities for each
// definition case. Review bindings remain template/recipe-derived.
type ComparisonBasis struct {
	Repetitions    int
	DefinitionName string
	BuiltInPayment bool
	Cases          []Case
	Channels       []string
	planIDs        map[string][2][]string // side -> case ID -> app/observer
}

func (b ComparisonBasis) MatchesSample(sample Sample) bool {
	ids, ok := b.planIDs[sample.Side]
	if !ok {
		return false
	}
	for index, scenarioCase := range b.Cases {
		if scenarioCase.ID == sample.CaseID {
			return sample.Execution.App.Plan == ids[0][index] && sample.Execution.Observer.Plan == ids[1][index]
		}
	}
	return false
}

// ValidateComparisonBasis reconstructs frozen templates without Docker or
// project execution, then validates preparation provenance and concrete plans.
func ValidateComparisonBasis(s *store.Store, receipt evidence.Receipt) (basis ComparisonBasis, err error) {
	bad := errors.New("unsupported or incompatible frozen plan, preparation, bindings or environments")
	if receipt.RequestID == "" || receipt.Bindings == nil {
		return basis, bad
	}
	artifacts := map[string]evidence.Artifact{}
	for _, artifact := range receipt.Artifacts {
		if _, duplicate := artifacts[artifact.Channel]; duplicate {
			return basis, bad
		}
		artifacts[artifact.Channel] = artifact
	}
	planArtifact, ok := artifacts["execution-plan"]
	if !ok || planArtifact.Completeness != evidence.Complete || planArtifact.Redacted || planArtifact.Truncated {
		return basis, bad
	}
	raw, err := s.ReadBlob(planArtifact.Content)
	if err != nil {
		return basis, err
	}
	if hash(raw) != receipt.Authorization {
		return basis, bad
	}
	plan, err := PrepareFromPreview(s, raw)
	if err != nil {
		return basis, bad
	}
	bindings := evidence.Bindings{Scenario: plan.scenario.ID, Input: plan.scenario.Input, Driver: plan.scenario.Driver, Observer: plan.scenario.Observer, Rules: plan.scenario.Rules}
	if !bytes.Equal(raw, plan.preview) || *receipt.Bindings != bindings || !reflect.DeepEqual(receipt.BaseEnvironment, &plan.environments[0]) || !reflect.DeepEqual(receipt.CandidateEnvironment, &plan.environments[1]) {
		return basis, bad
	}
	launcherBytes, err := verifiedLauncher(s, artifacts, plan)
	if err != nil {
		return basis, bad
	}
	generated := map[string][]byte{"after/launcher": launcherBytes, "after/service.json": plan.serviceConfigBytes()}
	basis = ComparisonBasis{Repetitions: plan.repetitions, DefinitionName: plan.definition.Name, BuiltInPayment: plan.definitionSource.Kind == "built-in-payment", Cases: append([]Case(nil), plan.definition.Cases...), Channels: append([]string(nil), plan.definition.Channels...), planIDs: map[string][2][]string{}}
	for side, label := range []string{"base", "candidate"} {
		appIDs := make([]string, len(plan.definition.Cases))
		observerIDs := make([]string, len(plan.definition.Cases))
		for caseIndex, template := range plan.appTemplates[side] {
			concrete, materializeErr := template.Materialize(generated)
			if materializeErr != nil {
				return ComparisonBasis{}, bad
			}
			_, appIDs[caseIndex] = concrete.Preview()
			observerIDs[caseIndex] = plan.planIDs[side][caseIndex][1]
		}
		basis.planIDs[label] = [2][]string{appIDs, observerIDs}
	}
	return basis, nil
}

func verifiedLauncher(s *store.Store, artifacts map[string]evidence.Artifact, plan *Plan) ([]byte, error) {
	bad := errors.New("missing, incomplete, or incorrectly bound launcher preparation evidence")
	get := func(channel string) (evidence.Artifact, error) {
		artifact, ok := artifacts[channel]
		if !ok || artifact.Completeness != evidence.Complete || artifact.Redacted || artifact.Truncated {
			return evidence.Artifact{}, bad
		}
		return artifact, nil
	}
	prepArtifact, err := get("preparation-result")
	if err != nil {
		return nil, err
	}
	diagnosticArtifact, err := get("preparation-diagnostics")
	if err != nil {
		return nil, err
	}
	_ = diagnosticArtifact
	metadata, err := s.ReadBlob(prepArtifact.Content)
	if err != nil {
		return nil, err
	}
	var record preparationEvidence
	decoder := json.NewDecoder(bytes.NewReader(metadata))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&record) != nil || record.Version != 1 || record.Status != "completed" || record.Plan == "" || record.DefinitionDigest != plan.definitionDigest || record.Platform != plan.definition.Platform || record.Image != sandbox.Image || record.ExitCode != 0 || !record.Cleaned || record.Truncated || record.Launcher == "" || record.Bytes < 64 || record.Bytes > launcherMaxBytes {
		return nil, bad
	}
	preparationPreview, preparationID := plan.preparation.Preview()
	_ = preparationPreview
	if record.Plan != preparationID {
		return nil, bad
	}
	launcher, err := get("launcher-executable")
	if err != nil || launcher.Content != record.Launcher || launcher.Bytes != int64(record.Bytes) || launcher.MaxBytes != launcherMaxBytes {
		return nil, bad
	}
	binary, err := s.ReadBlob(launcher.Content)
	if err != nil {
		return nil, err
	}
	if len(binary) != record.Bytes || hash(binary) != record.Launcher || validateStaticELF(binary, plan.definition.Platform) != nil {
		return nil, bad
	}
	return binary, nil
}

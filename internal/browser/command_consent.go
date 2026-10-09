package browser

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
)

const commandTopology = "one fresh offline command container; build then target via direct argv; no observer, mounts, host access or network"

type commandConsentPreview struct {
	Version           int                          `json:"version"`
	Request           evidence.Digest              `json:"request"`
	Snapshots         evidence.SnapshotPair        `json:"snapshots"`
	Scenario          evidence.Scenario            `json:"scenario"`
	DefinitionDigest  evidence.Digest              `json:"definition_digest"`
	Definition        runner.CommandDefinition     `json:"definition"`
	DefinitionSource  runner.DefinitionSource      `json:"definition_source"`
	CommandConfigs    []json.RawMessage            `json:"command_configs"`
	Repetitions       int                          `json:"repetitions"`
	Limits            sandbox.Limits               `json:"limits"`
	Preparation       consentSandboxPlan           `json:"preparation"`
	PreparationBudget string                       `json:"preparation_budget"`
	Concurrency       int                          `json:"concurrency"`
	Experiments       [][]commandConsentExperiment `json:"experiments"`
}

type commandConsentExperiment struct {
	App      consentSandboxPlan `json:"App"`
	Topology string             `json:"Topology"`
}

type commandLauncherConfig struct {
	Version     int      `json:"version"`
	BuildArgv   []string `json:"build_argv,omitempty"`
	Argv        []string `json:"argv"`
	Environment []string `json:"environment"`
}

func commandPlanPreview(raw []byte) bool {
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil {
		return false
	}
	var definition struct {
		Kind string `json:"kind"`
	}
	return json.Unmarshal(root["definition"], &definition) == nil && definition.Kind == "command"
}

func commandConsentSummary(raw []byte) ([]byte, error) {
	var preview commandConsentPreview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&preview) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("invalid strict command consent preview")
	}
	if err := preview.validate(); err != nil {
		return nil, err
	}
	var summary strings.Builder
	write := func(label string, value any) error {
		encoded, err := encodeConsentValue(value)
		if err != nil {
			return err
		}
		return appendConsentLine(&summary, label, encoded)
	}
	for _, field := range []struct {
		label string
		value any
	}{
		{"Snapshots", preview.Snapshots},
		{"Command definition", preview.Definition},
		{"Definition digest", preview.DefinitionDigest},
		{"Definition source", preview.DefinitionSource},
		{"Command image/platform", []string{preview.Definition.Image, preview.Definition.Platform}},
		{"Build argv (optional; direct, no shell)", preview.Definition.BuildArgv},
		{"Command cases (argv, base64 stdin bytes, fixed environment, additive input files)", preview.Definition.Cases},
		{"Comparison policy", preview.Definition.Comparison},
		{"Definition limits", preview.Definition.Limits},
		{"Preparation recipe", preview.Preparation},
		{"Preparation budget and transfer", preview.PreparationBudget},
		{"Per-case launcher configuration", preview.CommandConfigs},
	} {
		if err := write(field.label, field.value); err != nil {
			return nil, err
		}
	}
	if preview.DefinitionSource.ChangedOracle {
		if err := appendConsentLine(&summary, "Changed oracle", "the explicitly selected definition path differs between the captured base and candidate; no snapshot copy was discovered or substituted"); err != nil {
			return nil, err
		}
	}
	if err := appendConsentLine(&summary, "Runs", fmt.Sprintf("2 sides × %d cases × %d repetitions = %d fresh command containers · concurrency %d", len(preview.Definition.Cases), preview.Repetitions, 2*len(preview.Definition.Cases)*preview.Repetitions, preview.Concurrency)); err != nil {
		return nil, err
	}
	for _, field := range []struct {
		label string
		value any
	}{
		{"Container snapshots", uniqueCommandPlanValues(preview.Experiments, func(plan consentSandboxPlan) any { return plan.Snapshot })},
		{"Input archives", uniqueCommandPlanValues(preview.Experiments, func(plan consentSandboxPlan) any { return plan.Input })},
		{"Frozen stdin digests", uniqueCommandPlanValues(preview.Experiments, func(plan consentSandboxPlan) any { return plan.StdinDigest })},
		{"Container argv", uniqueCommandPlanValues(preview.Experiments, func(plan consentSandboxPlan) any { return plan.Argv })},
		{"Container environment", uniqueCommandPlanValues(preview.Experiments, func(plan consentSandboxPlan) any { return plan.Environment })},
		{"Mounts", uniqueCommandPlanValues(preview.Experiments, func(plan consentSandboxPlan) any { return plan.Mounts })},
		{"Docker policy", uniqueCommandPlanValues(preview.Experiments, func(plan consentSandboxPlan) any { return plan.Policy })},
		{"Generated runtime slots", uniqueCommandPlanValues(preview.Experiments, func(plan consentSandboxPlan) any { return plan.GeneratedSlots })},
		{"Container limits", uniqueCommandPlanValues(preview.Experiments, func(plan consentSandboxPlan) any { return plan.Limits })},
		{"Build output and helper diagnostics", "build stdout/stderr are discarded; helper diagnostics are limited by the container output budget and occur only on incomplete runs"},
		{"Exit-status limitation", "container statuses 125–255 are incomplete (helper/build/exec failure or possible signal); intentional application exits in this range are unsupported"},
	} {
		if err := write(field.label, field.value); err != nil {
			return nil, err
		}
	}
	return []byte(summary.String()), nil
}

func (p commandConsentPreview) validate() error {
	d := p.Definition
	if p.Version != 1 || len(p.Request) != 71 || !strings.HasPrefix(string(p.Request), "sha256:") || p.Snapshots.Base == "" || p.Snapshots.Candidate == "" ||
		p.Scenario.SchemaVersion != evidence.SchemaVersion || p.Scenario.ID == "" || p.Scenario.Input != p.DefinitionDigest || p.Scenario.Author != "AFTER operator-selected command v1" ||
		p.DefinitionDigest == "" || p.Definition.Validate() != nil || p.DefinitionSource.Kind != "operator-selected-file" ||
		p.Repetitions != d.Repetitions || p.Limits.Seconds != d.Limits.Seconds || p.Limits.OutputBytes != d.Limits.OutputBytes || p.Concurrency != 1 ||
		p.PreparationBudget != fmt.Sprintf("one in-approved-request launcher build; %d seconds; stdout ≤8 MiB binary, stderr ≤%d bytes diagnostics; no cache", d.Limits.PreparationSeconds, d.Limits.OutputBytes) ||
		len(p.CommandConfigs) != len(d.Cases) || len(p.Experiments) != 2 {
		return errors.New("command consent preview fields are incomplete or outside runner bounds")
	}
	if p.DefinitionSource.RepositoryPath != "" && (path.Clean(p.DefinitionSource.RepositoryPath) != p.DefinitionSource.RepositoryPath || strings.HasPrefix(p.DefinitionSource.RepositoryPath, "/") || strings.HasPrefix(p.DefinitionSource.RepositoryPath, "../")) {
		return errors.New("command consent source path is invalid")
	}
	if err := validateSandboxPreview(p.Preparation); err != nil || p.Preparation.Image != sandbox.Image || p.Preparation.Platform != d.Platform || p.Preparation.Limits.Seconds != d.Limits.PreparationSeconds || p.Preparation.Limits.OutputBytes != d.Limits.OutputBytes {
		return errors.New("command launcher preparation preview is incomplete or incompatible")
	}
	for index, scenarioCase := range d.Cases {
		config, err := json.Marshal(commandLauncherConfig{Version: 1, BuildArgv: d.BuildArgv, Argv: scenarioCase.Argv, Environment: scenarioCase.Environment})
		if err != nil {
			return err
		}
		var actual commandLauncherConfig
		configDecoder := json.NewDecoder(bytes.NewReader(p.CommandConfigs[index]))
		configDecoder.DisallowUnknownFields()
		if configDecoder.Decode(&actual) != nil || configDecoder.Decode(new(any)) != io.EOF {
			return errors.New("invalid command launcher configuration")
		}
		actualBytes, err := json.Marshal(actual)
		if err != nil || !bytes.Equal(config, actualBytes) {
			return errors.New("command launcher configuration differs from its frozen definition")
		}
	}
	for sideIndex, side := range p.Experiments {
		if len(side) != len(d.Cases) {
			return errors.New("command experiment case count differs from the definition")
		}
		for caseIndex, experiment := range side {
			if experiment.Topology != commandTopology || validateSandboxPreview(experiment.App) != nil || experiment.App.Image != d.Image || experiment.App.Platform != d.Platform ||
				len(experiment.App.Argv) != 1 || experiment.App.Argv[0] != "/input/after/launcher" || experiment.App.StdinDigest != commandStdinDigest(d.Cases[caseIndex].Stdin) ||
				len(experiment.App.GeneratedSlots) != 2 || len(experiment.App.Materialized) != 0 ||
				!containsString(experiment.App.Policy, "--network=none") || !containsString(experiment.App.Policy, "--read-only") || !containsString(experiment.App.Policy, "--cap-drop=ALL") {
				return errors.New("command container plan differs from the frozen offline contract")
			}
			launcherSlot, configSlot := experiment.App.GeneratedSlots[0], experiment.App.GeneratedSlots[1]
			if launcherSlot.Path != "after/launcher" || launcherSlot.Producer == "" || launcherSlot.MaxBytes != 8<<20 || launcherSlot.Mode != 0555 ||
				configSlot.Path != "after/service.json" || configSlot.Producer != string(p.DefinitionDigest) || configSlot.MaxBytes != 64<<10 || configSlot.Mode != 0444 {
				return errors.New("command generated overlay differs from the fixed trusted slots")
			}
			if sideIndex == 0 && experiment.App.Snapshot != string(p.Snapshots.Base) || sideIndex == 1 && experiment.App.Snapshot != string(p.Snapshots.Candidate) {
				return errors.New("command container snapshot does not match its side")
			}
		}
	}
	return nil
}

func commandStdinDigest(input []byte) string {
	sum := sha256.Sum256(input)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func uniqueCommandPlanValues(experiments [][]commandConsentExperiment, selectValue func(consentSandboxPlan) any) []any {
	values := []any{}
	seen := map[string]bool{}
	for _, side := range experiments {
		for _, experiment := range side {
			value := selectValue(experiment.App)
			encoded, _ := encodeConsentValue(value)
			if !seen[encoded] {
				seen[encoded] = true
				values = append(values, value)
			}
		}
	}
	return values
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

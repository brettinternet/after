package browser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/terminal"
)

// These structs mirror the runner's preview and nested sandbox previews.
type consentPreview struct {
	Version           int                     `json:"version"`
	Request           evidence.Digest         `json:"request"`
	Snapshots         evidence.SnapshotPair   `json:"snapshots"`
	Scenario          evidence.Scenario       `json:"scenario"`
	DefinitionDigest  evidence.Digest         `json:"definition_digest"`
	Definition        runner.Definition       `json:"definition"`
	DefinitionSource  runner.DefinitionSource `json:"definition_source"`
	ServiceConfig     json.RawMessage         `json:"service_config"`
	Repetitions       int                     `json:"repetitions"`
	Limits            sandbox.Limits          `json:"limits"`
	Preparation       consentSandboxPlan      `json:"preparation"`
	PreparationBudget string                  `json:"preparation_budget"`
	Concurrency       int                     `json:"concurrency"`
	Experiments       [][]consentExperiment   `json:"experiments"`
}

type consentExperiment struct {
	App      consentSandboxPlan `json:"App"`
	Observer consentSandboxPlan `json:"Observer"`
	Topology string             `json:"Topology"`
}

type consentSandboxPlan struct {
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

func decodeConsentPreview(raw []byte) (consentPreview, error) {
	var preview consentPreview
	if len(raw) == 0 || len(raw) > 1<<20 || !utf8.Valid(raw) {
		return preview, errors.New("preview is empty, oversized, or invalid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&preview); err != nil {
		return consentPreview{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return consentPreview{}, errors.New("preview must contain exactly one JSON value")
	}
	if err := preview.validate(); err != nil {
		return consentPreview{}, err
	}
	return preview, nil
}

func (p consentPreview) validate() error {
	if p.Version != 1 || p.Request == "" || p.Snapshots.Base == "" || p.Snapshots.Candidate == "" ||
		p.Scenario.SchemaVersion != evidence.SchemaVersion || p.Scenario.ID == "" || p.Scenario.Input == "" ||
		p.Scenario.Driver == "" || p.Scenario.Observer == "" || p.Scenario.Rules == "" ||
		p.Scenario.Boundary == "" || p.Scenario.Author == "" || len(p.Scenario.Limits) == 0 ||
		p.DefinitionDigest == "" || p.Scenario.Input != p.DefinitionDigest || p.Definition.Validate() != nil ||
		(p.DefinitionSource.Kind != "built-in-payment" && p.DefinitionSource.Kind != "operator-selected-file") || len(p.ServiceConfig) == 0 || len(p.ServiceConfig) > 64<<10 || p.Repetitions != p.Definition.Repetitions ||
		p.Limits.Seconds != p.Definition.Limits.Seconds || p.Limits.OutputBytes != p.Definition.Limits.OutputBytes ||
		p.Concurrency != 1 || p.PreparationBudget != fmt.Sprintf("one in-approved-request launcher build; %d seconds; stdout ≤8 MiB binary, stderr ≤%d bytes diagnostics; no cache", p.Definition.Limits.PreparationSeconds, p.Definition.Limits.OutputBytes) || len(p.Experiments) != 2 {
		return errors.New("preview fields are incomplete or outside runner bounds")
	}
	if err := validateSandboxPreview(p.Preparation); err != nil || p.Preparation.Image != sandbox.Image || p.Preparation.Platform != p.Definition.Platform || p.Preparation.Limits.Seconds != p.Definition.Limits.PreparationSeconds {
		return errors.New("launcher preparation preview is incomplete or incompatible")
	}
	if p.Preparation.Limits.OutputBytes != p.Definition.Limits.OutputBytes {
		return errors.New("launcher preparation output limit changed")
	}
	for _, side := range p.Experiments {
		if len(side) != len(p.Definition.Cases) {
			return errors.New("preview case count differs from selected definition")
		}
		for caseIndex, experiment := range side {
			if experiment.Topology == "" {
				return errors.New("observer topology is missing")
			}
			if validateSandboxPreview(experiment.App) != nil || validateSandboxPreview(experiment.Observer) != nil {
				return errors.New("nested sandbox preview is incomplete")
			}
			if experiment.Observer.Image != sandbox.Image || len(experiment.App.GeneratedSlots) != 2 || len(experiment.App.Materialized) != 0 || len(experiment.Observer.GeneratedSlots) != 0 || len(experiment.Observer.Materialized) != 0 {
				return errors.New("service or observer template differs from approved definition")
			}
			launcherSlot, configSlot := experiment.App.GeneratedSlots[0], experiment.App.GeneratedSlots[1]
			if launcherSlot.Path != "after/launcher" || launcherSlot.Producer == "" || launcherSlot.MaxBytes != 8<<20 || launcherSlot.Mode != 0555 || configSlot.Path != "after/service.json" || configSlot.Producer != string(p.DefinitionDigest) || configSlot.MaxBytes != 64<<10 || configSlot.Mode != 0444 {
				return errors.New("generated runtime overlay differs from the fixed trusted slots")
			}
			if len(experiment.App.Argv) != 1 || experiment.App.Argv[0] != "/input/after/launcher" || len(experiment.Observer.Argv) != 5 || experiment.Observer.Argv[0] != "/usr/local/go/bin/go" || experiment.Observer.Argv[1] != "run" || experiment.Observer.Argv[2] != "/input/observer.go" || experiment.Observer.Argv[3] != "/input/definition.json" || experiment.Observer.Argv[4] != p.Definition.Cases[caseIndex].ID {
				return errors.New("driver argv differs from the shipped observer")
			}
		}
	}
	return nil
}

func validateSandboxPreview(plan consentSandboxPlan) error {
	if plan.Version != 1 || plan.Preparation == "" || plan.Mounts == "" || plan.Image == "" || plan.Platform == "" ||
		plan.Snapshot == "" || plan.Input == "" || len(plan.Argv) == 0 || len(plan.Environment) == 0 || len(plan.Policy) == 0 ||
		plan.Limits.Seconds < 1 || plan.Limits.Seconds > 300 || plan.Limits.OutputBytes < 1 || plan.Limits.OutputBytes > 1<<20 {
		return errors.New("incomplete nested plan")
	}
	return nil
}

func ConsentSummary(raw []byte) ([]byte, error) {
	if commandPlanPreview(raw) {
		return commandConsentSummary(raw)
	}
	preview, err := decodeConsentPreview(raw)
	if err != nil {
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
	writeSandboxPlans := func(label string, selectValue func(consentSandboxPlan) any) error {
		values := []any{}
		seen := map[string]bool{}
		for _, side := range preview.Experiments {
			for _, experiment := range side {
				for _, plan := range []consentSandboxPlan{experiment.App, experiment.Observer} {
					value := selectValue(plan)
					encoded, err := encodeConsentValue(value)
					if err != nil {
						return err
					}
					if !seen[encoded] {
						seen[encoded] = true
						values = append(values, value)
					}
				}
			}
		}
		return write(label, values)
	}
	if err := write("Snapshots", preview.Snapshots); err != nil {
		return nil, err
	}
	if err := write("Definition", preview.Definition); err != nil {
		return nil, err
	}
	if err := write("Definition digest", preview.DefinitionDigest); err != nil {
		return nil, err
	}
	if err := write("Definition source", preview.DefinitionSource); err != nil {
		return nil, err
	}
	if preview.DefinitionSource.ChangedOracle {
		if err := appendConsentLine(&summary, "Changed oracle", "the selected definition path differs between the captured base and candidate; the selected file was not discovered or replaced from either snapshot"); err != nil {
			return nil, err
		}
	}
	repetitions := "repetitions"
	if preview.Repetitions == 1 {
		repetitions = "repetition"
	}
	if err := appendConsentLine(&summary, "Runs", fmt.Sprintf("2 sides × %d cases × %d %s = %d runs · concurrency %d", len(preview.Definition.Cases), preview.Repetitions, repetitions, 2*len(preview.Definition.Cases)*preview.Repetitions, preview.Concurrency)); err != nil {
		return nil, err
	}
	for _, field := range []struct {
		label      string
		value      any
		selectPlan func(consentSandboxPlan) any
	}{
		{"Service image/platform", []string{preview.Definition.Image, preview.Definition.Platform}, nil},
		{"Build argv (optional)", preview.Definition.BuildArgv, nil},
		{"Start argv", preview.Definition.StartArgv, nil},
		{"Service environment", preview.Definition.Environment, nil},
		{"Readiness", preview.Definition.Readiness, nil},
		{"Fake upstreams", preview.Definition.Upstreams, nil},
		{"Controlled-clock requests", preview.Definition.Cases, nil},
		{"Compared channels", preview.Definition.Channels, nil},
		{"Definition limits", preview.Definition.Limits, nil},
		{"Service configuration", preview.ServiceConfig, nil},
		{"Preparation recipe", preview.Preparation, nil},
		{"Preparation budget and transfer", preview.PreparationBudget, nil},
		{"Observer topology", topologySummary(preview.Experiments), nil},
		{"App and observer images", planImages(preview.Experiments), nil},
		{"Container snapshots", nil, func(plan consentSandboxPlan) any { return plan.Snapshot }},
		{"Input archives", nil, func(plan consentSandboxPlan) any { return plan.Input }},
		{"Container argv", nil, func(plan consentSandboxPlan) any { return plan.Argv }},
		{"Container environment", nil, func(plan consentSandboxPlan) any { return plan.Environment }},
		{"Mounts", nil, func(plan consentSandboxPlan) any { return plan.Mounts }},
		{"Docker policy", nil, func(plan consentSandboxPlan) any { return plan.Policy }},
		{"Generated runtime slots", nil, func(plan consentSandboxPlan) any { return plan.GeneratedSlots }},
		{"Materialized runtime files", nil, func(plan consentSandboxPlan) any { return plan.Materialized }},
		{"Container limits", nil, func(plan consentSandboxPlan) any { return plan.Limits }},
	} {
		if field.selectPlan != nil {
			if err := writeSandboxPlans(field.label, field.selectPlan); err != nil {
				return nil, err
			}
		} else if err := write(field.label, field.value); err != nil {
			return nil, err
		}
	}
	return []byte(summary.String()), nil
}

func topologySummary(experiments [][]consentExperiment) []string {
	var values []string
	for _, side := range experiments {
		for _, experiment := range side {
			values = append(values, experiment.Topology)
		}
	}
	return values
}

func planImages(experiments [][]consentExperiment) []string {
	values := []string{}
	seen := map[string]bool{}
	for _, side := range experiments {
		for _, experiment := range side {
			for _, image := range []string{experiment.App.Image + " / " + experiment.App.Platform, experiment.Observer.Image + " / " + experiment.Observer.Platform} {
				if !seen[image] {
					seen[image] = true
					values = append(values, image)
				}
			}
		}
	}
	return values
}

func encodeConsentValue(value any) (string, error) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(encoded.String(), "\n"), nil
}

func appendConsentLine(summary *strings.Builder, label, value string) error {
	if summary.Len()+len(label)+len(value)+2 > terminal.MaxTextBytes {
		return errors.New("consent summary exceeds terminal document bounds")
	}
	summary.WriteString(label)
	summary.WriteString(": ")
	summary.WriteString(value)
	summary.WriteByte('\n')
	return nil
}

func previewSize(size int) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	return fmt.Sprintf("%.1f KiB", float64(size)/1024)
}

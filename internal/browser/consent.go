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
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/terminal"
)

// These structs mirror the runner's preview and its nested sandbox previews.
// Keeping every field typed lets DisallowUnknownFields reject schema drift at
// every level instead of trusting raw experiment fragments.
type consentPreview struct {
	Version        int                   `json:"version"`
	Request        evidence.Digest       `json:"request"`
	Snapshots      evidence.SnapshotPair `json:"snapshots"`
	Scenario       evidence.Scenario     `json:"scenario"`
	Repetitions    int                   `json:"repetitions"`
	Limits         sandbox.Limits        `json:"limits"`
	Concurrency    int                   `json:"concurrency"`
	Experiments    [][]consentExperiment `json:"experiments"`
	BuildArgv      []string              `json:"build_argv"`
	AppArgv        []string              `json:"app_argv"`
	AppEnvironment []string              `json:"app_environment"`
}

type consentExperiment struct {
	App      consentSandboxPlan `json:"App"`
	Observer consentSandboxPlan `json:"Observer"`
	Topology string             `json:"Topology"`
}

type consentSandboxPlan struct {
	Version     int            `json:"version"`
	Preparation string         `json:"preparation"`
	Mounts      string         `json:"mounts"`
	Image       string         `json:"image"`
	Snapshot    string         `json:"snapshot"`
	Input       string         `json:"input_archive"`
	Argv        []string       `json:"argv"`
	Environment []string       `json:"environment"`
	Policy      []string       `json:"docker_policy"`
	Limits      sandbox.Limits `json:"limits"`
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
		p.Repetitions < 1 || p.Repetitions > 5 || p.Limits.Seconds < 1 || p.Limits.Seconds > 300 ||
		p.Limits.OutputBytes < 1 || p.Limits.OutputBytes > 1<<20 || p.Concurrency != 1 ||
		len(p.BuildArgv) == 0 || len(p.AppArgv) == 0 || len(p.AppEnvironment) == 0 {
		return errors.New("preview fields are incomplete or outside runner bounds")
	}
	if len(p.Experiments) != 2 {
		return errors.New("preview must contain exactly two experiment sides")
	}
	for _, experiment := range p.Experiments {
		if len(experiment) != 2 {
			return errors.New("preview must contain exactly two cases per side")
		}
		for _, sample := range experiment {
			if sample.Topology == "" {
				return errors.New("experiment topology is missing")
			}
			for _, plan := range []consentSandboxPlan{sample.App, sample.Observer} {
				if plan.Version != 1 || plan.Preparation == "" || plan.Mounts == "" || plan.Image == "" ||
					plan.Snapshot == "" || plan.Input == "" || len(plan.Argv) == 0 ||
					len(plan.Environment) == 0 || len(plan.Policy) == 0 || plan.Limits.Seconds < 1 ||
					plan.Limits.Seconds > 300 || plan.Limits.OutputBytes < 1 || plan.Limits.OutputBytes > 1<<20 {
					return errors.New("nested sandbox preview is incomplete")
				}
			}
		}
	}
	return nil
}

func consentSummary(raw []byte) ([]byte, error) {
	preview, err := decodeConsentPreview(raw)
	if err != nil {
		return nil, err
	}

	plans := make([]consentSandboxPlan, 0, 8)
	topologies := make([]any, 0, 4)
	for _, side := range preview.Experiments {
		for _, experiment := range side {
			topologies = append(topologies, experiment.Topology)
			plans = append(plans, experiment.App, experiment.Observer)
		}
	}
	collect := func(selectValue func(consentSandboxPlan) any) []any {
		values := make([]any, len(plans))
		for i, plan := range plans {
			values[i] = selectValue(plan)
		}
		return values
	}

	var summary strings.Builder
	write := func(label string, value any) error {
		encoded, err := encodeConsentValue(value)
		if err != nil {
			return err
		}
		return appendConsentLine(&summary, label, encoded)
	}
	writeDistinct := func(label string, values []any) error {
		encoded, err := distinctConsentValues(values)
		if err != nil {
			return err
		}
		return appendConsentLine(&summary, label, encoded)
	}

	if err := write("Snapshots", preview.Snapshots); err != nil {
		return nil, err
	}
	sides, cases := len(preview.Experiments), len(preview.Experiments[0])
	runs := sides * cases * preview.Repetitions
	repetitions := "repetitions"
	if preview.Repetitions == 1 {
		repetitions = "repetition"
	}
	if err := appendConsentLine(&summary, "Runs", fmt.Sprintf("%d sides × %d cases × %d %s = %d runs · concurrency %d", sides, cases, preview.Repetitions, repetitions, runs, preview.Concurrency)); err != nil {
		return nil, err
	}
	for _, field := range []struct {
		label string
		value any
	}{
		{"Build argv", preview.BuildArgv},
		{"App argv", preview.AppArgv},
		{"App environment", preview.AppEnvironment},
		{"Preparation", collect(func(plan consentSandboxPlan) any { return plan.Preparation })},
		{"Container snapshots", collect(func(plan consentSandboxPlan) any { return plan.Snapshot })},
		{"Input archives", collect(func(plan consentSandboxPlan) any { return plan.Input })},
		{"Container argv", collect(func(plan consentSandboxPlan) any { return plan.Argv })},
		{"Container environment", collect(func(plan consentSandboxPlan) any { return plan.Environment })},
		{"Images", collect(func(plan consentSandboxPlan) any { return plan.Image })},
		{"Topology", topologies},
		{"Mounts", collect(func(plan consentSandboxPlan) any { return plan.Mounts })},
		{"Docker policy", collect(func(plan consentSandboxPlan) any { return plan.Policy })},
		{"Run limits", preview.Limits},
		{"Container limits", collect(func(plan consentSandboxPlan) any { return plan.Limits })},
	} {
		values, ok := field.value.([]any)
		if !ok {
			if err := write(field.label, field.value); err != nil {
				return nil, err
			}
			continue
		}
		if err := writeDistinct(field.label, values); err != nil {
			return nil, err
		}
	}
	return []byte(summary.String()), nil
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

func distinctConsentValues(values []any) (string, error) {
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		encoded, err := encodeConsentValue(value)
		if err != nil {
			return "", err
		}
		if _, found := seen[encoded]; found {
			continue
		}
		seen[encoded] = struct{}{}
		unique = append(unique, encoded)
	}
	return strings.Join(unique, " | "), nil
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

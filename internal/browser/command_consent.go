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
	"strconv"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/terminal"
	"github.com/rivo/uniseg"
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

const (
	commandConsentLineWidth       = 70
	commandConsentValueWidth      = 42
	commandConsentStdinPreviewMax = 12
	commandConsentPlanIDPartWidth = 64
)

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
	writeText := func(text string) error {
		for _, line := range terminal.Wrap(text, commandConsentLineWidth) {
			if summary.Len()+len(line)+1 > terminal.MaxTextBytes {
				return errors.New("consent summary exceeds terminal document bounds")
			}
			summary.WriteString(line)
			summary.WriteByte('\n')
		}
		return nil
	}
	writeQuoted := func(label, value string, width int) error {
		return writeText(label + ": " + boundedCommandQuote(value, width))
	}
	writeDetail := func(label, value string, width int) error {
		return writeQuoted("  "+label, value, width)
	}
	writeArgv := func(label string, argv []string) error {
		if err := writeText(label + ":"); err != nil {
			return err
		}
		if len(argv) == 0 {
			return writeText("  (empty)")
		}
		for index, arg := range argv {
			if err := writeDetail(fmt.Sprintf("argv[%d]", index), arg, commandConsentValueWidth); err != nil {
				return err
			}
		}
		return nil
	}

	planID := commandPlanDigest(raw)
	for _, line := range []string{
		"Command consent · each case below is frozen for base and candidate",
		"Plan ID (concatenate the next two lines without spaces):",
		planID[:commandConsentPlanIDPartWidth],
		planID[commandConsentPlanIDPartWidth:],
		"Inspect full plan: after inspect PLAN (use the joined Plan ID)",
		"Inspect exact JSON bytes: after inspect PLAN --json",
		"Replace PLAN with the two ID parts joined; JSON is untruncated.",
	} {
		if err := writeText(line); err != nil {
			return nil, err
		}
	}
	if err := writeText("Snapshots:"); err != nil {
		return nil, err
	}
	if err := writeQuoted("  base", string(preview.Snapshots.Base), commandConsentValueWidth); err != nil {
		return nil, err
	}
	if err := writeQuoted("  candidate", string(preview.Snapshots.Candidate), commandConsentValueWidth); err != nil {
		return nil, err
	}
	if err := writeQuoted("Command definition", preview.Definition.Name, commandConsentValueWidth); err != nil {
		return nil, err
	}
	if err := writeQuoted("Definition digest", string(preview.DefinitionDigest), commandConsentValueWidth); err != nil {
		return nil, err
	}
	if err := writeQuoted("Definition source", preview.DefinitionSource.Kind, commandConsentValueWidth); err != nil {
		return nil, err
	}
	if preview.DefinitionSource.RepositoryPath != "" {
		if err := writeQuoted("  selected path", preview.DefinitionSource.RepositoryPath, commandConsentValueWidth); err != nil {
			return nil, err
		}
	}
	if preview.DefinitionSource.ChangedOracle {
		if err := writeText("Changed oracle: selected definition differs between snapshots; neither snapshot copy was substituted."); err != nil {
			return nil, err
		}
	}
	imageRepository, imageDigest, _ := strings.Cut(preview.Definition.Image, "@sha256:")
	if err := writeText("Image: " + imageRepository); err != nil {
		return nil, err
	}
	if err := writeText("Image SHA-256:"); err != nil {
		return nil, err
	}
	if err := writeText("  " + imageDigest); err != nil {
		return nil, err
	}
	if err := writeText("Platform: " + preview.Definition.Platform); err != nil {
		return nil, err
	}
	if err := writeArgv("Build argv (direct; no shell)", preview.Definition.BuildArgv); err != nil {
		return nil, err
	}
	for index, scenarioCase := range preview.Definition.Cases {
		if err := writeText(fmt.Sprintf("Case %d:", index+1)); err != nil {
			return nil, err
		}
		if err := writeDetail("id", scenarioCase.ID, commandConsentValueWidth); err != nil {
			return nil, err
		}
		if err := writeDetail("title", scenarioCase.Title, commandConsentValueWidth); err != nil {
			return nil, err
		}
		if err := writeArgv("  argv", scenarioCase.Argv); err != nil {
			return nil, err
		}
		if len(scenarioCase.Environment) == 0 {
			if err := writeText("  environment: (empty fixed environment)"); err != nil {
				return nil, err
			}
		} else {
			for envIndex, entry := range scenarioCase.Environment {
				if err := writeDetail(fmt.Sprintf("environment[%d]", envIndex), entry, commandConsentValueWidth); err != nil {
					return nil, err
				}
			}
		}
		if len(scenarioCase.InputFiles) == 0 {
			if err := writeText("  input files: (none)"); err != nil {
				return nil, err
			}
		} else {
			for fileIndex, input := range scenarioCase.InputFiles {
				if err := writeText(fmt.Sprintf("  input[%d]: path=%s · size=%d B", fileIndex, boundedCommandQuote(input.Path, 30), len(input.Content))); err != nil {
					return nil, err
				}
			}
		}
		if err := writeText("  stdin: size=" + fmt.Sprintf("%d B", len(scenarioCase.Stdin)) + " · preview=" + commandStdinPreview(scenarioCase.Stdin)); err != nil {
			return nil, err
		}
	}
	if err := writeText(fmt.Sprintf("Comparison: stdout=%s · stderr=%s · exit status=exact", preview.Definition.Comparison.Stdout, preview.Definition.Comparison.Stderr)); err != nil {
		return nil, err
	}
	if err := writeText(fmt.Sprintf("Definition limits: run %ds · output %d B · preparation %ds", preview.Definition.Limits.Seconds, preview.Definition.Limits.OutputBytes, preview.Definition.Limits.PreparationSeconds)); err != nil {
		return nil, err
	}
	if err := writeText(fmt.Sprintf("Runs: 2 sides × %d cases × %d repetitions = %d fresh command containers · concurrency %d", len(preview.Definition.Cases), preview.Repetitions, 2*len(preview.Definition.Cases)*preview.Repetitions, preview.Concurrency)); err != nil {
		return nil, err
	}

	if err := writeText("Launcher preparation:"); err != nil {
		return nil, err
	}
	if err := writeText("  image/platform: " + strconv.Quote(preview.Preparation.Image+" / "+preview.Preparation.Platform)); err != nil {
		return nil, err
	}
	if err := writeArgv("  argv", preview.Preparation.Argv); err != nil {
		return nil, err
	}
	if err := writeText("  recipe: " + boundedCommandQuote(preview.Preparation.Preparation, 58)); err != nil {
		return nil, err
	}
	if err := writeText("  mounts: " + boundedCommandQuote(preview.Preparation.Mounts, 58)); err != nil {
		return nil, err
	}
	if err := writeText("  limits: " + formatSandboxLimits(preview.Preparation.Limits)); err != nil {
		return nil, err
	}
	if err := writeText("  budget: " + strconv.Quote(preview.PreparationBudget)); err != nil {
		return nil, err
	}
	if err := writeCommandPlanGroups(&summary, "Launcher environment", preview.Experiments, func(plan consentSandboxPlan) []string { return plan.Environment }); err != nil {
		return nil, err
	}
	if err := writeCommandPolicies(&summary, "Launcher Docker policy", [][]consentSandboxPlan{{preview.Preparation}}); err != nil {
		return nil, err
	}
	if err := writeText("Command containers: one fresh offline container per side/case/repetition; no observer, host access, mounts or network."); err != nil {
		return nil, err
	}
	if err := writeText("  launcher argv: /input/after/launcher"); err != nil {
		return nil, err
	}
	if err := writeCommandPolicyGroups(&summary, "Docker policy (command containers)", preview.Experiments); err != nil {
		return nil, err
	}
	if err := writeCommandPlanGroups(&summary, "Container mounts", preview.Experiments, func(plan consentSandboxPlan) []string { return []string{plan.Mounts} }); err != nil {
		return nil, err
	}
	if err := writeCommandPlanGroups(&summary, "Generated runtime slots", preview.Experiments, func(plan consentSandboxPlan) []string {
		slots := make([]string, len(plan.GeneratedSlots))
		for index, slot := range plan.GeneratedSlots {
			slots[index] = fmt.Sprintf("%s · mode=%04o · max=%d B · producer=%s", slot.Path, slot.Mode, slot.MaxBytes, slot.Producer)
		}
		return slots
	}); err != nil {
		return nil, err
	}
	if err := writeCommandPlanGroups(&summary, "Container limits", preview.Experiments, func(plan consentSandboxPlan) []string { return []string{formatSandboxLimits(plan.Limits)} }); err != nil {
		return nil, err
	}
	if err := writeCommandInputArchives(&summary, preview); err != nil {
		return nil, err
	}
	if err := writeText("Build stdout/stderr are discarded; helper diagnostics are bounded and occur only on incomplete runs."); err != nil {
		return nil, err
	}
	if err := writeText("Exit statuses 125–255 are incomplete; intentional application exits in that range are unsupported."); err != nil {
		return nil, err
	}
	return []byte(summary.String()), nil
}

func commandPlanDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func boundedCommandQuote(raw string, width int) string {
	quoted := terminal.Sanitize(strconv.Quote(raw))
	if uniseg.StringWidth(quoted) <= width {
		return quoted
	}
	sum := sha256.Sum256([]byte(raw))
	marker := fmt.Sprintf("…#%x\"", sum[:4])
	prefixWidth := max(1, width-uniseg.StringWidth(marker))
	prefix := strings.TrimSuffix(terminal.Line(quoted, prefixWidth), "…")
	return prefix + marker
}

func commandStdinPreview(stdin []byte) string {
	previewBytes := stdin[:min(len(stdin), commandConsentStdinPreviewMax)]
	preview := boundedCommandQuote(string(previewBytes), 30)
	if len(stdin) > len(previewBytes) {
		sum := sha256.Sum256(stdin)
		preview += fmt.Sprintf("…#%x", sum[:4])
	}
	return preview
}

func formatSandboxLimits(limits sandbox.Limits) string {
	return fmt.Sprintf("%d seconds · %d output bytes", limits.Seconds, limits.OutputBytes)
}

func writeCommandPlanGroups(summary *strings.Builder, label string, experiments [][]commandConsentExperiment, selectValue func(consentSandboxPlan) []string) error {
	type group struct {
		values []string
		cases  []string
	}
	groups := []group{}
	indexes := map[string]int{}
	for sideIndex, side := range experiments {
		sideName := "base"
		if sideIndex == 1 {
			sideName = "candidate"
		}
		for caseIndex, experiment := range side {
			values := selectValue(experiment.App)
			encoded, err := encodeConsentValue(values)
			if err != nil {
				return err
			}
			groupIndex, ok := indexes[encoded]
			if !ok {
				groupIndex = len(groups)
				indexes[encoded] = groupIndex
				groups = append(groups, group{values: values})
			}
			groups[groupIndex].cases = append(groups[groupIndex].cases, fmt.Sprintf("%s/%d", sideName, caseIndex+1))
		}
	}
	for index, group := range groups {
		if len(groups) == 1 {
			if err := appendCommandConsentText(summary, label+" (all command cases):"); err != nil {
				return err
			}
		} else if err := appendCommandConsentText(summary, fmt.Sprintf("%s group %d (%s):", label, index+1, strings.Join(group.cases, ", "))); err != nil {
			return err
		}
		if len(group.values) == 0 {
			if err := appendCommandConsentText(summary, "  (empty)"); err != nil {
				return err
			}
		}
		for valueIndex, value := range group.values {
			if err := appendCommandConsentText(summary, fmt.Sprintf("  [%d] %s", valueIndex, boundedCommandQuote(value, 62))); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeCommandPolicies(summary *strings.Builder, label string, sides [][]consentSandboxPlan) error {
	for sideIndex, plans := range sides {
		for caseIndex, plan := range plans {
			prefix := fmt.Sprintf("%s:", label)
			if len(plans) > 1 || len(sides) > 1 {
				sideName := "base"
				if sideIndex == 1 {
					sideName = "candidate"
				}
				prefix = fmt.Sprintf("%s (%s/%d):", label, sideName, caseIndex+1)
			}
			if err := appendCommandConsentText(summary, prefix); err != nil {
				return err
			}
			for _, flag := range plan.Policy {
				if err := appendCommandConsentText(summary, "  "+strconv.Quote(flag)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func writeCommandPolicyGroups(summary *strings.Builder, label string, experiments [][]commandConsentExperiment) error {
	type group struct {
		policy []string
		cases  []string
	}
	groups := []group{}
	indexes := map[string]int{}
	for sideIndex, side := range experiments {
		sideName := "base"
		if sideIndex == 1 {
			sideName = "candidate"
		}
		for caseIndex, experiment := range side {
			policy := experiment.App.Policy
			encoded, err := encodeConsentValue(policy)
			if err != nil {
				return err
			}
			groupIndex, ok := indexes[encoded]
			if !ok {
				groupIndex = len(groups)
				indexes[encoded] = groupIndex
				groups = append(groups, group{policy: policy})
			}
			groups[groupIndex].cases = append(groups[groupIndex].cases, fmt.Sprintf("%s/%d", sideName, caseIndex+1))
		}
	}
	for index, group := range groups {
		if len(groups) == 1 {
			if err := appendCommandConsentText(summary, label+" (all command cases):"); err != nil {
				return err
			}
		} else if err := appendCommandConsentText(summary, fmt.Sprintf("%s group %d (%s):", label, index+1, strings.Join(group.cases, ", "))); err != nil {
			return err
		}
		for _, flag := range group.policy {
			if err := appendCommandConsentText(summary, "  "+strconv.Quote(flag)); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeCommandInputArchives(summary *strings.Builder, preview commandConsentPreview) error {
	for sideIndex, side := range preview.Experiments {
		sideName := "base"
		if sideIndex == 1 {
			sideName = "candidate"
		}
		for caseIndex, experiment := range side {
			line := fmt.Sprintf("Input archive %s/case[%d]: %s", sideName, caseIndex+1, boundedCommandQuote(experiment.App.Input, commandConsentValueWidth))
			if err := appendCommandConsentText(summary, line); err != nil {
				return err
			}
		}
	}
	return nil
}

func appendCommandConsentText(summary *strings.Builder, text string) error {
	for _, line := range terminal.Wrap(text, commandConsentLineWidth) {
		if summary.Len()+len(line)+1 > terminal.MaxTextBytes {
			return errors.New("consent summary exceeds terminal document bounds")
		}
		summary.WriteString(line)
		summary.WriteByte('\n')
	}
	return nil
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

package runner

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

type commandLauncherConfig struct {
	Version     int      `json:"version"`
	BuildArgv   []string `json:"build_argv,omitempty"`
	Argv        []string `json:"argv"`
	Environment []string `json:"environment"`
}

type commandExperimentPreview struct {
	App      json.RawMessage `json:"App"`
	Topology string          `json:"Topology"`
}

func prepareCommand(s *store.Store, pair evidence.SnapshotPair, raw []byte, source DefinitionSource, request evidence.Digest) (*Plan, error) {
	definition, err := ParseCommandDefinition(raw)
	if err != nil {
		return nil, err
	}
	if pair.Base == "" || pair.Candidate == "" {
		return nil, errors.New("base and candidate snapshots required")
	}
	if request != "" && !validRequestID(request) {
		return nil, errors.New("invalid saved request identity")
	}
	if err := validateDefinitionSource(source); err != nil || source.Kind != "operator-selected-file" {
		return nil, errors.Join(err, errors.New("command definition must be explicitly selected"))
	}
	snapshots := [2]evidence.Snapshot{}
	for side, id := range []evidence.Digest{pair.Base, pair.Candidate} {
		snapshot, err := store.Get[evidence.Snapshot](s, id)
		if err != nil {
			return nil, err
		}
		if err := checkSnapshot(snapshot); err != nil {
			return nil, err
		}
		snapshots[side] = snapshot
	}
	if source.RepositoryPath != "" {
		source.ChangedOracle = definitionChanged(s, snapshots[0], snapshots[1], source.RepositoryPath)
	}
	definitionArtifact, err := s.PutArtifact(raw, "command-definition", MaxDefinitionBytes)
	if err != nil || definitionArtifact.Completeness != evidence.Complete {
		return nil, errors.Join(err, errors.New("complete frozen command definition could not be retained"))
	}
	if request == "" {
		nonce := make([]byte, 32)
		if _, err := rand.Read(nonce); err != nil {
			return nil, err
		}
		request = hash(nonce)
	}
	definitionDigest := hash(raw)
	rulesDigest := hash([]byte(CommandComparisonRules))
	driverDigest := hash(append(append([]byte(nil), preparer...), commandRunner...))
	observerDigest := hash([]byte("no observer container; Docker command boundary v1"))
	scenario, err := store.Put(s, evidence.Scenario{
		SchemaVersion: evidence.SchemaVersion,
		Input:         definitionArtifact.Content, Driver: driverDigest, Observer: observerDigest, Rules: rulesDigest,
		Boundary: "Docker-inspected container exit status and separately attached stdout/stderr from the frozen command; no observer container",
		Author:   "AFTER operator-selected command v1", Limits: commandLimits(definition),
	})
	if err != nil {
		return nil, err
	}
	limits := sandbox.Limits{Seconds: definition.Limits.Seconds, OutputBytes: definition.Limits.OutputBytes}
	p := &Plan{
		request: request, pair: pair, scenario: scenario, definitionRaw: append([]byte(nil), raw...),
		definitionDigest: definitionDigest, definitionSource: source, command: &definition,
		repetitions: definition.Repetitions, limits: limits,
		commandConfigs: make([][]byte, len(definition.Cases)),
	}
	for side := range p.commandTemplates {
		p.commandTemplates[side] = make([]*sandbox.Plan, len(definition.Cases))
	}
	preparationLimits := sandbox.Limits{Seconds: definition.Limits.PreparationSeconds, OutputBytes: definition.Limits.OutputBytes}
	p.preparation, err = sandbox.PrepareImage(string(definitionDigest), map[string][]byte{"preparer.go": preparer, "launch.go": commandRunner}, []string{"/usr/local/go/bin/go", "run", "/input/preparer.go"}, preparationLimits, sandbox.Image, definition.Platform)
	if err != nil {
		return nil, err
	}
	preparationPreview, preparationID := p.preparation.Preview()
	_ = preparationPreview
	var environmentHashes [2][]byte
	for caseIndex, scenarioCase := range definition.Cases {
		config, err := json.Marshal(commandLauncherConfig{Version: 1, BuildArgv: append([]string(nil), definition.BuildArgv...), Argv: append([]string(nil), scenarioCase.Argv...), Environment: append([]string{}, scenarioCase.Environment...)})
		if err != nil || len(config) > 64<<10 {
			return nil, errors.Join(err, errors.New("trusted command configuration exceeds budget"))
		}
		p.commandConfigs[caseIndex] = config
		for side, snapshot := range snapshots {
			files := make(map[string][]byte, len(snapshot.Files)+len(scenarioCase.InputFiles))
			total := 0
			for _, file := range snapshot.Files {
				data, readErr := s.ReadBlob(file.Content)
				if readErr != nil {
					return nil, readErr
				}
				files[file.Path] = data
				total += len(data)
			}
			if total > 8<<20 {
				return nil, ErrSnapshotBudget
			}
			for _, input := range scenarioCase.InputFiles {
				for existing := range files {
					if pathsConflict(existing, input.Path) {
						return nil, fmt.Errorf("command input %q collides with captured path %q", input.Path, existing)
					}
				}
				files[input.Path] = append([]byte(nil), input.Content...)
				total += len(input.Content)
			}
			if total > 8<<20 {
				return nil, ErrSnapshotBudget
			}
			slots := []sandbox.GeneratedFile{
				{Path: "after/launcher", Producer: preparationID, MaxBytes: launcherMaxBytes, Mode: 0555},
				{Path: "after/service.json", Producer: string(definitionDigest), MaxBytes: 64 << 10, Mode: 0444},
			}
			template, prepErr := sandbox.PrepareTemplate(string(snapshot.ID), files, []string{"/input/after/launcher"}, limits, definition.Image, definition.Platform, slots)
			if prepErr != nil {
				return nil, prepErr
			}
			template, prepErr = template.WithCommandInput(scenarioCase.Stdin)
			if prepErr != nil {
				return nil, prepErr
			}
			p.commandTemplates[side][caseIndex] = template
			preview, _ := template.Preview()
			environmentHashes[side] = append(append(environmentHashes[side], preview...), []byte(definitionDigest)...)
		}
	}
	for side, snapshot := range snapshots {
		argv := []string{"/input/after/launcher"}
		argv = append(argv, definition.BuildArgv...)
		for _, scenarioCase := range definition.Cases {
			argv = append(argv, scenarioCase.Argv...)
		}
		imageDigest := definition.Image[strings.Index(definition.Image, "@")+1:]
		p.environments[side] = evidence.Environment{Environment: hash(environmentHashes[side]), Toolchain: evidence.Digest(imageDigest), Dependencies: snapshot.ID, Argv: argv}
	}
	var experiments [][]json.RawMessage
	for side := range p.commandTemplates {
		casePlans := make([]json.RawMessage, len(p.commandTemplates[side]))
		for caseIndex, template := range p.commandTemplates[side] {
			appPreview, _ := template.Preview()
			rawExperiment, marshalErr := json.Marshal(commandExperimentPreview{App: appPreview, Topology: "one fresh offline command container; build then target via direct argv; no observer, mounts, host access or network"})
			if marshalErr != nil {
				return nil, marshalErr
			}
			casePlans[caseIndex] = rawExperiment
		}
		experiments = append(experiments, casePlans)
	}
	commandConfigs := make([]json.RawMessage, len(p.commandConfigs))
	for index, config := range p.commandConfigs {
		commandConfigs[index] = append(json.RawMessage(nil), config...)
	}
	p.preview, err = json.MarshalIndent(struct {
		Version           int                   `json:"version"`
		Request           evidence.Digest       `json:"request"`
		Snapshots         evidence.SnapshotPair `json:"snapshots"`
		Scenario          evidence.Scenario     `json:"scenario"`
		DefinitionDigest  evidence.Digest       `json:"definition_digest"`
		Definition        CommandDefinition     `json:"definition"`
		DefinitionSource  DefinitionSource      `json:"definition_source"`
		CommandConfigs    []json.RawMessage     `json:"command_configs"`
		Repetitions       int                   `json:"repetitions"`
		Limits            sandbox.Limits        `json:"limits"`
		Preparation       json.RawMessage       `json:"preparation"`
		PreparationBudget string                `json:"preparation_budget"`
		Concurrency       int                   `json:"concurrency"`
		Experiments       [][]json.RawMessage   `json:"experiments"`
	}{1, p.request, p.pair, p.scenario, p.definitionDigest, definition, p.definitionSource, commandConfigs, p.repetitions, p.limits, mustPlanPreview(p.preparation), fmt.Sprintf("one in-approved-request launcher build; %d seconds; stdout ≤8 MiB binary, stderr ≤%d bytes diagnostics; no cache", definition.Limits.PreparationSeconds, definition.Limits.OutputBytes), 1, experiments}, "", "  ")
	if err == nil {
		p.id = string(hash(p.preview))
	}
	return p, err
}

func (p *Plan) commandPlatform() string {
	if p.command != nil {
		return p.command.Platform
	}
	return p.definition.Platform
}

func (p *Plan) runtimeFiles(caseIndex int, binary []byte) map[string][]byte {
	var config []byte
	if p.command != nil {
		config = p.commandConfigs[caseIndex]
	} else {
		config = p.serviceConfigBytes()
	}
	return map[string][]byte{"after/launcher": binary, "after/service.json": config}
}

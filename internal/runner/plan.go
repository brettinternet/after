// Package runner executes only frozen, versioned HTTP-service definitions.
// Project code remains data until exact consent and always runs in offline Docker.
package runner

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

//go:embed runtime/launch.go
var launcher []byte

//go:embed runtime/preparer.go
var preparer []byte

//go:embed runtime/observer.go
var observer []byte

//go:embed runtime/command.go
var commandRunner []byte

// ComparisonRules is immutable supported policy text retained with receipts.
const ComparisonRules = `{"version":1,"responses":"ordered HTTP status and body; JSON bodies structural, otherwise exact text","provider_calls":"ordered observer-received timestamp, endpoint, method, path, idempotency key and body","json":"exact decimal values, object key order ignored, arrays ordered, null distinct from missing; reject duplicate keys and invalid Unicode","masks":[],"normalization":[],"scope":"finite controlled-clock definition-declared requests only"}`

const rules = ComparisonRules

var scope = []string{
	"Finite operator-selected HTTP-service definition and request cases only; no production correctness, universal behavior, causation or performance claim.",
	"Candidate suites are not run; candidate-owned definitions, tests and drivers never select or authorize the frozen oracle.",
	"Services must be self-contained in the separately provisioned digest-pinned image; dependency downloads, external networking and host access are unavailable and fail closed.",
	"Docker daemon, CLI, pinned images, shipped launcher/observer and kernel are trusted; app and observer share only offline loopback; denial of service fails the run.",
}

var (
	ErrUnsupportedProject = errors.New("captured service source is unavailable")
	ErrIncompleteSnapshot = errors.New("incomplete snapshot cannot execute")
	ErrSnapshotBudget     = errors.New("snapshot exceeds the sandbox budget of 255 files and 8 MiB")
	ErrReservedPath       = errors.New("reserved AFTER runtime path in snapshot")
)

type Plan struct {
	request          evidence.Digest
	pair             evidence.SnapshotPair
	scenario         evidence.Scenario
	definition       Definition
	definitionRaw    []byte
	definitionDigest evidence.Digest
	definitionSource DefinitionSource
	command          *CommandDefinition
	commandTemplates [2][]*sandbox.Plan
	commandConfigs   [][]byte
	repetitions      int
	limits           sandbox.Limits
	preparation      *sandbox.Plan
	appTemplates     [2][]*sandbox.Plan
	experiments      [2][]*sandbox.Experiment
	environments     [2]evidence.Environment
	planIDs          [2][][2]string // side, case, app-template/observer
	preview          []byte
	id               string
}

type launcherConfig struct {
	Version                int             `json:"version"`
	BuildArgv              []string        `json:"build_argv,omitempty"`
	StartArgv              []string        `json:"start_argv"`
	Environment            []string        `json:"environment"`
	ReadinessProtocol      string          `json:"readiness_protocol"`
	ReadinessTimeoutSecond int             `json:"readiness_timeout_seconds"`
	Routes                 []launcherRoute `json:"routes"`
	ReservedPorts          []int           `json:"reserved_ports"`
}
type launcherRoute struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

func hash(b []byte) evidence.Digest {
	return evidence.Digest(fmt.Sprintf("sha256:%x", sha256.Sum256(b)))
}

func validRequestID(id evidence.Digest) bool {
	s := string(id)
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil
}
func (p *Plan) Preview() ([]byte, string)  { return append([]byte(nil), p.preview...), p.id }
func (p *Plan) RequestID() evidence.Digest { return p.request }

// ReviewBasis exposes template/recipe identities without executing preparation.
func (p *Plan) ReviewBasis() evidence.ReviewBasis {
	base, candidate := p.environments[0], p.environments[1]
	base.Argv = append([]string(nil), base.Argv...)
	candidate.Argv = append([]string(nil), candidate.Argv...)
	b := p.scenario
	return evidence.ReviewBasis{Snapshots: p.pair, Bindings: &evidence.Bindings{Scenario: b.ID, Input: b.Input, Driver: b.Driver, Observer: b.Observer, Rules: b.Rules}, BaseEnvironment: &base, CandidateEnvironment: &candidate}
}

// Prepare keeps the built-in payment experiment as a declarative HTTP-service
// instance. It neither contacts Docker nor builds anything.
func Prepare(s *store.Store, pair evidence.SnapshotPair, repetitions int, limits sandbox.Limits) (*Plan, error) {
	if repetitions < 1 || repetitions > 5 || limits.Seconds < 1 || limits.Seconds > 300 || limits.OutputBytes < 1 || limits.OutputBytes > 1<<20 {
		return nil, errors.New("invalid built-in payment bounds")
	}
	definition := BuiltinPaymentDefinition(repetitions, limits)
	raw, err := definition.CanonicalBytes()
	if err != nil {
		return nil, err
	}
	return prepare(s, pair, raw, DefinitionSource{Kind: "built-in-payment"}, "")
}

// PrepareDefinition accepts only explicitly selected definition bytes. The
// selected path is metadata for changed-oracle disclosure, never code discovery.
func PrepareDefinition(s *store.Store, pair evidence.SnapshotPair, raw []byte, source DefinitionSource) (*Plan, error) {
	if source.Kind != "operator-selected-file" {
		return nil, errors.New("a custom definition must come from an explicitly operator-selected file")
	}
	return prepare(s, pair, raw, source, "")
}

// PrepareCommandDefinition accepts only an explicitly selected command v1 file.
func PrepareCommandDefinition(s *store.Store, pair evidence.SnapshotPair, raw []byte, source DefinitionSource) (*Plan, error) {
	if source.Kind != "operator-selected-file" {
		return nil, errors.New("a command definition must come from an explicitly operator-selected file")
	}
	return prepareCommand(s, pair, raw, source, "")
}

// PrepareSelectedDefinition dispatches only between the two closed v1 schemas.
func PrepareSelectedDefinition(s *store.Store, pair evidence.SnapshotPair, raw []byte, source DefinitionSource) (*Plan, error) {
	kind, err := ParseSelectedDefinition(raw)
	if err != nil {
		return nil, err
	}
	if source.Kind != "operator-selected-file" {
		return nil, errors.New("a custom definition must come from an explicitly operator-selected file")
	}
	if kind == "command" {
		return prepareCommand(s, pair, raw, source, "")
	}
	return prepare(s, pair, raw, source, "")
}

func prepare(s *store.Store, pair evidence.SnapshotPair, raw []byte, source DefinitionSource, request evidence.Digest) (*Plan, error) {
	definition, err := ParseDefinition(raw)
	if err != nil {
		return nil, err
	}
	if pair.Base == "" || pair.Candidate == "" {
		return nil, errors.New("base and candidate snapshots required")
	}
	if request != "" && !validRequestID(request) {
		return nil, errors.New("invalid saved request identity")
	}
	if err := validateDefinitionSource(source); err != nil {
		return nil, err
	}
	snapshots := [2]evidence.Snapshot{}
	for i, id := range []evidence.Digest{pair.Base, pair.Candidate} {
		snap, err := store.Get[evidence.Snapshot](s, id)
		if err != nil {
			return nil, err
		}
		if err := checkSnapshot(snap); err != nil {
			return nil, err
		}
		snapshots[i] = snap
	}
	if source.RepositoryPath != "" {
		source.ChangedOracle = definitionChanged(s, snapshots[0], snapshots[1], source.RepositoryPath)
	}
	definitionArtifact, err := s.PutArtifact(raw, "http-service-definition", MaxDefinitionBytes)
	if err != nil || definitionArtifact.Completeness != evidence.Complete {
		return nil, errors.Join(err, errors.New("complete frozen definition could not be retained"))
	}
	if request == "" {
		nonce := make([]byte, 32)
		if _, err = rand.Read(nonce); err != nil {
			return nil, err
		}
		request = hash(nonce)
	}
	definitionDigest := hash(raw)
	inputConfig, err := json.Marshal(definition)
	if err != nil {
		return nil, err
	}
	observerDigest := hash(observer)
	driverDigest := hash(append(append(append([]byte(nil), preparer...), launcher...), observer...))
	author := "AFTER operator-selected http-service v1"
	if source.Kind == "built-in-payment" {
		author = "AFTER built-in synthetic payment"
	}
	scenario, err := store.Put(s, evidence.Scenario{SchemaVersion: 1, Input: definitionArtifact.Content, Driver: driverDigest, Observer: observerDigest, Rules: hash([]byte(rules)), Boundary: strings.Join(definition.Channels, " and ") + " from the frozen HTTP service and observer-recorded fake upstreams", Author: author, Limits: definitionLimits(definition)})
	if err != nil {
		return nil, err
	}
	p := &Plan{request: request, pair: pair, scenario: scenario, definition: definition, definitionRaw: append([]byte(nil), raw...), definitionDigest: definitionDigest, definitionSource: source, repetitions: definition.Repetitions, limits: sandbox.Limits{Seconds: definition.Limits.Seconds, OutputBytes: definition.Limits.OutputBytes}}
	for i := range p.experiments {
		p.appTemplates[i] = make([]*sandbox.Plan, len(definition.Cases))
		p.experiments[i] = make([]*sandbox.Experiment, len(definition.Cases))
		p.planIDs[i] = make([][2]string, len(definition.Cases))
	}
	preparationLimits := sandbox.Limits{Seconds: definition.Limits.PreparationSeconds, OutputBytes: definition.Limits.OutputBytes}
	p.preparation, err = sandbox.PrepareImage(string(definitionDigest), map[string][]byte{"preparer.go": preparer, "launch.go": launcher}, []string{"/usr/local/go/bin/go", "run", "/input/preparer.go"}, preparationLimits, sandbox.Image, definition.Platform)
	if err != nil {
		return nil, err
	}
	appConfig := launcherConfig{Version: 1, BuildArgv: append([]string(nil), definition.BuildArgv...), StartArgv: append([]string(nil), definition.StartArgv...), Environment: append([]string(nil), definition.Environment...), ReadinessProtocol: readinessABI, ReadinessTimeoutSecond: definition.Readiness.TimeoutSeconds, ReservedPorts: reservedPorts(definition)}
	for _, route := range definition.routeKeys() {
		method, requestPath, _ := strings.Cut(route, " ")
		appConfig.Routes = append(appConfig.Routes, launcherRoute{Method: method, Path: requestPath})
	}
	serviceConfig, err := json.Marshal(appConfig)
	if err != nil || len(serviceConfig) > 64<<10 {
		return nil, errors.Join(err, errors.New("trusted service configuration exceeds budget"))
	}
	compilerPreview, compilerID := p.preparation.Preview()
	_ = compilerPreview
	for side, snapshot := range snapshots {
		files := make(map[string][]byte, len(snapshot.Files))
		total := 0
		for _, file := range snapshot.Files {
			data, readErr := s.ReadBlob(file.Content)
			if readErr != nil {
				return nil, readErr
			}
			total += len(data)
			if total > 8<<20 {
				return nil, ErrSnapshotBudget
			}
			files[file.Path] = data
		}
		if len(files) == 0 {
			return nil, ErrUnsupportedProject
		}
		slots := []sandbox.GeneratedFile{
			{Path: "after/launcher", Producer: compilerID, MaxBytes: launcherMaxBytes, Mode: 0555},
			{Path: "after/service.json", Producer: string(definitionDigest), MaxBytes: 64 << 10, Mode: 0444},
		}
		appTemplate, prepErr := sandbox.PrepareTemplate(string(snapshot.ID), files, []string{"/input/after/launcher"}, p.limits, definition.Image, definition.Platform, slots)
		if prepErr != nil {
			return nil, prepErr
		}
		appPreview, appID := appTemplate.Preview()
		toolchainDigest := evidence.Digest(strings.TrimPrefix(definition.Image[strings.Index(definition.Image, "@")+1:], ""))
		serviceArgv := append(append([]string(nil), definition.BuildArgv...), definition.StartArgv...)
		p.environments[side] = evidence.Environment{Environment: hash(append(append(append([]byte(nil), appPreview...), []byte(compilerID)...), []byte(definitionDigest)...)), Toolchain: toolchainDigest, Dependencies: snapshot.ID, Argv: serviceArgv}
		for caseIndex, scenarioCase := range definition.Cases {
			p.appTemplates[side][caseIndex] = appTemplate
			observerFiles := map[string][]byte{"observer.go": observer, "definition.json": inputConfig}
			obs, prepErr := sandbox.PrepareImage(string(scenario.ID), observerFiles, []string{"/usr/local/go/bin/go", "run", "/input/observer.go", "/input/definition.json", scenarioCase.ID}, p.limits, sandbox.Image, definition.Platform)
			if prepErr != nil {
				return nil, prepErr
			}
			_, observerID := obs.Preview()
			p.planIDs[side][caseIndex] = [2]string{appID, observerID}
			p.experiments[side][caseIndex], err = sandbox.PrepareExperiment(appTemplate, obs)
			if err != nil {
				return nil, err
			}
		}
	}
	_ = inputConfig
	var experiments [][]json.RawMessage
	for side := range p.experiments {
		experiments = append(experiments, make([]json.RawMessage, len(p.experiments[side])))
		for caseIndex, experiment := range p.experiments[side] {
			raw, _ := experiment.Preview()
			experiments[side][caseIndex] = raw
		}
	}
	p.preview, err = json.MarshalIndent(struct {
		Version           int                   `json:"version"`
		Request           evidence.Digest       `json:"request"`
		Snapshots         evidence.SnapshotPair `json:"snapshots"`
		Scenario          evidence.Scenario     `json:"scenario"`
		DefinitionDigest  evidence.Digest       `json:"definition_digest"`
		Definition        Definition            `json:"definition"`
		DefinitionSource  DefinitionSource      `json:"definition_source"`
		ServiceConfig     json.RawMessage       `json:"service_config"`
		Repetitions       int                   `json:"repetitions"`
		Limits            sandbox.Limits        `json:"limits"`
		Preparation       json.RawMessage       `json:"preparation"`
		PreparationBudget string                `json:"preparation_budget"`
		Concurrency       int                   `json:"concurrency"`
		Experiments       [][]json.RawMessage   `json:"experiments"`
	}{1, p.request, p.pair, p.scenario, p.definitionDigest, p.definition, p.definitionSource, serviceConfig, p.repetitions, p.limits, mustPlanPreview(p.preparation), fmt.Sprintf("one in-approved-request launcher build; %d seconds; stdout ≤8 MiB binary, stderr ≤%d bytes diagnostics; no cache", definition.Limits.PreparationSeconds, definition.Limits.OutputBytes), 1, experiments}, "", "  ")
	p.id = string(hash(p.preview))
	return p, err
}

func mustPlanPreview(p *sandbox.Plan) json.RawMessage {
	if p == nil {
		return nil
	}
	raw, _ := p.Preview()
	return raw
}

func definitionLimits(d Definition) []string {
	return append(append([]string(nil), scope...), fmt.Sprintf("Definition %s@sha256 is frozen independently of the pair; %d case(s), %d repetition(s), sample %ds/%d-byte output, launcher preparation %ds, readiness %ds.", d.Name, len(d.Cases), d.Repetitions, d.Limits.Seconds, d.Limits.OutputBytes, d.Limits.PreparationSeconds, d.Readiness.TimeoutSeconds))
}

func reservedPorts(d Definition) []int {
	ports := []int{18080}
	for _, endpoint := range d.Upstreams {
		ports = append(ports, endpoint.Port)
	}
	return ports
}

func validateDefinitionSource(source DefinitionSource) error {
	if source.Kind != "built-in-payment" && source.Kind != "operator-selected-file" {
		return errors.New("definition must be built in or explicitly selected by the operator")
	}
	if source.Kind == "built-in-payment" && (source.RepositoryPath != "" || source.ChangedOracle) {
		return errors.New("invalid built-in definition source")
	}
	if source.RepositoryPath != "" && (path.Clean(source.RepositoryPath) != source.RepositoryPath || strings.HasPrefix(source.RepositoryPath, "/") || source.RepositoryPath == ".." || strings.HasPrefix(source.RepositoryPath, "../") || strings.ContainsAny(source.RepositoryPath, "\\\x00\r\n")) {
		return errors.New("invalid repository-relative definition path")
	}
	return nil
}

func definitionChanged(s *store.Store, base, candidate evidence.Snapshot, repositoryPath string) bool {
	contents := [2][]byte{}
	found := [2]bool{}
	for side, snapshot := range []evidence.Snapshot{base, candidate} {
		for _, file := range snapshot.Files {
			if file.Path == repositoryPath {
				found[side] = true
				contents[side], _ = s.ReadBlob(file.Content)
				break
			}
		}
	}
	return found[0] != found[1] || (found[0] && !bytes.Equal(contents[0], contents[1]))
}

// PrepareFromPreview reconstructs the immutable consent bytes from frozen
// definition and captures only. It does not load the original path, query Docker
// or compile the generated launcher.
func PrepareFromPreview(s *store.Store, preview []byte) (*Plan, error) {
	var saved struct {
		Version          int                   `json:"version"`
		Request          evidence.Digest       `json:"request"`
		Snapshots        evidence.SnapshotPair `json:"snapshots"`
		Scenario         evidence.Scenario     `json:"scenario"`
		DefinitionDigest evidence.Digest       `json:"definition_digest"`
		DefinitionSource DefinitionSource      `json:"definition_source"`
		Repetitions      int                   `json:"repetitions"`
		Limits           sandbox.Limits        `json:"limits"`
	}
	if len(preview) == 0 || len(preview) > 1<<20 {
		return nil, errors.New("saved execution plan exceeds bounds")
	}
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(preview))
	if err := decoder.Decode(&fields); err != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("invalid saved execution plan")
	}
	var previewDefinition struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(fields["definition"], &previewDefinition); err != nil {
		return nil, errors.New("saved execution plan definition is invalid")
	}
	allowed := map[string]bool{"version": true, "request": true, "snapshots": true, "scenario": true, "definition_digest": true, "definition": true, "definition_source": true, "repetitions": true, "limits": true, "preparation": true, "preparation_budget": true, "concurrency": true, "experiments": true}
	switch previewDefinition.Kind {
	case "http-service":
		allowed["service_config"] = true
	case "command":
		allowed["command_configs"] = true
	default:
		return nil, errors.New("saved execution plan kind is unsupported")
	}
	if len(fields) != len(allowed) {
		return nil, errors.New("saved execution plan has missing or unknown fields")
	}
	for key := range fields {
		if !allowed[key] {
			return nil, errors.New("saved execution plan has missing or unknown fields")
		}
	}
	for key, target := range map[string]any{"version": &saved.Version, "request": &saved.Request, "snapshots": &saved.Snapshots, "scenario": &saved.Scenario, "definition_digest": &saved.DefinitionDigest, "definition_source": &saved.DefinitionSource, "repetitions": &saved.Repetitions, "limits": &saved.Limits} {
		if err := json.Unmarshal(fields[key], target); err != nil {
			return nil, errors.New("invalid saved execution plan field")
		}
	}
	if saved.Version != 1 || !validRequestID(saved.Request) || saved.Snapshots.Base == "" || saved.Snapshots.Candidate == "" || saved.Repetitions < 1 || saved.Repetitions > 5 || saved.Limits.Seconds < 1 || saved.Limits.Seconds > 300 || saved.Limits.OutputBytes < 1 || saved.Limits.OutputBytes > 1<<20 || saved.Scenario.Input == "" || saved.Scenario.ID == "" {
		return nil, errors.New("invalid saved execution plan bindings")
	}
	raw, err := s.ReadBlob(saved.Scenario.Input)
	if err != nil || hash(raw) != saved.DefinitionDigest {
		return nil, errors.New("saved definition is missing or changed")
	}
	kind, err := ParseSelectedDefinition(raw)
	if err != nil {
		return nil, errors.New("saved definition bindings are invalid")
	}
	var p *Plan
	if kind == "command" {
		definition, parseErr := ParseCommandDefinition(raw)
		if parseErr != nil || definition.Repetitions != saved.Repetitions || definition.Limits.Seconds != saved.Limits.Seconds || definition.Limits.OutputBytes != saved.Limits.OutputBytes {
			return nil, errors.New("saved command definition bindings are invalid")
		}
		p, err = prepareCommand(s, saved.Snapshots, raw, saved.DefinitionSource, saved.Request)
	} else {
		definition, parseErr := ParseDefinition(raw)
		if parseErr != nil || definition.Repetitions != saved.Repetitions || definition.Limits.Seconds != saved.Limits.Seconds || definition.Limits.OutputBytes != saved.Limits.OutputBytes {
			return nil, errors.New("saved http-service definition bindings are invalid")
		}
		p, err = prepare(s, saved.Snapshots, raw, saved.DefinitionSource, saved.Request)
	}
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(p.preview, preview) || !reflect.DeepEqual(p.scenario, saved.Scenario) {
		return nil, errors.New("saved execution plan no longer matches frozen inputs")
	}
	return p, nil
}

func checkSnapshot(snap evidence.Snapshot) error {
	paths := map[string]bool{}
	reserved := false
	for _, file := range snap.Files {
		paths[file.Path] = true
		reserved = reserved || file.Path == "after-launch.go" || pathConflictsRuntime(file.Path)
	}
	switch {
	case snap.Completeness != evidence.Complete:
		return ErrIncompleteSnapshot
	case len(snap.Files) == 0:
		return ErrUnsupportedProject
	case len(snap.Files) > 255:
		return ErrSnapshotBudget
	case reserved:
		return ErrReservedPath
	}
	return nil
}

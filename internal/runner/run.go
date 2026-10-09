package runner

import (
	"bytes"
	"context"
	"debug/elf"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

// One process-wide gate bounds concurrent sandbox experiments, including calls
// from different UI requests. Waiting is cancellable; no goroutine queue is made.
var executionGate = make(chan struct{}, 1)

type Executor struct {
	Docker sandbox.Docker
	// Private test seams cannot be selected by production callers.
	prepareLauncher func(context.Context, *sandbox.Plan) (sandbox.Result, []byte, error)
	observe         func(context.Context, *sandbox.Experiment, string) (sandbox.ExperimentResult, error)
	executeCommand  func(context.Context, *sandbox.Plan, string) (sandbox.Result, error)
}

type Response struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}
type Call struct {
	At          int64  `json:"at"`
	Endpoint    string `json:"endpoint"`
	Destination string `json:"destination"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Key         string `json:"key"`
	Body        string `json:"body"`
}
type Observation struct {
	Version   int        `json:"version"`
	CaseID    string     `json:"case_id"`
	Seconds   int64      `json:"seconds,omitempty"` // legacy display projection; CaseID is authoritative
	Responses []Response `json:"responses"`
	Calls     []Call     `json:"provider_calls"`
}
type Sample struct {
	RequestID   evidence.Digest          `json:"request_id"`
	Snapshots   evidence.SnapshotPair    `json:"snapshots"`
	Side        string                   `json:"side"`
	CaseID      string                   `json:"case_id"`
	CaseSeconds int64                    `json:"case_seconds,omitempty"`
	Repetition  int                      `json:"repetition"`
	StartedAt   time.Time                `json:"started_at"`
	FinishedAt  time.Time                `json:"finished_at"`
	Status      string                   `json:"status"`
	Execution   sandbox.ExperimentResult `json:"execution"`
	Artifacts   []evidence.Artifact      `json:"artifacts"`
}
type Result struct {
	Receipt evidence.Receipt
	Samples []Sample
}

type preparationEvidence struct {
	Version          int             `json:"version"`
	Status           string          `json:"status"`
	Plan             string          `json:"plan"`
	DefinitionDigest evidence.Digest `json:"definition_digest"`
	Platform         string          `json:"platform"`
	Image            string          `json:"image"`
	ExitCode         int             `json:"exit_code"`
	Cleaned          bool            `json:"cleaned"`
	Truncated        bool            `json:"truncated"`
	Launcher         evidence.Digest `json:"launcher_digest,omitempty"`
	Bytes            int             `json:"launcher_bytes,omitempty"`
}

func decode(data string, scenarioCase Case, definition Definition) (Observation, error) {
	var observation Observation
	decoder := json.NewDecoder(bytes.NewBufferString(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&observation); err != nil || decoder.Decode(new(any)) != io.EOF || observation.Version != 1 || observation.CaseID != scenarioCase.ID || (observation.Seconds != 0 && observation.Seconds != CaseDuration(scenarioCase)) || len(observation.Responses) != len(scenarioCase.Requests) || observation.Calls == nil || len(observation.Calls) > maxProviderCalls {
		return observation, errors.New("invalid observer protocol or missing channel")
	}
	for _, response := range observation.Responses {
		if response.Status < 100 || response.Status > 599 || len(response.Body) > 4096 {
			return observation, errors.New("invalid response channel")
		}
	}
	endpoints := map[string]Upstream{}
	for _, endpoint := range definition.Upstreams {
		endpoints[endpoint.Name] = endpoint
	}
	for _, call := range observation.Calls {
		endpoint, ok := endpoints[call.Endpoint]
		if !ok || call.Destination != fmt.Sprintf("127.0.0.1:%d", endpoint.Port) || call.At < definition.Epoch || call.At > definition.Epoch+365*24*60*60 || call.Method == "" || len(call.Method) > 16 || call.Path == "" || len(call.Path) > 256 || call.Path[0] != '/' || len(call.Key) > 128 || len(call.Body) > 4096 {
			return observation, errors.New("invalid fake-upstream observation")
		}
	}
	return observation, nil
}

func status(err error, r sandbox.ExperimentResult) string {
	switch {
	case errors.Is(err, sandbox.ErrConsent):
		return "permission_denied"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case r.App.Truncated || r.Observer.Truncated || errors.Is(err, sandbox.ErrOutput):
		return "output_truncated"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case err != nil:
		return "execution_failed"
	case !r.App.Cleaned || !r.Observer.Cleaned:
		return "cleanup_failed"
	case r.Observer.ExitCode != 0:
		return "observer_failed"
	default:
		return "completed"
	}
}

// Run persists an incomplete receipt even on denied consent/cancellation. No
// selection pointer or mutable checkout is consulted after Prepare.
func (e Executor) Run(ctx context.Context, s *store.Store, p *Plan, approved string) (result Result, err error) {
	if p == nil {
		return result, sandbox.ErrConsent
	}
	if p.command != nil {
		return e.runCommand(ctx, s, p, approved)
	}
	started := time.Now().UTC()
	allowed := approved == p.id
	acquired := false
	if allowed {
		select {
		case executionGate <- struct{}{}:
			acquired = true
			defer func() { <-executionGate }()
		case <-ctx.Done():
		}
	}
	planArtifact, err := s.PutArtifact(p.preview, "execution-plan", 1<<20)
	if err != nil {
		return result, err
	}
	policyArtifact, err := s.PutArtifact([]byte(ComparisonRules), "comparison-rules", 4096)
	if err != nil {
		return result, err
	}
	artifacts := []evidence.Artifact{planArtifact, policyArtifact}
	allComplete := allowed && acquired && planArtifact.Completeness == evidence.Complete && policyArtifact.Completeness == evidence.Complete
	unstable := false
	previous := map[string]evidence.Digest{}
	observe := e.observe
	if observe == nil {
		observe = e.Docker.Observe
	}
	var runErrors []error
	cleanupBlocked := false
	preparedBytes := []byte(nil)
	preparationGood := false
	var materialized [2][]*sandbox.Experiment
	if allComplete {
		preparedBytes, artifacts, err = e.compileAndRecord(ctx, s, p, artifacts)
		if err != nil {
			runErrors = append(runErrors, fmt.Errorf("launcher preparation failed: %s", safeRunError(err)))
			allComplete = false
		} else {
			preparationGood = true
			allComplete = true
			generated := map[string][]byte{"after/launcher": preparedBytes, "after/service.json": p.serviceConfigBytes()}
			for side := range p.experiments {
				materialized[side] = make([]*sandbox.Experiment, len(p.experiments[side]))
				for caseIndex, template := range p.experiments[side] {
					materialized[side][caseIndex], err = template.MaterializeApp(generated)
					if err != nil {
						preparationGood, allComplete = false, false
						runErrors = append(runErrors, errors.New("approved app template could not be materialized"))
						break
					}
				}
			}
		}
	}
	for repetition := 0; repetition < p.repetitions; repetition++ {
		for side, label := range []string{"base", "candidate"} {
			for caseIndex, scenarioCase := range p.definition.Cases {
				caseSeconds := CaseDuration(scenarioCase)
				sample := Sample{RequestID: p.request, Snapshots: p.pair, Side: label, CaseID: scenarioCase.ID, CaseSeconds: caseSeconds, Repetition: repetition, StartedAt: time.Now().UTC()}
				var executionErr error
				switch {
				case !allowed:
					executionErr = sandbox.ErrConsent
				case ctx.Err() != nil:
					executionErr = ctx.Err()
				case !preparationGood:
					executionErr = errors.New("launcher preparation unavailable; sample not started")
				case cleanupBlocked:
					executionErr = errors.New("prior cleanup requires reconciliation")
				default:
					_, consent := materialized[side][caseIndex].Preview()
					sample.Execution, executionErr = observe(ctx, materialized[side][caseIndex], consent)
				}
				if (sample.Execution.App.Container != "" && !sample.Execution.App.Cleaned) || (sample.Execution.Observer.Container != "" && !sample.Execution.Observer.Cleaned) {
					cleanupBlocked = true
				}
				sample.FinishedAt = time.Now().UTC()
				sample.Status = status(executionErr, sample.Execution)
				if sample.Status == "execution_failed" && !preparationGood {
					sample.Status = "preparation_failed"
				}
				if sample.Status == "completed" {
					observation, decodeErr := decode(sample.Execution.Observer.Output, scenarioCase, p.definition)
					if decodeErr != nil {
						sample.Status = "incompatible_or_missing_channel"
						executionErr = decodeErr
					} else {
						raw, _ := json.Marshal(observation)
						a, storeErr := s.PutArtifact(raw, sampleChannel(label, scenarioCase.ID, repetition, "observation"), int64(p.limits.OutputBytes))
						if storeErr != nil {
							return result, storeErr
						}
						sample.Artifacts = append(sample.Artifacts, a)
						if a.Completeness != evidence.Complete {
							sample.Status = "redacted_or_truncated"
						} else {
							key := fmt.Sprintf("%s/%s", label, scenarioCase.ID)
							if prior, ok := previous[key]; ok && prior != a.Content {
								unstable = true
							}
							previous[key] = a.Content
						}
					}
				}
				for _, diagnostic := range []struct{ name, data string }{{"candidate-diagnostics", sample.Execution.App.Output}, {"observer-diagnostics", sample.Execution.Observer.Output}} {
					if diagnostic.name == "observer-diagnostics" && sample.Status == "completed" {
						continue
					}
					a, storeErr := s.PutArtifact([]byte(diagnostic.data), sampleChannel(label, scenarioCase.ID, repetition, diagnostic.name), int64(p.limits.OutputBytes))
					if storeErr != nil {
						return result, storeErr
					}
					sample.Artifacts = append(sample.Artifacts, a)
				}
				sample.Execution.App.Output = ""
				sample.Execution.App.Stdout, sample.Execution.App.Stderr = "", ""
				sample.Execution.Observer.Output = ""
				sample.Execution.Observer.Stdout, sample.Execution.Observer.Stderr = "", ""
				if sample.Status != "completed" {
					allComplete = false
					runErrors = append(runErrors, fmt.Errorf("%s/%s/%d: %s", label, scenarioCase.ID, repetition, sample.Status))
				}
				for _, artifact := range sample.Artifacts {
					if artifact.Completeness != evidence.Complete {
						allComplete = false
					}
				}
				artifacts = append(artifacts, sample.Artifacts...)
				metadata, _ := json.Marshal(sample)
				metadataArtifact, storeErr := s.PutArtifact(metadata, sampleChannel(label, scenarioCase.ID, repetition, "sample"), 1<<20)
				if storeErr != nil {
					return result, storeErr
				}
				artifacts = append(artifacts, metadataArtifact)
				if metadataArtifact.Completeness != evidence.Complete {
					allComplete = false
				}
				result.Samples = append(result.Samples, sample)
			}
		}
	}
	b := p.scenario
	receipt := evidence.Receipt{RequestID: p.request, SchemaVersion: 1, State: evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.NoEvidence, Applicability: evidence.Unknown, Execution: evidence.Failed, Comparison: evidence.Incomparable, Report: evidence.NoReport}, Snapshots: p.pair, Bindings: &evidence.Bindings{Scenario: b.ID, Input: b.Input, Driver: b.Driver, Observer: b.Observer, Rules: b.Rules}, BaseEnvironment: &p.environments[0], CandidateEnvironment: &p.environments[1], Authorization: evidence.Digest(p.id), StartedAt: started, FinishedAt: time.Now().UTC(), Completeness: evidence.Incomplete, Artifacts: artifacts, Limits: definitionLimits(p.definition)}
	if !allowed {
		receipt.State.Execution = evidence.NotRun
	}
	if ctx.Err() != nil {
		receipt.State.Execution = evidence.Cancelled
	}
	if allComplete {
		receipt.State.Kind = evidence.Observed
		receipt.State.Execution = evidence.Completed
		receipt.State.Applicability = evidence.Current
		receipt.State.Comparison = evidence.NotCompared
		receipt.Completeness = evidence.Complete
		if unstable {
			receipt.State.Comparison = evidence.Unstable
		}
	}
	result.Receipt, err = store.Put(s, receipt)
	if err != nil {
		return result, err
	}
	return result, errors.Join(runErrors...)
}

func commandStatus(err error, execution sandbox.Result) string {
	switch {
	case errors.Is(err, sandbox.ErrConsent):
		return "permission_denied"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case execution.Truncated || errors.Is(err, sandbox.ErrOutput):
		return "output_truncated"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, sandbox.ErrCommandStatus):
		return "signal_or_reserved_exit"
	case err != nil:
		return "execution_failed"
	case !execution.Cleaned:
		return "cleanup_failed"
	case execution.ExitCode < 0 || execution.ExitCode >= 125:
		return "signal_or_reserved_exit"
	default:
		return "completed"
	}
}

func (e Executor) runCommand(ctx context.Context, s *store.Store, p *Plan, approved string) (result Result, err error) {
	started := time.Now().UTC()
	allowed := approved == p.id
	acquired := false
	if allowed {
		select {
		case executionGate <- struct{}{}:
			acquired = true
			defer func() { <-executionGate }()
		case <-ctx.Done():
		}
	}
	planArtifact, err := s.PutArtifact(p.preview, "execution-plan", 1<<20)
	if err != nil {
		return result, err
	}
	policyArtifact, err := s.PutArtifact([]byte(CommandComparisonRules), "comparison-rules", 4096)
	if err != nil {
		return result, err
	}
	artifacts := []evidence.Artifact{planArtifact, policyArtifact}
	allComplete := allowed && acquired && planArtifact.Completeness == evidence.Complete && policyArtifact.Completeness == evidence.Complete
	preparationGood := false
	cleanupBlocked := false
	var runErrors []error
	materialized := [2][]*sandbox.Plan{}
	if allComplete {
		binary, nextArtifacts, prepareErr := e.compileAndRecord(ctx, s, p, artifacts)
		artifacts = nextArtifacts
		if prepareErr != nil {
			runErrors = append(runErrors, fmt.Errorf("command launcher preparation failed: %s", safeRunError(prepareErr)))
			allComplete = false
		} else {
			preparationGood, allComplete = true, true
			for side := range p.commandTemplates {
				materialized[side] = make([]*sandbox.Plan, len(p.commandTemplates[side]))
				for caseIndex, template := range p.commandTemplates[side] {
					materialized[side][caseIndex], err = template.Materialize(p.runtimeFiles(caseIndex, binary))
					if err != nil {
						preparationGood, allComplete = false, false
						runErrors = append(runErrors, errors.New("approved command template could not be materialized"))
						break
					}
				}
			}
		}
	}
	previous := map[string]evidence.Digest{}
	unstable := false
	for repetition := 0; repetition < p.repetitions; repetition++ {
		for side, label := range []string{"base", "candidate"} {
			for caseIndex, scenarioCase := range p.command.Cases {
				sample := Sample{RequestID: p.request, Snapshots: p.pair, Side: label, CaseID: scenarioCase.ID, Repetition: repetition, StartedAt: time.Now().UTC()}
				var executionErr error
				switch {
				case !allowed:
					executionErr = sandbox.ErrConsent
				case ctx.Err() != nil:
					executionErr = ctx.Err()
				case !preparationGood:
					executionErr = errors.New("command launcher preparation unavailable; sample not started")
				case cleanupBlocked:
					executionErr = errors.New("prior cleanup requires reconciliation")
				default:
					_, consent := materialized[side][caseIndex].Preview()
					if e.executeCommand != nil {
						sample.Execution.App, executionErr = e.executeCommand(ctx, materialized[side][caseIndex], consent)
					} else {
						sample.Execution.App, executionErr = e.Docker.ExecuteCommand(ctx, materialized[side][caseIndex], consent)
					}
				}
				if sample.Execution.App.Container != "" && !sample.Execution.App.Cleaned {
					cleanupBlocked = true
				}
				sample.FinishedAt = time.Now().UTC()
				sample.Status = commandStatus(executionErr, sample.Execution.App)
				if sample.Status == "execution_failed" && !preparationGood {
					sample.Status = "preparation_failed"
				}
				prefix := sampleChannel(label, scenarioCase.ID, repetition, "")
				for _, channel := range []struct {
					name string
					data string
				}{{"stdout", sample.Execution.App.Stdout}, {"stderr", sample.Execution.App.Stderr}} {
					a, storeErr := s.PutArtifact([]byte(channel.data), prefix+channel.name, int64(p.limits.OutputBytes))
					if storeErr != nil {
						return result, storeErr
					}
					sample.Artifacts = append(sample.Artifacts, a)
					if a.Completeness != evidence.Complete {
						allComplete = false
					}
				}
				if sample.Status == "completed" {
					key := fmt.Sprintf("%s/%s", label, scenarioCase.ID)
					var statusBytes [4]byte
					binary.BigEndian.PutUint32(statusBytes[:], uint32(sample.Execution.App.ExitCode))
					var stdoutLength, stderrLength [4]byte
					binary.BigEndian.PutUint32(stdoutLength[:], uint32(len(sample.Execution.App.Stdout)))
					binary.BigEndian.PutUint32(stderrLength[:], uint32(len(sample.Execution.App.Stderr)))
					signature := append([]byte(nil), statusBytes[:]...)
					signature = append(signature, stdoutLength[:]...)
					signature = append(signature, sample.Execution.App.Stdout...)
					signature = append(signature, stderrLength[:]...)
					signature = append(signature, sample.Execution.App.Stderr...)
					current := hash(signature)
					if prior, exists := previous[key]; exists && prior != current {
						unstable = true
					}
					previous[key] = current
				}
				sample.Execution.App.Output = ""
				sample.Execution.App.Stdout, sample.Execution.App.Stderr = "", ""
				if sample.Status != "completed" {
					allComplete = false
					runErrors = append(runErrors, fmt.Errorf("%s/%s/%d: %s", label, scenarioCase.ID, repetition, sample.Status))
				}
				artifacts = append(artifacts, sample.Artifacts...)
				metadata, marshalErr := json.Marshal(sample)
				if marshalErr != nil {
					return result, marshalErr
				}
				metadataArtifact, storeErr := s.PutArtifact(metadata, sampleChannel(label, scenarioCase.ID, repetition, "sample"), 1<<20)
				if storeErr != nil {
					return result, storeErr
				}
				artifacts = append(artifacts, metadataArtifact)
				if metadataArtifact.Completeness != evidence.Complete {
					allComplete = false
				}
				result.Samples = append(result.Samples, sample)
			}
		}
	}
	b := p.scenario
	receipt := evidence.Receipt{RequestID: p.request, SchemaVersion: 1, State: evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.NoEvidence, Applicability: evidence.Unknown, Execution: evidence.Failed, Comparison: evidence.Incomparable, Report: evidence.NoReport}, Snapshots: p.pair, Bindings: &evidence.Bindings{Scenario: b.ID, Input: b.Input, Driver: b.Driver, Observer: b.Observer, Rules: b.Rules}, BaseEnvironment: &p.environments[0], CandidateEnvironment: &p.environments[1], Authorization: evidence.Digest(p.id), StartedAt: started, FinishedAt: time.Now().UTC(), Completeness: evidence.Incomplete, Artifacts: artifacts, Limits: commandLimits(*p.command)}
	if !allowed {
		receipt.State.Execution = evidence.NotRun
	}
	if ctx.Err() != nil {
		receipt.State.Execution = evidence.Cancelled
	}
	if allComplete {
		receipt.State.Kind = evidence.Observed
		receipt.State.Execution = evidence.Completed
		receipt.State.Applicability = evidence.Current
		receipt.State.Comparison = evidence.NotCompared
		receipt.Completeness = evidence.Complete
		if unstable {
			receipt.State.Comparison = evidence.Unstable
		}
	}
	result.Receipt, err = store.Put(s, receipt)
	if err != nil {
		return result, err
	}
	return result, errors.Join(runErrors...)
}

func (p *Plan) serviceConfigBytes() []byte {
	config := launcherConfig{Version: 1, BuildArgv: append([]string(nil), p.definition.BuildArgv...), StartArgv: append([]string(nil), p.definition.StartArgv...), Environment: append([]string(nil), p.definition.Environment...), ReadinessProtocol: readinessABI, ReadinessTimeoutSecond: p.definition.Readiness.TimeoutSeconds, ReservedPorts: reservedPorts(p.definition)}
	for _, route := range p.definition.routeKeys() {
		method, requestPath, _ := strings.Cut(route, " ")
		config.Routes = append(config.Routes, launcherRoute{Method: method, Path: requestPath})
	}
	raw, _ := json.Marshal(config)
	return raw
}

func sampleChannel(side, caseID string, repetition int, channel string) string {
	return fmt.Sprintf("%s/%s/%d/%s", side, caseID, repetition, channel)
}

func CaseDuration(scenarioCase Case) int64 {
	var value int64
	for _, request := range scenarioCase.Requests {
		if request.AfterSeconds > value {
			value = request.AfterSeconds
		}
	}
	return value
}

func (e Executor) compileAndRecord(ctx context.Context, s *store.Store, p *Plan, artifacts []evidence.Artifact) ([]byte, []evidence.Artifact, error) {
	prepPreview, prepID := p.preparation.Preview()
	_ = prepPreview
	var result sandbox.Result
	var binary []byte
	var runErr error
	if e.prepareLauncher != nil {
		result, binary, runErr = e.prepareLauncher(ctx, p.preparation)
	} else {
		result, binary, runErr = e.Docker.ExecuteBinary(ctx, p.preparation, prepID)
	}
	stderr := []byte(result.Stderr)
	diagnostics, err := s.PutArtifact(stderr, "preparation-diagnostics", int64(p.limits.OutputBytes))
	if err != nil {
		return nil, artifacts, err
	}
	artifacts = append(artifacts, diagnostics)
	if len(binary) > 0 && (runErr != nil || result.Truncated || result.ExitCode != 0 || !result.Cleaned) {
		partial, storeErr := s.PutArtifact(binary, "preparation-output", launcherMaxBytes)
		if storeErr != nil {
			return nil, artifacts, storeErr
		}
		artifacts = append(artifacts, partial)
	}
	var launcherArtifact evidence.Artifact
	if runErr == nil && result.Plan == prepID && result.ExitCode == 0 && result.Cleaned && !result.Truncated && diagnostics.Completeness == evidence.Complete {
		if err = validateStaticELF(binary, p.commandPlatform()); err == nil {
			launcherArtifact, err = s.PutArtifact(binary, "launcher-executable", launcherMaxBytes)
			if err == nil && launcherArtifact.Completeness == evidence.Complete && !launcherArtifact.Redacted && !launcherArtifact.Truncated {
				var stored []byte
				stored, err = s.ReadBlob(launcherArtifact.Content)
				if err == nil && (!bytes.Equal(stored, binary) || hash(stored) != launcherArtifact.Content) {
					err = errors.New("stored launcher bytes do not match preparation output")
				}
			} else if err == nil {
				err = errors.New("stored launcher is redacted, truncated, or incomplete")
			}
		}
	} else if runErr == nil {
		runErr = errors.New("preparation did not exit cleanly or retain complete diagnostics")
	}
	if err != nil {
		runErr = errors.Join(runErr, err)
	}
	if launcherArtifact.Content != "" {
		artifacts = append(artifacts, launcherArtifact)
	}
	status := "failed"
	if runErr == nil {
		status = "completed"
	}
	record := preparationEvidence{Version: 1, Status: status, Plan: prepID, DefinitionDigest: p.definitionDigest, Platform: p.commandPlatform(), Image: sandbox.Image, ExitCode: result.ExitCode, Cleaned: result.Cleaned, Truncated: result.Truncated}
	if launcherArtifact.Content != "" {
		record.Launcher = launcherArtifact.Content
		record.Bytes = len(binary)
	}
	encoded, _ := json.Marshal(record)
	metadata, err := s.PutArtifact(encoded, "preparation-result", 4096)
	if err != nil {
		return nil, artifacts, err
	}
	artifacts = append(artifacts, metadata)
	if metadata.Completeness != evidence.Complete {
		runErr = errors.Join(runErr, errors.New("preparation record is incomplete"))
	}
	if runErr != nil {
		return nil, artifacts, runErr
	}
	return binary, artifacts, nil
}

func validateStaticELF(data []byte, platform string) error {
	if len(data) < 64 || len(data) > launcherMaxBytes {
		return errors.New("launcher executable is empty or outside the 8 MiB transfer bound")
	}
	file, err := elf.NewFile(bytes.NewReader(data))
	if err != nil || file.Class != elf.ELFCLASS64 || (file.Type != elf.ET_EXEC && file.Type != elf.ET_DYN) {
		return errors.New("launcher is not a supported 64-bit ELF executable")
	}
	want := elf.EM_X86_64
	if platform == "linux/arm64" {
		want = elf.EM_AARCH64
	} else if platform != "linux/amd64" {
		return errors.New("unsupported launcher platform")
	}
	if file.Machine != want {
		return errors.New("launcher ELF architecture does not match the approved platform")
	}
	loadable, executable := false, false
	for _, program := range file.Progs {
		switch program.Type {
		case elf.PT_INTERP, elf.PT_DYNAMIC:
			return errors.New("launcher ELF is dynamically linked")
		case elf.PT_LOAD:
			loadable = true
			if program.Flags&elf.PF_X != 0 {
				executable = true
			}
		}
	}
	if !loadable || !executable {
		return errors.New("launcher ELF has no executable load segment")
	}
	return nil
}

func safeRunError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline exceeded"
	}
	if errors.Is(err, sandbox.ErrOutput) {
		return "preparation output limit exceeded"
	}
	return "failed or invalid output"
}

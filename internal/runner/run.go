package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	// Test seam is private: production execution cannot replace the sandbox.
	observe func(context.Context, *sandbox.Experiment, string) (sandbox.ExperimentResult, error)
}

type Response struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}
type Call struct {
	At     int64  `json:"at"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Key    string `json:"key"`
	Body   string `json:"body"`
}
type Observation struct {
	Version   int        `json:"version"`
	Seconds   int64      `json:"seconds"`
	Responses []Response `json:"responses"`
	Calls     []Call     `json:"provider_calls"`
}
type Sample struct {
	RequestID   evidence.Digest          `json:"request_id"`
	Snapshots   evidence.SnapshotPair    `json:"snapshots"`
	Side        string                   `json:"side"`
	CaseSeconds int64                    `json:"case_seconds"`
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

func decode(data string, seconds int64) (Observation, error) {
	var o Observation
	d := json.NewDecoder(bytes.NewBufferString(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&o); err != nil {
		return o, errors.New("invalid observer protocol")
	}
	var tail any
	if d.Decode(&tail) != io.EOF || o.Version != 1 || o.Seconds != seconds || len(o.Responses) != 2 || o.Calls == nil || len(o.Calls) > 128 {
		return o, errors.New("incompatible observer or missing channel")
	}
	for _, r := range o.Responses {
		if r.Status < 100 || r.Status > 599 || len(r.Body) > 4096 {
			return o, errors.New("invalid response channel")
		}
	}
	return o, nil
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

// Run returns a persisted incomplete receipt even on denied consent/cancellation.
// Storage failure is an error: callers must not publish an in-memory result as
// durable. No selection pointer or mutable checkout is consulted after Prepare.
func (e Executor) Run(ctx context.Context, s *store.Store, p *Plan, approved string) (result Result, err error) {
	if p == nil {
		return result, sandbox.ErrConsent
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
	// Preserve the frozen plan alongside every result, including refused runs.
	planArtifact, err := s.PutArtifact(p.preview, "execution-plan", 1<<20)
	if err != nil {
		return result, err
	}
	artifacts := []evidence.Artifact{planArtifact}
	allComplete := allowed && acquired && planArtifact.Completeness == evidence.Complete
	unstable := false
	previous := map[string]evidence.Digest{}
	observe := e.observe
	if observe == nil {
		observe = e.Docker.Observe
	}
	var runErrors []error
	cleanupBlocked := false
	for repetition := 0; repetition < p.repetitions; repetition++ {
		for side, label := range []string{"base", "candidate"} {
			for c, sec := range seconds {
				sample := Sample{RequestID: p.request, Snapshots: p.pair, Side: label, CaseSeconds: sec, Repetition: repetition, StartedAt: time.Now().UTC()}
				var executionErr error
				switch {
				case !allowed:
					executionErr = sandbox.ErrConsent
				case ctx.Err() != nil:
					executionErr = ctx.Err()
				case cleanupBlocked:
					executionErr = errors.New("prior cleanup requires reconciliation")
				default:
					_, consent := p.experiments[side][c].Preview()
					sample.Execution, executionErr = observe(ctx, p.experiments[side][c], consent)
				}
				if (sample.Execution.App.Container != "" && !sample.Execution.App.Cleaned) || (sample.Execution.Observer.Container != "" && !sample.Execution.Observer.Cleaned) {
					cleanupBlocked = true
				}
				sample.FinishedAt = time.Now().UTC()
				sample.Status = status(executionErr, sample.Execution)
				if sample.Status == "completed" {
					observation, de := decode(sample.Execution.Observer.Output, sec)
					if de != nil {
						sample.Status = "incompatible_or_missing_channel"
						executionErr = de
					} else {
						raw, _ := json.Marshal(observation)
						a, se := s.PutArtifact(raw, fmt.Sprintf("%s/%d/%d/observation", label, sec, repetition), int64(p.limits.OutputBytes))
						if se != nil {
							return result, se
						}
						sample.Artifacts = append(sample.Artifacts, a)
						if a.Completeness != evidence.Complete {
							sample.Status = "redacted_or_truncated"
						} else {
							key := fmt.Sprintf("%s/%d", label, sec)
							if prior, ok := previous[key]; ok && prior != a.Content {
								unstable = true
							}
							previous[key] = a.Content
						}
					}
				}
				// Candidate diagnostics can never be parsed as observation. The store
				// redacts configured literals before retention; errors use fixed codes.
				for _, diagnostic := range []struct{ name, data string }{{"candidate-diagnostics", sample.Execution.App.Output}, {"observer-diagnostics", sample.Execution.Observer.Output}} {
					if diagnostic.name == "observer-diagnostics" && sample.Status == "completed" {
						continue
					}
					a, se := s.PutArtifact([]byte(diagnostic.data), fmt.Sprintf("%s/%d/%d/%s", label, sec, repetition, diagnostic.name), int64(p.limits.OutputBytes))
					if se != nil {
						return result, se
					}
					sample.Artifacts = append(sample.Artifacts, a)
				}
				sample.Execution.App.Output = ""
				sample.Execution.Observer.Output = ""
				if sample.Status != "completed" {
					allComplete = false
					runErrors = append(runErrors, fmt.Errorf("%s/%d/%d: %s", label, sec, repetition, sample.Status))
				}
				for _, a := range sample.Artifacts {
					if a.Completeness != evidence.Complete {
						allComplete = false
					}
				}
				artifacts = append(artifacts, sample.Artifacts...)
				metadata, _ := json.Marshal(sample)
				a, se := s.PutArtifact(metadata, fmt.Sprintf("%s/%d/%d/sample", label, sec, repetition), 1<<20)
				if se != nil {
					return result, se
				}
				artifacts = append(artifacts, a)
				if a.Completeness != evidence.Complete {
					allComplete = false
				}
				result.Samples = append(result.Samples, sample)
			}
		}
	}
	b := p.scenario
	receipt := evidence.Receipt{RequestID: p.request, SchemaVersion: 1, State: evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.NoEvidence, Applicability: evidence.Unknown, Execution: evidence.Failed, Comparison: evidence.Incomparable, Report: evidence.NoReport}, Snapshots: p.pair, Bindings: &evidence.Bindings{Scenario: b.ID, Input: b.Input, Driver: b.Driver, Observer: b.Observer, Rules: b.Rules}, BaseEnvironment: &p.environments[0], CandidateEnvironment: &p.environments[1], Authorization: evidence.Digest(p.id), StartedAt: started, FinishedAt: time.Now().UTC(), Completeness: evidence.Incomplete, Artifacts: artifacts, Limits: append([]string(nil), scope...)}
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

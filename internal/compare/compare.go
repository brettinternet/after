package compare

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/store"
)

// Report is a bounded artifact, not inline record values (which the record store
// sanitizes through generic JSON). This preserves numeric precision end to end.
type Report struct {
	Version        int                        `json:"version"`
	Receipt        evidence.Digest            `json:"receipt"`
	Snapshots      evidence.SnapshotPair      `json:"snapshots"`
	Rules          evidence.Digest            `json:"rules,omitempty"`
	DefinitionName string                     `json:"definition_name,omitempty"`
	BuiltInPayment bool                       `json:"built_in_payment,omitempty"`
	Cases          []string                   `json:"cases"`
	CaseTitles     []string                   `json:"case_titles"`
	Channels       []string                   `json:"channels"`
	Outcome        evidence.ComparisonOutcome `json:"outcome"`
	Artifacts      []evidence.Artifact        `json:"artifacts"`
	Witnesses      []Witness                  `json:"witnesses"`
	Limits         []string                   `json:"limits"`
}
type SampleRef struct {
	Side        string          `json:"side"`
	CaseID      string          `json:"case_id"`
	Seconds     int64           `json:"seconds,omitempty"`
	Repetition  int             `json:"repetition"`
	Metadata    evidence.Digest `json:"metadata"`
	Observation evidence.Digest `json:"observation"`
}
type Witness struct {
	Relation string                     `json:"relation"`
	Channel  string                     `json:"channel"`
	Before   SampleRef                  `json:"before"`
	After    SampleRef                  `json:"after"`
	Outcome  evidence.ComparisonOutcome `json:"outcome"`
	Changes  []Change                   `json:"changes"`
}

type sample struct {
	ref                 SampleRef
	responses, provider any
}

var scope = []string{
	"Only the finite requests and cases declared by this frozen http-service definition in this captured pair; equality is limited to its supported channels and samples, not universal safety, causation or performance.",
	"Fake upstreams are the fixed offline endpoints owned and logged by the frozen observer; calls include declared endpoint, destination, operation, key, payload, timestamp, count and order.",
	"JSON object order and decimal spelling are immaterial; arrays and Unicode code points are exact. No masking, field dropping, Unicode normalization or candidate-owned policies are accepted.",
	"Receipt, snapshot inventories, raw diffs and every retained sample remain inspectable; digest binding does not authenticate a producer. Terminal consumers must escape all untrusted values.",
}

// Run loads a validated receipt, writes bounded detail and comparison records,
// and never executes code. Corrupt/unavailable records are errors; valid but
// unsupported/partial evidence produces a persisted incomparable result.
func Run(s *store.Store, receipt evidence.Digest) (evidence.Comparison, error) {
	r, err := store.Get[evidence.Receipt](s, receipt)
	if err != nil {
		return evidence.Comparison{}, err
	}
	report := Report{Version: 1, Receipt: r.ID, Snapshots: r.Snapshots, Outcome: evidence.Incomparable, Artifacts: r.Artifacts, Witnesses: []Witness{}, Limits: append([]string(nil), scope...)}
	if r.Bindings != nil {
		report.Rules = r.Bindings.Rules
	}
	err = compareReceipt(s, r, &report)
	completeness := evidence.Complete
	if err != nil {
		report.Outcome = evidence.Incomparable
		completeness = evidence.Incomplete
		if !contains(report.Limits, err.Error()) {
			report.Limits = append(report.Limits, err.Error())
		}
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return evidence.Comparison{}, err
	}
	// A bounded error is preferable to publishing a truncated set of witnesses.
	if len(raw) > maxJSON {
		return evidence.Comparison{}, errors.New("comparison detail budget exceeded; receipt remains inspectable")
	}
	detail, err := s.PutArtifact(raw, "comparison-details-v1", maxJSON)
	if err != nil {
		return evidence.Comparison{}, err
	}
	if detail.Completeness != evidence.Complete {
		completeness = evidence.Incomplete
		report.Outcome = evidence.Incomparable
		report.Limits = append(report.Limits, "comparison details redacted or truncated; not conclusive")
		// Keep the artifact's summary consistent with the authoritative record,
		// even when a newly configured redaction policy hides a witness value.
		raw, err = json.Marshal(report)
		if err != nil || len(raw) > maxJSON {
			return evidence.Comparison{}, errors.New("comparison detail budget exceeded")
		}
		detail, err = s.PutArtifact(raw, "comparison-details-v1", maxJSON)
		if err != nil {
			return evidence.Comparison{}, err
		}
	}
	return store.Put(s, evidence.Comparison{SchemaVersion: 1, Receipt: r.ID, Outcome: report.Outcome, Completeness: completeness, Limits: report.Limits, Details: &detail})
}

func compareReceipt(s *store.Store, r evidence.Receipt, report *Report) error {
	if r.Completeness != evidence.Complete || r.State.Kind != evidence.Observed || r.State.Execution != evidence.Completed || r.Redacted {
		return errors.New("missing, failed or incomplete observations; no complete comparison")
	}
	basis, err := runner.ValidateComparisonBasis(s, r)
	if err != nil {
		return err
	}
	if basis.Command != nil {
		return compareCommandReceipt(s, r, report, basis)
	}
	reps := basis.Repetitions
	for _, scenarioCase := range basis.Cases {
		report.Cases = append(report.Cases, scenarioCase.ID)
		report.CaseTitles = append(report.CaseTitles, scenarioCase.Title)
	}
	report.DefinitionName = basis.DefinitionName
	report.BuiltInPayment = basis.BuiltInPayment
	report.Channels = append([]string(nil), basis.Channels...)
	artifacts := map[string]evidence.Artifact{}
	for _, a := range r.Artifacts {
		if _, ok := artifacts[a.Channel]; ok {
			return errors.New("duplicate artifact channel")
		}
		artifacts[a.Channel] = a
	}
	policy, ok := artifacts["comparison-rules"]
	if !ok || policy.Content != r.Bindings.Rules {
		return errors.New("missing or changed comparison policy")
	}
	raw, err := s.ReadBlob(policy.Content)
	if err != nil {
		return err
	}
	if string(raw) != runner.ComparisonRules {
		return errors.New("unsupported masks or normalization policy")
	}
	allowed := map[string]bool{"comparison-rules": true, "execution-plan": true, "preparation-result": true, "preparation-diagnostics": true, "launcher-executable": true}
	samples := map[string]sample{}
	for rep := 0; rep < reps; rep++ {
		for _, side := range []string{"base", "candidate"} {
			for _, scenarioCase := range basis.Cases {
				prefix := fmt.Sprintf("%s/%s/%d/", side, scenarioCase.ID, rep)
				for _, suffix := range []string{"sample", "observation", "candidate-diagnostics", "observer-diagnostics"} {
					allowed[prefix+suffix] = true
				}
				meta, ok := artifacts[prefix+"sample"]
				if !ok {
					return errors.New("missing sample metadata")
				}
				observation, ok := artifacts[prefix+"observation"]
				if !ok {
					return errors.New("missing response or provider channel")
				}
				raw, err := s.ReadBlob(meta.Content)
				if err != nil {
					return err
				}
				var m runner.Sample
				if err = strict(raw, &m); err != nil {
					return errors.New("invalid sample metadata")
				}
				if m.RequestID != r.RequestID || m.Snapshots != r.Snapshots || m.Side != side || m.CaseID != scenarioCase.ID || m.CaseSeconds != runner.CaseDuration(scenarioCase) || m.Repetition != rep || m.Status != "completed" || m.StartedAt.Before(r.StartedAt) || m.FinishedAt.After(r.FinishedAt) || m.FinishedAt.Before(m.StartedAt) || !m.Execution.App.Cleaned || !m.Execution.Observer.Cleaned || m.Execution.App.Truncated || m.Execution.Observer.Truncated || m.Execution.Observer.ExitCode != 0 {
					return errors.New("incompatible, incomplete or misbound sample")
				}
				if !basis.MatchesSample(m) {
					return errors.New("sample execution plan mismatch")
				}
				found := false
				seen := map[string]bool{}
				for _, a := range m.Artifacts {
					stored, ok := artifacts[a.Channel]
					if !ok || !reflect.DeepEqual(stored, a) || seen[a.Channel] || !strings.HasPrefix(a.Channel, prefix) {
						return errors.New("sample artifact binding mismatch")
					}
					seen[a.Channel] = true
					if a.Channel == observation.Channel {
						found = true
					}
				}
				if !found {
					return errors.New("observation not bound to sample")
				}
				raw, err = s.ReadBlob(observation.Content)
				if err != nil {
					return err
				}
				var o runner.Observation
				if completeObservation(raw) != nil || strict(raw, &o) != nil || o.Version != 1 || o.CaseID != scenarioCase.ID || o.Seconds != 0 && o.Seconds != runner.CaseDuration(scenarioCase) || len(o.Responses) != len(scenarioCase.Requests) || o.Calls == nil || len(o.Calls) > 128 {
					return errors.New("unsupported or missing observation channel")
				}
				responses, provider, err := channels(o)
				if err != nil {
					return err
				}
				samples[prefix] = sample{SampleRef{Side: side, CaseID: scenarioCase.ID, Seconds: runner.CaseDuration(scenarioCase), Repetition: rep, Metadata: meta.Content, Observation: observation.Content}, responses, provider}
			}
		}
	}
	for channel := range artifacts {
		if !allowed[channel] {
			return errors.New("new or unsupported artifact channel")
		}
	}
	report.Outcome = evidence.Equal
	total := 0
	compare := func(a, b sample, relation string) error {
		for _, channel := range basis.Channels {
			av, bv := a.responses, b.responses
			if channel == "provider_calls" {
				av, bv = a.provider, b.provider
			}
			changes := []Change{}
			if err := walk(av, bv, "", &changes); err != nil {
				return err
			}
			total += len(changes)
			if total > maxChanges {
				return errors.New("comparison witness budget exceeded")
			}
			outcome := evidence.Equal
			if len(changes) > 0 {
				outcome = evidence.Different
				if relation == "repetition" {
					report.Outcome = evidence.Unstable
				} else if report.Outcome == evidence.Equal {
					report.Outcome = evidence.Different
				}
			}
			report.Witnesses = append(report.Witnesses, Witness{relation, channel, a.ref, b.ref, outcome, changes})
		}
		return nil
	}
	for _, scenarioCase := range basis.Cases {
		for rep := 0; rep < reps; rep++ {
			a := samples[fmt.Sprintf("base/%s/%d/", scenarioCase.ID, rep)]
			b := samples[fmt.Sprintf("candidate/%s/%d/", scenarioCase.ID, rep)]
			if err := compare(a, b, "paired"); err != nil {
				return err
			}
			if rep > 0 {
				for _, side := range []string{"base", "candidate"} {
					if err := compare(samples[fmt.Sprintf("%s/%s/0/", side, scenarioCase.ID)], samples[fmt.Sprintf("%s/%s/%d/", side, scenarioCase.ID, rep)], "repetition"); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// Check presence before decoding into value fields: encoding/json otherwise
// turns omitted/null strings and numbers into indistinguishable zero values.
func completeObservation(raw []byte) error {
	v, err := parse(raw)
	if err != nil {
		return err
	}
	root, ok := v.(map[string]any)
	if !ok {
		return errors.New("missing observation object")
	}
	if _, ok := root["case_id"].(string); !ok {
		return errors.New("missing observation case binding")
	}
	for channel, fields := range map[string][]string{
		"responses":      {"status", "body"},
		"provider_calls": {"at", "endpoint", "destination", "method", "path", "key", "body"},
	} {
		items, ok := root[channel].([]any)
		if !ok {
			return errors.New("missing observation channel")
		}
		for _, item := range items {
			object, ok := item.(map[string]any)
			if !ok {
				return errors.New("missing observation item")
			}
			for _, field := range fields {
				if field == "status" || field == "at" {
					_, ok = object[field].(json.Number)
				} else {
					_, ok = object[field].(string)
				}
				if !ok {
					return errors.New("missing or invalid observation field")
				}
			}
		}
	}
	return nil
}

func strict(raw []byte, v any) error {
	if _, err := parse(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func body(s string) (any, error) {
	if !json.Valid([]byte(s)) {
		return map[string]any{"text": s}, nil
	}
	v, err := parse([]byte(s))
	if err != nil {
		return nil, errors.New("ambiguous or over-budget JSON body")
	}
	return map[string]any{"json": v}, nil
}
func channels(o runner.Observation) (any, any, error) {
	responses := []any{}
	for _, r := range o.Responses {
		if r.Status < 100 || r.Status > 599 || len(r.Body) > 4096 {
			return nil, nil, errors.New("invalid response channel")
		}
		b, err := body(r.Body)
		if err != nil {
			return nil, nil, err
		}
		responses = append(responses, map[string]any{"status": json.Number(fmt.Sprint(r.Status)), "body": b})
	}
	calls := []any{}
	for _, c := range o.Calls {
		if len(c.Body) > 4096 || len(c.Path) > 256 || len(c.Key) > 128 || c.Method == "" || !strings.HasPrefix(c.Path, "/") || c.Endpoint == "" || c.Destination == "" {
			return nil, nil, errors.New("invalid provider channel")
		}
		b, err := body(c.Body)
		if err != nil {
			return nil, nil, err
		}
		calls = append(calls, map[string]any{"at": json.Number(fmt.Sprint(c.At)), "endpoint": c.Endpoint, "destination": c.Destination, "method": c.Method, "path": c.Path, "key": c.Key, "body": b})
	}
	return responses, map[string]any{"count": json.Number(fmt.Sprint(len(calls))), "calls": calls}, nil
}

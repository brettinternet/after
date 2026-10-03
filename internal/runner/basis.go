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

// ComparisonBasis retains the expected execution identities for each sample.
type ComparisonBasis struct {
	Repetitions int
	planIDs     [2][2][2]string
}

func (b ComparisonBasis) MatchesSample(m Sample) bool {
	for side, name := range []string{"base", "candidate"} {
		for c, sec := range seconds {
			if m.Side == name && m.CaseSeconds == sec {
				ids := b.planIDs[side][c]
				return m.Execution.App.Plan == ids[0] && m.Execution.Observer.Plan == ids[1]
			}
		}
	}
	return false
}

// ValidateComparisonBasis reconstructs, but never executes, the shipped plan.
// Environment hashes include source archives, so unequal hashes alone do not
// mean unequal runtime policy. Reconstruct both sides from immutable captures
// instead of ignoring those hashes or demanding identical source dependencies.
// Old/unknown runners and policies fail closed. This requires a writable store
// for idempotent publication of the already-bound scenario/input records.
func ValidateComparisonBasis(s *store.Store, r evidence.Receipt) (basis ComparisonBasis, err error) {
	bad := errors.New("unsupported or incompatible frozen plan, bindings or environments")
	if r.RequestID == "" || r.Bindings == nil {
		return basis, bad
	}
	var plan *evidence.Artifact
	for i := range r.Artifacts {
		if r.Artifacts[i].Channel == "execution-plan" {
			if plan != nil {
				return basis, bad
			}
			plan = &r.Artifacts[i]
		}
	}
	if plan == nil || plan.Completeness != evidence.Complete || plan.Redacted || plan.Truncated {
		return basis, bad
	}
	raw, err := s.ReadBlob(plan.Content)
	if err != nil {
		return basis, err
	}
	if hash(raw) != r.Authorization {
		return basis, bad
	}
	var header struct {
		Repetitions int `json:"repetitions"`
		Experiments [2][2]struct {
			App struct {
				Limits sandbox.Limits `json:"limits"`
			}
		} `json:"experiments"`
	}
	if json.Unmarshal(raw, &header) != nil {
		return basis, bad
	}
	p, err := prepare(s, r.Snapshots, header.Repetitions, header.Experiments[0][0].App.Limits, r.RequestID)
	if err != nil {
		return basis, bad
	}
	b := p.scenario
	bindings := evidence.Bindings{Scenario: b.ID, Input: b.Input, Driver: b.Driver, Observer: b.Observer, Rules: b.Rules}
	if !bytes.Equal(raw, p.preview) || *r.Bindings != bindings || !reflect.DeepEqual(r.BaseEnvironment, &p.environments[0]) || !reflect.DeepEqual(r.CandidateEnvironment, &p.environments[1]) {
		return basis, bad
	}
	return ComparisonBasis{p.repetitions, p.planIDs}, nil
}

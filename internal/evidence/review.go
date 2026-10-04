package evidence

import (
	"errors"
	"reflect"
)

type PinScope string

const (
	FiniteExample PinScope = "finite_example"
	HumanIntent   PinScope = "human_intent"
)

type ReviewMode string

const (
	OriginalBase ReviewMode = "original_base"
	FollowUp     ReviewMode = "last_inspected"
)

// ReviewBasis is the expected whole-project execution boundary, not a result.
// Missing bindings or environments deliberately mean unknown applicability.
type ReviewBasis struct {
	Snapshots            SnapshotPair `json:"snapshots"`
	Bindings             *Bindings    `json:"bindings,omitempty"`
	BaseEnvironment      *Environment `json:"base_environment,omitempty"`
	CandidateEnvironment *Environment `json:"candidate_environment,omitempty"`
}

// ReviewContext is a selection and optional real receipt at one history event.
// It contains no predicted outputs and grants no execution permission.
type ReviewContext struct {
	Action         string      `json:"action"`
	Mode           ReviewMode  `json:"mode"`
	PriorCandidate Digest      `json:"prior_candidate"`
	Target         ReviewBasis `json:"target"`
	Receipt        Digest      `json:"receipt,omitempty"`
}

func BasisOf(r Receipt) ReviewBasis {
	return ReviewBasis{r.Snapshots, r.Bindings, r.BaseEnvironment, r.CandidateEnvironment}
}

func (b ReviewBasis) Validate() error {
	if !digests(b.Snapshots.Base, b.Snapshots.Candidate) {
		return errors.New("review requires a snapshot pair")
	}
	if b.Bindings != nil && !digests(b.Bindings.Scenario, b.Bindings.Input, b.Bindings.Driver, b.Bindings.Observer, b.Bindings.Rules) {
		return errors.New("invalid review bindings")
	}
	for _, env := range []*Environment{b.BaseEnvironment, b.CandidateEnvironment} {
		if env != nil && !environment(env) {
			return errors.New("invalid review environment")
		}
	}
	return nil
}

// ValidateReview preserves legacy pin decoding without assigning it new scope.
func (p Pin) ValidateReview() error {
	if p.Scope == "" {
		for _, e := range p.History {
			if e.Review != nil {
				return errors.New("review requires explicit pin scope")
			}
		}
		return nil
	}
	if !oneOf(p.Scope, FiniteExample, HumanIntent) || len(p.History) > 256 {
		return errors.New("invalid pin scope or history limit")
	}
	for i, e := range p.History {
		c := e.Review
		if c == nil || !oneOf(c.Mode, OriginalBase, FollowUp) || !digest(c.PriorCandidate) || (c.Receipt != "" && !digest(c.Receipt)) {
			return errors.New("invalid review context")
		}
		if err := c.Target.Validate(); err != nil {
			return err
		}
		if c.Mode == OriginalBase && c.Target.Snapshots.Base != p.BasisSnapshots.Base {
			return errors.New("original base changed")
		}
		if c.Mode == FollowUp && c.Target.Snapshots.Base != c.PriorCandidate {
			return errors.New("follow-up base is not last inspected candidate")
		}
		if i == 0 {
			if c.Action != "pin" || c.Mode != OriginalBase || c.Target.Snapshots != p.BasisSnapshots || c.PriorCandidate != p.BasisSnapshots.Candidate || c.Receipt != p.BasisReceipt || c.Target.Bindings == nil || c.Target.Bindings.Scenario != p.Scenario {
				return errors.New("pin review basis mismatch")
			}
			continue
		}
		prev := p.History[i-1]
		switch c.Action {
		case "select":
			if c.PriorCandidate != prev.Review.Target.Snapshots.Candidate {
				return errors.New("selection did not use last inspected candidate")
			}
			if !reflect.DeepEqual(c.Target, prev.Review.Target) || c.Mode != prev.Review.Mode {
				if e.Decision != Reopened || c.Receipt != "" {
					return errors.New("changed selection must reopen without a result")
				}
			} else if c.Receipt != prev.Review.Receipt || (e.Decision != prev.Decision && e.Decision != Reopened) {
				return errors.New("selection migrated acceptance or result")
			}
		case "attach", "accept":
			if !reflect.DeepEqual(c.Target, prev.Review.Target) || c.Mode != prev.Review.Mode || c.PriorCandidate != prev.Review.PriorCandidate || c.Receipt == "" {
				return errors.New("receipt or decision changed selection")
			}
			reopened := prev.Decision == Accepted && e.Decision == Reopened && c.Receipt != prev.Review.Receipt
			if c.Action == "attach" && e.Decision != prev.Decision && !reopened {
				return errors.New("receipt cannot accept a pin")
			}
			if c.Action == "accept" && (e.Decision != Accepted || c.Receipt != prev.Review.Receipt) {
				return errors.New("acceptance requires selected receipt")
			}
		default:
			return errors.New("unknown review action")
		}
	}
	return nil
}

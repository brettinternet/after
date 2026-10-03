// Package review separates explicit snapshot selection, immutable observations,
// and human decisions. It never executes or predicts project behavior.
package review

import (
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

const ReuseLimit = "Exact complete whole-project bindings only; finite examples, not universal equivalence. Human intent is not established by a receipt."

type View struct {
	Pin                  evidence.Pin           `json:"pin"`
	Applicability        evidence.Applicability `json:"applicability"`
	Reason               string                 `json:"reason"`
	MissingCurrentResult bool                   `json:"missing_current_result"`
	CurrentReceipt       *evidence.Receipt      `json:"current_receipt,omitempty"`
	Limits               []string               `json:"limits"`
}

func load(s *store.Store, id evidence.Digest) (evidence.Pin, error) {
	p, err := store.Get[evidence.Pin](s, id)
	if err == nil && p.Scope == "" {
		err = errors.New("legacy pin has no explicit scope; create an explicitly scoped pin")
	}
	return p, err
}

// Inspect derives applicability each time. Receipt flags and human acceptance
// cannot override missing or changed bindings. Historical artifacts remain at
// their original IDs; only a selected, bound receipt is exposed as current.
func Inspect(s *store.Store, id evidence.Digest) (View, error) {
	p, err := store.Get[evidence.Pin](s, id)
	if err != nil {
		return View{}, err
	}
	v := View{Pin: p, Applicability: evidence.Unknown, Reason: "legacy pin lacks explicit review scope", MissingCurrentResult: true, Limits: []string{ReuseLimit}}
	if p.Scope == "" {
		return v, nil
	}
	c := p.History[len(p.History)-1].Review
	receiptID := c.Receipt
	if receiptID == "" {
		// Assess the latest real observation against the new selection, but do not
		// return historical values as a prediction for the selected snapshot.
		for i := len(p.History) - 1; i >= 0; i-- {
			if p.History[i].Review.Receipt != "" {
				receiptID = p.History[i].Review.Receipt
				break
			}
		}
	}
	r, err := store.Get[evidence.Receipt](s, receiptID)
	if err != nil {
		return View{}, err
	}
	v.Applicability, v.Reason, err = assess(s, c.Target, r)
	if err != nil {
		return View{}, err
	}
	if c.Receipt != "" {
		v.CurrentReceipt = &r
		v.MissingCurrentResult = r.State.Kind != evidence.Observed || r.State.Execution != evidence.Completed || r.Completeness != evidence.Complete
	}
	if c.Receipt == "" {
		v.Reason += "; selected snapshot has no attached current result"
	}
	return v, nil
}

func assess(s *store.Store, b evidence.ReviewBasis, r evidence.Receipt) (evidence.Applicability, string, error) {
	for _, basis := range []evidence.ReviewBasis{b, evidence.BasisOf(r)} {
		if basis.Bindings == nil || basis.BaseEnvironment == nil || basis.CandidateEnvironment == nil {
			return evidence.Unknown, "unknown whole-project runtime/dependency footprint", nil
		}
		for _, id := range []evidence.Digest{basis.Snapshots.Base, basis.Snapshots.Candidate} {
			snap, err := store.Get[evidence.Snapshot](s, id)
			if err != nil {
				return evidence.Unknown, "", err
			}
			if snap.Completeness != evidence.Complete || len(snap.Excluded) > 0 || len(snap.Unsupported) > 0 {
				return evidence.Unknown, "incomplete or excluded whole-project footprint; cannot establish reuse", nil
			}
		}
	}
	old := evidence.BasisOf(r)
	switch {
	case old.Snapshots != b.Snapshots:
		return evidence.Stale, "whole-project snapshot identity changed; even harmless edits reopen conservatively", nil
	case !reflect.DeepEqual(old.Bindings, b.Bindings):
		return evidence.Stale, "scenario/input/driver/observer/rules identity changed; names do not transfer freshness", nil
	case !reflect.DeepEqual(old.BaseEnvironment, b.BaseEnvironment) || !reflect.DeepEqual(old.CandidateEnvironment, b.CandidateEnvironment):
		return evidence.Stale, "runtime/dependency/environment/argv identity changed", nil
	case r.State.Kind != evidence.Observed || r.State.Execution != evidence.Completed || r.Completeness != evidence.Complete:
		return evidence.Unknown, "receipt is not a complete observed execution", nil
	default:
		return evidence.Current, "identical complete bindings permit bounded reuse only", nil
	}
}

func Create(s *store.Store, receiptID evidence.Digest, expectation string, scope evidence.PinScope, reason string) (evidence.Pin, error) {
	r, err := store.Get[evidence.Receipt](s, receiptID)
	if err != nil {
		return evidence.Pin{}, err
	}
	if r.State.Producer != evidence.Runner || r.Bindings == nil {
		return evidence.Pin{}, errors.New("pin requires a runner receipt with concrete scenario bindings")
	}
	if scope == evidence.FiniteExample && (r.State.Kind != evidence.Observed || r.State.Execution != evidence.Completed || r.Completeness != evidence.Complete) {
		return evidence.Pin{}, errors.New("finite example requires a complete observed result")
	}
	p := evidence.Pin{SchemaVersion: evidence.SchemaVersion, Scenario: r.Bindings.Scenario, Expectation: expectation, BasisReceipt: r.ID, BasisSnapshots: r.Snapshots, Scope: scope}
	c := evidence.ReviewContext{Action: "pin", Mode: evidence.OriginalBase, PriorCandidate: r.Snapshots.Candidate, Target: evidence.BasisOf(r), Receipt: r.ID}
	return appendEvent(s, p, c, evidence.Pinned, reason)
}

// Select explicitly accepts a new selection, not its behavior. Original-base
// comparisons retain the pin's base; follow-ups use the last inspected candidate.
// Changed selections always drop current values, even when returning to an old
// snapshot; the caller must explicitly attach/review that old receipt again.
func Select(s *store.Store, id evidence.Digest, target evidence.ReviewBasis, mode evidence.ReviewMode, reason string) (evidence.Pin, error) {
	p, err := load(s, id)
	if err != nil {
		return evidence.Pin{}, err
	}
	prev := p.History[len(p.History)-1].Review
	c := evidence.ReviewContext{Action: "select", Mode: mode, PriorCandidate: prev.Target.Snapshots.Candidate, Target: target}
	decision := evidence.Reopened
	if reflect.DeepEqual(target, prev.Target) && mode == prev.Mode {
		c.Receipt = prev.Receipt
		v, err := Inspect(s, id)
		if err != nil {
			return evidence.Pin{}, err
		}
		if v.Applicability == evidence.Current {
			decision = p.Decision
		}
	}
	return appendEvent(s, p, c, decision, reason)
}

// Attach never executes or accepts. Reruns must first pass the runner's normal
// digest-bound authorization. A late/mismatched result remains stored at its
// receipt ID but cannot become the selected result here.
func Attach(s *store.Store, id, receiptID evidence.Digest, reason string) (evidence.Pin, error) {
	p, err := load(s, id)
	if err != nil {
		return evidence.Pin{}, err
	}
	r, err := store.Get[evidence.Receipt](s, receiptID)
	if err != nil {
		return evidence.Pin{}, err
	}
	c := *p.History[len(p.History)-1].Review
	if r.State.Producer != evidence.Runner || !reflect.DeepEqual(evidence.BasisOf(r), c.Target) {
		return evidence.Pin{}, errors.New("late or incompatible receipt does not match selected review basis")
	}
	c.Action = "attach"
	c.Receipt = r.ID
	return appendEvent(s, p, c, p.Decision, reason)
}

func Accept(s *store.Store, id evidence.Digest, reason string) (evidence.Pin, error) {
	p, err := load(s, id)
	if err != nil {
		return evidence.Pin{}, err
	}
	v, err := Inspect(s, id)
	if err != nil {
		return evidence.Pin{}, err
	}
	if v.Applicability != evidence.Current || v.MissingCurrentResult || v.CurrentReceipt == nil {
		return evidence.Pin{}, errors.New("acceptance requires explicit review of a current complete receipt")
	}
	c := *p.History[len(p.History)-1].Review
	c.Action = "accept"
	return appendEvent(s, p, c, evidence.Accepted, reason)
}

func appendEvent(s *store.Store, p evidence.Pin, c evidence.ReviewContext, decision evidence.HumanDecision, reason string) (evidence.Pin, error) {
	if strings.TrimSpace(reason) == "" || len(reason) > 4096 || len(p.Expectation) > 4096 {
		return evidence.Pin{}, errors.New("expectation/reason exceeds bounds or reason is empty")
	}
	if len(p.History) >= 256 {
		return evidence.Pin{}, store.ErrLimit
	}
	now := time.Now().UTC()
	if len(p.History) > 0 && now.Before(p.History[len(p.History)-1].At) {
		return evidence.Pin{}, errors.New("clock precedes pin history")
	}
	p.ID = ""
	p.Decision = decision
	p.History = append(p.History, evidence.DecisionEvent{Decision: decision, At: now, Reason: reason, Review: &c})
	return store.Put(s, p)
}

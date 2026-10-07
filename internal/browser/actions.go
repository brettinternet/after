package browser

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

// Actions shares one lazy writer across explicit UI jobs. Store serializes its
// publications; a long sandbox run does not prevent capture or pin selection.
// Close is called only after the model has joined every owned worker.
type Actions struct {
	Project     string
	Repetitions int
	Limits      sandbox.Limits
	Docker      sandbox.Docker
	mu          sync.Mutex
	s           *store.Store
}

func (a *Actions) Store() (*store.Store, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.s == nil {
		s, err := store.Open(a.Project, true, nil)
		if err != nil {
			return nil, err
		}
		a.s = s
	}
	return a.s, nil
}
func (a *Actions) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.s != nil {
		a.s.Close()
		a.s = nil
	}
}
func (a *Actions) Capture(ctx context.Context) (evidence.SnapshotPair, error) {
	s, err := a.Store()
	if err != nil {
		return evidence.SnapshotPair{}, err
	}
	r, err := capture.Capture(ctx, a.Project, s, capture.Options{})
	return evidence.SnapshotPair{Base: r.Base.ID, Candidate: r.Candidate.ID}, err
}
func (a *Actions) Prepare(pair evidence.SnapshotPair) ([]byte, string, error) {
	s, err := a.Store()
	if err != nil {
		return nil, "", err
	}
	p, err := runner.Prepare(s, pair, a.Repetitions, a.Limits)
	if err != nil {
		return nil, "", err
	}
	raw, digest := p.Preview()
	return raw, digest, nil
}
func (a *Actions) Pin(entry Entry, pair evidence.SnapshotPair) (evidence.Digest, error) {
	if entry.Receipt == "" || entry.Expectation == "" {
		return "", errors.New("select a complete current payment observation to pin")
	}
	s, err := a.Store()
	if err != nil {
		return "", err
	}
	r, err := store.Get[evidence.Receipt](s, entry.Receipt)
	if err != nil {
		return "", err
	}
	if r.Snapshots != pair {
		return "", errors.New("historical observation cannot pin the selected pair")
	}
	p, err := review.Create(s, r.ID, entry.Expectation, evidence.FiniteExample, "Explicit TUI p: preserve selected finite provider-request count, not whole-change approval")
	return p.ID, err
}
func (a *Actions) Select(sel Selection, pair evidence.SnapshotPair) (Selection, error) {
	s, err := a.Store()
	if err != nil {
		return sel, err
	}
	target := evidence.ReviewBasis{Snapshots: pair}
	p, err := runner.Prepare(s, pair, a.Repetitions, a.Limits)
	if err == nil {
		target = p.ReviewBasis()
	}
	next := sel
	next.Pair = pair
	next.Evidence = append([]evidence.Digest(nil), sel.Evidence...)
	for i, id := range sel.Evidence {
		old, err := store.Get[evidence.Pin](s, id)
		if errors.Is(err, store.ErrCorrupt) {
			return sel, err
		}
		if err != nil {
			continue
		}
		if old.BasisSnapshots.Base != pair.Base {
			return sel, errors.New("snapshot acceptance retains the original base")
		}
		pin, err := review.Select(s, id, target, evidence.OriginalBase, "Explicit TUI u: use captured snapshot, not its behavior")
		if err != nil {
			return sel, err
		}
		next.Evidence[i] = pin.ID
	}
	return next, nil
}
func (a *Actions) Run(ctx context.Context, pair evidence.SnapshotPair, preview []byte, digest string) (evidence.Digest, error) {
	s, err := a.Store()
	if err != nil {
		return "", err
	}
	p, err := runner.PrepareFromPreview(s, preview)
	if err != nil {
		return "", err
	}
	_, want := p.Preview()
	if want != digest || p.ReviewBasis().Snapshots != pair {
		return "", sandbox.ErrConsent
	}
	if a.Docker.Binary == "" || a.Docker.Host == "" {
		return "", errors.New("execution requires explicit AFTER_DOCKER_BINARY and AFTER_DOCKER_HOST; no host fallback")
	}
	result, runErr := (runner.Executor{Docker: a.Docker}).Run(ctx, s, p, digest)
	if result.Receipt.ID == "" {
		return "", runErr
	}
	comparison, err := compare.Run(s, result.Receipt.ID)
	return comparison.ID, errors.Join(runErr, err)
}
func (a *Actions) Attach(sel Selection, comparison evidence.Digest) (Selection, error) {
	s, err := a.Store()
	if err != nil {
		return sel, err
	}
	c, err := store.Get[evidence.Comparison](s, comparison)
	if err != nil {
		return sel, err
	}
	r, err := store.Get[evidence.Receipt](s, c.Receipt)
	if err != nil {
		return sel, err
	}
	if r.Snapshots != sel.Pair {
		return sel, errors.New("late result retained for originating snapshots only")
	}
	// Check before appending pin revisions so none are orphaned from the selection.
	if len(sel.Evidence) >= MaxEvidence {
		return sel, errors.New("evidence limit reached; result remains stored")
	}
	next := sel
	next.Evidence = append([]evidence.Digest(nil), sel.Evidence...)
	for i, id := range sel.Evidence {
		if _, err := store.Get[evidence.Pin](s, id); errors.Is(err, store.ErrCorrupt) {
			return sel, err
		} else if err != nil {
			continue
		}
		pin, err := review.Attach(s, id, r.ID, "TUI rerun: attach measured result without accepting behavior")
		if err != nil {
			return sel, fmt.Errorf("result retained at %s: %w", comparison, err)
		}
		next.Evidence[i] = pin.ID
	}
	next.Evidence = append(next.Evidence, comparison)
	return next, nil
}

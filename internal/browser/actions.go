package browser

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

// Actions shares one lazy writer across explicit UI jobs. Store serializes its
// publications; a long sandbox run does not prevent capture or pin selection.
// Close is called only after the model has joined every owned worker.
type Actions struct {
	Project        string
	CaptureOptions capture.Options
	Repetitions    int
	Limits         sandbox.Limits
	Docker         sandbox.Docker
	mu             sync.Mutex
	s              *store.Store
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
	options := a.CaptureOptions
	options.IncludeUntracked = append([]string(nil), a.CaptureOptions.IncludeUntracked...)
	r, err := capture.Capture(ctx, a.Project, s, options)
	return evidence.SnapshotPair{Base: r.Base.ID, Candidate: r.Candidate.ID}, err
}
func (a *Actions) SaveReviewSession(session ReviewSession) error {
	raw, err := MarshalReviewSession(session)
	if err != nil {
		return err
	}
	s, err := a.Store()
	if err != nil {
		return err
	}
	return s.WriteReviewSession(raw)
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
func (a *Actions) FindDuplicatePin(entry Entry) (evidence.Pin, bool, error) {
	if entry.Receipt == "" || entry.Expectation == "" {
		return evidence.Pin{}, false, errors.New("select a complete current observation with an expectation to pin")
	}
	s, err := store.Open(a.Project, false, nil)
	if err != nil {
		return evidence.Pin{}, false, err
	}
	defer s.Close()
	pins, err := review.Heads(s)
	if err != nil {
		return evidence.Pin{}, false, err
	}
	for _, pin := range pins {
		if pin.BasisReceipt == entry.Receipt && pin.Expectation == entry.Expectation {
			return pin, true, nil
		}
	}
	return evidence.Pin{}, false, nil
}
func (a *Actions) Pin(entry Entry, pair evidence.SnapshotPair, reason string) (evidence.Digest, error) {
	if entry.Receipt == "" || entry.Expectation == "" {
		return "", errors.New("select a complete current observation with an expectation to pin")
	}
	s, err := a.Store()
	if err != nil {
		return "", err
	}
	if duplicate, found, err := a.FindDuplicatePin(entry); err != nil {
		return "", err
	} else if found {
		return "", fmt.Errorf("matching pin already exists: %s", duplicate.ID)
	}
	r, err := store.Get[evidence.Receipt](s, entry.Receipt)
	if err != nil {
		return "", err
	}
	if r.Snapshots != pair {
		return "", errors.New("historical observation cannot pin the selected pair")
	}
	p, err := review.Create(s, r.ID, entry.Expectation, evidence.FiniteExample, reason)
	return p.ID, err
}
func (a *Actions) ChangedPathCount(before, after evidence.Digest) (int, error) {
	s, err := store.Open(a.Project, false, nil)
	if err != nil {
		return 0, err
	}
	defer s.Close()
	oldSnapshot, err := store.Get[evidence.Snapshot](s, before)
	if err != nil {
		return 0, err
	}
	newSnapshot, err := store.Get[evidence.Snapshot](s, after)
	if err != nil {
		return 0, err
	}
	view, err := rawdiff.Open(s, oldSnapshot, newSnapshot)
	if err != nil {
		return 0, err
	}
	return len(view.Inventory()), nil
}
func (a *Actions) Select(sel Selection, pair evidence.SnapshotPair, mode evidence.ReviewMode, reason string) (Selection, error) {
	s, err := a.Store()
	if err != nil {
		return sel, err
	}
	if mode != evidence.OriginalBase && mode != evidence.FollowUp {
		return sel, errors.New("unsupported comparison mode")
	}
	target := evidence.ReviewBasis{Snapshots: pair}
	p, err := runner.Prepare(s, pair, a.Repetitions, a.Limits)
	if err == nil {
		target = p.ReviewBasis()
	}
	type selectedPin struct {
		index int
		id    evidence.Digest
	}
	pins := []selectedPin{}
	for i, id := range sel.Evidence {
		old, err := store.Get[evidence.Pin](s, id)
		if errors.Is(err, store.ErrCorrupt) {
			return sel, err
		}
		if err != nil {
			continue
		}
		last := old.History[len(old.History)-1].Review
		if mode == evidence.OriginalBase && pair.Base != old.BasisSnapshots.Base {
			return sel, errors.New("original-base selection must retain the pin's original base")
		}
		if mode == evidence.FollowUp && pair.Base != last.Target.Snapshots.Candidate {
			return sel, errors.New("last-inspected selection must start at the pin's current candidate")
		}
		pins = append(pins, selectedPin{index: i, id: id})
	}
	next := sel
	next.Pair = pair
	next.Mode = mode
	if next.Baseline == "" {
		next.Baseline = sel.Pair.Base
	}
	next.Evidence = append([]evidence.Digest(nil), sel.Evidence...)
	next.PinRevisions = make([]evidence.Digest, 0, len(pins))
	for _, selected := range pins {
		pin, err := review.Select(s, selected.id, target, mode, reason)
		if err != nil {
			return sel, err
		}
		next.Evidence[selected.index] = pin.ID
		next.PinRevisions = append(next.PinRevisions, pin.ID)
	}
	return next, nil
}
func (a *Actions) AcceptPin(id evidence.Digest, reason string) (evidence.Digest, error) {
	s, err := a.Store()
	if err != nil {
		return "", err
	}
	pin, err := review.Accept(s, id, reason)
	return pin.ID, err
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
	next.PinRevisions = make([]evidence.Digest, 0, len(sel.Evidence))
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
		next.PinRevisions = append(next.PinRevisions, pin.ID)
	}
	next.Evidence = append(next.Evidence, comparison)
	return next, nil
}

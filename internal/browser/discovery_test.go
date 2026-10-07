package browser

import (
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/store"
)

func TestDiscoveryKeepsForksPrioritizesReviewAndBoundsResults(t *testing.T) {
	s, selection := setup(t, false)
	selection.Pair = evidence.SnapshotPair{
		Base:      discoveryCleanSnapshot(t, s, selection.Pair.Base),
		Candidate: discoveryCleanSnapshot(t, s, selection.Pair.Candidate),
	}
	comparisonID := viewComparison(t, s, selection, false, evidence.Complete)
	comparison, err := store.Get[evidence.Comparison](s, comparisonID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.Get[evidence.Receipt](s, comparison.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	originalPin, err := review.Create(s, receipt.ID, "forked expectation", evidence.FiniteExample, "initial expectation")
	if err != nil {
		t.Fatal(err)
	}
	firstCandidate := discoveryCandidate(t, s, selection.Pair.Candidate, "first fork")
	secondCandidate := discoveryCandidate(t, s, selection.Pair.Candidate, "second fork")
	firstBasis, secondBasis := evidence.BasisOf(receipt), evidence.BasisOf(receipt)
	firstBasis.Snapshots.Candidate = firstCandidate
	secondBasis.Snapshots.Candidate = secondCandidate
	firstFork, err := review.Select(s, originalPin.ID, firstBasis, evidence.OriginalBase, "first fork")
	if err != nil {
		t.Fatal(err)
	}
	secondFork, err := review.Select(s, originalPin.ID, secondBasis, evidence.OriginalBase, "second fork")
	if err != nil {
		t.Fatal(err)
	}
	currentPin, err := review.Create(s, receipt.ID, "current expectation", evidence.FiniteExample, "independent current pin")
	if err != nil {
		t.Fatal(err)
	}
	if firstFork.Decision != evidence.Reopened || secondFork.Decision != evidence.Reopened || currentPin.Decision != evidence.Pinned {
		t.Fatalf("unexpected fork decisions: first=%s second=%s current=%s", firstFork.Decision, secondFork.Decision, currentPin.Decision)
	}
	currentView, err := review.Inspect(s, currentPin.ID)
	if err != nil || currentView.Applicability != evidence.Current || currentView.MissingCurrentResult {
		t.Fatalf("current pin fixture is not reusable: %+v %v", currentView, err)
	}
	report, err := gotestreport.Import(strings.NewReader("{\"Action\":\"pass\",\"Package\":\"example.com/discovery\"}\n"), gotestreport.Metadata{Producer: "discovery fixture", Snapshot: selection.Pair.Candidate, ImportedAt: viewTime})
	if err != nil {
		t.Fatal(err)
	}
	reportArtifact := viewArtifact(t, s, report, "report")

	found, omitted, err := discoverEvidence(s, selection.Pair)
	if err != nil || omitted != 0 {
		t.Fatalf("initial discovery: %v omitted=%d err=%v", found, omitted, err)
	}
	foundSet := make(map[evidence.Digest]bool, len(found))
	for _, id := range found {
		foundSet[id] = true
	}
	for _, id := range []evidence.Digest{firstFork.ID, secondFork.ID, currentPin.ID, receipt.ID, comparisonID, reportArtifact.Content} {
		if !foundSet[id] {
			t.Fatalf("discovery omitted matching ID %s from %v", id, found)
		}
	}
	if foundSet[originalPin.ID] {
		t.Fatal("discovery loaded an ancestor instead of its fork heads")
	}
	firstThree := make(map[evidence.Digest]bool)
	for _, id := range found[:3] {
		firstThree[id] = true
	}
	if !firstThree[firstFork.ID] || !firstThree[secondFork.ID] || found[2] != currentPin.ID {
		t.Fatalf("pins needing another look were not prioritized as distinct forks: got=%v fork-a=%s fork-b=%s current=%s", found, firstFork.ID, secondFork.ID, currentPin.ID)
	}

	for i := 1; i < MaxEvidence+8; i++ {
		clone := receipt
		clone.ID = ""
		clone.StartedAt = viewTime.Add(time.Duration(i) * time.Minute)
		clone.FinishedAt = clone.StartedAt.Add(time.Second)
		clone, err = store.Put(s, clone)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Put(s, evidence.Comparison{SchemaVersion: 1, Receipt: clone.ID, Outcome: evidence.Different, Completeness: evidence.Complete, Limits: []string{"synthetic discovery record"}}); err != nil {
			t.Fatal(err)
		}
	}
	bounded, omitted, err := discoverEvidence(s, selection.Pair)
	if err != nil {
		t.Fatal(err)
	}
	const expectedRecords = 3 + 2*(MaxEvidence+8) + 1
	if len(bounded) != MaxEvidence || omitted != expectedRecords-MaxEvidence {
		t.Fatalf("discovery bound: loaded=%d omitted=%d, want %d loaded and %d omitted", len(bounded), omitted, MaxEvidence, expectedRecords-MaxEvidence)
	}
	if (bounded[0] != firstFork.ID && bounded[1] != firstFork.ID) || (bounded[0] != secondFork.ID && bounded[1] != secondFork.ID) || bounded[2] != currentPin.ID {
		t.Fatalf("bounded discovery displaced pins needing review: %v", bounded[:3])
	}
}

func discoveryCleanSnapshot(t *testing.T, s *store.Store, id evidence.Digest) evidence.Digest {
	t.Helper()
	snapshot, err := store.Get[evidence.Snapshot](s, id)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ID = ""
	snapshot.Excluded = nil
	snapshot.Unsupported = nil
	stored, err := store.Put(s, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return stored.ID
}

func discoveryCandidate(t *testing.T, s *store.Store, id evidence.Digest, source string) evidence.Digest {
	t.Helper()
	snapshot, err := store.Get[evidence.Snapshot](s, id)
	if err != nil {
		t.Fatal(err)
	}
	content, err := s.PutArtifact([]byte(source), "synthetic-source", 1024)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ID = ""
	snapshot.Files[0].Content = content.Content
	snapshot.Diff = content.Content
	stored, err := store.Put(s, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return stored.ID
}

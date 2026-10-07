package cli

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/store"
	ucli "github.com/urfave/cli/v2"
)

func startOrResumeReview(state *invocation, ctx *ucli.Context, requested capture.Options) error {
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	if !state.tty || !state.stderrTTY {
		return invalidWithFix("review requires a terminal", "nonterminal review inspection is unavailable until AFTER-37")
	}
	saved, found, invalidSession, err := readReviewSession(cfg.Project)
	if err != nil {
		return operational("cannot read private review session")
	}
	if invalidSession {
		_, _ = fmt.Fprintln(state.stderr, "after: saved review is invalid; it will be replaced at the next save")
	}
	optionsWereSet := ctx.IsSet("staged") || ctx.IsSet("base") || ctx.IsSet("target") || ctx.IsSet("include-untracked")
	if found && !ctx.Bool("new") && (!optionsWereSet || reflect.DeepEqual(browser.NewReviewSession(saved.Pair, requested).Capture, saved.Capture)) {
		selection := browser.Selection{Project: cfg.Project, Pair: saved.Pair, Discover: true}
		return runBrowser(state, ctx, cfg, selection, saved, true, true)
	}

	pair, err := captureForReview(state, cfg.Project, requested)
	if err != nil {
		return err
	}
	if !found && !ctx.Bool("new") {
		hasChanges, err := capturedPairHasChanges(cfg.Project, pair)
		if err != nil {
			return operational("cannot inspect the captured change")
		}
		if !hasChanges {
			_, _ = fmt.Fprintln(state.stderr, "Nothing to review: the capture has no changed or unknown paths.")
			return nil
		}
	}
	session := browser.NewReviewSession(pair, requested)
	if err := saveReviewSessionFile(cfg.Project, session); err != nil {
		return operational("cannot save private review session")
	}
	if found {
		_, _ = fmt.Fprintf(state.stderr, "Replaced saved review %s → %s with %s → %s\n", shortID(saved.Pair.Base), shortID(saved.Pair.Candidate), shortID(pair.Base), shortID(pair.Candidate))
	}
	selection := browser.Selection{Project: cfg.Project, Pair: pair, Discover: true}
	return runBrowser(state, ctx, cfg, selection, session, false, true)
}

func readReviewSession(project string) (browser.ReviewSession, bool, bool, error) {
	var empty browser.ReviewSession
	s, err := store.Open(project, false, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return empty, false, false, nil
		}
		return empty, false, false, err
	}
	defer s.Close()
	raw, err := s.ReadReviewSession()
	if errors.Is(err, os.ErrNotExist) {
		return empty, false, false, nil
	}
	if err != nil {
		return empty, false, true, nil
	}
	session, err := browser.DecodeReviewSession(raw)
	if err != nil {
		return empty, false, true, nil
	}
	return session, true, false, nil
}

func openExplicitReviewPair(state *invocation, ctx *ucli.Context, args []string) error {
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	if !state.tty || !state.stderrTTY {
		return invalidWithFix("review requires a terminal", "run after review BASE CANDIDATE in a terminal")
	}
	if len(args)-2 > 32 {
		return invalidWithFix("review accepts at most 32 evidence IDs", "use after review BASE CANDIDATE with no more than 32 evidence IDs")
	}
	selection, err := resolveBrowserSelection(cfg.Project, args[0], args[1], args[2:])
	if err != nil {
		return err
	}
	if len(args) == 2 {
		selection.Discover = true
	}
	session := browser.NewReviewSession(selection.Pair, capture.Options{Mode: evidence.WorkingTree})
	return runBrowser(state, ctx, cfg, selection, session, false, false)
}

func openExplicitReviewID(state *invocation, ctx *ucli.Context, rawID string) error {
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	if !state.tty || !state.stderrTTY {
		return invalidWithFix("review requires a terminal", "run after review ID in a terminal")
	}
	s, err := store.Open(cfg.Project, false, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return noIDMatch(rawID, "snapshot", "receipt", "comparison", "report", "pin")
		}
		return operational("cannot open private evidence store for reading")
	}
	defer s.Close()
	id, err := resolveID(s, rawID, "snapshot", "receipt", "comparison", "report", "pin")
	if err != nil {
		return err
	}
	kind, _, err := describeID(s, store.Entry{ID: id, Kind: recordStorageKind(s, id)})
	if err != nil {
		return operational("matching review record is corrupt or unavailable")
	}
	selection := browser.Selection{Project: cfg.Project}
	switch kind {
	case "snapshot":
		selection.Pair, err = pairForCandidate(s, id)
		selection.Discover = true
	case "receipt":
		var receipt evidence.Receipt
		receipt, err = store.Get[evidence.Receipt](s, id)
		selection.Pair = receipt.Snapshots
		selection.Evidence = []evidence.Digest{id}
	case "comparison":
		var comparison evidence.Comparison
		comparison, err = store.Get[evidence.Comparison](s, id)
		if err == nil {
			var receipt evidence.Receipt
			receipt, err = store.Get[evidence.Receipt](s, comparison.Receipt)
			selection.Pair = receipt.Snapshots
			selection.Evidence = []evidence.Digest{id}
		}
	case "pin":
		var pin evidence.Pin
		pin, err = store.Get[evidence.Pin](s, id)
		if err == nil {
			selection.Pair = pin.BasisSnapshots
			if len(pin.History) > 0 && pin.History[len(pin.History)-1].Review != nil {
				selection.Pair = pin.History[len(pin.History)-1].Review.Target.Snapshots
			}
			selection.Evidence = []evidence.Digest{id}
		}
	case "report":
		var report gotestreport.Report
		raw, readErr := s.ReadBlob(id)
		if readErr != nil || strictJSON(raw, &report) != nil || !validImportedReport(report) || report.Metadata.Snapshot == "" {
			return invalidWithFix("report has no valid candidate binding", "open a report bound to a stored candidate snapshot")
		}
		selection.Pair, err = pairForCandidate(s, report.Metadata.Snapshot)
		selection.Evidence = []evidence.Digest{id}
	default:
		return invalidWithFix("ID is not a reviewable record", "use a snapshot, receipt, comparison, report, pin revision, or BASE CANDIDATE pair")
	}
	if err != nil {
		return invalidWithFix("review record is unavailable", "inspect the stored ID or open a complete stored snapshot pair")
	}
	session := browser.NewReviewSession(selection.Pair, capture.Options{Mode: evidence.WorkingTree})
	return runBrowser(state, ctx, cfg, selection, session, false, false)
}

func recordStorageKind(s *store.Store, id evidence.Digest) string {
	for _, kind := range []string{"snapshot", "receipt", "comparison", "pin"} {
		if _, err := resolveStoredRecord(s, id, kind); err == nil {
			return kind
		}
	}
	if _, err := s.ReadBlob(id); err == nil {
		return "blob"
	}
	return ""
}

func resolveStoredRecord(s *store.Store, id evidence.Digest, kind string) (any, error) {
	switch kind {
	case "snapshot":
		return store.Get[evidence.Snapshot](s, id)
	case "receipt":
		return store.Get[evidence.Receipt](s, id)
	case "comparison":
		return store.Get[evidence.Comparison](s, id)
	case "pin":
		return store.Get[evidence.Pin](s, id)
	default:
		return nil, errors.New("unsupported record")
	}
}

func pairForCandidate(s *store.Store, candidate evidence.Digest) (evidence.SnapshotPair, error) {
	if _, err := store.Get[evidence.Snapshot](s, candidate); err != nil {
		return evidence.SnapshotPair{}, err
	}
	history, err := store.CapturesForSnapshot(s, candidate)
	if err != nil {
		return evidence.SnapshotPair{}, err
	}
	for _, record := range history.Records {
		if record.Candidate == candidate {
			return evidence.SnapshotPair{Base: record.Base, Candidate: candidate}, nil
		}
	}
	return evidence.SnapshotPair{Base: candidate, Candidate: candidate}, nil
}

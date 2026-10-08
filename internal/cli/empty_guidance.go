package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
)

type emptyCaptureAdvice struct {
	Message         string
	Details         []string
	ExcludedPaths   []string
	ExcludedOmitted int
	Next            []nextCommand
}

func (guidance emptyCaptureAdvice) nextBlock() ([]nextCommand, bool) {
	return guidance.Next, true
}

type captureOutput struct {
	Base      snapshotSummary     `json:"base_snapshot"`
	Candidate snapshotSummary     `json:"candidate_snapshot"`
	Index     *snapshotSummary    `json:"index_snapshot,omitempty"`
	Guidance  *emptyCaptureAdvice `json:"-"`
	Next      []nextCommand       `json:"-"`
}

func (output captureOutput) nextBlock() ([]nextCommand, bool) {
	if output.Guidance != nil {
		return output.Guidance.Next, true
	}
	return output.Next, len(output.Next) > 0
}

// captureReviewNext names the review command that opens this capture: a plain
// review recaptures with the same flags, and --new replaces a different saved
// review instead of silently resuming it.
func captureReviewNext(project string, pair evidence.SnapshotPair, options capture.Options) nextCommand {
	flags := captureFlagArgs(options)
	saved, found, _, err := readReviewSession(project)
	switch {
	case err == nil && found && saved.Pair == pair:
		return next("after review", "resume the saved review of this change")
	case err == nil && found:
		return next("after review --new"+flags, "replace the saved review with this change")
	default:
		return next("after review"+flags, "open a review of this change")
	}
}

func captureFlagArgs(options capture.Options) string {
	var args []string
	switch options.Mode {
	case evidence.Index:
		args = append(args, "--staged")
	case evidence.MergeBase:
		if !printableArgument(options.Base) || !printableArgument(options.Target) {
			return ""
		}
		args = append(args, "--base "+shellQuote(options.Base))
		if options.Target != "HEAD" {
			args = append(args, "--target "+shellQuote(options.Target))
		}
	}
	for _, path := range options.IncludeUntracked {
		if printableArgument(path) {
			args = append(args, "--include-untracked="+shellQuote(path))
		}
	}
	if len(args) == 0 {
		return ""
	}
	return " " + strings.Join(args, " ")
}

type emptyReviewOutput struct{ Guidance emptyCaptureAdvice }

func (output emptyReviewOutput) nextBlock() ([]nextCommand, bool) {
	return output.Guidance.Next, true
}

func capturedPairState(s *store.Store, pair evidence.SnapshotPair) (evidence.Snapshot, evidence.Snapshot, []string, bool, error) {
	base, err := store.Get[evidence.Snapshot](s, pair.Base)
	if err != nil {
		return evidence.Snapshot{}, evidence.Snapshot{}, nil, false, err
	}
	candidate, err := store.Get[evidence.Snapshot](s, pair.Candidate)
	if err != nil {
		return evidence.Snapshot{}, evidence.Snapshot{}, nil, false, err
	}
	view, err := rawdiff.Open(s, base, candidate)
	if err != nil {
		return evidence.Snapshot{}, evidence.Snapshot{}, nil, false, err
	}
	if base.Completeness != evidence.Complete || candidate.Completeness != evidence.Complete {
		return base, candidate, nil, false, nil
	}
	var excluded []string
	for _, entry := range view.Inventory() {
		if excludedUntracked(entry) {
			excluded = append(excluded, entry.Path)
			continue
		}
		return base, candidate, nil, false, nil
	}
	return base, candidate, excluded, true, nil
}

func emptyCaptureGuidanceForProject(ctx context.Context, project string, pair evidence.SnapshotPair, options capture.Options) (*emptyCaptureAdvice, bool, error) {
	s, err := store.Open(project, false, nil)
	if err != nil {
		return nil, false, err
	}
	defer s.Close()
	return buildEmptyCaptureGuidance(ctx, project, pair, options, s)
}

func excludedUntracked(entry rawdiff.Entry) bool {
	return entry.Change == "unknown" && len(entry.Limits) == 1 && entry.Limits[0] == "excluded: untracked; not selected"
}

func buildEmptyCaptureGuidance(ctx context.Context, project string, pair evidence.SnapshotPair, options capture.Options, s *store.Store) (*emptyCaptureAdvice, bool, error) {
	_, candidate, excluded, empty, err := capturedPairState(s, pair)
	if err != nil || !empty {
		return nil, empty, err
	}
	guidance := &emptyCaptureAdvice{Message: emptyCaptureMessage(candidate, options)}
	if len(excluded) > 0 {
		guidance.ExcludedPaths = excluded[:min(3, len(excluded))]
		guidance.ExcludedOmitted = len(excluded) - len(guidance.ExcludedPaths)
	}
	branch, found, branchErr := capture.FindDefaultBranch(ctx, project)
	if branchErr == nil && found {
		if branch.Ahead > 0 {
			name := terminal.Sanitize(branch.Name)
			guidance.Details = append(guidance.Details, fmt.Sprintf("HEAD is %d %s ahead of %s.", branch.Ahead, plural(branch.Ahead, "commit"), name))
			if printableArgument(branch.Name) && printableArgument(branch.Reference) {
				guidance.Next = append(guidance.Next, next("after review --base "+shellQuote(branch.Reference), fmt.Sprintf("review the %d commits ahead of %s", branch.Ahead, name)))
			}
		} else {
			guidance.Details = append(guidance.Details, fmt.Sprintf("HEAD has no commits ahead of %s.", terminal.Sanitize(branch.Name)))
		}
	}
	if len(guidance.ExcludedPaths) > 0 {
		args := make([]string, 0, len(guidance.ExcludedPaths))
		for _, path := range guidance.ExcludedPaths {
			if printableArgument(path) {
				args = append(args, "--include-untracked="+shellQuote(path))
			}
		}
		if len(args) > 0 {
			guidance.Next = append(guidance.Next, next("after review "+strings.Join(args, " "), "include the excluded untracked paths"))
		}
	}
	if len(guidance.Next) == 0 {
		guidance.Next = []nextCommand{next("after status", "check the stored capture and review state")}
	}
	return guidance, true, nil
}

func emptyCaptureMessage(candidate evidence.Snapshot, options capture.Options) string {
	switch options.Mode {
	case evidence.Index:
		if candidate.Unborn {
			return "Nothing to review: the index matches unborn HEAD."
		}
		return fmt.Sprintf("Nothing to review: the index matches HEAD (commit %s).", shortCommit(candidate.Commit))
	case evidence.MergeBase:
		return fmt.Sprintf("Nothing to review: merge-base comparison with %s has no changes.", terminal.Sanitize(options.Base))
	default:
		if candidate.Unborn {
			return "Nothing to review: the working tree matches unborn HEAD."
		}
		return fmt.Sprintf("Nothing to review: the working tree matches HEAD (commit %s).", shortCommit(candidate.Commit))
	}
}

func emptyGuidanceLines(guidance emptyCaptureAdvice) []readableLine {
	lines := []readableLine{textLine(guidance.Message, terminal.Strong)}
	for _, detail := range guidance.Details {
		lines = append(lines, textLine(detail, terminal.Plain))
	}
	if len(guidance.ExcludedPaths) > 0 {
		lines = append(lines, textLine("Use --include-untracked to select excluded paths:", terminal.Plain))
		for _, path := range guidance.ExcludedPaths {
			lines = append(lines, textLine("  "+terminal.Sanitize(path), terminal.Plain))
		}
		if guidance.ExcludedOmitted > 0 {
			lines = append(lines, textLine(fmt.Sprintf("  … and %d more", guidance.ExcludedOmitted), terminal.Muted))
		}
	}
	return lines
}

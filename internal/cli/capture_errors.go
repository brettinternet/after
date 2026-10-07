package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/brettinternet/after/internal/capture"
)

type captureFailureInfo struct {
	reason string
	fix    string
}

var captureFailureReasons = map[string]captureFailureInfo{
	"unmerged index is unsupported": {
		reason: "unmerged index is unsupported",
		fix:    "resolve the index conflicts, then retry capture",
	},
	"shallow repositories are unsupported": {
		reason: "shallow repositories are unsupported",
		fix:    "use a complete local clone, then retry capture",
	},
	"sparse or partial repositories are unsupported": {
		reason: "sparse or partial repositories are unsupported",
		fix:    "use a complete local checkout, then retry capture",
	},
	"sparse/skip-worktree or assume-unchanged index is unsupported": {
		reason: "sparse/skip-worktree or assume-unchanged index is unsupported",
		fix:    "disable sparse checkout and clear index flags, then retry",
	},
	"comparison needs exactly one merge base": {
		reason: "comparison needs exactly one merge base",
		fix:    "choose refs with one merge base, then retry capture",
	},
	"unsupported path encoding": {
		reason: "unsupported path encoding",
		fix:    "rename unsupported paths, then retry capture",
	},
	"invalid or private untracked selection": {
		reason: "invalid or private untracked selection",
		fix:    "select a non-ignored file inside the checkout",
	},
	"selected path is not a non-ignored untracked file": {
		reason: "selected path is not a non-ignored untracked file",
		fix:    "select an existing non-ignored untracked file",
	},
	"Git plumbing failed (unsupported repository, missing object or output budget)": {
		reason: "Git plumbing failed (unsupported repository, missing object or output budget)",
		fix:    "check the local checkout and retry capture",
	},
}

func captureFailure(err error) error {
	failure := captureFailureInfo{
		reason: "capture could not read or persist a supported snapshot",
		fix:    "check the checkout and private store, then retry capture",
	}
	switch {
	case errors.Is(err, capture.ErrBudget):
		failure = captureFailureInfo{"capture budget exceeded", "reduce the captured file count or size, then retry"}
	case errors.Is(err, capture.ErrInconsistent):
		failure = captureFailureInfo{"repository changed during capture; retry when writers are idle", "stop edits while capturing, then retry"}
	case errors.Is(err, context.DeadlineExceeded):
		failure = captureFailureInfo{"Git operation timed out", "retry capture when the local checkout is responsive"}
	case errors.Is(err, context.Canceled):
		failure = captureFailureInfo{"capture was interrupted", "retry capture when ready"}
	default:
		if known, ok := captureFailureReasons[err.Error()]; ok {
			failure = known
		}
	}
	return &exitError{
		code:       ExitOperational,
		diagnostic: fmt.Sprintf("after: capture failed: %s — %s", failure.reason, failure.fix),
	}
}

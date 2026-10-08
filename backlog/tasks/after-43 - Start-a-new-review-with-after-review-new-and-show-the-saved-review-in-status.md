---
id: AFTER-43
title: Start a new review with after review --new and show the saved review in status
status: Done
assignee: []
created_date: '2026-10-06 21:41'
updated_date: '2026-10-08 05:17'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-24
  - AFTER-37
documentation:
  - docs/CLI-DESIGN.md
  - docs/TUI-DESIGN.md
  - docs/CLI.md
  - docs/TUI.md
priority: medium
type: feature
ordinal: 43000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
AFTER-24 saves one review per checkout and always resumes it. When a reviewer moves on to an unrelated change with the same capture flags, `after review` reopens the old pair, and the only way out is deleting `.after/session.json` by hand. `after status` does not mention the saved review, and Next blocks written by AFTER-37 cannot suggest a plain `after review` if it did not exist yet.

Scope, per docs/CLI-DESIGN.md "status" and "review" and docs/TUI-DESIGN.md "Launch and resume": `after review --new` captures as usual, replaces the saved review with the fresh pair, and names the replaced review on stderr. When the saved review is on another pair, `after status` shows a `Saved review` row, and its Next block offers resuming and starting over. Next blocks suggest a plain `after review` for the newest capture's pair or the saved review's pair, and the explicit pair otherwise. The saved review is UI state, so no evidence is lost.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 `after review --new` captures with the given flags, replaces the saved review with the fresh pair, names the replaced review on stderr, and opens the fresh pair; nothing is removed from the store, and pins from the old review are still discovered.
- [x] #2 When the saved review is on another pair than the newest capture, `after status` shows a `Saved review` row naming it, its Next block offers `after review` and `after review --new`, and `after status --json` includes the saved pair.
- [x] #3 Next blocks suggest a plain `after review` for the newest capture's pair or the saved review's pair, and `after review BASE CANDIDATE` for any other pair; tests run each suggested command.
- [x] #4 PTY tests cover `--new` with and without a saved review, and docs/CLI.md and docs/TUI.md document starting over.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [x] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse the existing --new replacement and saved-review status implementation; close remaining Next-block gaps with pair-aware suggestions. 2. Extend real 80-column PTY and pipe coverage for fresh/replaced reviews, preservation/discovery, status and executable suggestions, with/without NO_COLOR. 3. Document starting over, run focused CLI and Go checks, obtain one scoped independent review, commit and integrate, then finalize task and remove the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented the remaining pair-aware Next behavior while retaining existing --new and saved-review status paths. Plain review intentionally resumes the saved selection; newest capture remains pending rather than silently replacing it. New PTY test proves --new with/without a session, staged capture, replacement pair announcement, preservation of every prior store entry, and old-pin discovery. At 80 columns with/without NO_COLOR, terminal and pipe status/JSON, inspect and capture passed; review in a pipe correctly exits 2 with after status --json. Excerpts: Saved review; after review; after review --new; Replaced saved review <base> → <candidate>. Suggested historical/newest/saved review commands were executed in PTYs. Verification passed: mise exec -- go test ./internal/cli -run ^TestNewReviewPTYAndSuggestions$ -v; mise exec -- task test:cli; mise exec -- task check:go (build, all race tests, vet, gofmt); mise exec -- task test (race tests, 142 links, 45 backlog tasks). Affected source LSP diagnostics clean. Independent verifier d647e182 passed all four criteria, ran focused tests and diff --check, no concrete findings. No Docker proof needed: execution behavior is unchanged. Next: staged checks, implementation commit, integrate main, finalize task and owned-worktree cleanup.

Delivery: implementation 87bedb9 fast-forwarded into main after passing staged formatting/secrets checks. Independent verification completed before integration. Session-owned after-43-new-review checkout and branch removed with Worktrunk after verifying creation receipt, clean checkout and idle shell-only pane; its matching Herdr workspace disappeared after the post-remove hook. Pre-existing unrelated worktrees were not adopted or modified. Final task metadata is committed separately on main. No remaining blocker or resumable implementation step; no push performed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Completed AFTER-43: pair-aware review suggestions, documented start-over semantics, and real PTY/pipe regression coverage for --new, saved-review status, evidence preservation and pin discovery. Integrated implementation 87bedb9 into main. Focused CLI tests, all Go race tests/build/vet/format, docs/backlog checks and independent verification passed. Owned worktree, branch and workspace cleaned up.
<!-- SECTION:FINAL_SUMMARY:END -->

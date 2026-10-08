---
id: AFTER-44
title: Point reviewers to their branch changes when the working tree matches HEAD
status: Done
assignee: []
created_date: '2026-10-06 21:41'
updated_date: '2026-10-08 06:01'
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
  - docs/CAPTURE.md
priority: medium
type: feature
ordinal: 44000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
When an agent commits its work, the working tree matches HEAD. `after capture` then records an empty change without comment: on a feature branch one commit ahead of `main`, the candidate's diff is the empty-content digest. A plain `after review` would open an empty review. The change is reachable only with `--base main`, and nothing suggests it.

Scope, per docs/CLI-DESIGN.md "review" and "capture": when the fresh capture has no changes and there is no saved review, `after review` opens nothing, exits 0, says what matched, and suggests capture flags that would find a change: `--base` with the default branch when HEAD is ahead of it (with the commit count), and `--include-untracked` when untracked files were excluded. The default branch is the remote default (`refs/remotes/origin/HEAD`), else a local `main`, else `master`, read through the capture package's hardened Git runner. `after capture` still records the empty capture and shows the same suggestions. Nothing is chosen automatically, and no project code runs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 On a feature branch whose changes are committed, `after review` with no saved review says the working tree matches HEAD, suggests `after review --base main` with the number of commits ahead, opens no TUI, and exits 0.
- [x] #2 When untracked files were excluded, the message names up to three, sanitized, with `--include-untracked` suggestions; an empty `--staged` or `--base` capture says what matched; on the default branch with nothing ahead, it says there is nothing to review.
- [x] #3 The default branch comes from `refs/remotes/origin/HEAD`, else a local `main`, else `master`, through the capture package's hardened Git runner; tests cover each source, a detached HEAD, an unborn repository and no candidate branch, and prove no project code runs.
- [x] #4 `after capture` records an empty capture as today and shows the same suggestions in its Next block, and with a saved review, `after review` resumes as usual.
- [x] #5 docs/CLI.md and docs/CAPTURE.md document the guidance.
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
1. Add a bounded default-branch/ahead lookup through capture's existing hardened Git runner; test remote HEAD, main/master fallbacks, detached/unborn/missing branches and hostile configuration.
2. Share clean-capture guidance between capture and fresh implicit review. Distinguish actual/unknown changes from excluded untracked inventory, name at most three safe paths, preserve saved-review and --new behavior, and retain empty capture records.
3. Update CLI/CAPTURE docs and focused tests, including real 80-column PTY and pipe checks with/without NO_COLOR. Run relevant Task checks and one independent verification focused on execution safety and empty-review semantics.
4. Commit implementation, fast-forward main preserving authoritative task state, finalize task via CLI, commit metadata, and remove the verified session-owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation complete in session-owned after-44-clean-guidance worktree; no commits yet. Final task test:cli and go test -race ./internal/capture passed. New real 80-column PTY/pipe tests passed with and without NO_COLOR, including hostile untracked paths. Earlier task test passed (Go race tests, 142 documentation links, 45-task integrity); final small test/output safety adjustments covered by focused reruns. Implementer hit 30-minute timeout during checks, resumed same child and completed; independent verification now running. No product/external blocker.

Final verification: mise exec -- task check:go passed (native build, all Go race tests, vet, gofmt); mise exec -- task test passed on final implementation (all Go race tests, 142 local links in 25 files, 45-task integrity). All changed Go files had clean LSP diagnostics. mise exec -- task check:staged passed formatting and secrets before implementation commit.
Independent verifier PASS: task test:cli; go test ./internal/capture -run TestFindDefaultBranch -count=1; go test ./internal/cli -run "TestEmpty|TestUnknownCaptureInventory" -count=1. No scoped defects. Verified precedence, hardened no-execution lookup, detached/unborn/no-default cases, persistence, saved review, --new, safe untracked guidance and JSON compatibility.
Real 80-column PTY and pipe checks, each with/without NO_COLOR, passed for review/capture. Excerpts: "Nothing to review: the working tree matches HEAD (commit 070add7)."; "HEAD is 3 commits ahead of main."; "after review --base main"; "Use --include-untracked to select excluded paths:" with at most three sanitized names and shell-quoted selection. Non-TTY review still rejects with terminal/status guidance.
Limitations: discovery is local-only, never fetches or chooses a comparison automatically. Unknown/unsupported captures remain inspectable; unsafe control-bearing arguments are not emitted as executable suggestions. No Docker execution proof needed for this capture-only change.
Delivered implementation db015b7, fast-forwarded into main preserving task metadata. Session-owned worktree and branch removed through Worktrunk after receipt verification and inactive-child checks; no matching Herdr workspace before/after cleanup. Pre-existing unrelated worktrees retained, not adopted. No push. No blocker or resumable implementation step remains.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Empty captures now guide reviewers to committed default-branch changes or bounded excluded untracked paths without opening an empty review or executing project code. Preserves capture JSON, empty records, saved reviews and explicit --new. Implemented in db015b7 and integrated to main; check:go, test, focused PTY/pipe checks and independent verification passed. Owned worktree/branch cleaned up.
<!-- SECTION:FINAL_SUMMARY:END -->

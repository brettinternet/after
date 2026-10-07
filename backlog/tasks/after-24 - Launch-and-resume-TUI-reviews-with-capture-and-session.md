---
id: AFTER-24
title: Start and resume reviews with a plain after review
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-07 14:47'
labels:
  - poc
  - tui
  - ux
  - cli
milestone: m-1
dependencies:
  - AFTER-23
  - AFTER-36
  - AFTER-38
documentation:
  - docs/CLI-DESIGN.md
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/CLI.md
  - docs/CAPTURE.md
priority: high
type: feature
ordinal: 24000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Reviewing takes four steps: capture, copy two 71-character IDs, run `review --tui CANDIDATE --base BASE`, then rebuild that command with up to 32 `--evidence` flags to resume. The TUI draws on stdout, so its session JSON can't be piped to a file. Reviewers re-review repeatedly, so one command should start and resume a review.

Scope, per docs/CLI-DESIGN.md "review" and docs/TUI-DESIGN.md "Launch and resume": `after review` captures with the same flags and policy as `after capture`, then opens the pair. The saved review in `.after/session.json` holds the pair, the comparison mode (always original base until AFTER-33) and the capture flags. It is UI state, not evidence, written atomically with mode 0600 and read as untrusted input. With a saved review, the TUI opens at once and captures in the background, a changed capture waits as pending (`u`), and different capture flags start a new saved review; `c` reuses the saved capture flags. Explicit IDs or a pair open without capturing or touching the saved review. Evidence is discovered: pin heads, the pair's newest runs and comparisons, and reports bound to the candidate, within 32 records with pins needing another look first. On quit, stderr names the saved review, and stdout carries the session JSON only with `--json`. Without a terminal, exit 2 pointing to `after status --json`. Nothing else runs on open.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Without a saved review, `after review` in a checkout captures exactly as `after capture` does with the same flags (`--staged`, `--base REF [--target REF]`, `--include-untracked PATH`), showing elapsed time on stderr after one second, writes a capture record, and opens that pair; untracked files stay excluded, no project code runs, and a capture failure exits with the specific sanitized reason after restoring the terminal.
- [x] #2 `.after/session.json` holds the pair, mode and capture flags and no source content; it is written atomically with mode 0600 after every selection change and on quit, an interrupted write leaves the previous valid file, and a non-regular, oversized, unknown-field or invalid-ID file is reported on stderr and replaced at the next save.
- [x] #3 With a saved review, `after review` opens its pair at once and captures in the background, as `c` does; a capture that differs is offered as pending (`u`) without changing the selected pair, and a failed background capture is reported without closing the review. Different capture flags start a new saved review, and stderr names the replaced one.
- [x] #4 `after review ID…` and `after review BASE CANDIDATE` open stored records or the pair without capturing and without reading or writing the saved review, and an explicit pin revision opens exactly that revision.
- [x] #5 Discovery loads each pin's head revisions (forks separately), the pair's newest runs and comparisons, and reports bound to the candidate, at most 32 records with pins needing another look first; the TUI states how many matching records were not loaded and that `after log` lists them; tests cover forks, the limit and an explicit older revision.
- [x] #6 On quit, stderr shows `Saved review <base> → <candidate> · after review resumes it` and stdout is empty unless `--json`; without a terminal, `after review` exits 2 naming `after status --json`; PTY tests cover start, resume, a pending capture, a replaced review and explicit IDs, and docs/CLI.md and docs/TUI.md document the command.
- [x] #7 `c` captures again with the saved review's capture flags, or HEAD against the working tree when explicit IDs opened the review, and the help overlay says which.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [x] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend existing CLI review routing and browser capture actions for no-argument start/resume and explicit record selection, reusing capture options and error handling. 2. Add bounded strict private atomic session UI-state persistence; preserve original base and saved capture options through background capture and selection. 3. Discover bounded relevant evidence using pin heads and stored bindings, preserving explicit revision identity and reporting omitted counts. 4. Add focused storage, discovery, CLI and real PTY coverage (80x24/120x40, color/NO_COLOR); update CLI/TUI docs. 5. Independently verify session trust boundaries and acceptance behavior, fix concrete findings, run relevant Task checks, commit, integrate into main, finalize metadata and remove the verified owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator-approved acceptance adjustment (2026-10-07): AFTER-37 has not implemented after log or after status --json. For AC5/AC6 and matching description wording, use truthful temporary guidance explicitly labeling these commands unavailable until AFTER-37 rather than claiming they work. All remaining review behavior and verification requirements remain unchanged; AFTER-37 is not added to this request. Executor run 0bdf2ca6-7252-42dc-91bf-7eec0ed17c2f timed out at 30 minutes; process termination confirmed, partial implementation diff inspected in owned .worktrees/after-24-review on after-24-review (base 6b2e281). Independent verification did not start. Resume the same child to finish implementation and tests, then run the single independent verification pass.

Implementation checkpoint: plain review start/resume, strict private atomic session storage, pending captures and saved flags, explicit revision selection, bounded discovery and stderr rendering implemented in owned worktree. Executor reports mise exec -- task check passed (Go race tests/build/vet/format, 141 links, 45-task graph, gitleaks), plus git diff --check. Focused TestReviewLaunchResumePTY passed at 80x24 and 120x40 with/without NO_COLOR using -race -v -timeout=45s; invalid-session replacement and explicit older-pin PTYs passed. PTY excerpt: Stored records only; New capture ... — u reviews it; Snapshot selected; prior evidence remains history; Capture running; selected pair unchanged; No new capture; Saved review ... → ... · after review resumes it. FIFO replacement exposed store-budget handling and was fixed/tested; initial clipped help assertion moved to grouped-help coverage and focused PTY rerun passed. Docker tui:proof not run: launch/resume is capture-only with synthetic real-PTY evidence. Independent verifier a6061e1e-e988-4440-9136-a0f00e556130 is now checking trust boundaries and AC outcomes before acceptance/delivery.

Independent verification completed: focused browser/store/CLI/cmd suites and docs:check passed; one concrete defect found, oversized session files were undercounted by the store budget. Parent fixed accounting to count full regular-file size (including invalid permissions), added TestOversizedReviewSessionCountsTowardStoreBudget proving publication rejects over-limit storage and session replacement restores the budget. Final mise exec -- task check:go passed all race suites, build, vet and gofmt; initial 120-second tool window expired, rerun with sufficient window passed (CLI suite 140.165s). LSP store diagnostics clean, git diff --check passed, staged formatting/secrets checks passed via mise exec -- task check:staged. No second general review was needed for this local accounting correction. Implementation 0b36771 fast-forwarded into main after refreshing claim and repository state. Owned worktree/branch removed with Worktrunk after matching creation receipt and verifying its sole Herdr pane was idle zsh; post-remove workspace disappearance confirmed. Pre-existing worktrees were not adopted or modified. AC5/6 checked under the operator-approved unavailable-command wording adjustment above. No remaining task blocker; final metadata commit completes delivery.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented plain after review capture/resume, private atomic session state, saved capture flags and pending selection, bounded evidence discovery, exact explicit revisions, and stderr TUI/stdout JSON separation. Verified with task check, focused real PTYs at 80x24/120x40 with and without NO_COLOR, independent verification, and final task check:go after fixing its sole store-budget finding. Implementation 0b36771 merged to main; owned worktree, branch and workspace cleaned. History/status commands remain explicitly unavailable until AFTER-37 as approved.
<!-- SECTION:FINAL_SUMMARY:END -->

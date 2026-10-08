---
id: AFTER-41
title: Print the captured change with after diff
status: Done
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-08 04:57'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-32
  - AFTER-37
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/RAW-DIFF.md
  - docs/TERMINAL.md
priority: medium
type: feature
ordinal: 41000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The captured patch is only available as base64 inside `inspect` JSON, so a reviewer cannot read or pipe a change from the shell.

Scope, per docs/CLI-DESIGN.md "diff": bare `after diff` prints the newest capture's patch, and `after diff BASE CANDIDATE` takes any two snapshots, using the AFTER-32 computed diff when they share no captured patch. Stdout carries only the patch. Stderr names the pair, the diff's origin, and the excluded, unsupported and unknown paths the patch doesn't cover, and says when sanitizing changed any byte. Lines are sanitized and colored on a terminal like the Diff view; tabs are kept in a pipe. `--raw` writes the exact bytes and is refused when stdout is a terminal. `--stat` prints the capture summary rows. There is no pager.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Bare `after diff` prints the newest capture's patch on stdout and the pair, origin and uncovered paths on stderr; `after diff BASE CANDIDATE` works for any two snapshots and labels a computed diff as such.
- [x] #2 Output escapes control and format characters and invalid UTF-8 through `internal/terminal`, keeps tabs in a pipe, and colors only on a terminal without NO_COLOR; when sanitizing changed any byte, stderr says so and suggests `--raw`, and hostile-patch tests prove no raw control sequence reaches stdout.
- [x] #3 `after diff --raw` into a file or pipe writes bytes identical to the stored patch, so `git apply` reproduces the candidate for an ordinary capture, and is refused with exit 2 when stdout is a terminal.
- [x] #4 `--stat` prints the capture summary rows; the 100,000-line captured diff streams within the docs/TERMINAL.md memory budget.
- [x] #5 `after capture` and `after status` Next blocks offer `after diff` where the design shows it.
- [x] #6 docs/CLI.md and docs/RAW-DIFF.md document the command.
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
1. Reuse captured-pair resolution, rawdiff inventory/computed patches and terminal sanitization for a read-only diff command with raw/stat modes and honest stderr coverage.
2. Add command/help/Next wiring, focused hostile-byte, exact git-apply, arbitrary-pair and 100000-line budget tests; document behavior.
3. Run focused Task checks and actual 80-column PTY/pipe checks with and without NO_COLOR; obtain one independent trust-boundary verification.
4. Commit implementation in the owned worktree, fast-forward main preserving task metadata, finalize authoritative task and remove the verified owned checkout.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation complete in owned after-41-diff branch (not yet integrated). Reused rawdiff and shared Diff theme classification; exact-sized bounded store reads avoid repeated large-blob allocation. Added streamed terminal sanitization, pair/origin/uncovered diagnostics, raw/stat modes, help and capture/status Next entries.
Validation: mise exec -- task test:cli; task test:terminal; task build; task format:go; task format:check; go test -race ./internal/store; bun scripts/check-docs.mjs all passed (executor). Parent ran mise exec -- task check successfully before and after the review correction (race tests, build, vet, formatting, 142 links, 45-task graph, secrets). Initial parent 200-second and verifier 120-second full-suite invocations timed out; the final 600-second-budget task check completed successfully. LSP diagnostics clean on diff, stream, their tests, store and browser diff files.
One independent verifier pass found a 100000-line cutoff inconsistent with printing the complete patch. Removed the cutoff; full 100250-line safe and raw outputs now match the complete expected SHA256. Focused correction check: mise exec -- go test -race -v ./internal/cli ./internal/terminal -run "TestDiff|TestWriteDiff|TestCaptureAndStatusNextSuggestDiff" passed; full-command allocations 46,504,784 bytes safe and 46,267,488 bytes raw, below 64 MiB. No second general review.
Actual terminal verification: mise exec -- go test ./internal/cli -run "^TestProjectCommandProcessModes$" -count=1 -v passed; 80x24 PTY and pipe with and without NO_COLOR for capture, status, diff and diff --stat. Captured output began "Captured candidate ... (merge base) against base ... (commit)"; status "Checkout · base ... → candidate ..."; pipe diff began "diff --git a/app/config.go b/app/config.go" with separate stderr "origin: captured patch". Fixed SGR only on color PTYs; none with NO_COLOR or pipes. Raw TTY refusal exit 2, hostile control escaping, exact raw bytes and git apply reproduction, arbitrary-pair computed labeling and uncovered inventory tested.
Limits: existing capture/store/computation byte bounds remain; no pager or execution added. Docker execution gates are out of scope and were not authorized/run. Executor timed out at reporting, then resumed solely for handoff; all children now completed. Next: stage/check/commit implementation, refresh claim and main, fast-forward integration, finalize metadata and verified owned-worktree cleanup.

Delivery: implementation commit fac1f7e fast-forwarded into main with authoritative task edits preserved. Staged implementation checks passed. Verified session creation receipt, Worktrunk listing and idle zsh-only workspace; Worktrunk removed the owned after-41-diff worktree and branch, and its post-remove hook closed the exact Herdr workspace (absence verified). Pre-existing worktrees were not adopted or touched. No remaining task blocker or resumable step. Final task metadata is committed separately on main after staged checks; no push requested or performed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered after diff with captured/computed pairs, full streamed safe output, exact non-TTY raw output, stat summaries, coverage diagnostics and documented help/Next guidance. Verified exact git apply reproduction, hostile bytes, real 80-column PTY/pipe color matrix and complete 100250-line output under 64 MiB. One independent verification finding (line truncation) was fixed and regression-tested; final task check passed. Implementation fac1f7e integrated into main; owned worktree, branch and workspace cleaned up.
<!-- SECTION:FINAL_SUMMARY:END -->

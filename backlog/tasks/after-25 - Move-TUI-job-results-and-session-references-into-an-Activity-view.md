---
id: AFTER-25
title: Move TUI job results and session references into an Activity view
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-07 17:27'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-23
  - AFTER-24
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
priority: medium
type: feature
ordinal: 25000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Job completions append `not checked | job completion, not evidence` rows to the evidence list, and every `s` press appends another `session IDs for restart` row. These pseudo-rows pollute the list the reviewer triages. An `i` import shows up only as such a row, and its report appears only after restarting with `--evidence`. The import also binds the report to the candidate given at launch, even after the candidate was switched. There is no record of what happened in the session, no elapsed time for long runs, and `q` during an approved run quits without asking.

Scope, per docs/TUI-DESIGN.md "Activity and session": a fourth tab with a SESSION block (the saved review from AFTER-24, the pair, and the loaded evidence) and a bounded ACTIVITY log with full IDs and sanitized errors in its details; `s` opens it; elapsed time that ticks once per second only while work is active; a `q` confirmation during a run, with Ctrl-C still immediate; and live imports.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 No job completion, failure, cancellation or `s` press adds a row to the evidence list; each becomes an Activity event with time, kind and short IDs, and Enter shows the full IDs and the full sanitized error.
- [x] #2 The Activity log is bounded and states when older events were dropped, and repeated `s` presses grow no list.
- [x] #3 A successful `i` import binds the report to the candidate selected when `i` was pressed and adds it to the loaded evidence, so its cards appear without a restart; at the 32-record limit the report stays stored and Activity shows its ID.
- [x] #4 While a capture, import or run is active, the frame shows elapsed time updated once per second, no timer ticks when nothing is active, and no percentage is shown.
- [x] #5 `q` during an active run asks for confirmation, while Ctrl-C quits immediately and still cancels and joins owned work; PTY tests cover both paths.
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
1. Extend the existing browser frame/key map with a bounded Activity tab and Session details; route job and action notices there without adding evidence rows. 2. Bind imports at keypress to the selected candidate and reload stored evidence within the existing 32-ID cap. 3. Extend the existing run clock to capture/import work and guard q during runs while preserving immediate Ctrl-C cleanup. 4. Add focused state, golden and actual PTY checks at both required sizes/color modes, update docs, and independently verify lifecycle/binding criteria. 5. Commit tested implementation, fast-forward main, finalize task metadata and remove the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation is present in owned after-25-activity worktree, not yet committed. Executor recovered from a 30-minute timeout; task test:terminal, test:views, format:check and focused race-enabled CLI PTY/argument tests passed. Full task test:cli hit a 240-second timeout and remains pending independent verification. Real PTYs cover Activity and run q-confirmation/Ctrl-C cleanup at 80x24 and 120x40 with and without NO_COLOR. Independent verification is now checking import binding, event bounds/idempotency and timer/quit lifecycle before integration.

Delivered implementation 49477cf via fast-forward into main. Independent verifier passed focused AC checks but found the full CLI gate unresolved; parent diagnosed transient PTY status expectations, changed them to persistent completion/indicator waits, and strengthened repeated-s assertions so neither list grows for unchanged references. Final checks passed: mise exec -- task test:cli (186s), task test:terminal, task check:go (build, all Go race tests, vet and gofmt), task test (all Go tests, 141 documentation links, 45 backlog tasks), task check:staged, git diff --check; affected LSP diagnostics clean. Earlier 240s/10m CLI attempts and bounded diagnostic runs failed/timed out; the final complete suites passed after the PTY correction. Actual PTYs covered 80x24 and 120x40 with color and NO_COLOR, plus q/n/y and Ctrl-C cleanup/join matrices. Excerpt: SESSION / loaded no evidence IDs / ACTIVITY / session opened; CLI PTY verified immediate STALE and REPORTED cards and stored report binding after candidate switch. Limits: Activity is session-memory only, 64 events, sanitized detail fields limited to 16 KiB with truncation markers; no new Docker execution proof was claimed. Verified session creation receipt and idle shell pane for the owned checkout; Worktrunk removed after-25-activity and its branch after integration. No remaining implementation blocker or resumable step for this item.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added bounded Activity/session browsing without evidence pollution, keypress-bound live imports, active-work elapsed time and confirmed run quit. Verified by independent acceptance checks, real PTY size/color matrices and complete Go/CLI/terminal suites. Implementation 49477cf integrated into main; owned worktree and branch removed. Task metadata committed separately on main; no push.
<!-- SECTION:FINAL_SUMMARY:END -->

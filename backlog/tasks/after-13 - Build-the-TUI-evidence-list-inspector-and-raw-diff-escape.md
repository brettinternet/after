---
id: AFTER-13
title: 'Build the TUI evidence list, inspector and raw-diff escape'
status: Done
assignee: []
created_date: '2026-10-03 05:42'
updated_date: '2026-10-03 21:53'
labels:
  - poc
  - tui
milestone: m-1
dependencies:
  - AFTER-10
  - AFTER-12
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 13000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Deliver the example-first browsing experience while keeping ordinary code review available. Scope: the selected terminal foundation over the existing headless APIs. Pin/rerun transition integration is AFTER-14; do not duplicate engine logic in update/view methods.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Launching review renders a keyboard-navigable list, exact scenario/input and before/after response/effect inspector with producer, snapshot bindings, timestamps and limits from real engine records.
- [x] #2 Observed, reported, not checked, stale/unknown, incomplete/incomparable and unstable states are distinguishable in text without color; no data is a useful screen, not a crash or green status.
- [x] #3 The d action opens the complete raw diff/inventory including unclassified, unsupported and binary changes; source context is captured content and the selection survives returning to the list.
- [x] #4 Help, keyboard focus, scrolling, cancel/quit and narrow/resize behavior work on an actual terminal; sanitized repository/log text cannot forge trusted badges or emit terminal controls.
- [x] #5 Background capture/import/run progress keeps navigation responsive and exposes cancellation; event tests discard stale UI updates without losing correctly bound stored receipts.
- [x] #6 Automated view/update tests plus a real PTY/manual transcript exercise the actual CLI/engine, not only synthetic screen fixtures; all example wording stays scoped to measured inputs and channels.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a read-only engine-backed browser dataset for selected immutable snapshot pairs and explicit receipt/comparison/report IDs; retain raw inventory independently of evidence failures.
2. Extend the terminal foundation with list/inspector/raw/context/help views, bounded byte paging, safe data prefixes, and request-bound cancellable background loading/jobs. Keep pin/rerun consent actions in AFTER-14.
3. Expose review --tui without changing headless review, with explicit background capture/import actions and no execution on open.
4. Exercise actual capture/import records, failure/unstable labels, stale results, paging and terminal resize/restoration through automated and PTY tests; independently verify trust boundaries.
5. Update docs, run relevant Task checks, commit implementation, integrate main, finalize task and clean up only the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented review --tui with selected immutable snapshot pair and explicit evidence IDs, engine-only state labels, captured inventory/source/raw patch byte paging, scoped receipt/report inspector and safe data prefixes. No project execution on open. Explicit c/i actions reuse headless capture/import, preserve selection, and expose running/cancellation/completion. Interactive pin/rerun remains AFTER-14; actual denied-run lifecycle test verifies persisted request/snapshot bindings survive discarded UI completions.
Validation passed: mise exec -- task test:terminal; mise exec -- task test:cli; mise exec -- task check:go (build, race tests, vet, gofmt); mise exec -- task test (87 local links, 20 tasks). LSP diagnostics clean for browser data/model and CLI adapter.
Real proof passed: configured local Docker CLI/socket, mise exec -- task cli:proof (359s); actual synthetic paired execution and native CLI PTY browsed observed/current/different receipt, frozen scenario/input, inventory and raw diff. Browser inspected eight actual observations: same responses, 12h provider calls 1->2 and 30s control 1->1. Separate actual CLI PTY exercised reported/unknown, help, background capture/import, 32x8 resize, quit and exact termios/alternate-screen/cursor restoration. No payload terminal controls emitted.
Independent read-only reviewer is checking concrete trust/acceptance risks before integration. Implementation worktree after-13-browser is session-owned with Git-local creation receipt; other pre-existing worktrees were not adopted.

Resumed with explicit operator takeover. Independent reviewer completed one general pass (run 020a839c-9bf9-4362-aadf-84f6c099371e), finding one P2 startup-order defect: job completion before initial load hid returned IDs. Fixed by disabling capture/import until initial load completes; TestJobsWaitForInitialLoad covers both actions and inspectable result IDs. No second general review was needed. Post-fix mise exec -- task test:terminal, task test:cli, task check:go and task test all passed; browser model/test LSP clean. The real mise exec -- task cli:proof was rerun successfully (353.29s) before this startup guard, exercising all eight measured observations and native PTY. Implementation 3c768fc fast-forwarded into main; task check:staged passed formatting and secret checks. No private receipts or credentials committed. Retaining the inherited implementation checkout because original-owner inactivity and exact workspace cleanup ownership could not be independently verified within session-root access; other inherited checkouts untouched. No delivery blocker remains; interactive pin/rerun is AFTER-14 and was not started.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered review --tui: engine-backed evidence list/inspector, complete captured inventory/diff/source, explicit cancellable capture/import, safe text and immutable request-bound results. Verified by race tests, CLI/terminal PTY checks, build/vet/format, link/backlog checks, independent defect review and real offline payment proof. Fixed the review finding with a deterministic startup regression test. Implementation commit 3c768fc integrated on main; final task metadata committed separately. Pin/rerun integration remains AFTER-14.
<!-- SECTION:FINAL_SUMMARY:END -->

---
id: AFTER-14
title: Complete the TUI pin-edit-reopen-rerun loop
status: Done
assignee: []
created_date: '2026-10-03 05:42'
updated_date: '2026-10-04 02:24'
labels:
  - poc
  - tui
  - review
  - reviewed
milestone: m-1
dependencies:
  - AFTER-11
  - AFTER-13
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 14000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
This is the first product-level demonstration: browsing, pinning and re-reviewing rather than a prettier test report. Scope: TUI actions over existing pin and runner APIs, explicit snapshot notifications and a real end-to-end test. Snapshot acceptance must be deliberate; avoid a background watcher changing review state under the cursor.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 On a real payment fixture, p records the selected one-request expectation and survives restart; changing retention and explicitly accepting the next captured snapshot reopens it with an exact basis-change reason.
- [x] #2 Before rerun, the new side shows not run or missing current evidence while prior observation/history remains inspectable; no predicted request count appears.
- [x] #3 r previews the exact plan and requires consent; rejection performs no execution, changed plans invalidate consent, and approved execution displays the real captured one-versus-two request witness.
- [x] #4 An untouched control stays inspectable with its actual evidence scope; broad invalidation is explained rather than pretending the control was rerun or is universally unchanged.
- [x] #5 While a run is blocked on a test barrier, another snapshot arrives; list focus and selected pair stay stable until explicitly switched and late results attach only to the originating snapshot.
- [x] #6 An end-to-end PTY check covers inspect, raw diff, pin, snapshot acceptance, denial, authorized rerun, cancellation and reopen after restart; no models/accounts or fabricated results are needed.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend the existing browser with lazy shared-store actions for explicit pin, capture acceptance, immutable plan preview/consent, and paired rerun; retain immutable revision IDs for restart. 2. Render pins, missing-current state, historical artifacts and finite payment observations through existing safe paged views. Keep capture notifications separate from selection and bind asynchronous completions to originating pairs. 3. Add focused state/barrier tests and an opt-in real Docker PTY proof for pin/edit/reopen/denial/rerun/cancellation/restart. 4. Run relevant Task checks and one independent trust-boundary verification, update docs, integrate implementation and finalize task metadata on main.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented explicit TUI finite provider-count pins, capture notification and deliberate original-base selection, conservative missing-current reopening with retained history, paged exact execution preview and one-shot consent, real paired comparison attachment, cancellation, and restart IDs. Shared lazy store writer permits capture/selection while sandbox work is active. Cross-capture pairs intentionally retain complete inventory and both stored sources without claiming a shared patch; this limitation is documented.

Verification: mise exec -- task test:terminal, task test:cli, task check:go, task test, and full task check passed (race tests, build, vet, formatting, 88 links, 20-task graph, secrets). LSP diagnostics clean on modified browser/action/PTY files. With explicitly configured local Docker binary/socket, mise exec -- task tui:proof passed under race detection in 176.66s: real PTY inspect/diff, one-request pin, restart, retention edit, capture acceptance, exact stale reason and missing current result, denial, authorized 1-to-2 witness with 1-to-1 control, cancellation and reopened restart. task cli:proof also passed in 353.37s including native CLI/browser and headless review regressions. No fabricated observed results or accounts.

Independent verifier completed one trust-boundary pass: no consent or snapshot-binding defect found; identified the old rawdiff test expecting rejection instead of the new explicit no-patch inventory fallback. Updated that assertion and reran full checks successfully. Initial PTY iterations exposed the cross-capture diff constraint and test-harness issues (fixture edit omitted required constants; combined jj was not two key events); corrected and real proof passed. Barrier test holds an actual denied runner completion, accepts a separate real capture without moving focus on arrival, and verifies late persisted receipt/request bindings are unchanged. No remaining external blocker; delivery pending implementation commit and fast-forward integration.

Delivered implementation commit b6ab322 to main by fast-forward after rereading the authoritative claim and preserving primary task edits. Final task metadata is committed separately on main. No push or PR. Remaining limitations: explicit immutable revision IDs for restart; original-base cross-capture inventory/source fallback when no shared patch exists. No further work started.

Review 2026: Found TUI Actions.Attach appended pin revisions before checking the evidence limit, then returned the old selection on limit error, orphaning the new revisions from the session; it also skipped corrupt pins silently. Limit is now checked first and ErrCorrupt fails like Select. Limitation retained: a run finishing while another TUI action is busy is kept in session results but not auto-attached. task check:go PASS; task tui:proof PASS (204s).
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Completed the real TUI pin-edit-reopen-rerun loop in b6ab322, integrated on main. Real race-enabled Docker PTY proof and native CLI proof passed; full task check passed. Independent verifier found an outdated rawdiff assertion, corrected and covered by the passing suite. All six acceptance criteria verified; no external blocker.
<!-- SECTION:FINAL_SUMMARY:END -->

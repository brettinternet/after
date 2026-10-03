---
id: AFTER-14
title: Complete the TUI pin-edit-reopen-rerun loop
status: To Do
assignee: []
created_date: '2026-10-03 05:42'
labels:
  - poc
  - tui
  - review
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
- [ ] #1 On a real payment fixture, p records the selected one-request expectation and survives restart; changing retention and explicitly accepting the next captured snapshot reopens it with an exact basis-change reason.
- [ ] #2 Before rerun, the new side shows not run or missing current evidence while prior observation/history remains inspectable; no predicted request count appears.
- [ ] #3 r previews the exact plan and requires consent; rejection performs no execution, changed plans invalidate consent, and approved execution displays the real captured one-versus-two request witness.
- [ ] #4 An untouched control stays inspectable with its actual evidence scope; broad invalidation is explained rather than pretending the control was rerun or is universally unchanged.
- [ ] #5 While a run is blocked on a test barrier, another snapshot arrives; list focus and selected pair stay stable until explicitly switched and late results attach only to the originating snapshot.
- [ ] #6 An end-to-end PTY check covers inspect, raw diff, pin, snapshot acceptance, denial, authorized rerun, cancellation and reopen after restart; no models/accounts or fabricated results are needed.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

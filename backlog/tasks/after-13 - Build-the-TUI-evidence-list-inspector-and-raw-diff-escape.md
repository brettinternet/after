---
id: AFTER-13
title: 'Build the TUI evidence list, inspector and raw-diff escape'
status: To Do
assignee: []
created_date: '2026-10-03 05:42'
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
- [ ] #1 Launching review renders a keyboard-navigable list, exact scenario/input and before/after response/effect inspector with producer, snapshot bindings, timestamps and limits from real engine records.
- [ ] #2 Observed, reported, not checked, stale/unknown, incomplete/incomparable and unstable states are distinguishable in text without color; no data is a useful screen, not a crash or green status.
- [ ] #3 The d action opens the complete raw diff/inventory including unclassified, unsupported and binary changes; source context is captured content and the selection survives returning to the list.
- [ ] #4 Help, keyboard focus, scrolling, cancel/quit and narrow/resize behavior work on an actual terminal; sanitized repository/log text cannot forge trusted badges or emit terminal controls.
- [ ] #5 Background capture/import/run progress keeps navigation responsive and exposes cancellation; event tests discard stale UI updates without losing correctly bound stored receipts.
- [ ] #6 Automated view/update tests plus a real PTY/manual transcript exercise the actual CLI/engine, not only synthetic screen fixtures; all example wording stays scoped to measured inputs and channels.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

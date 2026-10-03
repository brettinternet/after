---
id: AFTER-12
title: Evaluate and pin a responsive terminal framework
status: To Do
assignee: []
created_date: '2026-10-03 05:42'
labels:
  - poc
  - tui
  - security
milestone: m-1
dependencies:
  - AFTER-5
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: spike
ordinal: 12000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The proposal names Bubble Tea as a candidate, not a settled dependency. Scope: a bounded executable evaluation before committing the full UI, including terminal injection and asynchronous rendering risks. Do not spend the spike on theme polish or browser parity.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A small Go terminal experiment exercises Bubble Tea's model/update/view flow against captured large diffs and asynchronous fake jobs; a short decision record pins the tested dependency or documents a simpler selected fallback.
- [ ] #2 Rendering and event updates do not run Git or block on jobs; measured input/resize/quit responsiveness and memory on a reproducible large-diff workload meet a stated usable budget or trigger an explicit documented fallback.
- [ ] #3 All untrusted strings pass a tested terminal-safe rendering boundary; ESC/CSI/OSC clipboard/hyperlink payloads cannot execute, change titles, forge trusted styling or leak into restored terminal state.
- [ ] #4 Viewport/selection tests cover Unicode width, combining characters, tabs, long lines, 40-column layouts, resize and empty/no-evidence views; no full-patch rebuild on every keypress.
- [ ] #5 Core tests run without an interactive terminal; at least one actual PTY smoke check covers cancellation/quit and terminal restoration, including injected errors.
- [ ] #6 Experiment code is retained only as the minimal reusable view/sanitization foundation needed by AFTER-13; no second demo app or general UI framework remains.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

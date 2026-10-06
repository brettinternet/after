---
id: AFTER-25
title: Move TUI job results and session references into an Activity view
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
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

Scope, per docs/TUI-DESIGN.md "Activity and session": a fourth tab with a SESSION block (the session file and resume command from AFTER-24, the pair, and evidence IDs) and a bounded ACTIVITY log with full IDs and sanitized errors in its details; `s` opens it; elapsed time that ticks once per second only while work is active; a `q` confirmation during a run, with Ctrl-C still immediate; and live imports.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 No job completion, failure, cancellation or `s` press adds a row to the evidence list; each becomes an Activity event with time, kind and short IDs, and Enter shows the full IDs and the full sanitized error.
- [ ] #2 The Activity log is bounded and states when older events were dropped, and repeated `s` presses grow no list.
- [ ] #3 A successful `i` import binds the report to the candidate selected when `i` was pressed and adds its ID to the selection and session file, so its cards appear without a restart; at the 32-ID limit the report stays stored and Activity shows its ID.
- [ ] #4 While a capture, import or run is active, the frame shows elapsed time updated once per second, no timer ticks when nothing is active, and no percentage is shown.
- [ ] #5 `q` during an active run asks for confirmation, while Ctrl-C quits immediately and still cancels and joins owned work; PTY tests cover both paths.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

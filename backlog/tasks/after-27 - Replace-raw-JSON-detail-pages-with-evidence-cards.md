---
id: AFTER-27
title: Replace raw JSON detail pages with evidence cards
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-26
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/RUNNER.md
  - docs/COMPARISON.md
  - docs/GO-REPORTS.md
  - docs/REVIEW.md
priority: high
type: enhancement
ordinal: 27000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Enter on a row opens `json.MarshalIndent` dumps of receipts, pin views and inventory entries, followed by up to 20 sections named like `artifact: base/43200/0/observation`. The facts a reviewer needs are buried in records: base versus candidate counts, responses, the witness, the producer, run times and scope. Long scope text is a single clipped line.

Scope, per docs/TUI-DESIGN.md "Detail": a Card section first for every row type in the design's table, built by deterministic templates over strictly decoded records (receipt, comparison report, `runner.Observation`, `runner.Sample`, scenario and frozen input, pin view, report card), with a raw fallback and limitation; sections as lists of parts with trusted divider titles; the Witnesses, Artifacts, Plan and IDs channel mapping; a bounded prose wrap helper in internal/terminal; and, at widths of 110 or more, the selected Overview row's Card in a preview pane. Record values always render as data.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Payment case, receipt, pin, report card and unavailable-ID rows open a detail whose first section is a Card matching the design: base and candidate columns for counts and responses, plus input, witness, samples, run times and scope for cases; expectation, scope, review target, reopen reason, current result and history for pins; outcome, producer, binding, events and output for report cards.
- [ ] #2 A record that fails strict decoding or has an unexpected shape shows its raw content with a stated limitation instead of a partial card.
- [ ] #3 Detail sections are lists of parts whose divider titles come only from typed fields; channel names outside the runner pattern appear escaped under Other artifacts; a test proves that every artifact referenced by the receipt, comparison, scenario and pin history is reachable from some detail.
- [ ] #4 Long prose wraps inside the card through a terminal wrap helper that uses the same sanitizing and grapheme widths as `Line`, with hostile-content tests.
- [ ] #5 At widths of 110 or more, Overview shows the selected row's Card in a preview pane that loads off the event loop and ignores stale results; golden views cover each card type.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

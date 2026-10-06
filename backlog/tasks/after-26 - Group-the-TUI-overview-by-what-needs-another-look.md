---
id: AFTER-26
title: Group the TUI overview by what needs another look
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-25
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/README.md
  - docs/behavior-and-evidence.md
priority: high
type: enhancement
ordinal: 26000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The landing list shows evidence in launch order, and only the selected row shows its state. Reopened pins, differing observations, reported failures, history from earlier snapshots and equal controls all look alike, so the reviewer must open each row to learn what needs attention. With no evidence, the landing screen is two lines and hides the change behind `d`. The product's contact sheet (docs/README.md "The contact sheet"; docs/behavior-and-evidence.md section 10) puts "needs another look" first.

Scope, per docs/TUI-DESIGN.md "Overview" and "Next line": groups in the design's order with counts and collapsible headers; report lines that tell same-named reports apart; the CHANGES line with potential oracles; the no-evidence variant that lists every path; and the Next line rules. Grouping and the Next line derive only from typed engine fields, through tested mappings. Until AFTER-30 adds `a`, the accept rule points to `after review --accept`.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Overview groups rows as NEEDS ANOTHER LOOK, PINNED EXPECTATIONS, AGREES, REPORTED, EARLIER SNAPSHOTS (collapsed by default) and OTHER, in that order, with counts and without empty groups, and Enter on a header collapses or expands it.
- [ ] #2 On the payment loop after the regression rerun, the 12h case and the reopened pin appear under NEEDS ANOTHER LOOK, the 30s control under AGREES, and the first run's rows under EARLIER SNAPSHOTS, labelled with the candidate they ran on.
- [ ] #3 Each imported report has a line with its short ID, binding, import time and outcome counts, followed by the producer string as data, so two reports with identical test names are distinguishable; reported passes are never styled as observed results.
- [ ] #4 The CHANGES line counts paths, hunks and potential oracles (from rawdiff, with every hunk unclassified) and lists potential-oracle paths; with no evidence, Overview states NOT CHECKED and lists every inventory path, including excluded and unknown ones.
- [ ] #5 The Next line follows the design's rule table, a table-driven test covers every rule, and golden views cover the raw review, test report and payment loop overviews.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

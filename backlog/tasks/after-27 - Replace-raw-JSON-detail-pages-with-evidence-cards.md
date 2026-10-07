---
id: AFTER-27
title: Replace raw JSON detail pages with evidence cards
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-07 21:37'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-26
  - AFTER-35
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

Scope, per docs/TUI-DESIGN.md "Detail": a Card section first for every row type in the design's table, built by deterministic templates over strictly decoded records (receipt, comparison report, `runner.Observation`, `runner.Sample`, scenario and frozen input, pin view, report card), with a raw fallback and limitation; sections as lists of parts with trusted divider titles; the Witnesses, Artifacts, Plan and IDs channel mapping; a bounded prose wrap helper in internal/terminal; and, at widths of 110 or more, the selected Overview row's Card in a preview pane. Record values always render as data. The CLI prints the same Card from `after inspect ID` and `after pin PIN` (docs/CLI-DESIGN.md "inspect").
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Payment case, receipt, pin, report card and unavailable-ID rows open a detail whose first section is a Card matching the design: base and candidate columns for counts and responses, plus input, witness, samples, run times and scope for cases; expectation, scope, review target, reopen reason, current result and history for pins; outcome, producer, binding, events and output for report cards.
- [x] #2 A record that fails strict decoding or has an unexpected shape shows its raw content with a stated limitation instead of a partial card.
- [x] #3 Detail sections are lists of parts whose divider titles come only from typed fields; channel names outside the runner pattern appear escaped under Other artifacts; a test proves that every artifact referenced by the receipt, comparison, scenario and pin history is reachable from some detail.
- [x] #4 Long prose wraps inside the card through a terminal wrap helper that uses the same sanitizing and grapheme widths as `Line`, with hostile-content tests.
- [x] #5 At widths of 110 or more, Overview shows the selected row's Card in a preview pane that loads off the event loop and ignores stale results; golden views cover each card type.
- [x] #6 `after inspect ID` prints the same Card for receipts, comparisons, pin revisions and reports, followed by the IDs section, and `after pin PIN` prints the pin's Card; CLI golden files cover each card type.
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
1. Extend the existing browser detail/document and CLI readable rendering paths with shared deterministic evidence Cards; retain strict decoding and raw fallback for unsupported shapes. 2. Replace one-blob sections with ordered typed-title parts; map all receipt/comparison/scenario/pin-history artifacts without the current section cap. 3. Add bounded terminal prose wrapping and asynchronous selection-bound Overview previews at 110+ columns. 4. Cover card types, hostile inputs, artifact reachability, stale preview results, CLI goldens, and real PTY 80x24/120x40 color/no-color; update affected docs. 5. Run relevant Task checks and independent acceptance verification, fix concrete scoped defects, commit implementation, fast-forward main, finalize authoritative task state, and remove the verified owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented shared browser/CLI Cards with strict shape validation and raw fallback, ordered typed-title parts with artifact reachability, bounded terminal.Wrap, and asynchronous selection-bound wide previews. Independent verifier passed all six criteria using task test:views, task test:terminal, and task test:cli plus isolated PTY/Card reruns. Executor task check passed (format, Go race tests, 141 local links, 45 backlog tasks, build, vet, gofmt, secrets); parent LSP cards.go diagnostics clean. Real PTY 80x24 and 120x40 passed with color and NO_COLOR: 80 columns remained one pane; 120 columns showed NEEDS ANOTHER LOOK 1 beside Payment case and selected 12h same-key retry. Validation limitations: fixed-width terminal golden frames contain intentional trailing padding; an additional combined non-race package run encountered concurrent PTY store-writer/timing errors, while prescribed Task targets and isolated rerun passed. Direct nested observation/sample corruption and dedicated >20-artifact fixtures are coverage gaps; strict validators and uncapped reachability loops were independently inspected. No in-scope blocker found. Delivery pending staged checks, implementation commit, main integration, metadata commit, and owned-worktree cleanup.

Delivery: implementation commit 4650727 fast-forwarded into main; mise exec -- task check:staged passed before implementation commit. Verified session receipt, clean Worktrunk checkout and no active children or matching Herdr panes/workspace before cleanup. Worktrunk removed the owned after-27-cards checkout and branch; no matching workspace remains. Pre-existing worktrees were not adopted or modified. Final task metadata is committed separately on main. No push performed; no remaining task blocker or resumable implementation step.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered shared evidence Cards, strict raw fallbacks, complete artifact sections, safe wrapping and wide asynchronous previews to CLI/TUI in 4650727 on main. Full task check, focused CLI/terminal/golden tests and independent verification passed; real PTY matrix covered 80x24/120x40 with and without NO_COLOR. Coverage and concurrent-PTY limitations are recorded in notes. Owned worktree and branch cleaned up.
<!-- SECTION:FINAL_SUMMARY:END -->

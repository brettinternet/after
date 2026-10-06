---
id: AFTER-22
title: 'Add the TUI theme, evidence badges and golden view tests'
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-21
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/TERMINAL.md
  - docs/COMPARISON.md
priority: high
type: enhancement
ordinal: 22000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Rows have no visual hierarchy. There is one text style, and only the selected row shows state, as `STATE observed | current | completed | equal | complete | report=none`; names are Go-quoted. Payment case rows also inherit the receipt-level comparison outcome (internal/browser/data.go copies `e.Label` to every case row), so after the regression rerun the unchanged 30s control (1 → 1) reads `different`. There are no view-level regression tests, so later TUI tasks have nothing to lock screens down with.

Scope, per docs/TUI-DESIGN.md "Visual vocabulary", "Code boundaries", "Rendering untrusted content safely" and "Testing and verification":
- a fixed-SGR theme in internal/terminal, on unless NO_COLOR is non-empty or TERM=dumb, where styling is only possible through a function that sanitizes and clips first; no lipgloss or Bubbles;
- one badge-mapping function over typed engine fields, and a row layout (badge column, name, summary, trailing kind/freshness) for the existing evidence list and inventory;
- a per-case outcome helper in internal/compare, derived from the case's own `paired` and `repetition` witnesses, used by case rows;
- readable row text such as `12h same-key retry · provider requests 1 → 2 · responses same`, and report cards as `example.com/cart (package)` using the card's scope;
- golden view tests with deterministic inputs (fixed Git dates, fixed record timestamps, injected clock and UTC zone) and a test-only `-update` flag.

Grouping, the Next line and frame chrome are later tasks (AFTER-23, AFTER-26). docs/TERMINAL.md currently says no theme is introduced; record the superseding decision there.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Every evidence row shows a badge from the design's table, derived only from typed engine fields by one tested function that covers every precedence rule for observations, reports and pins; badges, names and trailing columns read correctly with color off.
- [ ] #2 Payment case rows take their outcome from their own witnesses: in a receipt whose 12h case differs and whose 30s case matches, the rows read `[DIFFERENT]` and `[EQUAL]`; a differing repetition witness makes its case `[UNSTABLE]`; incomplete or incomparable comparisons stay `[INCOMPLETE]`.
- [ ] #3 Rows read as plain text without Go quoting: delays use `12h`/`30s`, counts use `1 → 2` or per-repetition lists when repetitions disagree, and report cards use their package or test name and scope.
- [ ] #4 With NO_COLOR non-empty or TERM=dumb, rendered frames contain no SGR sequences; in color mode a test asserts that every escape sequence in a frame is a theme SGR, and hostile repository text cannot add or change styling.
- [ ] #5 Golden view tests render the evidence list and inventory at 120×40, 80×24 and 40×12 from deterministic synthetic stores without Docker, compare byte for byte, and regenerate only with an explicit `-update` flag.
- [ ] #6 docs/TERMINAL.md records the theme decision (fixed SGR, no lipgloss or Bubbles), and docs/TUI.md describes the badges.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

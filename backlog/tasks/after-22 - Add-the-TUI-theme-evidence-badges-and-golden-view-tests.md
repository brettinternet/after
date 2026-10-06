---
id: AFTER-22
title: 'Add the TUI theme, evidence badges and golden view tests'
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-06 23:12'
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
- [x] #1 Every evidence row shows a badge from the design's table, derived only from typed engine fields by one tested function that covers every precedence rule for observations, reports and pins; badges, names and trailing columns read correctly with color off.
- [x] #2 Payment case rows take their outcome from their own witnesses: in a receipt whose 12h case differs and whose 30s case matches, the rows read `[DIFFERENT]` and `[EQUAL]`; a differing repetition witness makes its case `[UNSTABLE]`; incomplete or incomparable comparisons stay `[INCOMPLETE]`.
- [x] #3 Rows read as plain text without Go quoting: delays use `12h`/`30s`, counts use `1 → 2` or per-repetition lists when repetitions disagree, and report cards use their package or test name and scope.
- [x] #4 With NO_COLOR non-empty or TERM=dumb, rendered frames contain no SGR sequences; in color mode a test asserts that every escape sequence in a frame is a theme SGR, and hostile repository text cannot add or change styling.
- [x] #5 Golden view tests render the evidence list and inventory at 120×40, 80×24 and 40×12 from deterministic synthetic stores without Docker, compare byte for byte, and regenerate only with an explicit `-update` flag.
- [x] #6 docs/TERMINAL.md records the theme decision (fixed SGR, no lipgloss or Bubbles), and docs/TUI.md describes the badges.
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
1. Add a fixed-SGR terminal theme that always sanitizes/clips before styling, and typed browser badge/row rendering with explicit precedence and readable names/counts/trailing evidence state.
2. Derive payment case outcomes from their own comparison witnesses in internal/compare; preserve incomplete/incomparable and history semantics.
3. Add deterministic synthetic-store golden views, hostile/color/precedence tests, and real PTY coverage at both required sizes and color modes; update screen expectations and docs.
4. Run focused Task checks and one independent verifier for rendering safety and case/badge correctness, fix concrete findings, commit implementation, fast-forward main, finalize authoritative task metadata, and remove only the receipt-owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Delivered and fast-forwarded to main as 7783eda. Fixed-SGR theme sanitizes/clips before styling; typed badge precedence and per-case paired/repetition witnesses replace receipt-wide labels. Readable delays/counts/package scope, trailing kind/freshness and six deterministic synthetic-store golden views added. No production records or execution evidence synthesized.
Verification: mise exec -- task format:go, test:terminal, test:views, test:cli, check:go, format, test and check:staged passed. Initial goldens were explicitly generated with task test:views -- -update and reviewed. An earlier check:go process exited 137; the rerun completed successfully including full race tests, build, vet and formatting. Post-integration task test:views and task test passed on main (140 local links, 45 tasks). LSP diagnostics clean for browser data/model/rows, compare case helper and terminal theme. Large captured-diff budget passed: 100250 lines, 47ms preparation, 11.6MB allocation, sub-millisecond maximum input/view and resize/view on local darwin/arm64 with warm OS cache.
Real native PTY test passed at 80x24 and 120x40 with NO_COLOR unset and set; asserts REPORTED and NOT CHECKED badges, package scope, trailing reported pass, actual color mode, hostile controls and terminal restoration. Synthetic PTY excerpt: [REPORTED] example.com/cart (package) · reported · pass; [NOT CHECKED] app/main.go; 1 │ package main; 8 │ STATE observed | forged (behind trusted gutter).
One independent verifier pass: PASS, no concrete defects. It independently ran task test:terminal, test:views and test:cli. Its sandbox could read only the worktree task copy; the parent refreshed authoritative primary task state before integration and confirmed unchanged intent/criteria. Verification output persisted by the harness; child did not change implementation or task state.
Docker settings AFTER_DOCKER_BINARY/HOST were absent; Docker-gated proofs have updated screen assertions but were not run. No new real Docker execution claim is made. This is a rendering/case-selection change with executable synthetic and PTY acceptance evidence.
Receipt-owned after-22-theme worktree and branch removed through Worktrunk after integration and verification; associated Herdr workspace verified gone after its post-remove hook. Pre-existing unrelated worktrees were not adopted or removed. All task state maintained in primary and final metadata committed separately. No remaining blocker or resumable step for this item; no push performed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added safe fixed-SGR styling, typed evidence badges and readable rows; payment controls now use their own case witnesses. Six golden views, hostile/color/precedence tests and four native PTY configurations pass. Full Go checks and independent verification passed. Delivered on main in 7783eda; owned worktree, branch and workspace cleaned.
<!-- SECTION:FINAL_SUMMARY:END -->

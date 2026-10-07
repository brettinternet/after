---
id: AFTER-23
title: Add the TUI frame and one key map for hints and help
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-07 00:32'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-22
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/DEMO.md
priority: high
type: enhancement
ordinal: 23000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The header says only `AFTER review | <screen>`: no project, snapshots, pending capture or running job. The footer shows the same five keys on every screen, while `p`, `r`, `a`, `y` and `n` appear only in a static help list that ignores context. `a` means "accept snapshot", although everywhere else in AFTER (headless `review --accept`, the `accepted` decision) accepting means accepting a pin.

Scope, per docs/TUI-DESIGN.md "Frame" and "Key map": a header with the sanitized project name, short snapshot IDs and source words, plus pending-capture and running indicators; a tab bar for the views that exist (Overview = the current evidence list, Changes = the inventory, Diff = the raw patch; Activity arrives in AFTER-25); breadcrumbs on drill-in screens; a next/status line; context-sensitive key hints; and a help overlay. One key-map table drives dispatch, hints and help; each entry carries contexts, an enabled check that returns a reason, and a hint priority. Move snapshot switching from `a` to `u` (confirmation arrives in AFTER-30). The small-terminal rules apply. Wide split panes arrive with AFTER-27 and AFTER-28.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 The header shows the sanitized project name, both short snapshot IDs with their source (`commit <hash>`, `working tree`, `staged`, `merge base <hash>`), `new capture <id> · u` while a capture is pending, and elapsed time while a run is active.
- [x] #2 Number keys and Tab/Shift+Tab switch between the views that exist, the active tab is marked in both color and no-color modes, and drill-in screens show a breadcrumb with the row's badge instead of the tab bar.
- [x] #3 Key hints list only actions enabled on the current screen, ordered by priority and trimmed to the width; the `?` overlay is generated from the same table, grouped, and lists unavailable actions with their reason; a test asserts that every dispatched key has a key-map entry.
- [x] #4 `u` uses a pending capture with the original base (the old `a` behavior) and `a` no longer switches snapshots; docs/TUI.md, docs/DEMO.md, examples/README.md, the examples/tui hints and the PTY tests use the new keys.
- [x] #5 Under 12 rows the tab bar hides, under 7 rows the status line hides and hints shrink to `? help · q quit`, and under 60 columns labels shorten; golden views at 120×40, 80×24 and 40×12 cover these layouts, and quit still works at 1×1.
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
1. Extend the existing browser model and terminal theme with responsive frame chrome, snapshot source labels, pending/job indicators and breadcrumbs. Preserve bounded safe text rendering and asynchronous job ownership.
2. Replace separate dispatch/help/hints with one contextual key table; add top-level tabs and move snapshot switching to u without implementing later-task mutations/search/views.
3. Update focused model/golden/PTY tests and required documentation/examples; exercise real PTYs at 80x24 and 120x40 with and without NO_COLOR.
4. Run focused Task targets and Go checks, then one independent verifier focused on key-map consistency, terminal safety/layout and snapshot-switch invariants. Fix concrete findings, commit and fast-forward main, finalize task metadata, and remove only the verified owned checkout.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation is present in receipt-owned after-23-frame checkout. Executor timed out, then its resumed run terminated; partial diff was inspected and operator explicitly approved direct parent takeover. Parent validation passed: mise exec -- task test:terminal test:views test:cli check:go; mise exec -- task test; git diff --check. Model/key-map LSP diagnostics clean. Native PTY suite passes 80x24 and 120x40 with color and NO_COLOR; excerpt: AFTER · 001 · base 2d13fdbf (commit 4117af3) → candidate 8ae79385 (working tree); [1 Overview] 2 Changes 4 3 Diff. Nine deterministic overview/inventory/patch goldens pass. Independent verifier running; not yet committed or integrated. Docker-backed execution gate not run; this task changes presentation/navigation, not execution proof.

Final delivery: 1271490 fast-forwarded to main after refreshing the authoritative claim. Independent verifier PASS: all five criteria, shared key-map/context checks, snapshot base preservation, responsive layouts, terminal safety; independently ran task test:terminal test:cli and task test:views test:go. Post-integration task test:views test passed (full race suite, 140 links, 45 tasks). Initial staged formatting check caught docs/TUI.md; task format fixed it, then task check:staged and commit hooks passed with no secret leaks. Required real-terminal evidence is the native PTY suite and excerpt above, not a claimed human visual study. Owned worktree and branch removed through Worktrunk after verifying creation receipt, clean state, inactive children and shell-only Herdr pane; unrelated existing worktrees preserved. No remaining blocker or resumable step; no push performed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added responsive snapshot/job header, tabs and breadcrumbs, contextual hints/help from one key map, and u for using captures. Nine golden views, four real PTY configurations, focused/full Go checks and independent verification pass. Delivered on main as 1271490; owned worktree and branch cleaned.
<!-- SECTION:FINAL_SUMMARY:END -->

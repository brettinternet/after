---
id: AFTER-23
title: Add the TUI frame and one key map for hints and help
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
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
- [ ] #1 The header shows the sanitized project name, both short snapshot IDs with their source (`commit <hash>`, `working tree`, `staged`, `merge base <hash>`), `new capture <id> · u` while a capture is pending, and elapsed time while a run is active.
- [ ] #2 Number keys and Tab/Shift+Tab switch between the views that exist, the active tab is marked in both color and no-color modes, and drill-in screens show a breadcrumb with the row's badge instead of the tab bar.
- [ ] #3 Key hints list only actions enabled on the current screen, ordered by priority and trimmed to the width; the `?` overlay is generated from the same table, grouped, and lists unavailable actions with their reason; a test asserts that every dispatched key has a key-map entry.
- [ ] #4 `u` uses a pending capture with the original base (the old `a` behavior) and `a` no longer switches snapshots; docs/TUI.md, docs/DEMO.md, examples/README.md, the examples/tui hints and the PTY tests use the new keys.
- [ ] #5 Under 12 rows the tab bar hides, under 7 rows the status line hides and hints shrink to `? help · q quit`, and under 60 columns labels shorten; golden views at 120×40, 80×24 and 40×12 cover these layouts, and quit still works at 1×1.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

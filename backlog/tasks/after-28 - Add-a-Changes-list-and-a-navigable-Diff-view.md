---
id: AFTER-28
title: Add a Changes list and a navigable Diff view
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
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/RAW-DIFF.md
priority: high
type: feature
ordinal: 28000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The inventory shows `not checked | modified | binary=false | potential oracle=true` for the selected row only, and names are quoted. The patch is quoted 32-byte chunks with no color, line numbers, file or hunk navigation, or summary of binary and mode-only changes. The proposal's code microscope (docs/README.md) calls for a normal, high-quality diff one key away.

Scope, per docs/TUI-DESIGN.md "Changes" and "Diff": a Changes tab grouped as potential oracles, changed and unknown, with A/D/M/? letters, word flags, recorded limitations verbatim and +/− counts; a path detail with Diff, Base source, Candidate source and Inventory record sections; a Diff tab over the captured patch with an old/new line-number gutter, colored `+`/`-`/`@@` lines, a sticky file header, `]`/`[` file and `}`/`{` hunk navigation using the rawdiff index, and summary dividers for binary, mode-only, added and deleted files; and a Changes preview pane at 110 columns or more. Pairs without a shared captured patch keep today's honest fallback until AFTER-32.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Changes lists every inventory entry, including excluded and unsupported paths, grouped as POTENTIAL ORACLES, CHANGED and UNKNOWN in inventory order, with letters, `oracle`, `binary` and `mode <old> → <new>` flags, recorded limitations verbatim and per-path +/− counts from the captured patch.
- [ ] #2 Enter on a path opens Diff (its hunks), Base source, Candidate source and Inventory record sections, and a missing side is stated, never blank.
- [ ] #3 Diff shows the complete captured patch unchanged after a gutter with old and new line numbers on hunk lines, a sticky header naming the current file, its position and flags, and `]`/`[`/`}`/`{` navigation; opening Diff from a path lands on that file.
- [ ] #4 Binary, mode-only, added and deleted files have trusted summary dividers, their raw patch lines stay visible, and `+` and `-` keep their meaning with color off.
- [ ] #5 For a pair without a shared captured patch, Diff and Changes say so and still list the complete inventory and both sources, and a test proves that no patch from another pair is ever shown.
- [ ] #6 Navigation and rendering on the 100,000-line captured diff stay within the docs/TERMINAL.md per-event budgets, and golden views cover Changes and Diff for the raw review fixture.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

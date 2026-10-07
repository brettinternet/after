---
id: AFTER-28
title: Add a Changes list and a navigable Diff view
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-07 01:45'
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
- [x] #1 Changes lists every inventory entry, including excluded and unsupported paths, grouped as POTENTIAL ORACLES, CHANGED and UNKNOWN in inventory order, with letters, `oracle`, `binary` and `mode <old> → <new>` flags, recorded limitations verbatim and per-path +/− counts from the captured patch.
- [x] #2 Enter on a path opens Diff (its hunks), Base source, Candidate source and Inventory record sections, and a missing side is stated, never blank.
- [x] #3 Diff shows the complete captured patch unchanged after a gutter with old and new line numbers on hunk lines, a sticky header naming the current file, its position and flags, and `]`/`[`/`}`/`{` navigation; opening Diff from a path lands on that file.
- [x] #4 Binary, mode-only, added and deleted files have trusted summary dividers, their raw patch lines stay visible, and `+` and `-` keep their meaning with color off.
- [x] #5 For a pair without a shared captured patch, Diff and Changes say so and still list the complete inventory and both sources, and a test proves that no patch from another pair is ever shown.
- [x] #6 Navigation and rendering on the 100,000-line captured diff stay within the docs/TERMINAL.md per-event budgets, and golden views cover Changes and Diff for the raw review fixture.
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
1. Extend the existing rawdiff index and browser document/list adapters for grouped complete inventory, per-path statistics, source sections and indexed file/hunk navigation. Keep immutable pair binding and safe bounded rendering; no computed cross-pair patch (AFTER-32).
2. Add responsive Changes preview and Diff gutters, trusted summaries and sticky headers through the existing theme/key map. Preserve raw bytes and honest unsupported/missing states.
3. Add focused fixture/golden, hostile-content, cross-pair, 100000-line budget and real PTY checks at 80x24 and 120x40 in color and NO_COLOR; update affected docs.
4. Independently verify acceptance and terminal-safety/performance risks once, fix concrete defects and rerun affected checks. Commit implementation, fast-forward main, finalize authoritative task metadata and clean only the receipt-owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation present in receipt-owned .worktrees/after-28-changes at base ed3f3af; initial executor terminated, same child resumed successfully. Focused task test:terminal, test:views, test:cli passed, including real PTYs at 80x24 and 120x40 color/NO_COLOR. PTY excerpt: CHANGED 2; > M app/config.go +1 · −4; app/config.go · file 1 of 2 · captured patch · modified. Browser 100250-row/50-file benchmark: max input+view 1.96ms, resize+view 1.80ms, navigation+view 4.90ms, 50465 bytes/event. Full check:go had one terminal timing sample 113ms over 100ms under suite load; isolated gate passed (<1ms); independent verifier is investigating/rechecking without weakening budgets. changes.go and diff.go LSP diagnostics clean; diff --check passes. Not yet committed or integrated. Parent corrected design status text to avoid claiming unimplemented AFTER-24–27.

Final delivery: implementation 83501ce fast-forwarded to main. Independent verifier PASS on all six criteria, with no scoped defects; independently passed mise exec -- task check:go, task test:terminal, task test:views, task test:cli and uncached go test -race -count=1 ./... . Earlier timing outlier did not reproduce; budgets remain unchanged and host-load sensitive. Parent post-integration mise exec -- task test passed full race suite, 140 local links and 45-task integrity. Staged formatting and secret checks passed before implementation commit. No Docker execution gate run because this task changes captured review presentation/navigation, not execution. Verified session receipt, clean integrated checkout, inactive children and shell-only workspace before Worktrunk removal; owned worktree and branch removed and exact Herdr workspace disappearance confirmed. Pre-existing unrelated worktrees preserved. No blocker or remaining resumable step; no push.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added grouped complete Changes inventory, source sections and wide preview, plus captured Diff gutters, typed summaries and indexed file/hunk navigation. All six criteria independently verified with goldens, real PTYs and 100250-line budget; full Go checks pass. Delivered on main as 83501ce; owned worktree, branch and workspace cleaned.
<!-- SECTION:FINAL_SUMMARY:END -->

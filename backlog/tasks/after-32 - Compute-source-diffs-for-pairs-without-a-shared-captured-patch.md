---
id: AFTER-32
title: Compute source diffs for pairs without a shared captured patch
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-28
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/RAW-DIFF.md
  - docs/TERMINAL.md
priority: high
type: feature
ordinal: 32000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A captured patch exists only for a commit base and the candidate captured with it (`rawdiff.CapturedPair`). Snapshot records embed their capture's diff identity, so every switch to a new capture (`u` with the original base, or the follow-up comparison in AFTER-33) pairs snapshots from different captures. The Diff view then only says "no shared captured patch", so the reviewer loses the diff right after the first edit, which is exactly when re-review matters. Both snapshots' stored file sets are available, so AFTER can compute a diff without claiming it is Git's patch.

Scope, per docs/TUI-DESIGN.md "Computed diffs": a deterministic, bounded, pure-Go line diff in internal/rawdiff (no subprocess, no new module) producing Git-style unified output with 3 lines of context, in inventory order; the binary, mode-only, oversized and unknown-path handling from the design; and the TUI Diff view, Changes counts and Overview CHANGES line using it for pairs without a shared patch, always labelled `computed from captured sources — not Git's patch`. Captured patches remain the only source of hunk IDs and `rawdiff.Count` results. Headless CLI exposure is out of scope.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 For any two stored snapshots without a shared captured patch, the Diff view shows a unified diff computed from their stored files in inventory order, labelled `computed from captured sources — not Git's patch` in the sticky header, the Changes counts and the Overview CHANGES line.
- [ ] #2 A property or fuzz test proves that applying the computed diff to the base file yields the candidate file, and identical inputs always produce identical output.
- [ ] #3 Binary files (a NUL byte in the first 8,000 bytes of either side) show `binary`, mode-only changes show `mode <old> → <new>`, files or totals over the documented bounds show `too large to diff here — open both sources`, and unknown paths keep their limitation without a diff.
- [ ] #4 The diff is computed off the event loop within the docs/TERMINAL.md budgets with no subprocess or new module, and captured pairs still show the captured patch with an unchanged hunk index.
- [ ] #5 On the payment loop, after `u` keeps the original base, Diff shows the retention edit in app/config.go as a computed diff, and docs/RAW-DIFF.md and docs/TUI.md document computed diffs and their bounds.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

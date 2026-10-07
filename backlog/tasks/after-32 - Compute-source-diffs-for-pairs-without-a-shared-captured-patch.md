---
id: AFTER-32
title: Compute source diffs for pairs without a shared captured patch
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-07 03:19'
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
- [x] #1 For any two stored snapshots without a shared captured patch, the Diff view shows a unified diff computed from their stored files in inventory order, labelled `computed from captured sources — not Git's patch` in the sticky header, the Changes counts and the Overview CHANGES line.
- [x] #2 A property or fuzz test proves that applying the computed diff to the base file yields the candidate file, and identical inputs always produce identical output.
- [x] #3 Binary files (a NUL byte in the first 8,000 bytes of either side) show `binary`, mode-only changes show `mode <old> → <new>`, files or totals over the documented bounds show `too large to diff here — open both sources`, and unknown paths keep their limitation without a diff.
- [x] #4 The diff is computed off the event loop within the docs/TERMINAL.md budgets with no subprocess or new module, and captured pairs still show the captured patch with an unchanged hunk index.
- [x] #5 On the payment loop, after `u` keeps the original base, Diff shows the retention edit in app/config.go as a computed diff, and docs/RAW-DIFF.md and docs/TUI.md document computed diffs and their bounds.
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
1. Add a bounded deterministic pure-Go computed-source diff API alongside rawdiff captured-patch access, preserving captured hunk identities/counts and explicit unknown/binary/mode/oversize limitations. 2. Prepare computed output in browser background Load and reuse the existing indexed Diff/Changes viewer with explicit origin labels and counts. 3. Add apply/determinism property tests, budget and capture-switch coverage, real PTY checks at both sizes/color settings, and update RAW-DIFF/TUI documentation. 4. Run focused Task checks and one independent acceptance verification, fix concrete scoped defects, commit in the owned worktree, fast-forward main preserving task metadata, finalize and clean up.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented in baadf5d (Compute diffs from captured sources), fast-forwarded into main preserving authoritative task edits. Session-owned worktree/branch removed with Worktrunk after verifying creation receipt and idle shell-only Herdr pane.
Executor first timed out during full check; the same child resumed preserved changes and completed checks. Independent verifier PASS, no scoped defects; no second general review.
Passed: mise exec -- task test:computed; mise exec -- task test:terminal; mise exec -- task check:go; mise exec -- task check; git diff --check. Full check includes formatting, race tests, 140 local links, backlog integrity (45 tasks), build, vet, gofmt and Gitleaks. Active fuzz: go test ./internal/rawdiff -run ^$ -fuzz FuzzUnifiedDiffAppliesAndIsDeterministic -fuzztime 15s passed 4,636,928 executions. Parent LSP diagnostics clean for computed.go and browser/data.go. Staged check passed before implementation commit.
Real payment app/config.go capture/use PTYs passed at 80x24 and 120x40, color and NO_COLOR, with original base retained after u. PTY excerpt: computed from captured sources — not Git's patch · 1 paths · +1 −1 lines; > M app/config.go +1 · −1. Sticky header: computed from captured sources — not Git's patch · app/config.go · file 1 of 1 · modified.
Computed performance: 10 files / 20,000 source lines, Load 13.294 ms / 21,399,216 allocated bytes, document prep 73.667 us, maximum input/view 1.525 ms and resize/view 1.444 ms. Bounds documented; unknown and over-bound paths retain limitations and source access. Captured IDs/Count unchanged.
Limitations: local macOS measurements and PTYs; Linux CI not run. Presentation unchanged, so docs:check not applicable. No push, private receipts or credentials. No remaining blocker or resumable implementation step. Pre-existing unrelated worktrees were not adopted or removed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered bounded deterministic computed source diffs in the TUI for non-captured pairs, with explicit provenance and preserved captured-hunk semantics. Implementation baadf5d merged into main. Independent verification, full task check, 4.6 million fuzz executions, performance gate and four payment PTY cases passed. Owned worktree/branch cleaned up; task finalized on main.
<!-- SECTION:FINAL_SUMMARY:END -->

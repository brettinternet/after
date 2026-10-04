---
id: AFTER-5
title: Expose the complete raw diff and changed-oracle inventory
status: Done
assignee: []
created_date: '2026-10-03 05:40'
updated_date: '2026-10-04 01:45'
labels:
  - poc
  - git
  - review
  - reviewed
milestone: m-0
dependencies:
  - AFTER-3
  - AFTER-4
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
An example-first interface must not hide anything it cannot explain. Scope: bounded raw diff/source-context access and inventory bookkeeping shared by CLI and TUI. Highlight fixture/test/expected-output edits without treating them as a newly approved oracle.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Every changed path and available text hunk in each capture mode is accessible through the ordinary diff, including unclassified changes; unsupported/binary/oversized changes remain visible by path and count.
- [x] #2 Unique hunk IDs make mapped/folded/unclassified totals reconcile without double-counting hunks referenced by multiple examples; zero mapped hunks is a valid first-use state.
- [x] #3 Test, fixture, mask, golden and comparison-policy changes can be inspected beside production changes and labeled as potential oracle changes; user-selected frozen scenarios are not silently replaced.
- [x] #4 Diff/context access uses captured content rather than the live working tree and disables external helpers; path traversal, weird filenames, missing context and large patches have regression tests.
- [x] #5 Streaming/viewport-friendly access is bounded and preserves a usable raw path when import or richer evidence fails; partial rendering clearly exposes its limit.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a read-only rawdiff view over captured snapshot manifests and stored diff/source blobs: complete sorted path inventory, conservative potential-oracle labels, stable hunk IDs and deduplicated mapped/folded/unclassified accounting. Reuse capture-generated helper-free diffs; do not regenerate from live files or involve reports/scenarios.
2. Provide bounded byte-window diff/context access and explicit missing/partial/index-budget limits; retain raw diff and inventory independently of richer evidence. Test all capture modes, unusual paths, traversal, binary/unsupported entries, missing context, redaction and large patches.
3. Document API and limits, run Task Go/test gates and one independent trust-boundary verification, fix concrete findings, then commit implementation and final task state on main as explicitly requested.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented internal/rawdiff read-only views over captured manifests/blobs with sorted changed/unknown path inventory, potential-oracle hints, stable hunk IDs, deduplicated partition counts and 64 KiB byte windows. Index capped at 100,000 hunks without discarding raw bytes; incomplete capture, redaction, missing artifacts and unmatched paths expose limits. Real temporary-repository tests cover all capture modes, control/Unicode filenames, binary/mode/add/delete changes, unsupported and oversized content, frozen context after live writes, malicious helpers, traversal, import failures and frozen-scenario preservation. Parent checks: mise exec -- task check:go PASS (build/race/vet/gofmt); mise exec -- task test PASS (race tests, 45 local links, 20-task graph); mise exec -- task check:staged PASS (format/secrets). LSP clean on both new Go files. Independent verifier dce8e84b-ee2a-4d07-952d-f391b31461fa running; no acceptance marked yet. Working directly on main per current operator request. Existing after-2-storage worktree remains untouched and unadopted.

Resuming on main under explicit operator handoff of incomplete work. Rechecking staged implementation, prior independent verification and focused checks before delivery.

Delivered implementation on main as 42bd3b5 (Add captured raw diff review). Independent verifier dce8e84b-ee2a-4d07-952d-f391b31461fa passed criteria 2-5 and found a criterion-1 redaction collision: distinct originals could produce equal stored records and disappear from inventory. Fixed by retaining equal records as unknown when either capture is incomplete; added real capture regression TestRedactionCollisionRetainsUnknownPath and documented conservative overinclusion. Final mise exec -- task check:go PASS (build, race tests including regression, vet, gofmt); mise exec -- task test PASS (race tests, 45 links, 20-task graph); mise exec -- task check:staged PASS (format/secrets); git diff --cached --check PASS; both Go files LSP clean. One general independent verification pass completed; concrete finding fixed and affected checks rerun. No blockers. Existing after-2-storage worktree remains untouched and unadopted because ownership was not established; no checkout created for this task. No push. Final task metadata committed separately on main.

Review 2026: rawdiff.Open claimed a patch for any pair sharing a diff digest, including reversed or index/worktree pairs. Added rawdiff.CapturedPair (commit base, non-commit candidate, shared diff) used by Open and the TUI loader; other pairs get inventory only. Regression TestPatchOnlyForCapturedPair; RAW-DIFF.md updated.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added captured-only raw diff/context windows, complete changed/unknown inventory, potential-oracle hints and stable deduplicated hunk accounting. Real Git tests cover all capture modes, hostile paths/helpers, missing artifacts, bounded partial access, frozen scenarios and redaction collisions. Implementation: 42bd3b5; Task Go/test/staged gates passed. All acceptance criteria satisfied; no remaining task step or blocker. CLI/TUI wiring remains with its existing dependent tasks.
<!-- SECTION:FINAL_SUMMARY:END -->

---
id: AFTER-3
title: Capture immutable local Git comparisons without changing the checkout
status: Done
assignee: []
created_date: '2026-10-03 05:40'
updated_date: '2026-10-03 15:50'
labels:
  - poc
  - git
  - security
milestone: m-0
dependencies:
  - AFTER-1
  - AFTER-2
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Evidence cannot bind to a moving branch name or mixed working-tree contents. Scope: Git capture, inventory and temporary-repository tests; use Git CLI semantics rather than a new Git implementation. Support the three modes and explicit untracked policy in docs/IMPLEMENTATION.md.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Default HEAD/working-tree, HEAD/index and explicit merge-base branch comparisons resolve and store exact identities and content manifests; staged and unstaged changes remain distinguishable.
- [x] #2 Fixtures cover additions/deletions/renames, binary files, executable modes, spaces/newlines/Unicode/leading-dash paths and an unborn repository; the real user's index, worktree, refs and untracked files remain unchanged.
- [x] #3 Non-ignored untracked files are excluded and listed unless selected; ignored files and .after never enter captures implicitly. Oversize/unsupported content remains in the inventory with an explicit limitation.
- [x] #4 Controlled concurrent edits during capture trigger bounded retry or inconsistent-capture failure; no receipt binds to a falsely complete mixed revision. Document residual filesystem consistency limits.
- [x] #5 Conflicts, submodules, symlinks, sparse checkout, LFS pointers and missing objects are handled explicitly without traversing outside the repo or claiming unsupported content is captured.
- [x] #6 Hostile .gitattributes/config/environment tests prove capture does not invoke hooks, external diff/textconv/clean-smudge helpers, fsmonitor or network commands; explicit argv and sanitized Git environment prevent command injection.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a bounded capture package using sanitized, read-only Git plumbing and rooted filesystem reads; preserve content/mode and index state for working-tree, staged and resolved merge-base pairs. Extend snapshots only for unborn identity and index association.
2. Store immutable source artifacts and an ordinary binary Git diff produced solely from captured regular files in an isolated temporary repository. Inventory exclusions/unsupported entries; fail closed on repository-wide unsupported state and repeated concurrent edits.
3. Add temporary-repository tests for mode/path semantics, unchanged source state, concurrency barriers, hostile Git configuration/environment, unsupported entries and resource bounds. Document residual consistency and diff limits; keep CLI wiring in AFTER-10.
4. Run focused Task checks and independent trust-boundary verification, fix concrete findings, commit implementation, integrate main, then finalize and commit authoritative task state.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented bounded read-only Git capture and private immutable persistence for working-tree/index/merge-base modes, including unborn snapshots and stored index association. Added path/mode/binary/deletion/rename, exclusion/oversize, unsupported repository/filesystem, controlled-edit, redaction and hostile config/environment tests. Independent verifier ran task check:go and found ambient PATH could substitute Git; fixed to trusted /usr/bin/git and added fake-Git PATH sentinel regression. Verification used the task copy in the worktree because the verifier sandbox denied primary-checkout reads; parent retained authoritative provider ownership. Parent also fixed private-store self-inventory changing capture IDs and ensured staged captures list excluded untracked paths, with regressions. After fixes: mise exec -- task check:go PASS (build, race suite, vet, gofmt); mise exec -- task test PASS (race suite, 40 local links, 20-task graph). LSP diagnostics clean on affected source. docs/CAPTURE.md documents bounded reads, unsupported states, non-atomic/ABA residual consistency and captured-regular-file-only diff limitations. No execution receipts or CLI capture command are claimed.

Delivery: implementation acb7d1f fast-forwarded into main without disturbing the authoritative task claim. Re-ran mise exec -- task check:go and mise exec -- task test on main: PASS. Implementation staging gate passed formatting and secret checks. Session-owned implementation worktree/branch removed with Worktrunk after receipt and idle-shell checks; matching Herdr workspace disappearance verified. Pre-existing after-2-storage worktree/workspace retained because it belongs to earlier work and was not adopted. No remaining AFTER-3 blocker; CLI exposure remains AFTER-10. Next dependency-ready item is AFTER-4; not started.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered immutable local Git capture in acb7d1f: three comparison modes, exact commit/content/mode identities, index association and unborn support; explicit exclusions/unsupported states, bounded consistency checks and hardened Git/filesystem access. Verified on main with task check:go and task test, plus staged format/secret checks. Independent verification identified PATH injection; fixed and regression-tested. Documented non-atomic filesystem and partial-diff limits in docs/CAPTURE.md. No project execution or fabricated observations.
<!-- SECTION:FINAL_SUMMARY:END -->

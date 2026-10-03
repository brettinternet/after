---
id: AFTER-3
title: Capture immutable local Git comparisons without changing the checkout
status: To Do
assignee: []
created_date: '2026-10-03 05:40'
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
- [ ] #1 Default HEAD/working-tree, HEAD/index and explicit merge-base branch comparisons resolve and store exact identities and content manifests; staged and unstaged changes remain distinguishable.
- [ ] #2 Fixtures cover additions/deletions/renames, binary files, executable modes, spaces/newlines/Unicode/leading-dash paths and an unborn repository; the real user's index, worktree, refs and untracked files remain unchanged.
- [ ] #3 Non-ignored untracked files are excluded and listed unless selected; ignored files and .after never enter captures implicitly. Oversize/unsupported content remains in the inventory with an explicit limitation.
- [ ] #4 Controlled concurrent edits during capture trigger bounded retry or inconsistent-capture failure; no receipt binds to a falsely complete mixed revision. Document residual filesystem consistency limits.
- [ ] #5 Conflicts, submodules, symlinks, sparse checkout, LFS pointers and missing objects are handled explicitly without traversing outside the repo or claiming unsupported content is captured.
- [ ] #6 Hostile .gitattributes/config/environment tests prove capture does not invoke hooks, external diff/textconv/clean-smudge helpers, fsmonitor or network commands; explicit argv and sanitized Git environment prevent command injection.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

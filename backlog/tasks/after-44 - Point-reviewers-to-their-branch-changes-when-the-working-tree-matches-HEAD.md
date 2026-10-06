---
id: AFTER-44
title: Point reviewers to their branch changes when the working tree matches HEAD
status: To Do
assignee: []
created_date: '2026-10-06 21:41'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-24
  - AFTER-37
documentation:
  - docs/CLI-DESIGN.md
  - docs/TUI-DESIGN.md
  - docs/CLI.md
  - docs/CAPTURE.md
priority: medium
type: feature
ordinal: 44000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
When an agent commits its work, the working tree matches HEAD. `after capture` then records an empty change without comment: on a feature branch one commit ahead of `main`, the candidate's diff is the empty-content digest. A plain `after review` would open an empty review. The change is reachable only with `--base main`, and nothing suggests it.

Scope, per docs/CLI-DESIGN.md "review" and "capture": when the fresh capture has no changes and there is no saved review, `after review` opens nothing, exits 0, says what matched, and suggests capture flags that would find a change: `--base` with the default branch when HEAD is ahead of it (with the commit count), and `--include-untracked` when untracked files were excluded. The default branch is the remote default (`refs/remotes/origin/HEAD`), else a local `main`, else `master`, read through the capture package's hardened Git runner. `after capture` still records the empty capture and shows the same suggestions. Nothing is chosen automatically, and no project code runs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 On a feature branch whose changes are committed, `after review` with no saved review says the working tree matches HEAD, suggests `after review --base main` with the number of commits ahead, opens no TUI, and exits 0.
- [ ] #2 When untracked files were excluded, the message names up to three, sanitized, with `--include-untracked` suggestions; an empty `--staged` or `--base` capture says what matched; on the default branch with nothing ahead, it says there is nothing to review.
- [ ] #3 The default branch comes from `refs/remotes/origin/HEAD`, else a local `main`, else `master`, through the capture package's hardened Git runner; tests cover each source, a detached HEAD, an unborn repository and no candidate branch, and prove no project code runs.
- [ ] #4 `after capture` records an empty capture as today and shows the same suggestions in its Next block, and with a saved review, `after review` resumes as usual.
- [ ] #5 docs/CLI.md and docs/CAPTURE.md document the guidance.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

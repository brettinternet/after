---
id: AFTER-34
title: >-
  Resolve the checkout root, keep .after/ out of Git, and report specific
  capture errors
status: To Do
assignee: []
created_date: '2026-10-06 21:14'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies: []
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/CAPTURE.md
  - docs/STORAGE.md
priority: high
type: bug
ordinal: 34000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The CLI audit in docs/CLI-DESIGN.md "Why" found that `after capture` from a subdirectory or outside Git prints only `capture failed; repository was not changed`, yet leaves a new `.after/writer.lock` in the current directory. Specific reasons such as `unmerged index is unsupported` or `shallow repositories are unsupported` are hidden behind that message. After the first capture, `git status` shows `?? .after/`, so `git add -A` would commit private captured content. Every later CLI task assumes commands work from anywhere in a checkout.

Scope, per docs/CLI-DESIGN.md "Project and storage": the project is the Git top-level containing the current directory or the configured project path, found by checking `.git` in that directory and its parents without running anything; the capture package still receives an explicit root and keeps its own no-discovery rule. Outside a work tree, exit 2 with the design's message and create nothing. Writers create `.after/` only at the top-level after resolution succeeds; read-only commands never create it. New stores get `.after/.gitignore` containing `*`, and existing stores get it at their next writable open, without editing the user's ignore files. Capture failures print the capture package's fixed reason strings, never repository text, and output never prints the project's absolute path.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Every command run from a nested subdirectory uses the checkout's top-level: a capture from a subdirectory equals one from the top-level, and no `.after/` appears in the subdirectory.
- [ ] #2 Outside a Git work tree, every project command exits 2 with `after: not inside a Git repository — run AFTER in a checkout, or pass --project DIR` and creates no `.after/` directory or lock file; `help`, `version` and `config` still work.
- [ ] #3 A store created by any writer contains `.after/.gitignore` with `*`, an existing store without one gains it at its next writable open, `git status --porcelain` shows nothing under `.after/` after a capture, and no user ignore file is modified.
- [ ] #4 Unmerged-index, shallow, sparse and ambiguous-merge-base fixtures each print `after: capture failed: <the capture package's reason> — <a fixed fix>` and exit with today's codes; tests prove no repository text or absolute project path appears.
- [ ] #5 docs/CLI.md, docs/CAPTURE.md and docs/STORAGE.md describe root resolution, the ignore file and the error format.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

---
id: AFTER-34
title: >-
  Resolve the checkout root, keep .after/ out of Git, and report specific
  capture errors
status: Done
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-07 03:43'
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
- [x] #1 Every command run from a nested subdirectory uses the checkout's top-level: a capture from a subdirectory equals one from the top-level, and no `.after/` appears in the subdirectory.
- [x] #2 Outside a Git work tree, every project command exits 2 with `after: not inside a Git repository — run AFTER in a checkout, or pass --project DIR` and creates no `.after/` directory or lock file; `help`, `version` and `config` still work.
- [x] #3 A store created by any writer contains `.after/.gitignore` with `*`, an existing store without one gains it at its next writable open, `git status --porcelain` shows nothing under `.after/` after a capture, and no user ignore file is modified.
- [x] #4 Unmerged-index, shallow, sparse and ambiguous-merge-base fixtures each print `after: capture failed: <the capture package's reason> — <a fixed fix>` and exit with today's codes; tests prove no repository text or absolute project path appears.
- [x] #5 docs/CLI.md, docs/CAPTURE.md and docs/STORAGE.md describe root resolution, the ignore file and the error format.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [x] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Resolve the configured project to its nearest checkout using filesystem-only .git ancestor checks in the shared CLI configuration path; preserve help/version/config behavior and capture explicit-root contract. 2. Publish the self-ignore file using existing private-store durability/safety primitives on writable opens only. 3. Map capture failures to allowlisted fixed reasons and actionable fixed guidance without exposing repository text. 4. Add root/no-side-effect/store/error regression tests and update CLI, capture and storage docs. 5. Run Task checks and real 80-column PTY/pipe checks with and without NO_COLOR; independently verify trust boundaries, then commit, integrate, finalize and clean the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented and fast-forwarded b21fb67 (Fix checkout resolution and private store capture errors) into main. Filesystem-only CLI root resolution leaves the capture API explicit-root; all writers durably publish the private self-ignore file; errors are fixed allowlisted reasons/fixes. Updated CLI/CAPTURE/STORAGE docs and help. Regression code is larger than production changes because it exercises all eight project commands in four process modes plus four real unsupported Git fixtures.

Validation passed independently in implementation and verifier sessions: mise exec -- task test:cli; mise exec -- go test -race ./internal/store ./internal/capture; mise exec -- go test ./internal/cli -run "^(TestCLIProcessHelper|TestProjectCommandProcessModes)$" -count=1 -v; mise exec -- go test ./internal/cli -run "^TestCaptureErrorsAreFixedAndActionable$" -count=1 -v; mise exec -- task check (formatting, full race suite, links/backlog integrity, build, vet, Go formatting, Gitleaks); git diff --check. Parent staged implementation and passed mise exec -- task check:staged before commit. LSP diagnostics clean for root resolver and store.

Real 80-column PTYs and OS pipes exercised capture/import/inspect/compare/export/run/pin/review plus help/version/config with NO_COLOR unset and set. Excerpts: capture emitted schema_version 1, kind capture JSON; missing receipt emitted after: invalid input: receipt ID was not found. Outside Git emitted after: not inside a Git repository — run AFTER in a checkout, or pass --project DIR (exit 2, no files). Unsupported fixture excerpt: after: capture failed: unmerged index is unsupported — resolve the index conflicts, then retry capture (exit 1). Shallow, sparse and ambiguous merge-base fixtures also passed exact safe-message assertions.

Single independent acceptance/review pass: PASS, no concrete scoped defects. Child filesystem boundary prevented primary task access; children read the complete copied task, and parent reread authoritative primary intent/criteria before integration (unchanged apart from claim/plan). No execution isolation change or Docker proof required for this filesystem/CLI change. No remaining blocker or resumable implementation step. Session-owned worktree and branch removed with Worktrunk after receipt, clean checkout, completed children and shell-only pane checks; post-remove Herdr hook removed the exact associated workspace, verified absent. Pre-existing unrelated worktrees were not adopted or altered. No push.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered b21fb67 on main: checkout-root CLI resolution, ignored private stores and safe actionable capture errors. Full Task checks, real PTY/pipe matrix and independent verification passed. Owned worktree/branch/workspace cleaned; final task metadata committed separately. No push or next-task work.
<!-- SECTION:FINAL_SUMMARY:END -->

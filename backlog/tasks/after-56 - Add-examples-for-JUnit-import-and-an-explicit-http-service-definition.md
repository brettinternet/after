---
id: AFTER-56
title: Add examples for JUnit import and an explicit http-service definition
status: Done
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 18:46'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-48
  - AFTER-49
documentation:
  - examples/README.md
  - docs/JUNIT-REPORTS.md
priority: low
type: docs
ordinal: 56000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
examples/ now shows the command scenario kind (07) but not the other two AFTER-48/49 features: importing a JUnit XML report from a non-Go producer, and running an operator-selected `http-service` definition with `--definition`. New users reading examples/README.md cannot discover either. Imported reports must come from a real producer run, not a canned fixture presented as fresh output, and examples must not download dependencies implicitly.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 An example imports JUnit XML produced live by a named producer available through mise (or explains the explicit toolchain prerequisite) and shows reported-only cards beside the diff
- [x] #2 A Docker example runs a non-payment http-service definition selected with --definition on its separately provisioned pinned image and shows a changed and an equal case
- [x] #3 examples/README.md lists both; task examples smoke-tests whichever needs no Docker
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [x] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse examples/lib.sh for a live Bun built-in JUnit producer (no package downloads) and reported cards beside the diff. 2. Reuse the pinned Python stdlib HTTP fixture with explicitly selected echo/changed-case definition and exact consent. 3. Add the non-Docker script to task examples, document prerequisites and limits, execute both examples, and perform one scoped review. 4. Stage/check/commit implementation, integrate into main, finalize task metadata, and remove only this session-owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator explicitly authorized AFTER-56 after M1/M2 exhaustion. Claimed before creating after-56-examples; setup hooks reviewed and completed. Original creation receipt is in the Git-local after-56-examples worktree metadata. Existing unrelated worktrees are not owned by this task.

Implemented examples/cli/08-junit-report.sh (live Bun 1.4.2 built-in JUnit producer; no packages) and 09-http-service.sh (operator-selected Python echo definition, lowercase changed and uppercase equal). Reused existing helpers and pinned Python fixture; no engine or trust-boundary changes. Bun suite file attribute is outside the current dialect: unmodified XML remains incomplete with unsupported_attribute and both closed reported cards retained; documented rather than stripping data or widening the parser. Validation: mise exec -- task examples passed (including the new non-Docker script); mise exec -- task test:reports passed; mise exec -- task test passed all race tests, 151 local links and 57-task integrity (first 180-second invocation timed out, rerun with 650-second limit passed); bash -n on both new scripts and task check:staged passed. Both shell LSP reports clean; YAML diagnostics unknown, Task executed successfully. Live PTY execution of mise exec -- examples/cli/09-http-service.sh with AFTER_EXAMPLES_STEP=0 and explicit local Docker settings approved the displayed exact synthetic plan: eight samples, completed/different, both responses equal, lowercase provider body hello to HELLO, uppercase provider equal; script exit 0 with required run/compare exit 4. First harness attempt used a nonexistent Docker CLI path and failed before Docker contact; corrected to the installed trusted CLI. One scoped self-review verified consent, no downloads, original report preservation and actual card inspection; no remaining defects. No browser/presentation changes. No new tests: existing suite plus direct script execution covers this documentation-only change.

Implementation commit 2537eb1 was fast-forwarded to main, preserving the primary task claim. Session receipt matched branch, checkout, creation commit and current session; Worktrunk listing was clean and merged. No delegated agents; the exact associated Herdr workspace contained only an idle zsh, no editor or active job. Worktrunk foreground removal deleted the owned checkout and branch and ran its Herdr teardown hook. Existing unrelated worktrees were preserved. Temporary synthetic example workspaces are retained by design for exploration; no private reports or local paths were committed. No remaining task blocker or resumable implementation step; no push authorized.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added live Bun JUnit-import and explicitly selected non-payment Python HTTP-service examples; documented report dialect and pinned-image limits. task examples, task test:reports, task test, shell syntax and staged checks passed; live exact-consent Docker run proved changed lowercase/equal uppercase cases. Implementation 2537eb1 merged to main; owned worktree/branch cleaned up.
<!-- SECTION:FINAL_SUMMARY:END -->

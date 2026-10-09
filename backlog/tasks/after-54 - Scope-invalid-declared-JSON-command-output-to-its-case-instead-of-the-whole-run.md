---
id: AFTER-54
title: >-
  Scope invalid declared-JSON command output to its case instead of the whole
  run
status: Done
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 17:07'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-50
documentation:
  - docs/COMPARISON.md
  - docs/RUNNER.md
priority: low
type: enhancement
ordinal: 54000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
When a command definition declares stdout or stderr as `json`, one sample with invalid or empty JSON makes the entire comparison fail (whole run incomparable), discarding valid witnesses for every other case. Error paths of real CLIs often print nothing or plain text on stdout, so examples/cli/07-command-scenario.sh had to make its CLI print JSON even for errors. Incomparability should stay honest but be scoped to the case/channel that caused it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Invalid declared JSON in one case/channel marks that witness incomparable with a fixed diagnostic, and other cases still compare; the overall outcome is never equal while any witness is incomparable
- [x] #2 Comparison rules and stored details stay strictly versioned; an existing stored receipt re-derives the same or a more conservative result
- [x] #3 Focused compare tests cover invalid JSON in one case alongside a valid changed case and a valid equal case
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [x] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Retain all command witnesses when declared JSON parsing fails: emit an incomparable witness with original artifact refs and a fixed channel diagnostic in existing v1 limits; keep aggregate incomparable and preserve strict frozen policy bytes/schema. Preserve conservative incomplete aggregate semantics for existing receipts.
2. Add focused public comparison regressions for mixed invalid/changed/equal cases, stdout/stderr and repetition precedence, and deterministic stored receipt re-derivation. Document retained witnesses and aggregate limitations.
3. Run command and Go checks; perform one independent trust-boundary verification, fix concrete scoped findings, then commit, integrate into main, finalize authoritative task state and clean the receipt-owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator explicitly authorized AFTER-54 after the automatic POC queue was exhausted. Claimed from main before creating after-54-json-witness; unrelated existing worktrees preserved. Project setup hooks reviewed and approved (tool installation, frozen dependencies and Lefthook; no services).

Implemented and fast-forward integrated aa90a6e into main, preserving the authoritative claim. Invalid declared JSON retains artifact-linked incomparable witnesses and fixed channel diagnostics; all valid paired/repetition witnesses continue. Aggregate outcome/completeness remain incomparable/incomplete, including when other cases differ or drift. Existing v1 shapes, exact policy bytes and consent remain unchanged; re-derivation is deterministic. Readable summaries/case projections intentionally remain conservative; full JSON/details expose retained valid witnesses. No execution boundary changed, so Docker proofs were not rerun.

Verification: task format:go, task command:check, task check:go, task test, git diff --check and task check:staged passed. All three edited Go files had clean gopls diagnostics. Initial check:go hit a 120-second tool timeout; its longer retry exposed an existing browser performance budget failure (navigation 137.9ms against 100ms) during concurrent checks. A subsequent check:go without concurrent verification passed, including the browser test, build, race suite, vet and staticcheck. task test passed all race tests, 150 links and 57-task integrity. Post-integration task command:check passed on main.

One independent verification pass: workflow 5dc90525-b9ef-45ed-abc0-bc21f8c305cf, verifier 383064a9-6326-4ac5-b712-2d4cbbc2cb0d, PASS with no concrete scoped defects; independently ran task command:check and inspected versioning and fail-closed projections. Legacy compatibility relies on unchanged schema/rules and conservative outcomes; the re-derivation test uses a generated stored receipt, not an archived pre-change fixture.

Cleanup verified the session creation receipt and live Worktrunk checkout/branch, completed child, and exact Herdr workspace w34 with only idle zsh. wt remove after-54-json-witness --foreground --format=json removed the worktree and branch; post-remove hook closed w34, confirmed absent on follow-up. Unrelated pre-existing worktrees preserved. No blocker or next implementation step remains; final task metadata is committed separately on main. No push or next task authorized.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered AFTER-54 in aa90a6e: invalid JSON is scoped to incomparable command witnesses without losing valid case/channel results; aggregate remains fail-closed. Focused regressions, full Go checks, link/backlog checks, staged checks and independent verification passed. Documentation records v1 compatibility and conservative presentation limits. Owned worktree, branch and workspace removed.
<!-- SECTION:FINAL_SUMMARY:END -->

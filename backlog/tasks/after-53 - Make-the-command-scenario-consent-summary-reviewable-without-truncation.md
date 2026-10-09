---
id: AFTER-53
title: Make the command-scenario consent summary reviewable without truncation
status: Done
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 16:42'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-50
documentation:
  - docs/RUNNER.md
priority: medium
type: enhancement
ordinal: 53000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The terminal consent summary for a command definition prints each field as one raw JSON line clipped with "…" (definition, build argv, cases, stdin digests, Docker policy). An operator cannot see which argv, stdin, environment or input files will run before typing yes without separately running `after inspect PLAN`. The payment consent was made readable in AFTER-29/AFTER-40; command plans (and http-service definitions) did not get the same treatment. Consent is only meaningful if the exact plan is reviewable on the screen that asks for it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 The consent summary lists each command case with id, title, argv, environment, input-file paths/sizes and stdin size plus a short safe preview, without silent truncation of any field that differs between cases
- [x] #2 Image/platform, build argv, comparison modes, Docker policy (including --interactive) and limits are readable at 80 columns; anything elided says how to see it in full (after inspect PLAN)
- [x] #3 Hostile definition text (terminal controls, very long argv) stays sanitized and bounded; consent still binds exact plan bytes
- [x] #4 Consent view goldens cover a multi-case command definition at 80x24 and 120x40
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [x] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Replace command consent raw-JSON rows with bounded, escaped per-case fields and readable execution/preparation policy; reuse existing terminal wrapping and exact-plan inspection instead of changing authorization.
2. Ensure CLI prompt/preview/inspection and TUI summaries expose all case differences or explicit elision with after inspect PLAN guidance. Add regression coverage for hostile/long fields and multi-case 80x24/120x40 consent goldens, then update RUNNER documentation.
3. Run focused Task checks and one independent acceptance verification emphasizing disclosure, terminal safety and unchanged exact-byte consent; fix concrete scoped defects only.
4. Refresh authoritative claim/main, commit staged checked implementation, integrate on main, finalize task metadata, and remove only the receipt-verified owned checkout/branch/workspace.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator explicitly authorized AFTER-53 after the M1/M2 queue was found complete. Claimed on primary; approved worktree hooks install pinned tools/dependencies/hooks only. Owned worktree .worktrees/after-53-consent, branch after-53-consent, created from 94613d4; Git-local agent-creation.json records session 01a12160-9d57-725c-8999-5bf708b221a7 and loop ccbd48ff-f742-4af7-b733-b6b41e59139d. Existing unrelated worktrees retained. Current code emits whole command definitions/case arrays as single JSON rows, then CLI clips each row; TUI shares the summary decoder. No execution behavior change planned.

Workflow 229d9ebd failed because executor fce52771 hit the 1800000ms run deadline before returning a final report; independent verifier never launched. Partial implementation remains on after-53-consent at base 94613d4: eight tracked files plus two new consent goldens. Tracked diff captured outside repository as after-53-timeout-partial.patch; no test subprocess remains. Resume the same retained executor through the subagent protocol to finish/checkpoint validation, then run the originally planned independent verification. No acceptance or integration claimed.

Recovered executor 39789435 completed; independent verifier b08aa036 passed AC1–4 with no scoped defect. task test:views (including regenerated 80x24/120x40 command goldens), task command:check, task test:terminal, task test:cli, task format:check and git diff --check passed. Verifier independently ran these targets and uncached focused command consent/browser/CLI/golden checks. Parent LSP diagnostics clean for command_consent.go, cli.go and render.go; task check:staged passed formatting/secrets. Implementation 4a5b37e fast-forward integrated into unchanged main at 94613d4, preserving primary task notes. Projection is bounded with explicit full-plan inspection guidance, per-case data and safe stdin preview; exact authorization bytes and runner are unchanged. Docker proofs and full release gate not run: no execution behavior changed. Goldens show initial scrollable viewports; CLI regression exercises both cases. Final task metadata and receipt-verified cleanup remain.

Post-integration task command:check passed on main. Cleanup matched Worktrunk listing and original session creation receipt; all delegated runs terminal, exact Herdr workspace w33 contained only idle zsh. wt remove after-53-consent --foreground --format=json removed the checkout and branch, and post-remove hook closed its workspace; filesystem/branch/workspace absence verified. Existing unrelated worktrees preserved. No blocker or next implementation step remains; final task state committed separately on main. No push or next task authorized.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered AFTER-53 on main in 4a5b37e: readable bounded per-case command consent, explicit full-plan inspection, unchanged exact-byte authorization, hostile/long input regressions and 80x24/120x40 goldens. Independent verification passed all four criteria; CLI, terminal/browser, command, format and staged checks passed. Docker/full release gate not rerun because execution is unchanged. Owned worktree, branch and workspace removed; claim released.
<!-- SECTION:FINAL_SUMMARY:END -->

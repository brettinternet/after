---
id: AFTER-40
title: 'Show a readable run consent, store plans privately, and guide Docker setup'
status: Done
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-08 04:06'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-29
  - AFTER-37
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/RUNNER.md
  - docs/SANDBOX.md
priority: medium
type: enhancement
ordinal: 40000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
An interactive `after run` dumps the whole plan JSON to stderr before asking for `yes`. A non-interactive run needs `--plan-out FILE`, then `--plan-file FILE --approve DIGEST`. Missing Docker settings get no guidance, and `run` needs two snapshot IDs.

Scope, per docs/CLI-DESIGN.md "run", "config" and "inspect": bare `after run` prepares a plan for the newest capture and names it. The terminal shows the consent summary rows from AFTER-29, decoded from the exact plan bytes by the same function, and typing `yes` stays the interactive consent. Plans are stored privately in `.after/` (mode 0600, never overwritten). `--approve DIGEST` finds a stored plan by its full digest, then reconstructs and checks it exactly as `--plan-file` does; `--plan-out` and `--plan-file` keep working, and the non-interactive preview still exits 3 with the exact approve command. A terminal shows elapsed time on stderr during a run. Missing Docker settings are named with the configuration lines to add, and a detected Docker CLI on `PATH` and an existing socket file may be suggested, without contacting Docker or choosing an endpoint. `after config` lists setup problems, and `after inspect PLAN` prints the summary, then the indented plan. Consent semantics and the execution boundary do not change.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 On a terminal, bare `after run` names the newest capture, prints the consent summary rows from the exact plan bytes and the plan's size, and runs only after `yes`; any other answer runs nothing, and a summary decode failure says so while consent still binds the exact bytes.
- [x] #2 Without a terminal, `after run` stores the plan in `.after/` with mode 0600, prints the exact `after run --approve sha256:…` command with the full digest, and exits 3; the stored file is never overwritten.
- [x] #3 `after run --approve DIGEST` runs only a stored plan whose reconstruction matches its bytes and digest, refuses a prefix, mismatch or modified file with nothing run, and `--plan-out`/`--plan-file` behave as today; tests cover each refusal.
- [x] #4 Missing or invalid Docker settings are named with the configuration lines to add, may suggest a detected Docker CLI path and existing socket file, and tests prove no Docker contact or endpoint selection; `after config` lists the same setup problems.
- [x] #5 During an approved run, stderr on a terminal shows elapsed time with no percentage; `after inspect PLAN` prints the summary, then the indented, sanitized plan; docs/CLI.md and docs/RUNNER.md document the flow.
- [x] #6 `after status` suggests `after run` by the design's rule, and Next blocks offer it where the design shows it.
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
1. Reuse the existing strict consent-summary decoder for both CLI and TUI; extend stored-only newest-capture resolution to run and readable plan inspection. Preserve exact preview bytes and current explicit file flow.
2. Store immutable private plans by full authorization digest using existing bounded private-store conventions; require full digest and reconstruct exact bytes before execution. Add passive Docker setup diagnostics shared with config and existing elapsed stderr notices.
3. Update status/Next/help and CLI/RUNNER docs. Add refusal, no-contact, immutable storage, summary and 80-column PTY/pipe tests with NO_COLOR variants.
4. Run focused and relevant Task checks, one independent acceptance verification focused on consent/storage/no execution boundaries, fix concrete defects, commit, integrate main, finalize authoritative task and remove receipt-verified owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation completed uncommitted in session-owned .worktrees/after-40-consent (branch after-40-consent, base 3572a511). Creation receipt: .git/worktrees/after-40-consent/agent-creation.json; original session 01a1197f-cd24-70e2-be64-ada621829913. Executor reports task test:cli, check:go, test and format:check PASS, with real 80-column PTY/pipe variants. Parent LSP diagnostics clean for workflows.go, docker_setup.go and store/plans.go. Single independent acceptance verification is running, concentrated on exact-byte approval, immutable private storage and no-contact setup; no completion claim yet. No live Docker run has been attempted.

Delivered implementation 9c25fbe (Add readable consent and private run plans), fast-forwarded main after rechecking authoritative claim and preserving primary task edits. Single independent verifier passed all six AC with no findings; no second general review performed.
Verification: executor and independent verifier both passed mise exec -- task test:cli (race), task check:go (build/all-package race/vet/gofmt), task test (tests/141 local links/backlog graph), task format:check and git diff --check. Parent task check:staged and commit hooks passed formatting/secrets with no leaks. LSP clean for run workflows, Docker setup and plan store.
Real 80-column PTY and pipe coverage with NO_COLOR unset/set passed for changed command forms, including bare preview, plan inspect and approval-prefix refusal; existing config/status process cases also exercised. Focused command: mise exec -- go test -v ./internal/cli -run ^TestProjectCommandProcessModes/(run-bare-preview|inspect-plan)/no-color=yes/pty$ -count=1. Output excerpts: Nothing has run. This exact plan is stored privately.; Using the newest capture: base … → candidate …; Execution plan … · 18.2 KiB · exact stored bytes; Consent summary. Preview exits 3, inspect exits 0. Exact bytes/digests, mode0600, immutable storage, prefix/mismatch/modified refusal and no receipt on refusal tested. Fake Docker markers prove passive diagnostics do not contact Docker or choose endpoints. Limitation: no live Docker daemon run performed; daemon connectivity is not established by these checks.
Cleanup complete: original creation receipt matched current session/path/branch; both child runs completed. Worktrunk listing showed clean checkout at main commit. Herdr workspace/pane inventories had no entry for the owned checkout before cleanup. wt remove after-40-consent --foreground --format=json --yes removed worktree and branch and ran post-remove hook; Git and Herdr inventories confirm absence. Six unrelated pre-existing worktrees retained untouched (not session-owned). No remaining task blocker or resumable step; claim released; no push.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Readable shared exact-byte run consent, immutable private full-digest plans, passive Docker setup guidance, plan inspection and Next suggestions implemented in 9c25fbe on main. All six criteria independently verified; Go/CLI/docs/backlog/PTY/pipe/staged checks passed. Owned worktree and branch removed; live Docker execution was not performed.
<!-- SECTION:FINAL_SUMMARY:END -->

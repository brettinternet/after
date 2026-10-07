---
id: AFTER-38
title: 'Use one grammar, honest help, and errors that say how to fix them'
status: Done
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-07 06:58'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-35
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/REVIEW.md
  - docs/DEMO.md
  - docs/EVALUATION.md
priority: high
type: enhancement
ordinal: 38000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Pairs are `CANDIDATE --base BASE` in `inspect` and `review` but `BASE CANDIDATE` in `run`. `--base` is a snapshot ID in some commands and a Git ref in `capture`, and pin decisions live under `review`. Help lists all ten global flags on every command, including Docker flags on `capture`, and misreports defaults: `--repetitions (default: 0)` is 1, `--run-seconds (default: 0)` is 180, and `--interactive (default: false)` is true. `after captrue` prints `unknown command; use --help`, and `after capture --stagd` prints `invalid command arguments or flags`.

Scope, per docs/CLI-DESIGN.md "Grammar changes" and "Errors and help": every grammar row except `import` (AFTER-39) and `run` (AFTER-40). Pin creation and decisions move to `after pin`, with the design's `--scope`, `--mode` and `--reason` defaults. Removed forms fail with an error showing the new form. Errors read `after: <what is wrong> — <how to fix it>`, with closest-match suggestions within two edits for commands and flags. Help gives one sentence, usage examples, the command's own options with real defaults read from the configuration defaults, and a short global group; Docker and run limits appear only on `run`. Exit codes are unchanged. This task updates every caller.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Pairs are `BASE CANDIDATE` in `inspect`, `review` and `run`, and `--base` accepts only a Git ref; `after pin PIN` prints the pin, and `--accept`, `--attach RECEIPT` and `--select SNAPSHOT [--mode MODE]` record decisions with `--reason` optional and a default reason naming the command line, stored verbatim in history.
- [x] #2 `after pin RECEIPT --expectation TEXT` defaults `--scope` to `finite_example`, and a receipt that cannot support it fails with an error suggesting `--scope human_intent`.
- [x] #3 Each removed form (`review PIN …`, `inspect CANDIDATE --base BASE`, `review --tui …`) exits 2 with an error that shows the equivalent new command; unknown commands and flags suggest the closest match within two edits, or none.
- [x] #4 Every error follows `after: <what is wrong> — <how to fix it>` with sanitized user input, missing arguments are named with a runnable example, and exit codes match today's for the same failures.
- [x] #5 Each command's help lists only its own options with defaults read from the configuration defaults (a test fails if a printed default differs), shows global options in a short group, and shows Docker and run limits only on `run`; golden help files cover every command.
- [x] #6 Examples, the demo, the study kit, the POC gate, Taskfile proofs, docs/CLI.md, docs/REVIEW.md, docs/DEMO.md and docs/EVALUATION.md use the new grammar; `task test`, `task examples`, `task demo:inspect` and `task study:check` pass, and Docker-gated proofs are rerun when Docker settings are available.
- [x] #7 `after --help` and `after help` start with one sentence, group the commands as the design's top-level help does with the everyday commands first, and list the global options once; a golden file covers it.
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
1. Update the existing urfave CLI grammar: positional snapshot pairs, terminal review without --tui, pin inspection/actions with finite scope, original-base and command-line reason defaults; preserve run consent and defer import/run redesign and bare-command defaults to their owning tasks.
2. Centralize sanitized actionable diagnostics and closest-match suggestions; replace generated help with honest per-command examples/options and configuration-backed defaults, covering every command with goldens.
3. Migrate tests, PTY proofs, examples, demo/study, POC callers and documentation. Exercise new/removed forms, history defaults, exits, safety, and 80-column terminal/pipe NO_COLOR matrix. Run relevant Task targets and Docker proofs only if settings are available.
4. One fresh independent acceptance verification focusing on grammar migration, actionable errors, help defaults and unchanged consent. Fix concrete findings, check staged changes, commit implementation, integrate main, finalize authoritative metadata, and remove the receipt-owned worktree/workspace.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation is uncommitted in receipt-owned after-38-grammar worktree. Executor reports passing task test (full Go race suite, 141 links, 45 tasks), check:go, examples, demo:inspect and study:check; native 80-column PTY/pipe plus NO_COLOR matrix and native review PTY pass. Excerpts: Captured candidate 4dbef42f (merge base); Snapshot 4dbef42f; run help: Prepare the frozen offline experiment; exact consent is required before it runs. Explicit Docker binary/host settings absent (CLI exists), so conditional Docker proofs not claimed. First independent verifier is running; workflow result-field mismatch prevented the originally planned verifier from launching, corrected by launching it separately. No implementation commit or integration yet.

Delivered implementation 4707b4c and fast-forwarded main. Independent verifier found no concrete defect and passed grammar, pin behavior/default history, diagnostics, help, caller migration and non-Docker checks; its overall UNVERIFIED label refers solely to unavailable conditional Docker proofs, not a failing criterion. Docker proofs remain unrun because explicit binary/host settings are absent, as permitted by AC6. Executor passed mise exec -- task examples, demo:inspect, study:check, test and check:go, plus TestProjectCommandProcessModes and TestBrowserDocumentPTY in real terminals/pipes with NO_COLOR matrix. Parent LSP diagnostics clean for CLI, diagnostics, help, review, workflows and config. Parent task check:staged initially found examples/README.md formatting; task format corrected only that file, then staged checks/secret scan passed before commit. Post-integration mise exec -- task test passed full race tests, 141 local links and 45-task integrity; initial 120-second command window expired, rerun with a sufficient window passed (CLI suite 127.8 seconds). Ownership receipt matched this session, checkout and branch; no child agents active, exact Herdr workspace had only idle zsh and no editor. Worktrunk removed owned worktree/branch and its hook closed the workspace; verified absent. Pre-existing unrelated worktrees retained without adoption. No push, private evidence or credentials committed. No remaining blocker or resumable step for this item; final task metadata committed separately.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Unified positional pair grammar and pin decisions, safe actionable errors with suggestions, truthful configuration-backed help/goldens, and migrated callers/docs. Integrated 4707b4c; tests, examples, demo/study and PTY checks pass. Independent verification found no defects; conditional Docker proofs unavailable. Owned worktree, branch and workspace removed.
<!-- SECTION:FINAL_SUMMARY:END -->

---
id: AFTER-37
title: 'Make bare commands useful with status, log, resolved defaults and Next blocks'
status: Done
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-07 16:03'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-36
  - AFTER-38
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/REVIEW.md
priority: high
type: feature
ordinal: 37000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
With no arguments, `inspect`, `review`, `compare`, `import`, `pin` and `export` exit 2 with `unexpected or missing command arguments`, `run` needs two IDs or a plan file, and bare `after` prints help. Nothing lists recent captures or runs, and no result suggests what to do next.

Scope, per docs/CLI-DESIGN.md "Commands", "IDs and defaults", "status", "log", "compare" and "pin": `after` and `after status` with the Next rule table; `after log [-n N]`; bare `inspect`, `compare`, `pin` (the list), `export`, and `pin --expectation` without a receipt, each resolving the design's defaults and naming them in readable output; and a Next block of at most three runnable commands on every result. Next blocks suggest only commands that exist; AFTER-24, AFTER-40 and AFTER-41 add theirs. Bare `review` and `run` are AFTER-24 and AFTER-40; until then, their missing-argument errors name a runnable example. Defaults read stored records only: they never capture, never execute, and never choose a pin revision.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 In a checkout, `after` and `after status` print the design's summary from stored records only and choose the Next block by the first matching rule; with no capture they suggest capturing, outside a checkout bare `after` prints short help, and `after status --json` returns the same facts.
- [x] #2 `after log` lists the 20 newest captures, runs, reports and pin events newest first with the design's columns, `-n N` changes the count, the footer says how many more exist, and `after log --json` returns the same rows with full IDs.
- [x] #3 Bare `inspect`, `compare`, `export` and `pin --expectation TEXT` resolve the newest capture, its newest receipt or comparison as the design's table says, print `Using …` naming each resolved record, and exit 2 naming the missing record and the command that creates it when none exists; bare `pin` lists pins by head revision.
- [x] #4 Every readable result ends with a Next block of at most three commands that exist, with suggested paths printable and single-quoted when needed; tests prove suggested commands run as written.
- [x] #5 Tests prove no default captures, runs, imports, or selects a pin revision, and that `--json` output names every resolved record by full ID; docs/CLI.md documents the bare forms.
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
1. Extend existing CLI command/rendering and bounded store-list APIs with stored-only status, newest-first event history, and newest-capture/pair receipt/comparison resolution; preserve explicit revision semantics and current review behavior.
2. Add full-ID resolved metadata to JSON, readable Using lines, and centralized context-preserving, shell-quoted Next suggestions using only shipped commands.
3. Add focused regression and real 80-column PTY/pipe checks including NO_COLOR, no implicit execution/capture/import or revision choice, missing/corrupt records, and executable suggestions; update CLI docs/help.
4. Independently verify acceptance and safety boundaries, fix concrete scoped defects, run relevant Task checks, commit implementation, fast-forward main, finalize authoritative task, and remove the verified owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation is present in owned after-37-defaults worktree at base 19f4a9e; no commits/integration yet. Executor completed source/tests/docs after a 30-minute timeout recovery; resumed child settlement failed with output-path ownership error after compaction, so independent verification was launched separately through the same subagent protocol. Executor reports task test:cli, task test, and final task check:go passed; first check:go saw a transient PTY review-resume failure, unchanged rerun passed. Parent LSP diagnostics are clean for all 12 changed/new Go files. Independent acceptance verification pending; do not mark Done until verified and delivered.

Delivered implementation f74cddf (Add stored CLI defaults and history), fast-forwarded into main after refreshing the primary claim. Single independent verification passed AC 2/4/5 and identified two concrete gaps: saved-review status precedence and equal-completion comparison ordering. Both fixed with regression assertions in TestBareDefaultsAreStoredOnlyAndNameResolvedRecords and TestLatestComparisonUsesCompletionThenComparisonID; no second general review. Authoritative task remained In Progress (verifier mistakenly reported the worktree copy as To Do).
Final parent checks: mise exec -- task test:cli PASS (CLI race tests 184.209s); mise exec -- task check:go PASS (build, all-package race tests, vet, gofmt; CLI 184.553s); mise exec -- task test PASS (race suite, 141 links across 25 docs, 45 backlog tasks); mise exec -- task check:staged PASS (format and secrets), repeated by commit hook. LSP diagnostics clean on affected Go files.
Terminal evidence: TestProjectCommandProcessModes runs actual 80x24 PTYs and pipes with NO_COLOR unset/set for new bare/status/log/inspect/compare/export/pin-list/default-pin forms and existing changed results. Executor and independent verifier both exercised it; final CLI suite includes it. Output excerpts: PASS TestProjectCommandProcessModes/pin/no-color=no/pty; PASS TestProjectCommandProcessModes/pin/no-color=yes/pipe. Readable assertions include No capture has been stored, Using the newest capture, Saved review, and Nothing has run for an executed-as-written run preview suggestion. JSON retains full IDs; shell suggestion tests use apostrophes/spaces in checkout/config paths and verify no receipt execution. Missing defaults and revision mutation guards are covered.
Limitations: comparison records lack timestamps; newest ordering uses bound receipt completion then comparison ID. Bare run/diff remain deferred; suggestions use explicit pairs and existing commands only. Initial executor full check had a transient existing PTY review-resume failure; later full runs passed. No Docker execution was required or newly authorized.
Cleanup: creation receipt matched this session, checkout and branch; fleet inactive; exact Herdr workspace contained only idle zsh. Worktrunk foreground removal deleted the clean merged worktree and branch, post-remove hook closed its workspace, absence verified. Pre-existing worktrees were not adopted or changed. No remaining blocker or resumable work for this item; no push.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added read-only status/history, pair-scoped stored defaults with full-ID JSON and Using lines, head-only pin listing, and shell-safe Next commands. Fixed both independent-verification findings and passed CLI/full Go/docs/backlog/staged checks including 80-column PTY/pipe color variants. Implementation f74cddf integrated into main; owned worktree/branch/workspace removed. Final task metadata is committed separately on main.
<!-- SECTION:FINAL_SUMMARY:END -->

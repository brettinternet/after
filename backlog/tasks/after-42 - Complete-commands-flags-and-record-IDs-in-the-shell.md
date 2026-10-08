---
id: AFTER-42
title: 'Complete commands, flags, and record IDs in the shell'
status: Done
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-08 14:28'
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
priority: low
type: feature
ordinal: 42000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Commands, flags and 71-character IDs must be typed in full; prefixes (AFTER-36) shorten IDs, but the reviewer still has to find them.

Scope, per docs/CLI-DESIGN.md "completion": `after completion [bash|zsh|fish]`, defaulting to `$SHELL`. Scripts complete commands, flags, flag values (`--scope`, `--mode`) and ID arguments. ID candidates are the 50 newest records valid for that argument, shown as short IDs with sanitized one-line descriptions. Completion reads the store read-only: it never creates `.after/` and never captures, imports or runs anything.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 `after completion` prints a script for `$SHELL`, `after completion bash|zsh|fish` prints that shell's script, and an unknown shell exits 2 listing the supported ones.
- [x] #2 In each shell, scripted tests complete commands, flags, `--scope` and `--mode` values, and ID arguments with only valid kinds, newest first, at most 50.
- [x] #3 Descriptions are sanitized and escaped for each shell's completion format; hostile tests with colons, quotes, newlines and escape sequences prove no injection or broken candidates.
- [x] #4 Completion opens the store read-only, creates nothing outside a store, and never captures, imports or runs; docs/CLI.md documents installation for each shell.
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
1. Extend the existing CLI grammar/help with completion script generation and an inert candidate endpoint, reusing command flag definitions and ID/store descriptions. 2. Add Bash, Zsh and Fish adapters with safe candidate transport; limit read-only ID lookup to the newest 50 valid records and bounded descriptions. 3. Test actual shell adapters, hostile descriptions, argument kind filtering and absent-store non-mutation; document installation and exercise terminal/pipe output. 4. Independently verify trust-boundary acceptance, run relevant Task checks, commit implementation, merge to main, finalize task metadata and remove the verified owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation is retained in owned .worktrees/after-42-completion (branch after-42-completion; creation receipt .git/worktrees/after-42-completion/agent-creation.json, parent session 01a11a1a-8b70-70e2-be64-ae1adae34e39). Executor timed out at 30 minutes during task test:cli after config passed; no test subprocess remains. Partial source/docs/tests preserved, not committed or accepted. Same executor resumed to diagnose stalled tests and finish bounded verification; independent verification still pending. No external blocker established.

Delivered 3fc032a (Add safe shell completion), fast-forwarded into main. Public Bash/Zsh/Fish scripts and inert backend reuse command flag definitions and existing ID descriptions/read-only store; tests cover actual adapters, valid kinds, newest-50 ordering, scope/mode, hostile text, absent-store nonmutation. Validation: mise exec -- task format:go; focused go test -race ./internal/cli -run ^TestCompletion -count=1 -timeout=90s; mise exec -- task check:go; mise exec -- task test; mise exec -- task check:staged all passed. Final full CLI race run took 318.491s. Earlier 120/180s timeouts were insufficient, not a demonstrated hang. First longer full run exposed missing completion in TestReadableCommandEnumeration; corrected expected command and script-only output exemption, then full build/race/vet/format rerun passed. Docs check: 142 links/25 files; backlog: 45 tasks. LSP completion.go clean. Independent verifier reviewed read-only/escaping boundaries and actual shells: PASS, no task-scoped defects. Parent Python pty.openpty+TIOCSWINSZ(24,80) verified all three explicit scripts in pipes and actual 80-column PTYs both with NO_COLOR=1 and unset, exit 0 and identical script payloads. Excerpts: Bash starts # AFTER Bash completion.; Zsh #compdef after; Fish # AFTER Fish completion. Default SHELL and unknown-shell exit2 covered by tests. Limitations documented: Bash Readline displays words without descriptions; legacy records lacking persisted event timestamps sort after dated records by stable ID. No Docker execution gate needed for this nonexecution command. Ownership receipt verified, both children inactive, Herdr list/panes had no matching worktree destination. Worktrunk foreground removal succeeded and deleted owned branch; workspace absence verified. Six pre-existing worktrees left untouched; none adopted. No blockers or resumable step remains for this item; no push authorized.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented read-only Bash, Zsh and Fish command/flag/value/ID completion and installation docs. Verified actual shells, hostile descriptions, nonmutation, full Go checks, and 80-column PTY/pipe output with and without NO_COLOR. Integrated implementation 3fc032a into main; owned worktree and branch removed. Independent verification passed.
<!-- SECTION:FINAL_SUMMARY:END -->

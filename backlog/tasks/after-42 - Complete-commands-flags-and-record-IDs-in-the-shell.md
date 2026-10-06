---
id: AFTER-42
title: 'Complete commands, flags, and record IDs in the shell'
status: To Do
assignee: []
created_date: '2026-10-06 21:14'
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
- [ ] #1 `after completion` prints a script for `$SHELL`, `after completion bash|zsh|fish` prints that shell's script, and an unknown shell exits 2 listing the supported ones.
- [ ] #2 In each shell, scripted tests complete commands, flags, `--scope` and `--mode` values, and ID arguments with only valid kinds, newest first, at most 50.
- [ ] #3 Descriptions are sanitized and escaped for each shell's completion format; hostile tests with colons, quotes, newlines and escape sequences prove no injection or broken candidates.
- [ ] #4 Completion opens the store read-only, creates nothing outside a store, and never captures, imports or runs; docs/CLI.md documents installation for each shell.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

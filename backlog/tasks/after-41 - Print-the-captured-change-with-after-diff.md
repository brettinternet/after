---
id: AFTER-41
title: Print the captured change with after diff
status: To Do
assignee: []
created_date: '2026-10-06 21:14'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-32
  - AFTER-37
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/RAW-DIFF.md
  - docs/TERMINAL.md
priority: medium
type: feature
ordinal: 41000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The captured patch is only available as base64 inside `inspect` JSON, so a reviewer cannot read or pipe a change from the shell.

Scope, per docs/CLI-DESIGN.md "diff": bare `after diff` prints the newest capture's patch, and `after diff BASE CANDIDATE` takes any two snapshots, using the AFTER-32 computed diff when they share no captured patch. Stdout carries only the patch. Stderr names the pair, the diff's origin, and the excluded, unsupported and unknown paths the patch doesn't cover, and says when sanitizing changed any byte. Lines are sanitized and colored on a terminal like the Diff view; tabs are kept in a pipe. `--raw` writes the exact bytes and is refused when stdout is a terminal. `--stat` prints the capture summary rows. There is no pager.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Bare `after diff` prints the newest capture's patch on stdout and the pair, origin and uncovered paths on stderr; `after diff BASE CANDIDATE` works for any two snapshots and labels a computed diff as such.
- [ ] #2 Output escapes control and format characters and invalid UTF-8 through `internal/terminal`, keeps tabs in a pipe, and colors only on a terminal without NO_COLOR; when sanitizing changed any byte, stderr says so and suggests `--raw`, and hostile-patch tests prove no raw control sequence reaches stdout.
- [ ] #3 `after diff --raw` into a file or pipe writes bytes identical to the stored patch, so `git apply` reproduces the candidate for an ordinary capture, and is refused with exit 2 when stdout is a terminal.
- [ ] #4 `--stat` prints the capture summary rows; the 100,000-line captured diff streams within the docs/TERMINAL.md memory budget.
- [ ] #5 docs/CLI.md and docs/RAW-DIFF.md document the command.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

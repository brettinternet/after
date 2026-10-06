---
id: AFTER-37
title: 'Make bare commands useful with status, log, resolved defaults and Next blocks'
status: To Do
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-06 21:14'
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
- [ ] #1 In a checkout, `after` and `after status` print the design's summary from stored records only and choose the Next block by the first matching rule; with no capture they suggest capturing, outside a checkout bare `after` prints short help, and `after status --json` returns the same facts.
- [ ] #2 `after log` lists the 20 newest captures, runs, reports and pin events newest first with the design's columns, `-n N` changes the count, and the footer says how many more exist.
- [ ] #3 Bare `inspect`, `compare`, `export` and `pin --expectation TEXT` resolve the newest capture, its newest receipt or comparison as the design's table says, print `Using …` naming each resolved record, and exit 2 naming the missing record and the command that creates it when none exists; bare `pin` lists pins by head revision.
- [ ] #4 Every readable result ends with a Next block of at most three commands that exist, with suggested paths printable and single-quoted when needed; tests prove suggested commands run as written.
- [ ] #5 Tests prove no default captures, runs, imports, or selects a pin revision, and that `--json` output names every resolved record by full ID; docs/CLI.md documents the bare forms.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

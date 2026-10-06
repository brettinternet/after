---
id: AFTER-38
title: 'Use one grammar, honest help, and errors that say how to fix them'
status: To Do
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-06 21:41'
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
- [ ] #1 Pairs are `BASE CANDIDATE` in `inspect`, `review` and `run`, and `--base` accepts only a Git ref; `after pin PIN` prints the pin, and `--accept`, `--attach RECEIPT` and `--select SNAPSHOT [--mode MODE]` record decisions with `--reason` optional and a default reason naming the command line, stored verbatim in history.
- [ ] #2 `after pin RECEIPT --expectation TEXT` defaults `--scope` to `finite_example`, and a receipt that cannot support it fails with an error suggesting `--scope human_intent`.
- [ ] #3 Each removed form (`review PIN …`, `inspect CANDIDATE --base BASE`, `review --tui …`) exits 2 with an error that shows the equivalent new command; unknown commands and flags suggest the closest match within two edits, or none.
- [ ] #4 Every error follows `after: <what is wrong> — <how to fix it>` with sanitized user input, missing arguments are named with a runnable example, and exit codes match today's for the same failures.
- [ ] #5 Each command's help lists only its own options with defaults read from the configuration defaults (a test fails if a printed default differs), shows global options in a short group, and shows Docker and run limits only on `run`; golden help files cover every command.
- [ ] #6 Examples, the demo, the study kit, the POC gate, Taskfile proofs, docs/CLI.md, docs/REVIEW.md, docs/DEMO.md and docs/EVALUATION.md use the new grammar; `task test`, `task examples`, `task demo:inspect` and `task study:check` pass, and Docker-gated proofs are rerun when Docker settings are available.
- [ ] #7 `after --help` and `after help` start with one sentence, group the commands as the design's top-level help does with the everyday commands first, and list the global options once; a golden file covers it.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

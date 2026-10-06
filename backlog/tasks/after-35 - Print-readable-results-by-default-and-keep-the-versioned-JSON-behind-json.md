---
id: AFTER-35
title: Print readable results by default and keep the versioned JSON behind --json
status: To Do
assignee: []
created_date: '2026-10-06 21:14'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-22
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/TUI-DESIGN.md
  - docs/TERMINAL.md
  - docs/DEMO.md
  - docs/EVALUATION.md
priority: high
type: enhancement
ordinal: 35000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Every command prints one line of versioned JSON: `after capture` prints 1.3 KB on one line, `inspect` returns the patch as base64, and no result says what happened in words (docs/CLI-DESIGN.md "Why"). Reviewers read these results; scripts can ask for JSON.

Scope, per docs/CLI-DESIGN.md "Output", "inspect" and the capture, compare, pin, import and run mockups: readable text on stdout by default, and `--json` prints exactly today's envelope. `export` always prints JSON. Each result is a sentence, then aligned rows or a list. Styling and badges come from the AFTER-22 theme on a terminal only. Untrusted text goes through `internal/terminal` and clips only on a terminal. IDs, times, counts and paths follow the TUI formatting rules. `inspect` follows the design's table; records whose Card arrives in AFTER-27 print their sentence and IDs, and plans arrive with AFTER-40. Next blocks are AFTER-37's. This supersedes the headless one-JSON-object rule, so this task moves every caller that parses output to `--json`: tests, `examples/`, `internal/demo` including the study kit, `internal/pocgate`, Taskfile proofs and docs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Every command except `export` prints readable text by default, and `--json` prints today's versioned envelope unchanged; a test enumerates the commands so none is missed.
- [ ] #2 Readable output follows the design's shape and formatting: a leading sentence, aligned rows, 8-character IDs, local times and counts; golden files in `internal/cli/testdata/` cover each command with a fixed clock, time zone and 80 columns, regenerated only with a test-only `-update` flag.
- [ ] #3 On a terminal, styling uses only theme SGR sequences and badges; NO_COLOR, `TERM=dumb` or a pipe produces no escape sequences, rows clip only on a terminal, and hostile paths, test names, expectations, producers and errors are sanitized in every command's output.
- [ ] #4 `after inspect` prints a pair's capture summary rows, a snapshot's source, capture times and path count, an artifact's kind, size and content through the content viewer rules, and a sentence plus an IDs section for receipts, comparisons, pin revisions and reports, with full IDs one per line and never clipped.
- [ ] #5 Every caller that parses output passes `--json`; `task test`, `task examples`, `task demo:inspect` and `task study:check` pass, Docker-gated proofs are rerun when Docker settings are available, and docs/CLI.md documents readable output and `--json`.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

---
id: AFTER-40
title: 'Show a readable run consent, store plans privately, and guide Docker setup'
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
- [ ] #1 On a terminal, bare `after run` names the newest capture, prints the consent summary rows from the exact plan bytes and the plan's size, and runs only after `yes`; any other answer runs nothing, and a summary decode failure says so while consent still binds the exact bytes.
- [ ] #2 Without a terminal, `after run` stores the plan in `.after/` with mode 0600, prints the exact `after run --approve sha256:…` command with the full digest, and exits 3; the stored file is never overwritten.
- [ ] #3 `after run --approve DIGEST` runs only a stored plan whose reconstruction matches its bytes and digest, refuses a prefix, mismatch or modified file with nothing run, and `--plan-out`/`--plan-file` behave as today; tests cover each refusal.
- [ ] #4 Missing or invalid Docker settings are named with the configuration lines to add, may suggest a detected Docker CLI path and existing socket file, and tests prove no Docker contact or endpoint selection; `after config` lists the same setup problems.
- [ ] #5 During an approved run, stderr on a terminal shows elapsed time with no percentage; `after inspect PLAN` prints the summary, then the indented, sanitized plan; docs/CLI.md and docs/RUNNER.md document the flow.
- [ ] #6 `after status` suggests `after run` by the design's rule, and Next blocks offer it where the design shows it.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

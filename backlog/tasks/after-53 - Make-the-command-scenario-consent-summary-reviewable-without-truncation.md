---
id: AFTER-53
title: Make the command-scenario consent summary reviewable without truncation
status: To Do
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 14:48'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-50
documentation:
  - docs/RUNNER.md
priority: medium
type: enhancement
ordinal: 53000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The terminal consent summary for a command definition prints each field as one raw JSON line clipped with "…" (definition, build argv, cases, stdin digests, Docker policy). An operator cannot see which argv, stdin, environment or input files will run before typing yes without separately running `after inspect PLAN`. The payment consent was made readable in AFTER-29/AFTER-40; command plans (and http-service definitions) did not get the same treatment. Consent is only meaningful if the exact plan is reviewable on the screen that asks for it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The consent summary lists each command case with id, title, argv, environment, input-file paths/sizes and stdin size plus a short safe preview, without silent truncation of any field that differs between cases
- [ ] #2 Image/platform, build argv, comparison modes, Docker policy (including --interactive) and limits are readable at 80 columns; anything elided says how to see it in full (after inspect PLAN)
- [ ] #3 Hostile definition text (terminal controls, very long argv) stays sanitized and bounded; consent still binds exact plan bytes
- [ ] #4 Consent view goldens cover a multi-case command definition at 80x24 and 120x40
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [ ] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

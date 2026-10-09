---
id: AFTER-56
title: Add examples for JUnit import and an explicit http-service definition
status: To Do
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 14:48'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-48
  - AFTER-49
documentation:
  - examples/README.md
  - docs/JUNIT-REPORTS.md
priority: low
type: docs
ordinal: 56000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
examples/ now shows the command scenario kind (07) but not the other two AFTER-48/49 features: importing a JUnit XML report from a non-Go producer, and running an operator-selected `http-service` definition with `--definition`. New users reading examples/README.md cannot discover either. Imported reports must come from a real producer run, not a canned fixture presented as fresh output, and examples must not download dependencies implicitly.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An example imports JUnit XML produced live by a named producer available through mise (or explains the explicit toolchain prerequisite) and shows reported-only cards beside the diff
- [ ] #2 A Docker example runs a non-payment http-service definition selected with --definition on its separately provisioned pinned image and shows a changed and an equal case
- [ ] #3 examples/README.md lists both; task examples smoke-tests whichever needs no Docker
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [ ] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

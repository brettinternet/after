---
id: AFTER-19
title: Add bounded seeded boundary exploration for the proven driver
status: To Do
assignee: []
created_date: '2026-10-03 05:44'
labels:
  - follow-up
  - exploration
milestone: m-2
dependencies:
  - AFTER-18
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: medium
type: feature
ordinal: 19000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
After the local loop proves useful, explore the shape of one real difference rather than creating a universal simulator. Scope: seeded generation around the existing controlled-clock payment boundary, reusing receipts, authorization and the TUI.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An explicit continue decision from AFTER-18 and operator selection is recorded before implementation; the POC does not automatically expand into this task.
- [ ] #2 The existing payment driver generates valid concrete cases just before/at/after both retention thresholds with a documented epsilon and stable seed/tool version; case count, runtime and concurrency are bounded.
- [ ] #3 Each measured case uses fresh state, frozen driver/observer/rules and digest-bound authorization; generation/proposal alone never produces an observed value.
- [ ] #4 The TUI distinguishes proposed/unrun inputs from measured cases and retains per-case witnesses/limits; no interpolation implies all values between samples were executed.
- [ ] #5 Reproduction stores concrete inputs as well as seed and generator version; unit/integration tests reproduce boundary divergences and unchanged controls without an LLM.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks pass and exact commands, actual results or objective external blockers are recorded in task notes.
- [ ] #2 Update affected docs and limitations; do not claim unrun experiments or human validation.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; keep sensitive/private artifacts out of Git.
<!-- DOD:END -->

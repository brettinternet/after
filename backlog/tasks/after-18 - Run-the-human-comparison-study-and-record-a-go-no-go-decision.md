---
id: AFTER-18
title: Run the human comparison study and record a go/no-go decision
status: To Do
assignee: []
created_date: '2026-10-03 05:44'
updated_date: '2026-10-06 20:30'
labels:
  - follow-up
  - evaluation
  - human-required
milestone: m-2
dependencies:
  - AFTER-17
  - AFTER-27
  - AFTER-29
  - AFTER-32
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: medium
type: task
ordinal: 18000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
This task is deliberately outside autonomous POC completion. Only real engineers can establish whether review reconstruction improves. An agent may prepare/analyze authorized anonymized data, but cannot recruit without permission or impersonate participants. Human input is an external prerequisite in addition to AFTER-17.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The operator explicitly authorizes the study and supplies recruited participants/scheduling; if absent, the task stays incomplete with the exact objective unblock condition, never simulated participant results.
- [ ] #2 Real participants complete counterbalanced raw-diff, guided-tour and AFTER tasks using the kit; anonymized observations record the defined time, correctness, missed-defect, trust and setup measures.
- [ ] #3 Results disclose participant count, exclusions, ordering effects and uncertainty; small samples or missing sessions do not become a statistically supported 30 percent benefit claim.
- [ ] #4 A written decision compares evidence to guardrails and chooses continue, narrow, integrate or stop; regressions in defect detection or silent retained freshness cannot be excused by faster clicks.
- [ ] #5 Only an explicit continue decision and new operator selection permits the exploration follow-ups; GitHub/browser scope is not automatically authorized.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks pass and exact commands, actual results or objective external blockers are recorded in task notes.
- [ ] #2 Update affected docs and limitations; do not claim unrun experiments or human validation.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; keep sensitive/private artifacts out of Git.
<!-- DOD:END -->

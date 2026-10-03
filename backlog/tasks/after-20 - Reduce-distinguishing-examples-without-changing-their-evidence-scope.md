---
id: AFTER-20
title: Reduce distinguishing examples without changing their evidence scope
status: To Do
assignee: []
created_date: '2026-10-03 05:44'
labels:
  - follow-up
  - exploration
milestone: m-2
dependencies:
  - AFTER-19
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: medium
type: feature
ordinal: 20000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Long scenarios are hard to understand even when the engine finds a difference. Scope: reduction over the one supported generated HTTP scenario family, not a general program minimizer or counterfactual patch engine.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A bounded reducer takes a measured distinguishing input/sequence and tests simpler valid candidates through the same paired runner, preserving the selected side-effect divergence.
- [ ] #2 Every reduction run obeys authorization, fresh-state, time/case budgets and provenance rules; timeout/cancel leaves the last verified witness and no orphan processes.
- [ ] #3 Both original and reduced concrete cases plus generator/reducer versions and receipts are saved; seed alone is never required to recover the exact witness.
- [ ] #4 The inspector calls it a reduced example, not a globally minimal case or proven root cause; unexecuted simplifications remain proposals.
- [ ] #5 Tests include reducible/irreducible sequences, unstable outcomes, incompatible versions and reductions that accidentally remove the effect; none can yield a falsely verified witness.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks pass and exact commands, actual results or objective external blockers are recorded in task notes.
- [ ] #2 Update affected docs and limitations; do not claim unrun experiments or human validation.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; keep sensitive/private artifacts out of Git.
<!-- DOD:END -->

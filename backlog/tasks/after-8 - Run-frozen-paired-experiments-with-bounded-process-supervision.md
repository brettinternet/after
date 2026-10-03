---
id: AFTER-8
title: Run frozen paired experiments with bounded process supervision
status: To Do
assignee: []
created_date: '2026-10-03 05:42'
labels:
  - poc
  - runner
milestone: m-0
dependencies:
  - AFTER-5
  - AFTER-6
  - AFTER-7
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 8000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The central evidence needs actual paired execution, not trust in a candidate's own tests. Scope: orchestration using the sandbox and fixture already proven by AFTER-6/7, immutable receipts and process-lifecycle tests. Do not introduce arbitrary shell command runners or baseline reuse before matching all bindings.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An authorized plan runs the exact same frozen scenario/driver/observer/rules against two captured versions, using independent fresh state and recording actual argv, image/toolchain/dependency identities, inputs, timestamps and completion on each side.
- [ ] #2 Real response and fake-provider artifacts persist through the store; a candidate-edited test or mask cannot replace the frozen oracle. Candidate-owned suite results remain separate and identified.
- [ ] #3 Timeout, denied permission, startup/build failure, incompatible driver, missing channel, cancellation and output truncation produce explicit partial/failure states, never a behavioral pass or fabricated baseline.
- [ ] #4 Snapshot/request IDs bind every result from submission to storage; barrier-controlled tests change the selected snapshot before completion and prove late output remains attached to the old pair.
- [ ] #5 Repeated runs retain every sample; mismatching repetitions surface instability. The executor bounds concurrency and drains stdout/stderr without deadlock or unlimited memory.
- [ ] #6 Cancellation and failure paths leave no owned child/container/service running, never stop unrelated processes, and retain bounded diagnostics without secrets. Builds do not escape the same authorization/isolation boundary.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

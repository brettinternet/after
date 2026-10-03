---
id: AFTER-11
title: Persist expectations and conservatively reopen changed evidence
status: To Do
assignee: []
created_date: '2026-10-03 05:42'
labels:
  - poc
  - review
  - storage
milestone: m-0
dependencies:
  - AFTER-10
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 11000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Review memory is the product hypothesis and its most dangerous trust failure. Scope: persistent pins, applicability derivation and explicit snapshot transition APIs. Start with broad content invalidation; do not invent a dependency graph to reduce reruns.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A user can pin a concrete scenario/result with its receipt basis and an explicit scope; broader requirements are saved as human intent, not established by a single example.
- [ ] #2 Pin state and history survive process restart; human decisions, immutable observations and applicability are independent. Fresh execution alone cannot auto-accept a reopened expectation.
- [ ] #3 Parameterized tests change code, fixture, driver, observer, runtime, dependency, environment and comparison rule/mask identities; old evidence becomes stale or unknown and the original expectation remains unchanged.
- [ ] #4 Unknown/incomplete dependency footprints fail closed at the whole-project boundary; a harmless edit may reopen and is explained. Identical complete bindings permit bounded reuse without asserting universal equivalence.
- [ ] #5 Accepting a new snapshot produces an explicit reason and missing-current-result state; it cannot fill in a predicted before/after value. Authorized rerun appends a new bound receipt and preserves old history.
- [ ] #6 Original-base comparison and last-inspected-to-latest follow-up remain distinguishable; changed base/renamed test/scenario title alone never migrates acceptance or freshness.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

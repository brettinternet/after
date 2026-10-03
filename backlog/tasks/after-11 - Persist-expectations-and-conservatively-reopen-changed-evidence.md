---
id: AFTER-11
title: Persist expectations and conservatively reopen changed evidence
status: Done
assignee: []
created_date: '2026-10-03 05:42'
updated_date: '2026-10-03 20:11'
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
- [x] #1 A user can pin a concrete scenario/result with its receipt basis and an explicit scope; broader requirements are saved as human intent, not established by a single example.
- [x] #2 Pin state and history survive process restart; human decisions, immutable observations and applicability are independent. Fresh execution alone cannot auto-accept a reopened expectation.
- [x] #3 Parameterized tests change code, fixture, driver, observer, runtime, dependency, environment and comparison rule/mask identities; old evidence becomes stale or unknown and the original expectation remains unchanged.
- [x] #4 Unknown/incomplete dependency footprints fail closed at the whole-project boundary; a harmless edit may reopen and is explained. Identical complete bindings permit bounded reuse without asserting universal equivalence.
- [x] #5 Accepting a new snapshot produces an explicit reason and missing-current-result state; it cannot fill in a predicted before/after value. Authorized rerun appends a new bound receipt and preserves old history.
- [x] #6 Original-base comparison and last-inspected-to-latest follow-up remain distinguishable; changed base/renamed test/scenario title alone never migrates acceptance or freshness.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend existing immutable Pin records with explicit finite-example versus human-intent scope and bounded, snapshot-bound review history; preserve legacy records as unknown until explicitly repinned.
2. Add a small review API for pin/create, inspect, explicit original-base or last-inspected snapshot transitions, receipt attachment, and separate human acceptance. Derive applicability from exact whole-project snapshots, frozen scenario/rules and both runtime/dependency/environment bindings; incomplete footprints fail closed.
3. Expose bounded headless pin/review actions using the existing store and frozen runner preview (no execution on review). Add parameterized invalidation, restart/history, no-prediction, late-result, and CLI tests.
4. Run focused and Go/Task checks, obtain one independent trust-boundary review, fix concrete defects, document limitations, commit and integrate into main, then finalize the authoritative task.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented immutable scoped pin revisions and explicit select/attach/accept APIs and CLI actions. task check:go passed build, all race tests, vet and gofmt; initial compile failure from an old unkeyed DecisionEvent test literal was fixed. Parameterized tests cover code, harmless edits, fixture, driver, observer, runtime, dependencies, environment, argv, rules/masks, names, incomplete/excluded/unknown footprints; subprocess restart preserves history. task cli:proof passed in 370.75s against the provisioned offline Docker image: real 1-versus-2 provider effects with equal responses and 30s control, then native pin/reopen/authorized rerun/attach/separate acceptance across process restarts. Independent acceptance/trust-boundary verification is running. No host execution fallback, network provisioning, or private receipts added to Git.

Operator explicitly handed off incomplete work. Reused the staged after-11-pins checkout without discarding any changes. Reran mise exec -- task check:go (build/race tests/vet/gofmt), task test (77 local links and 20-task graph), and task check:staged (format/secrets): all passed. Read the inherited cli:proof log and confirmed TestPaymentCLIProof PASS in 370.75s including the native pin/reopen/rerun/attach/separate-accept flow. A fresh independent acceptance verifier is checking the staged implementation before integration. Cleanup will retain the inherited checkout because original-session cleanup ownership cannot be fully verified under the transcript access boundary.

Final takeover verification: mise exec -- task check:go, mise exec -- task test, and mise exec -- task check:staged all passed. Fresh mise exec -- task cli:proof with the explicitly configured local Docker CLI/socket passed TestPaymentCLIProof in 375.54s: actual equal HTTP responses with 1-versus-2 provider effects, control, and pin/reopen/authorized-rerun/attach/separate-accept across native restarts. Independent verifier 6f9156a2 passed all six acceptance criteria with no concrete defects and independently ran task check:go and task test:cli. Its primary backlog read was sandbox-blocked; parent confirmed its reviewed criteria match the authoritative task. Implementation c6624fc fast-forwarded onto main. No private receipts or credentials committed. Retain inherited .worktrees/after-11-pins: cleanup ownership continuity was not independently established; other pre-existing worktrees remain untouched. No remaining implementation blocker or resumable step for AFTER-11; later TUI integration is AFTER-14.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered persistent scoped expectations with immutable review history, conservative whole-project invalidation, explicit original-base/follow-up selection, and separate receipt attachment and human acceptance. CLI and documentation included. Verified by race tests/build/vet/format, link/backlog and staged checks, fresh real Docker CLI proof, and independent six-criterion verification. Implementation c6624fc integrated into main; final task metadata committed separately.
<!-- SECTION:FINAL_SUMMARY:END -->

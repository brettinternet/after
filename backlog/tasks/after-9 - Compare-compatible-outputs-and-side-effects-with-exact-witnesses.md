---
id: AFTER-9
title: Compare compatible outputs and side effects with exact witnesses
status: Done
assignee: []
created_date: '2026-10-03 05:42'
updated_date: '2026-10-03 18:17'
labels:
  - poc
  - comparison
milestone: m-0
dependencies:
  - AFTER-8
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 9000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The engine must detect consequential effects and communicate precise limits without an LLM. Scope: typed deterministic comparison of the two supported channels and receipt-backed witness objects. No fuzzy natural-language equality, clustering platform or universal semantic comparator.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Structured JSON differences identify added/removed/changed paths and preserve number precision, null versus missing and array ordering; object key order alone is not a difference.
- [x] #2 Fake-provider observations compare operation, destination, payload, count and ordering under an explicit versioned rule; identical HTTP responses cannot hide one-versus-two provider requests.
- [x] #3 Only matching frozen scenario/observer/rules and compatible recorded environments/channels compare; missing/incomplete/incompatible channels yield explicit limits or incomparable state, never complete equality.
- [x] #4 Every changed mask/normalization policy has a new digest and reviewable diff; redacted/truncated fields cannot silently prove equal. Tests cover malicious or overbroad normalization rules.
- [x] #5 Disagreeing repetitions yield unstable with all samples inspectable; finite examples use scoped templates rather than claims of universal safety, causation or performance.
- [x] #6 Golden/property tests cover deterministic ordering, no-data/new-capability cases, controlled failures and exact numeric/Unicode values; witnesses link to stored artifacts and source inventories.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add bounded exact JSON differences and a versioned, immutable payment comparison policy; reject masks and ambiguous JSON rather than normalize evidence away.
2. Verify stored runner plans, frozen bindings, source inventories and environments; compare every response/provider sample with precise artifact-linked witnesses and conservative incomplete/unstable outcomes. Persist details as bounded artifacts using existing comparison records.
3. Add deterministic golden/property, failure, compatibility and receipt integration tests; document finite scope and limits. Obtain one independent trust-boundary review, run relevant Task checks, integrate implementation and commit final metadata on main.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation is in session-owned .worktrees/after-9-comparison (creation receipt .git/worktrees/after-9-comparison/agent-creation.json). Added exact bounded JSON and payment channel comparison, immutable versioned policy artifacts, reconstructed frozen-plan/environment verification, artifact/source-linked witnesses and conservative incomplete/unstable results. Policy masks and changed/unknown bindings fail closed. task check:go passed; comparison:fuzz passed 741931 executions; actual comparison:proof passed in 175.87s on local Colima Linux aarch64 with two repetitions, equal responses, 12h counts 1/2 and 30s 1/1, 16 stored channel witnesses. task check passed build/race/vet/format, 61 local links, 20-task graph and secret scan. Independent read-only trust-boundary review 63c0c7f4-97ab-40d2-8198-0fe5ad2cf8f6 is in progress; final integration/commit pending. No private receipts or credentials are intended for commit.

Resumed the existing implementation after explicit takeover; preserved original worktree receipt and recorded adoption in Git-local continuation state. The single independent review completed with two concrete P1 findings: sample execution-plan identities were unchecked, and omitted/null observation fields decoded as empty values. Fixed both with per-side/case app and observer plan matching and required typed non-null observation fields. Added TestSampleExecutionPlanBinding and TestObservationRequiredFields, including positive explicit empty/zero cases. No second general review was needed: corrections preserve the design. After fixes, mise exec -- task check:go passed all race tests/build/vet/format; affected Go LSP diagnostics were clean. The actual mise exec -- task comparison:proof passed again in 177.00s: two repetitions, equal responses, 12h provider counts 1/2, 30s 1/1, 16 linked witnesses. Earlier resumed comparison:fuzz passed 140700 executions; final staged checks and integration follow.

Delivery: implementation c8c2caf was fast-forwarded onto main with authoritative task edits preserved. Final mise exec -- task check and task check:staged passed (race tests, build, vet, formatting, 61 links, 20-task graph and secret scan). Acceptance evidence: TestJSONGolden/ExactAndInvalid/Properties for AC1; TestPaymentAndDeterminism, TestRepetitionsAndExactPayloads and live comparison:proof for AC2/5/6; TestIncomparable, TestSampleExecutionPlanBinding and TestObservationRequiredFields for AC3; TestPolicyDigestAndReviewableDiff and TestNewRedactionPolicyCannotPublishEquality for AC4. Both independent review findings are corrected and regression-tested. Owned after-9-comparison worktree and branch removed through Worktrunk; its Herdr workspace disappearance verified. Pre-existing after-2-storage and after-7-payment checkouts/workspaces were not adopted or altered. No remaining AFTER-9 blocker or resumable step; CLI integration is the separately scoped AFTER-10. Final task-state commit is this metadata delivery; no push authorized.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented deterministic, bounded JSON and payment-channel comparisons with exact stored witnesses, frozen policy/environment/sample-plan validation, conservative missing/redacted/unstable outcomes and inspectable source/artifact bindings. Integrated c8c2caf on main. Full Task checks, fuzzing and real two-repetition Docker proof passed; two independent review findings fixed with regressions. Scope and unsupported normalization limits documented in docs/COMPARISON.md. All six acceptance criteria satisfied; no outstanding blocker.
<!-- SECTION:FINAL_SUMMARY:END -->

---
id: AFTER-2
title: Persist immutable captures and receipts safely
status: Done
assignee: []
created_date: '2026-10-03 05:40'
updated_date: '2026-10-03 15:34'
labels:
  - poc
  - storage
  - security
milestone: m-0
dependencies:
  - AFTER-1
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 2000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Review history must survive another agent edit and a process crash without trusting repository-controlled paths or badges. Scope: the core local store and its tests, using private .after manifests and content-addressed bounded artifacts. Keep one writer; no database or multi-process synchronization framework.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Snapshots, scenarios, receipts, comparisons and pins can be persisted and reopened with stable content identities; immutable records cannot be overwritten under the same ID with different content.
- [x] #2 Atomic-write/crash-injection tests leave the last complete record readable; corrupt, truncated, unknown-version and missing-artifact records produce explicit errors without erasing valid history.
- [x] #3 Artifact resolution rejects traversal, absolute-path escape and symlink escape; files/directories use private permissions and byte/count limits, with clear disk-full and permission errors.
- [x] #4 Mutating operations reject a second writer; interrupted-owner recovery is documented and tested without stealing a live lock or deleting unrelated paths.
- [x] #5 Redaction policy and truncation are preserved as incompleteness metadata; sensitive fields are redacted before persistence, raw secrets are not logged, and partial artifacts cannot claim complete equality.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend the existing evidence contracts only as needed for persisted comparisons and redaction metadata; add an ordinary standard-library local store with stable content identities and bounded private artifacts. 2. Enforce immutable atomic publication, confined no-symlink resolution and one live writer with documented interrupted-owner recovery. 3. Add focused round-trip, corruption, fault-injection, limits, lock and secret-redaction tests; update storage/schema documentation. 4. Run relevant Task checks and one independent acceptance verification focused on storage escape, crash safety and false completeness. Correct concrete defects, integrate the tested implementation into main, commit authoritative task completion and clean up the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator explicitly handed the existing AFTER-2 claim to the successor session. The prior detached implementation child timed out waiting for parent schema decisions and left the implementation checkout clean. Resume the existing worker with a minimal receipt-bound Comparison record and explicit in-memory literal-secret redaction before truncation; persist only safe policy identifiers and incompleteness metadata. No acceptance criteria verified yet.

Current session adopted the existing clean after-2-storage checkout under the operator takeover instruction, verified the prior child failed with process terminal observed and no active capacity, and rebased the branch onto main dc4f84a. Resuming the existing worker with settled comparison/redaction decisions; one fresh-context executable acceptance verification follows before parent integration. No criteria claimed complete yet.

Operator explicitly authorizes takeover of incomplete work. This session reuses the clean after-2-storage checkout at dc4f84a, preserving its original creation receipt. Historical session evidence is outside the permitted shell scope, so no destructive cleanup authority is assumed. Continue the recorded implementation plan; final integration and task metadata commits remain parent-owned.

Successor session 01a10251-287f-71dc-9cb9-00e0e82531c1 takes over under explicit operator authorization; reused clean after-2-storage at dc4f84a without altering its original creation receipt. Bounded implementation then one independent executable acceptance verification are running in workflow ef0e658d-4fb7-48f8-8a1a-4c64135f83cc. Settled locking direction: permanent descriptor-held nonblocking OS lock, no stale PID deletion; literal-secret redaction precedes persistence/truncation. Parent owns integration and final metadata. Historical ownership evidence remains insufficient for destructive cleanup, so retain checkout unless verified.

Operator handed incomplete work to this session. Reusing the existing clean implementation checkout, preserving its original receipt; no destructive cleanup is assumed. Implement directly, then obtain one independent executable verification of confinement, crash safety, locking and redaction. No acceptance criteria verified yet.

Implemented the bounded immutable store, receipt-bound comparison record and explicit literal-v1 redaction metadata in the implementation checkout. Passed mise exec -- task check:go (build, race tests, vet, formatting) and mise exec -- task test (Go, documentation links, backlog graph). Independent executable acceptance verification is running as workflow 57bcdce2-5718-444e-9ea6-961bbdb1b2f3. No completion claimed before that result and integration.

Operator authorizes takeover. Resume existing staged implementation and recover the prior independent verification result before final checks and integration; preserve original worktree receipt and retain checkout if cleanup ownership remains unverified.

Operator-authorized takeover: prior verification workflow 57bcdce2 stopped because its extension session was replaced/reloaded; child resume unavailable in active session. Preserved staged implementation diff before same-protocol replacement verification. Reusing after-2-storage at dc4f84a; no cleanup ownership assumed.

Operator-authorized takeover: reusing staged after-2-storage implementation; verify prior acceptance evidence or run bounded independent verification, then integrate and commit on main. Preserve original receipt; no destructive cleanup authority assumed.

Takeover validation: implementation staged checks passed (mise exec -- task check:staged, formatting and secret scan). Independent acceptance verifier a4907847-084f-45d5-9dda-623c03aaf0d4 is checking the existing implementation and running check:go/test. Resume by consuming that exact result, fixing only concrete failures, committing implementation in after-2-storage, fast-forwarding main, and committing final provider metadata. No push authorized; retain prior checkout if cleanup provenance remains insufficient.

Operator-authorized takeover: existing independent verifier a4907847-084f-45d5-9dda-623c03aaf0d4 is still running, so consume its result rather than duplicate review. Preserve staged implementation and original checkout receipt; integrate onto main and commit after verification.

Delivered implementation commit 56cb6ce via fast-forward onto main. Independent verifier a4907847-084f-45d5-9dda-623c03aaf0d4 completed with PASS on all five criteria and no concrete findings. Parent reran mise exec -- task check:go and mise exec -- task test on main: build, race tests, vet, formatting, 36 documentation links and 20-task integrity passed. mise exec -- task check:staged passed formatting and secret scanning before implementation commit; final metadata receives the same gate. Tests exercise five-record reopen/identity, publication fault boundaries, corruption/version/missing references, confinement/permissions/limits, killed-writer recovery, redaction and false completeness. docs/STORAGE.md records same-user threat limits, filesystem durability limits and explicit literal-secret policy. No external blocker or resumable implementation remains. Inherited .worktrees/after-2-storage and its workspace are retained because historical cleanup ownership remains unverified; no new checkout created. No push performed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented bounded private content-addressed evidence storage, immutable atomic publication, single-writer recovery and explicit redaction/incompleteness metadata. Delivered as 56cb6ce on main. check:go, test and check:staged passed; independent acceptance verification passed all criteria. Final task metadata is committed separately on main. CLI remains help/version only; storage is a library.
<!-- SECTION:FINAL_SUMMARY:END -->

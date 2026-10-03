---
id: AFTER-2
title: Persist immutable captures and receipts safely
status: To Do
assignee: []
created_date: '2026-10-03 05:40'
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
- [ ] #1 Snapshots, scenarios, receipts, comparisons and pins can be persisted and reopened with stable content identities; immutable records cannot be overwritten under the same ID with different content.
- [ ] #2 Atomic-write/crash-injection tests leave the last complete record readable; corrupt, truncated, unknown-version and missing-artifact records produce explicit errors without erasing valid history.
- [ ] #3 Artifact resolution rejects traversal, absolute-path escape and symlink escape; files/directories use private permissions and byte/count limits, with clear disk-full and permission errors.
- [ ] #4 Mutating operations reject a second writer; interrupted-owner recovery is documented and tested without stealing a live lock or deleting unrelated paths.
- [ ] #5 Redaction policy and truncation are preserved as incompleteness metadata; sensitive fields are redacted before persistence, raw secrets are not logged, and partial artifacts cannot claim complete equality.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

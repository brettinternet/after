---
id: AFTER-9
title: Compare compatible outputs and side effects with exact witnesses
status: To Do
assignee: []
created_date: '2026-10-03 05:42'
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
- [ ] #1 Structured JSON differences identify added/removed/changed paths and preserve number precision, null versus missing and array ordering; object key order alone is not a difference.
- [ ] #2 Fake-provider observations compare operation, destination, payload, count and ordering under an explicit versioned rule; identical HTTP responses cannot hide one-versus-two provider requests.
- [ ] #3 Only matching frozen scenario/observer/rules and compatible recorded environments/channels compare; missing/incomplete/incompatible channels yield explicit limits or incomparable state, never complete equality.
- [ ] #4 Every changed mask/normalization policy has a new digest and reviewable diff; redacted/truncated fields cannot silently prove equal. Tests cover malicious or overbroad normalization rules.
- [ ] #5 Disagreeing repetitions yield unstable with all samples inspectable; finite examples use scoped templates rather than claims of universal safety, causation or performance.
- [ ] #6 Golden/property tests cover deterministic ordering, no-data/new-capability cases, controlled failures and exact numeric/Unicode values; witnesses link to stored artifacts and source inventories.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

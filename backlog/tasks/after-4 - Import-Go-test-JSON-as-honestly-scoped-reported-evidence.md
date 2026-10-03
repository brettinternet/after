---
id: AFTER-4
title: Import Go test JSON as honestly scoped reported evidence
status: To Do
assignee: []
created_date: '2026-10-03 05:40'
labels:
  - poc
  - import
milestone: m-0
dependencies:
  - AFTER-1
  - AFTER-2
  - AFTER-3
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 4000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
First use must be useful without granting execution permission. Scope: one stock go test -json importer, fixtures from the pinned Go toolchain and report cards. Test status/output is not a reconstruction of request inputs or side effects; do not add custom language instrumentation.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Versioned compatibility documentation and real captured fixtures cover passing/failing/skipped tests, subtests, package/build failures, interleaved events and empty reports.
- [ ] #2 An imported report retains producer, original artifact digest, capture/import times where available and explicit supplied snapshot binding; unverified provenance/applicability is reported or unknown, never upgraded to AFTER-observed execution.
- [ ] #3 Missing inputs, expected values and effects are explicitly unavailable; parsing a descriptive test name or free-form output cannot generate a runtime observation or a trusted badge.
- [ ] #4 Malformed/truncated JSON lines, unsupported event forms, duplicate test names across packages and size limits yield bounded diagnostics without dropping valid unrelated cases silently.
- [ ] #5 Import cannot execute code, fetch URLs or follow artifact paths; hostile terminal text stays data. Import-only tests succeed without Docker, model credentials, network or a usable project build.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

---
id: AFTER-4
title: Import Go test JSON as honestly scoped reported evidence
status: Done
assignee: []
created_date: '2026-10-03 05:40'
updated_date: '2026-10-03 16:03'
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
- [x] #1 Versioned compatibility documentation and real captured fixtures cover passing/failing/skipped tests, subtests, package/build failures, interleaved events and empty reports.
- [x] #2 An imported report retains producer, original artifact digest, capture/import times where available and explicit supplied snapshot binding; unverified provenance/applicability is reported or unknown, never upgraded to AFTER-observed execution.
- [x] #3 Missing inputs, expected values and effects are explicitly unavailable; parsing a descriptive test name or free-form output cannot generate a runtime observation or a trusted badge.
- [x] #4 Malformed/truncated JSON lines, unsupported event forms, duplicate test names across packages and size limits yield bounded diagnostics without dropping valid unrelated cases silently.
- [x] #5 Import cannot execute code, fetch URLs or follow artifact paths; hostile terminal text stays data. Import-only tests succeed without Docker, model credentials, network or a usable project build.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a bounded reader-only Go JSON importer with package/test report cards, explicit unavailable channels, provenance and unknown applicability; retain original digest and supplied binding without inventing runner receipts. 2. Capture pinned-toolchain fixtures and test malformed, interleaved, hostile and bounded inputs without execution dependencies. 3. Document the versioned dialect and limits; run Task checks and independent trust-boundary verification; commit implementation then final task metadata on main as explicitly requested.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented reader-only go-test-json-v1 report cards with raw SHA-256/provenance, caller-supplied snapshot binding, reported/unknown/not_run state and explicit unavailable inputs/expectations/effects. Captured real Go 1.27.1 fixtures with Task target; initial Task shell exited on expected test failure, corrected conditional exit handling. Initial empty-report command produced setup-failure JSON; replaced with invalid-timeout invocation yielding genuinely empty stdout. task check:go, task test and task format:check pass; affected LSP diagnostics clean. Independent verifier running. User explicitly requested implementation/commit on main; no implementation worktree created or old worktree adopted.

Operator explicitly handed off incomplete work on main. Resuming existing staged AFTER-4 implementation; verifying acceptance and prior review evidence before committing. No other checkout adopted.

Delivered on main in b963986. Verification: mise exec -- task check:go passed build, race tests, vet and gofmt; mise exec -- task test passed Go tests, 42 local links and 20-task integrity; mise exec -- task format:check passed; mise exec -- task check:staged passed formatting and secret scan before implementation commit. TestCapturedFixtures covers real pinned-Go statuses, interleaving and empty/build reports; provenance assertions and metadata tests cover digest/times/binding; malformed/repetition/bounds tests cover recovery; TestImportOnlyHostileData passes with unusable PATH/Docker and module networking off. Prior-session independent verifier returned PASS after inspecting trust boundaries and running Task checks; no concrete defects reported. No remaining task blocker. CLI/private-store integration remains assigned to AFTER-10; strings require sanitization by future renderers and imported bindings remain unverified. Final task metadata is committed separately. Existing after-2-storage checkout was not adopted or modified; ownership cleanup evidence is outside this task.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented bounded go-test-json-v1 report cards, honest reported/unknown evidence state, provenance and unavailable behavioral channels, real Go 1.27.1 fixtures, adversarial tests and compatibility documentation. Delivered implementation b963986 on main; Task checks and independent verification pass. All acceptance criteria satisfied; no further AFTER-4 work or external input required.
<!-- SECTION:FINAL_SUMMARY:END -->

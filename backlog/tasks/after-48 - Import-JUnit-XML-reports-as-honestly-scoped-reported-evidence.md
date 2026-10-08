---
id: AFTER-48
title: Import JUnit XML reports as honestly scoped reported evidence
status: To Do
assignee: []
created_date: '2026-10-08 23:14'
labels:
  - follow-up
  - extensibility
dependencies: []
documentation:
  - docs/EXTENSIONS.md
  - docs/GO-REPORTS.md
  - docs/SCHEMA.md
priority: medium
type: feature
ordinal: 48000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Stock `go test -json` is the only importer, so reported evidence exists only for Go projects. JUnit XML is the closest thing to a cross-ecosystem report format: pytest `--junitxml`, the Vitest `junit` reporter, jest-junit, Maven Surefire, Gradle, cargo-nextest, PHPUnit `--log-junit` and go-junit-report all emit it. One native importer gives most ecosystems useful first-use evidence without a plugin system; see docs/EXTENSIONS.md. JUnit XML has no single specification: producers differ in suite nesting, attributes, and how they encode errors, skips and captured output. It is also lossy: it carries outcomes and messages, never inputs or effects. Reuse the AFTER-4 evidence contract, store artifacts and card renderers. This is follow-up work outside the POC contract.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An explicit operator selection is recorded before implementation; this follow-up is not selected automatically from the POC queue.
- [ ] #2 A versioned dialect document and real captured fixtures from named producer versions (at least pytest, Vitest, and Maven Surefire or Gradle) cover passing, failing, erroring and skipped tests, nested suites, duplicate names in separate suites, captured stdout/stderr, and empty or no-test reports.
- [ ] #3 Imported cards are importer/reported/unknown/not_run/not_compared with inputs, expected values and effects explicitly unavailable; producer and snapshot stay unverified caller claims, and test names or failure messages never yield an observation or trusted badge.
- [ ] #4 XML parsing is bounded (input, depth, elements, attribute/text sizes, cards, retained output) and rejects DTDs, entity expansion and external references; malformed or unsupported content yields fixed line-numbered diagnostics without silently dropping valid unrelated cases, and any diagnostic makes the report incomplete.
- [ ] #5 Format selection between JUnit XML and go test JSON is explicit or unambiguous; ambiguous input fails with a fix instead of guessing. Import executes no code, fetches no URLs, follows no paths, keeps hostile text as sanitized data in CLI and TUI, and its tests pass without Docker, network or the producing toolchains.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials, real project reports or private receipts committed.
<!-- DOD:END -->

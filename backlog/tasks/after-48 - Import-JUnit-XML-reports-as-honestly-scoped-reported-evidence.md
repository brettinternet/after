---
id: AFTER-48
title: Import JUnit XML reports as honestly scoped reported evidence
status: Done
assignee: []
created_date: '2026-10-08 23:14'
updated_date: '2026-10-09 00:04'
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
- [x] #1 An explicit operator selection is recorded before implementation; this follow-up is not selected automatically from the POC queue.
- [x] #2 A versioned dialect document and real captured fixtures from named producer versions (at least pytest, Vitest, and Maven Surefire or Gradle) cover passing, failing, erroring and skipped tests, nested suites, duplicate names in separate suites, captured stdout/stderr, and empty or no-test reports.
- [x] #3 Imported cards are importer/reported/unknown/not_run/not_compared with inputs, expected values and effects explicitly unavailable; producer and snapshot stay unverified caller claims, and test names or failure messages never yield an observation or trusted badge.
- [x] #4 XML parsing is bounded (input, depth, elements, attribute/text sizes, cards, retained output) and rejects DTDs, entity expansion and external references; malformed or unsupported content yields fixed line-numbered diagnostics without silently dropping valid unrelated cases, and any diagnostic makes the report incomplete.
- [x] #5 Format selection between JUnit XML and go test JSON is explicit or unambiguous; ambiguous input fails with a fix instead of guessing. Import executes no code, fetches no URLs, follows no paths, keeps hostile text as sanitized data in CLI and TUI, and its tests pass without Docker, network or the producing toolchains.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials, real project reports or private receipts committed.
- [x] #4 Record extension seams in task notes: what needed code rather than data, and what differed from the existing instance; these feed the schema step in docs/EXTENSIONS.md.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend the existing report contract with a bounded native junit-xml-v1 parser and explicit CLI format selection; reuse storage and CLI/TUI cards without execution or stronger trust labels.
2. Capture synthetic reports from pinned pytest, Vitest and Maven Surefire producers, retaining reproduction sources and provenance; add parser and end-to-end regression coverage for malicious XML, partial input, bounds and format ambiguity.
3. Document dialect, supported producer shapes, limits and CLI behavior; run Task checks and independent trust-boundary verification.
4. Refresh the primary claim, integrate tested implementation, finalize task metadata, and remove only the receipt-verified owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator explicitly selected the next available item among AFTER-48–50 and authorized implementation in a worktree, commits, integration into main, and cleanup. Selected AFTER-48 by readiness and ID; existing AFTER-47 work is unrelated and preserved.

Implemented bounded junit-xml-v1 and explicit/auto format selection using existing schema-1 report cards, storage, history, CLI and TUI renderers. task fixtures:junit captured synthetic pytest 8.4.2, Vitest 3.2.4 and Maven Surefire 3.5.4 / JUnit Jupiter 5.13.4 output; capture removes machine-specific property inventory and normalizes synthetic paths/hostname. task test:reports passes offline with race detection; gopls diagnostics clean. Independent verifier a4ac5087 found no concrete parsing/trust defect and passed AC2–5. Its AC1 objection read stale copied task state: authoritative primary CLI reread confirms the prior explicit operator-selection note and In Progress claim. First task check:go ran the complete race suite and found only an 88-column import help line; shortened that line and added its existing golden test to the focused target before rerunning. A prior 120-second test:cli invocation timed out; the complete CLI race suite takes about six minutes here.

Extension seams (DoD4): JUnit needed native token/state parsing and bounded XML diagnostics rather than merely new data; it has outcome child elements, suite hierarchy, separate stdout/stderr, errors/skips, properties and schema hints, unlike Go JSON event streams. Existing report schema, evidence axes, unavailable fields, storage and shared cards needed no new record or plugin abstraction. Small dialect/channel selection and display helpers replace hard-coded Go-only checks. Producer-specific differences remain explicit dialect limitations, not inferred observations or a configurable plugin protocol.

Delivery: implementation commit 532dd88 rebased cleanly onto main a4b5b82 and fast-forward integrated into main, preserving concurrent extension-planning changes and all AFTER-47 work. Final task test:reports, task check:go, task test, task check:staged and post-rebase task check passed (full race tests, build, vet, Go/Prettier format checks, 151 local links, backlog graph, gitleaks with no leaks). The first post-rebase task check exceeded the 700-second harness window during its second CLI suite pass; rerun with a sufficient window passed. No Docker proofs were needed or claimed; no extended fuzz campaign was run. Independent verification artifact: junit-verification.md, workflow 1cf8bb12-c4c1-4da4-beaf-5e3169774877. No external blocker remains; only final task-state commit and receipt-verified cleanup remain.

Cleanup completed: verified the session creation receipt against the live Worktrunk path/branch, confirmed the verifier finished and the matching Herdr workspace held only an idle shell, then removed the owned worktree and branch with wt remove --foreground. The matching workspace disappeared; no session-owned checkout remains. Other pre-existing worktrees and the untracked AFTER-47 task are preserved. Final task-state commit is the only staged delivery change; no next task was started and nothing was pushed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered JUnit XML import in 532dd88 on main: bounded native parsing, explicit/auto selection, honest reported-only cards across CLI/TUI/history, actual synthetic pytest/Vitest/Surefire captures, dialect documentation and regression tests. task test:reports, task check:go, task test, task check:staged and post-rebase task check passed; independent verification found no concrete trust-boundary defects. Extension seams and limitations recorded. Owned worktree, branch and Herdr workspace removed. No blocker or resumable implementation step remains; AFTER-49/50 require a new request.
<!-- SECTION:FINAL_SUMMARY:END -->

---
id: AFTER-10
title: Deliver the complete headless review workflow
status: To Do
assignee: []
created_date: '2026-10-03 05:42'
labels:
  - poc
  - cli
milestone: m-0
dependencies:
  - AFTER-4
  - AFTER-5
  - AFTER-9
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 10000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A testable headless loop must exist before the TUI can hide engine faults. Scope: command composition over capture/import/run/compare, stable JSON output and real integration checks. Keep the vocabulary in docs/IMPLEMENTATION.md small and avoid adding another workflow runtime.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Documented CLI commands capture, import, inspect and export bounded machine-readable comparisons using stable IDs, preserving a useful no-evidence diff workflow.
- [ ] #2 Run commands preview exact execution plans and require interactive confirmation or a specific noninteractive authorization digest; imports/inspection/help never execute code and non-TTY commands never hang on prompts.
- [ ] #3 An integration test drives the real payment comparison through CLI subprocesses and inspects actual responses/effects/provenance; imported pass/fail is visibly reported rather than AFTER-observed.
- [ ] #4 JSON output is versioned and does not mix progress/ANSI on stdout; diagnostics go to stderr and documented exit statuses distinguish invalid input, denied execution, operational failure and comparison findings.
- [ ] #5 Models, GitHub access and accounts are absent from the path; missing Docker still permits import/diff/inspection. Errors leave terminal/process/store state usable.
- [ ] #6 CLI tests cover paths with spaces, bad IDs, unreadable artifacts, bounded output and injection strings; help examples work from a fresh prepared checkout.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

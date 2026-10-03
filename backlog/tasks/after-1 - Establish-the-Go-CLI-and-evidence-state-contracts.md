---
id: AFTER-1
title: Establish the Go CLI and evidence-state contracts
status: To Do
assignee: []
created_date: '2026-10-03 05:39'
labels:
  - poc
  - core
milestone: m-0
dependencies: []
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The repository currently has a proposal and tooling, not an engine. Establish the smallest executable foundation so capture, import, execution and rendering cannot conflate observations with human acceptance. Scope: Go module github.com/brettinternet/after, cmd/after, ordinary internal packages, schema documentation and Go Task/CI checks. Read the implementation contract before choosing types; do not add a plugin framework, database or model client.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A native after binary builds and exposes help/version on macOS and Linux without Bun, a model, a repository, Docker or network access at runtime; malformed arguments fail with documented exit status.
- [ ] #2 Versioned core records represent snapshot identity/completeness, scenario and observer/rule bindings, receipts/artifact limits, and pins; evidence producer/kind, applicability, execution/comparison outcome and human decision remain independent typed fields.
- [ ] #3 Tests round-trip valid records and reject unsupported versions, missing required binding fields and impossible state combinations; a human pin or imported pass cannot set observed/current evidence.
- [ ] #4 task build, task test:go and task check:go run real build, unit tests including race detection where supported, go vet and gofmt checks; task check/test and CI include them rather than silently skipping missing Go packages.
- [ ] #5 Schema examples are marked synthetic; errors and no-data output make no product-success claims. The shipped CLI has no init-time project commands or hooks.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials, generated private receipts or ignored artifacts committed.
<!-- DOD:END -->

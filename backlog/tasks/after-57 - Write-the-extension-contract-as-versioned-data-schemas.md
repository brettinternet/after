---
id: AFTER-57
title: Write the extension contract as versioned data schemas
status: Done
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 19:52'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-48
  - AFTER-49
  - AFTER-50
documentation:
  - docs/EXTENSIONS.md
  - docs/SCHEMA.md
priority: low
type: docs
ordinal: 57000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
docs/EXTENSIONS.md "Next decisions" step 1 says that after AFTER-48–50 the contract should be written as versioned data schemas (importer output cards, scenario definitions, observation artifacts and comparison rules) independent of how plugins would load or run, with built-ins conforming first. AFTER-48–50 recorded extension seams in their task notes for exactly this step, but nothing tracks it. Step 2 (whether code plugins are needed at all) depends on it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Machine-checkable schemas exist for report cards, http-service and command definitions, command/HTTP observation artifacts and comparison rules, each versioned and matching current strict decoders
- [x] #2 Built-in fixtures (payment, Python service, command proof definition, JUnit/Go report output) validate against the schemas in an existing Task check
- [x] #3 docs/EXTENSIONS.md records which AFTER-48–50 seams the schemas capture and what still needs code; no plugin runtime is added
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [x] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extract versioned machine-checkable data contracts from current evidence records, strict HTTP/command definition decoders, observations and fixed comparison rules; preserve runtime semantics and document constraints that still require Go validation. 2. Validate real built-in definitions and importer output against those schemas through the existing Go/Task checks; add focused rejection coverage for schema drift. 3. Document AFTER-48–50 seams and remaining code boundaries; run focused checks and one independent acceptance verification. 4. Commit implementation in the owned worktree, integrate into main, finalize authoritative task metadata and clean up.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator explicitly selected AFTER-57 after all M1/M2 tasks were confirmed Done. Owned checkout: .worktrees/after-57-schemas; branch after-57-schemas; creation commit 5ec36a576eb1aa4135a2f75d0e0926b97928ab67. Ownership receipt: .git/worktrees/after-57-schemas/agent-creation.json, session 01a121fc-d19c-725c-8999-5c582625039b. First Worktrunk invocation stopped before creation for hook approval; reviewed hooks then created successfully with --yes.

Delegated implementation workflow 76340a9d-d6b9-4098-ac2e-5ec4bdeaa2c8 stopped because executor 9cc463d0-0fee-4e6f-86bc-a264f2c9048d hit its 1800000ms harness deadline. Partial changes preserved and inspected in owned after-57-schemas checkout: seven v1 JSON schemas, contract tests, shared command fixture, docs and Go schema-validator dependency. Transcript records schema race tests and formatting passed; first full race suite passed; local-link check (159 links) and backlog check (57 tasks) passed. Full task check was interrupted during its repeated test pass, not recorded as passed. No remaining test processes. Same child resumed as 5224defb-d3d7-44fd-8c9c-e3596ea2ad66 for outstanding checks/report; independent verifier has not yet run. No commit or integration yet.

Delivered implementation commit 17c04dfc7e055c3ad561042d715d7642badbc364 and fast-forwarded main. Validation passed: go test -race ./internal/schemas; task command:check; task test:reports; task format:check; first go test -race ./... in task check; task build; go vet ./...; staticcheck -checks U1000 ./...; gofmt -l cmd internal (empty); task secrets; git diff --check; task check:staged. Aggregate task check remained incomplete at the harness deadline during a redundant second race pass; all distinct components passed. Independent verifier 48d0db5b-d5c1-4f9e-a639-1d8619454488 returned PASS for all ACs with no concrete defects, additionally passing go test ./internal/schemas ./internal/cli and go test ./internal/schemas -count=1 plus command/report Task checks. Both changed Go files had clean LSP diagnostics. No Docker proof was run because runtime execution behavior is unchanged. Structural schema limitations (byte budgets, duplicate keys, base64/cross-field checks and provenance) are documented; Go remains authoritative. Test-only JSON Schema dependency upgrades indirect x/text to v0.14.0. Verified ownership receipt and clean integrated checkout; all children completed. Worktrunk removed after-57-schemas checkout and branch, and its post-remove hook closed verified workspace w37 (only idle zsh pane); confirmed workspace and checkout absent. Pre-existing unrelated worktrees were preserved. No remaining blocker or implementation step; no push.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added seven versioned JSON Schemas, built-in/importer conformance and rejection tests, shared command proof fixture, and extension-boundary documentation. Integrated 17c04df into main; independent verification and all distinct check components passed (aggregate task check interrupted by harness timeout). Owned worktree, branch and workspace cleaned up; task claim released.
<!-- SECTION:FINAL_SUMMARY:END -->

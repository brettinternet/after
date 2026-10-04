---
id: AFTER-1
title: Establish the Go CLI and evidence-state contracts
status: Done
assignee: []
created_date: '2026-10-03 05:39'
updated_date: '2026-10-04 01:45'
labels:
  - poc
  - core
  - reviewed
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
- [x] #1 A native after binary builds and exposes help/version on macOS and Linux without Bun, a model, a repository, Docker or network access at runtime; malformed arguments fail with documented exit status.
- [x] #2 Versioned core records represent snapshot identity/completeness, scenario and observer/rule bindings, receipts/artifact limits, and pins; evidence producer/kind, applicability, execution/comparison outcome and human decision remain independent typed fields.
- [x] #3 Tests round-trip valid records and reject unsupported versions, missing required binding fields and impossible state combinations; a human pin or imported pass cannot set observed/current evidence.
- [x] #4 task build, task test:go and task check:go run real build, unit tests including race detection where supported, go vet and gofmt checks; task check/test and CI include them rather than silently skipping missing Go packages.
- [x] #5 Schema examples are marked synthetic; errors and no-data output make no product-success claims. The shipped CLI has no init-time project commands or hooks.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials, generated private receipts or ignored artifacts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a standard-library-only Go module and side-effect-free help/version CLI with explicit exit codes and native smoke tests.
2. Define version-1 snapshot, scenario, receipt and pin records with strict bounded JSON decoding, typed independent states and conservative binding/state validation; document synthetic examples and trust limits.
3. Wire build, race tests, vet and formatting into Task and macOS/Linux CI; run cross-project checks.
4. Obtain one independent verification focused on false observed/current claims and command side effects, correct concrete defects, integrate implementation into main, then commit final task metadata and clean up the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented standard-library-only CLI, version-1 snapshot/scenario/receipt/pin records, bounded validating decoder, synthetic schema example, state/binding negative tests, and Task plus macOS/Linux CI Go gates.
Verification: mise exec -- task check passed (build, race tests in 3 packages, vet, gofmt, formatting, links/backlog and secrets); task check:go passed again after docs update. LSP diagnostics clean for CLI, entrypoint and record validator. GOOS=linux GOARCH=arm64 CGO_ENABLED=0 mise exec -- task build passed; binary help/version ran successfully in a cached Linux container with network disabled, read-only filesystem and non-root user, and malformed capture returned 2. Linux amd64 cross-build passed; restored native macOS build. Remote CI has not run (no push authorized). Independent verifier is checking acceptance/trust-boundary behavior before integration.

Explicit operator handoff to the successor session. Recovered the original creation receipt and inactive owner transcript; independent verifier run 717d5dec-1e45-4786-aee3-efa95dc19212 is complete. Verification passed criteria 2-5 with no concrete defects; direct native binary smoke was blocked by verifier shell policy. Prior parent recorded Linux binary smoke evidence; successor will refresh local checks before integration.

Delivered implementation commit ebcc610 to main by fast-forward, preserving authoritative task metadata. Refreshed mise exec -- task check in the adopted worktree and on main: passed build, race tests (3 packages), vet, gofmt, formatting, 35 local links, 20-task backlog integrity and secret scan. mise exec -- task check:staged passed before implementation commit. Native macOS binary under env -i passed help/version and rejected capture with exit 2; entrypoint test passed in an empty non-repository directory. Rechecked prior transcript evidence for Linux arm64 binary help/version in an offline, read-only, non-root container and Linux amd64 cross-build. Independent verification found no concrete defects in criteria 2-5; its isolated-binary attempt was policy-blocked, covered by parent smoke evidence instead. No additional general review or source changes were needed. Worktrunk removed the verified adopted worktree and branch; its matching idle-shell Herdr workspace disappeared. No remaining blocker; next ready task is AFTER-2, not started. Remote CI remains unrun; no push authorized.

Review 2026: Found artifact validation accepted impossible redacted/truncated + completeness=complete combinations (AC3). Fixed in evidence.Validate for receipt artifacts and comparison details; added negative tests. task check:go PASS.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented native side-effect-free help/version CLI, versioned structurally validated evidence records with independent trust/review states, synthetic schema documentation, and macOS/Linux Go Task/CI checks. Integrated ebcc610 on main. Full local task check, staged checks, native smoke tests and independent trust-boundary verification passed within the recorded limitations. Final task metadata committed separately on main; claim released and implementation worktree cleaned up.
<!-- SECTION:FINAL_SUMMARY:END -->

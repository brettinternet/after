---
id: AFTER-8
title: Run frozen paired experiments with bounded process supervision
status: Done
assignee: []
created_date: '2026-10-03 05:42'
updated_date: '2026-10-03 17:42'
labels:
  - poc
  - runner
milestone: m-0
dependencies:
  - AFTER-5
  - AFTER-6
  - AFTER-7
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 8000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The central evidence needs actual paired execution, not trust in a candidate's own tests. Scope: orchestration using the sandbox and fixture already proven by AFTER-6/7, immutable receipts and process-lifecycle tests. Do not introduce arbitrary shell command runners or baseline reuse before matching all bindings.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 An authorized plan runs the exact same frozen scenario/driver/observer/rules against two captured versions, using independent fresh state and recording actual argv, image/toolchain/dependency identities, inputs, timestamps and completion on each side.
- [x] #2 Real response and fake-provider artifacts persist through the store; a candidate-edited test or mask cannot replace the frozen oracle. Candidate-owned suite results remain separate and identified.
- [x] #3 Timeout, denied permission, startup/build failure, incompatible driver, missing channel, cancellation and output truncation produce explicit partial/failure states, never a behavioral pass or fabricated baseline.
- [x] #4 Snapshot/request IDs bind every result from submission to storage; barrier-controlled tests change the selected snapshot before completion and prove late output remains attached to the old pair.
- [x] #5 Repeated runs retain every sample; mismatching repetitions surface instability. The executor bounds concurrency and drains stdout/stderr without deadlock or unlimited memory.
- [x] #6 Cancellation and failure paths leave no owned child/container/service running, never stop unrelated processes, and retain bounded diagnostics without secrets. Builds do not escape the same authorization/isolation boundary.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse the sandbox policy and owned-resource cleanup for two containers per case: candidate build/app and frozen observer in separate PID/filesystem namespaces, sharing only an offline network namespace. Keep candidate output out of observations.
2. Add a payment-specific frozen runner that loads validated captures, previews a request/pair/scenario/repetition-bound plan, runs serial fresh cases, persists every bounded sample and receipt, and surfaces failures and repetition instability without comparing versions.
3. Exercise consent, stale async selection, failures, bounds, redaction and real hostile-candidate/payment Docker cases. Document limits, obtain one independent trust-boundary verification, run Task checks, integrate and commit on main.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation in session-owned after-8-runner worktree, creation receipt at Git worktrees/after-8-runner/agent-creation.json (session 01a102bd-7b24-71dc-9cb9-01eba5bb95e8). Added protected two-container sandbox experiments, frozen payment runner, request-bound immutable receipts/sample artifacts, serial concurrency gate, failure handling and repetition instability. Real runner proof passed two repetitions of 12h 1/2 and 30s 1/1 provider calls with identical responses despite candidate oracle/stdout mutations. Five lifecycle probes passed: isolation, combined output overflow, startup failure, readiness-confirmed timeout and cancellation; unrelated sentinel remained running and owned resource inventories were empty.
Independent read-only review b8e36c3d-8c16-4a62-bd5f-c8f00a17d84b / child 106c2f65-5041-4f04-a211-2c6578d59547 found two concrete defects: launcher did not monitor app exit, and proxy default errors fabricated HTTP 502 observations. Fixed continuous child reaping/abort and transport-error connection abort. Added live regressions for exited parent with serving descendant, refused upstream, and real application 502 preservation. Final runner:proof passed (451.679s payment/regressions + 14.008s lifecycle); check:go passed build/race/vet/gofmt. No second general review. LSP clean. Docs/final repository checks and integration pending. Local macOS Colima Linux aarch64; no native Linux/Desktop/amd64 certification.

Delivered implementation 5e11e8c by fast-forward to main, preserving the authoritative task claim. Final mise exec -- task check passed tool checks, all formatting, Go race tests, vet/build/gofmt, 57 local links, 20-task graph and secret scan. mise exec -- task check:staged passed before implementation commit; task test passed again on main. Unit barrier/concurrency/instability/redaction/failure tests and real runner:proof supply AC evidence; independent read-only review defects are covered by passing live regressions. No private receipts or credentials committed. Docs/RUNNER.md records API, exact ABI, resource limits, failure/recovery contract and deferred public CLI.
Verified creation receipt and clean Worktrunk listing against this session; reviewer completed, matching Herdr pane was an idle shell. Removed after-8-runner checkout/branch with Worktrunk and verified workspace wY disappeared. Final owned Docker containers/images inventory empty. Pre-existing after-2-storage and after-7-payment checkouts/workspaces remain untouched: historical cleanup ownership was not adopted. No remaining task blocker; no subsequent item started; no push. Final task metadata committed separately on main.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered frozen paired payment execution in 5e11e8c: consent-bound immutable requests, isolated observer, stored exact responses/provider traffic, every repetition, conservative failures and instability, bounded supervision and owned cleanup. Full Task checks and real Docker payment/lifecycle/regression proofs pass. Independent review found two defects, both fixed and exercised. Integrated on main; owned implementation worktree cleaned; public CLI wiring remains AFTER-10.
<!-- SECTION:FINAL_SUMMARY:END -->

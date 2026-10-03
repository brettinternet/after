---
id: AFTER-6
title: Prove the offline execution sandbox and consent boundary
status: To Do
assignee: []
created_date: '2026-10-03 05:40'
labels:
  - poc
  - runner
  - security
milestone: m-0
dependencies:
  - AFTER-1
  - AFTER-2
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: spike
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A worktree and subprocess are not a sandbox. Settle one tested Docker-based execution topology before adding application execution. Scope: a narrow runnable isolation harness, safety tests, pinned image/dependency preparation instructions and a short decision record. No general sandbox-provider abstraction. If the environment cannot enforce the contract, record a real blocker; never fall back to host execution.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An executable proof runs a benign standard-library HTTP workload in disposable isolation on documented macOS/Linux Docker setups; the selected image is pinned by digest and provisioning is a separate explicitly authorized operation.
- [ ] #2 Builds and runs require a previewed digest-bound plan covering images, argv, snapshot inputs, mounts, network and limits; denied or changed plans start nothing. Repository text cannot grant consent.
- [ ] #3 Attack probes cannot reach external network, host services, ambient credentials, home directories or Docker socket; inputs are read-only, processes non-root, and only owned bounded writable storage is exposed. Record the sandbox's residual threat-model limits honestly.
- [ ] #4 CPU/memory/process/time/output limits and cancellation kill all owned descendants/containers and retain useful bounded failure evidence; no arbitrary path cleanup or unrelated-service termination.
- [ ] #5 Offline execution fails closed when images/dependencies or required isolation capabilities are missing; no implicit pull, install script or production URL is allowed.
- [ ] #6 Security decisions, exact supported Docker capabilities, provisioning, cleanup checks and runnable negative tests are documented for AFTER-8; the spike leaves reusable tested primitives, not only prose.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

---
id: AFTER-6
title: Prove the offline execution sandbox and consent boundary
status: Done
assignee: []
created_date: '2026-10-03 05:40'
updated_date: '2026-10-03 16:51'
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
- [x] #1 An executable proof runs a benign standard-library HTTP workload in disposable isolation on documented macOS/Linux Docker setups; the selected image is pinned by digest and provisioning is a separate explicitly authorized operation.
- [x] #2 Builds and runs require a previewed digest-bound plan covering images, argv, snapshot inputs, mounts, network and limits; denied or changed plans start nothing. Repository text cannot grant consent.
- [x] #3 Attack probes cannot reach external network, host services, ambient credentials, home directories or Docker socket; inputs are read-only, processes non-root, and only owned bounded writable storage is exposed. Record the sandbox's residual threat-model limits honestly.
- [x] #4 CPU/memory/process/time/output limits and cancellation kill all owned descendants/containers and retain useful bounded failure evidence; no arbitrary path cleanup or unrelated-service termination.
- [x] #5 Offline execution fails closed when images/dependencies or required isolation capabilities are missing; no implicit pull, install script or production URL is allowed.
- [x] #6 Security decisions, exact supported Docker capabilities, provisioning, cleanup checks and runnable negative tests are documented for AFTER-8; the spike leaves reusable tested primitives, not only prose.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a narrow Docker sandbox package with immutable source archives, explicit digest-bound previews, fixed no-network/non-root/read-only policy, bounded output and owner-only cleanup. Use one container with loopback-only HTTP; no host mounts or published ports.
2. Add a standard-library HTTP/attack probe and opt-in Task target exercising real Docker, denial/changed consent, offline preflight, resource limits and cancellation. Pin the separately provisioned official Go image.
3. Document supported Linux Docker capabilities/macOS Colima usage and residual limits. Run focused Go and live sandbox checks, get independent trust-boundary verification, then integrate and commit task evidence on main.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator explicitly approved provisioning the official Go image and disposable offline probes. Docker 29.5.2, Linux aarch64 kernel 6.8.0, cgroup v2, seccomp/AppArmor available via macOS Colima. Provisioned golang:1.27.1 at sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190. Existing unrelated containers untouched.

Implemented immutable plan/consent and Docker ownership primitives in after-6-sandbox. Real Docker rejected copying into a stopped read-only rootfs; changed preparation to copy bounded regular inputs into an unstarted container and commit an owned temporary input-only image, then execute with read-only root. No preparation command runs project code. Explicit executable tmpfs is needed for compiled Go output. All nine live sandbox cases now pass (32.136s): HTTP/security probes, denial, missing image/dependency, bounded output, deadline/descendants, cancellation, memory exhaustion, scratch exhaustion. Containers and derived images cleaned; independent trust-boundary review pending.

Resumed the explicitly handed-off implementation and integrated commit 5b7a762 into main by fast-forward, preserving the primary task claim. Independent trust-boundary review found one concrete proof defect: cancellation/deadline could pass before descendants started. Fixed both to require child-readiness output; cancellation additionally uses a random invocation token and matches the returned container identity. Added synthetic CLI tests for refusal of each missing isolation capability. No validated consent bypass or sandbox escape found. A duplicate reviewer was started before recovering the prior review; it was restricted to existing findings, with no further general review.

Final verification: mise exec -- task check:go passed (build, race tests, vet, gofmt); mise exec -- task test passed in implementation checkout and again on main (48 local links and 20-task graph); mise exec -- task check:staged passed formatting and secret scan before implementation commit. With explicit AFTER_DOCKER_BINARY/HOST, mise exec -- task sandbox:proof passed all nine real Docker cases in 32.659s after the readiness correction. Owned Docker container/image inventories were empty afterward. LSP diagnostics clean for changed tests. Live environment was macOS Colima with Linux aarch64 Docker 29.5.2/cgroup v2; native Linux workstation, Docker Desktop and amd64 were not independently tested. docs/SANDBOX.md records provisioning, exact limits, daemon/kernel trust, interrupted-mutation reconciliation and observer-isolation obligations for AFTER-8.

Delivery: adopted sandbox checkout and branch removed with Worktrunk after receipt verification and idle-pane inspection; matching Herdr workspace disappearance verified. Pre-existing after-2-storage checkout/workspace was not adopted and remains untouched. No push. No task blocker remains; subsequent fixture/runner work requires a separate task invocation.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented reusable digest-bound offline Docker sandbox primitives and executable security proof in 5b7a762, integrated on main. Denied/changed consent starts nothing; missing image/dependencies/capabilities fail closed. Real HTTP, isolation attacks, resource bounds and readiness-confirmed descendant cancellation pass. Go/repository checks and secret/format gates pass; review defect fixed. Documented residual limits and provisioning; sandbox worktree cleaned.
<!-- SECTION:FINAL_SUMMARY:END -->

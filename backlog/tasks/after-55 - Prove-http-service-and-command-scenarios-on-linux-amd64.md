---
id: AFTER-55
title: Prove http-service and command scenarios on linux/amd64
status: Done
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 18:21'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-49
  - AFTER-50
documentation:
  - docs/SANDBOX.md
  - docs/RUNNER.md
priority: low
type: task
ordinal: 55000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
All live AFTER-49/50/51 evidence is from Colima Linux arm64. The Python http-service fixture (internal/runner/testdata/python-service/http-service.json) pins a linux/arm64-only manifest digest, so `task http-service:proof` and `task test:poc` cannot pass on an amd64 host, and no amd64 Docker run of the command or HTTP launchers has been observed. Provisioning an amd64 Python image digest is an explicit operator network action; the runner itself stays pull-free.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 The Python fixture selects a separately provisioned, digest-pinned image matching the host platform (or the proof fails with a provisioning fix), without image pulls during runs
- [x] #2 task http-service:proof, task command:proof and task test:poc pass on a linux/amd64 Docker host, with exact commands and host details recorded in task notes, or an objective external blocker is recorded
- [x] #3 docs/SANDBOX.md lists the provisioned digests per platform
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [x] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a fail-closed Python proof platform check with an actionable provisioning fix; retain exact digest-bound consent and pull-free execution.
2. Document provisioned platform digests and the operator-approved amd64 host/image blocker.
3. Verify platform mismatch refusal and available arm64 HTTP/command proofs, run focused checks, review once, commit and integrate, then clean up the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator explicitly selected AFTER-55 and chose to record the amd64 host blocker. docker info reports linux/aarch64; docker context ls exposes only local Colima and default sockets; colima list reports a single running aarch64 profile. No native amd64 host or amd64 Python image is provisioned. Objective unblock: supply a native linux/amd64 machine with a local Docker Unix socket and separately authorize/provision a digest-pinned amd64 Python image, then run all three proof targets there. No emulation or remote Docker workaround authorized.

Implemented the explicit provisioning-failure alternative in AC #1: the Python proof parses the pinned fixture and rejects a mismatched native test-process platform before consent, storage setup or Docker access, with instructions to provision a matching digest and update platform/image together. Regression TestPythonProofPlatformProvisioning exercises arm64 acceptance and amd64 refusal. No production execution or trust-boundary code changed. docs/SANDBOX.md lists provisioned arm64 Go/Python digests and explicitly marks amd64 unprovisioned/unverified. One scoped self-review corrected documentation to distinguish daemon isolation checks from image-platform checks; no daemon CPU check or amd64 execution is claimed.

Verification on Colima linux/aarch64, Docker Engine 29.5.2, kernel 6.8.0-117-generic, cgroup v2: mise exec -- task check:go PASS (build, race tests including platform refusal, vet, U1000, format); mise exec -- task test PASS (race tests, docs links, backlog); mise exec -- task http-service:proof command:proof PASS (44.68s HTTP; 52.94s command); mise exec -- task test:poc PASS with all required proofs and both expected mutant failures. Proof commands supplied AFTER_DOCKER_BINARY as the installed absolute Docker CLI and AFTER_DOCKER_HOST as the local Colima Unix socket; no image provisioning/pulls performed. First full-gate attempt was interrupted by the tool 1200-second timeout, not a product assertion; rerun with a 3900-second tool budget passed. Two stopped preparer containers and one derived input image left by interruption were inspected and removed by exact ID after explicit operator approval; Docker after.owner container/image inventories are now empty. mise exec -- task check:staged PASS. AC #2 uses its objective external-blocker alternative: all evidence above is arm64, NOT amd64; the recorded native amd64 host and separately authorized image provisioning remain the next required step.

Delivery: implementation commit f79b68f fast-forwarded into main. Verified the session creation receipt and clean merged checkout, inspected the exact Herdr workspace shell, then removed the owned after-55-amd64 worktree and branch with Worktrunk foreground removal. Its Herdr workspace disappeared. Pre-existing unrelated worktrees were preserved. No push performed. Completion uses the task explicitly permitted provisioning-fix and external-blocker alternatives; native amd64 proof remains unperformed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added early Python proof platform refusal with actionable separate provisioning instructions and a regression covering the current arm64 fixture versus amd64. Documented exact provisioned arm64 digests and missing amd64 host/images. Integrated f79b68f into main and cleaned up the owned worktree/workspace. check:go, test, HTTP/command proofs and the full no-skip POC gate passed on arm64; both mutation controls failed as expected. Native amd64 execution remains externally blocked, as expressly permitted by AC #2 and selected by the operator. Resume amd64 verification only after a native host and separately authorized pinned images are supplied.
<!-- SECTION:FINAL_SUMMARY:END -->

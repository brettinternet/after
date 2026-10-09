---
id: AFTER-55
title: Prove http-service and command scenarios on linux/amd64
status: To Do
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 14:48'
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
- [ ] #1 The Python fixture selects a separately provisioned, digest-pinned image matching the host platform (or the proof fails with a provisioning fix), without image pulls during runs
- [ ] #2 task http-service:proof, task command:proof and task test:poc pass on a linux/amd64 Docker host, with exact commands and host details recorded in task notes, or an objective external blocker is recorded
- [ ] #3 docs/SANDBOX.md lists the provisioned digests per platform
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [ ] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

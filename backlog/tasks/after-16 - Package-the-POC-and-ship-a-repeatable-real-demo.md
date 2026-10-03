---
id: AFTER-16
title: Package the POC and ship a repeatable real demo
status: To Do
assignee: []
created_date: '2026-10-03 05:44'
labels:
  - poc
  - release
  - docs
milestone: m-1
dependencies:
  - AFTER-15
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: task
ordinal: 16000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The POC must be reproducible by someone other than its implementer. Scope: distribution builds, honest quick start, owned demo setup/teardown and real-engine CI. Do not publish a release or push binaries unless the current operator explicitly authorizes it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Documented build/package tasks produce versioned macOS/Linux native binaries with checksums, pinned Go/module dependencies and no runtime Bun requirement; supported architecture and execution prerequisites are explicit.
- [ ] #2 task demo (or one equally clear documented command) creates an owned disposable fixture workspace and walks capture, run consent, inspect, pin, edit, reopen and rerun using real receipts and an unchanged control.
- [ ] #3 Demo setup never resets the user's repository, copies secrets, silently pulls an image, starts production services or deletes an unknown path; cleanup targets only verified owned demo resources and leaves unrelated resources intact.
- [ ] #4 A fresh prepared checkout can run the no-Docker import/inspection walkthrough and, after explicit image provisioning, the full offline paired demo; missing prerequisites get actionable errors.
- [ ] #5 CI builds/tests supported host platforms and runs the sandbox POC gate on Linux with its exact image/toolchain identities; docs include diagnostic and recovery steps for store errors and cleanup failures.
- [ ] #6 Root README/HANDOFF and command help accurately distinguish working capabilities, untested broader behavior and deferred features; the simulated presentation is not relabeled as observed product evidence.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks pass and exact commands, actual results or objective external blockers are recorded in task notes.
- [ ] #2 Update affected docs and limitations; do not claim unrun experiments or human validation.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; keep sensitive/private artifacts out of Git.
<!-- DOD:END -->

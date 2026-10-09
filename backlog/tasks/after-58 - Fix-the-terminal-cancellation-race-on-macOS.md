---
id: AFTER-58
title: Fix the terminal cancellation race on macOS
status: Done
assignee:
  - '@pi'
created_date: '2026-10-09 20:54'
updated_date: '2026-10-09 21:17'
labels: []
dependencies: []
ordinal: 58000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Operator-reported TestPTYRestoration flake reproduced twice in 100 runs. Forced Bubble Tea shutdown closes cancelreader descriptors without joining the input read loop.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Context cancellation restores the PTY without data races in repeated race-enabled runs
- [x] #2 Full Go checks pass without disabling race detection or weakening existing PTY assertions
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Verified implementation is integrated into main and staged formatting and secret checks pass
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Preserve the reproduced context-cancellation race evidence and inspect upstream lifecycle behavior. 2. Share the two terminal entry points through a runner that translates parent cancellation into graceful Quit, keeps model/job contexts cancellable, and preserves ErrProgramKilled plus the context error. 3. Exercise existing PTY restoration 500 times with race detection, cover already-cancelled startup, then run full Go and staged checks, integrate and commit.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Baseline GOFLAGS="-run=^TestPTYRestoration$ -count=100" task test:terminal failed twice on macOS, both context-error: cancelreader wait calls File.Fd concurrently with cancelreader Close or slave Close. Bubble Tea v1.3.10 shutdown skips waitForReadLoop when kill=true; latest v1 is still v1.3.10. No race suppression or test weakening planned.

Delivered bb5effb by fast-forward to main. Both terminal entry points now share graceful parent-context cancellation; job contexts still cancel and errors preserve ErrProgramKilled/context.Canceled. Unchanged baseline on the pinned toolchain failed twice in 100 iterations; fixed PTY restoration and cancelled-startup tests passed 500 iterations (2,000 PTY exits) with race detection. task check:go passed full race suite, build, vet, staticcheck and formatting; all five affected Go files had clean LSP diagnostics. task check:staged and git diff --cached --check passed. Existing restoration assertions retained and context error assertion strengthened. No dependency upgrade, race suppression or test skips.

Post-integration task test passed all Go race tests and 159 local links, then caught a missing DoD field in this new task. Added the required completion gate via CLI; rerunning backlog validation as part of task test.

The later main-checkout task test rerun overlapped concurrent AFTER-59 integration and failed CLI goldens; no unrelated code or goldens were changed here. A subsequent focused PTY plus documentation/backlog run passed PTY and links but is blocked by AFTER-59 missing its completion gate. The full check:go pass in the isolated implementation checkout remains the delivery evidence. Receipt-verified after-58-pty-race checkout and branch were removed with Worktrunk; no matching Herdr workspace existed before removal. No owned checkout retained; no push.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Fixed context-cancellation shutdown for both terminal clients in bb5effb. Verified 500 repeated race-enabled PTY runs and full task check:go; baseline still reproduces the library race. Preserves terminal restoration and cancellation errors.
<!-- SECTION:FINAL_SUMMARY:END -->

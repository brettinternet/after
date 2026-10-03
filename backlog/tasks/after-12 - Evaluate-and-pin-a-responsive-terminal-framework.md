---
id: AFTER-12
title: Evaluate and pin a responsive terminal framework
status: Done
assignee: []
created_date: '2026-10-03 05:42'
updated_date: '2026-10-03 21:17'
labels:
  - poc
  - tui
  - security
milestone: m-1
dependencies:
  - AFTER-5
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: spike
ordinal: 12000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The proposal names Bubble Tea as a candidate, not a settled dependency. Scope: a bounded executable evaluation before committing the full UI, including terminal injection and asynchronous rendering risks. Do not spend the spike on theme polish or browser parity.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A small Go terminal experiment exercises Bubble Tea's model/update/view flow against captured large diffs and asynchronous fake jobs; a short decision record pins the tested dependency or documents a simpler selected fallback.
- [x] #2 Rendering and event updates do not run Git or block on jobs; measured input/resize/quit responsiveness and memory on a reproducible large-diff workload meet a stated usable budget or trigger an explicit documented fallback.
- [x] #3 All untrusted strings pass a tested terminal-safe rendering boundary; ESC/CSI/OSC clipboard/hyperlink payloads cannot execute, change titles, forge trusted styling or leak into restored terminal state.
- [x] #4 Viewport/selection tests cover Unicode width, combining characters, tabs, long lines, 40-column layouts, resize and empty/no-evidence views; no full-patch rebuild on every keypress.
- [x] #5 Core tests run without an interactive terminal; at least one actual PTY smoke check covers cancellation/quit and terminal restoration, including injected errors.
- [x] #6 Experiment code is retained only as the minimal reusable view/sanitization foundation needed by AFTER-13; no second demo app or general UI framework remains.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Evaluate pinned Bubble Tea v1.3.10 with an internal reusable terminal model and immutable, bounded sanitized text index; keep Git/storage/job preparation outside Update/View. 2. Exercise real captured large diffs, grapheme-aware bounded viewport/selection, tagged asynchronous fake work, and cancellation. 3. Add deterministic headless safety/layout tests, measured latency/allocation budgets and actual PTY restoration/error smoke tests; retain no separate demo binary. 4. Record decision/limits, obtain independent trust-boundary verification, run relevant Task checks, integrate implementation into main and commit final task state.

Final gate amendment: operator approved direct verification in place of independent verification after repeated extension-session infrastructure failures.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented bounded internal/terminal foundation in after-12-terminal. Pinned Bubble Tea v1.3.10/uniseg v0.4.7 with test-only creack/pty v1.1.24. task test:terminal passes race/headless/real PTY tests; actual 7506350-byte, 100250-line Git capture prepared in 41.3 ms with 11613488 bytes allocated, input+view max 1.14 ms, resize+view max 0.47 ms, PTY restoration 10.9-11.5 ms. task terminal:bench: 27572 ns/op, 16185 B/op; task terminal:fuzz: 1501230 executions without failures. task check:go and task test pass (build, all race tests, vet, gofmt, 79 local links, backlog integrity). PTY tests caught ordinary quit being misreported as context cancellation; separated job cancellation from program parent context and reran successfully. Independent trust-boundary verification is pending. No full TUI or CLI command is claimed; clipped long lines retain raw artifacts.

Operator explicitly handed off incomplete work. Resuming existing after-12-terminal implementation for focused checks, one independent trust-boundary verification, integration and final commits; no next task or push.

Current operator again authorizes takeover of incomplete AFTER-12. Reusing the existing staged implementation without discarding changes; refreshing verification and delivery evidence in this session.

Takeover verification passed in after-12-terminal: mise exec -- task check:go (build, race tests, vet, gofmt); task test (all Go tests, 79 local links, 20-task integrity); task check:staged (formatting and no secret leaks); task terminal:bench (28063 ns/op, 16185 B/op, 285 allocs/op); task terminal:fuzz (1962893 executions, no failure). Independent acceptance verification is running. Existing checkout will be retained after integration because original-session/delegated-run inactivity has not been independently established; no destructive cleanup is assumed from the handoff.

Operator explicitly authorizes takeover in this session. Resume existing implementation, verify trust boundary and focused checks, then commit and integrate into main; preserve existing checkouts.

Current session accepts the explicit operator handoff. Preserve and finish existing AFTER-12 implementation, perform one independent acceptance verification, then integrate and commit on main. Retain existing worktrees without destructive cleanup.

Current takeover checks passed: mise exec -- task check:go (build/race/vet/gofmt), task test (79 links and 20-task integrity), task terminal:bench (26538 ns/op, 16185 B/op, 285 allocs/op), task terminal:fuzz (2091665 executions without failure), and task check:staged (formatting/secrets). Independent read-only acceptance verification is the remaining delivery gate. Existing worktrees are preserved; cleanup ownership is not assumed.

Current takeover: task check:go, task test and task check:staged passed; terminal:bench passed at 26974 ns/op, 16186 B/op, 285 allocs/op; terminal:fuzz passed 1891628 executions. Prior verification workflow 3c974868 stopped because its extension session was replaced; staged implementation remains intact. Same-protocol independent read-only verification retry c24b9afa is active. Await its verdict before implementation commit and fast-forward integration; preserve existing worktrees.

Operator explicitly approved direct final trust-boundary verification instead of independent verification after workflow runs 3c974868 and c24b9afa both stopped on extension-session replacement. Direct review found no concrete item-scoped defects. Fresh GOFLAGS=-count=1 mise exec -- task test:terminal passed all headless, asynchronous and actual PTY tests: 7,506,350-byte captured diff prepared in 40.774 ms, 11,613,488 bytes allocated; maximum input/view 1.032 ms, resize/view 0.457 ms; PTY restoration 10.5-12.4 ms. mise exec -- task check:go, task test, task check:staged and git diff --cached --check passed. task terminal:bench passed at 26,479 ns/op, 16,186 B/op, 285 allocs/op; task terminal:fuzz passed 2,180,206 executions. Full TUI/CLI wiring remains AFTER-13. Existing worktrees retained without destructive cleanup; no push authorized.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered d861d57 and fast-forward integrated into main: pinned Bubble Tea v1.3.10, bounded immutable document/viewport and terminal-safe rendering, asynchronous cancellation and result binding, headless/Unicode/injection tests, actual PTY restoration, reproducible large-diff budgets and decision documentation. Build, race tests, vet, formatting, links/backlog checks, fresh PTY tests, benchmark and 2.18 million fuzz executions passed. Direct final verification substituted with explicit operator approval; no concrete defects found. All six acceptance criteria satisfied. No remaining blocker; full TUI integration is AFTER-13, not started. Final task state committed separately on main; no push. Existing worktrees retained because cleanup ownership was not established.
<!-- SECTION:FINAL_SUMMARY:END -->

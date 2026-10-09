---
id: AFTER-49
title: Run declarative HTTP-service scenarios on the paired runner
status: Done
assignee: []
created_date: '2026-10-08 23:14'
updated_date: '2026-10-09 07:11'
labels:
  - follow-up
  - extensibility
dependencies: []
documentation:
  - docs/EXTENSIONS.md
  - docs/RUNNER.md
  - docs/SANDBOX.md
  - docs/COMPARISON.md
  - docs/IMPLEMENTATION.md
priority: medium
type: feature
ordinal: 49000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The runner hardcodes one experiment: the payment ABI (`go.mod`, `app/main.go`), the pinned golang image, fixed build/start argv, two same-key requests, the fake-provider observer and fixed comparison rules. Any other HTTP service needs new Go code in AFTER, which is the coverage problem docs/EXTENSIONS.md describes. Most language variety is which image, which build and start argv, and which stimuli: data that can appear in the consent preview instead of code. Generalize the runner around a declarative `http-service` scenario definition and make the payment experiment one instance of it. This is the first concrete scenario kind from which a later plugin protocol may be extracted; it is not a plugin framework, universal adapter or dependency-preparation system. It widens the POC scope in docs/IMPLEMENTATION.md, which must be updated with it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 An explicit operator selection is recorded before implementation; this follow-up is not selected automatically from the POC queue.
- [x] #2 A versioned, strictly decoded http-service definition declares a digest-pinned, separately provisioned image, build and start argv, readiness contract, controlled-clock request sequence, fake upstream endpoints recorded by the frozen observer, compared channels, repetitions and limits; unknown fields, unpinned images, shell strings and out-of-bound values fail before a plan is prepared.
- [x] #3 Definitions come from an explicit operator-selected file and are frozen by digest independently of both snapshots; a definition inside the captured repository that differs between base and candidate is shown as a changed oracle, and repository content never selects or authorizes a definition.
- [x] #4 The consent preview binds the definition digest, image, argv, observer topology, network policy, inputs and limits; any change requires new consent, and the invalidation matrix reopens pins when the definition changes.
- [x] #5 The payment experiment runs from a definition with unchanged observations (12 hours 1 to 2 provider calls, 30 seconds 1 to 1), and the runner, comparison and POC gate proofs pass, including both mutants. A second synthetic service in another language with no third-party dependencies runs end to end on its own pinned image.
- [x] #6 Services that need dependency downloads, external network or host access fail closed with a visible limitation; no image pulls, mounts, published ports or host fallback are added.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks and the opt-in Docker proofs pass; record exact commands, actual results or objective external blockers in task notes.
- [x] #2 Update RUNNER, SANDBOX, COMPARISON, IMPLEMENTATION and EXTENSIONS docs with the new scope and limits; independent verification for the execution boundary.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; keep private artifacts out of Git.
- [x] #4 Record extension seams in task notes: what needed code rather than data, and what differed from the existing instance; these feed the schema step in docs/EXTENSIONS.md.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a bounded version-1 HTTP definition and explicit file selection; freeze definition bytes in stored scenario inputs and consent, without repository auto-selection. Keep the payment definition as the default instance.
2. Generalize sandbox image selection, HTTP startup/readiness, frozen observer inputs, samples and comparison to definition cases/channels while retaining offline topology, explicit argv, bounded execution and changed-oracle/pin invalidation.
3. Add a dependency-free Python synthetic service and separately provision its pinned image (operator authorized); implement critical parsing, consent, isolation and comparison regressions and update affected documentation.
4. Run focused checks and Docker/POC proofs including mutants, then one independent execution-boundary verification; fix only concrete scoped defects.
5. Refresh authoritative claim and main, integrate verified commits, finalize task metadata and clean up the receipt-verified owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator explicitly selected the next available item among AFTER-48–50 and authorized a worktree, implementation commits, merge to main and cleanup. AFTER-48 is Done; selected dependency-ready AFTER-49. Existing AFTER-47 work and unrelated worktrees will be preserved.

Initial discovery: hard-coded cases span runner plan/run/basis, compare and CLI rendering/history; sandbox already accepts separate app/observer plans but hardcodes the Go image. Docker Colima Linux is available with the pinned Go image. Operator explicitly authorized separate resolution/pull of an official digest-pinned Python image for the second-language proof. Reviewed repository Worktrunk hooks (toolchain, frozen dependencies, Lefthook; no services); owned worktree after-49-http created successfully. Receipt is in the Git-local worktree metadata; original session ownership is retained there.

Architecture decision after inspecting launcher delivery: version-1 definitions explicitly declare the language-neutral fd3 readiness-URL ABI, including Python; unmodified-service HTTP polling is not promised. Shipped launcher code will be compiled only after exact consent in the pinned Go sandbox, with bounded transfer into the separately pinned service image; no host/capture-time build or prebuilt-helper packaging. Preview must bind preparation source/image/argv/limits and transfer policy, and failures remain incomplete. Separately provisioned the official Python 3.13.16 slim-trixie linux/arm64 manifest sha256:cf97c3b79da1c706532c7042f851654d97d3b88c8ca6520acda9cf4479ea9b14 under explicit operator permission. Runner remains pull-free.

Implementation workflow e27b47c0 completed focused architecture advice, but implementation child hit its default 30-minute timeout (process termination observed). Partial source changes remain uncommitted in the owned after-49-http worktree; no verification or integration claimed. Captured the partial diff and resumed the same child/session with a longer deadline (resumed run 6b75766b). Independent verification remains pending after implementation finishes.

Resumed implementation finished uncommitted. Implementer reports task check and final task cli:proof passed; CLI proof now retains an explicit changed-oracle fixture and asserts its warning plus both inventory paths rather than stale UI labels. Parent gopls diagnostics are clean for definition.go, runner/run.go and sandbox/plan.go. Independent verifier 1e04ae25 is checking the complete acceptance surface and actual Docker/POC mutant evidence (not yet accepted). Main advanced with AFTER-47 and fish-completion changes to 5db1eb4 while implementation remains based on 99e89a4; integration must preserve those changes and rerun affected checks.

Verification complete: independent verifier 1e04ae25 passed observable AC2–6 and found no concrete scoped defect. It independently ran task check:go, task check, task runner:proof, task comparison:proof, task http-service:proof, task cli:proof and a successful task test:poc with both mutation controls killed. Initial live attempts exposed stale PTY assumptions; no failed result was counted as success. Parent rechecked actual mutation output. Scope remains finite synthetic observations on Colima Linux arm64; Docker/kernel, pinned images and shipped observer/launcher are trusted. No amd64/Desktop certification or production correctness claim.

Delivery: implementation 676d914 and integration fixes 8052d2e fast-forward merged into main after preserving AFTER-47 and fish-completion work. Rebase conflicts were only generated TUI goldens; regenerated with task test:views -- -run ^TestGolden(Card)?Views$ -update. The merged gate exposed missing AFTER-47 DoD metadata (operator authorized repair in separate commit 4f5547d), two unused helpers (removed), an 80-column exact-ID clipping regression (fixed prompt wrapping to inner width and extended existing regression), and stale PTY style/spacing/fixture assumptions (preserved assertions with visible text matching and a real behavior-preserving changed fixture). Final post-integration task check, task test:tui-mutations, task cli:proof, task tui:proof and task test:poc passed. Final POC gate ran every required proof without skips and killed snapshot-freshness and dropped-provider-call mutants with their expected assertions. task check:staged passed before each commit; gopls clean on affected source. The full check includes race tests, build, vet, staticcheck U1000, formatting, 151 local links, 50-task backlog graph and no secret leaks.

Extension seams (DoD4): service variability is now versioned data (pinned image/platform, direct build/start argv, environment, fd3 readiness, controlled-clock cases, fake upstreams and channels). Python differs by no build step and its separately provisioned Python-only image. Native code was required for consented static-launcher compilation, separate bounded binary/diagnostic streams, fixed executable slots and materialization, preparation provenance validation, definition-driven comparison and case rendering. Reused schema-1 scenarios/receipts/artifacts/pins and existing invalidation; no plugin or general dependency-preparation system. CLI reads only explicit --definition FILE; definitions remain frozen independent of source snapshots and changed repository oracles are visible. Definition changes reopen pins. No remaining external blocker; only final metadata commit and receipt-verified checkout/workspace cleanup remain.

Cleanup complete: matched after-49-http checkout/branch against its original session creation receipt and fresh wt list; no active delegated work or matching Herdr workspace/panes remained. wt remove after-49-http --foreground --format=json removed the clean integrated checkout and branch, ran the post-remove hook successfully, and subsequent filesystem/branch/workspace checks confirmed absence. Unrelated pre-existing worktrees preserved. No push performed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered declarative http-service scenarios on main in 676d914 and 8052d2e: explicit frozen definitions, exact consent, isolated preparation/execution, honest comparisons and pins, and a dependency-free Python proof. Independent verification passed. Final task check, CLI/TUI live proofs and test:poc passed, including both mutants. Limits and extension seams documented. Owned worktree and branch removed; unrelated worktrees preserved. No remaining blocker or push.
<!-- SECTION:FINAL_SUMMARY:END -->

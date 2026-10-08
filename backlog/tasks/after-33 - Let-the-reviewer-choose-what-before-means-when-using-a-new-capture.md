---
id: AFTER-33
title: Let the reviewer choose what "before" means when using a new capture
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-08 02:27'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-30
  - AFTER-32
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/REVIEW.md
  - docs/README.md
priority: medium
type: feature
ordinal: 33000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The proposal (docs/README.md) says the comparison selector states exactly what "before" means: a first review compares base with candidate, and a follow-up review can compare the last inspected candidate with the latest one, without either silently replacing the other. The engine and headless CLI support both (`after review PIN --select ID --mode original_base|last_inspected`), but the TUI always keeps the original base. After an agent's follow-up edit, the reviewer wants to see and rerun what changed since they last looked, not the whole change again.

Scope, per docs/TUI-DESIGN.md "Prompts" and "Frame": the `u` prompt asks for the mode, defaulting to original base. Last inspected selects the pair (current candidate → new capture) and records `last_inspected` on each selected pin through `review.Select`. The header names the active mode, resume restores it, and reruns, attachments and computed diffs work for the follow-up pair. Changing the mode happens at the next `u`.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 The `u` prompt offers `original base <id>` (the default) and `last inspected <id>`, states the pair each produces, and requires the confirmation and reason from AFTER-30.
- [x] #2 Choosing last inspected selects (current candidate → new capture), records `last_inspected` with the prior candidate on every selected pin through `review.Select`, and keeps earlier receipts as history; the header names the active mode.
- [x] #3 In follow-up mode, `r` prepares and runs the plan for the follow-up pair, results attach only to that pair, and Diff shows the computed diff between the two candidates.
- [x] #4 `after review` resumes the saved pair and mode exactly, and `after pin PIN` shows the `last_inspected` selection the TUI recorded.
- [x] #5 PTY tests cover both modes, the Docker-gated proof covers a follow-up rerun when Docker settings are available, and docs/TUI.md and docs/REVIEW.md describe the TUI selector.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [x] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend the existing use-capture confirmation with an original-base/default or last-inspected selector and explicit pairs/reason. Reuse review.Select; preserve the original baseline independently of the active pair. 2. Persist and restore selected mode, original baseline, pair and pin revisions using the existing session mechanism with compatibility for existing sessions. Keep preview/run/attachment and computed diff bound to the selected pair. 3. Add focused model/session/action, CLI inspection/resume, PTY size/color and Docker-gated follow-up tests; update TUI and REVIEW docs. 4. Run relevant Task checks and independent acceptance verification, then commit implementation, integrate to main, finalize task metadata and remove the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation is in owned branch after-33-comparison-mode. Initial executor timed out at its 30-minute harness deadline; partial changes preserved and resumed after confirming no active test processes. Focused race tests TestFollowUpPreviewAndComputedDiffUseActivePair, TestFollowUpLateResultCannotAttachToDifferentPair and TestSnapshotUsePromptSelectsFollowUpsAndSwitchesBackForEveryPin passed. Remaining full Task/PTY/Docker checks and independent acceptance verification must pass before integration.

Delivered implementation 8cd044f and fast-forwarded it into main. Final verification: mise exec -- env GOFLAGS=-timeout=10m task check passed (format, race tests for all Go packages, docs links, backlog integrity, build, vet, gofmt, secrets); mise exec -- task check:staged passed before implementation commit. Focused test:computed, test:tui-mutations and test:views passed. LSP diagnostics clean for actions.go and loop_pty_test.go. Independent verifier passed criteria 1/2/4 and non-Docker pair-binding checks, but found a PTY readiness race. Fixed the test to wait for loaded records before sending capture; go test -race -count=3 -timeout=2m ./internal/cli -run ^TestReviewLaunchResumePTY$ passed all four 80x24/120x40 color/NO_COLOR variants three times (34.620s). Full CLI suite passed in 198.113s; earlier 3m timeout was too short, not a product deadlock. Regenerated missed card-header goldens. Docker was locally provisioned despite absent exported settings; with explicit trusted CLI and Colima Unix socket, mise exec -- task tui:proof passed in 333.227s, including real follow-up execution, exact-pair attachment, retained original receipt and computed diff. Corrected proof waits for renderer order, pin-mode reopen and horizontal panning to the candidate digest; no execution isolation was changed. PTY excerpt: original-base and last-inspected choices; follow-up preview/rerun attached only to its selected pair; computed follow-up diff; terminal restored. Warm help input-to-render during the actual run: 17.03225ms. Ordinary tests still intentionally skip unrelated opt-in Docker proofs; no full POC release-gate or native Linux claim. No remaining blocker or resumable implementation step.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added original-base/default and last-inspected selection to the confirmed TUI capture prompt, mode-aware headers and pin output, exact saved pair/mode/baseline/revision resume, and follow-up rerun/diff coverage. Integrated 8cd044f into main. Full task check, staged checks, repeated four-variant PTY matrix and real Docker tui:proof pass; independent verification findings corrected and affected checks rerun. No blockers; final task metadata is committed separately.
<!-- SECTION:FINAL_SUMMARY:END -->

---
id: AFTER-51
title: Deliver frozen stdin to command scenario containers
status: Done
assignee:
  - '@pi'
created_date: '2026-10-09 13:37'
updated_date: '2026-10-09 14:48'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-50
documentation:
  - docs/RUNNER.md
  - docs/SANDBOX.md
type: bug
ordinal: 51000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
AFTER-50 command cases freeze `stdin_base64` into the consent-bound plan, but the target container is created without Docker's `--interactive` flag, so `docker start --attach --interactive` never opens stdin and every program reads EOF. Found while writing examples/cli/07-command-scenario.sh: a quote CLI whose candidate changed behavior on a stdin case reported EQUAL because both sides printed {"error":"EOF"}. A direct Docker probe confirmed it (create without -i: no output; with -i: input echoed). The AFTER-50 command proof used only empty stdin, so no Docker-level test exercised delivery. Input files work. This is a correctness defect in delivered evidence: comparisons on stdin-driven cases silently compare EOF behavior.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A command case with non-empty frozen stdin receives exactly those bytes in its fresh container, shown by the recorded container-boundary stdout in an opt-in Docker proof
- [x] #2 Stdin attachment is part of the consent-bound plan (visible Docker policy) and is absent from HTTP-service and preparation containers
- [x] #3 A runnable examples/cli/07-command-scenario.sh shows a stdin-driven changed case and an unchanged JSON control through capture, consent and run, and is listed in examples/README.md
- [x] #4 Inspecting a command comparison whose exit status changed renders its card (root-path "" witness changes are valid) instead of the strict-decoding raw fallback
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks and the opt-in Docker proofs pass; record exact commands and outcomes in task notes
- [x] #2 Update affected docs/help and record limitations
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no private artifacts committed
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. In the owned worktree, bind --interactive into command plan Docker policy in sandbox.Plan.WithCommandInput (only command plans), so preview/consent show it; remove unused duplicate PrepareCommandImage constructor.
2. Extend TestCommandCLIProof: control case receives non-empty frozen stdin and the stored stdout artifact must contain those exact bytes.
3. Run task command:check, task command:proof, task http-service:proof, task check:go; regenerate goldens only if consent views change.
4. Add examples/cli/07-command-scenario.sh (quote CLI; stdout JSON, typo case exits 0) and README row; run it end to end.
5. Integrate into main, finalize task from primary, clean up the receipt-verified worktree.

6. Accept JSON-pointer root path "" in browser strictReport (command exit_status witnesses) with a Load-level regression; example shows witnesses via after compare --json.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator selected: fix this AFTER-50 stdin defect as AFTER-51, then add the command-scenario example. Authorized implementation in a worktree, integration into main and cleanup; no push.

Fix verified: extended command:proof fails without the policy change (base/control/0/stdout lacked frozen stdin) and passes with it. Running the example exposed a second AFTER-50 defect: exit_status witnesses use JSON-pointer root path "" and browser strictReport rejects empty paths, so inspect/TUI fall back to raw for any changed exit status. Operator approved including the fix in AFTER-51.

Validation (worktree after-51-stdin, Colima Linux arm64): task command:check passed; task command:proof passed with fix and failed without it (base/control/0/stdout = "stable control: " lacking frozen stdin); new TestComparisonCardAcceptsRootPathWitnessChange fails without the cards.go change and passes with it; task check:go passed (race tests, build, vet, staticcheck U1000, gofmt); task http-service:proof passed; task test:poc passed (all required proofs, both negative controls killed); task test and task check:staged passed; gopls clean on plan.go and cards.go. Example 07 ran end to end via a PTY: order case EQUAL on exit/stdout/stderr despite reordered JSON keys; typo case exit 2 -> 0, stdout error removed/total_cents 0 added, stderr changed; run exit 4. Stored plan contains --interactive once per command container (2 sides x 2 cases); WithCommandInput is called only from prepare_command.go, so HTTP-service and preparation plans are unchanged. Removed unused duplicate sandbox.PrepareCommandImage. Fast-forward integrated c5128b2 into main. No independent verifier: small change directly exercised by the failing-then-passing Docker proof and full POC gate.

Completion hygiene: added and checked the repository DoD gate missing at creation (check-backlog requires it). Evidence is in the validation note above: Docker proofs/test:poc passed, docs/RUNNER.md and examples/README.md updated, c5128b2 and final state committed with no private artifacts.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Fixed two AFTER-50 defects found while writing a command example: command containers are now created --interactive (consent-bound in Docker policy) so frozen stdin actually reaches the target, and command comparisons with a changed exit status (root-path "" witness) render as cards instead of the raw fallback. Added examples/cli/07-command-scenario.sh (quote CLI: JSON control equal despite key order; misspelled field now exits 0 with a $0 quote). Verified by command:proof (fails without fix), a new card regression, check:go, http-service:proof, test:poc and an end-to-end example run. Delivered in c5128b2.
<!-- SECTION:FINAL_SUMMARY:END -->

---
id: AFTER-10
title: Deliver the complete headless review workflow
status: Done
assignee: []
created_date: '2026-10-03 05:42'
updated_date: '2026-10-03 19:36'
labels:
  - poc
  - cli
milestone: m-0
dependencies:
  - AFTER-4
  - AFTER-5
  - AFTER-9
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 10000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A testable headless loop must exist before the TUI can hide engine faults. Scope: command composition over capture/import/run/compare, stable JSON output and real integration checks. Keep the vocabulary in docs/IMPLEMENTATION.md small and avoid adding another workflow runtime.

Use urfave/cli for command and flag parsing behind a thin cmd/after/main.go. Model internal/config on github.com/brettinternet/worklease/internal/config: explicit input values and injectable environment access, a typed resolved Config, and per-setting Sources identifying the winning configuration layer. Adapt the shape rather than copying Worklease-specific settings or introducing speculative configuration knobs. Configuration provenance must be inspectable, not lost during CLI parsing.

Include YAML configuration support in this planned workflow, after the CLI/config foundation: explicit flags override AFTER_* environment variables, which override YAML values, which override defaults. Preserve explicit flag presence so parser defaults do not masquerade as user overrides. Document supported settings, config-file selection, empty-value semantics and source reporting. Configuration loading must not execute repository code or supply blanket execution consent.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Documented CLI commands capture, import, inspect and export bounded machine-readable comparisons using stable IDs, preserving a useful no-evidence diff workflow.
- [x] #2 Run commands preview exact execution plans and require interactive confirmation or a specific noninteractive authorization digest; imports/inspection/help never execute code and non-TTY commands never hang on prompts.
- [x] #3 An integration test drives the real payment comparison through CLI subprocesses and inspects actual responses/effects/provenance; imported pass/fail is visibly reported rather than AFTER-observed.
- [x] #4 JSON output is versioned and does not mix progress/ANSI on stdout; diagnostics go to stderr and documented exit statuses distinguish invalid input, denied execution, operational failure and comparison findings.
- [x] #5 Models, GitHub access and accounts are absent from the path; missing Docker still permits import/diff/inspection. Errors leave terminal/process/store state usable.
- [x] #6 CLI tests cover paths with spaces, bad IDs, unreadable artifacts, bounded output and injection strings; help examples work from a fresh prepared checkout.
- [x] #7 The entry point and command layer use urfave/cli with a thin main.go; focused tests preserve help/version, invalid-input exit behavior and stdout/stderr separation.
- [x] #8 A typed internal/config resolver modeled on Worklease exposes each effective setting and its winning source (flag, env, file or default). Tests exercise precedence and distinguish explicitly supplied values from CLI defaults, including supported false/zero values.
- [x] #9 YAML configuration is supported with documented file discovery and explicit path selection. Missing optional default files are allowed; missing explicit files, malformed YAML, unknown or duplicate keys and invalid types produce actionable errors identifying the setting/source. Tests cover flags > environment > YAML > defaults and empty-value semantics.
- [x] #10 Users can inspect effective configuration and per-setting provenance through documented CLI output without exposing sensitive values or executing project code. Configuration never substitutes for exact execution-plan authorization.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Compose existing capture/store/report/rawdiff/runner/compare APIs behind urfave/cli and a thin entry point; keep no-evidence capture inspection usable. 2. Add a bounded strict YAML typed config resolver patterned on the local Worklease Input/Config/Sources design, with explicit presence and flags > environment > file > default precedence; exclude consent. 3. Persist or reconstruct exact previewed runner plans across CLI invocations without changing their random request binding; require specific digest authorization, separate JSON/stdout from sanitized diagnostics, and document exits. 4. Add subprocess integration and adversarial configuration/CLI tests plus a real Docker payment proof Task target; update CLI documentation. 5. Run focused and full relevant checks, independent acceptance verification focused on consent/config trust boundaries, fix concrete findings, integrate and commit to main.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Takeover: prior workflow dcd17f18 stopped on session reload before edits; clean after-10-cli checkout at 9f05c8c adopted after matching original Git-local receipt and prior task/session evidence. Same-protocol retry 9bf29d23 implements then independently verifies the scoped consent/configuration boundary. Authoritative backlog remains on main.

Implemented urfave/cli headless capture/import/inspect/export/compare/run/config, strict layered YAML and per-setting sources, bounded JSON and artifact/diff pages, and exact saved-plan authorization. Independent verifier ba5a1515 passed AC1-2 and AC4-10 plus full checks, and found a real AC3 failure: observer artifact IDs were not inspectable. Parent added bounded exact-byte artifact inspection with optional complete JSON document and precision/injection/page regression tests; no second general review. Verification after fix: mise exec -- task format:go; mise exec -- task test:cli (race tests passed); explicitly configured local Docker plus mise exec -- task cli:proof PASSED TestPaymentCLIProof in 177.14s (actual identical responses, 12h provider requests 1 vs 2, 30s control 1 vs 1, retained provenance and imported reported pass/fail); mise exec -- task check PASSED; git diff --check PASSED; mise exec -- task check:staged PASSED. LSP diagnostics clean on changed workflow and artifact test files. No models/accounts used. No private artifacts staged. Remaining scope is payment-only runner; TUI and pins remain future tasks.

Delivery: implementation commit 4a8d8be fast-forwarded into main, preserving authoritative claim edits; mise exec -- task test:cli passed again on main. During takeover the old executor was discovered still running despite its parent reporting stopped; it was explicitly interrupted and confirmed terminal before sole-writer reconciliation continued. Current implementation and verifier runs completed. Adopted after-10-cli checkout is retained because the Herdr workbench plugin is unavailable, preventing required pane/unsaved-state verification for safe cleanup. Only its generated private writer lock is untracked; it was excluded from the implementation commit. No delivery blocker; no push. Final task metadata is committed separately on main.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered the complete headless workflow in 4a8d8be: stable-ID capture/import/inspect/export/compare, urfave/cli, strict YAML configuration with provenance, and exact saved-plan authorization. Independent verification found an observer-artifact inspection gap; fixed with bounded byte/JSON pages and regression tests. Full task check, focused race tests, staged checks, and actual Docker-backed CLI payment proof pass. All acceptance criteria met; payment-only execution scope and future TUI/pin work documented.
<!-- SECTION:FINAL_SUMMARY:END -->

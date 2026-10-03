---
id: AFTER-10
title: Deliver the complete headless review workflow
status: To Do
assignee: []
created_date: '2026-10-03 05:42'
updated_date: '2026-10-03 14:49'
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
- [ ] #1 Documented CLI commands capture, import, inspect and export bounded machine-readable comparisons using stable IDs, preserving a useful no-evidence diff workflow.
- [ ] #2 Run commands preview exact execution plans and require interactive confirmation or a specific noninteractive authorization digest; imports/inspection/help never execute code and non-TTY commands never hang on prompts.
- [ ] #3 An integration test drives the real payment comparison through CLI subprocesses and inspects actual responses/effects/provenance; imported pass/fail is visibly reported rather than AFTER-observed.
- [ ] #4 JSON output is versioned and does not mix progress/ANSI on stdout; diagnostics go to stderr and documented exit statuses distinguish invalid input, denied execution, operational failure and comparison findings.
- [ ] #5 Models, GitHub access and accounts are absent from the path; missing Docker still permits import/diff/inspection. Errors leave terminal/process/store state usable.
- [ ] #6 CLI tests cover paths with spaces, bad IDs, unreadable artifacts, bounded output and injection strings; help examples work from a fresh prepared checkout.
- [ ] #7 The entry point and command layer use urfave/cli with a thin main.go; focused tests preserve help/version, invalid-input exit behavior and stdout/stderr separation.
- [ ] #8 A typed internal/config resolver modeled on Worklease exposes each effective setting and its winning source (flag, env, file or default). Tests exercise precedence and distinguish explicitly supplied values from CLI defaults, including supported false/zero values.
- [ ] #9 YAML configuration is supported with documented file discovery and explicit path selection. Missing optional default files are allowed; missing explicit files, malformed YAML, unknown or duplicate keys and invalid types produce actionable errors identifying the setting/source. Tests cover flags > environment > YAML > defaults and empty-value semantics.
- [ ] #10 Users can inspect effective configuration and per-setting provenance through documented CLI output without exposing sensitive values or executing project code. Configuration never substitutes for exact execution-plan authorization.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

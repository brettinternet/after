---
id: AFTER-52
title: 'Show command witness values in readable run, compare and card output'
status: To Do
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 14:48'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-51
documentation:
  - docs/COMPARISON.md
  - docs/RUNNER.md
priority: medium
type: enhancement
ordinal: 52000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
For command scenarios, readable `after run`, `after compare` and the `after inspect` / TUI card show only per-channel EQUAL/DIFFERENT and "N stored witnesses". The actual difference (for example exit 2 -> 0, a removed `/error` key, changed stderr bytes) is only reachable through `--json`, which is how examples/cli/07-command-scenario.sh has to show it. Payment runs already summarize provider counts readably. Reviewers need to see what changed without jq, while untrusted program output stays safe: text witnesses are stored as base64 bytes so terminal controls never reach the terminal raw.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Readable run/compare output and the command comparison card show, per differing case and channel, the exit status before/after and bounded JSON-path changes, with repetition instability called out
- [ ] #2 Differing text stream witnesses render as sanitized, bounded text (or an explicit binary/invalid-UTF-8 marker) and never emit raw terminal control bytes
- [ ] #3 Equal cases stay one line; output for many cases/changes is bounded with a pointer to the full JSON or card
- [ ] #4 Existing CLI/TUI goldens cover a changed exit status, a JSON stdout change and a hostile text stream
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [ ] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

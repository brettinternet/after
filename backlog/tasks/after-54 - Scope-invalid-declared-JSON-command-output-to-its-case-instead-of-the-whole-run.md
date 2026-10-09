---
id: AFTER-54
title: >-
  Scope invalid declared-JSON command output to its case instead of the whole
  run
status: To Do
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 14:48'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-50
documentation:
  - docs/COMPARISON.md
  - docs/RUNNER.md
priority: low
type: enhancement
ordinal: 54000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
When a command definition declares stdout or stderr as `json`, one sample with invalid or empty JSON makes the entire comparison fail (whole run incomparable), discarding valid witnesses for every other case. Error paths of real CLIs often print nothing or plain text on stdout, so examples/cli/07-command-scenario.sh had to make its CLI print JSON even for errors. Incomparability should stay honest but be scoped to the case/channel that caused it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Invalid declared JSON in one case/channel marks that witness incomparable with a fixed diagnostic, and other cases still compare; the overall outcome is never equal while any witness is incomparable
- [ ] #2 Comparison rules and stored details stay strictly versioned; an existing stored receipt re-derives the same or a more conservative result
- [ ] #3 Focused compare tests cover invalid JSON in one case alongside a valid changed case and a valid equal case
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [ ] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

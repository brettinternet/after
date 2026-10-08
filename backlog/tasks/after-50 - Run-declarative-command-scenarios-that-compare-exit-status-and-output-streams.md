---
id: AFTER-50
title: Run declarative command scenarios that compare exit status and output streams
status: To Do
assignee: []
created_date: '2026-10-08 23:14'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-49
documentation:
  - docs/EXTENSIONS.md
  - docs/RUNNER.md
  - docs/SANDBOX.md
  - docs/COMPARISON.md
priority: medium
type: feature
ordinal: 50000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
CLIs, code generators and library test harnesses are observable at the process boundary: argv, stdin and a fixed environment go in; exit status, stdout and stderr come out. docs/behavior-and-evidence.md lists this as the useful comparison for CLI and library changes, and it applies to any language whose program runs in a pinned image. A `command` scenario kind reuses the definition, freezing and consent machinery from AFTER-49 and needs no observer container: the runner records streams from the Docker attachment instead of trusting program-reported output. Output file trees and interactive TTY programs are out of scope here.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An explicit operator selection is recorded before implementation; this follow-up is not selected automatically from the POC queue.
- [ ] #2 A versioned command definition declares a pinned image, build argv, and cases (argv, stdin bytes, fixed environment, optional input files), repetitions and limits; it follows the same strict decoding, explicit selection, digest freezing, consent and changed-oracle rules as http-service definitions.
- [ ] #3 Each case, side and repetition runs in fresh offline sandbox state; exit status, stdout and stderr are recorded at the container boundary with completeness and truncation flags; timeouts, output overflow, signals and build failures produce incomplete receipts, never equality.
- [ ] #4 Comparison is exact for exit status and text streams and structural for streams the definition declares as JSON, with exact witnesses, all repetitions retained (disagreement is unstable) and the rules digest bound; missing, truncated or redacted streams are incomparable.
- [ ] #5 A synthetic CLI fixture shows a changed case and an unchanged control end to end through capture, consent, run, compare, pin and reopen; tests cover hostile terminal output, binary stdout and nondeterministic output reported as unstable.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks and the opt-in Docker proofs pass; record exact commands, actual results or objective external blockers in task notes.
- [ ] #2 Update RUNNER, COMPARISON, IMPLEMENTATION and EXTENSIONS docs with the new scope and limits; independent verification for the execution boundary.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; keep private artifacts out of Git.
<!-- DOD:END -->

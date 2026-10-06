---
id: AFTER-24
title: Launch and resume TUI reviews with --capture and --session
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies: []
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/CLI.md
  - docs/CAPTURE.md
priority: medium
type: feature
ordinal: 24000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Opening the TUI needs a 71-character candidate ID and a 71-character `--base` ID from an earlier `after capture`. Resuming means rebuilding the command with every pin revision and comparison ID (up to 32 `--evidence` flags) from the session JSON printed on quit. Because the TUI draws on stdout, that JSON cannot simply be redirected to a file. Reviewers re-review repeatedly, so resuming should be one command.

Scope, per docs/TUI-DESIGN.md "Launch and resume": `--capture` (an explicit request equivalent to the default `after capture`: no project execution, untracked files excluded); `--session FILE` (an explicit, user-named file, written atomically with mode 0600 after every selection change and on quit, and read as untrusted input); the four flag combinations in the design's table; and a ready-to-paste resume command on stderr at quit. The session JSON on stdout stays. Session files are never discovered, nothing is captured on open without `--capture`, and nothing else runs on open.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 `after review --tui --capture` captures HEAD against the working tree exactly as `after capture` does (untracked files excluded, no project code run) and opens that pair; a capture failure exits with a sanitized error after restoring the terminal.
- [ ] #2 `--session FILE` follows the design's table: it creates the file on first use, reopens the exact saved selection, offers a new capture as pending when combined with `--capture`, errors when the file is missing without `--capture`, and refuses explicit IDs when the file already holds a session.
- [ ] #3 The session file is written atomically with mode 0600 after every selection change and on quit, contains the same JSON as the quit output and no source content, and an interrupted write leaves the previous valid file in place.
- [ ] #4 Reading a session file rejects non-regular files, oversized content, unknown fields, invalid IDs, more than 32 evidence IDs and a different project, each with a clear error and no store writes.
- [ ] #5 On quit, stdout still carries the session JSON and stderr carries a resume command that reopens the same selection; CLI and PTY tests cover create, resume and each error case, and docs/CLI.md and docs/TUI.md document the flags.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

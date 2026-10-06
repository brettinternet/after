---
id: AFTER-24
title: Start and resume reviews with a plain after review
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-06 21:41'
labels:
  - poc
  - tui
  - ux
  - cli
milestone: m-1
dependencies:
  - AFTER-23
  - AFTER-36
  - AFTER-38
documentation:
  - docs/CLI-DESIGN.md
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/CLI.md
  - docs/CAPTURE.md
priority: high
type: feature
ordinal: 24000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Reviewing takes four steps: capture, copy two 71-character IDs, run `review --tui CANDIDATE --base BASE`, then rebuild that command with up to 32 `--evidence` flags to resume. The TUI draws on stdout, so its session JSON can't be piped to a file. Reviewers re-review repeatedly, so one command should start and resume a review.

Scope, per docs/CLI-DESIGN.md "review" and docs/TUI-DESIGN.md "Launch and resume": `after review` captures with the same flags and policy as `after capture`, then opens the pair. The saved review in `.after/session.json` holds the pair, the comparison mode (always original base until AFTER-33) and the capture flags. It is UI state, not evidence, written atomically with mode 0600 and read as untrusted input. With a saved review, the TUI opens at once and captures in the background, a changed capture waits as pending (`u`), and different capture flags start a new saved review; `c` reuses the saved capture flags. Explicit IDs or a pair open without capturing or touching the saved review. Evidence is discovered: pin heads, the pair's newest runs and comparisons, and reports bound to the candidate, within 32 records with pins needing another look first. On quit, stderr names the saved review, and stdout carries the session JSON only with `--json`. Without a terminal, exit 2 pointing to `after status --json`. Nothing else runs on open.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Without a saved review, `after review` in a checkout captures exactly as `after capture` does with the same flags (`--staged`, `--base REF [--target REF]`, `--include-untracked PATH`), showing elapsed time on stderr after one second, writes a capture record, and opens that pair; untracked files stay excluded, no project code runs, and a capture failure exits with the specific sanitized reason after restoring the terminal.
- [ ] #2 `.after/session.json` holds the pair, mode and capture flags and no source content; it is written atomically with mode 0600 after every selection change and on quit, an interrupted write leaves the previous valid file, and a non-regular, oversized, unknown-field or invalid-ID file is reported on stderr and replaced at the next save.
- [ ] #3 With a saved review, `after review` opens its pair at once and captures in the background, as `c` does; a capture that differs is offered as pending (`u`) without changing the selected pair, and a failed background capture is reported without closing the review. Different capture flags start a new saved review, and stderr names the replaced one.
- [ ] #4 `after review ID…` and `after review BASE CANDIDATE` open stored records or the pair without capturing and without reading or writing the saved review, and an explicit pin revision opens exactly that revision.
- [ ] #5 Discovery loads each pin's head revisions (forks separately), the pair's newest runs and comparisons, and reports bound to the candidate, at most 32 records with pins needing another look first; the TUI states how many matching records were not loaded and that `after log` lists them; tests cover forks, the limit and an explicit older revision.
- [ ] #6 On quit, stderr shows `Saved review <base> → <candidate> · after review resumes it` and stdout is empty unless `--json`; without a terminal, `after review` exits 2 naming `after status --json`; PTY tests cover start, resume, a pending capture, a replaced review and explicit IDs, and docs/CLI.md and docs/TUI.md document the command.
- [ ] #7 `c` captures again with the saved review's capture flags, or HEAD against the working tree when explicit IDs opened the review, and the help overlay says which.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

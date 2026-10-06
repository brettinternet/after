---
id: AFTER-33
title: Let the reviewer choose what "before" means when using a new capture
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-06 21:14'
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
- [ ] #1 The `u` prompt offers `original base <id>` (the default) and `last inspected <id>`, states the pair each produces, and requires the confirmation and reason from AFTER-30.
- [ ] #2 Choosing last inspected selects (current candidate → new capture), records `last_inspected` with the prior candidate on every selected pin through `review.Select`, and keeps earlier receipts as history; the header names the active mode.
- [ ] #3 In follow-up mode, `r` prepares and runs the plan for the follow-up pair, results attach only to that pair, and Diff shows the computed diff between the two candidates.
- [ ] #4 `after review` resumes the saved pair and mode exactly, and `after pin PIN` shows the `last_inspected` selection the TUI recorded.
- [ ] #5 PTY tests cover both modes, the Docker-gated proof covers a follow-up rerun when Docker settings are available, and docs/TUI.md and docs/REVIEW.md describe the TUI selector.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

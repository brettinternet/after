---
id: AFTER-57
title: Write the extension contract as versioned data schemas
status: To Do
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 14:48'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-48
  - AFTER-49
  - AFTER-50
documentation:
  - docs/EXTENSIONS.md
  - docs/SCHEMA.md
priority: low
type: docs
ordinal: 57000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
docs/EXTENSIONS.md "Next decisions" step 1 says that after AFTER-48–50 the contract should be written as versioned data schemas (importer output cards, scenario definitions, observation artifacts and comparison rules) independent of how plugins would load or run, with built-ins conforming first. AFTER-48–50 recorded extension seams in their task notes for exactly this step, but nothing tracks it. Step 2 (whether code plugins are needed at all) depends on it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Machine-checkable schemas exist for report cards, http-service and command definitions, command/HTTP observation artifacts and comparison rules, each versioned and matching current strict decoders
- [ ] #2 Built-in fixtures (payment, Python service, command proof definition, JUnit/Go report output) validate against the schemas in an existing Task check
- [ ] #3 docs/EXTENSIONS.md records which AFTER-48–50 seams the schemas capture and what still needs code; no plugin runtime is added
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [ ] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

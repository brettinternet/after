---
id: AFTER-49
title: Run declarative HTTP-service scenarios on the paired runner
status: To Do
assignee: []
created_date: '2026-10-08 23:14'
labels:
  - follow-up
  - extensibility
dependencies: []
documentation:
  - docs/EXTENSIONS.md
  - docs/RUNNER.md
  - docs/SANDBOX.md
  - docs/COMPARISON.md
  - docs/IMPLEMENTATION.md
priority: medium
type: feature
ordinal: 49000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The runner hardcodes one experiment: the payment ABI (`go.mod`, `app/main.go`), the pinned golang image, fixed build/start argv, two same-key requests, the fake-provider observer and fixed comparison rules. Any other HTTP service needs new Go code in AFTER, which is the coverage problem docs/EXTENSIONS.md describes. Most language variety is which image, which build and start argv, and which stimuli: data that can appear in the consent preview instead of code. Generalize the runner around a declarative `http-service` scenario definition and make the payment experiment one instance of it. This is the first concrete scenario kind from which a later plugin protocol may be extracted; it is not a plugin framework, universal adapter or dependency-preparation system. It widens the POC scope in docs/IMPLEMENTATION.md, which must be updated with it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An explicit operator selection is recorded before implementation; this follow-up is not selected automatically from the POC queue.
- [ ] #2 A versioned, strictly decoded http-service definition declares a digest-pinned, separately provisioned image, build and start argv, readiness contract, controlled-clock request sequence, fake upstream endpoints recorded by the frozen observer, compared channels, repetitions and limits; unknown fields, unpinned images, shell strings and out-of-bound values fail before a plan is prepared.
- [ ] #3 Definitions come from an explicit operator-selected file and are frozen by digest independently of both snapshots; a definition inside the captured repository that differs between base and candidate is shown as a changed oracle, and repository content never selects or authorizes a definition.
- [ ] #4 The consent preview binds the definition digest, image, argv, observer topology, network policy, inputs and limits; any change requires new consent, and the invalidation matrix reopens pins when the definition changes.
- [ ] #5 The payment experiment runs from a definition with unchanged observations (12 hours 1 to 2 provider calls, 30 seconds 1 to 1), and the runner, comparison and POC gate proofs pass, including both mutants. A second synthetic service in another language with no third-party dependencies runs end to end on its own pinned image.
- [ ] #6 Services that need dependency downloads, external network or host access fail closed with a visible limitation; no image pulls, mounts, published ports or host fallback are added.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks and the opt-in Docker proofs pass; record exact commands, actual results or objective external blockers in task notes.
- [ ] #2 Update RUNNER, SANDBOX, COMPARISON, IMPLEMENTATION and EXTENSIONS docs with the new scope and limits; independent verification for the execution boundary.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; keep private artifacts out of Git.
<!-- DOD:END -->

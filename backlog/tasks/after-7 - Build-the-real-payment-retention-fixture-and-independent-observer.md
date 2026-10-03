---
id: AFTER-7
title: Build the real payment-retention fixture and independent observer
status: To Do
assignee: []
created_date: '2026-10-03 05:40'
labels:
  - poc
  - fixtures
  - runner
milestone: m-0
dependencies:
  - AFTER-3
  - AFTER-6
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 7000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Turn the presentation into a measurable experiment, not preloaded answers. Scope: a standard-library Go payment application fixture, frozen HTTP driver, controlled clock, and independently recording fake provider. Generate disposable Git test snapshots instead of hardcoding repository commit IDs. This supports a bounded sequential experiment, not production billing.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Two captured revisions differ in retention configuration (24 hours versus 5 minutes) while the HTTP driver and fake-provider observer are byte-identical; both actual applications build/run under the sandbox policy.
- [ ] #2 The same-key twelve-hour request sequence records one provider request on the base and two on the candidate while HTTP response status/body remain identical; counts derive from received traffic, never expected-output constants.
- [ ] #3 The thirty-second same-key control records one request on both versions; different-key/expired-key controls demonstrate that multiple requests can be intended. Each version/case starts with empty independent state.
- [ ] #4 One controlled clock drives creation and expiry with documented threshold semantics; advancing it cannot disagree with the app's actual key storage clock.
- [ ] #5 Deliberately disabling app deduplication changes the observer's captured calls; the fake provider does not deduplicate on the app's behalf. Tampering with an app-printed count does not change the independent count.
- [ ] #6 Fixture README states concrete inputs, supported channels, synthetic data, sequential/fake-provider limits and the exact reproduction command; no credentials, real payments, sleeps for simulated hours or network services are needed.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

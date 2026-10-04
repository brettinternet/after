---
id: AFTER-7
title: Build the real payment-retention fixture and independent observer
status: Done
assignee: []
created_date: '2026-10-03 05:40'
updated_date: '2026-10-04 02:17'
labels:
  - poc
  - fixtures
  - runner
  - reviewed
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
- [x] #1 Two captured revisions differ in retention configuration (24 hours versus 5 minutes) while the HTTP driver and fake-provider observer are byte-identical; both actual applications build/run under the sandbox policy.
- [x] #2 The same-key twelve-hour request sequence records one provider request on the base and two on the candidate while HTTP response status/body remain identical; counts derive from received traffic, never expected-output constants.
- [x] #3 The thirty-second same-key control records one request on both versions; different-key/expired-key controls demonstrate that multiple requests can be intended. Each version/case starts with empty independent state.
- [x] #4 One controlled clock drives creation and expiry with documented threshold semantics; advancing it cannot disagree with the app's actual key storage clock.
- [x] #5 Deliberately disabling app deduplication changes the observer's captured calls; the fake provider does not deduplicate on the app's behalf. Tampering with an app-printed count does not change the independent count.
- [x] #6 Fixture README states concrete inputs, supported channels, synthetic data, sequential/fake-provider limits and the exact reproduction command; no credentials, real payments, sleeps for simulated hours or network services are needed.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a standard-library synthetic payment app plus frozen HTTP driver/fake-provider recorder. Use one driver-owned clock for creation and expiry, separate app process, fresh state per case, and explicit sequential limits; do not claim hostile-candidate observer isolation (AFTER-8).
2. Generate disposable Git revisions for 24h/5m retention and dedup/count mutations; capture with existing capture/store APIs and execute frozen bytes only through existing consent-bound offline sandbox. Validate actual response/call logs, boundary and intended-repeat controls.
3. Add opt-in Task reproduction and fixture README, run focused/Go/repository checks and real Docker experiment, obtain one independent trust-boundary verification, then integrate and commit on main.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented the synthetic fixture in after-7-payment using captured Git revisions, separate app process, driver-owned clock, and provider-received HTTP logs. First live offline proof passed all four revisions and nine cases per revision: twelve-hour base/candidate 1/2, thirty-second 1/1, different/expired 2/2, threshold and no-sliding-expiry controls, disabled-dedup traffic mutation, printed-count invariance. mise exec -- task check:go and task test passed; changed Go files have clean LSP diagnostics. Independent acceptance verification is running; no observer security-boundary claim is made before AFTER-8. No external services, implicit pulls or host fixture execution.

Explicit operator handoff: taking over existing staged after-7-payment implementation. Preserve original creation receipt; cleanup ownership remains unverified because historical session access is scope-blocked. Revalidate existing implementation and integrate to main; do not start another item.

Delivery: implementation dbae18c fast-forwarded to main with authoritative task edits preserved. Revalidated mise exec -- task check:go (build/race/vet/gofmt), mise exec -- task test (Go, 50 links, 20 tasks), and mise exec -- task check:staged: all passed. With explicitly configured local Docker CLI and Colima socket, mise exec -- task fixtures:payment passed four revisions x nine cases in 95.231s. Independent verifier run e25d5eb0-32a4-41df-bc14-d92d062bf3a2 completed: all six AC pass, no blockers, separate live proof passed in 95.431s. Counts come from exact received traffic; threshold/no-sliding and both mutations passed. No source corrections needed after verification. General hostile-candidate isolation remains AFTER-8, not a fixture claim. Retain .worktrees/after-7-payment and its branch: historical ownership continuity could not be fully verified for destructive cleanup. Existing after-2-storage checkout untouched. Final task metadata is committed separately on main; no push authorized. No remaining implementation blocker; next eligible work requires a new request.

Review 2026: No defects found. task fixtures:payment PASS: base 1 vs candidate 2 provider calls for 12h same-key, 30s control 1/1, expired/different-key 2, threshold at 5m/24h, no sliding expiry, disabled-dedup and printed-count controls.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered real synthetic payment experiment in dbae18c: immutable generated Git snapshots, frozen driver/provider, controlled clock, observed 12h 1/2 and 30s 1/1 calls with equal responses, intended-repeat/boundary controls and dedup/printed-count mutations. All Go, repository, staged checks and independent live offline verification passed. Fixture limits and reproduction documented; all acceptance criteria satisfied.
<!-- SECTION:FINAL_SUMMARY:END -->

# Review-loop study kit v1

This is a runnable technical kit, not a completed human study. Generated runs are **author rehearsals**; there are no participant results, usability gains, or market-value findings. AFTER-18 requires explicit human authorization.

## Prepare and freeze

Use a prepared checkout and the [demo prerequisites](DEMO.md), including separately provisioned pinned Docker image. Use only the kit's synthetic payment files, never a participant's repository or credentials.

```sh
mise exec -- task study:check
mise exec -- task study:assign -- --assign-seed cohort-01 --participants 12
# Set AFTER_DOCKER_BINARY and AFTER_DOCKER_HOST as described in DEMO.md.
mise exec -- task study:proof -- --keep
```

`study:proof` explicitly authorizes six synthetic offline runs and validates the independently observed keys. It fails on incomplete runs, wrong responses/call counts, missing controls, cleanup failures, or failure to reopen stale/missing evidence. `--keep` retains the private kit; omit it for automatic cleanup. `study:check` needs no Docker: it checks assignment consistency and captures the three initial cases, including changed-oracle inventory, but does not validate observed keys. For interactive per-plan consent, use `task demo -- --study --keep`. No silent pull or host-execution fallback is allowed. Do not distribute an incomplete generation without all three facilitator keys.

Before recruitment, freeze the Git commit, Go version, executable checksum, assignment seed/output, kit version, and successful proof log in private study storage. Preserve generated receipt IDs, which bind snapshots, driver, observer, rules, and runtime—not just screenshots. Keep generated paths and `.after` stores private and out of Git. A case or scoring change requires a new kit version and rehearsal.

## Cases and keys

The synthetic payment API is unfamiliar to eligible participants. Exclude AFTER implementers and anyone who has seen this fixture or its keys. Teach the API, not the retention defect. The cases are matched variants of one service, not three independent domains; record prior familiarity and analyze order/period effects.

| Case | Initial change from 24h                                     | Follow-up                                      | Twelve-hour provider calls, base → candidate |
| ---- | ----------------------------------------------------------- | ---------------------------------------------- | -------------------------------------------- |
| A    | Retention becomes 5m; test expectation is changed to match. | Restore 24h and the matching test.             | Initial 1→2; follow-up 1→1.                  |
| B    | Retention becomes 1h; test expectation is changed to match. | Comment only.                                  | Initial and follow-up 1→2.                   |
| C    | Retention becomes 6h; test expectation is changed to match. | Weaken test to a positive-retention assertion. | Initial and follow-up 1→2.                   |

Every run has identical HTTP responses on both sides and a thirty-second control with provider calls 1→1. Counts come from the runner's protected observer, not app stdout or expected-output files. `facilitator-key.json` is written after those checks pass and links the initial and independently executed follow-up receipts. `initial-observations.json` and `key-observations.json` contain the response/call documents used by assertions. Participant-edited tests are captured source, **not executed test reports**.

The finite expectation is one provider request for a repeated payment at twelve hours. Initial A/B/C violate it; A's follow-up repairs this witnessed defect, while B/C do not. B changes no executable behavior: whole-footprint invalidation is conservative, not proof of a regression. C changes only the oracle and adds no behavioral evidence. A changed oracle cannot redefine the frozen expectation.

### Phases

1. **Missing:** show captured code/diff, no observation. A labeled prediction is allowed; an observed/fresh claim is wrong. `missing-project/` is a separate pre-execution checkpoint with no receipt.
2. **Initial:** provide initial response/call artifacts to all conditions. A setup pin expresses the finite requirement, not participant acceptance.
3. **Follow-up:** show the controlled edit, old observation, and reopened pin. Applicability is stale, with no current result. Do not reveal the independently verified follow-up until the participant explains and chooses whether another experiment would help. Old evidence is not fresh even in B.

For this study's question, rerunning B only to recover the same finite example after a comment edit is scored avoidable; it can still be a reasonable conservative policy. Record the reason. Rerunning A to check the repair is useful. For C, a changed test is not proof of preserved behavior; verifying unchanged executable code is conservative, not evidence of a new effect.

Limits: sequential synthetic retries at 30s and 12h only; no real charges, concurrency, other intervals, production provider, or universal correctness. Score source inference separately from independent observation.

## Conditions and private packets

Each person sees each case once, in one condition. All conditions get the same scenario, finite expectation, source, diffs, evidence documents, and time budget; only navigation/presentation differs. Raw gets all artifacts too, so information availability is not confounded with interface quality.

Generated `A/`, `B/`, `C/` directories contain:

| Files                                                             | Contents                                                                                                          |
| ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `initial.diff`, `followup.diff`                                   | Full tracked app/oracle diffs vs HEAD. Follow-up still uses original base; compare diffs to isolate the new edit. |
| `missing.json`, `initial.json`, `followup.json`                   | Captured pair and immutable pin revision IDs. Follow-up deliberately has no current receipt.                      |
| `missing-project/`, `initial-project/`                            | Pre-execution and initial source/store checkpoints. Initial has no follow-up snapshot.                            |
| `payment/`                                                        | Participant inspection store with initial observations and stale follow-up, **not** the answer-key run.           |
| `facilitator-project/`, `facilitator-key.json`, `key-*`, `step-*` | Facilitator answers and diagnostics; never share with participants.                                               |

The generated root is a facilitator master, not a handout. Make separate private participant directories/accounts. Copy only the selected source, phase diff/ID file, and permitted initial-observation document. For AFTER, copy only `missing-project`, `initial-project`, or `payment` for the relevant phase. Never share the master, keys, proof log, or this facilitator document. Use an isolated participant account; never use the `payment` store during the initial phase because it contains the follow-up snapshot.

| Condition | Allowed material                                                                                                     |
| --------- | -------------------------------------------------------------------------------------------------------------------- |
| Raw       | Ordinary diff/source viewer, phase facts, plain JSON response/call artifacts; no AFTER labels or guided annotations. |
| Tour      | Same material plus the identical scripted tour below.                                                                |
| AFTER     | Native CLI/TUI, same artifacts, complete raw-diff escape; no extra verbal diagnosis.                                 |

Use an absolute native binary path after copying a store. Read IDs from the phase JSON; never choose newest receipt automatically. Missing phase:

```text
after review BASE CANDIDATE --project PARTICIPANT_MISSING_PROJECT
```

Initial/follow-up: use `capture.candidate_snapshot.id`, `capture.base_snapshot.id`, and `pin` from that phase JSON:

```text
after review BASE CANDIDATE PIN --project PARTICIPANT_PHASE_PROJECT
```

The facilitator handles any execution preview after recording the participant's explicit choice. No blanket authorization or arbitrary commands. If a participant requests a rerun, record request and wait time, then reveal the already independently captured follow-up artifacts to every condition under the same policy. Treat this as standardized experiment request, not live execution time. To measure actual rerun/setup friction, obtain new explicit consent in all conditions and log it separately.

### Strong guided-tour script

Deliver verbatim; do not invent conclusions or hide files:

1. “Here is the changed-file map: `app/config.go` is runtime retention; `app/config_test.go` is the test oracle. Read their changes together.”
2. “Trace retention into the duplicate-request handling in `app/main.go`. The driver issues a repeated payment. Consider response and provider effects separately, using the stated twelve-hour requirement and thirty-second control.”
3. “The evidence index identifies the captured pair. When available, open the response array and the provider-call array for both sides and both intervals. Check provenance and missing channels before drawing a conclusion.”
4. “For the follow-up, compare initial and follow-up diffs. The old observation refers to the old candidate. Decide what can be inferred from source, what is actually observed now, and whether another experiment would change your decision.”

This is a curated code/evidence tour, not a straw-man file list. It gets no facilitator-only answers. Freeze the wording; log deviations and help.

## Assignment and sessions

`study:assign` emits anonymous P01… slots. Freeze the operator-chosen seed. SHA-256 ordering shuffles the six condition permutations within six-person blocks and initial case order; case order rotates by block. At 12 people, each condition occurs in each period four times and each case/condition pair appears four times, but case/period balance is incomplete. At 18, case/period exposure balances fully. At 16, the last partial block is incomplete. Report actual cells; never reroll assignments based on skill or desired results.

Target **12–16 practicing engineers**, not a completed sample. Reserve 75 minutes: consent/background 5m, neutral CLI/TUI/diff training 10m, three 15m trials, breaks 5m, debrief 10m. Train on an unrelated diff, not A/B/C. Each trial: missing-evidence probe ≤2m, initial explanation ≤5m, follow-up explanation/experiment choice ≤5m, confidence/debrief 3m. Mark timeouts/censoring; never invent completion time. Offer breaks/withdrawal. During troubleshooting stop the active timer and record downtime and wall time separately.

Ask exactly:

> What changed, what concrete user-visible or external consequence follows, and what supports that claim? Is this result observed for the selected candidate? What is unknown? Would you accept this finite requirement, request another example/run, or withhold judgment? Explain why.

Use [trial sheet](evaluation/trials.csv) and [session sheet](evaluation/sessions.csv). Record monotonic elapsed seconds, not identifying timestamps. No audio/video, screen recording, credentials, employer names, emails, private source, or free-form personal background. Get informed consent separately; keep contact/scheduling data outside this repository. Record only synthetic-task reasoning. Tell participants they can stop and request deletion; agree on a retention deadline first (recommended: raw notes deleted within 30 days of report).

## Scoring and interpretation

Two raters independently score anonymized explanations against the keys, hiding condition where possible; adjudicate disagreements and retain both scores. Freeze the rubric before collection.

| Measure                   | Rule                                                                                                                                                                                                                                                                                             |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Correct explanation (0–4) | One point each: retention/oracle distinction; finite consequence (initial/B/C 1→2 or A repair 1→1, with identical responses); phase applicability (missing/current/stale); finite scope/unknowns. A qualified missing-phase prediction earns the consequence point but is not observed evidence. |
| Missed defect (0/1)       | Initial or B/C follow-up accepts the twelve-hour behavior as meeting one-call expectation or misses duplicate calls. Record false-positive follow-up claims separately.                                                                                                                          |
| False freshness (0/1)     | Calls missing or old-candidate evidence a current observation. Source reasoning that a comment cannot affect execution is not a trust error if the observation is still called stale.                                                                                                            |
| Useful example            | Whether participant finds an available response/effect witness that answers the question; record yes/no/unavailable, find time, and whether manual guidance was needed. Missing-phase unavailability is not navigation failure. Report availability and conditional find rate.                   |
| Cost                      | Initial/re-review active and wall seconds, setup/help seconds, experiment requests, and B comment-only avoidable reruns with reason. Missing times are NA, never zero; retain censored trials.                                                                                                   |

Primary descriptive endpoint: paired participant re-review-time ratio for AFTER vs each baseline, described by case and period. Report anonymous cell counts, medians, ratios, uncertainty intervals, and individual trajectories. Exclude downtime from active time but report it separately. Identical cases are matched across people, not repeated within a person; account for case/order, learning, familiarity, and tour assistance before causal interpretation.

**Proposed target, not achieved:** ~30% lower re-review time (ratio ≤0.70), no drop in defect detection, and zero false-freshness errors on constructed missing/stale phases. Any false-freshness error triggers safety review; do not average it away with faster times. Report misses, usefulness, setup/rerun cost, and speed together. Small samples, related synthetic cases, facilitator effects, censoring, and carryover do not establish general safety, broad usability, or market demand.

## AFTER-18 authorization

Do not start M3 automatically. The operator must authorize recruitment/scheduling and identify a facilitator, second scorer, consent/contact-data owner, and deletion date; recruit 12–16 consenting engineers unfamiliar with the fixture; reserve prepared macOS/Linux sessions; and approve the frozen assignment, scoring, and rerun policy. Run a facilitator dry run labeled **author rehearsal**, never as participant data. Only consenting humans supply outcomes. AFTER-18 must report failures, missed defects, false freshness, unnecessary reruns, and a justified continue/narrow/integrate/stop decision. This kit supplies no study result.

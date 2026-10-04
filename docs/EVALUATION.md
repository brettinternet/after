# Review-loop study kit v1

**Status: runnable technical kit, not a completed study.** Generated runs are
**author rehearsals**. No participants, timing observations, usability gains or
market-value results are supplied. M1/M2 technical completion does not establish
usability or market value. AFTER-18 requires explicit human authorization.

## Prepare and freeze

Use a prepared checkout and the [demo prerequisites](DEMO.md), including the
separately provisioned pinned Docker image. These commands only use synthetic
payment files; never substitute a participant's repository or credentials.

```sh
mise exec -- task study:check
mise exec -- task study:assign -- --assign-seed cohort-01 --participants 12
# Set AFTER_DOCKER_BINARY and AFTER_DOCKER_HOST as in docs/DEMO.md first.
# Explicit authorization for six bounded synthetic runs; retains private kit:
mise exec -- task study:proof -- --keep
```

`study:proof` is the automated consistency/smoke check and author rehearsal. It
fails on incomplete runs, wrong responses/counts, missing controls, cleanup
failures or failure to reopen stale/missing. It executes the shipped native CLI,
not a simulated evidence renderer. Omit `--keep` for an automatically cleaned
check. Without Docker, `task study:check` exercises assignments and native
capture of all three initial cases, including their changed oracle inventory.
That check alone does **not** validate observed keys.

For per-plan interactive consent instead, run `task demo -- --study --keep`.
No silent image pull or host execution fallback is permitted. Interrupted runs
retain diagnostics under the demo's ownership rules; see [recovery](DEMO.md).
Do not distribute an incomplete generation lacking all three facilitator keys.

Freeze the Git commit, Go version, executable checksum, assignment seed/output,
kit version and successful proof log in private study storage before recruitment.
Generated receipt IDs bind the actual snapshots, driver, observer, rules and
runtime. Preserve those IDs, not just a screenshot or this prose. All generated
paths and `.after` stores stay private and out of Git. Changing cases or scoring
requires a new kit version and a new rehearsal before collecting human data.

## Cases and independent keys (facilitator only)

The synthetic service is unfamiliar to eligible participants: exclude people who
implemented AFTER or have seen this fixture/answer key. Explain the small payment
API during training, not the retention defect. These are matched variants of one
service, not three independent real-world domains. Transfer learning is likely;
record prior familiarity and analyze period/order effects.

| Case | Initial edit from 24h retention                 | Controlled follow-up                        | Key at 12h, base → candidate   |
| ---- | ----------------------------------------------- | ------------------------------------------- | ------------------------------ |
| A    | Retention 5m; test expectation changed to match | Restore 24h and matching test               | Initial 1 → 2; follow-up 1 → 1 |
| B    | Retention 1h; test expectation changed to match | Comment only                                | Initial and follow-up 1 → 2    |
| C    | Retention 6h; test expectation changed to match | Weaken test to positive-retention assertion | Initial and follow-up 1 → 2    |

For **every** run: two identical HTTP responses on both versions, and a 30s
control with provider requests 1 → 1. Counts are actual requests recorded by the
runner's protected observer, not app stdout or changed expected-output files.
`facilitator-key.json` is written only after those assertions pass; it links the
initial receipt and independently executed follow-up receipt.
`initial-observations.json` and `key-observations.json` contain the actual response
and call documents used by the assertions. The participant's changed tests are
not the observer. They are captured source, **not executed test reports**; do not
describe them as passing or use them to claim preserved behavior.

The expectation is one provider request for a repeated payment at twelve hours.
Initial A/B/C all violate that finite expectation even though responses match.
A repairs this witnessed defect; B/C do not repair it. B adds no executable change:
whole-footprint invalidation is conservative, not evidence of a new regression.
C changes only the oracle and supplies no stronger behavioral evidence. A changed
oracle cannot redefine the frozen expectation.

Each case has three presentation phases:

1. **Missing:** captured code/diff, no observation. Prediction is allowed if labeled
   as such; an observed/fresh claim is wrong. `missing-project` is a separate
   pre-execution checkpoint with no receipt.
2. **Initial:** provide the initial response/call artifacts in all conditions.
   A setup pin expresses the finite requirement, not participant acceptance.
3. **Follow-up:** show the controlled edit, old observation and reopened pin.
   Applicability is stale, with no current result. Do not reveal the separately
   verified follow-up until the participant submits an explanation and chooses
   whether another experiment is useful. Old results are not fresh even in B.

A rerun of B solely to recover the same finite example after a comment edit is
scored as avoidable for this study's question; it may still be a reasonable
conservative operational policy. Record the participant's reason and distinguish
that cost from a false-freshness error. Rerunning A to check the repair is useful.
C does not justify treating the changed test as preserved behavior; verifying an
unchanged executable can be conservative, not proof of a new effect.

Limits: sequential, synthetic retries at 30s and 12h only; no real charges,
concurrency, other intervals, production provider, or universal correctness.
Source-based inference and independent observations must be scored separately.

## Equivalent conditions and packet isolation

One participant sees each case once, in one condition. Never repeat a case in a
second condition for the same person. All conditions receive the same scenario,
finite expectation, source, diffs, evidence documents and time budget. Only the
navigation/presentation differs. A raw condition without the available artifacts
would confound information availability with interface quality.

Generated case directories `A/`, `B/`, `C/` contain:

- `initial.diff`, `followup.diff`: complete tracked app/oracle diffs versus HEAD.
  Explain that follow-up diffs still use the original base; compare the two to
  isolate the new edit.
- `missing.json`, `initial.json`, `followup.json`: exact captured pair and immutable
  pin revision IDs. `followup.json` deliberately has no current receipt.
- `missing-project/`: pre-execution source/store checkpoint.
- `initial-project/`: initial source/store checkpoint, with no follow-up snapshot.
- `payment/`: participant inspection store, containing initial observations and
  the stale follow-up, **not** the follow-up answer-key run.
- `facilitator-project/`, `facilitator-key.json`, `key-*`, `step-*`: facilitator
  diagnostics/answers. Never place these in a participant's accessible directory.

The generated root is a **facilitator master**, not a participant handout. Prepare
separate private participant directories/accounts: copy only the selected source,
phase diff, phase ID file and permitted initial observation document. For AFTER,
copy only the appropriate `missing-project`, `initial-project`, or `payment` store
for the missing, initial, or follow-up phase, respectively. Do not share the master root, key files, proof log or this
facilitator document. These synthetic copies contain no secrets; nevertheless
use an isolated participant account so sibling paths cannot reveal answers.
Never use the `payment` store in the initial phase: it contains the follow-up
snapshot. Separate checkpoints permit independent browsing without future-state
or answer-key leakage.

| Condition | Allowed presentation                                                                                                    |
| --------- | ----------------------------------------------------------------------------------------------------------------------- |
| Raw       | Ordinary diff/source viewer, phase facts and plain JSON response/call artifacts. No AFTER labels or guided annotations. |
| Tour      | The same materials plus the scripted guided tour below, delivered identically by the facilitator.                       |
| AFTER     | Native CLI/TUI, the same artifacts and complete raw-diff escape; no additional verbal diagnosis.                        |

Use an absolute native binary path after copying a store. Read IDs from the phase
JSON; do not choose the newest receipt automatically. For missing:

```text
after review --tui CANDIDATE --base BASE --project PARTICIPANT_MISSING_PROJECT
```

For initial or follow-up, take `capture.candidate_snapshot.id`,
`capture.base_snapshot.id` and `pin` from the respective phase JSON:

```text
after review CANDIDATE --base BASE --evidence PIN --tui --project PARTICIPANT_PHASE_PROJECT
```

The facilitator handles any execution preview, with the participant's explicit
choice recorded first. Never give blanket authorization or permit arbitrary
commands. When a participant requests a rerun, record the request and wait time,
then reveal the already independently captured matching follow-up artifacts to
**all conditions under the same policy**. Treat this as a standardized experiment
request, not live execution time. If measuring actual rerun/setup friction as an
additional endpoint, run the exact selected pair with new explicit consent in all
conditions and log it separately; do not mix the two timings.

### Strong guided tour script

Deliver without inventing conclusions or withholding inconvenient files:

1. “Here is the changed-file map: `app/config.go` is runtime retention;
   `app/config_test.go` is the test oracle. Read their changes together.”
2. “Trace retention into the duplicate-request handling in `app/main.go`.
   The driver issues a repeated payment. Consider response and provider effects
   separately, using the stated twelve-hour requirement and thirty-second control.”
3. “The evidence index identifies the captured pair. When available, open the
   response array and the provider-call array for both sides and both intervals.
   Check provenance and missing channels before drawing a conclusion.”
4. “For the follow-up, compare initial and follow-up diffs. The old observation
   refers to the old candidate. Decide what can be inferred from source, what is
   actually observed now, and whether another experiment would change your decision.”

This is a strong, curated code/evidence tour, not a straw-man file list. It gets
no facilitator-only answers. Freeze wording; log deviations and assistance.

## Reproducible assignment and session

`study:assign` emits anonymous P01… slots. Freeze an operator-chosen seed before
assignment. SHA-256 ordering shuffles the six condition permutations within each
six-person block, and shuffles initial case order. Case order rotates across
blocks. At 12 participants each condition occupies each period four times and
each case/condition pair appears four times. At 18, case/period exposure also
balances completely. At 16, the final partial block is intentionally incomplete:
report the actual cells rather than claiming exact balance. At 12, case/period
balance is incomplete; report that limitation too. Assign consented participants
to the next unused slot, without rerolling based on skill or desired results.

Target **12–16 practicing engineers**, not a completed sample. Reserve 75 minutes:
consent/background 5m, uniform neutral CLI/TUI/diff training 10m, three 15m trials,
breaks 5m, debrief 10m. Use an unrelated neutral diff for training, not A/B/C.
Each trial: missing-evidence probe (2m maximum), initial explanation (5m maximum),
follow-up explanation/experiment choice (5m maximum), confidence/debrief (3m).
Mark timeout/censoring rather than inventing a completion time. Offer breaks or
withdrawal without penalty. Do not troubleshoot during the timer: log downtime,
stop the active timer and report wall-clock/setup time separately.

For each phase ask exactly:

> What changed, what concrete user-visible or external consequence follows, and
> what supports that claim? Is this result observed for the selected candidate?
> What is unknown? Would you accept this finite requirement, request another
> example/run, or withhold judgment? Explain why.

Use the [blank recording sheet](evaluation/trials.csv) and
[session sheet](evaluation/sessions.csv). Record monotonic elapsed seconds, not
identifying timestamps. No audio/video, screen recording, credentials, employer
names, emails, private source or free-form personal background. Obtain informed
consent separately; keep contact/scheduling data in an operator-controlled system,
not this repository. Notes must describe synthetic task reasoning only. Tell
participants they may stop and request deletion; agree on a retention deadline
before collection (recommended: delete raw notes within 30 days of the report).

## Predeclared scoring and interpretation

Two raters independently score anonymized explanations against the keys, with
condition hidden where possible; adjudicate disagreements and retain both scores.
Do not silently change the rubric after seeing outcomes.

- **Correct explanation (0–4):** one point each for retention/oracle distinction;
  exact finite consequence (1→2 initial/B/C or 1→1 A repair, identical responses);
  evidence applicability (missing/current/stale for the phase); explicit finite
  scope/unknowns. In the missing phase a properly qualified prediction earns the
  consequence point; it is not an observed result.
- **Missed defect (0/1):** initial or B/C follow-up accepts the twelve-hour behavior
  as satisfying one-call requirement, or fails to identify duplicate requests.
  A follow-up instead records false-positive defect claims separately.
- **False freshness (0/1):** treats missing or old-candidate evidence as a current
  observation. Source reasoning that a comment cannot affect execution is not
  itself a trust error if the observation is still called stale.
- **Useful example:** participant can identify an available response/effect
  witness answering the question; record yes/no/unavailable, time to find, and
  whether manually authored guidance was necessary. Missing-phase unavailable is
  not a navigation failure. Report both availability and conditional find rate.
- **Cost:** initial/re-review active seconds, wall seconds, setup/assistance
  seconds, experiment requests, and B comment-only avoidable reruns with reason.
  Record missing timings as NA, never zero. Keep censored trials in the report.

Primary descriptive endpoint: paired participant re-review-time ratio for AFTER
versus each baseline, adjusted/described by case and period. Report raw anonymous
cell counts, medians, ratios, uncertainty intervals and individual trajectories;
exclude downtime from active time but report it separately. With one trial per
condition per person, case/condition matching is across people, not repeated
identical tasks. Do not interpret a within-person difference as causal without
accounting for case/order, learning, familiarity and guided-tour assistance.

**Proposed target:** roughly **30% lower re-review time** (ratio ≤0.70), with no
reduction in defect detection and **zero false-freshness errors** on constructed
missing/stale phases. These are go/no-go targets, not achieved effects or powered
statistical guarantees. Any false-freshness error triggers a safety/design review;
do not average it away with faster times. Report missed-defect counts alongside
speed, and describe usefulness/setup/rerun costs even if the speed target passes.
Small samples, related synthetic cases, facilitator effects, censoring and
carryover preclude general safety, broad usability or market-demand claims.

## Exact handoff for AFTER-18

Do not auto-start M3. The operator must authorize scheduling/recruitment and name
a facilitator, a second scorer, a consent/contact-data owner and deletion date;
recruit 12–16 consenting engineers unfamiliar with the fixture; reserve 75-minute
sessions on prepared macOS/Linux terminals; approve the frozen assignment and
scoring protocol and the standardized rerun policy. Perform a facilitator dry run
first, explicitly labeled **author rehearsal**, not a participant or a data row.
Only actual consenting humans supply study outcomes. AFTER-18 must report failures,
missed defects, false freshness, unnecessary reruns and a justified continue,
narrow, integrate-or-stop decision. No result is supplied by this kit.

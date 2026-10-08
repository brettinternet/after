# How AFTER derives behavior and observes evidence

AFTER's current Go CLI/TUI supports local Git capture, stock `go test -json` import, stored evidence, pins, and one consented synthetic payment experiment. It is not a general behavior engine. This document gives the broader evidence model; see [the implementation contract](IMPLEMENTATION.md), [CLI reference](CLI.md), and [payment fixture](../internal/paymentfixture/README.md) for the implemented bounds.

## 1. Define one observable scenario

A scenario has setup, a stimulus, and an observation. The expectation is separate.

```text
SETUP                     STIMULUS                         OBSERVATION
empty state, fixed clock + request, arguments, action → output and side effects
```

| Part        | Payment retry example                                           |
| ----------- | --------------------------------------------------------------- |
| Setup       | Empty app state, fixed clock, separate fake payment provider    |
| Stimulus    | Submit key K, advance twelve hours, submit K again              |
| Observation | HTTP responses, provider requests, persisted state if captured  |
| Expectation | One provider request for this sequence                          |
| Limit       | Sequential synthetic case; no production network or concurrency |

“Retry handling works” is too broad. “This sequence returned these responses and caused one provider request” is measurable. A passing example does not establish a universal requirement. A removed API field can still be a structural fact when it cannot be run.

## 2. Where facts come from

These sources can suggest scenarios or establish bounded facts. None is a complete behavioral description.

| Source                                    | Can show                                                           | Cannot establish by itself                                            |
| ----------------------------------------- | ------------------------------------------------------------------ | --------------------------------------------------------------------- |
| Test report                               | Test identity, pass/fail/skip, failure text and reported locations | Inputs, outputs or effects not serialized by the test runner          |
| Fixture or snapshot                       | Concrete inputs and expected data                                  | That the program ran or that an updated oracle is correct             |
| API/type contract                         | Declared fields, endpoints and signatures                          | That the implementation follows the contract                          |
| Browser/integration trace                 | Captured actions, DOM, screenshots, calls or attachments           | Complete assertions or backend behavior outside the captured boundary |
| Static analysis                           | Changed symbols, values, imports and possible call paths           | That a path executed or dynamic dependencies were found               |
| Coverage                                  | Code reached by an instrumented run                                | That a particular test asserted the important effect                  |
| Property, model-based or metamorphic test | Finite generated cases and tested relationships                    | A correct oracle or all possible inputs                               |
| Runtime recording                         | Observed inputs and responses available for replay                 | Safe, complete, deterministic, or copyable production data            |

AFTER imports stock Go `go test -json`; the report supplies test outcomes, not reconstructed inputs or side effects. A report can support “`TestRetry` passed on base and failed on candidate; expected 1, got 2.” It cannot, alone, support “a real provider received two charges for the same key.” See [the report adapter](GO-REPORTS.md).

Test selection can use changed symbols, compiler references, explicit test links, imports, and coverage. Aggregate coverage does not identify which test exercised a statement. Per-test attribution needs additional instrumentation or isolated runs. Dynamic imports, plugins, environment lookups, triggers, and generated code can hide dependencies; incomplete selection must stay visible.

A Vitest JSON report illustrates the distinction: it lists test results, but an `assertionResults` entry does not serialize every passing assertion's operands. Custom reporter APIs also need a tested compatibility range. [Vitest JSON reports](https://vitest.dev/guide/reporters) · [Custom reporters](https://vitest.dev/advanced/reporters).

## 3. Compare observations deterministically

A useful pipeline is:

```text
Git snapshots → changed symbols/values → candidate scenarios
             → matched observations → typed differences → inspectable cards
```

Use explicit, versioned comparison rules:

- JSON objects compare by field path; key order is immaterial, array order remains meaningful unless a rule says otherwise.
- CLI results compare exit status, stdout, and stderr.
- Effect logs compare operation, destination, payload, count, and relevant ordering.
- Migrations compare resulting schemas and records; UI comparisons use only captured DOM, accessibility, screenshot, or network channels.
- Public-interface analyzers report only the compatibility facts they support.

Normalization is part of the evidence. A UUID mask may hide a real change; a comparison cannot claim complete equality if a channel is missing, redacted, truncated, or incompatible. Keep every repetition. Disagreement is **unstable**, not a result to cherry-pick.

A measured card can be generated without a model:

```text
POST /payments · same key after 12h
Provider requests  1 → 2
HTTP response      200 → 200
Scope              this frozen fixture; sequential only
```

Group by a conservative signature, such as boundary, channel, changed field paths, and divergence type. Show measured members and a representative case; a shared signature does not prove shared cause. Say “14 captured cases lost `user.name`,” not “all users fail.”

To find missing examples, sample values around changed thresholds, use existing schema/property generators, model action sequences, supported fuzzers, or metamorphic relations. Each needs valid setup, a harness, and an oracle. Shrinking can make a distinguishing sequence easier to inspect; call it “reduced,” not “minimal,” unless minimality was proved. Store the concrete reduced input, not only a generator seed. [fast-check shrinking and reports](https://fast-check.dev/docs/tutorials/quick-start/read-test-reports/).

## 4. Bind every observation to its run

A paired run compares the same frozen scenario on two deliberately selected snapshots:

1. Capture exact code states, including a consistent dirty working tree when selected.
2. Freeze the fixture, driver, observer, and comparison-rule identities independently of application code.
3. Prepare separate environments and initial state. Record each side's actual runtime and dependencies; dependency drift is part of the result.
4. Control supported sources of variation such as clock, seed, locale, timezone, and service responses. List what remains uncontrolled.
5. Reset state between versions and repetitions unless shared state is the explicit scenario.
6. Record process completion, outputs, selected effects, errors, and whether each channel is complete.
7. Compare only compatible observations. A driver failure is incomplete or incomparable, not a regression or equality.
8. Persist producer, bindings, limits, and artifact references with the result.

Example fields below are illustrative, not the versioned storage schema:

```json
{
    "scenario": "same-payment-key-after-12h",
    "producer": "after-paired-runner",
    "snapshots": { "base": "digest-A", "candidate": "digest-B" },
    "input": "fixture-digest",
    "observer": "http-and-fake-provider-v1",
    "result": {
        "base": { "status": 200, "provider_calls": 1 },
        "candidate": { "status": 200, "provider_calls": 2 }
    },
    "limits": ["fake provider", "sequential fixture", "no production execution"]
}
```

A real receipt also records completion, timestamps, command/driver, authorization, each side's environment and toolchain, artifact references, observation completeness, redaction policy, and limits. A digest binds bytes; it is not a signature or proof that a producer is honest. A CI report remains “reported by this producer” unless its revision and execution bindings are established. A model's claim to have run a command is not a runner record.

Capture effects outside the application when practical: record HTTP requests at the client or fake-provider boundary, CLI streams at the process boundary. This improves independence but does not make the fixture a security boundary against malicious code. A fake service can differ from production, and a candidate can detect its test environment.

Opening, importing, inspecting, diffing, previewing, and config display do not execute project code. Builds and dependency setup count as execution too. A worktree is not a sandbox. The supported run requires exact-plan consent and the configured offline Docker boundary; no credentials, host fallback, or external network. Treat repository content, reports, artifacts, and model output as untrusted data: do not execute embedded content or follow arbitrary artifact paths, and sanitize terminal output. See [sandbox design and residual limits](SANDBOX.md).

## 5. Keep the oracle separate from the implementation

Show two different questions:

| Evidence                                            | Question                                          |
| --------------------------------------------------- | ------------------------------------------------- |
| Same frozen scenario and observer on both snapshots | What changed under the same experiment?           |
| Candidate's own tests, with test edits visible      | What does the new suite assert, and does it pass? |

A test can rely on removed internals, so failure to run an old driver may be incomparable rather than a user-facing regression. A newly added candidate-authored test can establish its own observed transition, not that its expectation is correct. A new feature may have no base behavior; show absence separately instead of inventing a base output.

Never treat fewer assertions, wider tolerances, changed masks, or refreshed golden files as automatic proof that behavior was preserved. Keep test-oracle edits visible beside the frozen comparison.

## 6. The real payment-retention fixture

The shipped fixture in `internal/paymentfixture` runs a standard-library Go HTTP app and an independently recording fake provider. A fixed clock and fresh app state let the same frozen driver test both revisions. Expiration uses `now >= created + retention`; a cached retry does not refresh creation time. The fake provider always returns the same successful response and records every request; it does not implement deduplication for the app.

| Case                      | Base provider calls | 5-minute-retention candidate |
| ------------------------- | ------------------: | ---------------------------: |
| Same key after 12 hours   |                   1 |                            2 |
| Same key after 30 seconds |                   1 |                            1 |
| Same key at 5 minutes     |                   1 |                            2 |
| Same key at 24 hours      |                   2 |                            2 |

Both versions return the same HTTP status and body. The observer records exact requests and controlled times, not the app's printed count. Mutation checks verify that disabling deduplication changes the independent log, while changing the app-printed count does not. The fixture also checks before/at-threshold, different-key, expired-key, and non-sliding-expiry cases; see its [full case table and limits](../internal/paymentfixture/README.md).

The runner executes only after consent to the exact plan. It uses isolated, offline containers and bounded time and output; it does not move money. This is evidence for a finite synthetic fixture, not concurrent billing, durable transactions, real-provider behavior, all keys, or a proof over all inputs. The fixture is not a general application adapter.

## 7. Make review memory conservative

A receipt's applicability can depend on more than the source line:

```text
code + fixture + driver + observer + rules + runtime + dependencies
                     + configuration and relevant state
                                      ↓
                           receipt applicability
```

The first implementation uses a broad project footprint rather than a complete semantic dependency graph. This causes extra reruns but avoids false freshness. Static dependencies and traces can improve selection later; a trace only describes the path it observed. External services and state need explicit versioning or a bounded validity policy.

Keep the human expectation stable when its evidence becomes stale. Explain what binding changed and what is missing; never invent a new observation. Track these independently:

- Evidence kind and producer.
- Applicability to the selected snapshot.
- Execution and comparison outcome, including incomplete, incomparable, or unstable.
- Human decision and its scope.

“No recorded dependency changed” is not universal equivalence. If applicability cannot be established, show unknown. See [persistent pins and reopening](REVIEW.md).

## 8. Use models only for interpretation

A model may propose a distinguishing scenario, summarize a measured field change, suggest a relationship among edits, or group changes that deterministic structure cannot. Suggestions remain untrusted until a person approves and a runner records them.

A model must not create an observed or fresh badge, invent output for an unexecuted input, hide unclassified changes, declare equivalence from missing mismatches, change rules silently, authorize a run, or accept a review. With models disabled, capture, ordinary diff, report import, structured comparison, pins, and evidence history must remain useful. Exact templates are a valid baseline.

## 9. Stay useful while analysis is incomplete

Open the raw diff and inventory first. Import compatible artifacts without requiring a full test run; label missing data. Add deterministic facts next, then reuse compatible results and propose selected experiments. Run only authorized, relevant scenarios.

Coalesce rapid edits into explicit snapshots. Cancel obsolete queued work but keep completed evidence bound to its actual snapshot. Never attach a late result to the newest code by accident. Group repeated signatures without hiding members or unsupported paths. Show unknown and budget limits instead of silently dropping changes.

Performance targets are evaluation goals, not guarantees: warm raw view within one second and cached list within two seconds on a medium-sized repository. Benchmark with the hardware and cache state recorded. Keep the UI responsive while background work runs, virtualize large lists, cap output, and disclose partial analysis.

## 10. First slice and acceptance

The first slice is a local CLI and thin TUI: capture Git states; import one report format; run the real HTTP fixture by explicit consent; compare responses and provider calls; expose exact witnesses and unclassified changes; pin an expectation; reopen it on a basis change; and rerun only after authorization. This loop is implemented for the bounded payment experiment, not arbitrary projects. [The implementation contract](IMPLEMENTATION.md) has the release gates: changed side effects, changed test oracle, basis changes, late results, failures, instability, missing adapters, disabled models, hostile repository content, and finite evidence scope.

Compare the TUI with a raw diff and a strong guided tour on unfamiliar changes and follow-up edits. Measure correct explanations, defect detection, re-review effort, mistaken trust in stale evidence, and unnecessary reruns. Technical completion is not proof of usability; the human study is a separate, authorized activity.

## 11. Preferred implementation language

Go is the preferred language for the current engine, CLI, and TUI. The implementation is in `cmd/after` and `internal/`; it does not need a JavaScript runtime to display review state. Supported project drivers still need their own runtimes and dependencies. The current TUI uses Bubble Tea v1.3.10.

Use Go for snapshot identity, evidence records, deterministic comparison, process orchestration, review state, and terminal presentation. Keep language-specific analysis and test capture in existing tools rather than rewriting compilers or runners in Go. Use the installed Git CLI with explicit arguments and NUL-safe paths; handle unusual filenames, renames, binary changes, index/working-tree/base semantics, and incomplete captures rather than approximating Git.

Run Git, parsing, and fixture work outside the UI event loop. Use explicit argument vectors, not shell-built commands. Bound and drain output, apply timeouts, tag results with snapshot and request IDs, and support cancellation and owned-process cleanup. Canceling a parent alone may leave descendants. None of this creates a sandbox. Keep repository text and captured output safe from terminal control sequences; test Unicode, tabs, long lines, narrow layouts, and resize behavior.

Bubble Tea was selected by the terminal foundation evaluation, not left as an open candidate. Keep ordinary Go packages and current interfaces; do not add a plugin framework, database, CGO parser suite, or model dependency without a demonstrated requirement. See [terminal evaluation](TERMINAL.md) and [the implementation contract](IMPLEMENTATION.md).

## Source notes

References establish available tooling, not an AFTER integration: [Vitest JSON reports](https://vitest.dev/guide/reporters), [Vitest custom reporters](https://vitest.dev/advanced/reporters), [fast-check reports and shrinking](https://fast-check.dev/docs/tutorials/quick-start/read-test-reports/), and [Playwright Trace Viewer](https://playwright.dev/docs/trace-viewer) / [tracing API](https://playwright.dev/docs/api/class-tracing). Documentation inspected October 2, 2026; version compatibility must be checked when an adapter is added.

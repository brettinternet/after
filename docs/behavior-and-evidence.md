# How AFTER derives behavior and observes evidence

## Build an observation engine first

**Tests, fixtures, contracts, and execution traces should supply the facts. LLMs should be optional interpreters and experiment designers.**

AFTER is an independent local tool, not a Hunk extension. It owns its capture format, comparison engine, review state, and interface. It consumes Git snapshots and external test artifacts without depending on an editor, coding agent, forge, or model provider.

The practical starting point is not “infer everything this repository does.” It is:

> For the scenarios we can inspect, what differs between these two versions, and which differences deserve attention?

That can be useful without an LLM. It also sets a clear limit. No test suite, static graph, or model can supply a complete behavioral description of an arbitrary repository.

This document describes a proposed engine. The accompanying presentation remains a simulated interaction, not a working implementation.

## 1. Define behavior precisely enough to measure

An observable scenario has three parts.

```text
SETUP                    STIMULUS                       OBSERVATION
state, dependencies,  +  request, arguments, action  →  outputs and side effects
clock, environment       or action sequence
```

An expectation is a separate object. It describes what a human or test author wants to remain true. The observation records what happened.

For example:

| Part | Payment-retry scenario |
| --- | --- |
| Setup | Empty test database, fixed clock, isolated fake payment service |
| Stimulus | Submit a payment, advance the clock twelve hours, submit the same key |
| Observation | HTTP responses, recorded payment-service requests, persisted ledger rows |
| Expectation | Exactly one payment creation for this selected sequence |
| Limits | Sequential execution; fake provider; no production network or concurrency |

“Retry handling works” is not measurable enough. “This sequence made one provider request and one ledger entry” is.

The same representation works for a CLI's exit code and output, an API's JSON and database writes, a UI's state after an interaction, or a migration's transformed records. Each needs a bounded observer suited to its interface.

AFTER should distinguish an **example**, a **general requirement**, and a **structural fact**. A single passing example cannot establish a general requirement. A removed API field is a structural fact even when nothing can be executed.

## 2. Where scenarios and behavior facts come from

The order below reflects increasing setup and execution cost, not a universal ranking of reliability.

| Source | What can be obtained without an LLM | What cannot be assumed |
| --- | --- | --- |
| Test reports | Test identity, pass/fail/skip, failures, duration, source locations when reported | Passing tests generally do not expose all inputs, returned values, or side effects |
| Fixtures and snapshots | Existing requests, representative records, expected outputs, paired screenshots | An expected file is not an observed run; an updated snapshot may approve a regression |
| API and type contracts | Added/removed endpoints, required fields, response shapes, exported signatures | A declared contract is not evidence the implementation follows it |
| Captured browser tests | Actions, DOM states, screenshots, network traffic, selected attachments | A click trace does not automatically reveal backend behavior or all assertions |
| Integration-test traces | Recorded calls, effects, timings, input/output payloads when captured | Instrumentation coverage can be partial; traces can contain secrets |
| Static analysis | Changed symbols, exact values, imports, resolved calls, condition boundaries | Static reachability is not execution; dynamic dependencies may be missing |
| Property-based tests | Generated cases, reproducible seeds, smaller counterexamples | A finite sample does not prove the property |
| Runtime traffic recordings | Realistic inputs and recorded responses for replay | Production data is safe to copy, complete, deterministic, or available by default |
| Model-based and metamorphic tests | Sequences and relationships such as encode/decode round trips | The model or relation is correct just because it was written as a test |

### What can be derived from tests today

Use existing structured outputs first. Vitest's JSON report, for example, includes test results and failure messages, and includes a coverage map when coverage is enabled. Its example `assertionResults` entries describe test cases; that field name does not mean every passing assertion's operands are serialized. [1]

A stock report can support a card saying:

> `retries a payment` passed on the base and failed on the candidate. The failure reports expected 1, received 2.

It generally cannot support this stronger card on its own:

> The same payment key, retried twelve hours later, created two real provider charges.

The latter requires the input sequence, observation method, and side-effect evidence. Do not infer those from the test name or parse arbitrary source code as if the test were declarative data.

There are three practical levels of integration.

1. **Import reports.** Useful immediately for changes in test status and existing artifacts. Explicitly label input or output information as unavailable.
2. **Import rich fixtures and attachments.** A project may already emit request/response snapshots, screenshots, trace archives, or state dumps. Preserve their producer, revision binding, and limits.
3. **Add an opt-in capture adapter.** Record selected observations at a boundary such as HTTP, a CLI process, or a test helper. A reporter can collect artifacts, but a reporter alone does not magically observe application internals.

Vitest supports custom reporters, but its documentation warns that exposed reporting APIs can change within a minor version. A real adapter needs a tested compatibility range. [2]

### Tests help choose where to look

Changed source plus test relationships produces a candidate set. Use compiler references, imports, reported coverage, and explicit test links where available.

Aggregate coverage does not establish which individual test exercised a statement. Per-test attribution requires additional instrumentation or isolated runs with an accurately scoped coverage collector. Even measured coverage says “this code executed,” not “an assertion checked its important effect.”

When selection is uncertain, expand the selected set or label it incomplete. Unsupported dynamic imports, plugins, environment lookups, database triggers, and code generation must not become invisible dependencies.

## 3. Derive useful contrasts without an LLM

A useful initial engine can be entirely deterministic.

```text
Git difference
   ↓
Changed interfaces, values, and symbols
   ↓
Existing scenarios likely to touch those changes
   ↓
Matched observations on base and candidate
   ↓
Typed differences and exact witnesses
   ↓
Contact sheet with inputs, outputs, source links, and limits
```

### Direct comparison

Use domain-appropriate comparators, not a model judging whether two strings “mean the same thing.”

- JSON objects produce added, removed, or changed field paths.
- CLI executions produce exit status, stdout, and stderr differences.
- Side-effect logs produce differences in operation, destination, payload, count, and relevant ordering.
- Migration fixtures produce record-level and schema differences.
- Browser observations produce screenshot, DOM, accessibility, and network differences when those channels were captured.
- Public interfaces produce structural compatibility facts, with the analyzer's supported scope stated.

Exact identity is appropriate for some channels. Others need explicit rules. JSON key order can be ignored; array order cannot be ignored unless the contract permits it. A UUID mask can reduce noise but hide a relevant change. Normalization rules are part of the evidence and must be versioned and reviewable.

Templates can already produce readable cards.

> `POST /payments` · repeat key after 12h
>
> Recorded provider requests **1 → 2**
>
> Response status **200 → 200**
>
> Observed in the same selected fixture on both versions.

No natural-language understanding is required to produce that card. More importantly, a response-only comparison would have missed the changed side effect.

### Group by observable difference

Group cases by a conservative signature such as the boundary, observation channel, changed output paths, and kind of divergence. Show one representative example with the number of additional measured cases. Keep the members expandable.

Do not imply that a cluster establishes one root cause. Ten requests can produce similar errors for different reasons. Different resource IDs, permission contexts, or affected consumers may require separate groups even when the JSON diff has the same shape.

A good display says “14 captured cases lost `user.name`.” It does not silently promote that observation to “all users will fail.”

### Find informative new examples

Existing tests will miss important differences. Non-LLM methods can expand the search:

- Boundary values around changed comparisons, limits, lengths, and timeouts.
- Schema-based input generation using existing generators and constraints.
- Property-based generation with stable seeds.
- Model-based action sequences for protocols and state machines.
- Coverage-guided fuzzing for supported executable targets.
- Metamorphic relations, such as a serialize/parse round trip or a transformation that should preserve a result.

These techniques require valid inputs, a harness, and meaningful observers. An OpenAPI schema alone does not supply valid accounts, authentication, stateful setup, or a correct business oracle.

Counterexample reduction is particularly valuable for the interface. If a long sequence distinguishes the two versions, try to retain the divergence with fewer steps and simpler data. Fast-check already demonstrates this pattern through shrinking and replay metadata. Store the reduced concrete input too, because a seed is not stable across arbitrary generator or tool changes. [3]

Use “reduced example,” not “the minimal example,” unless minimality is actually established.

## 4. What observing evidence actually means

An AFTER-owned runner, or an explicitly identified external producer, executes a frozen scenario and records specified observation channels.

A sound paired run needs the following sequence.

1. **Capture exact code states.** Select the base and candidate deliberately. A dirty working tree needs a consistent content capture, not just a branch name.
2. **Freeze the scenario and observer.** Record fixtures, driver code, expectation version, comparison rules, and observer version independently of the application versions.
3. **Prepare isolated environments.** Use the same controlled inputs and initial state. Record each version's actual dependencies and runtime. A dependency upgrade is part of the comparison, not something to normalize away.
4. **Control the sources of nondeterminism that the fixture supports.** Clock, random seed, locale, timezone, scheduler assumptions, and service responses matter. State plainly what remains uncontrolled.
5. **Reset state between versions and repetitions.** Never run the candidate against data left by the base unless that sequence is the explicit experiment.
6. **Execute and capture.** Record process completion, test status, outputs, selected effects, and errors. A partial capture is a partial result.
7. **Compare only compatible observations.** If one application cannot load the driver or reach the boundary, report “incomparable” or “could not run,” not a behavioral regression.
8. **Persist provenance and limitations with the result.** Derive the visible badge from the record, never from generated prose.

### An example receipt

This is illustrative data, not a proposed finalized schema or an observed run.

```json
{
  "scenario": "payment-key-retry-12h",
  "producer": "after-paired-runner",
  "code": { "base": "content-digest-A", "candidate": "content-digest-B" },
  "driver": "driver-content-digest",
  "input": "fixture-content-digest",
  "observer": "http-and-fake-provider-v1",
  "comparisonRules": "rules-content-digest",
  "environment": { "base": "environment-A", "candidate": "environment-B" },
  "result": {
    "base": { "responseStatus": 200, "recordedProviderCalls": 1 },
    "candidate": { "responseStatus": 200, "recordedProviderCalls": 2 }
  },
  "limits": ["fake provider", "sequential fixture", "no production execution"]
}
```

A real record also needs completion status, timestamps, command or driver identity, artifact references, observation completeness, redaction policy, and the actual raw or structured data needed to inspect the comparison. Digests identify content; they do not prove that a producer is honest or a fixture adequate.

Store an imported CI report as “reported by this CI producer” unless its execution and revision bindings can be established. A model saying it ran a command is not a substitute for a runner record. Likewise, a success exit code can establish process completion, but not an unobserved business effect.

### Measure outside the code being evaluated where practical

For HTTP, capture at the test client's boundary. For a CLI, capture the process's actual streams and status. For a fake payment service, record the requests it receives rather than trusting the application to print a counter.

This improves observability and reduces self-reporting, but it is not a general defense against adversarial candidate code. A malicious program can detect its test environment. A fake provider may differ from the real provider. Claims remain scoped to the experiment.

Playwright shows that rich artifacts can come from existing tooling. Its trace viewer exposes action snapshots, screenshots, network requests, and attachments. Those are useful inputs, but two traces still need matched scenarios and compatible state before AFTER can compare them. [4][5]

### Execution must be deliberately authorized

Checking out a PR or importing an artifact must not execute it. Tests, install scripts, build tools, and trace attachments are untrusted input.

A worktree is not a sandbox. Execution needs isolated compute, no ambient credentials, no production services, bounded resources, and a deliberate network/dependency policy. HTTP fixtures should use disposable local services or explicitly configured safe targets. An automatically discovered URL must not authorize a request to an arbitrary external or internal service.

Retain safe, bounded artifacts. Redact secrets before storage or sharing; document when redaction prevents a faithful comparison. Do not silently drop sensitive fields and then claim equality of the complete outputs. Views of HTML, logs, archives, and screenshots must not execute embedded content or follow arbitrary artifact paths.

## 5. Keep the test oracle separate from the implementation

A common AI failure mode is changing both the implementation and its test until everything passes.

AFTER should make two different comparisons available.

| Comparison | Question |
| --- | --- |
| Frozen scenario and observer on both versions | What changed under the same experiment? |
| Candidate's own tests, with test edits visible | What does the proposed suite now assert, and does it pass? |

Neither replaces the other. Existing tests can depend on internal APIs that a valid refactor removes. If the old driver cannot run on the new implementation, show the incompatibility and inspect the public boundary instead. A failed test import is not a measured user-facing regression.

For a newly added test, the same candidate-authored fixture can be run on both versions if it is compatible. If it fails before and passes after, that establishes the fixture's transition, not the correctness of its expectation. Keep its authorship visible.

For a brand-new feature, the base may have no comparable behavior. Show an absent endpoint or capability separately from an observed candidate result. Inventing a fictional base response would be misleading.

The tool must never treat fewer assertions, broader tolerances, changed masks, or refreshed golden files as automatic evidence of preserved behavior.

## 6. The payment example, made implementable

Build a tiny fixture that makes the presentation's example real without moving money.

```text
For each version, with clean state:

  Set fake clock to T0
  Submit key K through the application boundary
  Record fake-provider requests and persisted payment state
  Advance clock by 12 hours
  Submit key K again
  Record responses, fake-provider requests, and persisted state
```

The fake clock must drive both key creation and expiration. Advancing an application's clock while the actual key store uses a different clock would invalidate the setup. A real TTL store may require a controlled adapter or a different test design.

Compare the observations. Do not encode “the candidate must charge twice” into the fixture or return the expected answer from a mock. The fake provider should record application calls and respond consistently; it must not implement the application's deduplication logic on its behalf.

Then sample around both expiration thresholds using a harness-appropriate epsilon:

```text
just before 5 minutes   ·   at 5 minutes   ·   just after 5 minutes
just before 24 hours    ·   at 24 hours    ·   just after 24 hours
```

Use a fresh initial state for each case. A changed comparison operator may make the exact threshold important. Three agreeing observations do not establish every point in the interval.

If an LLM proposes that retention caused the divergence, label that an explanation. A controlled experiment that varies only the retention value can strengthen the causal case. If the whole candidate contains many edits, a paired run alone has not isolated the cause. Counterfactual patches require separate snapshots and authorization and are not a first-release requirement.

The first compelling technical demo should show a real captured request count changing from one to two, plus an untouched control case. It should not rely on a model describing the intended output.

## 7. Make review memory conservative enough to trust

A receipt binds to more than the source line a reviewer clicked.

```text
Code + fixtures + driver + observer + comparison rules
     + runtime + dependencies + relevant configuration and state
                              ↓
                    applicability of the receipt
```

For the first implementation, invalidate broadly at the relevant test project or package boundary. This creates extra reruns, but avoids pretending to have a complete semantic dependency graph.

Later, improve selection using conservative static dependencies and measured runtime reads. Dynamic traces describe the path observed on that run; they do not capture every dependency a different path might read. External services and state need explicit versioning or a bounded validity policy.

Keep the human expectation stable while evidence becomes stale. A pin is not lost when it reopens. The interface should say what changed and what is required to inspect it again.

Use separate states for:

- Evidence kind and producer.
- Applicability to the selected snapshot.
- Run/comparison outcome, including incomparable or unstable.
- Human decision and its scope.

“No recorded dependency changed” is not a universal guarantee. If applicability cannot be established, show unknown. Do not infer freshness from a matching test title or source filename.

## 8. Use LLMs where they add something specific

An LLM is useful for:

- Suggesting which missing scenarios would discriminate the two versions.
- Turning a measured field-level change into a concise explanation.
- Proposing relationships between scattered source changes and a selected outcome.
- Grouping a change for human reading where deterministic structure is inadequate.
- Suggesting fixtures or observers, which remain untrusted code until approved and executed.

It must not:

- Set an observed or fresh badge.
- Invent before/after results for an unexecuted input.
- Discard unclassified changes because they do not fit the story.
- Turn absence of a mismatch into proof of equivalence.
- Define a producer's trustworthiness by its own confidence.
- Authorize execution, change comparison rules silently, or accept a review.

With models disabled, the ordinary diff, source inventory, report importer, structured comparisons, pins, and evidence history should still work. Exact templates are an acceptable baseline. If the product becomes useless without a paragraph generator, the underlying observations are probably too weak.

## 9. Make it effective at agent speed

The bottleneck will be getting useful observations cheaply, not rendering cards.

### Start with what is already available

Open the diff immediately. Import compatible artifacts and show their freshness. Do not make a user run the entire repository before they can inspect a change. With no artifacts, show structural facts and suggest experiments without claiming results.

### Spend execution on questions, not all possible tests

Prioritize pinned expectations whose inputs changed, public contract changes, unexplained effects, and likely distinguishing cases. Test selection should widen when dependency knowledge is uncertain.

Reuse baseline observations only when the code, driver, inputs, observer, environment, and comparison policy are compatible. A previously reviewed candidate can be the base for a follow-up comparison, but keep the original PR-base comparison separately accessible.

Coalesce rapid agent edits into explicit snapshots. Cancel obsolete queued work while retaining completed results against their actual snapshots. Never attach a late result to whatever version happens to be current when it arrives.

### Reduce information without concealing the remainder

Group repeated divergence signatures, show representative examples, and keep all measured members accessible. Keep unclassified source changes visible. Do not produce 700 cards because 700 tests ran.

Use risk as an explainable ordering signal, not a correctness score. A public field removal, a stale payment expectation, or a changed side-effect count can rank ahead of formatting. Human reviewers can override the ordering.

### Handle unstable results explicitly

Repeat candidate mismatches and selected controls when the experiment may be nondeterministic. Record distributions and flakiness rather than picking the most convincing run. Do not treat a single timing sample as a performance regression. Capture startup state, warmup, repetitions, and variance before making a bounded performance claim.

### Measure whether the observations can detect a mistake

Controlled mutations can test whether an observer notices a relevant fault. For example, temporarily disable deduplication in an isolated fixture experiment and see whether the provider-call observation changes. An observer that stays green under that mutation is weak evidence for deduplication.

This is an evaluation technique, not a default operation on every user change. It requires an adequate mutation, clean isolation, and bounded cost. A mutation score does not establish correctness.

## 10. The smallest effective implementation

Build one real end-to-end slice in a local CLI and thin TUI before a plugin framework or browser application. This supersedes the proposal's original browser-first plan. Keep capture, comparison, and receipt state separate from terminal rendering, but do not design a general UI or adapter platform before the first supported scenario works.

### The terminal experiment

The following screen is illustrative, not implemented.

```text
AFTER   working tree   inspected → latest

NEEDS ANOTHER LOOK
  Payment retry after 12h    1 request → 2 requests   observed
  Missing authorization     evidence stale          not rerun

OTHER CHANGES
  Response field removed    user.name               structural
  7 changes without an explanation                  inspect

Enter inspect   d code diff   p pin   r request rerun
```

Inspection reveals the exact scenario, both observations, producer, snapshot bindings, and limits. Requesting a rerun still requires execution authorization. A dependency edit first changes evidence applicability to stale; it cannot populate a new output until a real observation arrives.

Make the payment-retention fixture in section 6 actually execute. A fake provider is appropriate when its recorded requests come from the application under test. A hardcoded count, simulated card, or model's predicted result is not enough. Include an untouched control case and keep the absence of concurrency and production evidence visible.

**First slice**

- Capture two local Git states and expose the normal diff from the TUI.
- Import a supported test report with honest provenance and missing-data labels.
- Run a small existing HTTP/JSON fixture driver, by explicit permission, against two disposable local instances.
- Compare responses and one deliberately supported side-effect channel.
- Render a keyboard-driven list and inspector using deterministic observations, evidence links, source context, and visible unclassified changes.
- Save one expectation pin and reopen it conservatively when another edit changes its basis.
- Rerun the selected scenario after authorization and inspect the actual new result, completing the repeated-review loop.

HTTP is a useful first runtime boundary because it crosses implementation languages. It does not make arbitrary services runnable without setup. Existing artifacts remain the zero-execution entry point. TypeScript-specific analysis can improve selection later without defining the whole product.

**Next slice**

Add a bounded property-based input generator for the same driver, reduce a distinguishing case, and make the selected inputs and outcomes inspectable in the TUI. Add GitHub PR import after the local loop proves useful. Add another artifact type, such as a CLI result or browser trace, only if real users need it.

A TUI can validate orientation, source inspection, evidence labels, pins, and repeated review. It cannot fully validate rich screenshot comparison or spatial boundary exploration. Test those separately in a browser interface if the observed need justifies one. The final product form remains open.

**Do not build first**

A browser application, GitHub discussion synchronization, universal simulator, unrestricted autonomous test author, cross-language whole-program graph, custom agent runtime, or complete CI replacement.

### Acceptance checks for the first slice

1. A response stays identical while a side-effect count changes. AFTER exposes the side-effect difference.
2. An agent edits both implementation and expected output. The changed oracle stays visible; the frozen comparison still runs where compatible.
3. A relevant fixture, observer, runtime, dependency, or mask changes. Old evidence does not retain current applicability silently.
4. A later snapshot arrives before an earlier run finishes. The result stays attached to the earlier snapshot.
5. One version fails to start or rejects the driver. The comparison reports incomplete/incomparable, not pass or regression.
6. Repeated runs disagree. The UI reports instability rather than cherry-picking an outcome.
7. A scenario has no artifact or adapter. The UI shows not checked and retains the code diff.
8. Models are disabled. Import, comparison, inspection, and pins still work.
9. Repository-controlled content tries to execute a command or inject a false badge. It gains neither execution authority nor evidence status.
10. The selected case is real, but broader behavior is untested. The card does not generalize beyond its evidence.

Evaluate the TUI against an ordinary diff and a strong guided tour on unfamiliar changes and controlled follow-up edits. Include misleading tests, missing evidence, and harmless edits that reopen evidence unnecessarily. Measure engineers' correct explanations, defect detection, and re-review effort. The loop is successful only if it reduces reconstruction without increasing missed problems or mistaken trust in stale results.

Evaluate the engine on useful-example availability, false retained freshness, unnecessary reruns, comparison stability, and setup cost. More generated prose is not a success metric. A polished terminal screen alone does not validate the product.

## 11. Preferred implementation language

Go is the preferred language for the local engine, CLI, and first TUI. There is no fundamental mismatch with the proposed product. The TUI framework, storage implementation, and adapter versions remain choices to validate in the first slice, not settled requirements.

Go is well suited to subprocess supervision, structured artifact processing, local HTTP drivers, concurrent background work, and distributing a native command-line application. The core does not need a JavaScript runtime merely to display a review. Supported test drivers still need their own runtimes and dependencies; a single AFTER binary does not make an arbitrary project self-contained.

[Bubble Tea](https://github.com/charmbracelet/bubbletea) is a strong TUI candidate. Its documented model/update/view structure fits a responsive interface receiving background capture and execution results. Evaluate it with the actual diff workload before committing to the framework. Styling libraries are optional; terminal polish is not the first acceptance criterion.

### The main engineering cautions

- **Do not reimplement language intelligence in Go.** Generic artifact and HTTP comparison can remain in Go. TypeScript symbol resolution, test-specific capture, and other language-sensitive operations should use the appropriate compiler or runner through bounded subprocess adapters. Syntax parsing alone does not resolve semantic dependencies. These adapters need not all exist in the first slice.
- **Use Git's semantics rather than approximating them.** Prefer the installed Git CLI for snapshot and diff operations initially. Handle NUL-delimited path output where available, unusual filenames, renames, binary changes, and explicit index/working-tree/base semantics. Do not assume a pure-Go Git library covers every workflow the user expects.
- **Keep the TUI event loop responsive.** Run Git, parsing, capture, and fixture execution outside UI updates. Return bounded, snapshot-tagged results. Render a viewport rather than rebuilding an entire large patch on every keypress. Limit concurrent jobs and retained output.
- **Treat process supervision as real infrastructure.** Use explicit argument vectors, not shell-built command strings. Bound and drain stdout/stderr, handle cancellation and timeouts, and terminate owned child processes with platform-appropriate process-tree handling. Canceling a parent process alone may leave descendants running. None of this creates a sandbox.
- **Preserve the distribution advantage deliberately.** Native parser bindings or other CGO dependencies can complicate cross-compilation and packaging. Add them only for a demonstrated requirement. Do not pick a database, plugin system, or multi-language parser suite before the first scenario needs one.
- **Treat terminal content as untrusted.** Render repository text and captured output without executing embedded terminal control sequences. Test wide Unicode characters, tabs, long lines, narrow layouts, resize events, and selection behavior. These are correctness and security issues, not merely styling.

The recommended boundary is simple: Go owns snapshot identities, comparison records, review state, process orchestration, and terminal presentation; existing language tools produce scoped facts and test artifacts. Keep those concerns separable in ordinary code without inventing a framework. A later browser UI can consume the same comparison records if the interaction warrants it.

Before expanding scope, prove that the chosen TUI stays responsive on a large diff while a fixture runs, cancellation leaves no owned processes behind, and a late result cannot attach to a newer snapshot. Keep the core testable without launching an interactive terminal. These checks supplement the behavioral acceptance checks in section 10.

## Bottom line

The most practical version of AFTER is **a change-aware test and observation browser**.

Tests provide candidate experiments and expectations. Runners and artifacts supply observations. Deterministic comparators expose differences. People decide which differences they accept. An LLM can help ask better questions and explain the results, but it should not be the source of truth.

## Source notes

Documentation inspected October 2, 2026. These references establish available primitives, not a tested AFTER integration. Features and adapter APIs require version-specific checks when implemented.

1. [Vitest reporters](https://vitest.dev/guide/reporters), especially the JSON result example and coverage-map behavior.
2. [Vitest custom reporters](https://vitest.dev/advanced/reporters), including the API stability warning and artifact conventions.
3. [Fast-check test reports](https://fast-check.dev/docs/tutorials/quick-start/read-test-reports/), counterexamples, shrinking, and replay metadata.
4. [Playwright Trace Viewer](https://playwright.dev/docs/trace-viewer), action snapshots, screenshots, network details, and attachments.
5. [Playwright tracing API](https://playwright.dev/docs/api/class-tracing), recording options and trace artifacts.

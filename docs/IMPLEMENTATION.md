# Implementation contract

This is the bounded engineering plan for the CLI/TUI proof of concept, not a claim that AFTER exists yet. [Backlog.md](../backlog/tasks) is the authoritative work queue. [Behavior and evidence](behavior-and-evidence.md) supplies the product rationale; this document narrows its first slice so agents can deliver without inventing a platform.

## Definition of a convincing POC

An engineer can capture a dirty local Git candidate, browse its complete change inventory and an imported report without executing project code, authorize a frozen HTTP experiment on two isolated versions, see identical responses but different independently captured provider-request counts, pin an expectation, accept a new snapshot, see the pin reopen without invented results, and explicitly rerun it. This works without accounts, models, GitHub, or an editor integration.

The app, fake provider, clock, and HTTP driver must actually run. A seeded screenshot, a report's test name, or an expected-output file does not establish a payment effect. The shipped payment fixture has both a twelve-hour distinguishing case and an unchanged thirty-second control. The fake provider records calls; it does not supply the app's deduplication logic.

Technical completion requires every M1/M2 acceptance criterion and the release gate. User benefit is a separate human study, not something an autonomous agent can certify.

## Scope and defaults

| Area                  | POC decision                                                                                                                                                                                                                                        |
| --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Language              | Go engine, CLI, and TUI; no JS runtime dependency for the shipped executable                                                                                                                                                                        |
| Platforms             | macOS and Linux inspection; Docker Engine/compatible Docker CLI required for execution; no Windows execution promise                                                                                                                                |
| Git                   | Installed Git CLI with explicit argv, NUL-safe paths, no external diff/textconv/filter execution; never change the user's index or working tree                                                                                                     |
| Snapshot modes        | HEAD versus captured working tree by default; HEAD versus captured index; explicit merge-base branch comparison. No network fetch                                                                                                                   |
| Untracked files       | Excluded by default and inventoried; explicitly selected non-ignored files may be included. Never silently capture ignored secrets                                                                                                                  |
| Unsupported Git cases | Detect unmerged index, submodules, LFS-only content, missing objects, sparse/partial captures and non-regular files; show limitations or reject capture rather than pretend completeness                                                            |
| First report adapter  | Stock `go test -json`, with a documented tested Go version/dialect; status and available output only, never inferred request inputs or effects                                                                                                      |
| Fixture boundary      | Versioned HTTP/JSON request sequence plus observer-owned fake-payment request log; fixed clock and fresh state per version/case/repetition                                                                                                          |
| Storage               | Versioned, bounded JSON manifests/receipts and content-addressed artifacts in private, ignored `.after/`; no database or plugin system initially                                                                                                    |
| Invalidation          | Whole captured project footprint initially, including fixture, driver, observer, rules, toolchain, dependencies, and environment bindings; conservative unknown/stale when incomplete                                                               |
| Execution             | Opt-in, digest-bound authorization. Builds and dependency preparation count as execution. No host-process fallback if isolation is unavailable                                                                                                      |
| Sandbox               | Docker-based disposable isolation, non-root, no ambient credentials/host home/socket, read-only inputs, resource/time/output limits, and no external network. Exact topology/image digest is settled and attack-tested in AFTER-6 before the runner |
| Dependencies          | Trusted, explicitly provisioned toolchain image pinned by digest; standard-library-only demo. No implicit package download/install scripts during a run                                                                                             |
| TUI                   | Bubble Tea v1.3.10 selected by the [AFTER-12 terminal gate](TERMINAL.md); bounded safe-text viewport and background-job/PTY tests, with evidence browsing in AFTER-13                                                                               |
| Models                | None in the POC. No model can grant permission, set an evidence badge, or accept a review                                                                                                                                                           |

Do not add universal adapters, a semantic dependency graph, GitHub synchronization, a browser application, production replay, arbitrary URL execution, autonomous repair, or a second agent runtime. Generated boundaries and reduction belong to the explicitly selected follow-up milestone.

## Minimal records and trust boundaries

AFTER-1 owns the first schema documentation and exported core types. Subsequent tasks extend those only for an exercised requirement; avoid adapter registries and framework interfaces.

- **Snapshot:** immutable ID, source mode, resolved commit/merge-base identities where relevant, path/content/mode inventory, excluded/unsupported entries, capture completeness, ordinary diff identity. Hash content, not branch names or mtimes. A capture overlapping writes must retry within a bound or fail visibly; do not claim an atomic filesystem snapshot where Git/filesystem guarantees are weaker.
- **Scenario:** ID and concrete frozen setup/actions/input digest, driver and observer identities, comparison policy, supported boundary, authorship and limits. Expectations remain separate. Candidate test/oracle edits are inventory items, not automatic replacements for a frozen scenario.
- **Receipt:** schema version, producer/trust kind, snapshot pair, scenario/observer/rules digests, actual environment and toolchain identities on each side, argv, authorization identity, completion/timestamps, bounded artifact references, observation completeness/redaction and limits. References cannot escape owned storage.
- **Comparison:** compatible channel-level witnesses with exact values/paths; JSON object key order is immaterial, array order is meaningful. Missing, redacted, truncated or incompatible channels cannot produce complete equality. Repetitions disagreeing are unstable, never cherry-picked.
- **Pin:** specific scenario and expected result or explicitly broader human requirement, basis receipt/snapshot, persistent human decision and history. Reopening never rewrites the expectation or manufactures an observation.

Evidence kind/producer, applicability, execution/comparison outcome, and human decision are independent fields. CLI and TUI derive labels from these fields, never from repository prose, terminal text, test names, or imported booleans saying “observed”. Imported reports are reported evidence with unknown applicability unless explicit validated binding is supplied; digest binding is not a signature or proof of producer honesty.

Immutable records and atomic writes should survive crashes. One writer per review store is enough initially: reject concurrent mutation clearly instead of adding a multi-writer database. Reject unsupported newer schema versions without rewriting data. Current observations and private source captures stay out of Git by default. The distributed demo only contains deliberately synthetic, non-sensitive inputs.

## CLI and TUI behavior contract

The implemented headless command syntax, configuration and exit semantics are documented in [CLI.md](CLI.md). It covers `capture`, `import`, `inspect`, `compare`, `export`, `run`, `config`, and [headless pin/review actions](REVIEW.md); the [TUI browser](TUI.md) now supports stored evidence, raw inventory/source and explicit background capture/import. Pin/rerun integration remains AFTER-14.

```text
after capture [--staged | --base REF --target REF]
after import <go-test-json-file> --producer <caller-provenance> [--snapshot <id>]
after inspect <snapshot-or-record-id> [--base <snapshot-id>]
after compare <receipt-id>
after export <comparison-id>
after run <base-id> <candidate-id> --plan-out <private-file>
after run --plan-file <private-file> --approve <exact-preview-digest>
after config
```

Every run requires a specific approved plan/digest or an interactive confirmation of the displayed exact plan, never a blanket “yes to all repository commands”. The preview shows images, commands, mounts, network policy, limits, inputs and selected snapshots. A saved plan is reconstructed and checked byte-for-byte before execution. Import, inspection, comparison, help, config and preview do not execute repository code; non-TTY commands never read from stdin for confirmation. Configuration cannot authorize a run.

The TUI needs only a list, inspector, ordinary diff, and explicit actions:

- `Enter`: input, before/after outputs and effects, producer, bindings, limits and errors.
- `d`: ordinary diff, including every unclassified/unsupported change.
- `p`: pin an expectation, not an approval of the whole change.
- `r`: request rerun, inspect its plan and confirm; no execution on selection.
- Explicitly accept a new snapshot. A notification must not change the selected comparison underneath the user.
- Help, cancel and quit with owned-process cleanup and terminal restoration.

Sanitize terminal control sequences, including OSC clipboard/hyperlink sequences, on every untrusted surface. Render wide Unicode, tabs, long lines, empty results and narrow/resized terminals safely. Git/capture/execution jobs cannot block the UI event loop. Tag asynchronous results with their originating snapshot and request IDs.

## Dependency order and stop conditions

Backlog dependencies are the authority; this map explains milestones, not a second task-status database.

- **M1 / AFTER-1–11:** foundation and schemas → capture/store/import/diff and execution-isolation work → real fixture and paired runner → comparator/headless workflow → pins and conservative invalidation.
- **M2 / AFTER-12–17:** bounded TUI framework evaluation → inspectable TUI → follow-up review loop → adversarial acceptance gate → reproducible distribution/demo → evaluation kit.
- **M3 / AFTER-18–20:** human study, then optional boundary generation/reduction after an explicit go decision. Do not auto-select M3 when finishing the POC.

AFTER-1 is initially ready. Work on only tasks whose dependencies are Done; a lower ID alone does not imply readiness. All first-slice tasks are implementable without product research decisions from the user. Missing Docker or unsupported isolation is a real execution blocker: finish import/inspection and record an actionable setup request, never weaken the safety boundary. Framework/library versions and sandbox mechanics are bounded engineering decisions assigned to their gate tasks, not unspecified permission to redesign the product.

## Evidence and release gates

| Required design check                    | Owning tasks and observable check                                                                     |
| ---------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| 1. Same response, changed side effect    | AFTER-7/8/9/15: actual HTTP traffic; one versus two provider requests                                 |
| 2. Implementation and oracle both change | AFTER-5/8/15: oracle diff visible; frozen driver unchanged                                            |
| 3. Basis changes                         | AFTER-11/14/15: parameterized fixture/observer/runtime/dependency/rule changes reopen pins            |
| 4. Late result                           | AFTER-8/14/15: barrier-controlled out-of-order completion retains old snapshot binding                |
| 5. Start/driver failure                  | AFTER-8/9/13/15: incomplete/incomparable, never regression or equality                                |
| 6. Unstable repetitions                  | AFTER-8/9/13/15: every repetition retained, explicit unstable state                                   |
| 7. Missing adapter/artifact              | AFTER-4/5/13/15: not checked plus usable complete inventory/diff                                      |
| 8. Models disabled                       | AFTER-10/14/15: no provider/account/API key needed                                                    |
| 9. Hostile repository content            | AFTER-2/3/6/12/15: no implicit execution, terminal injection, traversal, credential or network access |
| 10. Finite evidence scope                | AFTER-9/13/15: exact inputs and sequential fake-provider limits, no universal claims                  |

The release gate additionally covers crash recovery, bounded outputs, cancellation of descendants/containers, contaminated environment variables, permission denial, dirty/index/untracked Git semantics and Unicode paths. Mutation testing of the demo must demonstrate that disabling deduplication changes the independent observer, not just its expected-output golden file.

Use deterministic clocks/barriers rather than timing sleeps in correctness tests. Document a generated medium/large diff benchmark and its hardware/cache state; measure raw-view latency and input responsiveness while a fixture is active. Warm raw view within one second and cached list within two seconds are evaluation targets, not portable guarantees. Budget failures must be visible, with scoped limitations, rather than silently dropping changes.

AFTER-16 adds the single documented demo/check command and CI jobs for the real engine. The current tooling CI does not test an unimplemented engine. M1/M2 completion must leave a clean checkout, runnable instructions, compatible checksums/builds for supported platforms, and actual receipt-backed demo evidence. Never fabricate results to complete a task.

## Human evaluation boundary

AFTER-17 produces a reproducible study kit with raw diff, a strong guided tour and AFTER conditions; unfamiliar changes, misleading tests, missing evidence and harmless edits; independent answer keys; randomized/counterbalanced ordering; and timing/correctness/trust scoring. AFTER-18 requires recruited humans and explicit scheduling authorization. It records anonymized actual observations, missed defects and unnecessary reruns. The proposed 12–16 participants and roughly 30% re-review improvement are targets from the proposal, not achieved metrics. No agent impersonates participants or calls an unrun study successful.

Use the outcome to continue, narrow scope, integrate another tool, or stop. Keep GitHub and richer browser experiments deferred until the local loop earns expansion.

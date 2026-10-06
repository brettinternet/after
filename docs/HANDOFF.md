# AFTER handoff

## Start here

Read [the product proposal](README.md), [the behavior and evidence design](behavior-and-evidence.md), and [the competitive research](research.md).

The concept migration is committed and pushed. The latest request authorized adapting tooling from `../project` and creating an autonomous implementation backlog for a robust CLI/TUI proof of concept. The headless [CLI](CLI.md), version-1 evidence records, private storage, local Git capture package, [Go test report importer](GO-REPORTS.md), [captured raw review API](RAW-DIFF.md), [offline sandbox proof](SANDBOX.md), [real synthetic payment fixture](../internal/paymentfixture/README.md), [frozen paired payment runner](RUNNER.md), [exact finite comparison](COMPARISON.md), and [persistent expectations/reopening](REVIEW.md) are implemented, along with the [captured evidence browser](TUI.md). The interactive pin/edit/reopen/consented-rerun loop is implemented with real Docker-backed PTY verification. AFTER-10's operator confirmation is a prompt or exact preview digest, not configuration consent. The earlier no-push instruction was superseded for the bootstrap request, not permanent authorization for future agents.

**Next implementation entry point: [the implementation contract](IMPLEMENTATION.md), [record schema](SCHEMA.md), [repository agent workflow](../AGENTS.md), and the next dependency-ready unclaimed Backlog.md task.** Backlog.md is authoritative: 30 POC tasks in M1/M2, including the AFTER-21–33 review TUI redesign specified in [TUI-DESIGN.md](TUI-DESIGN.md), plus three explicitly gated follow-ups in M3. Claim ready work from primary `main` and reread before creating an implementation worktree. Do not treat the human study as an autonomous deliverable.

**Latest direction: CLI + thin local TUI first, backed by a real paired-execution loop, with Go as the preferred implementation language. This supersedes the earlier browser-first recommendation. The browser presentation is a concept visualization, not the required first implementation.**

## Settled direction

- AFTER is an independent local tool. It is not a Hunk extension and has no required Hunk, editor, coding-agent, or model-provider dependency.
- Start with local Git snapshots in a thin TUI. GitHub PR inspection remains a product goal but follows validation of the local loop; defer PR synchronization.
- The TUI is a test instrument, not a terminal-only commitment. Separate capture/comparison state from rendering without building a general plugin framework.
- The central interaction is browsing concrete before/after examples, pinning an expectation, and reopening it when its supporting state changes.
- Tests, fixtures, contracts, static analysis, and captured traces supply candidate scenarios and bounded facts.
- Runners and identifiable external artifacts supply observations. Ordinary passing test reports usually do not contain enough inputs or side effects to reconstruct behavior.
- Deterministic comparators produce differences. LLMs may propose experiments, grouping, and explanations, but cannot manufacture observed/fresh badges or authorize execution.
- Observe effects as well as outputs. An HTTP response can remain identical while the number of payment-provider requests changes.
- Freeze the scenario, observer, and comparison rules across versions. Review changed test expectations separately so an implementation and its tests cannot silently move the oracle together.
- Evidence applicability, execution outcome, and human acceptance are separate states. Missing, stale, incomparable, and unstable results remain visible.
- The ordinary diff and unclassified-change inventory remain available when richer analysis fails.

Go is the user's preferred implementation language. AFTER-12 selects Bubble Tea v1.3.10 after the [terminal foundation evaluation](TERMINAL.md): a bounded safe-text viewport, measured large-capture responsiveness, and real PTY restoration tests. AFTER-13 adds the [engine-backed evidence browser](TUI.md), captured raw inventory/source pages, explicit background capture/import and real CLI PTY checks. Keep language-specific analysis and test capture in existing tools rather than rewriting them in Go. The implementation contract now bounds the first adapter to stock Go test JSON, storage to versioned local JSON/content-addressed artifacts, and execution to explicitly authorized offline Docker isolation. AFTER-1 establishes the [initial schema and its trust limits](SCHEMA.md); AFTER-6 provides [tested sandbox primitives and a pinned image](SANDBOX.md); the paired payment runner preserves their consent/isolation boundary and protects its observer in a separate container. Do not mistake the illustrative receipt in the design document for a finalized API.

## Where the answers are documented

| Question                                    | Location in `behavior-and-evidence.md`                                                                                                                 |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| What constitutes a behavior?                | Section 1, setup + stimulus + observation, with expectation separate                                                                                   |
| Can we derive it without an LLM?            | Sections 2–3, tests, fixtures, contracts, static facts, traces, generated cases                                                                        |
| What does a test report actually establish? | Section 2, report import versus richer artifacts versus capture adapters                                                                               |
| How is evidence observed?                   | Section 4, paired execution, provenance, observers, isolation, and limitations                                                                         |
| What if the agent edits the tests too?      | Section 5, frozen comparisons and candidate-owned oracles                                                                                              |
| How do we make the demo real?               | Section 6, fake-provider request counts, controlled clocks, threshold sampling                                                                         |
| How does review memory remain valid?        | Section 7, conservative applicability and dependency tracking                                                                                          |
| Where do LLMs help?                         | Section 8, optional interpretation and experiment proposals                                                                                            |
| How does this scale to rapid changes?       | Section 9, reuse, selection, grouping, explicit snapshots, instability                                                                                 |
| What should we build first?                 | Section 10, the TUI screen, real paired-run loop, and ten acceptance checks                                                                            |
| What would prove the TUI useful?            | Section 10, comparison against raw diffs and guided tours, including stale/missing evidence and unnecessary reruns                                     |
| Is Go appropriate, and what needs care?     | Section 11, preferred language, candidate TUI framework, language adapters, Git semantics, process handling, rendering, packaging, and terminal safety |

## Recommended first implementation experiment

When implementation is requested, build one working slice in a local CLI and thin TUI, not a browser application or general adapter platform.

1. Capture two local Git states and retain access to a normal diff.
2. Import one supported test-report format with accurate provenance and missing-data labels.
3. With explicit execution permission, run an existing HTTP/JSON fixture against two disposable local instances.
4. Capture responses and one supported side-effect channel, such as requests received by a fake payment service.
5. Compare structured results without an LLM and show the exact witness in a keyboard-driven list and inspector.
6. Pin one expectation, let another edit change its basis, and reopen the evidence conservatively without inventing a new result.
7. Rerun the selected fixture with authorization and inspect the new observation.

The [payment-retention fixture](../internal/paymentfixture/README.md) now executes for real in the offline sandbox: twelve-hour retries record one versus two provider requests with identical HTTP responses; the thirty-second control records one on both. Deduplication and printed-count mutations verify the independent recording path. The payment-specific paired runner now protects the observer and persists receipts; it is not a general application adapter. The payment-retention example must continue to execute for real, with a recorded provider-request count and an untouched control case. No hardcoded outcomes, mandatory LLM, or comprehensive dependency graph is needed. Inspect, open raw diff, pin, and request rerun are sufficient initial actions.

Prove the loop against ordinary diffs and a strong guided tour. Measure whether users understand consequential changes and re-review with less reconstruction without missing more problems. Include misleading tests, absent evidence, and harmless edits that cause unnecessary reopening. Do not judge success by terminal polish alone.

Then test generated boundary cases and counterexample reduction. Rich TypeScript impact analysis is optional. Add GitHub PR support after the local loop proves useful. Screenshot comparison and rich visual boundary exploration require separate browser experiments later; the TUI cannot establish their usability.

## Artifacts and current status

| File                       | Status                                                                                                                                                               |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `README.md`                | Full proposal; independent project boundary, TUI-first plan, and Go preference                                                                                       |
| `behavior-and-evidence.md` | Test-first, LLM-optional design, real TUI review loop, acceptance checks, and Go implementation cautions                                                             |
| `research.md`              | Sixteen primary-source comparisons; original local Hunk link replaced with a repository citation                                                                     |
| `presentation.html`        | Twelve-slide interactive simulation; standalone concept with a TUI-first implementation slide and mechanism link; browser views illustrate possible later interfaces |
| `presentation.pdf`         | Regenerated from the standalone HTML; twelve pages, no measured print overflow                                                                                       |
| `preview.png`              | Regenerated from the standalone HTML in Chromium                                                                                                                     |
| `IMPLEMENTATION.md`        | Bounded POC decisions, evidence contracts, CLI/TUI behavior and acceptance mapping                                                                                   |
| `../backlog/`              | CLI-managed milestones and 20 dependency-linked, acceptance-based tasks                                                                                              |
| `HANDOFF.md`               | This continuation guide                                                                                                                                              |

All payment data and evidence in the presentation are invented. The native CLI supports the [headless workflow](CLI.md), including explicit pin/review actions. The [TUI browser](TUI.md) inspects stored records without execution on open; explicit keys pin finite counts, accept captured candidates, and preview/authorize reruns. Restart uses immutable pin revision IDs, not automatic newest-result selection. Evidence stays bounded to the supported report format and frozen paired payment runner.

The destination presentation has now passed real Chromium checks for all twelve slides, keyboard navigation, slide 4's pin/edit/rerun and dialog interactions, slide 5's unexecuted-input state, 390px mobile layouts, no page errors or HTTP requests, and twelve PDF pages without measured print overflow. `scripts/check-presentation.mjs` reproduces the checks; outputs go to ignored `artifacts/presentation/`, and `task docs:render` updates the committed PDF/preview.

The original seven documentation artifacts were committed as `1337132` and pushed to `origin/main`. This bootstrap adds pinned tools, hooks, CI, local link/backlog checks and implementation tasks. Recheck Git state before beginning implementation; preserve any intervening user or agent work.

## Packaging and repeatable demo

AFTER-16 adds [native packaging and the owned demo](DEMO.md). `task package` emits unpublished versioned macOS/Linux amd64/arm64 binaries and checksums. `task demo:inspect` exercises capture/import without Docker; `task demo` walks real receipt-backed pin/edit/reopen/rerun with two exact consent prompts, while `task demo:proof` explicitly authorizes the same synthetic plans for CI. Successful setup/teardown owns only its private temporary workspace; failures retain diagnostic state. The root README now describes implemented capabilities rather than the initial scaffold. Human evaluation remains unrun.

## Evaluation kit and human boundary

AFTER-17 supplies the [version-1 study kit](EVALUATION.md): three matched payment cases, isolated missing/initial/follow-up checkpoints, strong guided-tour/raw/AFTER conditions, reproducible assignments, blank recording sheets and explicit scoring/limitations. `task study:check` checks native capture and assignment consistency; `task study:proof` explicitly authorizes six real synthetic runs and validates independent observer-backed keys. Rehearsals are not participant data. M1/M2 technical completion establishes neither usability nor market value. AFTER-18 requires operator authorization, 12–16 consenting unfamiliar engineers, a facilitator and second scorer, private consent ownership/deletion policy and scheduled 75-minute sessions. Do not autonomously start M3.

## Tooling and next action

Run `mise trust && mise install && mise exec -- task init`. The template's mise/Task/Lefthook/Prettier/Gitleaks/Worktrunk conventions are adapted without its application, databases, secrets or service stack. Go and Backlog.md are pinned; Bun/Playwright are development-only documentation dependencies.

Run `mise exec -- task build` for the native CLI, `task check:go` for build/race tests/vet/gofmt, `task test` for Go tests plus local links and task integrity, `task check:staged` before committing, and `task docs:check` for presentation changes. CI checks the Go foundation on macOS/Linux as well as documentation. AFTER-15 adds the [adversarial POC gate](POC-GATE.md): `task test:poc` authorizes all synthetic offline Docker/PTY proofs and requires binding/observer mutations to fail. It rejects skips; green foundation tests alone are not evidence that the full review loop works.

The documentation migration checks and artifact regeneration are complete. Continue from the next ready Backlog.md task when implementation is requested. Work through ready M1/M2 tasks; the final deliverable is a real repeatable payment demo, CLI, TUI review loop, security/negative tests, packaging and evaluation kit. M3 requires separate authorization and human participation.

The original hunk-guide source worktree is retained. This bootstrap did not adopt its cleanup authority, inspect its active owner/panes or operate outside the current repository's shell scope. It is not needed to build AFTER. Its optional cleanup is a machine-local maintenance concern, not an implementation prerequisite.

## Optional machine-local migration history

The original workstation may retain `after-migration-continuation.json` inside this checkout's Git directory. It contains historical paths and the original source-worktree creation receipt; it is private continuation state, not a portable project dependency or authorization to act.

Fresh clones should ignore that absent file and need no access to the source repository. Use the committed browser-check script here, not a historical external script. If the original operator separately requests source-worktree cleanup, first verify task continuity, inactive owners, the live receipt and exact checkout/panes under the applicable ownership rules. If any evidence or permission is unavailable, retain it. Never bypass a workspace scope restriction or infer cleanup authority from this document.

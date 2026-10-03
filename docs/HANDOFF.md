# AFTER handoff

## Start here

Read [the product proposal](README.md), [the behavior and evidence design](behavior-and-evidence.md), and [the competitive research](research.md).

The user asked to move the concept into this separate repository, document how it can work, and commit the documentation. The latest discussion selected a local TUI as the first experiment; the user requested that direction be documented for handoff. The current scope is documentation and migration completion, not permission to implement the entire product.

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

Go is the user's preferred implementation language. Bubble Tea is a candidate to evaluate, not a committed dependency. Keep language-specific analysis and test capture in the appropriate existing tools rather than rewriting them in Go. The specific algorithms, supported adapter versions, storage format, sandbox implementation, and TUI libraries have not been chosen. Do not mistake the illustrative receipt in the design document for a finalized API.

## Where the answers are documented

| Question | Location in `behavior-and-evidence.md` |
| --- | --- |
| What constitutes a behavior? | Section 1, setup + stimulus + observation, with expectation separate |
| Can we derive it without an LLM? | Sections 2–3, tests, fixtures, contracts, static facts, traces, generated cases |
| What does a test report actually establish? | Section 2, report import versus richer artifacts versus capture adapters |
| How is evidence observed? | Section 4, paired execution, provenance, observers, isolation, and limitations |
| What if the agent edits the tests too? | Section 5, frozen comparisons and candidate-owned oracles |
| How do we make the demo real? | Section 6, fake-provider request counts, controlled clocks, threshold sampling |
| How does review memory remain valid? | Section 7, conservative applicability and dependency tracking |
| Where do LLMs help? | Section 8, optional interpretation and experiment proposals |
| How does this scale to rapid changes? | Section 9, reuse, selection, grouping, explicit snapshots, instability |
| What should we build first? | Section 10, the TUI screen, real paired-run loop, and ten acceptance checks |
| What would prove the TUI useful? | Section 10, comparison against raw diffs and guided tours, including stale/missing evidence and unnecessary reruns |
| Is Go appropriate, and what needs care? | Section 11, preferred language, candidate TUI framework, language adapters, Git semantics, process handling, rendering, packaging, and terminal safety |

## Recommended first implementation experiment

When implementation is requested, build one working slice in a local CLI and thin TUI, not a browser application or general adapter platform.

1. Capture two local Git states and retain access to a normal diff.
2. Import one supported test-report format with accurate provenance and missing-data labels.
3. With explicit execution permission, run an existing HTTP/JSON fixture against two disposable local instances.
4. Capture responses and one supported side-effect channel, such as requests received by a fake payment service.
5. Compare structured results without an LLM and show the exact witness in a keyboard-driven list and inspector.
6. Pin one expectation, let another edit change its basis, and reopen the evidence conservatively without inventing a new result.
7. Rerun the selected fixture with authorization and inspect the new observation.

The payment-retention example must execute for real, with a recorded provider-request count and an untouched control case. No hardcoded outcomes, mandatory LLM, or comprehensive dependency graph is needed. Inspect, open raw diff, pin, and request rerun are sufficient initial actions.

Prove the loop against ordinary diffs and a strong guided tour. Measure whether users understand consequential changes and re-review with less reconstruction without missing more problems. Include misleading tests, absent evidence, and harmless edits that cause unnecessary reopening. Do not judge success by terminal polish alone.

Then test generated boundary cases and counterexample reduction. Rich TypeScript impact analysis is optional. Add GitHub PR support after the local loop proves useful. Screenshot comparison and rich visual boundary exploration require separate browser experiments later; the TUI cannot establish their usability.

## Artifacts and current status

| File | Status |
| --- | --- |
| `README.md` | Full proposal; independent project boundary, TUI-first plan, and Go preference |
| `behavior-and-evidence.md` | Test-first, LLM-optional design, real TUI review loop, acceptance checks, and Go implementation cautions |
| `research.md` | Sixteen primary-source comparisons; original local Hunk link replaced with a repository citation |
| `presentation.html` | Twelve-slide interactive simulation; standalone concept with a TUI-first implementation slide and mechanism link; browser views illustrate possible later interfaces |
| `presentation.pdf` | Copied from the earlier version; regenerate from the current HTML |
| `preview.png` | Copied from the earlier version; regenerate from the current HTML |
| `HANDOFF.md` | This continuation guide |

All payment data and evidence in the presentation are invented. No AFTER analysis or execution engine has been implemented.

The original presentation passed real Chromium interaction checks, desktop/mobile layout checks, zero-network checks, and a twelve-page PDF check before migration. The updated destination files have not received that final validation. Checks from the hunk-guide repository do not establish correctness of this independent project.

No documentation commit was created during this session. The destination was an existing empty Git repository on unborn `main` when inspected. Recheck its current state and preserve any work added by the user or another agent since then.

## Finish the documentation migration

1. Work from `/Users/brett/dev/me/after` and read any current repository instructions first.
2. Format and check the documents without importing Hunk's package manifest, runtime dependencies, or project configuration.
3. Check local links and ensure the HTML/proposal still describe a standalone project.
4. Exercise the presentation in an actual browser. Test slide navigation, slide 4's pin → edit → rerun sequence, slide 5's unexecuted-input state, and narrow layouts. Verify that no network calls occur during normal use.
5. Regenerate `presentation.pdf` and `preview.png` from the current HTML. Check twelve PDF pages and no clipping. The HTML is self-contained and needs no server.
6. Stage only the intended documentation and create the requested local documentation commit. Do not push or open a PR. Update this handoff's status when the pending work is done.
7. Reconcile the original source worktree only after the destination artifacts are safely committed and ownership is verified. Cleanup need not prevent the documentation commit if access or ownership remains unavailable.

## Original worktree and continuation evidence

The original five artifacts are still retained in:

```text
/Users/brett/dev/me/hunk-guide/.worktrees/after-concept/docs/after/
```

Machine-local continuation details, the original creation receipt, the previous validation-script path, and pending operations are preserved in:

```text
/Users/brett/dev/me/after/.git/after-migration-continuation.json
```

That file is local continuation state, not a document to commit. Its previous browser script still targets the original worktree; change its artifact destination before reuse, or create an equivalent check here.

Before adopting destructive cleanup authority, confirm task continuity, confirm the previous owner and delegated work are inactive, and match the stored receipt against the live checkout, branch, and Git-local receipt. Re-inspect the exact associated Herdr workspace and panes. If verified and permitted, remove the worktree and its branch together with Worktrunk from a surviving checkout, allowing teardown hooks to run, then verify the workspace disappeared. Never remove the primary hunk-guide checkout or the user's AFTER workspace. If ownership or tool access is unclear, retain the old checkout and report it.

The prior session's shell remained rooted in `hunk-guide` even after the user ran `cd ../after`; it blocked shell navigation outside that root. File edits succeeded, but shell validation and commit did not. Do not bypass a scope restriction. Continue with an agent whose permitted workspace is this repository.

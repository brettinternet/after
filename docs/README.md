# AFTER: review changes through examples

AFTER is a local Go CLI/TUI for inspecting Git changes and their evidence. This proposal describes a broader example-first review experience; the current build supports a bounded local workflow, not arbitrary application replay. See [the implementation contract](IMPLEMENTATION.md) and [CLI reference](CLI.md) for what works today.

**The idea:** browse concrete before/after examples, pin an expectation, and reopen it when its basis changes. Keep the ordinary diff and every unknown one action away. The promise is faster orientation and less repeated reconstruction, not automatic approval. AFTER is a working name and an independent project, not a Hunk extension, editor plugin, or model-provider service.

- [Behavior and evidence model](behavior-and-evidence.md)
- [Competitive research and sources](research.md)
- [Interactive concept presentation](presentation.html) · [PDF](presentation.pdf)
- [Handoff and implementation status](HANDOFF.md)

The presentation is a simulation. Its payment data and all displayed counts, evidence, and timings are invented; its controls never run repository code or contact GitHub. Open the standalone HTML locally; it needs no server or network. Use arrow keys or slide numbers to navigate, `N` for presenter notes, slide 4 for the pin/edit/rerun simulation, and slide 5 for the boundary explorer. The PDF is static. The repository's real synthetic payment fixture and consented runner are separate from this presentation demo.

## What works today

The current CLI/TUI captures local Git snapshots, keeps the raw diff and excluded or unsupported inventory available, imports stock `go test -json` reports, and stores and inspects evidence and expectation pins. Execution is limited to the frozen synthetic payment experiment in an explicitly authorized offline Docker sandbox. There is no model, account, editor, or GitHub dependency. General application adapters and GitHub review remain future work.

For example, this command ran inside a throwaway Git repo where `config.yml` changed from `retries: 3` to `retries: 5`:

```text
$ after capture
Captured candidate ca4ec43b (working tree) against base 86609ccd (commit)
  Base         86609ccd · 1 path · complete · 0 excluded · 0 unsupported
  Candidate    ca4ec43b · 1 path · complete · 0 excluded · 0 unsupported
  Limit        two matching reads; not an atomic filesystem snapshot
  Limit        diff includes captured regular files only; inspect excluded and unsupported inventory
Next
  after review
    open a review of this change
  after diff --stored
    print this captured patch
```

Capture reads Git state; it does not execute the project. See [CLI](CLI.md) for commands, flags, limits, configuration, and exit codes.

## The review object

A patch groups edits by file. A reviewer usually needs to know which observable examples changed, which changes are intentional, and what still has no evidence. One key expiration change, for example, can leave the worker code untouched while changing its effects:

> Same payment key; retry twelve hours later. Previously: one provider request. Now: two. The worker did not change.

That result describes one bounded scenario, not every retry. The reviewer can inspect its input, outputs, observer, code, and limits, then pin the expectation that matters.

| Card field     | Example                                                                |
| -------------- | ---------------------------------------------------------------------- |
| Condition      | Same key, twelve hours after the first request                         |
| Before → after | One provider request → two                                             |
| Evidence       | Inputs, outputs, observer, producer, snapshot and environment bindings |
| Source         | Exact changed and relevant unchanged code                              |
| Limit          | Sequential synthetic fixture; no concurrency or production provider    |
| Human state    | Unread, inspected, pinned, or needs another look                       |

A **pin** records a selected expectation and its basis. “One request for this sequence” is a finite example. “Never charge twice within 24 hours” is a broader human requirement; one example cannot establish it. Pins are optional. Without them, the contact sheet and raw diff still work.

## Five views over one comparison

The broader proposal organizes review into five views:

1. **Contact sheet.** Group consequences by example and evidence state. Show how many paths and hunks remain unclassified. The first review compares base with candidate; a follow-up explicitly chooses original-base or last-inspected against the new candidate. Never switch the selected pair underneath the reviewer.
2. **Boundary view.** Compare available inputs side by side. A selectable but unexecuted case says “not run,” not a predicted result. Inferred regions remain distinct from measured points.
3. **Cause strip.** Show a short path from an example to changed and unchanged code. Mark recorded execution, static relationships, and inference separately; an import or static edge does not prove a line ran. Say “suspected cause” until stronger evidence supports causation.
4. **Review memory.** Preserve the pin and old receipt when a dependency changes. Mark the evidence stale or unknown and name the changed basis. Only a new authorized run can supply a new observation.
5. **Code view.** Open the exact source and ordinary diff. Keep all paths, unclassified changes, and unsupported inventory reachable even when richer analysis fails.

Do not add a global risk score. Order by explainable signals—reopened expectations, public contract changes, unexplained edits, and unknown high-impact boundaries—and let reviewers inspect or override that order. “Two decisions” means two unresolved human choices, not two bugs.

## Evidence and trust

Evidence and human review are separate axes. A test pass is not acceptance; reading a card does not prove its claim.

| Label       | Means                                                              | Does not mean                 |
| ----------- | ------------------------------------------------------------------ | ----------------------------- |
| Observed    | A recorded run produced this result for these inputs and snapshots | Every input behaves this way  |
| Structural  | A bounded parser, compiler, or exact comparison established a fact | Runtime equivalence           |
| Inferred    | A model or heuristic predicts a behavior                           | Executed or verified          |
| Author says | An agent or author supplied the claim                              | Independent evidence          |
| Not checked | Evidence is absent or unsupported                                  | Safe                          |
| Stale       | A known binding changed                                            | The old observation was false |

Imported reports retain their producer and limits. A changed test assertion stays visible as a changed oracle. No model can create an observed or fresh badge, hide an unknown, authorize execution, or accept a review. The ordinary diff remains usable without fresh evidence or a model.

Inspecting, importing, or opening a checkout must not execute repository code. A worktree is not a sandbox. The supported run requires exact-plan consent, isolated Docker execution, no ambient credentials, bounded resources, and no external network; setup failures never fall back to host execution. The payment example uses a fake provider and moves no money. Local-first does not mean private inference: any future model use must be opt-in, identify the provider, and disclose what leaves the machine. See [sandbox limits](SANDBOX.md) and [the review workflow](REVIEW.md).

## First product, later goals

Start with local Git and a thin TUI. Go is the preferred language; Bubble Tea v1.3.10 is selected for the current terminal UI. Keep capture, comparison, and review state separate from rendering without introducing a plugin framework. Existing language tools should supply language-specific facts rather than being rewritten in Go. The TUI is an initial test instrument, not a commitment to a terminal-only product.

The implemented local loop covers the current bounded workflow. The larger proposal would add broadly available fixtures and artifacts, more supported observation channels, and later GitHub PR inspection. A future PR adapter should show exact base/head identities, inspect without switching a dirty checkout, and ask before publishing; full discussion synchronization is not a first-release requirement. The local loop must work without a model, forge, or editor integration.

Useful evidence varies by change:

| Work           | Useful comparison                                              | Honest fallback                |
| -------------- | -------------------------------------------------------------- | ------------------------------ |
| Backend API    | Same request, response and side effects                        | Source-backed prediction       |
| Frontend       | Same state and action; DOM, screenshot or accessibility output | Component and prop diff        |
| CLI or library | Arguments, exit status, output or public type result           | Export and syntax diff         |
| Configuration  | Same environment selector and effective values                 | Exact key/value comparison     |
| Migration      | Same fixture record and resulting shape                        | Schema diff; rollback untested |
| Refactor       | Existing observations plus identified moves                    | No equivalence claim           |
| Docs or assets | Factual text, image or binary metadata                         | Conventional diff              |

These are examples, not first-release adapter promises. Arbitrary infrastructure replay, screenshot comparison, and rich visual boundary exploration need separate experiments. A later local browser service must bind to loopback, authenticate sessions, and resist cross-site command requests.

## Measure the idea

The prototype should show a real observation changing, let a person pin it, edit its basis, see the pin reopen without an invented result, and rerun only after authorization. Compare it with an ordinary diff and a strong guided tour on unfamiliar changes and follow-up edits. Include misleading tests, missing evidence, and harmless changes that cause unnecessary reopening.

Measure correct explanations, consequential defect detection, repeated-review effort, trust in stale or inferred claims, setup cost, available examples, and unnecessary reruns. A proposed study uses 12–16 practicing engineers. Initial targets are roughly 30% less re-review time without worse defect detection, useful first use without handwritten contracts, and no silently retained-fresh pin in deliberate invalidation tests. These are hypotheses, not results; the human study has not been run.

The main risks are too few useful examples at first use and freshness logic that is either misleading or noisy. If the contact sheet helps but pins do not, ship the useful part. If reviewers prefer the tour and routinely leave for the raw diff, stop or change the wedge. A plausible distribution model is a free local browser with optional paid shared history and artifact sync; it is unvalidated, and basic diff browsing must not require an account. Research found substantial prior art for guided tours, semantic grouping, behavior comparison, review memory, and evidence freshness; the proposed distinction is the human-first example-browsing interaction and its low setup cost, not a novel verification technique. See [the research limits](research.md#research-limits-and-next-work).

**A diff should let you inspect a change in behavior, then remember why you accepted it.**

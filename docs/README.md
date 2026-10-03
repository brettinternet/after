# AFTER

## See what the code will do differently.

**A product proposal for an example-first diff browser.** Open a working tree or a GitHub pull request. Browse the consequences of the change. Pin the examples that matter. When the code changes again, see which of those examples need another look.

Working name only. This is a researched concept, not an implemented review engine or a claim of worldwide novelty. Research checked October 2, 2026.

- [Open the interactive presentation](presentation.html). Download/open the HTML in a browser if your Markdown viewer displays its source. No server, account, or network connection is needed.
- [How behavior is derived and evidence is observed](behavior-and-evidence.md), including a test-first, LLM-optional implementation plan.
- [Read the research and competitive assessment](research.md).
- [Read the presentation as a PDF](presentation.pdf). This copied edition still needs regeneration from the revised standalone HTML.
- [Agent handoff](HANDOFF.md), including settled decisions and remaining validation, commit, and cleanup work.

Use the arrow keys or numbered navigation to move through the twelve slides. Slide 4 has the pin → edit → rerun simulation; slide 5 has the boundary explorer. Press `N` for presenter notes. The PDF is the static edition.

The presentation uses invented payment-service data. All outputs, evidence states, counts, and timings in the demo are illustrative. Clicking a demo control never runs repository code or contacts GitHub.

## The recommendation

Build a **local-first browser of observable differences**, with a conventional diff always one action away.

The primary review object is a small comparison someone can understand without reconstructing five files in their head.

> Same payment key. Retry twelve hours later.
>
> Previously reviewed version → one charge.
>
> Latest version → two charges.
>
> The key now expires after five minutes. The worker did not change.

A reviewer can follow that example into the code, inspect how the result was obtained, and pin the expectation that matters. On the next agent iteration, the tool brings back the examples whose supporting code or evidence changed.

This is a different starting point from a narrated file list. It is also different from a feed of AI bug allegations. Intentional changes, preserved behavior, structural changes, and unknowns all belong in the browser.

**The product promise is faster orientation and less repeated reconstruction. It is not automatic approval.**

## What the research changed

The first ideas were not distinctive enough.

Linear Guides, Graphite Code Tours, CodeRabbit Change Stack, Limn, and hunk-guide already organize code into explanations. Diffcore adds flow-oriented review and graphs. SemanticDiff removes structural noise. Entire preserves agent context. HyperTest and ReGrade compare runtime behavior. Critique binds evidence to repository state and invalidates it. Proof maintains an intent graph with stale evidence and human acceptance. Semctx models change impact and explicit unknowns.

So the opening is not “AI understands your diff,” “a guided tour,” “a graph,” “a test receipt,” or “review memory.” Those ideas have substantial prior art.

The proposed wedge is a particular human interaction.

**Browse by example. Pin an expectation. Reopen it when its basis moves.**

That interaction should be usable on an ordinary uncommitted change, before someone has installed an organization-wide verification system, written formal requirements, instrumented production, or adopted a particular agent.

This is a product-design hypothesis. The researched pages do not establish that nobody offers this exact workflow. Critique and Proof are especially close on the underlying model; Limn is close on the local review experience. See the [competitive assessment](research.md) for what was actually checked.

## Why files stop being the right home screen

A patch organizes edits by where they live. An engineer often needs different answers.

1. What will behave differently?
2. Which differences do I want?
3. What important behavior is supposed to stay the same?
4. Which part of that answer is observed, inferred, or still unknown?
5. What has changed since I formed my last judgment?

AI-generated changes amplify a familiar mismatch. One consequence can span dozens of files; one file can carry several unrelated consequences. Generated code makes line count a particularly poor proxy for review effort. A change to a shared constant can matter more than a thousand lines of plumbing.

The ordinary diff remains essential for implementation quality, security, maintenance, and anything the higher-level view missed. AFTER changes the index into that diff, not its authority.

## The core object

Call it an **example card** in the interface, not a proof object or an obligation.

An example card contains:

| Field          | What the reviewer sees                                             |
| -------------- | ------------------------------------------------------------------ |
| Condition      | “Same key, twelve hours after the first charge”                    |
| Before → after | “One charge → two charges”                                         |
| Relevance      | “The advertised retry window is twenty-four hours”                 |
| Basis          | Observed run, structural fact, model inference, or author claim    |
| Evidence       | Input, output, command or artifact, environment, captured revision |
| Code           | Exact ranges plus relevant unchanged code                          |
| Limits         | “Sequential fixture only. Concurrent requests not tested.”         |
| Human state    | Unread, inspected, pinned expectation, needs another look          |

Not every card has a runtime example. A removed export can have a concrete import and a compiler result. A configuration edit can have an exact value comparison. A documentation change can have a plain factual edit. Missing runtime evidence must not make these changes disappear.

The system tracks **two independent axes**. Evidence has a type and freshness. Human review has a decision and scope. A passing test is not human acceptance. Reading a card is not evidence that its claim is true.

### A pin has a precise meaning

The default pin records the selected example and expected result, not an inferred universal property.

“Keep one charge for this key retried at twelve hours” is an executable expectation if the fixture exists. “Never double-charge any payment within twenty-four hours” is a broader human requirement. The interface can save that requirement too, but it must state that the selected example does not establish it.

Pins are optional and created during ordinary browsing. There is no mandatory specification-writing ceremony. Without pins, the contact sheet and raw diff still work.

## Five ways to see one change

These are lenses over the same snapshot, not separate products or a dashboard of unrelated widgets.

### 1. The contact sheet

The home screen is a compact sheet of consequences. Each row has one condition, one before/after contrast, and an evidence label.

```text
AFTER     payments / resilient-retries     working tree · snapshot 12

Needs another look
  Same key, retry after 12h      1 charge → 2 charges    Observed

Behavior changes
  Timeout with safe key         error → retry          Observed
  Legacy client, missing key    reject → reject        Observed example
  Webhook arrives out of order  unknown                Not checked

Inventory     66 mapped hunks · 54 folded candidates · 17 unclassified
              Show all 137 hunks
```

The comparison selector says exactly what “before” means. First review uses base versus candidate. Follow-up review can use last inspected versus latest candidate. Neither silently replaces the other.

No global risk score. “Two decisions” means two explicitly unresolved choices, not two bugs or all the remaining work. Ranking prioritizes reopened expectations, incompatible public contracts, unexplained edits, and unknown high-impact boundaries. The reviewer can inspect and override the ordering.

### 2. The boundary lens

Instead of asking a chatbot to explain the entire change, alter one relevant input.

```text
Same payment key       Retry delay

                       30 sec        12 hours        48 hours
Reviewed version       1 charge      1 charge        2 charges
Latest version         1 charge      2 charges        2 charges
                                      ↑ difference
```

Each selectable point comes from an available fixture or is explicitly marked as a proposed experiment. Selecting an unexecuted input shows “not run,” not a fabricated output.

A model might infer that behavior differs throughout the interval between the two expiration thresholds. The interface draws that region differently from the individual observed points. It does not imply that three examples exhaust the state space.

This is the central interaction worth inventing carefully. A person can discover the _shape_ of a change by browsing the conditions where the two versions diverge, with much less code reconstruction. For APIs, use requests and responses. For a CLI, use arguments, exit status, and output. For UI, use captured states and interactions. For a schema, use representative old records and migration results.

### 3. The cause strip

From an example, expand a short path into its cause.

```text
retry worker          key lookup             retention policy
UNCHANGED       →     source inspected  →    24 hours → 5 minutes
```

Distinguish execution edges recorded by a trace from static relationships and inferred relationships. A static import is not proof that the line ran. A replay mismatch locates a symptom; it does not by itself prove causation. Use “suspected cause” until the relationship has appropriate evidence.

Keep the initial view to a short path and let users expand it. A whole-repository graph is not the home screen. The important innovation is making relevant unchanged code visible, not drawing more nodes.

### 4. Review memory

The tool remembers a concrete reviewed example, its evidence, and the known dependency footprint.

An agent edits the retention configuration. The twelve-hour retry pin reopens even though the payment worker and the pinned source range are unchanged. The interface gives the reason.

> Needs another look. The retention setting used by this example changed.
>
> Previous observation remains in history. Current evidence is missing.

Only after an authorized new execution can it say that the latest version produced two charges. Invalidation and verification are separate events.

Other receipts may remain recorded against unchanged known footprints. That means “no recorded dependency changed,” not “all possible dependencies are identical” or “the behavior is universally safe.” If the dependency inventory is incomplete, freshness becomes unknown or the receipt reopens conservatively.

### 5. The code microscope

One key reveals the exact implementation, in a normal high-quality diff. Related hunks and unchanged context remain navigable. There is a persistent Show all changes action, an unclassified tray, and a source-coverage inventory.

Do not make people fight the summary to reach the code. A one-line correction may sensibly open straight into a diff. The example view earns its place on changes that need it.

## A ninety-second demo

All figures below are illustrative, not product measurements.

**0–15 seconds.** An agent has made an 84-file payment-retry change. Open AFTER. Raw changes appear first; the semantic layer can arrive later. The contact sheet gives eight grouped consequences and explicitly retains seventeen unclassified hunks.

**15–35 seconds.** Open duplicate-payment protection. Select the existing twelve-hour fixture. Both compared versions produced one charge. Read the fixture's limits and pin that specific expectation.

**35–50 seconds.** Inspect the adjacent “different key” and “expired key” examples. Two charges can be intentional. The tool describes differences rather than treating every divergence as a bug.

**50–65 seconds.** The agent changes key retention from twenty-four hours to five minutes. The review snapshot stays still. A new-snapshot indicator appears; accepting it reopens the pin because its known dependency changed.

**65–80 seconds.** Rerun the selected fixture with permission. Now the twelve-hour example differs. Follow it through the unchanged worker to the modified configuration. The fixture establishes this case, not all retry behavior.

**80–90 seconds.** Draft a request to preserve the expected retry window, including the exact input, observed outputs, evidence limits, and source anchors. Send it only after confirmation. When the next candidate arrives, the example reopens for verification rather than silently turning green.

The useful moment is not an AI saying “possible idempotency issue.” It is the reviewer seeing the concrete change and knowing exactly what to inspect next.

## Two entry points, one review model

The following commands are proposed UX, not installed commands.

```text
after                  Open a captured working-tree comparison
after --staged         Compare HEAD with the captured index
after main...HEAD      Review branch changes from the merge base
after pr 482           Inspect a GitHub PR without switching this checkout
```

### Local agent loop

Capture staged, unstaged, deleted, binary, and selected untracked changes explicitly. Display which source is selected. Keep excluded files and unsupported artifacts visible by count and path. Avoid leaking ignored secrets into captures or external model context.

Capture file content consistently. If a file changes during capture, retry that capture or label it inconsistent. Never attach evidence to a blend of two agent revisions. A watch notification means a newer snapshot exists, not that the review silently changed beneath the cursor.

Review state is local and durable, independent of agent logs. Optional agent context helps explain intent but is author-supplied, not evidence. No transcript access is required for first use.

### GitHub review

Read PR metadata, the exact base/head identities, checks, and published comments through an adapter. Fetch required Git objects without checking out over a dirty working tree. Show merge-base comparison semantics and base movement explicitly. A remote update becomes a new candidate, not an invisible refresh.

Local inspection works without write permission. Draft comments privately. Publishing, requesting changes, approving, or handing a repair to an agent requires explicit human confirmation. If the PR head changed after review, block stale publication or ask for an explicit re-review decision.

Pins can travel from local work to the PR only after content and dependency reconciliation. A matching title or rewritten history is not sufficient. Preserve earlier decisions as history if equivalence cannot be established.

The first version can deep-link to GitHub for submission. Full discussion synchronization is not required to test the central interaction.

## The trust model is part of the design

Use plain labels with a source and a time, not confidence percentages.

| Label       | Meaning                                                                   | Does not mean                |
| ----------- | ------------------------------------------------------------------------- | ---------------------------- |
| Observed    | Captured execution produced this result for these inputs on this snapshot | All inputs behave this way   |
| Structural  | A parser, compiler, or exact comparison established a bounded fact        | Runtime equivalence          |
| Inferred    | A model or heuristic predicts this behavior                               | Executed or verified         |
| Author says | Intent or evidence reported by the authoring agent                        | Independent verification     |
| Not checked | Evidence is absent or analysis is unsupported                             | Safe                         |
| Stale       | A dependency, input, toolchain, or environment binding changed            | The earlier result was false |

An artifact imported from CI retains its producer and binding. A changed test assertion must be shown as a changed oracle, not accepted as proof of preserved behavior. Before/after comparisons should use the same selected fixture and controlled inputs; where dependency installation differs by revision, record both environments. Do not erase that difference with an “identical environment” claim.

Repository content and model output are untrusted. They cannot authorize commands, select unrestricted executables, dismiss unknowns, approve a PR, or publish content.

### What safe execution requires

A separate worktree is not a security sandbox. Test execution is an explicit operation in isolated compute, without ambient credentials or production access, with resource limits and no network by default. Dependencies need a deliberate supply policy. Importing existing results is the default first-release path.

Build failure, nondeterminism, missing services, incompatible fixtures, and unsupported languages are visible results. The normal diff stays usable when models, analysis, or execution fail.

Local-first also does not automatically mean private inference. Model use is opt-in, with a visible provider and explicit disclosure of what leaves the machine. A local model or no model remains possible.

## How to make it fast enough

The product must be useful before analysis completes.

1. Capture and render the ordinary diff and inventory immediately.
2. Add deterministic facts such as paths, symbols, exact config values, and recognizable structural changes.
3. Reuse bound results and existing examples where they are still applicable.
4. Generate compact example suggestions and grouping asynchronously.
5. Run selected experiments only on demand with permission.

Avoid a model pass per file per keystroke. Cache immutable captures, analyze touched neighborhoods, coalesce bursts of writes, and preserve the user's selected snapshot. A million-line generated change should not become a million-line model prompt.

Use virtualized lists and byte/size limits. Chunk oversized changes without pretending that a partial analysis is complete. Count total, mapped, folded, and unclassified hunks with deterministic inventory bookkeeping. A hunk can support several cards, so card membership must not double-count coverage. “All hunks represented” still does not mean “all behavior understood.”

Targets for a prototype evaluation, not promises: a useful raw view within one second on a warm medium-sized repository, a cached contact sheet within two seconds, and no blocking model call on opening a diff. Cold semantic analysis can take longer and must say so.

## The smallest product that tests the idea

Start with a local CLI and a thin TUI backed by real observations. This supersedes the earlier browser-first recommendation. The TUI belongs beside the coding agent and lets us test repeated review without changing editors or building a richer visual surface first.

The first experiment must execute a real scenario against two snapshots, show its outputs and a captured side effect, let the reviewer pin an expectation, then reopen and rerun it after another edit. Hardcoded outcomes or LLM-only explanations cannot validate this loop.

Go is the preferred language for the engine, CLI, and TUI. Bubble Tea is a candidate, not a settled framework choice. Use existing language-specific tools for semantic analysis and test capture rather than requiring those capabilities to be rewritten in Go. The [implementation-language notes](behavior-and-evidence.md#11-preferred-implementation-language) cover process supervision, rendering, packaging, and safety tradeoffs.

Keep capture, comparison, and receipt state separate from terminal rendering without introducing a general plugin framework. The TUI is the initial test instrument, not a commitment to a terminal-only final product. The existing browser presentation illustrates possible later interfaces.

Start with:

- Any Git repository for ordinary patch browsing and explicit unknowns.
- Existing structured test reports and artifacts for an immediate, no-execution entry point.
- Existing HTTP/JSON fixtures for the first authorized paired-run adapter.
- TypeScript source relationships as a later selection improvement, not a product dependency.
- One contact sheet, one example inspector, optional pins, conservative invalidation, and the ordinary diff.
- Local working-tree snapshots first. Defer GitHub PR import and synchronization until the local review loop earns repeat use.

Do not start with a new agent harness, universal sandbox runner, full requirements graph, autonomous repairs, team policy engine, whole-repository 3D map, or formal equivalence prover.

Inspection should remain useful without fresh execution. Organize structural facts and imported observations, preserve their provenance, and show missing evidence explicitly. Running code remains a separate authorized operation. Nevertheless, the proof-of-concept experiment must include one real paired-run adapter so we can test the entire observation → pin → edit → invalidate → rerun loop.

Compare the TUI against an ordinary diff and a guided tour on unfamiliar changes and follow-up edits. Success means less reconstruction and lower re-review effort without worse defect detection or mistaken trust in stale evidence. Include misleading tests, missing evidence, and harmless changes that cause unnecessary reruns.

Next, test generated boundary examples and counterexample reduction. Add GitHub PR support after the local loop proves useful. Rich screenshot comparisons and visual boundary exploration may justify a browser interface later; the TUI does not validate those interactions. Any later local browser service must bind to loopback, authenticate sessions, and resist cross-site command requests.

### Standalone product boundary

AFTER is a separate project. It is not a Hunk extension, a new guide format, or a companion service that depends on Hunk. It owns its interface, evidence model, comparison engine, review state, and Git/GitHub adapters.

The earlier guided-tour work is prior art, not an architectural constraint. Retain the useful principles of exact source navigation, explicit snapshots, visible unknowns, and access to every change. Design the example browser and evidence capture around their own requirements.

The core must work without a model provider. Tests and fixtures supply candidate experiments; runners and imported artifacts supply observations; deterministic comparisons supply differences. Optional models can propose useful experiments and explain results, but cannot manufacture evidence. The [behavior and evidence design](behavior-and-evidence.md) describes the mechanism and its acceptance checks.

## Does this serve ordinary engineering?

| Work           | Useful example or fact                                         | Honest fallback                         |
| -------------- | -------------------------------------------------------------- | --------------------------------------- |
| Backend API    | Same request, response/error/side effect before and after      | Source-backed prediction                |
| Frontend       | Same state and action, paired capture and accessibility output | Component and prop diff                 |
| CLI or library | Same arguments, exit status/output/public type result          | Export and syntax diff                  |
| Configuration  | Same environment selector, effective values                    | Exact key/value comparison              |
| Migration      | Same fixture record, resulting data shape                      | Schema operations and untested rollback |
| Refactor       | Existing observations agree; moves and renames identified      | No equivalence claim                    |
| Docs or assets | Factual text change, image comparison, binary inventory        | Conventional diff or metadata           |

These do not all need bespoke first-release adapters. They demonstrate that the central object is broad. Runtime replay for arbitrary infrastructure is not an MVP promise.

The initial user is an engineer repeatedly reviewing multi-file agent work. The mechanism also helps human-written PRs. “AI-native” describes the rate of iteration, not an exclusive dependency on AI authorship.

## Why this could win, and why it might not

The advantage would come from accumulated review context, excellent navigation, and reliable evidence bindings. An AI summary prompt is not defensible. A generic diagram is not defensible. Pins that correctly preserve or reopen a review across real messy edits could make the tool habitual.

The hard problems are example availability, behavior grouping, and incomplete dependency knowledge. False freshness is dangerous; excessive invalidation destroys the time savings. Tests can also agree while production differs. The interface must keep these limits legible without becoming an assurance bureaucracy.

Incumbents could add this workflow. If Limn or Linear makes examples and freshness first-class, the distribution advantage shrinks. Critique, Proof, and Semctx could be evidence suppliers or competitors. Integration is more sensible than rebuilding every verifier.

A local free tool with optional paid shared review history and artifact synchronization is a plausible distribution path. It is not a validated business model. Do not make the basic diff browser depend on account creation or a per-seat enterprise sale.

## How to decide whether to build it

Run a counterbalanced study with 12–16 practicing engineers using real, unfamiliar changes and controlled follow-up edits. Compare the ordinary diff, a good guided tour, and the example-first prototype. Include tasks where the prototype's interpretation is wrong or incomplete.

Measure:

- Time to correctly explain the important consequences, scored against independently established answers.
- Detection of consequential regressions, including unchanged-code interactions.
- Re-review effort after a new agent iteration.
- How often users mistake inferred or stale evidence for a current observation.
- Setup burden, number of useful available examples, and raw-diff escape frequency.
- False retention and unnecessary reopening of pins under dependency, environment, test, and base changes.

Proposed go/no-go targets: materially lower re-review time, roughly 30% as an initial target, without reduced defect detection; useful first use without handwritten contracts; and no silent retained-fresh state in the deliberately constructed invalidation tests. A small study would not establish general safety or market demand.

If the contact sheet is useful but pins are not, ship the contact sheet. If people prefer the tour and almost always escape to the raw diff, stop. If the benefit requires months of instrumentation, the wedge has drifted into a different product.

## The line to remember

**A diff should let you inspect a change in behavior, then remember why you accepted it.**

Not less access to the code. Less reconstruction between the code and the decision.

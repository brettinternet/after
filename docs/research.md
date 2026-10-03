# Research behind AFTER

Primary product documentation, repository READMEs, and research pages inspected October 2, 2026. This is a public-documentation comparison, not a hands-on benchmark or exhaustive market survey. Capabilities below are vendor/project descriptions unless explicitly identified as local repository facts.

Search results were used for discovery, then the cited primary pages were inspected. GitHub READMEs were retrieved through `gh api`. Marketing accuracy, performance, maturity, and user adoption were not independently verified. Current webpages may differ from the inspected versions.

## The conclusion

There is no credible whitespace in “AI summaries of diffs,” “semantic groups,” “guided tours,” “flow graphs,” “agent provenance,” or “evidence becomes stale.” There is substantial overlap even in combining claims, evidence, dependencies, and human decisions.

The narrower opening worth testing is **an example-first human browsing experience, available directly on local Git changes, with optional expectation pins and dependency-aware re-review**. Its distinction is the primary interaction and adoption cost, not a new verification technique.

I did not find enough evidence to claim that this exact workflow is absent from all existing products. No novelty or patent claim is intended.

## Directly inspected comparisons

### 1. Linear Guides

[Reviews documentation](https://linear.app/docs/diffs#guides) · [Code review should be fast](https://linear.app/now/code-review-should-be-fast)

The documentation says Guides organize related changes into structured sections, explain purpose and impact, surface core implementation first, and separate supporting changes. Guide sections link to the relevant diff. Linear also offers structural highlighting and GitHub-synchronized review actions.

**Implication.** A polished narrative beside a diff is not a differentiator. Keep its directness and code proximity. AFTER would use an observable contrast as the starting object rather than a section of explanation.

### 2. hunk-guide

[Repository and README](https://github.com/brettinternet/hunk-guide), inspected locally in the original hunk-guide checkout during the research. AFTER is a separate project.

The plugin groups edits into ordered sections, navigates exact targets, focuses Hunk's existing diff, retains Show all changes, validates generated output, and exposes explicit checkpoints for live edits. Reviewed targets can become stale when their definitions or containing files differ. Review state and checkpoints are session-local; runtime code does not call model providers.

**Implication.** Local conceptual navigation and checkpoint awareness are already here. Dependency-aware example pins are a larger extension than a new summary format. Preserve the public API and fail-open boundaries.

### 3. Graphite Code Tours

[Introducing Code Tours](https://www.graphite.com/blog/code-tours)

The announcement describes a continuous narrative using the PR description, conversations, stack, and code. Visual artifacts and test validation are described as upcoming work on this particular page. Reviewers can invoke an agent to investigate.

**Implication.** A tour plus attached tests and visuals is an anticipated direction, not a durable differentiator. Do not present the announced future features as established shipping capabilities from this page alone.

### 4. CodeRabbit Change Stack

[How CodeRabbit Review reads a PR](https://www.coderabbit.ai/blog/coderabbit-review-reads-a-pr-how-author-would-explain-it)

The article describes semantically related code blocks grouped into cohorts and ordered by dependency, with range-specific summaries and diagrams. It describes a context and verification system beneath the walkthrough.

**Implication.** Grouping by dependency and progressive narrative layers are crowded. A smaller product should not compete on the broad claim that it understands a PR better than a summary bot.

### 5. Limn

[Repository and README](https://github.com/glebmish/limn)

A macOS app for local branch review backed by Claude or Codex. It describes semantic grouping, narration, spec/plan comparison, comments, source navigation, local review state, and views of changes since viewing or approval. It watches for branch updates. The README calls the distribution a demo tool rather than a production product.

**Implication.** This is among the closest UX competitors. “Local, agent-agnostic guided review with memory” is already taken as a concept. AFTER needs the example-and-boundary interaction to be genuinely useful, not just different terminology.

### 6. Diffcore

[Product page](https://understandingdata.com/tools/diffcore/)

The page describes structural analysis, flow groups, call-graph visualization, ranking, optional model refinement, comments, and a replay mode that walks through files in execution order.

**Implication.** A causal-looking map and a guided execution path are not new. Importantly, the documented replay mode is a navigation sequence; it should not be confused with executing identical inputs against two program versions.

### 7. Differential

[Repository and README](https://github.com/thepartly/differential)

A terminal reviewer that separates important work, groups by semantic similarity, orders dependencies, and supplies context. It can render a plan as synthetic commits and includes unclassified hunks and folded repetitive changes.

**Implication.** Even semantic terminal review and explicit remainder accounting have strong precedents. AFTER should adopt visible unclassified inventory, not claim to invent it.

### 8. Agent Diff Visualizer

[Product page](https://galadriel-tech.com/adv)

The page advertises local-first semantic task grouping, risk signals, recovered agent intent, scope drift, dependency topology, review tracking, and an atomic reversion timeline. The same page's roadmap describes later reversion/snapshot work, so implementation completeness is not established by the page alone.

**Implication.** Agent logs plus semantic groups, risk scores, and a timeline are not a novel bundle. Avoid reproducing this surface with another name. Treat the advertised maturity cautiously.

### 9. SemanticDiff

[What is SemanticDiff?](https://semanticdiff.com/docs/what-is-semanticdiff/)

A language-aware diff for VS Code and GitHub. It suppresses syntax-level noise and recognizes moves, renames, and language-specific invariances. Unsupported formats use a fallback diff.

**Implication.** Structural noise reduction is useful infrastructure, not AFTER's central invention. “Semantic diff” can mean syntax-aware comparison; do not imply that it automatically establishes runtime equivalence.

### 10. Entire Checkpoints

[The Entire CLI](https://entire.io/blog/the-entire-cli-how-it-works-and-where-its-headed)

Checkpoints preserve prompts, transcripts, code state, attribution, and other session metadata linked to Git history. The article discusses intent-oriented review and temporary versus permanent storage.

**Implication.** Provenance and recoverable agent history are already product categories. They explain how a change was produced; they do not by themselves establish what its code will do. AFTER should work when this provenance is unavailable.

### 11. HyperTest

[Product page](https://www.hypertest.co/)

Its described workflow captures execution traces, builds an execution map, replays baseline inputs on a feature branch with mocked outbound calls, and compares divergences. The FAQ acknowledges that paths without traces fall back to static analysis.

**Implication.** Before/after execution evidence in code review is not new. AFTER's boundary browsing and local-first human workflow would need to be the distinction. The site's accuracy and time-saving claims were not validated and are not adopted as evidence for AFTER.

### 12. Curtail ReGrade

[Product page](https://www.curtail.com/)

The page describes recording requests, replaying them against a candidate, and comparing field-level responses and performance. It offers code-review evidence and agent access through MCP.

**Implication.** “Same input, two outcomes” is an existing testing and review capability. AFTER should consume comparable artifacts where possible rather than build every replay system. Research and performance claims on this vendor page were not independently checked.

### 13. Critique

[Independent verification protocol](https://www.critique.sh/docs/architecture/independent-verification) · [Product page](https://www.critique.sh/)

The protocol describes durable claims, evidence, findings, decisions, and completion objects. Evidence binds to repository, environment, tool, and relevant-file state. Relevant changes make evidence stale; missing dependency information fails closed. Author-supplied claims do not become independent proof. The product page emphasizes a CLI verifier used by coding agents.

**Implication.** This is direct prior art for the proposed evidence and freshness model. AFTER cannot honestly claim to invent bound receipts or conservative invalidation. The distinction would be a lightweight, human-first comparison browser rather than owning an independent reviewer harness and completion protocol.

### 14. Proof

[The Software Intent Graph](https://reqproof.com/product)

The page describes requirements, dependencies, hazards, implementation, evidence, issues, and change records. It explicitly separates readiness from human acceptance and describes automatic invalidation of stale evidence. It also labels its displayed portal a seeded product demo.

**Implication.** This is strong conceptual overlap, including the separation of evidence and acceptance. The proposed difference is starting with opportunistic examples on an ordinary diff, not requiring a maintained requirements graph. The inspected page does not establish adoption, general accuracy, or an absence of lightweight workflows.

### 15. Semctx

[Repository and README](https://github.com/hoklims/semctx)

A local-first change-impact tool mapping diffs to symbols, contracts, invariants, and statically related tests. It describes provenance, explicit unknowns, analysis freshness, and authored intent. It explicitly distinguishes static impact analysis from executing runtime checks; TypeScript is the compatibility baseline.

**Implication.** Contract-aware blast radius and explicit uncertainty are existing capabilities. This could be a source of bounded impact facts, subject to capability and correctness checks, rather than a subsystem to recreate.

### 16. Microsoft Research SymDiff

[Project overview](https://www.microsoft.com/en-us/research/project/symdiff-differential-program-verifier/)

Differential verification relates two program versions and can express equivalence or other relational properties. SymDiff uses Boogie and specifications called mutual summaries. The project predates today's agent tooling by many years.

**Implication.** Behavioral comparison and relational verification are not new research ideas. A runtime example establishes one observation. It is not a substitute for a proof over all relevant inputs.

## The strongest competing explanations

| Proposed differentiator         | Why it is insufficient                         | What remains to test                                                  |
| ------------------------------- | ---------------------------------------------- | --------------------------------------------------------------------- |
| A guided semantic tour          | Linear, Graphite, CodeRabbit, Limn, hunk-guide | Example-first entry rather than narrative-first entry                 |
| A flow graph                    | Diffcore, ADV                                  | Tiny evidence-labeled cause strips tied to a selected example         |
| Runtime evidence                | HyperTest, ReGrade                             | Interactive condition browsing inside a general diff browser          |
| Persistent review state         | Limn, hunk-guide's session checkpoints         | Review examples surviving revisions with explicit dependency limits   |
| Bound evidence and invalidation | Critique, Proof, Semctx                        | An optional one-click pin instead of adopting a verification workflow |
| Local and agent-neutral         | Many of the above                              | Near-zero-setup utility before models or execution are available      |

No row is an exclusive capability claim. The hypothesis is that the combination can be made unusually direct and broadly useful.

## Other directions considered

### A spatial atlas of the repository

Attractive screenshots, weak default interaction. Large dependency graphs are hard to scan and often imply more causal knowledge than the analysis has. Use a small expandable path after selecting a question, not a map as the entrance.

### A movie of everything the agent did

Useful for archaeology, attribution, and rollback. Usually the wrong compression for deciding what will ship. Reading the production history of a bad change can be slower than examining its final consequence. Existing checkpoint products also cover much of this ground.

### An inbox containing only decisions for the human

Promising but too easy to hide missing work. A tool that says “only these two questions matter” can be dangerously persuasive. AFTER can prioritize explicit choices while keeping source inventory and unknowns visible.

### Automatically split every large change into ideal commits

Useful for sequencing and author hygiene. It depends on whether edits can be separated without invalid intermediate states, and it does not solve understanding the resulting behavior. Differential already explores synthetic review stacks.

### A universal counterfactual simulator

The most exciting long-term interface, but an implausible first product. Arbitrary applications need services, state, credentials, timing, and environment control. Start with available observations and explicitly supported experiments. Never draw fabricated output while a real runtime is unavailable.

### A second autonomous agent that certifies the first

A different product category with existing specialists. Agreement between models is not independent execution evidence, and more agents can increase latency and cost without improving human understanding. AFTER should display their evidence and limits, not sell another opinion as a proof.

## Research limits and next work

This pass compared documentation, not installed products. It did not validate claimed performance, inspect every competitor's implementation, review patents, or interview users. Feature availability changes quickly. A few discovery results pointed to hackathon submissions and small projects; those were not treated as mature competitors.

Before building an engine, install and use Limn, Diffcore, a leading guided PR interface, and a bound-evidence verifier on the same repository. Attempt the exact pin → dependency edit → re-review interaction. Observe engineers doing that task today. If an existing tool already handles it elegantly, integrate with it or choose a different wedge.

The most important unanswered question is whether enough useful examples exist at first use. The second is whether conservative freshness can reduce repeated review without becoming either misleading or noisy. Neither question is answered by a presentation.

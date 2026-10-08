# Extension design

Status: plugin proposal, not implemented. Native [JUnit XML import](JUNIT-REPORTS.md) is implemented by AFTER-48; declarative scenarios remain proposals. The POC has no plugin system; see the [implementation contract](IMPLEMENTATION.md). This page records how AFTER could accept community and private integrations without weakening its evidence labels. Follow-up tasks AFTER-48–50 in the [backlog](../backlog/tasks) build the first concrete cases.

## The problem

Each language, test runner, application type and observation channel is a separate integration. One Go binary cannot own all of them, and [behavior and evidence](behavior-and-evidence.md#11-preferred-implementation-language) already says language facts should come from existing tools. Today every integration is hardcoded: `internal/gotestreport` imports Go JSON and JUnit XML, and `internal/runner` runs one payment experiment.

## Why not copy Pi's model

[Pi](https://pi.dev) extensions are TypeScript modules loaded into the host process. They register tools and commands and can observe, block, rewrite or replace nearly any event, with the user's full permissions. Distribution is just as open: a file in a directory, or a package from npm, git or a local path, gated by a project-trust prompt (Pi 1.1.0 documentation, inspected October 8, 2026).

That fits an agent harness, where the user is the only authority and the transcript is not a trust artifact. In AFTER the evidence label is the product. If a plugin could write `kind: observed`, edit a comparison or hide unknown inventory, every card would mean "author says."

## Open surface, closed labels

Plugins supply bytes, plans and presentation. The core assigns meaning by how data arrived, never by what a plugin claims:

| How data arrived                                     | Label ([vocabulary](README.md#evidence-and-trust)) |
| ---------------------------------------------------- | -------------------------------------------------- |
| Parsed from an imported file                         | Reported                                           |
| Derived from captured bytes without execution        | Structural                                         |
| Recorded by an observer during an approved run       | Observed, naming the observer                      |
| Proposed by a suggester or model                     | Inferred; never evidence                           |
| Produced through an `after-<name>` command or import | Reported, through the same import path             |

Plugins may move state only in the conservative direction: add a limitation, veto a run, or mark evidence stale. They never grant consent, promote a label, hide inventory or rewrite stored records. A plugin's content digest is part of every binding it touches, so upgrading a plugin reopens affected pins through the existing [reopening rules](REVIEW.md#applicability-and-limits).

## Extension points

| Point                      | Examples                                          | Runs in                                    | Best label   | Bound as                           |
| -------------------------- | ------------------------------------------------- | ------------------------------------------ | ------------ | ---------------------------------- |
| Report importer            | Vitest JSON, pytest, cargo-nextest                | WASM, no host capabilities                 | Reported     | producer name and digest           |
| Static fact provider       | tree-sitter symbol diff, OpenAPI or schema diff   | WASM over captured bytes                   | Structural   | producer digest                    |
| Scenario kind (driver)     | HTTP service, CLI argv to exit/output, migration  | Sandbox container after exact-plan consent | None (input) | driver digest in the preview       |
| Observer                   | Fake HTTP upstream, SMTP sink, DB dump, file diff | Separate container in the run topology     | Observed     | observer digest                    |
| Comparison rules           | Masks, unordered arrays, numeric tolerance        | Declarative data                           | None         | rules digest; masks stay visible   |
| Suggester                  | Boundary sampling, model proposals                | Any tier                                   | Inferred     | never evidence                     |
| Commands, views, exporters | GitHub sync, HTML report, extra TUI tab           | `after-<name>` on `PATH`, reading `--json` | None         | writes only through `after import` |

`internal/evidence` would replace the closed `Producer` enum (`importer`, `runner`) with a producer class plus plugin name and digest. That is a schema version change. Scenarios and receipts already bind driver, observer and rules digests, and pins bind their scenario.

## Mechanisms

Go's [`plugin`](https://pkg.go.dev/plugin) package requires cgo and an exact toolchain and dependency match, does not support Windows, and cannot unload. It is unsuitable for community distribution. Integrations must run outside the host process, which suits the trust model. Each tier matches one level of authority:

1. **Data.** Scenario definitions, comparison rules, image digests and build/start argv. Most language variety is which image and which commands: configuration shown in the consent preview, not code. A definition must come from outside the compared pair; one inside the repository that differs between base and candidate is a changed oracle.
2. **Sandboxed code** for importers, fact providers and rule functions: either WASM through [wazero](https://wazero.io) (pure Go, no cgo) or an embedded hermetic interpreter such as [Starlark](https://github.com/google/starlark-go). Plugins get no clock, filesystem or network unless granted and run under memory and time limits. WASM accepts any language that compiles to it; Starlark needs no build step. A bake-off chooses one (see [next decisions](#next-decisions)). Import and inspection still execute nothing with host authority.
3. **Containers** for drivers and observers, through the existing [sandbox](SANDBOX.md) and exact-plan consent. The trusted observer stays outside the app's PID, filesystem and scratch namespaces.
4. **`after-<name>` executables**, discovered git-style, for workflows, exporters and integrations. Any language, full user permissions, no API approval. They read `--json` output and write only through `after import`, so they cannot forge labels.

## Distribution

Keep Pi's openness:

- Discover a file or directory dropped in a conventional location.
- Bundle several resource kinds in one package.
- Install from git, a local path or a registry, with pinned refs.
- Separate user and project scope; enable or disable each resource.
- Override a built-in by registering the same name.
- Implement built-ins against the public interface.

Private integrations need no account or registry; a local path or private git repository is enough. An illustrative package, not a committed format:

```text
after-vitest/
├── after-package.toml     # name, version, interface version, resources
├── importers/vitest.wasm  # reported cards from Vitest JSON
└── rules/vitest.toml      # declarative comparison rules
```

Change Pi's trust model:

- Trust content digests recorded in a lockfile, as Terraform's [dependency lock file](https://developer.hashicorp.com/terraform/language/files/dependency-lock) does, rather than a directory.
- Treat a repository's plugin declarations as proposals: list them, never load them automatically. Plugin code shipped in a repository is project code.
- Fetch or update plugins only as a separate explicit network action, like [provisioning the sandbox image](SANDBOX.md#provision-and-prove).
- Keep the raw diff and unknown-inventory views core-owned; plugin views are additions.

## Sequence

The POC contract rules out a plugin framework without a demonstrated requirement. Build concrete cases first, then extract the interface:

1. **AFTER-48: native JUnit XML import (implemented).** The bounded [dialect](JUNIT-REPORTS.md) is tested against captured pytest, Vitest and Maven Surefire output. Other producers may emit compatible XML but are not certified.
2. **AFTER-49: declarative `http-service` scenarios.** The payment experiment becomes one definition; a second service in another language proves the runner is not Go-specific.
3. **AFTER-50: declarative `command` scenarios.** Exit status, stdout and stderr cover CLIs, generators and library harnesses.

These are operator-selected follow-up tasks, not POC queue items. Each records its extension seams in task notes: what needed code rather than data, and what differed from the existing instance.

## Next decisions

After AFTER-48–50, decide in order. Stop at any step whose answer is "not needed."

1. **Write the contract as versioned data schemas.** Cover importer output (cards), scenario definitions, observation artifacts and rules, independent of how plugins load or run. Built-ins conform internally first. The schemas are the plugin API; the runtime is a detail.
2. **Decide whether code plugins are needed.** Count real integration requests that declarative definitions, JUnit import and `after-<name>` commands cannot express, from the AFTER-18 study and use on real repositories. If there are almost none, stop here.
3. **Run one bake-off.** Implement the same importer, such as Vitest JSON, in Starlark and as WASM through [Extism](https://extism.org), whose PDKs let authors write TypeScript. Compare authoring steps, debugging, sandbox guarantees, speed on an 8 MiB report and binary size. Ship one runtime, not both.
4. **Port the built-ins.** Move the Go test importer and payment runner onto the chosen interface as first-party plugins. If they cannot be expressed, the interface is insufficient.
5. **Open distribution** on the `gh extension install` model, with the digest lockfile described above.

Revisit the host language only if most requests are for custom TUI views. That is where a TypeScript host clearly wins; AFTER should answer it with structured cards that the core renders, not plugin-drawn UI.

## Open problems

Loading plugins is the easy part.

- **Offline dependencies.** The sandbox has no network. `npm ci`, `pip install` and `cargo build` download, and some run install scripts. Each ecosystem needs a consented preparation step that produces a digest-pinned dependency layer. Most per-language work lives here.
- **Static analysis that executes code.** rust-analyzer runs build scripts and proc macros, and Gradle or Maven project import runs build scripts. An analyzer that wraps a toolchain is execution and belongs in the sandbox. Only pure parsers, such as tree-sitter compiled to WASM, can run during inspection.
- **Observer trust.** A plugin observer becomes something the reviewer has to trust. Cards must read "observed by NAME@DIGEST," and changing it must reopen pins.
- **Rules that hide changes.** A mask can hide a real difference. Masked counts stay visible, and a rule change requires a new approved run.

# AFTER handoff

This page gives developers and agents the current product direction, safety rules, and implementation entry points. The [backlog](../backlog/tasks) is the source of truth for task status.

## Product and implementation

AFTER is an independent local Go CLI/TUI for reviewing concrete changes and their evidence. It has no required editor, model, account, or GitHub dependency. Local Git review comes first; GitHub PR sync and browser comparison remain deferred. The presentation is a concept simulation. Read the [proposal](README.md), [behavior and evidence design](behavior-and-evidence.md), and [research](research.md) for product rationale.

The working POC includes native capture, private versioned storage, stock `go test -json` import, raw diff/inventory inspection, the [payment fixture](../internal/paymentfixture/README.md), offline runner, exact comparison, persistent pins, and a terminal evidence browser. See [CLI](CLI.md), [TUI](TUI.md), [schema](SCHEMA.md), and the [implementation contract](IMPLEMENTATION.md) for current behavior.

Start implementation from [AGENTS.md](../AGENTS.md), [IMPLEMENTATION.md](IMPLEMENTATION.md), and the dependency-ready task in the [Backlog](../backlog/tasks). Work only on ready M1/M2 tasks. M3 tasks AFTER-18–20 are gated; AFTER-18 requires explicit human-study authorization. Do not select follow-up work automatically.

## Safety rules

- Opening, importing, inspecting, comparing, and pinning do not execute project code. A build is execution too.
- `after run` supports only the frozen synthetic payment experiment. Each run needs consent for its exact preview; there is no host-execution fallback. Provision Docker and its pinned image separately; see [SANDBOX.md](SANDBOX.md).
- An imported test report is reported evidence, not an observation or proof that tests ran on a captured snapshot. A changed snapshot or test oracle does not establish observed behavior.
- Evidence applicability, execution outcome, comparison result, and human acceptance are separate. Missing, stale, incomplete, unstable, or incomparable evidence stays visible.
- The ordinary diff and unknown/unsupported inventory remain usable when richer evidence is missing or fails.
- Synthetic receipts establish only finite observations. The study kit is rehearsed by its authors; no human usability or market-value result is claimed.

## First commands

```sh
mise trust
mise install
mise exec -- task init
mise exec -- task build
./bin/after --help
```

The build compiles AFTER; capture reads the selected checkout without building or running its code. This abbreviated output is from `./bin/after capture --project DIR` against a throwaway Git repository; IDs are shortened and the temporary path omitted:

```text
Captured candidate e7070067 (working tree) against base 8e750e49 (commit)
  Base         8e750e49 · 2 paths · complete · 0 excluded · 0 unsupported
  Candidate    e7070067 · 2 paths · complete · 0 excluded · 0 unsupported
  Limit        two matching reads; not an atomic filesystem snapshot
```

Use `mise exec -- task check:go` for build, race tests, vet, and Go formatting; `task test` also checks local links and backlog integrity. Before a commit, stage only intended files and run `mise exec -- task check:staged`. Presentation changes need `task docs:check`; `task docs:render` also updates the PDF and preview.

## Scope boundary

The runner exercises sequential synthetic same-key payment requests at twelve hours and thirty seconds. The protected observer records provider requests independently of app output. It is not a general application adapter, production billing test, universal correctness claim, or usability result. AFTER-18 requires recruited consenting engineers, a facilitator, and a second scorer; see the [evaluation kit](EVALUATION.md).

Do not commit private `.after/` data, credentials, participant data, or local paths. Never weaken execution isolation to work around a missing Docker daemon or unsupported project.

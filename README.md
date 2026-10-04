# AFTER

**See what the code will do differently.** A local-first, example-first review tool: inspect concrete before/after behavior, pin an expectation, and reopen it when its evidence changes.

**Status:** working native Go CLI/TUI for local Git capture, Go report import, raw inspection, frozen offline payment experiments, exact comparison and pin/edit/reopen/rerun. This is a bounded proof of concept, not a general application runner or validated human review tool. The presentation is a simulation.

The [versioned evaluation kit](docs/EVALUATION.md) provides matched review conditions, verified synthetic answer keys and scoring sheets. It is an author-rehearsed kit, not human validation; recruitment and scheduling remain gated.

Start with the [repeatable demo and native packages](docs/DEMO.md): `mise exec -- task demo:inspect` needs no Docker; `task demo` requires separate image provisioning and exact consent for each real run.

- [Agent handoff](docs/HANDOFF.md)
- [Implementation contract and POC gates](docs/IMPLEMENTATION.md)
- [Version-1 record schema and trust limits](docs/SCHEMA.md)
- [Product proposal](docs/README.md) · [behavior/evidence design](docs/behavior-and-evidence.md)
- [Interactive concept](docs/presentation.html) · [PDF](docs/presentation.pdf)
- [Backlog tasks](backlog/tasks)

## Development setup

Install [mise](https://mise.jdx.dev/), then:

```sh
mise trust
mise install
mise exec -- task init
mise exec -- task build
./bin/after --help
./bin/after --version
mise exec -- task test
```

The pinned tools are Go, Bun, Prettier, Gitleaks, Lefthook, Task, Worktrunk and Backlog.md. Bun and Playwright support documentation checks only; the AFTER binary does not need them at runtime. Help/version work outside a repository without Docker, network access or model credentials. No arguments prints help. Unknown commands, flags and extra arguments exit 2; help/version exit 0; output failures exit 1. Inspection never executes project code. Only an explicitly authorized `run` executes the frozen payment experiment; interactive runs require exact-plan confirmation.

The tooling is adapted from the sibling `project` template: mise, Task targets, staged formatting/secret hooks, EditorConfig and blocking Worktrunk setup. Web/server tasks, Docker Compose services, Hum, Varlock and copied environment state were deliberately omitted: this project currently has no development service stack or secrets to configure. Fixture isolation uses the separately provisioned, pinned offline Docker boundary, not a development database stack.

## Checks

```sh
mise exec -- task check:staged          # after staging intended files
mise exec -- task build                 # bin/after native executable
mise exec -- task test:go               # Go tests with race detection
mise exec -- task check:go              # build, race tests, vet and gofmt check
mise exec -- task format:go             # format Go sources
mise exec -- task test                  # Go tests, local links and backlog checks
mise exec -- task check                 # all Go/tooling/documentation checks
mise exec -- task docs:browser:install  # once, when Chromium is absent
mise exec -- task docs:check            # real browser checks; ignored artifacts/
mise exec -- task docs:render           # also regenerate PDF and preview
```

CI is configured for Go build/race tests/vet/format, CLI/demo smoke checks and native packages on macOS/Linux amd64/arm64, plus formatting, backlog/link checks, secret scanning and Chromium presentation checks. The Linux Docker job runs the real demo and adversarial POC gate with the exact pinned image. Race tests require a C compiler; the shipped binary needs no Go or JavaScript runtime. Green foundation checks alone do not prove execution, and finite synthetic receipts do not establish universal behavior or human benefit.

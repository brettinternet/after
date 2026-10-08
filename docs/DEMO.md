# Build, package and reproduce

AFTER is a local CLI/TUI proof of concept with a payment-specific offline runner, not a general application runner. This page covers native builds, the no-Docker inspection demo, and the separately authorized real fixture loop. The presentation is a simulation; no human benefit study has run.

## Build and package

From a fresh checkout:

```sh
mise trust
mise install
mise exec -- task init
mise exec -- task build
./bin/after --help
mise exec -- task package
```

Go and modules are pinned in `mise.toml`, `go.mod` and `go.sum`. Tool/module setup may use the network; it is separate from offline project execution. `task package` uses the local Go toolchain, read-only module resolution, `CGO_ENABLED=0` and trimmed paths. It creates a unique ignored `dist/0.1.0-dev.<commit>[.dirty]-<suffix>/` directory with four executables, `SHA256SUMS` and `BUILD.txt`; it does not sign, archive, install, upload or publish a release.

| Target                   | Support                                                           |
| ------------------------ | ----------------------------------------------------------------- |
| macOS/Linux, amd64/arm64 | Inspection; native CI is configured for all four targets.         |
| Execution                | Linux Docker daemon with cgroup v2; no Windows execution support. |

Capture and demo setup require trusted `/usr/bin/git`; the TUI requires a terminal. The executable has no runtime Go, Bun or C-library requirement. Race tests need a C compiler. Cross-compilation is not proof of native execution.

Verify a package before installing a matching binary:

```sh
shasum -a 256 -c SHA256SUMS
# Linux: sha256sum -c SHA256SUMS
```

Checksums detect corruption, not publisher authenticity. Unsigned development binaries may need local execution approval.

## Inspect without Docker

```sh
mise exec -- task demo:inspect
# Keep the private workspace and JSON steps for inspection:
mise exec -- task demo:inspect -- --keep
```

The demo creates its own temporary Git repository from four allowlisted synthetic payment files, commits a base, changes retention, captures the candidate, inspects the diff and imports a checked-in Go report. It does not run the imported tests, the payment app, or Docker; the report is labeled `reported / unknown / not_run`. It copies no caller project, ignored files, credentials or production services. JSON steps live outside the fixture repository. Use captured IDs with `after inspect` or `after review BASE CANDIDATE --project WORKSPACE/payment`.

There is no observed receipt to pin in this route. In the TUI, `1`/`2`/`3` and Tab/Shift+Tab switch Overview, Changes and Diff. After an edit, `c` captures and `u` explicitly selects the new candidate while retaining the base. `p`, `u` and eligible `a` actions identify their target and record a reason; Esc cancels. `a` accepts a pin's current complete result, never a snapshot.

## Run the synthetic payment experiment

Execution is a separate operator action. It requires an explicitly configured absolute Docker CLI, local Unix socket, Linux daemon with cgroup v2/resource controls, and the pinned image. On macOS, prepare a Linux VM such as Colima with at least 2 GiB RAM. See [sandbox setup and recovery](SANDBOX.md).

Pulling the image is an explicit network action; the demo never pulls it:

```sh
docker pull docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190
```

Example for Linux:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
mise exec -- task demo -- --keep
```

On macOS, use the absolute CLI path and local socket from `docker context inspect colima`; the sandbox does not inherit Docker contexts or credentials. The demo shows the exact plan and requires its digest. EOF or a different digest stops before execution. Builds and processes run offline in isolation and may take several minutes.

The actual workflow is:

1. The original 24-hour/5-minute pair has identical HTTP responses, with provider requests **1 → 2** at 12 hours and **1 → 1** at 30 seconds.
2. AFTER pins a finite expectation with a synthetic-demo reason.
3. The demo restores 24-hour retention with a comment edit and captures the new candidate.
4. Selecting it reopens the pin as stale with no current result.
5. A second exact-plan consent reruns the original base against the new candidate. Both cases record **1 → 1**; the receipt is attached but not accepted.
6. Human acceptance remains a separate [review action](REVIEW.md). The expectation is not a general assertion.

`task demo:proof` explicitly authorizes only generated synthetic plans, without prompts; no environment variable bypasses consent. This is a CI proof mode, not permission for arbitrary project commands. `task test:poc` runs the broader adversarial Docker/PTY gate and mutation checks. Neither uses a host fallback or production service.

## Ownership and recovery

A successful demo deletes only its own temporary directory after checking the canonical temp directory, inode, random ownership token and prefix. It does not follow symlinks. Runner resources use exact random names and ownership labels. `--keep`, consent denial and operational errors retain the printed workspace and JSON steps. Inspect the exact retained directory and `.owner` before moving it to Trash; never delete by glob, label alone or guessed name. Keep `.after/` private and out of Git.

| Failure                                 | Safe next step                                                                                                                                                                                             |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Missing tools/binary                    | Run `task init`, then `task build`; demo setup requires `/usr/bin/git`.                                                                                                                                    |
| Docker/image/isolation                  | Inspect incomplete run JSON and sandbox diagnostics; configure both Docker variables, provision the exact image and verify daemon/cgroup controls. Start a new demo. Never use host execution.             |
| Store lock, permissions, schema or hash | Stop competing writers, check ownership/private permissions and preserve the store. Do not remove a live lock, edit hashes, or downgrade a newer schema. Start a fresh workspace for this disposable demo. |
| Interrupted run or cleanup failure      | Stop execution. Follow [sandbox recovery](SANDBOX.md#residual-limits-and-failure-recovery), matching exact names, labels, time and inputs. Never globally prune.                                           |
| Package/checksum failure                | Retain the failed directory; retry into a new directory. Do not distribute partial/mismatched files.                                                                                                       |

Normal SIGINT/SIGTERM cancels the runner context. Forced process death, power loss or daemon failure may leave resources; a killed client does not prove daemon cleanup. The runner observes only sequential synthetic HTTP responses and fake-provider requests in a finite window. Arbitrary adapters, remote PR sync, browser behavior and human validation are deferred.

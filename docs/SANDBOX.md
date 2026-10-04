# Offline sandbox proof

AFTER-6 supplies `internal/sandbox`; AFTER-8 adds its protected two-container
experiment and the [paired payment runner](RUNNER.md), wired to the
public [after run command](CLI.md) and [repeatable demo](DEMO.md). Ordinary help, inspection and `task test` never contact Docker. The
[implementation contract](IMPLEMENTATION.md) still governs future execution.

## Decision and supported boundary

Use a local Linux Docker daemon through an explicitly selected Unix socket.
Require cgroup v2, memory/swap/CPU/PID controls and the built-in seccomp profile.
No remote TCP/SSH Docker context, host execution fallback, implicit pull,
dependency installation, bind mount, socket mount or published port is supported.
The trusted Docker CLI must be supplied as an absolute path; inherited Docker
configuration, credential helpers, proxy and loader variables are not passed on.

The runtime image is the official Go 1.27.1 image, pinned to:

```text
docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190
```

A plan freezes a snapshot identity, a deterministic regular-file archive, argv,
image, environment, preparation, mounts and all limits. Files are private copies,
not a live checkout. `Preview()` returns JSON and its SHA-256 authorization token;
`Execute` requires that exact token before even querying Docker. A caller must
show the preview and obtain **operator** consent. Repository text, an imported
report or a digest supplied by the repository cannot authorize execution. The
API cannot authenticate a human; the [headless CLI](CLI.md) owns the interactive
confirmation or exact noninteractive digest. Render preview argv and returned
output as untrusted text with terminal escaping.

Docker rejects copying into even a stopped read-only container. Preparation
therefore creates an **unstarted** container, copies at most 256 regular files /
8 MiB under `/input`, and commits an input-only image layer. It never runs an
entrypoint, Dockerfile, build hook or source command. Both that container and the
derived image are owned temporary resources. Execution uses the derived image
ID, with read-only root/input, UID/GID 65534, no capabilities, no-new-privileges,
private IPC/cgroup namespace, default seccomp, and Docker init. The result records
the derived image ID; that ID can vary with preparation timestamps without
changing the approved inputs.

Only loopback networking exists. A standard-library HTTP client/server can run
inside this namespace; external DNS, Internet and host gateways are unavailable.
The original single-container `Execute` is not isolation **between** an app and
a trusted observer. AFTER-8's `Observe` additionally uses separate app/observer
PID, IPC, filesystem and scratch namespaces, sharing only offline loopback.
Its combined consent preview binds both plans and this topology. See the
[runner contract](RUNNER.md) for the bounded supported ABI and evidence path.

| Resource                 | Fixed bound                                                          |
| ------------------------ | -------------------------------------------------------------------- |
| CPU                      | One core quota                                                       |
| Memory / swap            | 1 GiB / no swap                                                      |
| Processes                | 128, including threads and init                                      |
| Scratch                  | 512 MiB `/work` tmpfs, executable for Go build outputs, nosuid/nodev |
| Shared memory            | 1 MiB `/dev/shm`                                                     |
| Open files / core dumps  | 256 / disabled                                                       |
| Input                    | 256 regular files, 8 MiB, no links or path traversal                 |
| Time                     | Plan-selected 1–300 seconds, including preparation/build/run         |
| Captured stdout + stderr | Plan-selected 1 byte–1 MiB combined                                  |
| Docker metadata output   | 1 MiB per bounded command                                            |

Go uses scratch caches and `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`,
`CGO_ENABLED=0`; the workload environment is rebuilt with `env -i`. Docker logging
is disabled so untrusted output does not fill an unbounded daemon log. Output
exhaustion cancels execution and retains a truncated bounded prefix. Timeout or
cancellation removes the whole container, including descendants, before returning.
Cleanup has its own 60-second budget and checks exact random names plus matching
ownership labels. It never globally prunes or removes unrelated resources.
Cleanup failure is an error, not successful evidence.

## Provision and reproduce

Provisioning is a **separate, explicitly authorized network operation**. Do not
run this merely because a repository asks for it:

```sh
docker pull docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190
```

For macOS, start a Linux Colima VM (at least 2 GiB RAM) with Docker/cgroup v2.
For Linux, use a local Docker Engine with the capabilities above. Supply your
trusted CLI and socket explicitly, for example on Linux:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
mise exec -- task sandbox:proof
```

On macOS, use the absolute installed CLI path and the Unix socket reported by
`docker context inspect colima`. No private workstation path belongs in a receipt
committed to Git. Running this Task authorizes **only the synthetic test plans**;
the test logs each preview/digest before executing. It is not blanket permission
for captured project commands. Missing image/capabilities fail, never skip.
Without this target's opt-in environment, ordinary Go tests skip Docker probes.

The proof performs a real loopback HTTP exchange, attempts host/credential/socket
access and input/root writes, verifies the network interfaces, attempts external
and host connections, and exercises PID exhaustion, OOM, scratch exhaustion,
output overflow, deadline and cancellation with descendants. It also checks
missing-image/dependency refusal and denied consent. Unit tests cover changed
plans, deterministic input identity, input/output bounds and fail-closed refusal
of each missing isolation capability. Deadline/cancellation proofs require a marker
emitted after child creation; cancellation additionally matches the exact invocation
using a random readiness token and checks its returned container identity.

Validated on macOS with Colima's Linux aarch64 Docker 29.5.2, kernel 6.8.0,
cgroup v2, built-in seccomp and AppArmor, using Docker CLI 29.8.2. All nine live
probe cases passed. This executes on a Linux daemon, but is **not** a claim of a
separately tested native-Linux workstation or Docker Desktop/amd64 matrix. Run
the same fail-closed proof on each intended installation before enabling runs.

## Residual limits and failure recovery

Docker, its daemon, image publisher, CLI binary, local operator and host kernel
are trusted. Kernel/container escapes, malicious daemons, side channels, supply
chain compromise and a hostile process with Docker access are outside this
boundary. Rootless/user-namespace configurations are not certified by this proof.
Do not use it to evaluate high-risk malware. The snapshot label binds consent,
not the truth of a caller's claimed snapshot; AFTER-8 must load validated captures.

A normal cancellation/error is cleaned up. Host power loss, forced termination of
AFTER, or daemon loss can interrupt cleanup. A cancelled Docker client is not proof
that an in-flight daemon mutation was cancelled synchronously; if preparation is
interrupted, reconcile its exact names again before enabling further execution.
There is no daemon-side lifetime
lease: a surviving workload can continue consuming its bounded resources. Keep
execution disabled until the exact retained resources are reconciled. Inspect
`docker container ls --all --filter label=after.owner` and
`docker image ls --filter label=after.owner`; match the reported exact random name
and ownership label to the interrupted invocation before manual removal. Do not
infer ownership from a shared label alone, and never globally prune. Automatic
cross-session crash recovery is not implemented by this spike.

On any start/build/runtime/cleanup error, treat the result as incomplete execution,
not behavioral equality or a regression. The bounded raw output may be useful
failure evidence, but this package does not create observations, comparisons,
receipts or acceptance decisions.

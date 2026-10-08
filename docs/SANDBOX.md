# Offline sandbox proof

`internal/sandbox` runs explicitly approved commands in disposable local Docker containers. The [paired payment runner](RUNNER.md) adds a separate observer container; ordinary help, inspection and `task test` do not contact Docker. This sandbox is not a general execution platform.

## Supported boundary

Use a local Linux Docker daemon through an explicitly selected Unix socket. It must provide cgroup v2, memory/swap/CPU/PID controls and Docker's built-in seccomp profile. Remote TCP/SSH contexts, host fallback, implicit image pulls, dependency installation, bind mounts, socket mounts and published ports are unsupported. Supply an absolute trusted Docker CLI path. The run does not inherit ambient Docker configuration, credential helpers, proxies or loader variables.

The runtime image is pinned:

```text
docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190
```

A plan freezes snapshot identity, deterministic regular-file archive, argv, image, environment, preparation, mounts and limits. `Preview()` returns JSON plus its SHA-256 consent token; `Execute` checks the exact token before even querying Docker. The caller must show the preview and obtain operator consent. Repository text, imported reports and repository-supplied digests do not grant permission. The API cannot authenticate a human; the CLI owns interactive or exact-digest confirmation. Escape argv and output as untrusted text.

Docker cannot copy into a stopped read-only container. Preparation therefore creates an **unstarted** container, copies at most 256 regular files / 8 MiB into `/input`, then commits an input-only image layer. It runs no entrypoint, Dockerfile, build hook or source command. The container and derived image are owned temporary resources. Execution uses the derived image ID with read-only root/input, UID/GID 65534, no capabilities, no-new-privileges, private IPC/cgroup namespaces, default seccomp and Docker init. The derived image ID can vary with preparation timestamps without changing approved inputs.

Only loopback networking is available; external DNS, Internet and host gateways are unavailable. The single-container `Execute` primitive does not isolate an app from a trusted observer. The paired runner additionally separates app/observer PID, IPC, filesystem and scratch namespaces, sharing only offline loopback; the combined preview binds this topology.

## Fixed resource limits

| Resource                 | Limit                                                                       |
| ------------------------ | --------------------------------------------------------------------------- |
| CPU                      | One core quota                                                              |
| Memory / swap            | 1 GiB / no swap                                                             |
| Processes                | 128, including threads and init                                             |
| Scratch                  | 512 MiB `/work` tmpfs, executable, `nosuid`/`nodev`                         |
| Shared memory            | 1 MiB `/dev/shm`                                                            |
| Open files / core dumps  | 256 / disabled                                                              |
| Input                    | 256 regular files, 8 MiB; no links or traversal                             |
| Time                     | Plan-selected 1–300 seconds, including preparation/build/run                |
| Combined stdout + stderr | Plan-selected 1 byte–1 MiB                                                  |
| Docker metadata output   | 1 MiB per bounded command                                                   |
| Cleanup                  | Separate 60-second budget; exact random names and matching ownership labels |

Go uses scratch caches, `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`, `CGO_ENABLED=0`; workload environment is rebuilt with `env -i`. Docker logging is disabled. Output exhaustion cancels execution and retains a bounded truncated prefix. Timeout or cancellation removes the whole container and descendants before returning. Cleanup failure is an error, not evidence; unrelated resources are never globally pruned.

## Provision and prove

Provisioning is a separate authorized network action. Do not pull merely because a repository asks:

```sh
docker pull docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190
```

On macOS, prepare a Linux Colima VM with at least 2 GiB RAM and Docker/cgroup v2. On Linux, use a local Docker Engine with the required capabilities. Example for Linux:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
mise exec -- task sandbox:proof
```

On macOS, set the absolute CLI path and Unix socket reported by `docker context inspect colima`. The opt-in target authorizes synthetic test plans only and logs each plan/digest. It is not blanket permission to run captured projects. It never pulls. Missing image or capabilities fail; they do not skip. Without the target's opt-in setting, normal Go tests skip Docker probes.

The proof exercises loopback HTTP; host, credential and socket access; writes to input/root; external and host connections; PID, memory, scratch and output exhaustion; deadline and cancellation with descendants; missing-image/dependency refusal and denied consent. Unit tests cover changed plans, deterministic input identity, bounds and fail-closed missing isolation capabilities. Deadline/cancellation tests require a readiness marker after child creation; cancellation also matches a random invocation token and exact container identity.

Previously validated on macOS with Colima Linux aarch64 Docker 29.5.2, kernel 6.8.0, cgroup v2, built-in seccomp and AppArmor, using CLI 29.8.2; all nine live probe cases passed. This is not a native-Linux workstation or Docker Desktop/amd64 certification. Run the fail-closed proof on each intended installation.

## Residual limits and failure recovery

Docker, its daemon, image publisher, CLI, local operator and host kernel are trusted. Kernel/container escapes, malicious daemons, side channels, supply-chain compromise and a hostile process with Docker access are outside this boundary. Rootless/user-namespace configurations are not certified. Do not use this for high-risk malware. A snapshot label binds consent but does not prove a caller's claim; the runner must load validated captures.

Normal cancellation/errors clean up resources. Power loss, forced termination or daemon loss can interrupt cleanup. Cancelling a Docker client does not prove an in-flight daemon mutation stopped; an interrupted preparation may leave resources. There is no daemon-side lifetime lease, so a surviving workload can keep consuming its bounded resources. Disable further execution until exact leftovers are reconciled.

Inspect only the interrupted invocation's resources:

```sh
docker container ls --all --filter label=after.owner
docker image ls --filter label=after.owner
```

Match exact reported random names and ownership labels to that invocation before manual removal. A shared label alone is not ownership proof. Never globally prune. Automatic cross-session crash recovery is not implemented. Any start/build/runtime/cleanup error is incomplete execution, not equality or regression; bounded raw output may still help diagnosis. This package does not create observations, comparisons, receipts or acceptance decisions.

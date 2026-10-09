# Offline sandbox proof

`internal/sandbox` runs explicitly approved commands in disposable local Docker containers. The HTTP-service runner uses a paired observer; v1 command scenarios use one fresh container and Docker's actual exit/stdout/stderr boundary. Ordinary help, inspection and `task test` do not contact Docker. This sandbox is not a general execution platform.

## Supported boundary

Use a local Linux Docker daemon through an explicitly selected Unix socket. It must provide cgroup v2, memory/swap/CPU/PID controls and Docker's built-in seccomp profile. Remote TCP/SSH contexts, host fallback, implicit image pulls, dependency installation, bind mounts, socket mounts and published ports are unsupported. Supply an absolute trusted Docker CLI path. The run does not inherit ambient Docker configuration, credential helpers, proxies or loader variables.

The trusted compiler/observer runtime image is pinned:

```text
docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190
```

An `http-service` definition separately selects a digest-pinned service image and explicit Linux platform. Current proof provisioning is:

| Platform      | Trusted Go compiler/observer image                                                                 | Python service image                                                                               |
| ------------- | -------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `linux/arm64` | `docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190` | `docker.io/library/python@sha256:cf97c3b79da1c706532c7042f851654d97d3b88c8ca6520acda9cf4479ea9b14` |
| `linux/amd64` | Not provisioned or observed on an amd64 proof host                                                 | Not provisioned; the arm64 Python manifest above is not an amd64 image                             |

These are provisioning observations, not a claim that an image index lacks other platforms. The Python fixture currently pins the arm64 manifest. Before consent or Docker access, the Python proof rejects a different native test-process architecture with a provisioning fix. After exact consent, Docker preflight independently checks Linux daemon isolation capabilities and the image's approved platform. This does not certify the daemon's CPU architecture; run the proof process on a matching native Docker host, not across architectures.

To unblock amd64, supply a native amd64 machine with a local Docker Unix socket, separately authorize and provision matching digest-pinned Go and Python images, then update `internal/runner/testdata/python-service/http-service.json` platform and image together and record the provisioned digest here. Do not merely change the platform on the arm64 manifest. Run `task http-service:proof`, `task command:proof` and `task test:poc` on that host and record exact commands and host details in AFTER-55. Until then, amd64 execution remains unverified; an early provisioning error is not a passing execution proof.

Provision every required image as a separate authorized network action. AFTER uses `--pull=never`; it does not build images or download service dependencies. The compiler, observer and service images must all support the frozen `linux/amd64` or `linux/arm64` platform; no emulation or automatic provisioning is added.

A plan freezes snapshot identity, deterministic regular-file archive, argv, image/platform, environment, optional base64 stdin digest, preparation, mounts and limits. `Preview()` returns JSON plus its SHA-256 consent token; `Execute` checks the exact token before even querying Docker. The caller must show the preview and obtain operator consent. Repository text, imported reports and repository-supplied digests do not grant permission. The API cannot authenticate a human; the CLI owns interactive or exact-digest confirmation. Escape argv and output as untrusted text.

Docker cannot copy into a stopped read-only container. Preparation therefore creates an **unstarted** container, copies bounded regular files into `/input`, then commits an input-only image layer. It runs no entrypoint, Dockerfile, build hook or project source command. The container and derived image are owned temporary resources. Execution uses the derived image ID with read-only root/input, UID/GID 65534, no capabilities, no-new-privileges, private IPC/cgroup namespaces, default seccomp and Docker init. The derived image ID can vary with preparation timestamps without changing approved inputs.

After exact consent, the runner separately compiles only its embedded trusted HTTP or command launcher in the pinned Go image and streams the static executable from compiler stdout before container exit. Binary stdout has a separate 8 MiB cap; compiler diagnostics are stderr, bounded by the approved output limit. Preparation gets its own declared time budget (90 seconds in shipped definitions), distinct from per-sample runtime. It must finish successfully and clean up before any service sample starts.

The target plan overlays only `/input/after/launcher` (`0555`, ≤8 MiB) and `/input/after/service.json` (`0444`, ≤64 KiB; HTTP or command configuration), in addition to read-only captured and explicitly selected additive input files under `/input`. The definition cannot choose generated paths or file modes. The snapshot source budget is unchanged; the overlay adds at most 8 MiB + 64 KiB and is disclosed in the preview. Every app-plan archive includes digests of the actual generated files.

Only loopback networking is available; external DNS, Internet and host gateways are unavailable. Command cases have no observer container; their direct child replaces the trusted helper with `syscall.Exec`, preserving the target's container-boundary exit status. HTTP's paired runner separately isolates app/observer PID, IPC, filesystem and scratch namespaces, sharing only offline loopback; the combined preview binds that topology.

## Fixed resource limits

| Resource                | Limit                                                                                                                                          |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| CPU                     | One core quota                                                                                                                                 |
| Memory / swap           | 1 GiB / no swap                                                                                                                                |
| Processes               | 128, including threads and init                                                                                                                |
| Scratch                 | 512 MiB `/work` tmpfs, executable, `nosuid`/`nodev`                                                                                            |
| Shared memory           | 1 MiB `/dev/shm`                                                                                                                               |
| Open files / core dumps | 256 / disabled                                                                                                                                 |
| Captured project input  | At most 255 regular files / 8 MiB; no links or traversal                                                                                       |
| Trusted runner source   | HTTP/command launcher, observer and preparation source embedded in their own plans                                                             |
| Trusted runtime overlay | One static launcher ≤8 MiB (`0555`) plus one JSON config ≤64 KiB (`0444`)                                                                      |
| Per-container time      | Plan-selected 1–300 seconds for each preparation or service/observer run                                                                       |
| Per-container output    | Plan-selected combined command stdout/stderr 1 byte–1 MiB; other diagnostics share the bounded limit; compiler binary stdout separately ≤8 MiB |
| Docker metadata output  | 1 MiB per bounded command                                                                                                                      |
| Cleanup                 | Separate 60-second budget; exact random names and matching ownership labels                                                                    |

The trusted Go compiler/observer uses scratch caches, `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`, `CGO_ENABLED=0`; service environments are rebuilt from an allowlisted base plus validated definition entries. Docker logging is disabled. Output exhaustion cancels execution and retains only a bounded prefix. Timeout or cancellation removes the whole container and descendants before returning. Cleanup failure is an error, not evidence; unrelated resources are never globally pruned.

## Provision and prove

Provisioning is a separate authorized network action. Do not pull merely because a repository asks. For the provisioned `linux/arm64` proof host, provision the trusted toolchain image and, only for the Python proof, its explicitly selected service image:

```sh
docker pull --platform linux/arm64 docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190
docker pull --platform linux/arm64 docker.io/library/python@sha256:cf97c3b79da1c706532c7042f851654d97d3b88c8ca6520acda9cf4479ea9b14
```

On macOS, prepare a Linux Colima VM with at least 2 GiB RAM and Docker/cgroup v2. On Linux, use a local Docker Engine with the required capabilities. Example for Linux:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
task sandbox:proof
```

On macOS, set the absolute CLI path and Unix socket reported by `docker context inspect colima`. The opt-in target authorizes synthetic test plans only and logs each plan/digest. It is not blanket permission to run captured projects. It never pulls. Missing image or capabilities fail; they do not skip. Without the target's opt-in setting, normal Go tests skip Docker probes.

The sandbox proof exercises loopback HTTP; host, credential and socket access; writes to input/root; external and host connections; PID, memory, scratch and output exhaustion; deadline and cancellation with descendants; missing-image/dependency refusal and denied consent. `task http-service:proof` exercises the separately provisioned Python image through exact-plan consent without pulling it. `task command:proof` exercises the command fixture through capture, exact consent, isolated build/run, comparison, pin and reopen on the provisioned Go image. Unit tests cover changed plans, deterministic input identity, bounds and fail-closed missing isolation capabilities. Deadline/cancellation tests require a readiness marker after child creation; cancellation also matches a random invocation token and exact container identity.

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

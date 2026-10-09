# Exact finite comparisons

`internal/compare.Run(store, receiptID)` compares stored HTTP-service or command observations and writes a versioned comparison plus a bounded `comparison-details-v1` artifact. It never runs code. The CLI's `compare`, `inspect` and `export` commands read stored data; comparison needs a writable store to publish its result.

## What can be compared

Only the shipped `runner.ComparisonRules` HTTP policy or `runner.CommandComparisonRules` command policy is accepted. Exact policy bytes are stored and hashed into the scenario, receipt and authorization plan; command stream modes are part of the frozen definition. A policy or definition change requires a new approved run. Masks, field dropping, arbitrary normalization and candidate-owned rules are unsupported. Unknown or older policies are incomparable, not upgraded; old placeholder-policy receipts need a newly authorized run.

Before comparison, AFTER reconstructs the frozen definition, preparation recipe, templates, limits, repetition count and request ID from the exact consent preview. It verifies complete successful preparation metadata, diagnostics and launcher artifact; rehashes and validates the static ELF's platform; and rebuilds each materialized plan ID as data only. HTTP samples must match both app and observer identities; command samples match the single target plan and retain boundary status plus separate stdout/stderr artifacts. Plan bytes, authorization, scenario/input/driver/observer/rules, toolchain, argv and environment/dependency bindings must match. Environment hashes remain template/recipe-derived; different snapshots need not have identical hashes. This validation does not call Docker or build the project.

Every definition-declared case, side, repetition and channel must have complete, correctly bound sample metadata and artifacts. Missing, duplicate, malformed, redacted, truncated, extra or unsupported channels fail closed. Command streams are raw byte artifacts; incomplete output flags and statuses 125–255 are incomparable. Preparation and observation records require their declared case IDs; optional legacy duration projections, when present, must agree with the frozen case. Required fields must exist and have the right type; explicit empty strings, empty arrays, zero values and JSON `null` remain distinct. Report imports, failed preparation and failed executions cannot compare. Corrupt or unavailable storage returns an error; the independent raw inventory and diff remain available.

## Compared values

| Channel                       | Values compared                                                                                                                                                                                                         |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `responses`                   | Ordered HTTP status and body. Valid JSON bodies compare structurally; other bodies compare as exact text. JSON and text are different representations.                                                                  |
| `provider_calls`              | Ordered timestamp, declared fake-upstream name, destination, method, path, idempotency key and body. Call count is the independently recorded log length. The observer owns this log.                                   |
| `exit_status` (command)       | Exact Docker-inspected container exit status. Statuses 125–255 are incomplete, never comparable outcomes.                                                                                                               |
| `stdout` / `stderr` (command) | Separate Docker-attached raw byte streams. Definition-declared `text` compares exact bytes (including invalid UTF-8); `json` uses the bounded structural rules below. Text difference witnesses encode bytes as base64. |

For HTTP JSON bodies and command streams declared `json`, object keys are traversed in sorted order; array order and Unicode code points are exact. `null` differs from a missing field. Numbers use exact decimal comparison, not floating point: `1`, `1.0` and `1e0` are equal, while adjacent large integers remain distinct. Original numeric tokens remain in witnesses. RFC 6901 pointers escape `~` and `/`; absent values are omitted, not rendered as `null`. Duplicate keys, invalid UTF-8 and unpaired UTF-16 escapes are rejected.

Each witness links both sample metadata and observation blobs. The detail artifact retains all receipt references, paired witnesses and within-side repetition witnesses, including samples from failed comparisons. Readable cards lead back to full IDs and source inventories. Strict decode failures retain the raw artifact with a limitation rather than partial comparison prose.

| Outcome        | Meaning                                                                                 |
| -------------- | --------------------------------------------------------------------------------------- |
| `equal`        | All required samples and channels are complete and equal under the frozen policy.       |
| `different`    | A paired base/candidate witness differs; this is not a regression judgement.            |
| `unstable`     | Repetitions on either side disagree; takes precedence over `different`.                 |
| `incomparable` | Evidence is missing, partial, incompatible, redacted or unsupported; no equality claim. |

The engine recomputes repetition stability under semantic JSON rules, so key-order-only differences are not unstable. It never cherry-picks a repetition. A newly redacted detail artifact makes the persisted comparison incomparable.

Readable command run/compare output and receipt cards summarize each equal case on one line and show changed exit statuses, JSON paths, and text streams from the frozen command definition. Repetition drift is labeled `UNSTABLE`. Text witnesses are decoded from the exact-byte base64 wrapper only when that stream is declared `text`; declared JSON values at a `/base64` path remain ordinary JSON. Text snippets are terminal-sanitized, clipped to 20 display columns per side, and mark invalid UTF-8 or NUL-containing binary data. The shared summary is capped at 24 lines of 76 display columns. CLI output points to `--json` for the complete report; cards retain the complete comparison-details artifact under Artifacts. These display bounds do not change stored witnesses or JSON output.

## Bounds and limits

| Input or work                      |                                                                                                                                                                  Bound |
| ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------: |
| JSON body and stored detail report |                                                                                                                                                             1 MiB each |
| JSON nesting / nodes               |                                                                                                                                               64 levels / 32,768 nodes |
| Number token / exponent            |                                                                                                                                            1,024 bytes / signed 32-bit |
| Witness changes per comparison     |                                                                                                                                                                  2,048 |
| Samples                            | 1–5 repetitions, at most 4 definition cases; HTTP: 16 requests/case, 128 calls and 4,096-byte bodies; command: stdout/stderr combined within the declared output limit |

Decimal comparison does not allocate exponent-sized numbers. Limit failures are explicit, never silent truncation. If detail publication exceeds its budget, no conclusive comparison record is written; the receipt stays inspectable.

The built-in payment instance sends two sequential same-key requests at 12 hours and 30 seconds; the Python standard-library proof is a separate one-case service on its own pinned image. Custom definitions remain finite, explicitly selected inputs—not a universal adapter. Neither proves universal safety, causation or performance. `provider_calls` records only declared fake upstreams and bounded request fields; it is not a general network trace. Hashes bind records but do not authenticate a producer. There is no model, account, network service or API key. Terminal clients must escape all untrusted strings.

## Checks and live proof

`task check:go` covers definition-driven HTTP/command cases, preparation provenance, materialized plan identities, deterministic JSON, exact numbers and Unicode, missing/new channels, failures, malicious rules, repetitions, redaction and artifact/inventory links. `task comparison:fuzz` fuzzes bounded JSON determinism and reflexivity.

The opt-in live proof requires the pinned image and explicit local Docker CLI/socket:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
task comparison:proof
```

It authorizes only synthetic payment runs and checks identical responses, 12-hour counts 1 vs 2, 30-second counts 1 vs 1, two repetitions and 16 artifact-linked witnesses. `task http-service:proof` separately authorizes the dependency-free Python service and requires equal response plus `provider_calls` witnesses on its own image. `task command:proof` checks a changed command case and unchanged control through capture, exact consent, comparison, pin and reopen on the separately provisioned Go image. Both use the same local Docker configuration described in [RUNNER.md](RUNNER.md); neither pulls an image. Unit tests do not run Docker or project code. The existing live validation was macOS with a Colima Linux aarch64 daemon; it does not certify other host/daemon combinations.

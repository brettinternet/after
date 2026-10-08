# Exact finite comparisons

`internal/compare.Run(store, receiptID)` compares stored observations and writes a versioned comparison plus a bounded `comparison-details-v1` artifact. It never runs code. The CLI's `compare`, `inspect` and `export` commands read stored data; comparison needs a writable store to publish its result.

## What can be compared

Only the shipped `runner.ComparisonRules` policy is accepted. Its exact bytes are stored and hashed into the scenario, receipt and authorization plan. A policy change requires a new approved run. Masks, field dropping, arbitrary normalization and candidate-owned rules are unsupported. Unknown or older policies are incomparable, not upgraded; old placeholder-policy receipts need a newly authorized run.

Before comparison, AFTER reconstructs the frozen plan from source, limits, repetition count and request ID. Plan bytes, authorization, scenario/input/driver/observer/rules, toolchain, argv and environment/dependency bindings must match. Environment hashes include the source archive; different snapshots need not have identical hashes. This validation does not call Docker or build the project.

Every expected case, side and repetition must have complete, correctly bound sample metadata and observation artifacts. Missing, duplicate, malformed, redacted, truncated, extra or unsupported channels fail closed. Required fields must exist and have the right type; explicit empty strings, empty arrays, zero values and JSON `null` remain distinct. Report imports and failed executions cannot compare. Corrupt or unavailable storage returns an error; the independent raw inventory and diff remain available.

## Compared values

| Channel      | Values compared                                                                                                                                               |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Responses    | Ordered HTTP status and body. Valid JSON bodies compare structurally; other bodies compare as exact text. JSON and text are different representations.        |
| Provider log | Ordered timestamp, method, fixed observer destination `127.0.0.1:18082`, path, idempotency key and body. Call count is the independently recorded log length. |

JSON object keys are traversed in sorted order; array order and Unicode code points are exact. `null` differs from a missing field. Numbers use exact decimal comparison, not floating point: `1`, `1.0` and `1e0` are equal, while adjacent large integers remain distinct. Original numeric tokens remain in witnesses. RFC 6901 pointers escape `~` and `/`; absent values are omitted, not rendered as `null`. Duplicate keys, invalid UTF-8 and unpaired UTF-16 escapes are rejected.

Each witness links both sample metadata and observation blobs. The detail artifact retains all receipt references, paired witnesses and within-side repetition witnesses, including samples from failed comparisons. Readable cards lead back to full IDs and source inventories. Strict decode failures retain the raw artifact with a limitation rather than partial comparison prose.

| Outcome        | Meaning                                                                                 |
| -------------- | --------------------------------------------------------------------------------------- |
| `equal`        | All required samples and channels are complete and equal under the frozen policy.       |
| `different`    | A paired base/candidate witness differs; this is not a regression judgement.            |
| `unstable`     | Repetitions on either side disagree; takes precedence over `different`.                 |
| `incomparable` | Evidence is missing, partial, incompatible, redacted or unsupported; no equality claim. |

The engine recomputes repetition stability under semantic JSON rules, so key-order-only differences are not unstable. It never cherry-picks a repetition. A newly redacted detail artifact makes the persisted comparison incomparable.

## Bounds and limits

| Input or work                      |                                                                      Bound |
| ---------------------------------- | -------------------------------------------------------------------------: |
| JSON body and stored detail report |                                                                 1 MiB each |
| JSON nesting / nodes               |                                                   64 levels / 32,768 nodes |
| Number token / exponent            |                                                1,024 bytes / signed 32-bit |
| Witness changes per comparison     |                                                                      2,048 |
| Samples                            | 1–5 repetitions, 2 cases, at most 128 provider calls and 4,096-byte bodies |

Decimal comparison does not allocate exponent-sized numbers. Limit failures are explicit, never silent truncation. If detail publication exceeds its budget, no conclusive comparison record is written; the receipt stays inspectable.

This experiment is two sequential same-key requests at 12 hours and 30 seconds. It does not prove universal safety, causation or performance. The provider channel omits query strings, other headers, other destinations and effects outside its observation window; it is not a general network trace. Hashes bind records but do not authenticate a producer. There is no model, account, network service or API key. Terminal clients must escape all untrusted strings.

## Checks and live proof

`task check:go` covers deterministic JSON, exact numbers and Unicode, missing/new channels, failures, malicious rules, repetitions, redaction and artifact/inventory links. `task comparison:fuzz` fuzzes bounded JSON determinism and reflexivity.

The opt-in live proof requires the pinned image and explicit local Docker CLI/socket:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
task comparison:proof
```

It authorizes only synthetic payment runs and checks identical responses, 12-hour counts 1 vs 2, 30-second counts 1 vs 1, two repetitions and 16 artifact-linked witnesses. It uses the same Docker settings as [the runner proof](RUNNER.md). Unit tests do not run Docker or project code. The existing live validation was macOS with a Colima Linux aarch64 daemon; it does not certify other host/daemon combinations.

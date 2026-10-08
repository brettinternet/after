# Evidence records, version 1

`internal/evidence` defines and validates AFTER's local record contracts. [Storage](STORAGE.md), [capture](CAPTURE.md), [Go report import](GO-REPORTS.md), [runner](RUNNER.md), [comparison](COMPARISON.md), and [review](REVIEW.md) own persistence and behavior. The [schema example](schema-example.json) is synthetic; its digests are placeholders, not measurements.

## Encoding and identity

Every record has `schema_version: 1` and an ID of `sha256:` plus 64 lowercase hex digits. Content references use that form. Resolved Git commits are 40- or 64-character lowercase hex. An unborn snapshot has `unborn: true` and an empty `commit` (never in merge-base mode). Merge-base captures retain the resolved base tip, target commit, and merge-base—not branch names. A working-tree snapshot may bind an index snapshot with the same commit/unborn basis.

Storage derives a new record ID from the Go JSON encoding with `id` empty; supplied IDs must match. This is a local encoding contract, not canonical cross-language JSON or a signature. Capture/storage verify referenced bytes and content hashes. Branch names, timestamps, and paths are not content identities.

`Decode[Snapshot|Capture|Scenario|Receipt|Comparison|Pin]` reads at most 4 MiB, accepts one JSON object, rejects unknown fields, unsupported versions (including zero), trailing values, and invalid records, and returns a zero record on failure. It does not rewrite or migrate input. Strings are UTF-8 JSON; times are RFC 3339. Go's JSON decoder handles duplicate keys by replacing/merging earlier values with later values; records are local data, not signed canonical JSON.

Call `Validate` before persisting constructed records. Go structs are mutable; storage must not overwrite immutable receipts. Optional fields use `omitempty`; required enums have no implicit success value. A nil collection means no entries, not evidence of coverage.

A successful immutable `Capture` event stores `captured_at`, mode, base/candidate snapshot IDs, optional index ID, and exact selected untracked paths (each at most 4096 bytes). Recapturing unchanged content preserves snapshot IDs but creates a new event. Capture time is not inferred from file mtimes; snapshots without a matching event have no recorded time. The event ID does not change snapshot identity.

## Record fields and bounds

| Record       | Contract                                                                                                                                                                                                                                                                      |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Snapshot`   | Source mode (`commit`, `working_tree`, `index`, `merge_base`), resolved commit, merge-base identities when applicable, relative path/content digest/Git-mode inventory, excluded and unsupported entries with reasons, completeness, ordinary diff digest, and limits.        |
| `Capture`    | Successful capture time/mode, base/candidate and optional index snapshot IDs, selected untracked paths. A new immutable event per successful capture.                                                                                                                         |
| `Scenario`   | Frozen input/setup/actions artifact, driver, observer and rules digests, supported boundary, author, and explicit limits. Expectations are separate.                                                                                                                          |
| `Receipt`    | Independent state axes, snapshot pair, ordered start/finish times, completeness, bounded artifact references, and limits. Runner receipts also bind scenario/input/driver/observer/rules, both environments, and authorization.                                               |
| `Comparison` | Outcome, completeness, and scoped limits, bound to a receipt. Optional `details` references a `comparison-details-v1` artifact with exact channel witnesses, sample references, policy digest, and source-inventory links. Old comparisons without `details` remain readable. |
| `Pin`        | Scenario, human expectation, basis receipt and snapshot pair, last human decision, and chronological history. It has no execution authority or observation fields.                                                                                                            |

A complete snapshot is complete only within its selection policy; excluded files were not captured. Unsupported entries force incomplete capture. Incomplete captures need a limitation. Paths cannot be absolute, traverse upward, contain NUL/backslash/colon, or repeat across inventory lists. Selected untracked paths cannot include `.git` or `.after` components. Only regular Git modes `100644` and `100755` are captured; symlinks and other entries are unsupported.

Imported Go reports may omit `producer`. If present, it is an unverified caller claim; absence means no producer claim. Optional report `snapshot` is likewise a caller binding, not proof of test applicability.

Runner receipts—including failed or cancelled runs—require the complete planned binding. Do not invent actual environment evidence when unavailable. Pre-execution planning errors need no run receipt. An unstarted sample retains frozen plan bindings but omits actual derived image identities if no container was prepared. Samples retain actual images, completion, cleanup, timestamps, and separate observation/diagnostic artifact references. AFTER-8 may add `request_id` to bind asynchronous submission through storage.

Artifacts use content digests, never arbitrary paths, with channel, retained bytes, positive max bytes, completeness, redaction, and truncation flags. Storage enforces byte limits and hashes. A complete receipt needs at least one complete, unredacted, untruncated artifact. Redacted records require `redaction_policy: literal-v1`; receipt-level redaction forces incomplete evidence. The initial contract permits conclusive comparison only for complete receipts. Comparison details cannot be partial, redacted, or truncated if used for a conclusive result. Values remain in the artifact to preserve JSON number tokens.

## Independent state axes

| Axis          | Values                                                           |
| ------------- | ---------------------------------------------------------------- |
| Producer      | `importer`, `runner`                                             |
| Kind          | `reported`, `observed`, `none`                                   |
| Applicability | `unknown`, `current`, `stale`                                    |
| Execution     | `not_run`, `completed`, `failed`, `cancelled`                    |
| Comparison    | `not_compared`, `equal`, `different`, `incomparable`, `unstable` |
| Report        | `none`, `pass`, `fail`, `skip`                                   |
| Pin decision  | `pinned`, `accepted`, `reopened`                                 |

Imported evidence is always `importer`/`reported`/`unknown`/`not_run`/`not_compared`. A snapshot association is not applicability. Importers cannot assert runner environments, frozen bindings, or authorization. A report without terminal status is `none`, never inferred success. Caller-supplied snapshot binding never promotes applicability.

Observed evidence requires completed runner execution; completion means the experiment finished, not that behavior passed. Failed, cancelled, or unexecuted runs have no observation and cannot be current, equal, different, or unstable. Old observations may remain observed while stale or unknown. Missing, incomplete, redacted, or truncated observations cannot establish equality.

A pin's latest history event determines its decision; history starts with `pinned`. Accept/reopen never changes the expectation, receipt, or applicability. The store/review layer checks snapshot/scenario/receipt basis and appends immutable revisions. Scoped pins use `finite_example` or `human_intent`; each history event records action, comparison mode, prior candidate, expected bindings, and optional actual receipt. Changed selection reopens without a current result. Attachment does not accept; attaching a different receipt reopens an accepted pin; acceptance cannot change selection. Legacy pins without scope/context remain readable but confer no current applicability. See [review](REVIEW.md) for transitions and reuse rules.

## Trust boundary

Validation checks structural consistency, not truth. It cannot prove referenced captures exist, that a producer executed, that artifacts are compatible, or that a receipt applies to a newly selected pair. A digest is not a signature. Never decode arbitrary repository JSON as trusted runner evidence. `internal/gotestreport` constructs importer cards; `internal/runner` creates locally authorized receipts; `internal/compare` and `internal/review` derive compatibility/applicability.

The [CLI](CLI.md) accepts and emits records through bounded commands. Help, configuration, import, inspection, and comparison do not execute repository code. Execution uses the separate exact-consent runner path. No model client or init-time hook creates evidence or grants consent.

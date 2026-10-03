# Evidence records, version 1

AFTER-1 provides Go types and validation in `internal/evidence`. AFTER-2 adds
[private storage](STORAGE.md); AFTER-3 adds [local Git capture](CAPTURE.md).
Report import, execution, comparison computation and review remain separate tasks. All test
records and the [scenario example](schema-example.json) are **synthetic**.
Their digests are placeholders, not measurements.

## Encoding and identities

Each record has `schema_version: 1` and an `id` formatted as
`sha256:` plus 64 lowercase hex digits. Content references use the same format.
Resolved Git commit identities are 40 or 64 lowercase hex digits. An unborn
snapshot instead has `unborn: true` and an empty `commit` (never in merge-base mode).
Merge-base captures additionally retain the resolved input `base_commit` alongside
the target `commit` and `merge_base`. Working-tree snapshots may bind an
`index_snapshot` digest; storage verifies that it is an index snapshot with the
same commit/unborn basis. Branch names,
timestamps and paths are not content identities. ID derivation and checking
referenced bytes against digests belong to capture/storage, not this validator.
Stored pin IDs identify immutable content versions; AFTER-11 owns stable review
selection and append-only history transitions.

`Decode[Snapshot|Scenario|Receipt|Comparison|Pin]` accepts one JSON object, at most 4 MiB,
rejects unknown fields, unsupported versions (including zero), trailing values
and invalid records, and returns a zero record on failure. It never rewrites or
migrates input. Strings are UTF-8 JSON; times use RFC 3339. The Go JSON decoder's
duplicate-key handling applies (later values replace/merge earlier values);
records are local data, not signed canonical JSON.

Call `Validate` before persisting constructed records. Values are ordinary Go
structs, not immutable objects: storage must preserve immutable receipts rather
than overwrite them. Optional fields use `omitempty`; required enums have no
implicit safe-success zero value. A nil collection means no entries, not evidence
of coverage.

## Record fields

| Record   | Required content and meaning                                                                                                                                                                                                                                                              |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Snapshot | Source mode (`commit`, `working_tree`, `index`, `merge_base`), resolved commit, merge-base only for that mode, files with relative path/content digest/Git mode, excluded and unsupported entries with reasons, completeness, ordinary diff digest, limits.                               |
| Scenario | Frozen concrete input/setup/actions artifact digest, driver, observer and rules digests, supported boundary, author and explicit limits. Expectations are deliberately absent.                                                                                                            |
| Receipt  | Independent state fields, selected snapshot identities, ordered start/finish times, completeness, bounded artifact references and limits. Runner records additionally require both snapshots, scenario/input/driver/observer/rules bindings, both environments, and authorization digest. |
| Pin      | Scenario, human expectation text (specific result or explicitly broader requirement), basis receipt and snapshot pair, human decision and chronological history. No evidence state, execution authority or observation fields.                                                            |

A complete snapshot means complete within the declared selection policy, not that
excluded files were captured. Untracked exclusions remain inventoried. Unsupported
entries force incomplete capture. Incomplete captures need a limitation. Paths
cannot be absolute, traverse upwards, contain NUL/backslash/colon or repeat across
inventory lists. Only regular Git modes `100644` and `100755` are captured here;
symlinks and other entries must be inventoried as unsupported.

Environment bindings contain environment, toolchain and dependency digests plus
explicit argv. These are actual execution identities when a runner exists, not
permission to execute. Runner receipts, including failed/cancelled runs, require
the entire planned binding; unavailable actual environment evidence must not be
fabricated to make a record validate. Pre-execution planning errors need not
produce a run receipt.

Artifacts contain a content digest (never an arbitrary path), channel, retained
byte count, positive maximum byte count, completeness, redaction and truncation
flags. A complete receipt requires at least one complete, unredacted, untruncated
artifact. Storage must additionally enforce real byte limits and content hashes.
Artifacts and receipts optionally carry `redaction_policy: literal-v1`; redacted
records require it. Receipt-level `redacted` also forces incomplete evidence.
The initial conservative contract permits conclusive comparisons only for complete
receipts. A stored Comparison binds an outcome, completeness and scoped limits to
a receipt; channel-level witnesses and repetition records arrive in AFTER-9.

## Independent state axes

| Field                     | Values                                                           |
| ------------------------- | ---------------------------------------------------------------- |
| Producer                  | `importer`, `runner`                                             |
| Kind                      | `reported`, `observed`, `none`                                   |
| Applicability             | `unknown`, `current`, `stale`                                    |
| Execution                 | `not_run`, `completed`, `failed`, `cancelled`                    |
| Comparison                | `not_compared`, `equal`, `different`, `incomparable`, `unstable` |
| Report status             | `none`, `pass`, `fail`, `skip`                                   |
| Human decision (pin only) | `pinned`, `accepted`, `reopened`                                 |

An imported pass is always reported/unknown/not_run/not_compared. Its selected
candidate snapshot is an association, not validated applicability. It cannot
assert runner environments, frozen bindings or authorization. Validated external
bindings can be added with the importer task when that requirement is exercised;
this foundation intentionally does not promote imports.

Observed evidence requires completed runner execution. Completion means the
experiment completed, not that behavior passed. Failed/cancelled/unexecuted runs
have no evidence and cannot be current, equal, different or unstable. An old
observation may remain observed while applicability becomes stale or unknown.
Incomplete, missing, redacted or truncated observations cannot establish equality.

A pin's last history event determines its decision; history begins with pinned.
Accepting/reopening it never changes a receipt, expectation or applicability.
The future store/review layer must enforce append-only history and basis checks.

## Trust boundary and deferred checks

Validation establishes **structural consistency, not truth**. It cannot check
whether referenced captures exist, whether a producer actually executed, whether
artifacts are compatible, or whether a receipt applies to a newly selected pair.
Even a fully bound digest is not a signature. Consumers must not decode arbitrary
repository JSON as trusted runner evidence. AFTER-4 must construct importer
records from supported report facts; AFTER-8 owns locally authorized runner
receipts; AFTER-9/11 own compatibility and applicability derivation.

No CLI command accepts these records yet. The shipped CLI only prints help and
version; it has no startup hooks, repository access, subprocesses, network client,
model client or init-time execution. Unsupported commands fail rather than
pretending a capture, comparison or review succeeded.

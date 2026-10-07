# Evidence records, version 1

AFTER-1 provides Go types and validation in `internal/evidence`. AFTER-2 adds
[private storage](STORAGE.md); AFTER-3 adds [local Git capture](CAPTURE.md).
AFTER-4 adds [Go test report cards](GO-REPORTS.md), including incomplete cards
with report status `none`. AFTER-8 adds [frozen paired execution](RUNNER.md);
AFTER-9 adds [exact finite comparisons](COMPARISON.md); AFTER-11 adds
[persistent expectations and explicit review transitions](REVIEW.md). All test
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

`Decode[Snapshot|Capture|Scenario|Receipt|Comparison|Pin]` accepts one JSON
object, at most 4 MiB, rejects unknown fields, unsupported versions (including
zero), trailing values and invalid records, and returns a zero record on failure.
It never rewrites or migrates input. Strings are UTF-8 JSON; times use RFC 3339.
The Go JSON decoder's
duplicate-key handling applies (later values replace/merge earlier values);
records are local data, not signed canonical JSON.

Call `Validate` before persisting constructed records. Values are ordinary Go
structs, not immutable objects: storage must preserve immutable receipts rather
than overwrite them. Optional fields use `omitempty`; required enums have no
implicit safe-success zero value. A nil collection means no entries, not evidence
of coverage.

An immutable **Capture** record identifies one successful capture event: its
`captured_at` RFC 3339 time, `mode`, base/candidate snapshot IDs, optional index
snapshot ID, and exact selected untracked paths. It is stored separately from
snapshots so recapturing unchanged content preserves snapshot IDs but records a
new event. Capture times are event metadata, never inferred from file mtimes;
older snapshots without a matching Capture record have no recorded capture time.
The event ID is content-addressed like other records and does not change snapshot
identity.

## Record fields

| Record   | Required content and meaning                                                                                                                                                                                                                                                              |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Snapshot | Source mode (`commit`, `working_tree`, `index`, `merge_base`), resolved commit, merge-base only for that mode, files with relative path/content digest/Git mode, excluded and unsupported entries with reasons, completeness, ordinary diff digest, limits.                               |
| Capture  | Successful capture time and mode, base/candidate and optional index snapshot IDs, and selected untracked paths. It is a new immutable event on every successful capture, even when all snapshot IDs are unchanged.                                                                        |
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
produce a run receipt. AFTER-8 receipts add optional `request_id` (a runner-only
digest) to bind asynchronous submission through storage. Its failed/unstarted
samples retain the frozen plan bindings but explicitly omit actual derived image
identities when no container was prepared; they never claim observed execution.
Per-side/case/repetition sample artifacts record actual images, completion,
cleanup, timestamps and separate observation/diagnostic artifact references.

Artifacts contain a content digest (never an arbitrary path), channel, retained
byte count, positive maximum byte count, completeness, redaction and truncation
flags. A complete receipt requires at least one complete, unredacted, untruncated
artifact. Storage must additionally enforce real byte limits and content hashes.
Artifacts and receipts optionally carry `redaction_policy: literal-v1`; redacted
records require it. Receipt-level `redacted` also forces incomplete evidence.
The initial conservative contract permits conclusive comparisons only for complete
receipts. A stored Comparison binds an outcome, completeness and scoped limits to
a receipt. AFTER-9 adds optional `details`, an artifact descriptor for the bounded
`comparison-details-v1` report: exact channel witnesses, paired/repetition sample
references, policy digest and source inventory links. Old records without details
remain readable. Values live in the artifact, preserving number tokens rather
than passing through generic record sanitization. Partial/redacted/truncated
details cannot support a conclusive comparison.

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
assert runner environments, frozen bindings or authorization. A report without a
terminal status uses `none`, never inferred success. Importer snapshot bindings
remain caller-supplied and unverified; they never promote applicability.

Observed evidence requires completed runner execution. Completion means the
experiment completed, not that behavior passed. Failed/cancelled/unexecuted runs
have no evidence and cannot be current, equal, different or unstable. An old
observation may remain observed while applicability becomes stale or unknown.
Incomplete, missing, redacted or truncated observations cannot establish equality.

A pin's last history event determines its decision; history begins with pinned.
Accepting/reopening it never changes a receipt, expectation or applicability.
The store/review layer enforces snapshot/scenario/receipt basis checks and appends
immutable revisions. Scoped pins add `scope` (`finite_example` or `human_intent`)
and a `review` context per history event: action, comparison mode, prior candidate,
expected target bindings, and optional actual receipt ID. Changed selections
reopen without a current receipt. Attachment cannot accept, and a different
receipt reopens an accepted pin; explicit acceptance
cannot change selection. Missing contexts/scope in legacy pins remain readable
but confer no current applicability. See [review](REVIEW.md) for bounds and reuse.

## Trust boundary and deferred checks

Validation establishes **structural consistency, not truth**. It cannot check
whether referenced captures exist, whether a producer actually executed, whether
artifacts are compatible, or whether a receipt applies to a newly selected pair.
Even a fully bound digest is not a signature. Consumers must not decode arbitrary
repository JSON as trusted runner evidence. AFTER-4 constructs importer
cards from supported report facts; AFTER-8 owns locally authorized runner
receipts; AFTER-9/11 own compatibility and applicability derivation.

The [headless CLI](CLI.md) accepts and emits these records through bounded
commands. Help, configuration display, import, inspection, and comparison do not
execute repository code. The exact authorized runner path is separate; no model
client or init-time hook can create evidence or grant consent.

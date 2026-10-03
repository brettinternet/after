# Captured raw review

`internal/rawdiff.Open(store, result.Base, result.Candidate)` adds the shared
read-only review surface for AFTER-5. Use a pair returned by
[`capture.Capture`](CAPTURE.md), not arbitrary snapshots or the working-tree
capture's auxiliary index snapshot. CLI commands remain AFTER-10; TUI rendering
remains AFTER-13. This is not a behavior comparator or an execution engine.

```go
v, err := rawdiff.Open(s, captured.Base, captured.Candidate)
// Handle err before using v.
entries := v.Inventory()
hunks := v.Hunks()
page, err := v.Raw(0, rawdiff.MaxPageBytes)
context, err := v.Context(rawdiff.Candidate, "main.go", 0, 4096)
counts, err := v.Count(nil, nil) // initially all indexed hunks are unclassified
```

## Inventory and hunk bookkeeping

The sorted inventory includes every content/mode change plus every excluded or
unsupported path from either manifest. Unsupported and excluded entries have
`Change: "unknown"` even if both manifests list the same reason: their bytes were
not captured, so unchanged cannot be established. When either capture is incomplete,
equal stored file records also remain unknown: redaction can collapse different
original contents into identical blobs. This conservatively includes unchanged
paths too. Inventory length is the visible path count, not a claim that every
unknown path changed. Binary paths are marked
from Git's binary-patch header. Mode-only changes, binary changes, empty additions
and deletions need not have text hunks. Renames retain capture's deletion/addition
representation. Every path remains inspectable without an example mapping.

Each hunk has its exact captured patch byte range and a stable SHA-256 identity
bound to the diff digest and header offset. Duplicate identical hunk text at
different positions stays distinct. `Count` unions references across examples;
folded IDs must belong to that union. Its disjoint counts satisfy:

```text
Total = Mapped (visible) + Folded (mapped but hidden) + Unclassified
```

Unknown IDs are errors, not invented mappings. `Complete` is false when capture or
indexing is incomplete. Counts then describe indexed hunks only, never omitted
ones. A zero-hunk complete diff and a zero-mapped first review are valid.

Paths containing test, spec, fixture, mask, golden, expected, snapshot, comparison
or compare-policy (case insensitive) receive a **potential oracle** hint. This is
a deliberately broad filename heuristic with false positives and false negatives,
not semantic classification or approval. Production and potential-oracle changes
have identical raw access. The view has no scenario, receipt or pin mutation API;
opening it cannot replace a frozen driver, expectation or comparison policy.

## Bounds and failure behavior

- `Raw` and `Context` return byte windows of at most 64 KiB. `Next`, `Total` and
  `More` support viewport/paging consumers. Follow `Next` until `More` is false;
  never present the first window as the entire patch. Offsets at EOF return an
  empty final page. Invalid ranges, zero sizes and oversized requests fail.
- The raw patch is loaded once within storage's 16 MiB blob limit. At most 100,000
  hunks are indexed; exceeding this limit leaves the complete retained raw patch
  accessible, adds a limitation and makes counts incomplete. Long lines do not
  encounter a scanner token limit. Inventory and hunk lists are bounded by the
  existing capture/blob budgets, not lazily loaded from a repository.
- Context checks an exact manifest path and reads only its content-addressed
  blob. No path is joined to a repository directory. Each call verifies the whole
  bounded source blob before returning a window; it is not disk streaming.
- Missing/unsupported context returns `ErrContext`. A missing or corrupt diff
  retains inventory, adds a limitation and makes `Raw` return `ErrDiff`. No live
  filesystem fallback or helper execution occurs. Already opened raw bytes remain
  available if a context artifact disappears later.
- Capture limitations, including redaction, are carried on every page. Reaching
  the final retained byte does not remove these limits or establish completeness.
  Unmatched patch headers remain in raw bytes and make hunk counts incomplete.
- Strict `store.Get` still rejects missing referenced artifacts. Inventory recovery
  described above requires already available validated manifests, such as the
  capture result or an already loaded pair; this is not a damaged-store recovery
  loader. `Open` validates record shape and shared diff identity, not producer
  honesty or arbitrary pair provenance.

Page bytes, paths, hints and limits are **untrusted**, not terminal-safe strings.
Byte windows can split Unicode or lines. CLI/TUI renderers must decode across
windows and sanitize controls (including OSC), and keep partial-view limits
visible. This package does not render terminal content or silently alter raw
bytes. Captured source can be sensitive; keep it in the private store.

Neither opening a view nor reading any page launches a process. Only capture
creates the patch, using its isolated Git object database and disabled external
helpers. Import failure, no report adapter, missing observations and absent
example mappings do not participate in this API and cannot disable raw access.

## Verification

`mise exec -- task check:go` exercises all three capture modes with real temporary
Git repositories, weird/Unicode/control-character paths, binary and mode changes,
additions/deletions, unsupported/oversized paths, excluded untracked files, frozen
context after live edits, hostile helper configuration, traversal rejection,
missing artifacts, malformed reports, frozen scenarios, redaction, long lines,
page bounds, hunk-index limits, stable IDs and deduplicated counts.

# Captured raw review

`internal/rawdiff.Open(store, base, candidate)` exposes captured inventory, patch pages, source context and hunk bookkeeping. It never runs a process. This is a review surface, not a behavior comparator or execution engine.

## A stored patch

```sh
./bin/after diff --stored
```

Output from a throwaway Git repository (IDs and file content vary). It lists the CLI symlink as an unselected untracked file:

```text
after diff: stored pair 4747a69f → 93a57585
  origin: captured patch
  uncovered inventory:
    bin/after — excluded: untracked; not selected
  limits:
    two matching reads; not an atomic filesystem snapshot
    diff includes captured regular files only; inspect excluded and unsupported inventory
diff --git a/README.md b/README.md
new file mode 100644
index 0000000..fea9225
--- /dev/null
+++ b/README.md
@@ -0,0 +1,3 @@
+# Example
+
+A local change.
```

The command's diagnostic is on stderr; patch lines are on stdout. [Capture](CAPTURE.md) creates the patch in a private temporary Git repository from captured regular files only.

## Captured pairs and inventory

A captured patch is available only for a commit base and non-commit candidate with the same diff identity—the pair returned as `rawdiff.CapturedPair`. Cross-capture, reversed and index-vs-base/candidate pairs still expose both inventories and captured sources, but no captured patch or complete hunk count. CLI and TUI may instead show a computed diff (below).

```go
v, err := rawdiff.Open(s, result.Base, result.Candidate)
if err != nil { /* handle before use */ }
entries := v.Inventory()
hunks := v.Hunks()
page, err := v.Raw(0, rawdiff.MaxPageBytes)
context, err := v.Context(rawdiff.Candidate, "main.go", 0, 4096)
counts, err := v.Count(nil, nil) // all indexed hunks initially unclassified
```

The sorted inventory includes content/mode changes and every excluded/unsupported path from either manifest. Unknown entries remain `unknown`, even when both manifests list the same reason: uncaptured bytes cannot prove they are unchanged. With an incomplete capture, even identical stored records remain unknown because redaction can collapse distinct source into the same bytes. Inventory length counts visible paths; it does not claim every unknown path changed. Binary paths follow Git's binary-patch header. Mode-only changes, binary changes, empty additions and deletions may have no text hunk. Renames remain delete/add pairs.

Each indexed hunk has its exact byte range and a stable SHA-256 identity bound to the diff digest and header offset; identical text at two offsets remains distinct. `Files()` returns copies of exact `diff --git` sections. Unmatched headers stay visible with an empty path. `Count` unions example references; folded IDs must belong to that union. Counts satisfy:

```text
Total = visible Mapped + Folded + Unclassified
```

Unknown IDs are errors. `Complete` is false when capture or indexing is incomplete; counts then cover indexed hunks only. Zero-hunk complete diffs and zero-mapped initial reviews are valid. Paths with names such as `test`, `spec`, `fixture`, `mask`, `golden`, `expected`, `snapshot` or `comparison` receive a broad `potential oracle` filename hint. It has false positives and negatives and does not classify or approve behavior. The view cannot mutate scenarios, receipts, pins or frozen policies.

## Paging, source context and failures

`Raw` and `Context` return at most 64 KiB. Continue from `Next` until `More` is false; the first window is not the whole patch. Offsets at EOF return an empty final page. Zero, invalid or oversized windows fail. The raw patch is loaded within storage's 16 MiB blob bound; at most 100,000 hunks are indexed. Exceeding that cap leaves raw bytes available but makes counts incomplete. Long patch lines do not hit a scanner token limit.

`Context` matches an exact manifest path and reads its content-addressed blob; it never joins a repository path. Each call verifies the whole bounded blob before returning a window; it is not streaming disk access. Missing or unsupported context returns `ErrContext`. Missing/corrupt patch retains inventory, adds a limitation and makes `Raw` return `ErrDiff`. No live filesystem fallback or helper runs. Limits remain on every page even at EOF. `Open` validates record shape and requires an eligible captured pair before exposing patch bytes; it is not a damaged-store recovery loader or producer authentication.

Patch bytes, paths, hints and limits are untrusted and not terminal-safe. Windows may split UTF-8 or lines. Renderers must decode across windows, escape controls (including OSC), and keep partial-view limits visible. The package never alters raw bytes or renders terminal content. Keep captured source in the private store.

## Computed source diffs

For a stored pair without a shared captured patch, `View.Compute(ctx)` creates a deterministic Git-style display from the two stored source manifests. It uses sorted inventory, three context lines and a pure-Go Myers diff. It reads only content-addressed blobs; it does not invoke Git or any other process. This is not Git's patch: computed offsets have no hunk IDs and computed hunks do not appear in `Hunks()` or `Count()`.

| Work                                   |                                       Bound |
| -------------------------------------- | ------------------------------------------: |
| One text-file pair                     | 1 MiB combined bytes; 20,000 combined lines |
| One path                               |                                       4 KiB |
| Changed known source bytes across pair |                                       8 MiB |
| Myers walk                             |  1,000,000 diagonal visits/line comparisons |
| Generated output                       |                                      16 MiB |
| Binary detection                       |     NUL in first 8,000 bytes of either side |

A work-limit failure says `too large to diff here — open both sources`. Output exhaustion limits later paths but keeps them and their limits in Changes/source detail. Binary files show as binary; mode-only changes show both modes; unknown paths keep their limitation and have no computed hunk. Storage separately caps any source blob at 16 MiB. Browser `Load` computes and prepares the document off the UI event loop.

`task test:computed` checks applying-diff properties, source/output bounds, capture/load integration, a 10-file/20,000-line performance fixture and payment-change PTY behavior.

## CLI streaming

Bare `after diff` uses `capture.ReadLive` and `rawdiff.Unstored`: consistent hardened reads and a private-Git patch, without writing storage. `after diff --stored` and explicit pairs page a captured patch in 64 KiB windows; pairs without one use `View.Compute` from stored sources. `internal/terminal.WriteDiff` streams both safe and raw output under the existing artifact bounds instead of building an unbounded line index.

Safe display sanitizes controls, format characters and invalid UTF-8; tabs stay tabs in pipes and expand on terminals. Terminal color uses fixed theme SGR. `--raw` emits exact selected bytes and is refused when stdout is a terminal; computed raw bytes are the computed patch, not Git's original. `--stat` counts changed lines by selected patch section like `git diff --stat`; it cannot combine with `--raw`. Stderr names the pair/source and patch origin, then lists every unknown/excluded/unsupported/limited path and limit. See [CLI diff forms](CLI.md#diff).

`task check:go` covers real temporary Git repositories, unusual paths, binary/mode changes, unsupported files, frozen context, hostile helpers, missing artifacts, redaction, long lines, page/hunk bounds and stable accounting. Streaming tests cover 100,250 lines and a complete output digest under the terminal's 64 MiB preparation budget.

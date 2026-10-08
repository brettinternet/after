# Local Git capture

`internal/capture.Capture(ctx, repositoryRoot, store, options)` implements AFTER-3.
The [headless CLI](CLI.md) exposes capture and raw-diff inspection. Capture reads
source; it never builds, tests, runs project commands or creates an execution receipt.

Project commands resolve their checkout root by checking for a `.git` directory
or worktree `.git` file from the selected directory upward. This lookup is
filesystem-only; it runs no Git command. The CLI passes that root explicitly to
`Capture`, which retains its no-discovery rule. If capture fails, the CLI prints
an allowlisted fixed reason and fix, such as `after: capture failed: unmerged
index is unsupported — resolve the index conflicts, then retry capture`. Git
output, repository-controlled text and absolute project paths are never included.

## Selection and identity

- Default: HEAD versus tracked working-tree files, plus explicitly selected
  non-ignored untracked paths. The candidate also references a separately stored
  index snapshot, so staged and unstaged bytes remain distinguishable. Each
  snapshot's diff is the patch from HEAD to that snapshot, so the index snapshot
  carries the staged patch, never the working-tree one.
- `Mode: evidence.Index`: HEAD versus the index, ignoring unstaged edits.
- `Mode: evidence.MergeBase`, with `Base` and `Target`: resolve both commit-ish
  values, compare their unique merge base with the target commit, without fetch.
  Store the resolved base tip, target and merge-base identities, not branch names.
- An unborn repository has an empty base with `unborn: true` and an empty commit
  field, not a fabricated all-zero commit. Its index and working tree can still
  be captured.

An empty implicit `after review` capture, when no review is saved, exits 0 without
opening the TUI. It says the working tree or index matches HEAD (or that the
merge-base comparison has no changes), suggests a default-branch comparison when
HEAD has commits ahead, and names up to three sanitized excluded untracked paths
with `--include-untracked` suggestions. `after capture` persists the empty capture
and shows the same guidance in `Next`. Excluded untracked entries alone do not
suppress the guidance; unknown or unsupported entries still require review. The
branch check uses the hardened, network-disabled Git runner to read
`refs/remotes/origin/HEAD`, then local `main`, then `master`; it executes no project
code and does not fetch. Saved reviews and `after review --new` retain their normal
resume/start-over behavior, and the JSON capture envelope is unchanged.

Content-addressed private storage retains source bytes and Git executable modes.
Capture IDs include the selection inventory, limitations, diff and index binding;
mtimes are not identities. No user index, refs or files are changed. The only
persistent writes are to the caller's already-open [private store](STORAGE.md).

Non-ignored untracked files are excluded and listed by default. Inclusion takes
exact repository-relative paths, not globs or directories. Ignored untracked
files cannot be selected; existing tracked files follow Git's tracked semantics.
Any `.after` or `.git` path component is always excluded, even if tracked.
Invalid UTF-8 and paths not representable by the [record schema](SCHEMA.md)
(backslash, colon, traversal) fail capture explicitly rather than being rewritten.
Spaces, tabs, newlines, Unicode and leading dashes are supported using NUL-delimited
Git inventory and explicit argument vectors.

## Incomplete and unsupported content

Symlinks, symlink parents, non-regular files, submodules, LFS pointers and files
larger than 8 MiB remain in the unsupported inventory. Their bytes are not
captured, and completeness is `incomplete`. Submodules are never traversed; LFS
payloads are never fetched. Missing Git objects, conflicts, sparse/skip-worktree
or assume-unchanged indexes, shallow/partial repositories and ambiguous merge
bases fail visibly rather than produce a supposedly complete snapshot.

The ordinary binary Git diff is generated in a private temporary repository from
captured regular-file bytes only. Renames appear as deletion/addition. Unsupported
or excluded entries are **not** represented faithfully by that patch: always
inspect both inventories alongside it. The [captured raw review API](RAW-DIFF.md)
combines those inventories with bounded diff/context windows and hunk bookkeeping. Literal redaction in the store forces incomplete snapshots;
a redacted capture must not be treated as the original executable source.

Budgets are 2,000 entries per inventory, 8 MiB per source file, 64 MiB of source
bytes across base/index/candidate per read pass, and 16 MiB per Git stdout/diff.
Repository-wide/output-budget failures return errors, never silently truncated
complete captures. Each Git command has a 30-second deadline; caller cancellation
also applies. Storage has its own additional bounds. Failed publication may leave
unreferenced immutable artifacts, never a successful partial result.

## Consistency and execution boundary

Each attempt performs two full reads of selected content, modes, index inventory,
untracked selection and resolved refs. Only identical reads are persisted. A
changed read retries up to three attempts; repeated change fails with
`ErrInconsistent`. A file changing size or mtime while open fails immediately.

`ReadLive` (bare `after diff`) and `Unchanged` (the status/inspect freshness check)
perform the same consistent read without writing evidence. `ReadLive` returns
unstored snapshots whose file and diff identities are SHA-256 digests of the bytes
read, plus the generated patch. `Unchanged` compares a stored working-tree or index
capture's commit, file content identities, excluded and unsupported inventory, and
index snapshot with the current read; it never uses timestamps and accepts only
complete stored snapshots.
The tests use a deterministic between-read barrier, not timing sleeps.

This is **not an atomic filesystem snapshot**. Coordinated ABA edits (changing and
restoring bytes between reads), adversarial metadata restoration, or edits after
the accepted read cannot be ruled out. Stop writers for stronger consistency;
subsequent edits never alter already stored bytes or create an observation.
Rooted reads prevent escapes outside the supplied root; symlinks are rejected,
not intentionally followed. This is capture isolation, not an execution sandbox.

Git receives a fresh environment without ambient Git overrides, loader settings,
traces, credentials or proxies. System/global config and attributes are disabled;
local fsmonitor, hooks, automatic maintenance, protocols and external attributes
are overridden. Source access uses only read-only plumbing (`ls-tree`, `ls-files`,
`cat-file`, ref/config queries). Blobs are read in batches: one `cat-file
--batch-check` for identities and sizes, then `cat-file --batch` chunks kept under
the 16 MiB process-output ceiling. Replies must match each requested object's ID,
type and size, and the capture byte budget is charged before payloads are read.
Patch generation hashes each distinct frozen payload once, in one private
`hash-object --stdin-paths` process with generated file names. No source-repository diff, textconv, clean/smudge,
checkout, hook, credential or network command is requested. Lazy fetch is disabled
and all transport protocols are denied. Supported macOS/Linux hosts must provide trusted Git at `/usr/bin/git`; capture
never searches ambient `PATH`. That system executable and the operator-selected
repository root are trusted inputs, not repository commands.

## Verification

`mise exec -- task check:go` runs build, race tests, vet and formatting checks.
The capture tests exercise all modes, staged/unstaged separation, unborn history,
weird paths/binary/mode changes, exclusions, unsupported content, source immutability,
controlled overlapping writes, hostile config/environment/helpers, cancellation
and redaction. These are capture tests, not evidence that the future runner or
review loop works.

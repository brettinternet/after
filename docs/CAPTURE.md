# Local Git capture

`internal/capture.Capture(ctx, repositoryRoot, store, options)` stores bounded Git snapshots and a normal diff. Capture reads source files; it never builds, tests, runs project commands, or creates an execution receipt.

Snapshot IDs include the paired diff: changing the candidate can change the base snapshot ID without changing its Git commit. The `Base commit` row identifies that stable commit.

## Modes and paths

| Mode                                       | Compared content                                                        | Notes                                                                                     |
| ------------------------------------------ | ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Default `working_tree`                     | `HEAD` → tracked working tree, plus explicitly selected untracked files | Also stores the index snapshot, so staged and unstaged bytes stay distinct.               |
| `--staged` (`index`)                       | `HEAD` → index                                                          | Ignores unstaged edits.                                                                   |
| `--base REF [--target REF]` (`merge_base`) | Unique merge base of `REF` and target commit                            | Target defaults to `HEAD`; no fetch. Stores resolved commit identities, not branch names. |

An unborn repository has an empty base with `unborn: true` and no fake all-zero commit. Its index and working tree can still be captured. Each snapshot's patch is from its commit base: an index snapshot contains the staged patch, not working-tree edits.

Untracked paths are excluded and listed by default. `--include-untracked PATH` accepts repeated exact repository-relative, non-ignored file paths—not globs or directories. Ignored files cannot be selected. Existing tracked files follow Git's tracked-file rules. Any `.after` or `.git` path component is excluded, even when tracked. Invalid UTF-8 or schema-unrepresentable paths (backslash, colon, traversal) fail instead of being rewritten. Spaces, tabs, newlines, Unicode and leading dashes work through NUL-delimited Git output and argument vectors.

The CLI finds a checkout root by walking upward for a `.git` directory or worktree `.git` file; this is filesystem-only. It passes the root explicitly because the capture API does no discovery. Failures use fixed allowlisted reason/fix text; Git output, repository text and absolute project paths are not printed.

## Example

From a throwaway repository, capture one explicitly selected untracked file:

```sh
./bin/after capture --include-untracked README.md
```

Actual output excerpt (IDs vary). The temporary checkout used a `bin/after` symlink to the built CLI, so capture listed it as excluded:

```text
Captured candidate 93a57585 (working tree) against base 4747a69f (commit)
  Base         4747a69f · 0 paths · complete · 0 excluded · 0 unsupported
  Candidate    93a57585 · 1 path · complete · 1 excluded · 0 unsupported
  Limit        two matching reads; not an atomic filesystem snapshot
  Limit        diff includes captured regular files only; inspect excluded and unsupported inventory
```

If `after review` has no saved review and the implicit capture has no changed or unknown paths, it exits 0 without opening the TUI. It explains whether the working tree/index matches `HEAD` or a merge-base comparison is empty. If `HEAD` is ahead, it suggests a local default-branch comparison, checking `refs/remotes/origin/HEAD`, then `main`, then `master`; it does not fetch. Up to three sanitized excluded untracked paths are shown with `--include-untracked` suggestions. Excluded-only paths do not suppress this guidance; unknown or unsupported paths remain reviewable. `after capture` persists an empty capture and gives the same `Next` guidance. Saved reviews and `after review --new` keep their resume/start-over behavior; the JSON capture envelope is unchanged.

## Completeness and budgets

Symlinks, symlink parents, non-regular files, submodules, LFS pointers and files over 8 MiB are inventoried as unsupported, not read; they make the snapshot incomplete. Submodules are never traversed and LFS data is never fetched. Missing Git objects, conflicts, sparse/skip-worktree or assume-unchanged indexes, shallow/partial repositories and ambiguous merge bases fail visibly.

The normal diff is generated in a private temporary Git repository from captured regular-file bytes. Renames appear as deletion plus addition. Excluded and unsupported entries are not faithfully represented by the patch: inspect both inventories too. [Raw review](RAW-DIFF.md) adds bounded patch pages, context and hunk bookkeeping. Literal redaction marks the snapshot incomplete; redacted source must not be treated as executable original content.

| Bound                                             |                                              Limit |
| ------------------------------------------------- | -------------------------------------------------: |
| Entries per inventory                             |                                              2,000 |
| One source file                                   |                                              8 MiB |
| Source bytes per read pass (base/index/candidate) |                                             64 MiB |
| Git stdout or diff                                |                                 16 MiB per process |
| Git command deadline                              |               30 seconds, plus caller cancellation |
| Consistency retries                               | 3 attempts; each requires two identical full reads |

A budget failure errors; it never returns a silently truncated complete capture. Failed publication may leave unreferenced immutable artifacts, not a successful partial result. A changed read retries and then returns `ErrInconsistent`. File size or mtime changes while a file is open fail immediately.

`ReadLive` (bare `after diff`) and `Unchanged` (status/inspect freshness) perform the same consistent read without storing evidence. `ReadLive` returns unstored file/diff SHA-256 identities and a patch. `Unchanged` compares commit, content hashes, index, and excluded/unsupported inventory; it uses no timestamps and accepts only complete stored snapshots. These reads are not atomic filesystem snapshots: ABA edits, restored metadata, or writes after the accepted read are not ruled out. Stop writers for stronger consistency. Later edits cannot alter stored bytes or create an observation.

## Git trust boundary

Capture invokes only read-only Git plumbing. It uses `/usr/bin/git` on supported macOS/Linux hosts, a fresh environment, disabled system/global config and attributes, disabled fsmonitor/hooks/maintenance/external filters, disabled lazy fetch and all transport protocols. It does not request checkout, diff drivers, textconv, clean/smudge, credentials or network access. Blobs are read in bounded batches after charging the byte budget; replies must match requested object IDs, types and sizes. Patch generation hashes frozen payloads once in a private object database. The system Git binary and caller-selected repository root are trusted inputs; repository commands are not run.

`task check:go` runs build, race tests, vet and gofmt checks. Capture tests cover modes, index separation, unborn history, unusual paths, binary/mode changes, exclusions, unsupported content, source immutability, overlapping writes, hostile Git configuration, cancellation and redaction. They do not prove the runner or review loop.

# Private local storage

`internal/store` persists versioned records and content-addressed artifacts in a project's private `.after/` directory. It is a library, not a database or execution engine; opening it never runs project code.

## Open and identify records

`Open(project, writable, secrets)` takes a trusted project directory. A read-only open creates nothing. A writer creates `.after/` with mode `0700`, takes a nonblocking advisory lock, and publishes `.after/.gitignore` containing `*`. This ignores store contents without changing the checkout's `.gitignore` or `.git/info/exclude`.

```text
.after/                 0700, private and ignored
  writer.lock           permanent lock inode; do not delete or replace
  .gitignore            *
  <kind>-<sha256>        immutable records and artifact descriptors/blobs
  session.json           replaceable UI session, not evidence
  pending-<random>        possible interrupted publication
```

Stored records include snapshots, successful capture events, scenarios, receipts, comparisons and pins. Leave a new record ID empty: storage derives it from SHA-256 of Go JSON encoding with `id` empty. A supplied ID must match. Field order, slice order and nil-versus-empty slices affect identity. This is a local Go encoding contract, not cross-language canonical JSON or a signature. Records and artifacts are immutable; reads verify references, size and content hashes. Digests do not authenticate producers.

A successful capture creates a new immutable capture event even when snapshot IDs are unchanged; failed capture creates no event. Snapshot IDs exclude capture time. `CapturesForSnapshot` scans at most 512 records/16 MiB and returns at most eight matching events; reached bounds are visible, never presented as complete history. Legacy snapshots without an event have no recorded capture time. Pin revisions are also immutable; review history and selection rules are in [REVIEW.md](REVIEW.md).

Artifact identity is the hash of retained bytes plus an immutable descriptor (channel, byte counts, redaction policy and completeness). Receipts must reference an already stored exact descriptor; changing flags cannot turn partial bytes into complete evidence. Snapshot source/diff blobs and scenario input blobs must exist. Storage checks snapshot/scenario/receipt and pin-basis bindings but does not authenticate a producer. Toolchain, authorization, driver, observer and rules digests are identities; they need not be stored as blobs.

## Single writer and durability

Writers hold the permanent `writer.lock` inode for their lifetime. A second writer is rejected, including another handle in the same process; read-only handles may coexist. There is no stale-PID timeout or lock stealing. Normal close, process death or reboot releases the OS lock. Retry only after its owner exits; never delete or rename the lock as recovery.

Publication writes a private random pending file, syncs it, links the complete inode to an unused content name without replacement, removes the pending link, then syncs the directory. Successful return means file and directory sync completed, subject to filesystem guarantees. A failed publication may leave an unreferenced complete object; retries are idempotent and prior versions are not replaced. A crash after linking but before removing the pending link can leave a two-link object, which reads reject.

Pending files count toward limits. There is no automatic garbage collection or broad pending-file deletion. For repair, stop all store users, keep a backup and inspect exact pending/content inodes. Never remove the lock. If ownership is uncertain, recapture into a new private store rather than repairing evidence.

## Confinement, bounds and redaction

The store uses an `os.Root` directory capability, digest-only flat names, no-follow/nonblocking opens and regular-file checks. It rejects symlink stores/artifacts, arbitrary path IDs, hard links, FIFOs and unsafe permissions rather than repairing them. Files must be owned by the current user and mode `0600`; `.after/` must be a real `0700` directory.

| Bound                                 |                                    Maximum |
| ------------------------------------- | -----------------------------------------: |
| One record                            |                                      4 MiB |
| One input/retained blob               |                                     16 MiB |
| Entries, including lock/pending files |                                     10,000 |
| Logical aggregate storage             |                                    512 MiB |
| Redaction literals                    | 128; each nonempty and at most 4,096 bytes |

Space for publication is checked. Disk-full and permission errors preserve wrapped OS identities. Failed multi-object operations may leave unreferenced blobs/descriptors, never a complete fabricated receipt.

`literal-v1` redaction replaces matching byte spans (including overlaps) **before truncation, hashing or filesystem writes**. JSON strings are decoded first. It covers artifact bytes/channels and record free text: paths, reasons, authors, boundaries, limits, argv and expectations. Hashes, timestamps and enums are structural, not redacted text. Literals stay in memory and are never logged or persisted.

This is explicit redaction, not secret discovery. Supply safe inputs and known secrets; encoded/transformed variants need their own literals. Truncation or redaction marks artifacts incomplete. Redacted receipt free text and source manifests also mark records incomplete; comparisons reject conclusive partial evidence. Reopening preserves those flags even with a different literal list. Incomplete evidence is not equal, fresh, accepted or executable.

The caller's project path is trusted. Storage does not defend against another process running as the same user that actively replaces directories or lock inodes. Use a local filesystem with advisory locks, hard links and `fsync` support.

`mise exec -- task check:go` and `mise exec -- task test` cover immutable records, reopen, damaged references, publication fault points, ENOSPC/EACCES, permissions, path/link/FIFO rejection, budgets, decoded-field and overlapping redaction, and a killed writer followed by lock recovery without deleting unrelated files. They do not simulate a physical drive losing acknowledged writes.

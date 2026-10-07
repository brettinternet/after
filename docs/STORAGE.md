# Private local storage

`internal/store` implements AFTER-2 storage on macOS and Linux. It is a library;
the [headless CLI](CLI.md) composes it with capture, import, execution, comparison,
and inspection commands. Opening storage never executes project code.

## Objects and identities

`Open(project, writable, secrets)` takes a trusted project directory and opens its
private `.after/` directory. Read-only opens create nothing. A writable open
publishes `.after/.gitignore` containing `*` through the same private, durable,
no-overwrite publication path used for store objects. This also supplies the file
for an existing store that predates it. The rule ignores store contents without
changing the checkout's `.gitignore` or `.git/info/exclude`. `Put`/`Get` support
snapshots, capture records, scenarios, receipts, comparisons and pins. Leave a new
record ID empty; storage derives it from SHA-256 of the Go JSON encoding with
`id` set to the empty string. A supplied ID must match. Ordered slices, nil
versus empty slices, and all other fields participate in identity. This is a
local encoding contract, not a cross-language canonical JSON or signature
scheme.

Every version is immutable, including capture events and pin history revisions.
A successful capture publishes one new event even when its snapshot identities
are unchanged; failed capture attempts do not create an event. Snapshot identity
never includes a capture time. `CapturesForSnapshot` searches at most 512 capture
records and 16 MiB of capture-record data, then returns at most the eight latest
matching events found. It reports when a scan or result bound was reached, so a
partial history is never presented as complete. No filesystem modification time
is used to fill missing capture history. Legacy snapshots with no event have no
recorded capture time.

This task stores pin versions; AFTER-11 owns append-only history transitions and
stable review selection. Comparison records currently bind a result and scope to
a receipt; AFTER-9 owns channel witnesses and actual comparison computation.

Artifacts have a hash of retained bytes plus an immutable descriptor containing
channel, retained/max bytes, redaction policy and completeness. Receipts must
reference an already persisted exact descriptor; changing its flags does not
turn partial bytes into complete evidence. Snapshot content/diff and scenario
input blobs must exist. Receipt snapshot/scenario bindings and pin receipt/basis
are checked. Toolchain, authorization, driver, observer and rules digests are
identities, not necessarily blobs stored here. Reads verify local dependencies,
byte sizes and content hashes; missing, corrupt or unsupported-version records
produce errors without deleting history. This does not authenticate producers.

## Durability and one writer

Writers acquire a nonblocking OS advisory lock on the permanent `writer.lock`
inode. A second writer is rejected, including another handle in the same process.
Read-only handles may coexist. Do not delete, replace or rename the lock file.
There is no stale-PID timeout or lock stealing: normal close, process death and
reboot release the kernel-held lock. Retry opening after the owner has exited.
An unrelated file is never deleted during lock recovery.

Publication writes a private random pending file, syncs it, links the complete
inode to an unused content name without replacement, removes the pending link,
then syncs the directory. A failure may leave an unreferenced complete object;
retrying is idempotent. A crash between link creation and pending-link removal
can leave a two-link object, which reads reject rather than trusting a hard link.
A prior complete version is never replaced. A successful return means both file
and directory sync completed, subject to the filesystem's durability guarantees.

Interrupted pending files count toward storage limits. No automatic garbage
collection or broad pending-file deletion is implemented. If repair is needed,
stop all users of the store, retain a backup, and inspect the exact pending and
content inodes before making a manual repair. Never remove the permanent lock
as a recovery shortcut. Prefer recapturing into a new private store when ownership
of leftover data is uncertain.

## Confinement, limits and redaction

The store uses a directory capability (`os.Root`), digest-only flat names,
no-follow nonblocking file opens and regular-file checks. `.after` must be a real
0700 directory; files must be owned by the current user, 0600 and single-linked.
Symlink stores, symlink artifacts, hard links, FIFOs and arbitrary path IDs are
rejected. Existing unsafe permissions are rejected, not silently repaired.
The caller's project path is trusted; the store is not a sandbox against another
process running as the same user that actively replaces directories or lock
inodes. Use a local filesystem supporting advisory locks, hard links and fsync.

Limits are 4 MiB per record, 16 MiB per input/retained blob, 10,000 directory
entries (including lock/pending files), and 512 MiB logical aggregate bytes.
Space for each publication is checked. Disk-full and permission errors retain
wrapped OS error identities. Failed multi-object operations can leave harmless
unreferenced blobs/descriptors; no complete receipt is fabricated.

Supply up to 128 nonempty literal secrets (maximum 4096 bytes each) in memory
when opening a writer. `literal-v1` replaces all matching byte spans, including
overlapping matches, **before truncation, hashing, or any filesystem write**.
Artifacts, channels and record free-text fields (paths, reasons, authors,
boundaries, limits, argv and expectations) are covered. JSON strings are decoded
before redaction, so escaped secrets are covered. Hashes, timestamps and enums
are structural fields, not secret-bearing text. The literal list is never
persisted; the store does not log payloads or secret values. This is explicit
redaction, not a secret detector: callers must select safe inputs and supply
known sensitive values. Encoded/transformed variants require their own literals.

Truncation or redaction marks artifacts incomplete. Receipt free-text redaction
marks the receipt incomplete with its policy; conclusive partial comparisons are
rejected. Redacted source manifests are marked incomplete. Reopening preserves
these flags even with a different in-memory secret list. Nothing here implies
that incomplete evidence is equal, fresh, accepted or executable.

## Verification

Run `mise exec -- task check:go` and `mise exec -- task test`. Storage tests cover
all five record types, immutable identity, reopen, damaged manifests/references,
publication fault injection, ENOSPC/EACCES propagation, permissions, paths,
symlink/hard-link/FIFO rejection, byte/count budgets, decoded-field and overlapping
redaction, metadata forgery, and a real killed writer process followed by lock
recovery without deleting unrelated files. Fault tests exercise durability
boundaries; they do not simulate a physical drive losing acknowledged writes.

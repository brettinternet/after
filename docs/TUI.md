# Captured evidence browser

`after review --tui` opens the stored engine records, not a demo or a live checkout.
It does not run Git, import a report, build code or contact Docker on open.
The existing headless `review <pin-revision-id>` remains unchanged.

```sh
after capture --project /path/to/project
# Use the returned candidate/base snapshot IDs. Evidence IDs are optional.
after review --tui <candidate-id> --base <base-id> \
  --project /path/to/project --evidence <comparison-or-receipt-or-report-id>
```

Repeat `--evidence` for up to 32 stored IDs. There is no automatic discovery or
implicit selection of the newest result. Start without evidence to review the
complete captured inventory and patch with an explicit **not checked** screen.
Unknown/corrupt evidence IDs do not hide the raw inventory.

## Keys and evidence

| Key                                     | Action                                                                                                                  |
| --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| Up/down or `j`/`k`, PgUp/PgDn, Home/End | Select a record or scroll the focused pane                                                                              |
| Enter                                   | Inspect the selected record or inventory entry                                                                          |
| `d`                                     | Open the complete unclassified inventory, including unknown, excluded, unsupported, binary and potential-oracle entries |
| Tab in inventory                        | Open the captured raw patch                                                                                             |
| Tab / Shift+Tab in inspector or patch   | Next/previous section                                                                                                   |
| `[` / `]`                               | Previous/next 4096-byte page; all available stored bytes remain reachable                                               |
| Left/right or `h`/`l`                   | Pan clipped data/help text                                                                                              |
| Escape                                  | Return to list, preserving selection                                                                                    |
| `?`                                     | Scrollable help                                                                                                         |
| `c`                                     | Explicitly capture HEAD versus working tree in the background; untracked files stay excluded                            |
| `i`                                     | Explicitly import the file configured at launch, if any                                                                 |
| `x`                                     | Request cancellation of the active job                                                                                  |
| `q` / Ctrl-C                            | Quit, cancel and join owned work, restore terminal                                                                      |

The text-only `STATE` row comes from validated engine enums, not test names or
logs. Reported outcomes remain reported with unknown applicability; a binding to
a different selected snapshot is stale. Receipts for another pair are stale,
never rebound. Failed/not-run, incomplete, incomparable and unstable results are
not equality. A job completion is not an observation or acceptance.

Receipt sections expose producer, both snapshot bindings, timestamps, execution
and environment metadata, frozen scenario/input, exact comparison witnesses,
limits, every repetition's before/after observation and diagnostics. Channel
names such as `base/43200/0/observation` identify side, delay and repetition.
Observation JSON contains exact responses and independently recorded provider
calls. Report sections show caller provenance and unavailable inputs/effects;
they do not reconstruct observations from test names. No screen asserts more
than the recorded finite inputs and supported channels.

Every untrusted line is prefixed `data |`. Artifact bytes are quoted in chunks
of at most 32 bytes, including escaped newlines and binary/control bytes; page
boundaries and clipping are explicit. This is a deliberately plain inspector,
not syntax highlighting. Source sections read immutable captured blobs, never
live paths. Use [headless export](CLI.md) for machine-readable/raw artifact pages.

## Background actions and limits

To enable `i`, supply `--import-file /path/to/report.jsonl --producer 'caller Go version'`.
The file is opened only on that key action, through the same bounded regular-file
reader as the CLI. Capture/import completion adds a result row with persisted IDs;
it **does not replace the selected capture or turn reported data into observed**.
Open a new browser explicitly with those IDs. Actions are unavailable until the
initial immutable records finish loading. Only one mutation job runs at once.
Active/completed/cancellation status is visible; there is no invented percentage.
Page loads run off the UI loop. Late page/request results cannot replace a newer
view, and discarding a UI completion never deletes a stored receipt.

Pinning, accepting a new snapshot, and interactive rerun consent are AFTER-14.
Use the [headless pin workflow](REVIEW.md) and exact-plan [run command](CLI.md)
until then. The browser's job lifecycle is tested against the actual runner's
denied-execution persistence path; no key or configuration bypasses run consent.
There is no automatic refresh, filesystem watcher or live run subscription.

The viewport is capped at 240 columns and 100 rows; extremely narrow terminals
clip explicitly. Storage/capture/report bounds still apply. Artifact reads verify
the whole bounded blob before serving a page. An unavailable artifact produces a
limitation, never an empty successful result. The browser is not an authenticated
producer verifier: content-addressed records bind bytes, not producer honesty.

## Verification

- `mise exec -- task test:terminal`: real Git captures, full inventory, immutable
  source, byte-page round trips, state labels, stale messages, responsive
  cancellation and persisted denied-run receipts, plus terminal foundation tests.
- `mise exec -- task test:cli`: actual CLI capture/import followed by PTY browsing,
  inspector/diff/help, background capture/import, 32×8 resize and termios,
  alternate-screen and cursor restoration. No payload terminal controls escape.
- `mise exec -- task cli:proof` with the separately provisioned Docker settings:
  real paired synthetic payment execution, then native CLI PTY browsing of its
  observed receipt and all eight observations. Twelve-hour provider calls are
  one versus two with identical responses; thirty-second control is one versus
  one. This target explicitly authorizes only the logged synthetic test plans.

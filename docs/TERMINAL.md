# Terminal foundation decision

AFTER-12 selects **Bubble Tea v1.3.10** for the thin local TUI. The evaluated v1 model/update/string-view API meets this slice's budgets; migrating to the newer v2 API is not needed for this decision. `go.mod`/`go.sum` pin the tested dependency graph, including **uniseg v0.4.7** for grapheme widths and **creack/pty v1.1.24** for tests only. No Bubbles components, theme, extra executable, or generic UI/job framework is introduced.

This is an executable framework evaluation, **not the finished TUI**. AFTER-13 owns evidence browsing and CLI wiring; AFTER-14 owns the interactive pin/rerun loop. Run the retained experiment as tests:

```sh
mise exec -- task test:terminal
mise exec -- task terminal:bench
mise exec -- task terminal:fuzz
```

## Retained boundary

`internal/terminal` contains an immutable captured-text line index, bounded text/hex document access, a terminal-safe text boundary, and Bubble Tea lifecycle handling. Construct `NewDocument` in background preparation, not `Update` or `View`. It copies bytes once; keypresses and resize reuse the same index and render only visible rows. Neither rendering nor event updates has a repository/store/process dependency.

Background work belongs in Bubble Tea commands, not the event loop. `Result` carries snapshot and request identities; a mismatched result cannot change the displayed job status. Completion does not claim observed/fresh/preserved/accepted evidence or replace a document. The test-only fake command waits on cancellation while input, resize and quit continue through the actual event loop. The runner remains responsible for bounded execution and joining its owned jobs: **Bubble Tea does not join commands**. `Model.Context` is cancelled on quit; `Run` also cancels it on every error/exit. The program uses the parent context so ordinary quit is not incorrectly returned as an external-cancellation error.

All untrusted title, patch and error text passes `Line`. It renders control/format characters as visible `\uNNNN` text, including ESC, C1 CSI/OSC, carriage return, backspace, bidi controls and embedded newlines. OSC clipboard/title/hyperlink payloads cannot become terminal instructions. Tabs expand to four-column stops; grapheme clusters preserve combining marks and wide-character selection/layout. Leading combining clusters get a dotted-circle base to avoid modifying the trusted row prefix. Bracketed paste does not invoke navigation/quit commands. Trusted UI labels and selection markers remain outside content rows; raw repository prose is never interpreted as an evidence badge.

Bounds and limitations are explicit:

- Input: at most 16 MiB. Text indexes at most 250,000 lines; a larger document retains its exact bytes and reports the text-index limitation instead of silently dropping the remainder. Hex rows are computed directly from byte offsets, without a line index.
- Viewport: at most 240 columns and 100 rows, clipped to the actual smaller dimensions. Tiny terminals may show only the header; quit still works.
- Each text row examines at most 4,096 source bytes and clips at a grapheme boundary with `…`. `LineAt` pans by display columns (including tab stops) without splitting clusters. Content-viewer callers point long-line clipping to exact hex view. Captured bytes remain untouched.
- Format controls, including emoji joiners, are escaped rather than preserved. Width follows uniseg's Unicode tables; terminal-specific ambiguous-width/font differences are not solved. Invalid UTF-8 becomes replacement characters.
- `Run` returns errors without printing them after terminal restoration. Future callers must also use safe rendering for untrusted diagnostics. The headless CLI is not changed by this task.
- SIGKILL, a broken terminal device and non-cooperative external jobs cannot be made safe by a TUI library. No project execution or signal/process sandbox is added here.

## Reproducible gate and measurements

The workload creates a temporary Git repository with 50 files of 1,000 fixed 74-byte lines each, commits the base and edits every line. The real capture/store/rawdiff path yields **7,506,350 bytes / 100,250 lines**. Git and storage work finishes before timings; document preparation is timed separately. The test then removes executable lookup from PATH while processing 1,000 input and 1,000 resize/view events. The separate async test runs a cancellable fake command. No real fixture execution is authorized or needed for this framework test.

Stated usable budgets: preparation/warm raw view under 1 second, under 64 MiB total document-preparation allocations, each measured input+view/resize+view/quit update under 100 ms, under 256 KiB allocated per event, and actual PTY exit/restoration under 1 second. The test fails visibly if a budget is exceeded. Before accepting a failure on a supported host, investigate the workload; if the framework cannot meet these bounds, retain raw export and select a simpler paged renderer rather than dropping content.

Measured on macOS arm64, Apple M5 Max, 18 logical CPUs, Go 1.27.1; warmed tool/module/filesystem caches, freshly captured repository per test, no cold-disk or cross-machine claim:

| Check                                                   | Result                                     |
| ------------------------------------------------------- | ------------------------------------------ |
| Race-instrumented document preparation                  | 41.3 ms, 11,613,488 bytes total allocation |
| Maximum input + view / resize + view                    | 1.14 ms / 0.47 ms over 1,000 each          |
| Quit update                                             | 8.8 µs                                     |
| Mean allocation per input/resize event                  | 14,553 bytes                               |
| Non-race 120-column cached viewport benchmark           | 27,572 ns/op, 16,185 B/op, 285 allocs/op   |
| Active fake-job resize/input/quit                       | 0.41 ms                                    |
| Actual PTY quit/Ctrl-C/cancelled-context/job-error exit | 10.9–11.5 ms                               |
| Five-second terminal fuzz target                        | 1,501,230 executions, no failure           |

Memory figures are Go allocation deltas, **not process peak RSS**. They exclude capture/storage construction, which has its own budgets. Timing thresholds are a regression/evaluation gate, not a portable latency guarantee.

Core tests run without an interactive terminal. On macOS/Linux, automated PTY checks open a real pseudo-terminal, verify raw mode was entered, inject quit/Ctrl-C/cancellation and a hostile job error, check exact pre/post termios equality, and verify alternate-screen exit and cursor restoration in emitted bytes. Clipboard/title/hyperlink attack sequences must not appear in that stream. No visual screenshots or fake participant observations are evidence for these checks. Tests are part of the existing macOS/Linux Go CI gate; the measurements above are local macOS observations only.

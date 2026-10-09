# Terminal foundation decision

AFTER uses **Bubble Tea v1.3.10** for its local TUI. The evaluated v1 model/update/string-view API fits the first slice; v2 is not needed. `go.mod`/`go.sum` pin Bubble Tea and `uniseg v0.4.7`; `creack/pty v1.1.24` is test-only. No Bubbles component or generic UI/job framework is needed. This is a framework decision, not the finished TUI; see [TUI](TUI.md).

Run the existing gates:

```sh
task test:terminal
task terminal:bench
task terminal:fuzz
```

## Text and event-loop boundary

`internal/terminal` owns a bounded text/hex document index, untrusted-text sanitizer, fixed-SGR theme and Bubble Tea lifecycle support. Construct `NewDocument` during background preparation, not in `Update` or `View`. It copies the source once; keypresses and resizes reuse the index and render visible rows. Rendering and event updates do not access repositories, stores or processes.

All untrusted titles, patches and errors pass through `Line`. Control/format characters are visible escapes, including ESC, C1 CSI/OSC, carriage return, backspace, bidi controls and embedded newlines. OSC clipboard/title/hyperlink text cannot issue terminal commands. Tabs expand at four-column stops. Grapheme clusters preserve combining marks and wide-character selection; a leading combining cluster gets a dotted circle. Invalid UTF-8 becomes U+FFFD. Format controls such as emoji joiners are escaped. Bracketed paste does not invoke navigation or quit keys. Trusted UI labels and selection markers remain outside content rows; repository prose is never an evidence badge.

Background work runs as Bubble Tea commands, not inside the event loop. Results carry snapshot/request identities; stale results cannot change the current job state or replace a document. Search is cancellable, uses sanitized displayed text, and checks request/view identity before applying. Queries are capped at 512 sanitized bytes. The test fake command waits on cancellation while real event-loop input, resize and quit continue. Bubble Tea does **not** join commands: callers must clean up owned work. `Model.Context` is cancelled on quit; `Run` cancels it on every exit/error and returns errors only after terminal restoration.

## Display bounds and theme

| Limit                         |                                                          Bound |
| ----------------------------- | -------------------------------------------------------------: |
| Text/raw input                |                                                         16 MiB |
| Indexed text lines            |          250,000; exact raw bytes remain available in hex view |
| Source bytes examined per row |               4 KiB; long-line suffix is available in hex view |
| Viewport                      | 240 columns × 100 rows, clipped to the actual smaller terminal |
| Pan/render                    |                Display columns; never splits grapheme clusters |

A line-limit overflow retains its exact bytes and indexed prefix, with an explicit limitation; hex rows read directly by byte offset. Tiny terminals may show only a header, but quit remains available. Format controls are escaped rather than preserved. Width follows `uniseg`'s Unicode tables; terminal-specific ambiguous-width/font differences are not solved. `Run` does not print errors after restoring the terminal; callers must still sanitize diagnostics. SIGKILL, broken terminal devices and non-cooperative external jobs are outside the library's protection.

`terminal.Theme.Render` sanitizes and clips before applying a closed set of SGR: ANSI 16-color foreground, bright black for rules, bold, faint, reverse and reset, plus five fixed xterm-256 backgrounds (Diff added/removed tints and emphasis, and the selection bar) when the theme is rich. Callers cannot supply markup. Color is enabled unless `NO_COLOR` is nonempty or `TERM=dumb`; rich mode additionally requires `TERM` containing `256color` or `COLORTERM=truecolor`/`24bit`. `Theme.Bar` re-applies only the closed selection background after each trusted reset. Badges retain bracketed words in every mode; green evidence styling means only a current, complete, equal observation—not a reported pass or human decision. Search highlighting recognizes exactly this closed set when splitting styled rows.

## Regression gates and measurements

`task terminal:bench` builds a temporary Git capture with 50 files × 1,000 fixed 74-byte lines, edits each line and measures the captured view. The real raw patch is **7,506,350 bytes / 100,250 lines**; Git/store work finishes before model timings. It removes executable lookup from `PATH` while handling 1,000 inputs and 1,000 resizes. `task test:terminal` also tests a 100,000-line asynchronous search; input+view must stay below 100 ms / 256 KiB, with cancellation when the view changes.

| Gate                                 |                      Bound |
| ------------------------------------ | -------------------------: |
| Warm raw view / cached list          |                <1 s / <2 s |
| Document preparation                 | <1 s and <64 MiB allocated |
| Input+view, resize+view, quit update |               <100 ms each |
| Per event allocation                 |                   <256 KiB |
| PTY exit/restoration                 |                       <1 s |

The gate fails visibly rather than dropping content. These are regression thresholds, not portable latency promises. One retained local macOS arm64/M5 Max/Go 1.27.1 run measured race-instrumented document preparation at 41.3 ms / 11,613,488 bytes allocated; maximum input+view was 1.14 ms, resize+view 0.47 ms, quit 8.8 µs, and PTY exit/restoration 10.9–11.5 ms. A cached 120-column viewport benchmark measured 27,572 ns/op, 16,185 B/op, 285 allocs/op. These are Go allocation deltas, not process RSS.

Computed source diffs have a separate `task test:computed` gate. Its 10-file/20,000-source-line fixture checks source computation and background `Load` below one second and 64 MiB allocation, plus the same event/PTY limits. `task test:views` checks deterministic Overview, Changes and Diff at 120×40, 80×24 and 40×12; only `task test:views -- -update` regenerates goldens. Native CLI PTY tests cover report badges, inventory, documents and terminal restoration at 80×24 and 120×40, with and without `NO_COLOR`.

On macOS/Linux, PTY tests verify raw mode entry, exact pre/post termios equality, alternate-screen exit and cursor restoration after quit, Ctrl-C, context cancellation and hostile job errors. The tests prove terminal behavior, not user benefit or actual project execution. Measured values above are one local run, not a cross-machine guarantee.

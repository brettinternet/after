# Go test report importer v1

`internal/gotestreport.Import(io.Reader, Metadata)` imports stock `go test -json`
bytes without opening a project, running Go, fetching URLs, or following paths.
CLI import wiring belongs to AFTER-10; the current CLI still provides help/version
only. Report cards are data for later inspection, not runner receipts.

## Tested compatibility

Dialect `go-test-json-v1`, report schema 1, is tested with **Go 1.27.1** on
macOS/arm64. Checked-in JSONL fixtures and their deliberately synthetic source
live in `internal/gotestreport/testdata/`. Regenerate them explicitly with:

```sh
mise exec -- task fixtures:go-report
```

This developer-only target executes synthetic tests with module networking off.
It is never invoked by import or normal tests. It records `go version`, captures
passing/failing/skipped tests, nested subtests, paused/continued parallel tests,
interleaved packages with duplicate test names, package failure, build failure,
and a package run selecting no tests. An invalid timeout invocation captures
empty stdout (exit 2); an empty report does not establish success. Test/build
failure fixtures intentionally exit 1. Capture timestamps and scheduling vary
on regeneration. The nested module prevents deliberately failing fixture sources
from entering the product test/build graph.

Supported test actions: `start`, `run`, `pause`, `cont`, `output`, `pass`, `fail`,
`skip`; build actions: `build-output`, `build-fail` with `ImportPath`. Supported
fields are `Time`, `Action`, `Package`, `Test`, `Elapsed`, `Output`, `OutputType`
(empty, `frame`, or `error`), `FailedBuild`, and `ImportPath`. Output annotations,
elapsed time, and failed-build names do not create observations. Unknown fields,
actions, incompatible field combinations, or invalid field types are diagnosed,
not silently interpreted as a newer dialect. Other Go versions are not certified.
Benchmarks and fuzz exploration are not reconstructed into scenarios.

## Evidence and provenance

Each report retains the exact original byte count and SHA-256, caller-supplied
producer, import time, optional capture time, and optional explicit snapshot
SHA-256 binding. The importer validates digest syntax, not snapshot existence or
producer honesty. The caller must resolve a supplied snapshot through the local
store when integrating the command. With or without that binding, applicability
is **unknown**. Event timestamps are retained as reported first/last event times,
not trusted capture or AFTER execution times. Missing timestamps stay absent.

Each card has a package/test/build scope and attempt number. Identical test names
in different packages remain separate. A new `run`/`start` after completion opens
a new attempt; conflicting events after completion are diagnosed rather than
overwriting prior status. Package pass is not proof that any tests ran. Build
failure is a build card, not a fabricated failing runtime test. A card without a
terminal event has report status `none` and a `missing_completion` diagnostic.

Every card has producer `importer`, kind `reported`, applicability `unknown`,
execution `not_run`, and comparison `not_compared`. Reported pass/fail/skip is
separate from AFTER execution. Inputs, expected values, and effects are explicitly
`unavailable`; test names and output are never mined for behavioral observations.
Even a complete report does not support equality, freshness, or observed badges.

## Bounds and recovery

- Read at most 8 MiB + one sentinel byte. Oversize or reader errors fail the import
  entirely with a fixed error; no misleading partial original digest is returned.
- Lines over 64 KiB, malformed/truncated JSON, unsupported events, and invalid
  UTF-8 produce line-numbered fixed diagnostic codes. Later valid lines survive.
- At most 4,096 cards and 16 KiB output per card are retained. Additional cards
  and truncated output are explicitly diagnosed; output truncates on a UTF-8
  boundary. The original digest always covers the whole accepted input.
- At most 100 diagnostics are retained with an exact suppressed-diagnostic count.
  Any diagnostic makes report completeness `incomplete`.
- A final valid JSON object without a newline is accepted. Missing terminal
  events and empty reports remain incomplete. A valid prefix cannot prove that
  the producer supplied its entire original stream.

Callers own reader deadlines/cancellation. The importer provides bounded byte
consumption, not a deadline for a blocking reader. It is not a Go event-sequence
or producer-authenticity validator.

All strings remain untrusted data, including producer, package, test names and
output. JSON encoding escapes terminal controls; a future terminal renderer must
sanitize every string before display. No importer function renders raw text or
interprets an output URL/path. The API returns report data; it does not write the
original bytes to disk. Later storage integration must use private store artifact
redaction and distinguish the original digest from any redacted artifact digest.
Do not commit real project reports, credentials, or participant data.

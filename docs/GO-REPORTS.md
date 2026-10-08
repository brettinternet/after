# Go test report importer v1

`internal/gotestreport.Import(io.Reader, Metadata)` parses stock `go test -json` as reported data. It opens no project, runs no Go command, fetches no URLs and follows no paths. An imported report is not a runner receipt or behavioral observation.

The shared schema also supports [JUnit XML](JUNIT-REPORTS.md). CLI format selection is explicit (`--format go-test-json` or `--format junit`) or based on an unambiguous input prefix, never the filename.

## Import

```sh
go test -json ./... | ./bin/after import --project /work/payment
./bin/after import report.jsonl --producer 'go1.27.1 on linux/amd64' \
  --snapshot CANDIDATE_ID --project /work/payment
```

The CLI accepts a file, `-`, or piped stdin. Without `--snapshot`, it captures the working tree with normal capture rules and writes a capture event. A supplied snapshot is a caller binding, not proof that the tests ran on it. The default binding is also not proof. Omit `--producer` for no producer claim; `--producer` and `--captured-at` are unauthenticated caller claims. A supplied snapshot digest is syntax-checked but not resolved by the importer.

Every card has this evidence state:

| Field                            | Value                      |
| -------------------------------- | -------------------------- |
| Producer / kind                  | `importer` / `reported`    |
| Applicability                    | `unknown`                  |
| Execution / comparison           | `not_run` / `not_compared` |
| Inputs, expected values, effects | `unavailable`              |

A reported `pass`, `fail` or `skip` stays separate from AFTER execution. Test names, output and elapsed times never establish inputs/effects, equality or freshness. Package pass does not prove any tests ran. Build failure is a build card, not a fabricated runtime failure. An event stream without terminal status has report status `none` and a `missing_completion` diagnostic; empty input does not establish success. Event times remain reported first/last event times, not trusted capture or AFTER run times.

## Supported dialect

The importer accepts dialect `go-test-json-v1` (report schema 1), tested with Go 1.27.1 on macOS/arm64. Checked-in synthetic JSONL fixtures live in `internal/gotestreport/testdata/`; their sources are nested so deliberately failing fixtures stay outside product tests/builds. Regenerate only with:

```sh
task fixtures:go-report
```

That developer target runs synthetic tests with module networking disabled; import and normal tests never invoke it. Fixtures cover passing/failing/skipped and nested/parallel tests, duplicate names in separate packages, package/build failure, no tests, and an invalid-timeout command with empty output.

Supported test actions are `start`, `run`, `pause`, `cont`, `output`, `pass`, `fail`, `skip`; build actions are `build-output` and `build-fail` with `ImportPath`. Supported fields: `Time`, `Action`, `Package`, `Test`, `Elapsed`, `Output`, `OutputType` (empty, `frame` or `error`), `FailedBuild` and `ImportPath`. Unknown fields/actions, invalid types and incompatible fields produce diagnostics; they are not interpreted as a newer dialect. Benchmarks and fuzz exploration are not reconstructed as scenarios. Other Go versions are not certified.

Each card is scoped to package/test/build and has an attempt number. A new `run`/`start` after completion begins another attempt. Conflicting events after completion are diagnosed, not used to overwrite status.

## Limits and trust

| Input or output          |                                        Limit |
| ------------------------ | -------------------------------------------: |
| Complete report input    | 8 MiB; reads at most one extra sentinel byte |
| JSONL line               |                                       64 KiB |
| Cards                    |                                        4,096 |
| Retained output per card |                       16 KiB, UTF-8 boundary |
| Diagnostics              |             100, plus exact suppressed count |

Oversize input or reader errors fail the import entirely; no partial digest is returned. Malformed lines, unsupported events and invalid UTF-8 produce fixed line-numbered codes; later valid lines survive. A valid final JSON line without newline is accepted. Missing completions, empty reports, truncation or any diagnostic make the report incomplete. A valid prefix cannot prove the producer supplied the full original stream. Callers own reader deadlines/cancellation; a blocking reader has no importer deadline.

The report stores original byte count and SHA-256, optional producer/snapshot claims, import time and optional capture-time claim. The API does not write the original stream. The CLI stores a bounded report artifact under store redaction policy; original and retained artifact digests are distinct. Malformed stored reports remain available as raw bytes with a limitation, not guessed cards.

All report strings are untrusted, including names, producer and output. JSON encoding escapes controls; the shared CLI/TUI Card renderers sanitize and bound display. The importer does not render text or interpret URLs/paths. Never commit real project reports, credentials or participant data.

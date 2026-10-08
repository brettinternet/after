# JUnit XML report importer v1

`internal/gotestreport.ImportJUnit(io.Reader, Metadata)` imports the `junit-xml-v1` dialect into the same schema-1 reported cards as [Go test JSON](GO-REPORTS.md). It uses only Go's standard XML decoder. It runs no producer, opens no embedded path, resolves no entity, fetches no schema/URL, and does not execute project code.

```sh
after import report.xml --format junit --producer 'pytest 8.4.2'
pytest --junitxml=report.xml  # separate, explicitly executed producer command
after import report.xml --snapshot CANDIDATE_ID
```

`--format` accepts `junit`, `go-test-json`, or `auto` (default). Auto chooses XML only for a leading `<`, or Go JSON only for a leading `{`, after whitespace (and an XML UTF-8 BOM). Other input fails with an explicit-format fix. File extensions, producer claims, project files and XML schema hints never select parsers. Selection is not validation: a recognized prefix with malformed content produces an incomplete report, not a successful test. Empty auto input retains the existing Go-import error. Explicit JUnit empty input or an empty suite is stored with `no_cases`, zero test cards, and incomplete status. Import's exit 0 means the report was stored, not that its tests passed or its content is complete.

The TUI's explicit `i` action on `review --import-file FILE` uses auto selection too. For an unrecognized format, first use the CLI with `--format`. Both dialects are discoverable in review, history, status, completion and inspection.

## Evidence contract

Every card is `importer` / `reported` / `unknown` / `not_run` / `not_compared`. Inputs, expected values and effects are explicitly `unavailable`. Producer, snapshot and capture-time metadata are unauthenticated caller claims; the import-time working-tree binding is not proof of where tests ran. Test names, assertion text, timing, properties and reported success do not establish behavior, freshness or trusted provenance.

Closed testcases without an outcome child report `pass`; `failure` and `error` report `fail`; `skipped` reports `skip`. The failure/error distinction is retained in labeled output. Conflicting outcomes, missing names, or unsupported content inside a case yield report status `none`, not inferred success. A truncated unclosed case is not retained. A completed case in an incomplete document remains only a reported outcome; document completeness stays visible.

Suite paths (quoted components) and optional class names identify a card's `package`; `test` keeps the testcase name. Duplicate names in different suites remain distinct; repetitions with the same identity receive increasing attempt numbers. No case is merged away. Case stdout/stderr and outcome messages are retained in labeled output in document order. Suite-level output becomes a separate package-scope card with status `none`, never copied onto tests or used to imply success. Suite summary counts are not reconstructed as tests or checked as an assertion of coverage. Reported time attributes are not promoted to trusted event times.

## Supported XML shape

A single `testsuites` or `testsuite` root; nested `testsuites`, nested `testsuite`, and `testcase` within a suite are supported. Within cases: `failure`, `error`, `skipped`, `system-out`, `system-err`, and `properties`/`property`. Suites also allow output and properties. Text/CDATA and XML's five predefined/numeric character references are decoded as data. Known metadata properties are ignored, not interpreted.

Allowed attributes:

| Element                       | Attributes                                                                                                    |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `testsuites`, `testsuite`     | name, tests, failures, errors, skipped, disabled, time, timestamp, hostname, id, package, assertions, version |
| `testcase`                    | name, classname, time, file, line, assertions                                                                 |
| `failure`, `error`, `skipped` | message, type                                                                                                 |
| `property`                    | name, value                                                                                                   |

Surefire's `xmlns:xsi` declaration and `xsi:noNamespaceSchemaLocation` on suite elements are accepted as inert metadata; the referenced schema is **never fetched or validated**. Other namespace-qualified elements/attributes, XInclude, producer-specific rerun/flaky extensions and unknown attributes/elements are unsupported. This is not a universal JUnit specification or certification of every producer/version.

All DTD/directive tokens are rejected, including internal or external entity declarations. External entity references cannot resolve; undefined references are malformed XML. Non-XML processing instructions (including stylesheets) are diagnosed, never acted on. Unsupported well-formed subtrees are skipped with a diagnostic while valid sibling cases survive. Malformed XML, structural/attribute bounds and multiple roots stop parsing and retain already closed cases only; the importer cannot safely resynchronize arbitrary broken XML. It never silently claims later cases were imported.

## Bounds and diagnostics

| Resource                                   |                                  Limit |
| ------------------------------------------ | -------------------------------------: |
| Input                                      | 8 MiB; at most one extra sentinel byte |
| Element nesting                            |                                     32 |
| Elements                                   |                                100,000 |
| Attributes per element                     |                                     64 |
| Attribute name/namespace/value bytes       |                        16 KiB combined |
| Character-data token                       |                                 64 KiB |
| Cards                                      |                                  4,096 |
| Card package identity                      |                                 16 KiB |
| Retained identities / output across report |                             4 MiB each |
| Output per card                            |                 16 KiB, UTF-8 boundary |
| Diagnostics                                |              100 plus suppressed count |

Oversize/unreadable input fails outright with no partial digest. Parsing is bounded by the input cap even before token-level checks. Callers own reader deadlines/cancellation. Any diagnostic makes the report incomplete. Codes are fixed strings with 1-based lines, never raw decoder errors containing attacker text: `invalid_xml`, `forbidden_directive`, `unsupported_processing_instruction`, `unsupported_element`, `unsupported_attribute`, `duplicate_attribute`, `unsupported_text`, `text_outside_root`, `multiple_roots`, `missing_test_name`, `conflicting_outcomes`, `no_cases`, `xml_structure_limit`, `attribute_limit`, `text_limit`, `card_limit`, `identity_limit`, `output_limit`.

The report records the original byte count and SHA-256, not original XML bytes. CLI storage retains the normalized JSON report through existing private-artifact redaction/bounds. The final encoded artifact must also fit the store's 16 MiB limit. Text remains untrusted; existing CLI/TUI rendering sanitizes controls and limits display. Digests are not signatures. Never commit real reports, credentials or participant data.

## Captured fixtures and verification

Synthetic producer captures are in `internal/gotestreport/testdata/junit/`:

- `pytest.xml`: pytest **8.4.2**, pass, assertion failure, fixture-setup error, skip, duplicate short test names in different classes, stdout/stderr.
- `pytest-empty.xml`: pytest **8.4.2**, all tests deselected; no outcome inferred.
- `vitest.xml`: Vitest **3.2.4**, pass, assertion/runtime failures, skip, nested describe names, stdout/stderr. Vitest flattens describe nesting into testcase names.
- `surefire.xml`: Maven Surefire **3.5.4**, JUnit Jupiter **5.13.4**, Maven **3.9.11**, Temurin **21.0.8+9**; pass, failure, error, skip and CDATA output.

These are actual captured XML, not hand-authored expected reports. Capture normalizes the temporary source directory to `/synthetic/junit` and hostname to `synthetic-host`; it removes Surefire's JVM/system-property block to avoid committing machine paths and ambient settings. Test content/outcomes, XML structure, timestamps and timings otherwise remain producer output. Sources and the capture script are committed. Supplemental parser tests construct literal nested suite trees, duplicate names in separate suites, unknown extensions, malicious XML and boundary violations; those tests are not claimed as producer captures.

Regenerate only with `task fixtures:junit`. This explicitly runs synthetic failing tests and downloads producer dependencies; it requires Bun, uv, and separately installed `mise install java@temurin-21.0.8+9 maven@3.9.11`. It is not part of import, normal checks or CI. No producing toolchain, Docker or network is needed for `task test:reports` or the Go suite. The focused checks exercise retained fixture outcomes, parsing bounds, trust labels, format selection, storage/inspection, and sanitized TUI cards.

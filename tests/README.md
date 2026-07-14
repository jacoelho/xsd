# XSD 1.0 Harness

Standalone XSD 1.0 validation corpus plus a Go test runner. This directory contains data, expected results, and `harness_test.go`; it does not require Xerces-J source code or the original source checkouts.

## Contents

- Cases: 14548
- W3C cases: 14414
- Xerces-J cases: 13
- Project cases: 121
- Schema checks: 14498
- Schema-backed instance validations: 25128
- Expected-invalid schema instance skips: 21
- Explicit schema-less instance skips: 50
- Unique manifest file references: 39754

## Files

- `manifest.json`: source of truth for expected results and paths.
- `unsupported.txt`: sorted allowlist of unsupported-feature skips.
- `harness_test.go`: Go test runner for this corpus.
- `corpus/`: all schema, XML, and auxiliary files referenced by the manifest.
- `corpus/w3c`: copied W3C files with original relative layout preserved.
- `corpus/xerces-j`: selected Xerces-J XSD/XML validation fixtures.
- `corpus/project`: project-owned regression fixtures.

## Corpus Contract

- Every manifest path is relative to this directory.
- Every referenced schema, instance, and auxiliary file resolves under `corpus/`.
- `source.w3cSuitePath` and `source.xercesJPath` are provenance only. Consumers MUST NOT need those paths to run the corpus.
- Per-case `oracle.xerces` entries document expected Xerces-J deviations from the spec expectation.
- This artifact MUST NOT contain Java classes, jars, shell scripts, or Go exclusion manifests.

## Go Test Runner

Run the full harness:

```sh
go test ./tests
```

Run one source:

```sh
go test ./tests -run '^TestHarness/project'
go test ./tests -run '^TestHarness/xerces-j'
go test ./tests -run '^TestHarness/w3c'
```

## Runner Contract

A runner SHOULD read `manifest.json`, compile each case schema document set, then validate each listed instance against that schema. Expected values are `valid` or `invalid`. If a consumer is comparing against Xerces-J, use `oracle.xerces.expected` when present; otherwise use the case expected value.

Every entry in `schema.documents` is a compilation root, in manifest order.
Entries listed only in `files` with role `dependency` are copied dependencies
used through relative schema locations and are never promoted to roots.
Instance file paths are independent validation inputs.

Cases without a `schema` member require schema-less or instance-directed schema
assessment, which the precompiled `Engine` contract deliberately does not
perform. The Go runner exposes each such instance as an explicit skipped
subtest; it does not invent roots from instance hints.

`unsupported.txt` is part of the test oracle. Each line is tab-separated:

```text
schema<TAB>source<TAB>caseID<TAB>code
instance<TAB>source<TAB>caseID<TAB>instanceName<TAB>code
```

The file MUST stay sorted and unique. New unsupported skips fail until added deliberately; stale entries fail after the full harness passes.

## Export Evidence

- Manifest closure check: 39754 unique referenced files, all present under `corpus/`.

# XSD 1.0 Harness

Standalone XSD 1.0 validation corpus plus a Go test runner. This directory contains data, expected results, and `harness_test.go`; it does not require Xerces-J source code or the original source checkouts.

## Contents

- Cases: 14477
- W3C cases: 14414
- Xerces-J cases: 13
- Project cases: 50
- Schema checks: 14427
- Instance checks: 25132
- Manifest file references: 39614

## Files

- `manifest.json`: source of truth for expected results and paths.
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

A schema document with role `principal` is the entry schema. Files listed with role `dependency` are copied dependencies used through relative schema locations. Instance file paths are independent validation inputs.

## Export Evidence

- Manifest closure check: 39614 referenced files, all present under `corpus/`.

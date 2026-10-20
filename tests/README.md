# Test And Conformance Harness

Standalone XSD 1.0 validation corpus plus a Go test runner. This directory contains data, expected results, and `harness_test.go`; it does not require Xerces-J source code or the original source checkouts.

Use the smallest evidence surface that proves the changed contract:

| Question | Primary evidence | Wider guard |
| --- | --- | --- |
| One package-owned rule or representation | Co-located package test, fuzz target, or benchmark | Callers of that package interface |
| Public compile, validation, source, session, or diagnostic behavior | Root external-package tests and `external_api_smoke_test.go` | Full corpus and API-shape tests |
| XSD/XML conformance | A minimal project-owned corpus regression | Full `TestHarness` and unsupported allowlist checks |
| Package direction, public/internal seam, state ownership, or borrowed lifetime | `phase_*`, `root_public_shape_test.go`, `schema_build_boundary_test.go`, or `stream_boundary_test.go` | `go test ./...` |
| Large streaming time or peak memory | Focused package benchmark first | Opt-in `TestLargeXMLLintBenchmark` with frozen inputs |
| WASM, worker, page, or local-server lifecycle | Owning `cmd/wasmxsd`, `docs/js`, or `cmd/xsdweb` tests | Browser integration for browser routes |

Behavior tests belong at the owning module's interface. Add a white-box test
only when a private corruption or lifetime state cannot be exercised through
that interface. A regression fixture should isolate one semantic rule and name
the violated contract, not preserve incidental control flow.

## Files

- `manifest.json`: source of truth for expected results and paths.
- `unsupported.txt`: sorted allowlist of unsupported-feature skips.
- `harness_test.go`: Go test runner for this corpus.
- `corpus/`: all schema, XML, and auxiliary files referenced by the manifest.
- `corpus/w3c`: copied W3C files with original relative layout preserved.
- `corpus/xerces-j`: selected Xerces-J XSD/XML validation fixtures.
- `corpus/project`: project-owned regression fixtures.

The manifest and allowlist own inventory. Do not copy their changing counts into
documentation or plans; derive them when a revision-scoped measurement needs
them.

## Project Fixture Organization

Project fixtures are grouped by the contract they exercise:

| Directory under `corpus/project/` | Contract |
| --- | --- |
| `datatypes/` | Value rules, subdivided into numeric, temporal, string, binary, list, union, name, URI, QName, notation and regex families |
| `attributes/` | Attribute declarations, uses, groups, defaults, fixed values and wildcard processing |
| `elements/` | Element declarations, XSI selection, nil and value constraints |
| `content-models/` | Particles, model groups, derivation, attribution and substitution |
| `identity/` | ID/IDREF ownership, keys, unique constraints, keyrefs and XPath fields |
| `namespaces/` | Expanded names, qualification, bindings and namespace-sensitive consumers |
| `composition/` | Includes, imports, schema defaults and source assembly |
| `schema-syntax/`, `xml/` | Schema representation and XML input admission |
| `limits/` | Deliberately bounded implementation limits |

Datatype families own their value-specific cases. Shared constructor, facet,
applicability and precision groups keep cross-family parameter grids together;
splitting those grids by concrete type would obscure the comparison.

A case has one physical owner. Cross-cutting cases reference their related
source conditions in `source.conditions`; they do not duplicate fixture bodies.
New source-grounded cases also identify their specification clauses in
`source.specification`. These fields describe provenance, not another oracle.

Case IDs are stable and independent of directory paths. Moving an existing
fixture does not rename its test or change its expectation. Harness filters
match manifest case IDs, not directories. Upstream W3C and Xerces-J layouts
remain unchanged.

Keep each case's schema closure together so relative include/import paths
survive a move. Update the manifest's file paths and `source.sourceFile` when
moving a project case. Reuse the existing closure and integrity tests to check
the result.

For a new discriminator, pair a valid control with an isolated opponent where
the rule permits it. An instance test requires a valid compiled schema; a
schema-rejection case supplies no instance-validation evidence. Keep XML
admission, unsupported features, resource limits and semantic rejection
distinct. An unresolved source question or unavailable output is an explicit
coverage limit, not a reason to derive an expectation from current behavior.

## Corpus Contract

- Every manifest path is relative to this directory.
- Every referenced schema, instance, and auxiliary file resolves under `corpus/`.
- `source.w3cSuitePath` and `source.xercesJPath` are provenance only. Consumers MUST NOT need those paths to run the corpus.
- Per-case `oracle.xerces` entries retain expected Xerces-J results and reasons
  as provenance. They may match a later corrected project expectation; they do
  not establish a fresh validator run or redefine XSD validity.
- Each schema or instance `expected` is the effective project expectation. The
  `source` metadata and per-fixture `status` preserve upstream provenance
  without overriding it.
  When a project expectation corrects an upstream W3C result,
  `schema.oracle.w3c` records the original expectation, reason, and
  specification reference. Oracle metadata is explanatory; consumers do not
  merge it into the effective expectation or infer a Xerces-J result.
  An instance's `oracle.project` likewise records a corrected original project
  expectation. Existing `internalExpected` values remain historical extraction
  metadata. A legacy test name containing `valid` or `invalid` is not an oracle;
  preserve its identity when correcting `expected`.
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

Upstream cases that need additional schema documents supply those documents as
explicit manifest roots. Recovered W3C companion files retain the layout and
exact bytes from the manifest-pinned upstream revision. Instance location hints
never supply components or dynamically promote files to compilation roots.

The checked-in `manifest.json` is the authoritative expectation source. The
extractor named in its metadata is not part of this repository; any future
generator must preserve explicit project corrections such as `oracle.w3c`
entries.

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

- Manifest closure is derived from `manifest.json` and verified by `harness_manifest_test.go`.

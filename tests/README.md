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

## Cross-validator Diagnosis

For XSD/XML conformance investigations and new or changed corpus cases, compare
the minimal discriminator and its valid/opposing controls across these four
independent implementations:

| Implementation | Validation interface | Oracle key |
| --- | --- | --- |
| Java Xerces-J | Apache `org.apache.xerces.jaxp.validation.XMLSchemaFactory` | `xerces` |
| .NET XSD | `System.Xml.Schema.XmlSchemaSet` and schema-validating `XmlReader` | `dotnet` |
| Python xmlschema | `xmlschema.XMLSchema10`, explicitly using XSD 1.0 | `xmlschema` |
| libxml2 | `xmllint` or a binding with its actual libxml2 version recorded | `libxml2` |

Python `lxml` uses libxml2 and counts as that implementation, not as an
independent Python xmlschema result.

1. Freeze the schema roots, dependency closure, instance bytes and relevant
   options. Use XSD 1.0 and XML 1.0. An adapter that cannot represent the same
   source set must report the limitation rather than silently rewriting inputs
   or omitting roots.
2. Run all four using installed runtimes and local dependencies. Record an
   unavailable implementation and its concrete reason; an omitted run is not
   agreement. Keep schema acquisition offline: allow only the explicit closure,
   block network protocols and external entities, and preserve locationless
   imports without inventing schema documents. Record unsupported security
   settings and the mechanism that enforces the same boundary.
   Verify that resolved include/import URIs select the intended documents;
   choosing a different allowlisted document is still an adapter failure.
3. Isolate cases with fresh schema factories/sets and fresh instance validators.
   Use fresh processes when provider globals still leak between cases. Clear
   diagnostics between phases. Record schema compilation separately from
   instance validation. Schema rejection blocks its instances; it does not make
   them independently invalid. Keep unsupported features, resource limits,
   harness failures and unavailable observations distinct from validity.
   For unexpected provider exceptions, reproduce with supported validation APIs
   and valid/invalid controls before assigning a cause. Record yielded validation
   errors, iterator completion and terminal exceptions separately. An unexpected
   exception leaves the run incomplete even if validation errors were yielded.
4. Retain implementation/runtime versions, commands, options, fixture identities
   and raw diagnostics in revision-scoped evidence. Reproduce each disagreement
   with the smallest opposing controls and diagnose specification, processor,
   source-closure or harness differences before changing behavior.
5. Derive effective corpus expectations from the local specifications, approved
   corrections and documented qualifications. Agreement is diagnostic evidence,
   not a majority-vote replacement for those sources. Preserve measured provider
   disagreements as implementation-specific oracle metadata without overwriting
   the project expectation or fabricating results for blocked phases.
   Where specification text, adopted corrections and accepted W3C expectations
   leave a reading doubtful, the interpretation record applies the
   [processor tie-break](../docs/spec/INTERPRETATIONS.md#processor-tie-break).

Historical releases and modified provider copies are separate diagnostic
profiles. Record their selection reason, versions, source changes and hashes;
run the complete affected packet with the same offline inputs and controls.
Their results never replace an unavailable stock result or enter that stock
profile's oracle metadata. An exception-routing patch can explain a provider
failure without independently establishing the instance's required validity.

The comparison is complete when every implementation has a measured result or
an explicit unavailable/blocked reason, and every disagreement has a diagnosis
or a stated unresolved boundary. Report remaining coverage limits to the user.

## Corpus Contract

- Every manifest path is relative to this directory.
- Every referenced schema, instance, and auxiliary file resolves under `corpus/`.
- `source.w3cSuitePath` and `source.xercesJPath` are provenance only. Consumers MUST NOT need those paths to run the corpus.
- Schema or instance `oracle.<implementation>` entries retain provider results
  and reasons as provenance, using the keys above. They may match a later
  corrected project expectation; metadata alone does not establish a fresh
  validator run or redefine XSD validity. Actual comparisons follow
  [Cross-validator Diagnosis](#cross-validator-diagnosis).
- Each schema or instance `expected` is the effective project expectation. The
  `source` metadata and per-fixture `status` preserve upstream provenance
  without overriding it.
  When a project expectation corrects an upstream W3C result,
  `schema.oracle.w3c` or `instance.oracle.w3c` records the original expectation,
  reason, and specification reference. Oracle metadata is explanatory; consumers do not
  merge it into the effective expectation or infer a Xerces-J result.
  A schema or instance's `oracle.project` likewise records a corrected original project
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

A runner SHOULD read `manifest.json`, compile each case schema document set,
then validate each listed instance against that schema. Expected values are
`valid` or `invalid`. For provider comparisons, use the phase's
`oracle.<implementation>.expected` when present; otherwise compare the measured
result against the project expectation. A provider phase recorded with
`status: "blocked"` and no `expected` is an execution limitation, not a validity
expectation. In particular, schema rejection blocks its instance phases.
Provider expectations remain separate from the effective project result.

The Go runner rejects internal diagnostics anywhere in a compilation or
validation error tree, including wrapped causes and aggregate branches. An
expected-invalid result, a matching `errorCode` (even `internal.invariant`), or
an allowlisted unsupported diagnostic cannot make an internal failure pass or
skip. Other rejection expectations retain their existing meaning, including
malformed XML and explicit resource-limit cases.

Every entry in `schema.documents` is a compilation root, in manifest order.
Entries listed only in `files` with role `dependency` are copied dependencies
used through relative schema locations and are never promoted to roots.
Instance file paths are independent validation inputs.

Upstream cases that need additional schema documents supply those documents as
explicit manifest roots. Recovered W3C companion files retain the layout and
exact bytes from the manifest-pinned upstream revision. Instance location hints
never supply components or dynamically promote files to compilation roots.

Project-owned auxiliary namespace schemas can close an upstream case's missing
XML/XLink declarations as explicit roots. Their source metadata must identify
their limited scope; they are not complete copies of the upstream vocabulary
schemas. Supplying these roots does not authorize network loading.

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

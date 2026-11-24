# Test And Conformance Harness

Standalone XSD 1.0 validation corpus plus a Go test runner. This directory contains data, expected results, and `harness_test.go`.

Use the smallest evidence surface that proves the changed contract:

| Question | Primary evidence | Wider guard |
| --- | --- | --- |
| One package-owned rule or representation | Co-located package test, fuzz target, or benchmark | Callers of that package interface |
| Public compile, validation, source, session, or diagnostic behavior | Root external-package tests and `external_api_smoke_test.go` | Full corpus and API-shape tests |
| Harness-expressible XSD/XML conformance | Project-owned corpus cases, each minimal, covering every edge case | Full `TestHarness` and unsupported allowlist checks |
| Package direction, public/internal seam, state ownership, or borrowed lifetime | `phase_*`, `root_public_shape_test.go`, `schema_build_boundary_test.go`, or `stream_boundary_test.go` | `go test ./...` |
| Large streaming time or peak memory | Focused package benchmark first | Opt-in `TestLargeXMLLintBenchmark` with frozen inputs ([reproduction](../docs/BENCHMARKS.md)) |
| WASM, worker, page, or local-server lifecycle | Owning `cmd/wasmxsd`, `web/js`, or `cmd/xsdweb` tests | Browser integration for browser routes |

Behavior tests belong at the owning module's interface. Add a white-box test
only when a private corruption or lifetime state cannot be exercised through
that interface. A regression fixture should isolate one semantic rule and name
the violated contract, not preserve incidental control flow.

## Files

- `manifest/`: the hand-written inventory, the source of truth for cases, expected results and paths. `manifest/index.json` holds the header and the explicit, ordered shard list; `internal/corpus` concatenates the shards in that order and rejects a case filed in a shard other than the one `corpus.ShardName` assigns by upstream test set or project area.
- `oracles/`: the measured cross-validator observations, written only by `make oracle-measure` and never edited by hand. Each project shard has one observation file of sorted rows, the measured W3C cases share `oracles/w3c.tsv`, and `oracles/profile.json` records the provider versions, the digest of the oracle image inputs under `tools/oracles` and the date of the measurement. A partial measurement keeps the profile and must match it.
- `unsupported.txt`: sorted allowlist of unsupported-feature skips.
- `harness_test.go`: Go test runner for this corpus.
- `harness_manifest_test.go`: verifies the manifest closure: every referenced file exists and every corpus file is referenced.
- `oracle_observations_test.go`: verifies the oracle rules of the [Corpus Contract](#corpus-contract) against the observations.
- `internal/compilework/`: the command that `content_model_compile_work_test.go`, `attribute_use_audit_work_test.go` and `shared_attribute_group_work_test.go` build with coverage counters to count the statements and charged steps of one compilation, compiled or rejected.
- `oracle-backlog.txt`: sorted, unique IDs of measured cases with a provider difference that no record owns yet.
- `corpus/`: all schema, XML, and auxiliary files referenced by the manifest.
- `corpus/w3c`: copied W3C files with original relative layout preserved.
- `corpus/xerces-j`: selected Xerces-J XSD/XML validation fixtures.
- `corpus/project`: project-owned regression fixtures.

The manifest and allowlist own inventory. Do not copy their changing counts into
documentation; derive them when a revision-scoped measurement needs
them.

## Project Fixture Organization

Project fixtures are grouped by the contract they exercise:

| Directory under `corpus/project/` | Contract |
| --- | --- |
| `datatypes/` | Value rules, subdivided into numeric, temporal, string, list, union, name, URI, QName, notation and regex families |
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

Cover every edge case of the affected rule that no existing case covers.
Depending on the rule, edge cases include boundary values, lexical and
whitespace variants, namespace variants, derivation composition, absent and nil
content, and inputs the rule rejects in its own phase. For a new discriminator,
pair a valid control with an isolated opponent where the rule permits it. An
instance test requires a valid compiled schema; a schema-rejection case
supplies no instance-validation evidence. Keep XML
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
2. Run all four with [`tools/oracles/run.sh`](../tools/oracles/run.sh)
   (`--help` documents its arguments; `make oracles` prebuilds the pinned
   image). Pass every `schema.documents` root in manifest order and a `--root`
   covering the dependency closure, then record the provider versions it
   reports with the results. The image runs without network on a read-only
   mount of the root. The Xerces-J, .NET and xmlschema adapters report
   non-file references and documents outside the root as `blocked`; libxml2
   blocks network references but can read image files outside the root, and
   accepts one root, reporting `blocked` for more. Record an unavailable implementation and its
   concrete reason; an omitted run is not agreement. Preserve locationless
   imports without inventing schema documents. The mount is coarser than the
   explicit closure, so verify from the diagnostics that resolved include/import
   URIs selected the intended documents; choosing a different file under the
   root is still an adapter failure.
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
4. Report implementation/runtime versions, commands, options, fixture
   identities and raw diagnostics with the change, scoped to its revision. The
   repository retains only the observation rows and their profile.
   Reproduce each disagreement with the smallest opposing controls and diagnose
   specification, processor, source-closure or harness differences before
   changing behavior.
5. Record measured results with `make oracle-measure ORACLE_CASES='<case-id>...'`
   for every added or changed measured case (without `ORACLE_CASES` it
   re-measures every measured case), record each difference and derive
   expectations as the [Corpus Contract](#corpus-contract) defines.

Historical releases and modified provider copies are separate diagnostic
profiles. Record their selection reason, versions, source changes and hashes;
run the complete affected packet with the same offline inputs and controls.
Their results never replace an unavailable stock result or enter the
observation rows. An exception-routing patch can explain a provider
failure without independently establishing the instance's required validity.

The comparison is complete when every implementation has a measured result
(validity or `unassessed`) or an explicit unavailable/blocked reason, and every disagreement has a diagnosis
or a stated unresolved boundary. Report remaining coverage limits to the user.

## Corpus Contract

- Every manifest path is relative to this directory.
- Every referenced schema, instance, and auxiliary file resolves under `corpus/`.
- `source.w3cSuitePath` and `source.xercesJPath` are provenance only. Consumers MUST NOT need those paths to run the corpus.
- A phase is the schema phase, named `schema`, or an instance phase, named by
  its `testName`, which is unique in its case. A phase's optional `rule` names
  the rule that decides its project expectation once: `specRef`, a
  specification URL or a repository anchor such as an interpretation record,
  and `clause`, its label.
- Cross-validator observations live in `oracles/`, not in the inventory. A
  row is `case-id<TAB>phase<TAB>provider<TAB>verdict<TAB>message`, using the
  provider keys above: exactly the verdict and first diagnostic that
  `tools/oracles/run.sh` printed, with paths relative to the mounted root.
  `schema-valid`, `schema-invalid`, `valid` and `invalid` are validities;
  `unassessed` means the provider ran but produced no validity outcome, such
  as .NET assessing an undeclared root laxly; `blocked` is an execution
  limitation, and an `error` verdict reads as `blocked`. None of the last three
  is a validity. Rows record observations, never expectations: a measured
  disagreement never overwrites the project expectation.
- The measured cases are every project case and every W3C case with a phase
  whose `oracle.w3c` expectation differs from its effective `expected`: the
  cases where the corpus departs from the upstream result, so the providers'
  results on them need an owner. Other W3C cases and the Xerces-J cases have
  no rows. `corpus.Case.Measured` is the one membership predicate.
- A case is measured when its schema phase has a row for each of
  `xerces`, `dotnet`, `xmlschema` and `libxml2`, and each instance phase has a
  row for every provider whose schema-phase verdict is `schema-valid`, and for
  no other; a provider that rejected or could not run the schema has no
  instance rows. The measurement command produces exactly these rows, and
  every measured case has them. `TestMeasuredPhasesHaveObservations` enforces
  the rule and rejects rows that name no such phase and provider.
- A row whose validity differs from the phase's project expectation has an
  entry under the phase's `differences`, keyed by provider. Instance phases of
  a case whose schema the project expects invalid have no expectation to differ
  from and carry none. The entry is `{"record": "<name>"}`, naming a
  `docs/spec/INTERPRETATIONS.md` record (`I-NNN`) or the `<a id>` anchor of a
  qualification-table row, that lists the case as a backticked ID or `*`
  pattern and names the provider (Xerces, .NET, xmlschema or libxml2); the
  record owns the rule and the diagnosis. Every entry explains such a row.
  `TestObservedDifferencesAreDiagnosed` enforces both directions and lists
  every undiagnosed row, except in cases listed in `oracle-backlog.txt`. For a
  record it checks only that the record exists, that its text lists the case,
  and that the provider's name appears in it outside link targets; it cannot
  tell whether that text describes the provider's behaviour on the case. Review
  owns that: a change that adds or edits a `record` entry confirms the record
  describes the provider's behaviour on the case.
  The test fails when the backlog is unsorted, lists a case that is not a
  measured case, or lists a case whose differences are all owned. The backlog
  only shrinks: a new case is never listed, and a change that modifies a listed
  case owns its differences and removes it from the list, which review checks
  because the test cannot tell a modified case from an untouched one.
- Each schema or instance `expected` is the effective project expectation,
  derived from the published specifications, approved corrections and
  documented qualifications. Provider agreement is diagnostic evidence, not a
  majority-vote replacement for those sources. Where specification text,
  adopted corrections and accepted W3C expectations leave a reading doubtful,
  the interpretation record applies the
  [processor tie-break](../docs/spec/INTERPRETATIONS.md#processor-tie-break).
  The `source` metadata and per-fixture `status` preserve upstream provenance
  without overriding it.
  When a project expectation corrects an upstream W3C result,
  `schema.oracle.w3c` or `instance.oracle.w3c` records the original expectation,
  reason, and specification reference; an entry whose expectation equals the
  effective one corrects nothing and is rejected. This provenance is explanatory; consumers do not
  merge it into the effective expectation or infer a Xerces-J result.
  A schema or instance's `oracle.project` likewise records a corrected original project
  expectation, which also differs from the effective one. Existing `internalExpected` values remain historical extraction
  metadata. A legacy test name containing `valid` or `invalid` is not an oracle;
  preserve its identity when correcting `expected`.
- This directory contains no Java classes, jars, or shell scripts.

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

Check the observations against the inventory:

```sh
go test ./tests -run '^(TestMeasuredPhasesHaveObservations|TestObservedDifferencesAreDiagnosed)$'
```

## Runner Contract

A runner SHOULD read the inventory through `internal/corpus`, compile each case schema document set,
then validate each listed instance against that schema. Expected values are
`valid` or `invalid`; provider comparisons follow the
[Corpus Contract](#corpus-contract).

A schema's or instance's optional `errorCode` checks the first structured diagnostic
returned by the Go runner. Use it for default-option diagnostic precedence,
including malformed XML after an earlier schema error. Custom readers, option
budgets, error causes, and reuse of one session belong in public API tests.

The Go runner rejects internal diagnostics anywhere in a compilation or
validation error tree, including wrapped causes and aggregate branches. An
expected-invalid result, a matching `errorCode` (even `internal.invariant`), or
an allowlisted unsupported diagnostic cannot make an internal failure pass or
skip. Other rejection expectations retain their existing meaning, including
malformed XML and explicit resource-limit cases.

Every diagnostic of category schema parse, schema compile or unsupported that a
schema compilation produces, and every diagnostic of category validation or
unsupported that an instance run produces while reading a document, carries a
path and a line; the Go runner fails the case otherwise, with no exemptions.
Failures about a whole explicit document (it cannot be opened or read, or
exceeds a size limit), about the source set, a nil reader and option errors
have no document position and do not occur in the harness.

Every entry in `schema.documents` is a compilation root, in manifest order. A
present `schema` has at least one document, and every document and instance
names a file.
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

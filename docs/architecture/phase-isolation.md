# Phase Isolation

This document describes the current package ownership model. It is a boundary
contract for contributors, not a migration log.

The public import path is `github.com/jacoelho/xsd`. Root package files should
stay small and should not accumulate compiler, runtime, source-resolution, XML
streaming, formatting, or diagnostics implementation.

## Public Packages

- `github.com/jacoelho/xsd` is the facade package. It owns exported public API
  types and methods: `Engine`, `Session`, `CompileOptions`, `ValidateOptions`,
  `SchemaSource`, `Resolver`, `ResolverFunc`, `File`, `Bytes`, `Reader`,
  `LimitedReader`, `Compile`, `CompileWithOptions`, and validation entrypoints.
- `github.com/jacoelho/xsd/internal/format` owns repository-internal XML formatting.
- `github.com/jacoelho/xsd/xsderrors` owns public diagnostics: structured
  errors, error lists, categories, codes, and unsupported-error inspection.

Root `xsd` MAY adapt public options, source wrappers, and sessions to internal
types. Root `xsd` MUST NOT expose old root-level diagnostics or formatting
types/functions; those belong to `xsderrors` and `format`.

## Internal Packages

- `internal/source` owns schema source primitives, resolver adaptation,
  source-name validation, schemaLocation normalization, include/import planning,
  loaded-source de-duplication, chameleon include decisions, and source-key
  canonicalization.
- `internal/compile` owns schema parsing and compilation: schema XML limits,
  component syntax, child-order and admission rules, name/index allocation,
  built-in declarations, facets, derivation checks, identity-constraint
  compilation, content-model compilation, source loading, and freeze-time
  schema validation before publication.
- `internal/runtime` owns immutable schema runtime data and runtime vocabulary:
  typed IDs, names, declarations, simple and complex type metadata, facets,
  value constraints, identity metadata, wildcards, substitution groups,
  content-model execution, read projections, clone helpers, and runtime
  invariant validation.
- `internal/validate` owns instance validation: option normalization, XML
  reader preflight, parser error classification, validation recovery,
  document structure, start/end element decisions, attributes, content,
  simple-content assessment, identity-state storage and resolution, XSI
  handling, and schemaLocation hint handling.
- `internal/stream` owns XML token streaming and declaration scanning shared by
  schema parsing, instance validation, and formatting.
- `internal/lex` owns low-level XML lexical helpers used by source and stream
  code.
- `internal/xmlns` owns namespace stack behavior and duplicate expanded
  attribute detection.
- `internal/vocab` owns XML/XSD namespace and vocabulary constants.

Internal packages MUST NOT import root `xsd`. Compile-time packages MUST NOT
depend on validation packages. Runtime vocabulary packages MUST remain below
compile and validate packages.

## Data Flow

Compilation flow:

1. Public callers provide `xsd.SchemaSource` values.
2. Root `xsd` converts them to `internal/source.Source` values.
3. `internal/compile` loads source documents, compiles schema components, builds
   runtime tables, validates runtime invariants, and returns a runtime read
   boundary.
4. Root `xsd.Engine` stores only the validation-facing runtime interface.

Validation flow:

1. Public callers validate through `Engine.Validate`, `ValidateWithOptions`, or
   a reusable `Session`.
2. Root `xsd` adapts public validation options.
3. `internal/validate` owns the validation session and reads immutable schema
   data through runtime interfaces.
4. Runtime table execution and metadata checks stay in `internal/runtime`;
   instance-validation policy stays in `internal/validate`.

Formatting flow:

1. Repository-owned tools call `internal/format`.
2. Formatting uses internal XML lexical/streaming helpers as needed.
3. Formatting does not belong in the root public `xsd` package.

Diagnostics flow:

1. Internal packages return structured errors using public `xsderrors` types.
2. Public callers inspect `xsderrors.Error`, `xsderrors.Errors`,
   `xsderrors.Category`, and `xsderrors.Code`.
3. Root `xsd` does not duplicate diagnostic types.

## Tests And Enforcement

The boundary is enforced by tests, not only by convention:

- `tests/phase_import_graph_test.go`
  - `TestInternalPhasePackageImportGraph`
  - `TestValidationInputPackageImportGraph`
  - `TestFormatPackageImportGraph`
  - `TestXMLNamespacePackageImportGraph`
  - `TestRuntimeVocabularyPackageImportGraph`
  - `TestSourcePackageImportGraph`
- `tests/phase_boundary_test.go`
  - `TestInternalImplementationPackagesExist`
  - `TestRootCompileIsFacade`
  - `TestRootDoesNotImportRuntimeImplementation`
  - `TestRootDoesNotExposeOldPublicAPIs`
- `tests/root_public_shape_test.go`
  - `TestRootTestsUsePublicPackage`
- `tests/stream_boundary_test.go`
  - `TestStreamBorrowedAttributeFieldsStayBehindAccessors`
- `tests/external_api_smoke_test.go`
  - `TestExternalModuleUsesPublicSchemaAPI`

Root tests and benchmarks MUST use `package xsd_test`. Root tests MUST NOT
import `internal` packages. Implementation fuzz tests MUST live with the package
that owns the implementation being fuzzed.

Current fuzz ownership:

- `internal/stream`: `FuzzXMLStreamParser`
- `internal/compile`: `FuzzSchemaParserLimits`, `FuzzXSDRegexSyntax`
- `internal/validate`: `FuzzValidateNeverPanics`

## Build Targets

Build and smoke targets must name the packages that own the code they exercise:

- `make test` runs `go test ./...`.
- `make fuzz-smoke` runs fuzzers in their internal owning packages.
- `make bench-smoke` runs the benchmark smoke selection over `./...`, because
  the selected benchmarks span root and `internal/runtime`.
- `make xmllint` directly runs `go build -o bin/xmllint ./cmd/xmllint`; the Go
  build cache, not Makefile file prerequisites, decides whether internal
  package changes require rebuild work.

## Documentation Ownership

- README documents public usage and command workflows.
- `docs/spec` contains local specification/reference material.
- Architecture docs must describe the current package graph. Do not commit
  migration-era notes that mention deleted root implementation files or
  nonexistent boundary tests.

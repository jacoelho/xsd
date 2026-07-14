# Phase Isolation

This document describes the current package ownership model. It is a boundary
contract for contributors, not a migration log.

The public import path is `github.com/jacoelho/xsd`. Root package files should
stay small and should not accumulate compiler, runtime, source-resolution, XML
streaming, formatting, or diagnostics implementation.

## Public Packages

- `github.com/jacoelho/xsd` is the facade package. It owns exported public API
  types and methods: `Engine`, `Session`, `CompileOptions`, `ValidateOptions`,
  `SchemaSource`, `Resolver`, `ResolverFunc`, `File`, `Bytes`, `Open`, `Compile`,
  `CompileWithOptions`, and validation entrypoints.
- `github.com/jacoelho/xsd/internal/format` owns repository-internal XML formatting.
- `github.com/jacoelho/xsd/xsderrors` owns public diagnostics: structured
  errors, error lists, categories, codes, and unsupported-error inspection.

Root `xsd` MAY adapt public options, source wrappers, and sessions to internal
types. Root `xsd` MUST NOT expose old root-level diagnostics or formatting
types/functions; those belong to `xsderrors` and `format`.

## Internal Packages

- `internal/source` owns immutable/repeatable schema source primitives,
  context-aware callbacks, staged bounded reads, resolver adaptation, local and
  generic backend policy, and source identity. It does not own XSD vocabulary
  or schema-graph policy.
- `internal/uriref` owns XSD 1.0 URI-reference validity after XLink escaping,
  raw and escaped projections, fragment syntax, and raw-preserving RFC 2396
  composition. Arbitrary source names and Unix paths do not enter this type.
- `internal/compile` owns schema parsing and compilation: schema XML limits,
  source-aware diagnostics, component syntax, child-order and admission rules,
  opaque annotation-payload consumption, name/index allocation,
  built-in declarations, facets, derivation checks, identity-constraint
  compilation, content-model compilation, transitive source loading,
  include/import and chameleon graph semantics, and construction of
  compiler-owned mutable `runtime.SchemaBuild` state. Correlated topology
  mutations are confined to `internal/compile/schema_build.go`; compiler
  algorithms may mutate nested records but must use that owner for declaration
  registration, placeholder completion, ID allocation, atomic element-constraint
  and substitution finalization,
  compiled-model alignment, builtin handles, notations, and publication.
- `internal/runtime` owns the schema runtime model and publication boundary:
  typed IDs, names, declarations, simple and complex type metadata, facets,
  value constraints, identity metadata, wildcards, substitution groups,
  `SchemaBuild` invariant validation, `PublishSchema`, sealed `Schema` state,
  the bounded immutable substitution table, canonical element read table,
  precomputed type-derivation indexes, the single published simple-type cold
  table that owns union-member storage for both derivation and value validation,
  the canonical content-model restriction relation shared by compilation and
  publication audit, validation reads, content-model execution, and
  publication-owned clones. Publication accepts the compile context, audits
  without consuming compiler state, then performs one final cancellation check
  before the build-consumption linearization point.
  Cross-table `TypeID` values expose only typed constructors, classification,
  and projections; their tag and payload remain runtime-owned. Identity-path
  QName absence is returned by value and has no mutable package-global state.
- `internal/validate` owns instance validation: finite default limits, option normalization, XML
  reader preflight, parser error classification, validation recovery,
  document structure, start/end element decisions, attributes, content,
  simple-content assessment, identity-state storage and resolution, XSI
  handling, and schemaLocation hint handling.
- `internal/stream` owns XML token streaming and declaration scanning shared by
  schema parsing, instance validation, and formatting. Its parser owns prolog
  preflight, the sole input buffer, and reader detachment for each stream.
- `internal/lex` owns low-level XML lexical helpers used by source and stream
  code.
- `internal/xmlns` owns namespace binding validity, lexical-name resolution,
  and duplicate expanded-attribute detection for both schema and instance XML.
- `internal/vocab` owns XML/XSD namespace and vocabulary constants.

Internal packages MUST NOT import root `xsd`. Compile-time packages MUST NOT
depend on validation packages. Runtime vocabulary packages MUST remain below
compile and validate packages.

## Data Flow

Compilation flow:

1. Public callers provide `xsd.SchemaSource` values.
2. Root `xsd` converts them to immutable or repeatable `internal/source.Source`
   values.
3. `internal/compile` applies document-local XSD admission before resolving a
   document's references. Each resolved include/import edge is target-namespace
   checked before its resolver context or descendants are activated. Explicit
   source descriptors are bounded before facade conversion; the loader charges
   their count plus each distinct resolver-returned canonical identity against
   one source budget. It applies inherited `xml:base` per resolver context,
   accounts every source byte and failure stage,
   and parses every schema token against XML namespace and resource limits.
   The compile context is call-local and is passed unchanged to resolver and
   opener callbacks. Cancellation is checked around callbacks, reads, tokens,
   graph work, and major compilation batches.
   Schema-provided URI references are admitted before graph resolution, and graph
   edges retain a validated reference rather than a reparsable string. Their
   whitespace-normalized spelling remains the datatype and custom-resolver
   value; an XLink-escaped projection is created only for generic or file fallback.
   Source identity canonicalizes hierarchical and opaque URI components while
   preserving explicit empty authority, query, and fragment delimiters and the
   case of IPv6 zone identifiers. Source names remain opaque identities, so Unix
   filename characters such as `#` and `?` are not reinterpreted as URI syntax.
   Malformed URI references fail before resolver invocation. A custom resolver
   receives the raw normalized location and a valid raw composed base even when the
   local-path backend cannot represent it; after an exclusive not-found result,
   unsupported local fallback remains an unavailable optional hint. A successful
   resolver result is authoritative for an edge; a generic identity-only
   candidate remains pending until a document with that identity is actually
   loaded, when the loader binds and target-checks every pending edge before
   activating the document.
   Annotation payload is consumed as namespace-well-formed opaque XML without
   entering the retained schema tree. The compiler then applies chameleon graph
   semantics. Once every effective target namespace is known, it indexes one
   declaration representative for each exact document content and effective
   namespace while retaining every source occurrence for resolver traversal and
   graph validation. It compiles schema components and populates a compiler-owned
   mutable `runtime.SchemaBuild`. After all types and substitution affiliations are
   complete, element value constraints and effective substitution types are
   finalized atomically; the bounded transitive substitution table is the only
   retained substitution lookup.
4. `internal/runtime.PublishSchema` audits exact global registries and component
   ownership before constructing validation reads, audits those projections,
   and checks cancellation before consuming the build. A failure, including
   cancellation, leaves compiler state retryable. Once the final check passes,
   build consumption and successful return form one commit with no later
   cancellation override.
5. Root `xsd.Engine` stores that sealed validation schema.

Validation flow:

1. Public callers validate through `Engine.Validate`, `ValidateWithOptions`, or
   a reusable `Session`.
2. Root `xsd` adapts public validation options.
3. Root `xsd` delegates construction and one-shot validation to
   `validate.NewSession` and `validate.Validate`; there is no separately
   initializable internal session state.
   A reusable public session is a handle to one guarded internal owner: copies
   alias that owner, and overlapping calls fail before reading the second input.
   That owner also holds bounded scalar scratch for type derivation and compiled
   string-pattern execution; immutable schema tables never hold validation work
   buffers. Every return path clears document-local state before releasing the
   overlap guard, while returned diagnostics remain caller-owned. The context
   is call-local and is not retained by a session. The overlap guard is acquired
   before context inspection, and cleanup completes before that guard is released.
4. `internal/validate` reads immutable schema facts through methods on the
   sealed `*runtime.Schema`, then applies validation policy to those facts.
   Element frames distinguish assessed nodes, nodes admitted by a
   `processContents="skip"` wildcard, and validation-recovery containment.
   Identity fields distinguish an absent field from a validated value and a
   selected node that has no valid simple value. Skipped and lax-missing nodes
   therefore remain visible to identity XPath matching without being assigned
   declarations or types, while recovery invalidates affected ancestor fields
   without creating secondary identity diagnostics. A nillable element-field
   marker is enforced only when selection finalization has established a
   complete qualified key sequence. At element end, selections owned by that
   element's identity scope receive current nillable-field markers and finish
   before the scope closes. Scope-local failure is then folded into the element
   assessment and invalidates still-pending ancestor-owned fields before those
   selections finish. Element-start assessment extracts `xsi:nil` and `xsi:type` before
   assessing either attribute, preserves each successful result when the other
   fails, and owns their diagnostics. Identity capture only records a matched
   value or invalidates the matched field, so it cannot duplicate those
   diagnostics.
5. Runtime table execution and metadata checks stay in `internal/runtime`;
   instance-validation policy stays in `internal/validate`.

Cancellation is cooperative at effect and batch boundaries. The library cannot
forcibly interrupt a resolver, opener, file operation, or arbitrary `io.Reader`
that ignores cancellation. Public callbacks MUST honor their context; callers
that require hard interruption of validation reads MUST provide a reader that
unblocks or is closed when its context is canceled.

Formatting flow:

1. Repository-owned tools call `internal/format`.
2. Formatting uses internal XML lexical/streaming helpers as needed.
3. Formatting does not belong in the root public `xsd` package.

Diagnostics flow:

1. Internal packages return source-aware structured errors using public
   `xsderrors` types; schema diagnostics identify the originating schema in
   `xsderrors.Error.Path`.
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
  - `TestRootRuntimeImportIsConfinedToEngineAndSession`
  - `TestRootDoesNotExposeOldPublicAPIs`
- `tests/root_public_shape_test.go`
  - `TestRootTestsUsePublicPackage`
- `tests/stream_boundary_test.go`
  - `TestStreamBorrowedAttributeFieldsStayBehindAccessors`
  - `TestStreamBoundaryCallIdentityRejectsNameCollisions`
- `tests/schema_build_boundary_test.go`
  - `TestCompilerSchemaBuildTopologyHasOneOwner`
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

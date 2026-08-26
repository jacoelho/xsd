# Architecture

This is the canonical architecture contract for contributors. It describes the
current target state, not migration history or alternative designs.

The public import path is `github.com/jacoelho/xsd`. Root package files should
stay small and should not accumulate compiler, runtime, source-resolution, XML
streaming, formatting, or diagnostics implementation.

## Public Packages

- `github.com/jacoelho/xsd` is the facade package. It owns exported public API
  types and methods: `Engine`, `Session`, `CompileOptions`, `ValidateOptions`,
  `SchemaSource`, `Resolver`, `ResolverFunc`, `File`, `Bytes`, `Open`, `Compile`,
  `CompileWithOptions`, and validation entrypoints.
- `github.com/jacoelho/xsd/xsderrors` owns public diagnostics: structured
  immutable errors, owning error aggregates, the category/code catalog, one
  location decorator, one presentation projection, and unsupported-error
  inspection.

Root `xsd` MAY adapt public options, source wrappers, and sessions to internal
types. Root `xsd` MUST NOT expose old root-level diagnostics or formatting
types/functions; those belong to `xsderrors` and `internal/format`.

## Internal Packages

- `internal/source` owns immutable/repeatable schema source primitives,
  explicit source kinds, repeatable callbacks, staged bounded acquisition,
  resolver adaptation, local and generic backend policy, resolution context,
  and source identity. `Source.Acquire` is the only source-read path and
  `Source.ResolveFrom` is the only reference-resolution path. The package does
  not own XSD vocabulary or schema-graph policy.
- `internal/uriref` owns XSD 1.0 URI-reference validity after XLink escaping,
  raw and escaped projections, fragment syntax, and raw-preserving RFC 2396
  composition. Arbitrary source names and Unix paths do not enter this type.
- `internal/compile` owns schema parsing and compilation: schema XML limits,
  source-aware diagnostics, component syntax, child-order and admission rules,
  opaque annotation-payload consumption, name/index allocation,
  built-in declarations, facets, derivation checks, identity-constraint
  compilation, content-model compilation, transitive source loading,
  include/import and chameleon graph semantics, and construction of
  compiler-owned mutable `runtime.SchemaBuild` state. Loading produces a
  `loadedSchemaGraph`; graph validation and chameleon expansion produce a
  `schemaPlan`; indexing and component compilation consume only that plan.
  Schema graph expansion, target-context planning, and component-dependency
  resolution share one finite dependency-work budget; active component
  expansion also has a fixed stack-safety depth cap of 1024. Content-model
  analysis uses a separate finite work budget shared by consistency and
  restriction graph traversal, ambiguity checks, compilation, and publication
  audit; reusable graph summaries are memoized. Correlated topology mutations are confined to
  `internal/compile/schema_build.go`; compiler
  algorithms may mutate nested records but must use that owner for declaration
  registration, placeholder completion, ID allocation, atomic element-constraint
  and substitution finalization, compiled-model alignment, builtin handles,
  notations, and publication.
- `internal/runtime` owns the schema runtime model and publication boundary:
  typed IDs, names, declarations, simple and complex type metadata, facets,
  value constraints, identity metadata, wildcards, substitution groups,
  `SchemaBuild` invariant validation, `PublishSchema`, sealed `Schema` state,
  the bounded immutable substitution table, canonical element read table,
  precomputed type-derivation indexes, the single published simple-type cold
  table that owns union-member storage for both derivation and value validation,
  one aggregate immutable identity-constraint read per constraint,
  `ContentModelAnalysis`, the sole bounded and memoized owner of content-model
  emptiability, count-range, substitution-aware matching, and overlap facts,
  and the canonical content-model restriction relation shared by compilation
  and publication audit, validation reads, content-model execution, and
  publication-owned clones. Publication audits without consuming compiler state,
  then consumes the build only after the audit succeeds.
  Cross-table `TypeID` values expose only typed constructors, classification,
  and projections; their tag and payload remain runtime-owned. Identity-path
  QName absence is returned by value and has no mutable package-global state.
- `internal/validate` owns instance validation: finite default limits, option
  normalization, XML reader preflight, parser error classification, validation
  recovery, document structure, start/end element decisions, attributes,
  content, simple-content assessment, the concrete document-local identity
  evaluator and its lifecycle, XSI handling, and schemaLocation hint handling.
- `internal/format` owns repository-internal XML formatting and its output
  bound. It consumes the shared stream and namespace boundaries and exposes no
  root-package API.
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
   accounts every source byte and failure stage, and parses every schema token
   against XML namespace and resource limits.
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
   The completed loader state becomes a closed loaded graph: planning may consume
   and update graph-owned state but performs no further I/O. Planning validates
   target namespaces and expands effective chameleon contexts without reopening
   or resolving sources. Annotation payload is consumed as
   namespace-well-formed opaque XML without entering the retained schema tree.
   Source loading, graph planning, and component resolution consume one aggregate
   dependency-work budget. Component references also enter a bounded active
   expansion stack; cache hits remain charged, while cycles retain their specific
   schema diagnostics. Once every effective target namespace is known, the compiler indexes one
   declaration representative for each exact document content and effective
   namespace while retaining every source occurrence for resolver traversal and
   graph validation. It compiles schema components and populates a compiler-owned
   mutable `runtime.SchemaBuild`. After all types and substitution affiliations are
   complete, element value constraints and effective substitution types are
   finalized atomically; the bounded transitive substitution table is the only
   retained substitution lookup.
4. `internal/runtime.PublishSchema` audits exact global registries and component
   ownership before constructing validation reads and auditing those projections.
   An audit failure leaves compiler state retryable. A successful audit and build
   consumption form one publication commit.
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
   overlap guard, while returned diagnostics remain caller-owned. The overlap
   guard is acquired before reading input, and cleanup completes before that guard
   is released.
4. `internal/validate` reads immutable schema facts through methods on the
   sealed `*runtime.Schema`, then applies validation policy to those facts.
   Identity evaluation reads each constraint through one aggregate runtime
   projection. One concrete evaluator owns the element identity stack, matching
   path, per-element ID state, document IDs and IDREFs, key/unique/keyref scopes,
   pending selections, resource accounting, and reset/discard behavior. Value
   capture uses one borrowed prepared target at a time: callers prepare, record,
   capture, then commit, or reject the target on validation failure. Element-end
   finalization is also evaluator-owned, including recoverable diagnostic
   reporting and the ordering of field completion, scope closure, ancestor
   invalidation, and path/stack release.
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
   selections finish. Element-start assessment extracts `xsi:nil` and `xsi:type`
   before assessing either attribute, preserves each successful result when the
   other fails, and owns their diagnostics. Identity capture only records a
   matched value or invalidates the matched field, so it cannot duplicate those
   diagnostics.
   Every element start is a transaction across XML/namespace stacks,
   schema-location hints, parent-content state, content-model bits, and identity
   state. A fatal start rolls all of them back. A semantic-stop transition keeps
   the committed XML syntax state needed to parse the remainder while discarding
   semantic state. Batched hint and identity writes preflight their full resource
   cost before mutating their owners.
   Generic event-sink or matcher interfaces are intentionally absent: there is
   one evaluator implementation and one validation caller, while an interface
   would hide the required transaction and element-lifecycle sequencing without
   providing a real substitution boundary.
5. Runtime table execution and metadata checks stay in `internal/runtime`;
   instance-validation policy stays in `internal/validate`.

Compilation and validation are synchronous. Resource limits bound admitted input
and retained work, but the library does not own deadline or interruption policy.
Callers that require interruption of a blocked resolver, opener, file operation,
or arbitrary `io.Reader` MUST provide I/O that they can close or otherwise unblock.

Formatting flow:

1. Repository-owned tools call `internal/format`.
2. Formatting uses internal XML lexical/streaming helpers as needed.
3. Formatting does not belong in the root public `xsd` package.

Diagnostics flow:

1. Internal packages return source-aware structured errors using public
   `xsderrors` types; schema diagnostics identify the originating schema through
   `xsderrors.Error.Path()`.
2. Public callers inspect `xsderrors.Error`, `xsderrors.Errors`,
   `xsderrors.Category`, and `xsderrors.Code`. Constructors reject invalid
   category/code combinations. Aggregate construction owns its input, accessors
   do not expose mutable storage, and `xsderrors.Flatten` is the sole top-level
   presentation projection.
3. `xsderrors.WithLocation` is the only path/line/column decorator. Root `xsd`
   and formatter packages do not duplicate diagnostic types or codes.

Browser flow:

1. `cmd/wasmxsd` owns the JavaScript-facing tagged response contract and the
   generated limit catalog. Validation responses are exactly one of `valid`,
   `invalid`, or `error`; formatting responses are `ok` or `error`.
2. `docs/js/xsd-worker.js` owns the Go WASM runtime. WASM functions and limits
   never enter the window global scope.
3. `ValidationWorkerClient` owns worker lifecycle, a single active request, one
   latest pending request, cancellation, initialization and execution timeout
   termination, restart, and immutable state snapshots. The page ignores
   results whose input epoch is no longer current. The page owns editor/file
   generations so an older asynchronous file read cannot replace newer text,
   and applies UTF-8 byte and rendered-line bounds before syntax highlighting or
   gutter construction.
4. `cmd/xsdweb` serves only built assets, binds to loopback by default, prevents
   directory listings and writes, disables caching, and bounds HTTP lifecycle
   time and headers. It does not compile or validate schemas.

## Tests And Enforcement

The boundary is enforced by tests, not only by convention:

- `tests/phase_import_graph_test.go`
  - `TestInternalCapabilityImportAllowlist`
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
- `docs/js/validation-worker.test.js` enforces worker queue, cancellation,
  timeout, and state ownership.
- `docs/js/browser/validator.spec.js` runs the built WASM application in
  Chromium and checks both response branches and main-thread isolation.

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
- `make wasm` builds the worker-owned WASM module and copies the matching Go
  runtime support file.
- `make web` builds those assets before starting the bounded loopback server.
- `make web-test` runs deterministic JavaScript boundary and lifecycle tests.
- `make browser-test` runs the browser-level WASM integration test.
- `make fuzz-smoke` runs fuzzers in their internal owning packages.
- `make bench-smoke` runs the benchmark smoke selection over `./...`, because
  the selected benchmarks span root and `internal/runtime`.
- `make xmllint` directly runs `go build -o bin/xmllint ./cmd/xmllint`; the Go
  build cache, not Makefile file prerequisites, decides whether internal
  package changes require rebuild work.

## Rejected Alternatives

- Raising the DFA state cap to admit avoidable expansion was rejected. Safe
  single-particle occurrence normalization prevents that expansion; the
  separately bounded scalar-work default is calibrated to the retained-state
  cap so valid counted models can reach it.
- Interleaving resolution, graph validation, and component compilation was
  rejected because partially known namespaces made ownership and failure state
  ambiguous. The loaded-graph-to-plan boundary is the only route into indexing.
- A generic runtime matcher/evaluator interface was rejected because there is
  one implementation and one caller; it obscured the required transactional
  sequencing without creating a substitution boundary.
- Main-thread WASM execution was rejected because synchronous validation blocks
  rendering and cannot be interrupted. Worker termination is the cancellation
  boundary.
- Compatibility fields, aliases, and parallel constructors for diagnostics were
  rejected because they would preserve two representations and two location
  paths for the same fact.

## Documentation Ownership

- README documents public usage and command workflows.
- `docs/spec` contains local specification/reference material.
- This file is the sole architecture source of truth. Other documentation may
  link here but must not restate a competing package graph or lifecycle model.

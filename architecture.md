# Architecture

This document is derived from code and tests in current tree. Existing repository documentation was ignored.

## Scope

Project is an XSD 1.0 compiler and validator in Go.

Main flow:

1. Load one or more XSD roots from `io/fs`.
2. Parse them into a symbolic schema graph.
3. Resolve and validate schema semantics.
4. Assign deterministic IDs and build compile-time indexes.
5. Compile validator artifacts and lower whole result into an immutable runtime schema.
6. Validate XML instances with pooled streaming sessions.

High-level shape is deliberate:

- Public API is tiny and lives in root package `xsd`.
- Most logic is internal and phase-separated.
- Runtime schema is immutable.
- Validation sessions are mutable, pooled, and reset per document.
- Architecture tests enforce package boundaries, file ownership, and public API stability.

## Module Layout

| Package | Role |
| --- | --- |
| `github.com/jacoelho/xsd` | Public shell. Source loading, option types, schema/validator wrappers. |
| `github.com/jacoelho/xsd/errors` | Public validation error model and formatting. |
| `github.com/jacoelho/xsd/cmd/xmllint` | Thin CLI over public API. |
| `github.com/jacoelho/xsd/pkg/xmltext` | Low-level XML tokenizer with limits, line/column tracking, entity handling, QName interning. |
| `github.com/jacoelho/xsd/pkg/xmlstream` | Namespace-aware streaming XML reader built on `xmltext`. |
| `github.com/jacoelho/xsd/internal/model` | Core XSD domain model: types, elements, groups, wildcards, facets, builtins, copying, walking. |
| `github.com/jacoelho/xsd/internal/parser` | XSD parser. Produces unresolved symbolic schema graph plus directive metadata and compact subtree DOM. |
| `github.com/jacoelho/xsd/internal/semantics` | Reference resolution and schema semantic validation. Produces semantic views used by compiler. |
| `github.com/jacoelho/xsd/internal/analysis` | Deterministic IDs, cycle checks, reference indexes, runtime ID plan, ancestor plan. |
| `github.com/jacoelho/xsd/internal/contentmodel` | Shared content-model algorithms: group expansion, Glushkov construction, determinism, DFA/NFA lowering. |
| `github.com/jacoelho/xsd/internal/complexplan` | Precomputed effective complex-type view shared by semantics and compiler. |
| `github.com/jacoelho/xsd/internal/validatorbuild` | Compiles runtime value validators, enums, facets, defaults, fixed values. |
| `github.com/jacoelho/xsd/internal/compiler` | Schema loading, include/import merge, orchestration, preparation, runtime-schema lowering. |
| `github.com/jacoelho/xsd/internal/runtime` | Immutable packed runtime schema and runtime tables. |
| `github.com/jacoelho/xsd/internal/validator` | Runtime XML validator. Streaming execution, element stack, attribute validation, identity constraints, facet programs. |
| `github.com/jacoelho/xsd/internal/value` | Lexical parsing, canonicalization, QName parsing, value-key derivation, temporal helpers. |
| `github.com/jacoelho/xsd/internal/value/num` | Numeric parsing and comparison helpers for XSD numeric types. |
| `github.com/jacoelho/xsd/w3c` | Test-only conformance harness package. No non-test code. |

### Internal file-family layout

Two large internal packages are intentionally split by file prefix.

`internal/compiler` families:

- `source_*`: source application and root orchestration
- `load_session.go`, `loader_*`: document load state machine, cache/cycle handling, include/import resolution
- `merge_*`: include/import merge planning and application
- `pending_*`: deferred resolution during load/merge
- `prepare*`: semantic preparation entry
- `schema_*`: runtime-schema lowering pieces
- `builder_*`: top-level runtime builder shell

`internal/validator` families:

- `session_*`: pooled per-document state
- `validation_executor_process.go`: event loop dispatch
- `runtime_*`: runtime table execution paths and stage-oriented execution shells
- `start_*`: start-element matching and frame planning
- `attr_*`: attribute collection, defaults, and validation
- `value_*`: datatype, facet, enum, and canonicalization execution
- `close`, `finalize`, `lifecycle`, `select`: identity and end-of-element coordination

## Public Surface

Root package `xsd` is intentionally narrow. Architecture tests allow only:

- Types: `Schema`, `Validator`, `QName`, `SourceSet`, `PreparedSchema`, `SourceOptions`, `BuildOptions`, `ValidateOptions`
- Functions: `Compile`, `CompileFile`, `NewSourceSet`, `NewSourceOptions`, `NewBuildOptions`, `NewValidateOptions`
- Methods on those types

That shell does not expose parser, semantic model, runtime schema, or internal compiler packages.

### Entry points

- `Compile(fsys, location, sourceOpts, buildOpts)` loads one root from any `fs.FS`.
- `CompileFile(path, sourceOpts, buildOpts)` loads one root from local filesystem.
- `SourceSet` supports multi-root prepare/build.
- `PreparedSchema` separates source preparation from runtime build.
- `Schema.NewValidator(opts)` creates per-validator instance options.
- `Schema.Validate*` lazily creates one default validator with default validate options.

### `CompileFile` path policy

Implementation uses `os.OpenRoot(dir)` on root file directory, then resolves nested imports/includes inside that directory tree. Tests confirm:

- explicit root symlink is allowed
- nested symlink escape is rejected
- relative imports work inside selected tree

## Phase Model

Public tests define three visible phases:

1. Source phase
2. Build phase
3. Validation phase

Internal implementation splits that into finer steps.

### 1. Source phase

Public types:

- `SourceSet`
- `PreparedSchema`
- `SourceOptions`

`SourceSet` owns root entries:

- `fs.FS`
- root location
- optional custom resolver for `CompileFile`

`SourceSet.Prepare()` resolves source options, loads roots, parses schemas, merges includes/imports, runs semantics and analysis, and returns `PreparedSchema`.

For single-root input, root package passes `compiler.LoadConfig{FS, Location, Resolver}`.

For multi-root input, root package passes `compiler.LoadConfig{Roots}`.

Multi-root behavior:

- each root is loaded independently
- origin/import-context keys are disambiguated per root index
- roots sharing target namespace are merged as include-equivalent
- roots with different target namespaces are merged as import-equivalent
- same textual location across distinct filesystems is allowed

### 2. Load and parse phase

Main owner is `internal/compiler`.

Important pieces:

- `Loader`
- `loadSession.loadResolved`
- merge `PlanInclude` / `PlanImport`
- parser `ParseWithImportsOptionsWithPool`

Load flow:

1. Resolve schema document with resolver.
2. Reuse cached loaded schema when same `(systemID, targetNamespace)` key already exists.
3. Detect circular loads and defer when needed.
4. Parse document into `parser.ParseResult`.
5. Initialize origin tracking and import metadata.
6. Load and merge directives in deterministic order.
7. Finish with one merged `parser.Schema`.

Directive behavior:

- `xs:include` must have `schemaLocation`
- `xs:import` may omit `schemaLocation`
- `SourceOptions.WithAllowMissingImportLocations(true)` skips imports without location
- include namespace compatibility is enforced
- import target namespace match is enforced
- merge insertion for includes preserves source declaration order via recorded directive indexes

Unsupported:

- `xs:redefine`

### 3. Parser phase

Owner is `internal/parser`.

Parser output is `parser.Schema`, split into:

- `SchemaGraph`
- `SchemaMeta`

`SchemaGraph` contains:

- `Groups map[QName]*ModelGroup`
- `TypeDefs map[QName]Type`
- `AttributeDecls map[QName]*AttributeDecl`
- `SubstitutionGroups map[QName][]QName`
- `AttributeGroups map[QName]*AttributeGroup`
- `ElementDecls map[QName]*ElementDecl`
- `NotationDecls map[QName]*NotationDecl`
- `GlobalDecls []GlobalDecl`

`SchemaMeta` contains:

- origin maps for major component kinds
- import contexts
- imported namespaces
- ID attribute tracking
- namespace declarations
- schema location
- target namespace
- form defaults
- final/block defaults

Parser design points:

- `GlobalDecls` preserves source order and later drives semantics and deterministic ID assignment
- top-level non-XSD elements are skipped
- directive subtrees are parsed through a compact subtree DOM, not whole-document DOM
- parse buffer size is `256 * 1024`, aligned with streaming layer reuse

Supported top-level schema declarations:

- `element`
- `complexType`
- `simpleType`
- `group`
- `attribute`
- `attributeGroup`
- `notation`
- `key`
- `keyref`
- `unique`

### 4. Semantic preparation

Owners:

- `internal/semantics`
- `internal/compiler/prepare_semantics.go`

`compiler.Prepare` clones parsed schema, then normalizes and validates.

`compiler.PrepareOwned` consumes schema in place.

Semantic pipeline in `semantics.ResolveAndValidateSchema`:

1. `ResolveGroupReferences`
2. `ValidateStructure`
3. `NewResolver(sch).Resolve()`
4. `ValidateReferences`
5. `ValidateDeferredRangeFacetValues`
6. reject any remaining parser placeholders

Resolver order is fixed:

1. simple types
2. complex types
3. groups
4. elements
5. attributes
6. attribute groups

After semantic resolution, compiler builds a `semantics.Context` and:

- validates UPA through `Context.Particles().ValidateUPA()`
- builds effective complex-type plan through `Context.ComplexTypes()`

Architectural boundary is strict: only `internal/compiler/prepare_semantics.go` may import `internal/semantics`.

### 5. Analysis phase

Owner is `internal/analysis`.

Purpose is deterministic indexing after semantic correctness is established.

Main steps:

1. `AssignIDs`
2. `DetectCycles`
3. `ResolveReferences`
4. build ancestor/runtime ID plans when lowering

`AssignIDs`:

- requires placeholder-free schema
- walks `GlobalDecls` source order
- assigns stable IDs starting at 1
- covers global/local elements, global/local attributes, global/anonymous/named types

`Registry` stores:

- `Types`, `Elements`, `Attributes`
- local element and attribute pointer maps
- anonymous type map
- deterministic `TypeOrder`, `ElementOrder`, `AttributeOrder`

`ResolvedReferences` stores non-mutating lookup indexes such as:

- element QName to `ElemID`
- attribute QName to `AttrID`
- group QName to target QName

`BuildRuntimeIDPlan` assigns final runtime IDs in deterministic order:

1. builtin types in fixed builtin order
2. schema types in registry order
3. elements in registry order
4. attributes in registry order

### 6. Complex-type plan

Owner is `internal/complexplan`.

This is a small shared leaf package. It precomputes effective complex-type state once and shares it between semantic UPA checks and runtime lowering.

For each complex type it stores:

- effective attribute declarations
- effective attribute wildcard
- effective content particle
- simple-content text type

## Value and Content-Model Compilation

Compilation is split on purpose.

### `internal/validatorbuild`

Builds runtime value-validation artifacts from prepared schema:

- type validator IDs
- validator lookup by schema type
- default and fixed value tables for elements and attributes
- attribute-use defaults/fixed values
- validator bundle
- enum tables
- facet instruction tables
- pattern tables
- value blob storage
- complex-type plan reference

Order:

1. create runtime type ID plan
2. compile registry validators
3. compile defaults/fixed values
4. compile attribute-use data

### `internal/contentmodel`

Owns shared automata and determinism logic:

- expand group refs
- optionally preserve or rewrite `xs:all`
- build Glushkov automaton
- expand substitution groups
- check determinism / UPA
- compile DFA with state cap
- fall back to NFA when DFA limit is exceeded

Current defaults:

- `defaultMaxDFAStates = 4096`
- build option `WithMaxDFAStates(0)` uses that default

## Runtime Lowering

Owner is `internal/compiler`.

Lowering starts from `compiler.Prepared`.

`Prepared` stores:

- prepared `parser.Schema`
- `analysis.Registry`
- `analysis.ResolvedReferences`
- `complexplan.ComplexTypes`
- lazy compiled validator artifacts
- cached prepare/build error state

`Prepared.Build(cfg)` lazily creates validator artifacts once, then lowers everything into `*runtime.Schema`.

`schemaBuilder.build()` order is fixed:

1. intern symbols and namespaces
2. initialize runtime ID spaces
3. build types
4. build ancestor chains
5. build attributes and complex-type attribute indexes
6. build elements
7. build content models
8. build identity constraints
9. attach wildcard/path tables
10. compute `BuildHash`

### Build defaults

- `BuildOptions.WithMaxOccursLimit(0)` uses compiler default `1_000_000`
- `BuildOptions.WithMaxDFAStates(0)` defers to content-model default `4096`

### Runtime schema shape

`internal/runtime.Schema` is the immutable execution artifact. It contains packed tables instead of pointer-heavy graph structures.

Major sections:

- symbol and namespace intern tables
- global type/element/attribute lookup tables
- runtime type table
- ancestor table
- complex-type table
- element and attribute tables
- complex attribute index
- validator, facet, enum, pattern, and value tables
- content-model bundles for DFA, NFA, and `xs:all`
- wildcard tables
- identity-constraint tables
- compiled path programs
- predefined builtin IDs
- root policy
- build hash

Design intent is clear: compile pointer-rich semantic graph once, then validate against compact indexed tables.

## Validation Runtime

Owner is `internal/validator`.

Public root type `xsd.Validator` is a thin wrapper over `validator.Engine`.

### Engine and sessions

`validator.Engine` owns:

- immutable `*runtime.Schema`
- `sync.Pool` of sessions
- parse options for `xmlstream.Reader`

`NewEngine` clones parse options and configures pool factory.

`ValidateWithDocument`:

1. acquires session
2. validates one document
3. resets and returns session to pool

`Session` holds per-document mutable state:

- XML reader and parse options
- reusable text, key, norm, value, and error buffers
- attribute tracking structures
- namespace/name state
- element stack
- identity-constraint runtime state
- validation error list
- arena-backed temporary storage

`Session.Reset()` clears state but keeps capacity, with shrink guards for very large buffers.

### Streaming execution

Validation loop is event-driven:

1. create or reset `xmlstream.Reader`
2. call `NextResolved()` in a loop
3. dispatch start, end, and char-data events through `validationExecutor`
4. finalize end-of-document checks

Finalize checks:

- document must have root
- element stack must be empty
- unresolved `IDREF` values are reported
- deferred identity-constraint errors are committed
- accumulated validation list is returned

### Start-element path

Start handling resolves:

- root or child element match
- xsi:type
- xsi:nil
- attribute classification
- attribute defaults/fixed values
- content-model transition state
- identity-constraint scope/match startup

### Value validation path

Value validation uses runtime programs, not schema graph traversal.

`RuntimeProgram` contains:

- enum table
- facet instruction slice
- pattern slice
- value blob
- normalized bytes
- canonical bytes
- validator metadata

`ValidateRuntimeProgram` interprets facet instructions and calls validator-owned callbacks for:

- regex checks
- enumeration key derivation/cache
- range checks
- length checks
- digit counts

### Identity constraints

Identity processing is stateful and transactional.

Key structures:

- `State[F]`
- `RuntimeFrame`
- `Scope`
- committed and uncommitted violation queues
- rollback `Snapshot`

On start event, validator can checkpoint, push frame, open scopes, evaluate selectors and fields, and roll back if surrounding element validation fails.

## Streaming Stack

Validation and parsing both depend on streaming XML packages.

### `pkg/xmltext`

Low-level tokenizer with:

- caller-owned token buffers
- optional entity resolution
- QName interning
- strict XML declaration checks
- line/column tracking
- configurable token, depth, attribute, and interner limits

Important defaults:

- decoder buffer size `32 * 1024`
- negative configured limits are clamped to `0` before final decoder option resolution

### `pkg/xmlstream`

Namespace-aware event reader built on `xmltext`.

Responsibilities:

- namespace scope tracking
- duplicate attribute detection
- raw and resolved events
- subtree skipping
- decode helpers
- QName caching and resolved-name caching

Reader buffer size is `256 * 1024`.

## Options

Options are immutable value builders. Internal `intOption` and `uint32Option` wrappers distinguish unset from explicit zero.

### `SourceOptions`

- `WithAllowMissingImportLocations(bool)`
- `WithSchemaMaxDepth(int)`
- `WithSchemaMaxAttrs(int)`
- `WithSchemaMaxTokenSize(int)`
- `WithSchemaMaxQNameInternEntries(int)`

Meaning:

- controls schema-load missing import handling
- configures schema XML parsing limits

### `BuildOptions`

- `WithMaxDFAStates(uint32)`
- `WithMaxOccursLimit(uint32)`

Meaning:

- caps DFA determinization before NFA fallback
- caps compilation of repeated particle occurrences

### `ValidateOptions`

- `WithInstanceMaxDepth(int)`
- `WithInstanceMaxAttrs(int)`
- `WithInstanceMaxTokenSize(int)`
- `WithInstanceMaxQNameInternEntries(int)`

Meaning:

- configures instance XML parsing limits per validator

### XML parse defaults

Shared root-level defaults in `xml_limits.go`:

- max depth: `256`
- max attrs: `256`
- max token size: `4 << 20`
- max QName interner entries: `0` meaning leave `xmlstream` default

Validation options are per validator/session, not global schema state.

## Core Domain Structures

### `internal/model`

This package is canonical schema domain model.

Important types:

- `QName`
- `Type` interface
- `SimpleType`
- `ComplexType`
- `ElementDecl`
- `AttributeDecl`
- `AttributeGroup`
- `ModelGroup`
- `AnyElement`
- `AnyAttribute`
- `IdentityConstraint`
- builtins and facet structures

`QName` is represented as `{Namespace, Local}` and stringifies as `{namespace}local` or `local`.

`ElementDecl` carries:

- type
- name
- substitution-group head
- default/fixed values plus namespace contexts
- constraints
- min/max occurs
- block/final
- form
- nillable/abstract/reference flags

`AttributeDecl` carries analogous value/type/default/fixed/use/form/reference state.

### `errors`

Public diagnostics are explicit typed data, not opaque strings.

`errors.Validation` carries:

- code
- message
- document URI
- instance path
- actual value
- expected values
- line
- column

`errors.ValidationList` sorts deterministically by document, line, column, code, and message.

## Architectural Constraints Enforced by Tests

Architecture is not just convention. Tests enforce it.

### Package edge rules

Forbidden imports include:

- `parser` must not import `analysis`, `compiler`, `validator`
- `analysis` must not import `compiler`, `validator`
- `semantics` must not import `compiler`, `validator`
- `contentmodel` must not import `parser`, `analysis`, `complexplan`, `compiler`, `semantics`, `validator`
- `complexplan` must not import `parser`, `contentmodel`, `compiler`, `semantics`, `validator`
- `validator` must not import `parser`, `analysis`, `compiler`, `semantics`
- `validatorbuild` must not import `compiler`, `semantics`, `validator`

Only one alias import is allowed in repo: `xsderrors`.

### Root-shell policy

- root public API is allowlisted
- root source-set files must be `sourceset.go`, `sourceset_entry.go`, `sourceset_prepare.go`
- `internal/compiler` and `internal/validator` shell packages must not define type aliases
- leaf packages under `internal/compiler/...` or `internal/validator/...` must not import their parent shell package

### File ownership policy

Tests pin major file layouts, including:

- phase packages must keep `doc.go`
- content-model files belong in `internal/contentmodel`
- complex-type-plan files belong in `internal/complexplan`
- validator artifact files belong in `internal/validatorbuild`
- streaming packages have fixed file ownership splits

### Structural rules

- pointer-to-slice types are forbidden repo-wide
- retired names like `SchemaSet` and old package names are forbidden in docs and `doc.go`

## CLI

`cmd/xmllint` is thin:

- requires `--schema`
- validates exactly one XML document
- maps CLI instance token limit to `ValidateOptions`
- prints sorted validation diagnostics from public `errors` package
- supports CPU and heap profiling flags

No extra architecture lives there.

## Current Architectural Read

Current design priorities are consistent across code and tests:

- tiny stable public shell
- strict internal package boundaries
- deterministic compilation order and IDs
- immutable compiled runtime
- pooled mutable validation sessions
- streaming-first XML handling
- explicit option propagation per phase
- boundary tests used as architecture guardrails

Main tradeoff is code volume. Project favors explicit phase-local packages and many small files over fewer large abstractions. That keeps dependencies narrow and behavior testable, but increases navigation cost.

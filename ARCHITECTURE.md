# Architecture

This is the canonical architecture contract for contributors. It describes the
current target state and the rejected alternatives whose constraints still
apply, not migration history.

The public import path is `github.com/jacoelho/xsd`. Root package files should
stay small and should not accumulate compiler, runtime, source-resolution, XML
streaming, formatting, or diagnostics implementation.

## System Model

The system has two execution planes joined by one immutable semantic kernel:

```text
explicit schema sources
        |
        v
source acquisition -> schema compilation -> sealed schema
                                              |
                                              v
                                    immutable schema.Schema
                                              |
        +-------------------------------------+
        |
        v
instance XML -> document-local validation -> structured diagnostics
```

The schema plane performs I/O, closes the source graph, compiles XSD semantics,
and commits exactly once by publishing a `schema.Schema`. The document plane
streams one XML document, reads that schema, owns all mutable assessment state,
and discards document state before returning. Neither plane reaches through the
other: validation never mutates or reconstructs schema semantics.

The published schema is the system's narrow waist. The schema-producing side
turns external syntax into validated canonical facts; the document-consuming
side applies those facts without knowing how they were derived. This shape
permits one expensive compile followed by many isolated validations and makes
concurrency a property of immutable sharing rather than coordination.

The package graph follows capability ownership:

| Module | Authoritative responsibility |
| --- | --- |
| `internal/source` | Bounded acquisition, source identity, explicit reference resolution, and cleanup |
| `internal/xmlstream` | XML 1.0 tokenization, namespace admission, borrowed data, and retained namespace contexts |
| `internal/schema` | Typed schema sources, closed source graph, component compilation, private construction state, and immutable schema reads |
| `internal/value` | XSD lexical admission, values, type derivation, facets, typed equality, and value scratch |
| `internal/xsdregex` | XSD regular-expression syntax, bounded compilation, and matching |
| `internal/diagnostictext` | Bounded, escaped diagnostic excerpts; no validation policy or message templates |
| `internal/validate` | Document assessment, recovery, identity state, and reusable sessions |
| `internal/format` | Repository-internal formatting and its input/output limits |
| `internal/lex`, `internal/vocab`, `internal/uriref`, `xsderrors` | Shared lexical facts, vocabulary, URI-reference semantics, and structured errors |
| `internal/valuebench` | Value-program benchmarks over the `internal/value` interface; no production code |
| Root `xsd` | Sources, options, engines, and sessions delegated to their owners |
| CLI, WASM, browser, and local web adapters | Wire translation, product composition, and process lifecycle |

Schema compilation never depends on document validation. Validation reads the
published schema and value program. Value semantics do not depend on schema
source syntax, XML streams, or document assessment. The source package performs
library file I/O; the library does not perform network I/O.

Each module must be deep: callers learn one small interface while the module
owns the correlated state, sequencing, limits, and failure behavior behind it.
A new seam is justified only by a present variation or ownership break. A
pass-through seam that merely renames data or forwards calls must be deleted.

## Control Surfaces And State Machines

The system has no hidden operational control plane. Callers control admitted
work through explicit sources and finite options; modules expose results and
structured errors rather than mutable internal state or log-dependent outcomes.

| Control surface | Controls | Does not control |
| --- | --- | --- |
| `SchemaSource` and `Resolver` | Exact schema bytes, repeatability, identity, and explicit resolution | Network discovery or instance-directed loading |
| `CompileOptions` | Source, graph, name, dependency, content-model, substitution, and union work | Cancellation of caller-owned blocking I/O |
| `ValidateOptions` | Errors, identity state, depth, attributes, text, tokens, input bytes, and per-value work | Schema mutation or dynamic schema loading |
| `Engine` | Safe concurrent schema reuse and one bounded idle session | Shared active document state |
| Reusable `Session` | One owner's bounded scratch reuse | Overlapping calls or cross-session coordination |
| `xsderrors` | Stable category, code, cause, and source/document location | Policy inferred from message strings or logs |
| Browser worker generation | Cancellation, timeout, latest-request ownership, and stale-result suppression | Library-level asynchronous execution |

The principal state machines are deliberately linear or owner-local:

| Owner | States and only legal progress | Failure/cleanup invariant |
| --- | --- | --- |
| Compiler | normalize -> load closed graph -> plan/index -> compile/finalize -> seal | No engine before publication; failed sealing does not consume the retryable build |
| Engine session pool | normalize -> checkout or allocate -> validate -> reset -> retain or drop | At most one idle session; concurrent calls never wait; a panic drops the checked-out session |
| Validation session | idle -> guarded document -> semantic or syntax-only processing -> reset -> idle | Overlap fails before input; every exit clears document references before releasing the guard |
| Element start | prepared -> XML/namespace committed -> semantic commit | Fatal failure rolls back every staged owner; semantic stop retains only syntax state needed to finish parsing |
| XML stream | reset -> borrowed token -> advance/invalidate -> EOF/error -> detach | Borrowed bytes never survive advance; only token-boundary EOF is success |
| Namespace frame | prepare -> commit -> end or abort | The opaque top-frame capability is the sole pop authority |
| Schema publication | mutable build -> validate -> consume/seal | Validation preserves build data; work stays charged; successful consumption is the only commit |
| Browser client | loading -> ready -> running -> ready/failed/disposed | Only the owning generation publishes; termination owns cancellation and timer cleanup |

## Authority And Navigation

Use the narrowest authoritative source that answers the question:

1. Explicit requirements, XSD 1.0, XML 1.0, and exported behavior are binding.
2. This file owns cross-module architecture: module ownership, boundaries,
   contracts, data flow, state transitions, lifecycle, resource policy, and
   rejected alternatives.
3. Code, Go doc comments, and tests own package-local mechanisms and prove the
   current implementation. A conflict with this file is architecture drift to
   resolve, not a second design.
4. `README.md` owns public usage. `docs/spec` records specification source
   authority, adopted corrections, and interpretation decisions; it bundles no
   specification copies. `tests/README.md` owns corpus and test-harness
   operation.
5. Plans describe proposed work and may not redefine current architecture.

For a change, start at the public or package interface named in the routing
table below, trace the single execution path to its state owner, then inspect
all callers, readers, writers, failure exits, limits, tests, and relevant history. Read
only the detailed module and flow sections reached by that route.

| Change concerns | Start at | Authoritative owner | Required adjacent inspection |
| --- | --- | --- | --- |
| Public compile, validation, source, option, or session behavior | root `compile.go`, `session.go`, `source.go` | root `xsd` interface; delegated policy remains internal | Public examples/tests, `xsderrors`, option normalization, affected capability |
| Diagnostic category, code, aggregation, location, or presentation | `xsderrors/errors.go` | `xsderrors` | Every constructor caller and public diagnostic tests |
| Source opening, closing, resolution, identity, URI composition, or byte accounting | `internal/source`, `internal/uriref` | `internal/source` for acquisition; `internal/uriref` for valid references | Compiler graph loading, source tests, import-graph enforcement |
| XML syntax, positions, buffering, borrowed data, or declaration policy | `internal/xmlstream`, `internal/lex` | `internal/xmlstream` | Compile, validate, format consumers; stream boundary tests; parser fuzz/benchmarks |
| Namespace admission, lookup, rollback, or retained contexts | `internal/xmlstream` | `internal/xmlstream` | Stream lifetimes, compile/validate/format callers, namespace churn benchmarks |
| Schema syntax, graph planning, component semantics, or compilation budgets | `internal/schema` | `internal/schema` | `schemaBuild`, conformance corpus, publication, focused compile benchmarks |
| Published components, complex derivation, wildcards, substitution, or content algorithms | `internal/schema` | `internal/schema` | Both compile and validate callers, publication corruption/alias tests, schema/validation benchmarks |
| Simple-type derivation, lexical admission, facets, or value equality | `internal/value` | `internal/value` | Schema literal admission, instance validation, datatype corpus, value and public validation benchmarks |
| Element/attribute/content/XSI/identity behavior, recovery, or reusable-session state | `internal/validate` | `internal/validate` | Schema reads, public validation tests, corpus, race tests, affected allocation benchmarks |
| Formatting | `internal/format` | `internal/format` | Shared stream/namespace contracts, WASM adapter, writer failure tests |
| WASM, worker, page, or local-server lifecycle | `cmd/wasmxsd`, `web/js`, `cmd/xsdweb` | The narrowest listed adapter | Tagged response tests, worker generation/timeout tests, browser integration |
| A finite limit or retained allocation | Declaring option/constant, then allocating symbol | The module that first admits or retains the work | Below/at/above-limit tests, rollback/reset, opposing-shape allocation and retention evidence |

## Public Packages

- `github.com/jacoelho/xsd` is the facade package. It owns the exported
  compile, validate, session, option, and schema-source API, and adapts public
  options, source wrappers, resolvers, and sessions to internal types. The
  adapter normalizes nil resolver forms before they reach `internal/source`.
- `github.com/jacoelho/xsd/xsderrors` owns public diagnostics: structured
  immutable errors, aggregates, the category/code catalog, location decoration,
  the `Flatten` presentation projection, and conservative `IsConclusiveValidation`
  classification. The two traversals are separate; their rules are in godoc.

Root `xsd` MUST NOT expose root-level diagnostics or formatting types/functions;
those belong to `xsderrors` and `internal/format`.

## Internal Packages

Internal packages MUST NOT import root `xsd`. Value and lexical packages MUST
remain below schema and validation packages.

### `internal/source`

`internal/source` owns schema source primitives (bytes, opener, file), bounded
acquisition, resolver adaptation, built-in local/generic fallback, resolution
context, and source identity (`Key`). It owns no XSD vocabulary or graph policy.

- `Source.OpenInput` is the only opening path and `Source.ResolveFrom` the only
  reference-resolution path. `internal/schema` has one source-loading path that
  must not bypass or duplicate either.
- Content identity is `ReadResult` byte count plus SHA-256, compared by
  `internal/schema` across reopenings: a collision-resistance assumption, not
  exact equality. `Source.StaticContent` exposes the same identity of an
  in-memory source without opening it; it applies no byte limit.
- Resolver spelling and built-in fallback capability are tracked separately
  (`ReferenceBase`); a valid reference a backend cannot represent is an
  unavailable optional hint, not an error.

### `internal/uriref`

`internal/uriref` owns XSD 1.0 URI-reference validity after XLink escaping,
fragment syntax, and raw-preserving RFC 2396 composition. `Reference.Raw` is the
datatype value (`internal/value`) and custom-resolver spelling;
`Reference.Escaped` serves only `internal/source` built-in fallback. Local
source names and Unix paths never enter `Reference`; a scheme-bearing source
name is parsed only as a URI base.

### `internal/schema`

`internal/schema` owns schema syntax and diagnostics, graph loading and
planning, chameleon contexts, component compilation, and immutable publication.
It parses `internal/source` inputs through `internal/xmlstream` into a closed
graph before compiling; planning performs no I/O. `internal/value` owns builtin
IDs, effective facets, literal evaluation and NOTATION constructor-use IDs;
schema owns facet admission, final constraints and schema-use policy.
`MaxSchemaDependencySteps` bounds loading, planning and resolution;
`MaxContentModelAnalysisSteps` bounds consistency, restriction, UPA,
compilation and sealing. Spent work is never refunded. Retained derived records
are charged to `MaxSchemaDependencySteps` at their admission owner:
`appendModelParticles` is the only particle append and charges one step per
particle before allocating; `addModel` (one step per record) and
`completeModel` write the model table, and `completeModel` installs only
particles already charged; `addAttributeUseSet` charges `len(Uses)` and
`appendWildcard` charges `len(Namespaces)`. Admitted models are immutable and
`contentModelByID` shares them on read. A new retained table must charge in its
admission function. Admission functions return unlocated budget errors.
`enterComponent`'s leave fills the missing location of any diagnostic leaving a
component from that component's node, so a failure is located at the innermost
component being compiled, not at the referring site; a charge made at a
reference before the scope opens keeps the reference location. A `schema.limit`
failure raised after loading and outside every component scope is set-wide:
target-context planning locates it at the document being added or whose
references it is walking, and `compileMappedSources` locates the rest (name
indexing, substitution closure, consistency checks, publication audit) at the
first explicit root document in plan order, which is by document name. Loading
locates its dependency charge at the reference being resolved and a
`MaxSchemaReferences` failure at the document whose references exceed it.
Publication consumes the
build only on success; `*schema.Schema` exposes only immutable derived reads.

`internal/schema`, `internal/value` and `internal/validate` export only
identifiers consumed outside their package: for schema the published read
surface plus `Compile`, `CompileMappedSources`, `NormalizeOptions`,
`Options` and `Limits`. Types in exported signatures stay exported; everything
else is unexported; external test packages reach internals
only through aliases in `export_test.go`. Fixed-value comparison, for attributes
and elements, is owned by `internal/validate` (`fixedValueEqual`, value space,
identity required).

Contracts `internal/validate` relies on:

- `ElementFrameRead`: effective-type text policy, simple content and initial
  content state; the declaration, not the `xsi:type`, owns default/fixed.
- `ContentTransition.Commit` owns the stale-state check; validate commits once,
  after identity commit readiness.
- `AttributeUseSetRead`: ordered required and value-constraint slots and
  `HasIDUse`; XSI value constraints are omitted ([I-007](docs/spec/INTERPRETATIONS.md#i-007--xsi-attribute-uses-and-psvi-contributions)).
- Value constraints carry an owner-typed cache, application text and a
  namespace projection; under a changed `xsi:type`, validate revalidates the
  text locally.
- Identity dispatch programs are the identity read surface; validate owns only
  active scopes and scratch.

Caller-visible policy: XSD/XSI references need no import ([I-006](docs/spec/INTERPRETATIONS.md#i-006--xsdxsi-schema-references));
XML/XLink declarations come only from supplied sources; XSI attribute
declarations are intrinsic; a member with a missing substitution head validates
as `anyType` ([I-023](docs/spec/INTERPRETATIONS.md#i-023--substitution-group-member-with-a-missing-head)).

### `internal/value`

`internal/value` owns simple-type semantics as one immutable program: lexical
normalization, primitive values, lists, unions, facets, canonical text, typed
equality, and ID/IDREF projections. Schema declarations keep only component
metadata and value-program type IDs. No other package parses simple values,
evaluates facets, or defines value equality. Fixed-grammar boolean syntax in
schema documents and `xsi:nil` is parsed with `ParseBooleanValue`; only
`minOccurs`/`maxOccurs` and facet integer arguments are parsed by their syntax
owner, `internal/schema`, because no value-owned parser exists for them.

- **Construction.** The schema compiler drives `Builder` (`Reserve`,
  `Complete`, `Seal`) and supplies its construction limits. Compile-time
  literals (`Builder.Validate`) and instance values have the same value
  semantics. Restriction derivation returns the effective facet program, and
  the compiler does not reconstruct that merge. `FacetAllowed` is the only
  facet-applicability rule for schema admission, value construction, and
  diagnostics.
- **Inputs.** `ValidateBytes` and `ValidateAdmitted` trust that
  `internal/xmlstream` has already admitted UTF-8 XML 1.0 character data.
  `Validate` admits external text itself. All paths share value semantics and
  work accounting.
- **Work bounds.** Instance validation supplies `MaxInstanceValueWork` for each
  value. Its default is `defaultMaxInstanceValueWork` in `internal/validate`,
  listed in `docs/RESOURCE_LIMITS.md`, and it does not depend on compilation
  options. The sealed program keeps no caller limits or work counters.
  `ErrLimit` failures keep their schema or instance diagnostic category at the
  calling boundary. `internal/schema` translates every value failure through
  one function, `schemaValueError`: `ErrMetadata` is an internal invariant,
  then limits are `schema.limit`, then unsupported diagnostics and existing
  structured diagnostics pass through, and any other failure keeps its value
  reason under the caller's code. Whitespace loosening is rejected only by the
  value Builder's derivation check.
- **Pattern work.** Pattern matching draws on two accounts.
  `EvaluationLimits.PatternWork` is the per-value budget, supplied by
  `MaxInstancePatternWork` at instance time and `MaxSchemaPatternWork` for
  compiler literals. One per-value budget is shared by every pattern group,
  list item and union attempt of a top-level evaluation; each bound,
  enumeration, fixed or default literal is its own top-level evaluation, even
  where one type's facet literals share traversal work. It bounds one value,
  not a document. `EvaluationLimits.PatternTotal` points to a caller-owned
  `value.PatternAccount` that sums pattern work over many evaluations. The
  validate session owns the per-document account (limit
  `MaxInstanceTotalPatternWork`) and resets it at every `session.validate`
  entry, including after a document that failed mid-stream. `value.Builder`
  owns the per-compilation account, configured by `internal/schema` through
  `BuilderOptions.MaxTotalPatternWork` (`MaxSchemaTotalPatternWork`). Every
  compile-time evaluation through `Builder.Validate` or `Builder.Complete`
  charges it, including a facet literal's base-type check. There is no
  uncharged evaluation path, so `MaxSchemaTotalPatternWork` bounds all
  compile-time pattern matching. A facet literal can be charged for its
  base-type check and again when facet compilation re-evaluates it; a value
  constraint is charged once. Publication audits element and attribute value
  constraints structurally (shape, QName context, resolved-name proof
  membership) and never re-evaluates them. The
  account is passed by pointer from its owner at each call and never
  snapshotted, so the XSI identity path receives `EvaluationLimits` built from
  the live session, not from `identityEvaluation.limits`. Each match runs with
  the smaller of the two remainders and a limit failure charges the granted
  work to both. Exhausting the per-value budget returns `ErrPatternLimit`;
  exhausting the account returns `ErrPatternTotalLimit` only when its
  remainder was strictly smaller, so ties name the per-value option. Both wrap
  `ErrLimit`, so each plane names the option its caller can raise. Defaults are
  finite and owned by their planes (`internal/validate`, `internal/schema`);
  the instance total default is the per-value default times
  `MaxInstanceBytes / MaxInstanceTextBytes`. `Program.Validate*` rejects a zero
  `EvaluationLimits` field, a nil `PatternTotal` or a zero account limit, and
  `BuilderOptions.MaxPatternWork` and `MaxTotalPatternWork` have no default;
  each is `ErrMetadata`.
- **Input requirements.** Callers read one scalar `InputRequirements`
  projection for QName context, document identity, and raw-text comparison
  eligibility (`UnconstrainedString`, `LexicalCanonical`). They never read
  facet metadata. Builder-only `TypeView`s give the compiler component
  metadata.
- **Document identity.** The value program's ID/IDREF carriers are the only
  type-level ID-ness predicate
  ([I-016](docs/spec/INTERPRETATIONS.md#i-016--recursive-id-ness)). Schema
  value-constraint and attribute-use checks, wildcard counting, identity
  activation, and instance ID accounting read it through `InputRequirements`.
  Each validated `Value` carries its own ID/IDREF projections, recorded even
  when no identity constraint asked for a value. Static metadata cannot
  replace them.
- **QName resolution.** `ExpandedName` is both the `QNameResolver` result and
  the QName/NOTATION payload. Compilation, facet-literal proof replay, and document
  assessment supply resolvers under that one contract. Value revalidates the
  result and owns the NOTATION declaration check.
- **Equality.** Equality uses the value space (`Value.Equal`, `IdentityKey`).
  Text projections never define equality, and duration has no canonical form.
  Fixed-value checks (validate-owned) and schema value-constraint identity checks
  compare typed constraints with `Value.Equal` only; canonical text is compared only when a
  constraint has no value type.
  A retained `Value` stays valid after its scratch is reused or reset.
- **Patterns.** Pattern syntax, the fixed compile bounds and the match work
  unit belong to `internal/xsdregex`; the match work unit is defined on
  `xsdregex.MatchOptions` and restated for callers in `docs/RESOURCE_LIMITS.md`;
  work is a function of pattern and input only. Value owns pattern grouping (alternatives
  within one restriction step, and every inherited step applies) and the
  per-value pattern budget described under **Pattern work**. The aggregate
  `MaxSchemaPatternRetainedBytes` budget and the per-compilation pattern cache
  keyed by pattern text belong to `internal/schema`; identical text is
  compiled and charged once.

### `internal/xsdregex`

`internal/xsdregex` owns XSD 1.0 whole-input pattern semantics, from syntax
through bounded compilation and matching. It does not translate patterns into a
host regexp dialect. XML input admission belongs to `internal/xmlstream`, so
match callers must pass valid UTF-8 XML text. Compilation uses fixed
xsdregex-owned structural bounds (constants, not options; numbers in
[RESOURCE_LIMITS.md](docs/RESOURCE_LIMITS.md)), because the NFA expands counted
repeats and precomputes closures. The schema plane reports a compile-bound
failure as `schema.limit`. The state bound limits the number of compiled
states, and so every match-time state set; it never limits input length.
Per-pattern bounds do not bound aggregate memory. `Pattern.RetainedBytes` is
the admitted measure: an estimate covering instructions or linear atoms,
precomputed closures and closure bitsets, the source and the character ranges,
weighted by byte constants that tests keep at least `unsafe.Sizeof` of what
they estimate. Every retained derived structure must have a term there. Ranges
are charged once per backing array, by capacity; the over-count for subslices
is deliberate. `internal/schema` sums the estimate over distinct pattern
texts. Matching takes only a work budget (`MatchOptions.MaxWork`)
whose unit definition lives in the `MatchOptions` doc comment and reports the
work it used. `internal/value` charges that work to its per-value pattern
budget and maps exhaustion to `ErrPatternLimit`. Compiled patterns are
immutable and safe to share. Scratch is owned by the caller, and no
process-wide cache keeps patterns. The block catalog leaves out the XSD 1.0
surrogate block names, including in complemented escapes. Corpus expectations
follow that rule, and upstream disagreements are recorded as provenance in the
authoritative test manifest.

### `internal/validate`

`internal/validate` owns instance validation: limits and option normalization,
XML reader preflight and parser error classification, recovery, document
structure, element selection and start/end decisions, attributes, content,
simple-content assessment, XSI handling (schema-location hint checking belongs to attribute
assessment), and the
document-local identity evaluator (IDs, IDREFs, key/unique/keyref). Session,
pool, and element-start lifecycles are the **Engine session pool**,
**Validation session**, and **Element start** rows of the state-machine table.

Cross-module contracts:

- It reads only the sealed `*schema.Schema`; immutable schema tables never hold
  validation work buffers, so one schema serves concurrent sessions.
- `internal/schema` publishes identity selector/field programs, a dispatch index
  without self branches, `SelectsSelf`, and `AttributeFieldBaseDepths`;
  validate owns all matching state, scopes, and tables.
- `internal/schema` strips XSI attribute-use defaults and fixed values, so the
  four intrinsic XSI attributes use dedicated assessment only.
- Schema-location hints never participate in component selection. On assessed
  elements each hint item is checked as anyURI during attribute assessment
  (xsi:schemaLocation is a list of anyURI of any length; no pair-count rule);
  skip-processed content is not checked; no hint state is retained. An assessed
  element whose start fails fatally before attribute assessment gets no hint
  diagnostic. Missing roots, types, and
  wildcard declarations are ordinary assessment errors against the compiled
  component set.
- A failed selection never contains its subtree: after its diagnostic, the
  element is assessed laxly (cvc-assess-elt), as Xerces does, using its global
  declaration when one exists, otherwise anyType; an unavailable type drops the
  declaration (Structures §5.3). Descendant diagnostics therefore follow the
  selection error.
- QName and NOTATION spellings use the `internal/xmlstream` reader's bounded
  string cache, but resolve against the current namespace frame each time.
- Session validation and `CheckXMLWellFormed` start the reader through one
  limit projection (`Limits.readerConfig`, `resetInstanceReader`), so both
  enforce and report the same instance syntax limits. `session.setLimits` is
  the only writer of session and identity limits, for fresh and pooled
  sessions alike.
- Instance diagnostics are located by their owner. A site attaches a location
  only when it holds one the session cannot derive: stored identity
  locations, reader-positioned stream errors, and parent-content diagnostics
  (located on the parent at the child's position). Pre-commit start
  assessment returns unlocated errors that `locatePendingStart` locates on the
  admitted start (`Reader.StartLocation`). It receives the selected mode from
  `recoverElementStartAssessment` and, through `sessionStartTransaction.fail`,
  from `runStartTransaction`, so a start renders one path segment before and
  after commit. The exception is a fatal failure before selection: it renders
  the lexical segment and ends the document.
  `sessionStartTransaction.fail` and `validateTokens` locate every remaining
  unlocated diagnostic, document-end diagnostics use the reader position, and
  diagnostics are located before they are joined. Every validation or
  unsupported diagnostic produced while reading a document therefore carries a
  path and line; the corpus harness and the validation fuzz target assert it.
  A nil reader and option errors have no document position by design.
  `MaxIdentityEntries` bounds stored entries, pending selector matches, and
  pending field slots independently, as `ValidateOptions` documents.

### `internal/format`

`internal/format` owns repository-internal XML formatting and its finite
input, token, node, depth, and output bounds. It consumes `internal/xmlstream`,
is called by `cmd/wasmxsd`, and exposes no root-package API. Success means the
complete document was written.

### `internal/xmlstream`

`internal/xmlstream` owns XML 1.0 tokenization, declaration and prolog policy,
line-ending normalization and positions, namespace admission and retained
contexts, duplicate-attribute detection, and document topology. Schema parsing,
instance validation, and formatting share one `Reader` contract:

- `Reader.Next` returns one reader-owned, borrowed, read-only token; start and
  end admission consume that current token, never caller copies. Borrowed bytes
  are invalid after the next advance, `Reset`, or `Detach`; retained values must
  be materialized through the reader's caches.
- The reader owns rejection of DTDs and forbidden outside-root data and reports
  neutral `ErrorKind` failures; consumers translate them into their own
  diagnostics.
- The reader owns the closed unsupported-feature classification
  (`UnsupportedFeatureOf`: DTD, non-UTF-8 encoding, XML version, undeclared
  entity reference); a reader error has `ErrorUnsupported` kind exactly when it
  classifies. Schema, instance, and format consumers map every feature in one
  exhaustive switch and own only the message text, so a new feature fails the
  `exhaustive` lint in each consumer instead of degrading to a syntax error.
- The reader owns the closed limit classification (`Limit`, `LimitOf`:
  input bytes, token bytes, attributes, depth); a reader error has
  `ErrorLimit` kind exactly when `LimitOf` classifies, and there is no separate
  depth kind. Negative limits are `ErrorState` configuration failures, not
  exceeded limits. Schema, instance, and format consumers map every `Limit` to
  their own option name in one exhaustive switch (`schemaReaderLimitError`,
  `readerLimitOption`, `formatReaderLimitErr`) and report
  `<option>=<n> exceeded`, so a new limit fails the `exhaustive` lint in each
  consumer instead of degrading to a syntax error. Each consumer has one
  reader-error mapping that classifies limits first: validate `streamError`,
  schema `schemaStreamError`, format `classifyLimit`. Consumers do not re-check
  limits already handed to the reader; `internal/schema` owns only the
  cross-token measure the reader cannot see, character data accumulated in one
  element under `MaxSchemaTokenBytes`.
- The reader owns the exclusive input-failure classification
  (`IsInputFailure`): a failure of the caller's `io.Reader` (read error, no
  progress, nil reader) rather than of the document. It is false whenever
  `LimitOf` or `UnsupportedFeatureOf` classifies the error, so a limit error
  joined with a read error stays a limit and precedence is owned here, not by
  consumer branch order; `Error.Kind` is `ErrorInput` exactly when it is true.
  `internal/validate` maps it to `validation.read`. `internal/schema` and
  `internal/format` do not: `source.Input` owns schema read failures
  (`schema.read`), and format reads in-memory text.
  End of input is `io.EOF` itself, decided once in `byteStream.readUnderlying`;
  every other reader error, including one that wraps or joins `io.EOF`, is an
  input failure, the same rule `source.Input` applies to schema sources. `IsEOF`
  is the only end-of-input predicate.
- `CharacterDataKind` preserves lexical origin (literal text, references,
  CDATA). Format uses it to keep CDATA whitespace; compile and validate treat
  all kinds as character data.
- Attribute values are read only through `Reader.AttrValue`, which returns an
  owned string while the attribute's token is current. A value skipped by
  `Config.SkipOrdinaryAttrValues` fails with `ErrAttrValueSkipped` instead of
  reading as empty; `NewAttr` builds owned values without a reader cache.
  `Attr.RawValue` is a borrowed, current-token-only view guarded by
  `TestXMLStreamBorrowedValuesStayEphemeral`
  (`tests/stream_boundary_test.go`).
- Comment mode is chosen per consumer: validation uses `CommentModeDiscard`,
  schema parsing `CommentModeBoundedDiscard`, formatting `CommentModeEmit`.
- `Context` values are immutable and outlive frame close and `Reset`; consumers
  use them for QName resolution after the element is gone.
- Relative namespace names are an admitted extension and keep their spelling.

## Data Flow

Compilation flow:

1. Public callers provide `xsd.SchemaSource` values.
2. Root `xsd` converts them to immutable or repeatable `internal/source.Source`
   values.
3. `internal/schema` loads a closed source graph, then plans, indexes, and
   compiles it. Document-local XSD admission, including URI-reference syntax,
   precedes resolving a document's references, so malformed references fail
   before resolver invocation; each resolved include/import edge is
   target-namespace checked before its resolver context or descendants are
   activated. Explicit source descriptors are bounded before facade conversion;
   the loader's loaded-document table is the only document-count ledger: a
   document counts once, when it is parsed and registered, so an explicit file
   that is also imported counts once and an unavailable target never counts (the
   document that exceeds the count is opened, closed unread and rejected). It applies inherited
   `xml:base` per resolver context, accounts every source byte and failure
   stage through `internal/source`, and parses through `internal/xmlstream`
   under XML namespace and resource limits. An opener source reached
   again is reopened and charged again; a File source whose file was already
   read for the document is not reread (`Source.ContentFixedByName`); an
   in-memory source is compared by its static digest (`Source.StaticContent`)
   without charge. Differing content is
   an error. `MaxSchemaTotalBytes` failures are located at the referring
   include/import element, or at the source name for an explicit source.

   Planning applies src-include and src-import checks to every chameleon
   target-context view against its effective target namespace
   ([I-021](docs/spec/INTERPRETATIONS.md#i-021--chameleon-reference-checks-use-the-effective-target-namespace)).
   Schema document identity is the resolved source key (I-019): one key
   reached through several edges is one document, while distinct keys are
   distinct documents even with identical bytes, so callers that want copies
   treated as one document canonicalize them in their resolver. Compilation
   populates one compiler-owned build.
4. Private schema publication seals and consumes that one build.
5. Root `xsd.Engine` stores a shared `validate.SessionPool` owning that sealed
   validation schema.

Validation flow:

1. Public callers validate through `Engine.Validate`, `ValidateWithOptions`, or
   a reusable `Session`.
2. Root `xsd` adapts public validation options.
3. Root `xsd` delegates construction and validation to `SessionPool.NewSession`
   and `SessionPool.Validate`; the pool owns schema and scratch lifecycle.
4. `internal/validate` reads immutable schema facts through methods on the
   sealed `*schema.Schema` and applies instance-validation policy; schema table
   execution and metadata checks stay in `internal/schema`.

Compilation and validation are synchronous. Resource limits bound admitted input
and retained work, but the library does not own deadline or interruption policy.
Callers that require interruption of a blocked resolver, opener, file operation,
or arbitrary `io.Reader` MUST provide I/O that they can close or otherwise
unblock.

Formatting flow:

1. The WASM adapter passes its bounded XML string to `internal/format`.
   The formatter accepts this immutable supplied input directly; it does not
   acquire or buffer a reader's complete contents.
2. One `internal/xmlstream` reader validates the complete input and records
   bounded source spans and layout decisions; malformed input writes nothing.
3. Rendering replays those spans from the caller's immutable string. Writer
   failures may leave partial output; the WASM adapter discards its output
   builder on any error.
4. Formatting does not belong in the root public `xsd` package.

Diagnostics flow:

1. Internal packages return source-aware structured errors using public
   `xsderrors` types; schema diagnostics identify the originating schema through
   `xsderrors.Error.Path()`. Instance paths are rendered on demand by the
   `xmlDocument` path owner in `internal/validate` through
   `internal/diagnostictext`: each name is cut at 128 bytes and escaped in the
   same dialect as quoted values, and a path over 4096 bytes keeps whole
   leading and trailing segments around `/...`. The document caches one
   unelided rendered prefix of the open-element path, at most 4096 bytes, so
   diagnostics under one ancestor chain share a rendering; elided paths are
   rendered per diagnostic. Retained diagnostic text stays bounded by
   `MaxErrors`.
2. Public callers inspect `xsderrors.Error`, `xsderrors.Errors`,
   `xsderrors.Category`, and `xsderrors.Code`. Constructors reject invalid
   category/code combinations. Aggregate construction owns its input, accessors
   do not expose mutable storage, and `xsderrors.Flatten` is the sole top-level
   presentation projection.

   `internal/value/errors.go` owns value-validation message text, error
   identities, and diagnostic formatting. Evaluators return cheap failure
   identities, and diagnostics are enriched only after the complete value has
   failed, because a rejected union member can be part of a successful path.
   Enrichment never replays QName/NOTATION resolvers or resets evaluation work.

   `internal/validate/errors.go` owns document-assessment messages and the
   shared translation of value failures: metadata, limit and unsupported
   failures take precedence over ordinary assessment errors, in that order.
   Ordinary value and XSI callers supply their own assessment code and work
   limit; URI failures keep their URI-specific message without a value-error
   suffix. Content expectations come from `internal/schema`'s bounded read-only
   projection of the current compiled state (`Schema.ExpectedContent`), so the
   diagnostic layer never reimplements content matching. Published schema
   diagnostic names are cold immutable projections copied and charged during
   publication; they do not enlarge frame, session, or hot component-read
   state. Identity errors name constraints and one-based fields, and deferred
   keyrefs name their retained target constraint; they never add source IDs or original tuple lexemes
   to document state, and opaque identity keys are never presented as values.

   `internal/diagnostictext` owns bounded quoting for all these owners.
   Diagnostics never retain borrowed input. `xsderrors` defines absent
   diagnostics and wrapper preservation in its package documentation. Unsupported DTD diagnostics retain the positioned
   stream cause as their sole message source.
3. `xsderrors.WithLocation` is the only path/line/column decorator. Root `xsd`
   and formatter packages do not duplicate diagnostic types or codes.
4. Missing document roots and forbidden outside-root character data use
   `CodeValidationXML`; a failing or nil caller reader uses the inconclusive
   `CodeValidationRead`. `CodeValidationRoot` and `CodeValidationText` describe
   schema assessment failures. Fatal XML, limit, or operational errors take
   precedence over earlier recoverable diagnostics; adapters preserve the
   diagnostics returned by the core without reconstructing discarded history.
   A diagnostic is recoverable exactly when
   `xsderrors.IsConclusiveValidation` holds: the collected diagnostics form
   the `invalid` result, so nothing inconclusive may be collected. `xsderrors`
   owns the per-code disposition in one exhaustive switch; validate adds only
   the identity exclusion from element-assessment failure.

Browser flow:

1. `cmd/wasmxsd` owns the JavaScript-facing tagged response contract and the
   generated limit catalog. Validation responses are exactly one of `valid`,
   `invalid`, or `error`; formatting responses are `ok` or `error`.
   `valid` has neither error field. `invalid` carries a nonempty `errors` array
   and means conclusive schema violations after successful compilation and XML
   admission. `error` requires a nonempty summary and may carry a nonempty
   diagnostic array: compilation, malformed XML, unsupported features, resource
   limits, invalid options, session failures, and unknown failures prevent
   assessment. Fatal failures dominate mixed aggregates. Reaching `MaxErrors`
   alone remains invalid; a later fatal parser failure is an error. Compilation
   failures retain the bounded well-formedness check and XML-first diagnostic
   ordering. The page displays structured diagnostics for either unsuccessful
   branch and formats only valid documents.
2. `web/js/xsd-worker.js` wires the worker; `web/js/xsd-runtime.js` owns the
   Go WASM runtime lifecycle. Any settlement of the Go program's `run` is
   fatal; startup failure and run settlement together post at most one
   `fatal` message, so a runtime exit restarts the worker instead of failing
   every later request. WASM
   functions and limits never enter the window global scope. One runtime-owned Go
   `validationAdapter`, captured by its JavaScript callback, retains one
   successfully compiled engine and its exact XSD text. Compilation uses the
   fixed `schema.xsd` source identity, default compile options, and no resolver;
   those fixed inputs plus exact text define the key. Type and byte admission
   precede cache access. A changed admitted key evicts before compilation, which
   publishes only on success; failed compilation leaves the cache empty.
   Instance failures preserve the cached engine. Worker termination releases the
   runtime and cache together. There is no process-wide cache, negative caching,
   synchronization, or retained key history.
3. `ValidationWorkerClient` owns worker lifecycle, a single active request, one
   latest pending request, cancellation, initialization and execution timeout
   termination, restart, and immutable state snapshots. The page ignores
   results whose input epoch is no longer current. The page owns editor/file
   generations so an older asynchronous file read cannot replace newer text,
   and applies UTF-8 byte and rendered-line bounds before syntax highlighting or
   gutter construction.
4. `cmd/xsdweb` serves only built assets (`web/` by default), binds to loopback
   by default, and is confined to that asset directory. It does not compile or
   validate schemas.

## Change Protocol

Every architectural or behavioral change is one dependency-complete packet:

- **Contract:** the observable requirement and its oracle.
- **Owner:** the one module and state owner responsible for the decision.
- **Invariant:** what must always hold before and after the change.
- **Path:** entrypoint, canonical representation, commit or acknowledgement
  point, failure exits, and cleanup.
- **Bounds:** time, temporary memory, retained memory, recursion, I/O, and
  concurrency implications at the admitting owner.
- **Proof:** focused behavior tests, affected seam tests including opposing and
  adversarial input shapes, and measurements for a changed hot path or retention
  shape.
- **Writeback:** the authoritative document, executable invariant, or local
  intent comment that prevents rediscovery.

Keep facts, inferences, and proposals distinct. Inspectable repository facts
need no prose cache. Expensive derived evidence may be recorded only with the
exact revision, command, environment, and status. A proposal stays in its plan
until implemented; the implementing change updates this file and its
enforcement and deletes the superseded plan text.

Every packet follows these design rules:

- Keep one canonical representation and one execution path. Replace a path
  atomically and delete superseded code instead of adding shims, feature flags,
  deprecated paths, or parallel state models, unless an external migration
  contract requires them.
- Make illegal states unrepresentable where practical. Validate and normalize
  external data once at entry; enforce invariants at construction and
  transition.
- Keep domain decisions deterministic and side-effect-free where practical.
  Isolate I/O, clocks, randomness, concurrency, storage, transport, and vendor
  formats at their owning edge. Presentation and transport translate or render;
  they own no domain policy.
- Keep dependencies explicit and acyclic and pass them directly. Add a narrow,
  consumer-owned interface only at a present substitution seam; use concrete
  types otherwise. Avoid globals, registries, hidden initialization, callbacks,
  service locators, and cross-module reach-through. Library code uses no
  reflection; prefer structural guarantees.
- Choose the smallest design satisfying current constraints. An abstraction
  must delete more complexity than it adds. Prefer small syntax duplication to
  false coupling; centralize duplicated policy and invariants.
- Use one term per concept. Name modules for owned capabilities, not vague
  roles such as `manager`, `core`, `common`, `engine`, or `utils`.
- Bound work, retained memory, queues, batches, recursion, retries,
  concurrency, and I/O at the admitting owner. Caches and projections remain
  reproducible from an explicit source and have bounded retention.
- Make ordering, acknowledgement, retry, cancellation, timeout, and
  partial-failure semantics explicit wherever asynchronous behavior exists.
- Prefer explicit control flow and actionable contextual errors. Never depend
  on logs to reconstruct a failure or collapse errors into ambiguous booleans.
- Prefer returning replacement pointers over double-pointer output parameters;
  explain exceptions at the declaration.

Test through the changed module's interface. Internal white-box tests are
appropriate for representations whose lifetime or corruption states cannot be
reached through a wider interface, but they supplement rather than replace
seam-level behavior. When a defect exposes split ownership, an invalid state,
duplicated policy, or unsafe sequencing, repair the model that permits the
defect class and delete the obsolete path.

An agent should be able to finish a packet without loading the repository-wide
history. The packet is complete only when a future agent can recover the current
contract from this file and the owner's doc comments, find the owner through
the routing table, reproduce the
proof from tests or commands, and see no competing current representation.

## Tests And Enforcement

Architecture is enforced by behavior rather than convention. Tests verify
observable contracts at the owning interface with implementation-independent
expectations and deterministic fakes. They exercise invariants, boundaries,
failure, rollback, reuse, concurrency, and resource limits. Delete
change-detector tests that mirror the implementation or assert their own setup.

| Enforcement owner | Contract enforced |
| --- | --- |
| `tests/phase_import_graph_test.go` | Package allowlists, dependency direction, context-free and reflection-free library code, source I/O ownership, and capability isolation |
| `tests/phase_boundary_test.go` | Required modules, single facade execution edges, session construction ownership, and retired public paths |
| `tests/schema_diagnostic_text_test.go` | Schema diagnostics quote schema text through `internal/diagnostictext`: an allowlist lint over `internal/schema` message operands (known gaps are listed in its doc comment); the public hostile-value table in `schema_diagnostic_bound_test.go` is the contract proof |
| `tests/schema_export_surface_test.go` | The exported top-level identifiers of `internal/schema`, `internal/value` and `internal/validate` equal an explicit allowlist of the surface consumed across packages |
| `tests/resource_limits_doc_test.go` | `docs/RESOURCE_LIMITS.md` option tables list every public option with its code-owned default |
| `tests/root_public_shape_test.go` and `tests/external_api_smoke_test.go` | External callers use only the supported public packages and interface |
| `tests/stream_boundary_test.go` | Structural borrowed-type restrictions and intraprocedural retention checks, including raw byte fields and aliases erased to interfaces |
| `tests/schema_build_boundary_test.go` | Compiler topology mutation has one owner |
| `cmd/wasmxsd/api_test.go`, `cmd/wasmxsd/main_js_test.go`, `cmd/wasmxsd/build_test.go`, and `cmd/xsdweb/main_test.go` | Go adapter response/build contracts, input bounds, asset catalog, and server shutdown lifecycle |
| `web/js/validation-worker.test.js` | Worker queue, cancellation, timeout, generation, and state ownership |
| `web/js/xsd-runtime.test.js` | Go runtime readiness and single-fatal reporting on runtime exit or startup failure |
| `web/js/browser/validator.spec.js` | Built WASM behavior, response branches, stale-input protection, and main-thread bounds |

The files own their exact test inventory. This contract records why each
enforcement family exists; it does not cache function names.

The stream retention guard tracks borrowed provenance intraprocedurally.
Synchronous helper calls remain reviewed non-retaining boundaries: the guard
does not prove arbitrary helper-return ownership or detect every use after
reader advance, reset, or detach. Those lifetime obligations remain with each
stream consumer.

Root tests and benchmarks MUST use `package xsd_test`. Root tests MUST NOT
import `internal` packages. Implementation fuzz tests MUST live with the package
that owns the implementation being fuzzed.

Fuzz targets live with their implementation owner: stream parsing in
`internal/xmlstream`, schema parsing in `internal/schema`, regex syntax in
`internal/xsdregex`, and document validation in `internal/validate`. The
`Makefile` owns the executable smoke inventory.

A benchmark must be correct at any `b.N`, including one or two iterations split
across workers: the benchmark smoke target runs every benchmark briefly, so an
oracle that fails, or that breaks on a reused second document, fails CI.

## Build Targets

The `Makefile` is the executable source of target names and commands. Its target
graph preserves these ownership rules:

- Library, race, lint, static-analysis, fuzz, and benchmark gates address the
  owning Go packages directly.
- WASM build/test targets own the Go/JavaScript bridge and matching Go runtime
  support file.
- Deterministic worker tests remain separate from the browser-level built-WASM
  integration target.
- The web target builds assets before starting the bounded loopback server.
- The CLI target builds `cmd/xmllint` directly; the Go build cache, not copied
  source prerequisites, decides when internal changes require rebuilding.
- The benchmark smoke target is separate from the timed exhaustive benchmark
  target.
- The oracle image target builds the pinned cross-validator container. It
  needs Docker and network and stays outside the verification contract.

`.github/workflows/test.yml` owns CI ordering and grouping.

## Rejected Alternatives

- A hidden 16 Mi match-work default in `internal/xsdregex`, plus a state cap
  that also limited linear input length, was rejected: it rejected valid values
  below the documented text limits (a 100,001-rune value failed `[a-z]+`) and
  its diagnostic named an option callers could not raise.
- A documentation-only aggregate bound was rejected: it left the stated
  document bound false, since CPU grew as the number of values times about one
  second per value.
- Redefining `MaxInstancePatternWork` and `MaxSchemaPatternWork` as aggregates
  was rejected: one value needs a roughly one-second ceiling while large valid
  documents need far more total work, so one number cannot serve both.
- Uncharged re-evaluation of already-counted literals
  (`Builder.ValidateUncharged`) and publication replay of value constraints
  were rejected. The once-per-literal rule was an unenforced caller
  convention, and the replay multiplied a shared fixed/default literal by the
  number of attribute use sets. Consequence: publication no longer re-proves
  that a cached `Lexical` matches its cached `Value` (including IDs, IDRefs
  and `IdentityKey`), because the compiler-produced build is trusted within
  one compilation.
- An exported account pointer on `BuilderOptions` was rejected: `value.Builder`
  owns the compilation account so a caller cannot share or snapshot it; the
  option is a scalar.
- Charging pattern matching to `MaxInstanceValueWork` was rejected: that
  default is sized for type-graph traversal (text bytes times the type graph)
  and gives no time bound for matching, whose cost grows as runes times
  states. It would also change the documented meaning of an existing option.
- Charging pattern memory through the value-storage bound was rejected: that
  bound defaults to about 1.5 GiB, so a 925 MB pattern set passes, and its
  failure names no pattern option.
- Metering compiled patterns by state count alone, or by an unweighted
  state-plus-range count, was rejected: closures and range arrays escape a
  states-only measure, and an unweighted count rejects valid Unicode-category
  schemas whose ranges retain few bytes each. The unit is estimated retained
  bytes.
- Tunable `xsdregex` compile options were rejected: no production caller set
  them, and they contradicted fixed per-pattern bounds. Aggregate admission is
  the only tunable.

### Diagnostics

- Formatting every rejected value candidate, or replaying validation to explain
  a final error, was rejected: rejected union members occur on successful paths,
  and replay could repeat resolver effects and replenish work.
- Retaining identity tuple spellings or enlarging hot identity program reads
  for messages was rejected because it adds successful-validation cost; cold
  schema name projections, retained target IDs and field positions supply the
  context instead.
- Reimplementing content matching in the diagnostic layer was rejected because
  it would duplicate occurrence, wildcard and substitution rules.

- Bounding segments inside an incremental path-text cache was rejected: escaped
  lengths need a per-element name scan on the element-start hot path, and every
  path consumer is diagnostic-only. A path-only `\uNNNN` escape dialect was
  rejected because one diagnostic would show a name in two renderings.
- One location target for every diagnostic site was rejected: deferred
  identity diagnostics and parent-content diagnostics have a different,
  legitimate target than the element being started.

### Sources And Schema Graph

- Skipping the reopen for a repeated opener identity was rejected: the reopen
  is the only detection of an opener or resolver serving different content
  under one identity. In-memory sources (digest known) and built-in File
  sources (same name; a reread only detects an on-disk change, which the first
  read is not protected against) skip it. Rereading File sources per edge was
  rejected: it made `MaxSchemaTotalBytes` scale with edges times size.
- Deduplicating schema documents by content fingerprint, and ignoring later
  documents for an already-loaded namespace (the XSD 1.0 note on `<import>`
  hints), were rejected (I-019): both hide real sch-props-correct.2 conflicts
  and would make the verdict depend on undefined load order.
- Implicit XML/XLink attribute declarations, including fallbacks used only when
  no source is supplied, were rejected because explicit sources own the schema
  component set; a fallback schema would need its own version and product
  contract.
- Restricting import-free XSD/XSI access to a builtin type catalogue was
  rejected after adopting WG correction 1890, which exempts both namespaces;
  namespace authorization still supplies no components and bypasses no kind,
  existence or duplicate checks.
- Keeping source buffers for exact byte equality was rejected: it retains
  complete inputs, and reopening cannot recover prior bytes. Raw byte length
  plus SHA-256 identifies content.
- Returning on the first parse failure was rejected for streamed acquisition
  because it hides later read, close, and limit failures that take precedence;
  `Input.Finish` drains and cleans up. `Input.Discard` closes without
  draining only where a limit has already ended the load and neither content
  nor read failures matter (the document over `MaxSchemaSources`).
- Interleaving resolution, graph validation, and component compilation was
  rejected because partially known namespaces made ownership and failure state
  ambiguous. The loaded-graph-to-plan boundary is the only route into indexing.
- Traversing canonical source references per effective context was rejected:
  it exceeded the 3% compile CPU budget, so the planner keeps a transient
  filtered edge projection.
- A retained source vocabulary tag and copied declaration index were rejected
  because they duplicate admitted names and root-child order; allowlisting
  child names would discard unknown XSD grammar before its diagnostic owner.
  A semantic source
  struct with one nullable pointer per capability was rejected because it
  permits contradictory records; one closed variant owns at most one capability
  record.

### Schema Compilation And Content Models

- Charging group flattening to `MaxContentModelAnalysisSteps` was rejected: its
  16.7M-step default admits over 1 GB of particles before failure. Charging
  `len(Particles)` at `addModel` was rejected because the main path installs
  particles through `completeModel`, and shared variants would be charged for
  copies that do not happen. Cloning content models on read was rejected: it
  is uncharged work proportional to references times group size.
- Skipping all validation below a zero-count compositor was rejected because
  the mapping removes only that compositor's component (see
  `compileDeferredModelChildren`).
- Deferring complex types only when their immediate base is active was
  rejected: intermediate bases still produce false cycles and renaming types
  changes acceptance (see `complexTypeCompletion`).
- Deriving maximum-one execution overrides from a repeated base choice was
  rejected: RecurseAsIfGroup adds only a comparison wrapper, and execution
  projects the admitted particle unchanged.
- Copying the complete model table per restriction was rejected because
  unrelated components amplify comparison cost; a private append-only view is
  used instead. Applying language-equivalent occurrence multiplication during
  semantic admission was rejected because it changes the XSD 1.0 structural
  rule.
- Rebuilding unchanged groups to relocate occurrence bounds, and cloning child
  slices inside trusted analysis, were rejected: comparison already composes
  both layers, and wide groups amplify copying cost.
- Discarding generated-choice provenance during flattening, or using global
  unordered flags with permutation search, was rejected: either reintroduces
  storage-order dependence or waives explicit sibling order (I-005).
- Refunding work after failed publication validation was rejected because the
  computation has already occurred; repeated failures would evade the aggregate
  work bound. Retryability preserves build data, not spent resources.
- Sharing UPA results across roots and eager DFA follow interning were
  rejected because each changes work accounting or limit-failure order.
- Publishing unentered content models through a sparse table was rejected (a
  zero `CompiledModel` reads as Empty and a remap adds a lookup per read);
  skipping their compilation was rejected because their errors and work
  charges belong to admission.
- Raising the DFA state cap to admit avoidable expansion was rejected; safe
  single-particle occurrence normalization prevents that expansion.
- Per-row pointers for optional content dispatch were rejected because they
  enlarge every published row. Deferring all map cloning was rejected to keep
  the original projection lifecycle and small-schema construction path.
- Inferring empty complex content from an empty normalized child model was
  rejected because element-only particles can accept no children while still
  permitting whitespace.

### Values

- Canonical-text comparison for typed fixed values was rejected: equivalent
  duration and timezone-normalized g* values have different lexical
  projections, and canonical spelling can select a different union member.
  A second equality API, lexical shortcut or identity-key cache would
  duplicate value-owned policy.
- Combining Gregorian primitive parsers was rejected because their field
  grammars and failure precedence differ.
- Re-resolving schema-supplied QNames in instance namespaces was rejected
  because missing or rebound prefixes change the constraint's value; a bounded
  application-prefix projection replaces retaining the compiler's namespace
  history. Capturing only names resolved by the declared type is insufficient
  when a string-first union or anyType later selects QName.
- Adding normalized spelling to every parsed value was rejected because it
  enlarges every retained list item; evaluation returns it instead.
- Lazy or capped list growth was rejected because storage still scales with
  input, and rejecting a list that exceeds an enclosing enumeration's item
  bound was rejected because it can change union member selection.
- Unindexed or string-only enumeration membership was rejected because it made
  compilation quadratic in repeated literals; charging membership to value
  work was rejected because it slowed raw validation, and capping literals
  would reject valid schemas.

### Regular Expressions

- Flattening repeated character-class ranges before merging was rejected
  because temporary storage would scale with repeated inputs; a global
  named-category cache was rejected because parser-local caching already has
  the needed bounded lifetime.

### XML Stream And Namespaces

- Checking only named token types or immediate selectors for borrowed-data
  retention was rejected because borrowed fields and aliases lose that type;
  intraprocedural provenance closes the gap without SSA.
- Caching complete lexical QNames instead of prefix/local components was
  rejected because independently cacheable components can form whole names
  exceeding the length or entry limit, introducing repeated allocations.
- A validator cache of resolved schema names and larger spelling caches are
  deferred: no qualified CPU result justifies their retained strings, lookup
  probes, or invalidation policy.
- Shrinking the `recentCacheEntries` ring was rejected: a ring that inserts on
  a miss gets no hits on cyclic vocabularies wider than itself.
- Adding document depth to the tokenizer was rejected: the `Reader` already
  owns topology, and lexical text origin supplies the outside-root fact.
- Linear live namespace lookup was rejected because deep binding chains
  amplify churn; a second authoritative map was rejected because retained
  contexts need immutable history, so the active-prefix map is only a
  projection.
- Parser-mode-specific line counters were rejected because CR/CRLF positions
  would diverge; the byte stream owns positions and token modes own payload
  normalization.
- Rejecting relative namespace names, and keeping lexical `xmlns` sentinels
  after expansion, were rejected; expanded declarations use the reserved XMLNS
  URI, which ordinary bindings cannot claim.

### Validation, Sessions And Identity

- A global or unbounded session pool was rejected: schema ownership and
  retained memory would become implicit. One idle slot per engine reuses the
  sequential path without blocking concurrent calls.
- Containing a failed-selection subtree was rejected: it hid descendant facet,
  identity, and IDREF diagnostics that Xerces reports, and needed a third
  element mode. Lax assessment reuses the anyType and lax-wildcard paths.
- Scanning every scope index on close, closure callbacks, and a second
  registration journal were rejected. The evaluator unwinds its own scope
  declarations directly.
- A stored self flag on identity paths and a schema-wide list of self selector
  branches were rejected. The flag gave self two encodings, and the list joined
  every self constraint at every start, although a self branch matches only
  where its scope opened.
- Scanning every active selection's field paths instead of terminal-filtered
  dispatch was rejected after unrelated field names regressed. A maintained
  selection index would add membership updates to rollback.
- Per-constraint attribute-field depth windows were rejected because they need
  a second, constraint-ordered membership index. Publishing raw step bounds was
  rejected because validation would duplicate the matcher's depth rule.
- A schema-wide attribute-name prefilter (a local-name bitset plus wildcard
  flag checked before the base-depth window) was not adopted: in serialized
  n=10 runs it cut 5.5% from rows with one `@id` field among 16 attributes but
  cost 1.7% when an `@*` field disabled it and 0.7% on
  `IdentityCPUDisparateFieldNames`; a name map or sorted slice would only add
  lookup cost, and per-name windows need a second name-keyed index. Reopen for
  a workload where unmatched attribute names dominate identity cost.
- Replacing published selector-path records with a scalar registration
  invariant is deferred: allocation-byte comparisons did not establish
  nonexpansion, and smaller records with matching allocation counts do not
  prove it.
- Clearing every match word per value was rejected because sparse matches then
  cost work proportional to all field slots.
- Propagating conflict markers to the nearest declared ancestor was rejected,
  because every intervening element completes its own inherited tables. Eager
  empty frames on every element were rejected in favour of sparse inherited
  frames.
- Alternative field states for undeclared lax attributes are rejected in
  [I-022](docs/spec/INTERPRETATIONS.md#i-022--undeclared-lax-attributes-are-unknown-field-values).
- A retained set of hint namespaces with `MaxSchemaLocationNamespaces` and
  `MaxSchemaLocationNamespaceBytes` was removed: hint names never take part in
  selection, so the set's only subject was its own accounting, and it made a
  schema-valid document with many hint namespaces fatal. `MaxInstanceTokenBytes`
  already bounds every hint attribute.
- An even item count for `xsi:schemaLocation` was rejected: the declared type
  is a list of anyURI, Xerces-J, .NET, xmlschema and libxml2 accept odd counts,
  and the library never uses hints.
- Checking hints before selection was rejected: it checked skip-processed
  content and needed a guessed path mode.
- Staging the path mode on the start transaction was rejected: assessment can
  replace the start, so the mode is passed at the failure site.
- Per-value heap staging and post-hoc limit checks for ID/IDREF batches were
  rejected: they created garbage per value and could exceed the identity budget
  before failing.
- A separate `MaxIdentityFieldSlots` option was rejected as an extra public
  tuning dimension. `MaxIdentityEntries` owns identity cardinality through
  three independent ceilings.
- Eager path strings, per-element path nodes, chunked encodings, and global
  interners were rejected because they copied shared prefixes or namespaces per
  fact, or added rollback-sensitive document-wide state. An identity-owned path
  arena or separate path limit would add a competing writer or public policy
  dimension.

### Formatting

- A one-pass pretty printer was rejected because later mixed content changes
  earlier indentation and malformed input must produce no output; repeated full
  tokenization was rejected on measured cost.
- Unlimited zero-value limits were rejected because formatting retains spans,
  decisions, and frames; zero selects finite defaults.
- Trusting a writer's nil error after an incomplete write was rejected because
  formatter success must imply complete XML output; the bounded writer checks
  every delegated write in one place.

### Public API, Browser And Tooling

- Command import edges are an explicit allowlist enforced by `TestCmdImportEdges`
  in `tests/phase_import_graph_test.go` (host and `GOOS=js GOARCH=wasm`):
  `cmd/wasmxsd` imports the root package, `xsderrors`, `internal/format`, and
  `internal/validate`; `cmd/xmllint` imports the root package and `xsderrors`;
  `cmd/xsdweb` imports no repository package. A new command fails the test until
  listed. `cmd/wasmxsd` calls `validate.CheckXMLWellFormed` directly to give
  schema-free XML diagnostics when compilation fails, with options converted from
  its single `xsd.ValidateOptions` value. A public root `CheckWellFormed` was
  rejected because it would add public API for one adapter.
- Path-based asset checks followed by unrestricted serving were rejected because
  symlinked ancestors and replacements could escape the configured directory.
  One standard-library root handle contains both operations.
- Context-bearing compilation and validation APIs were rejected because the
  library cannot interrupt arbitrary resolvers, openers, files, or readers;
  they would add parallel APIs without providing cancellation.
- A universal low cognitive-complexity limit was rejected because it fragments
  cohesive parsers, state machines, and invariant audits; the configured
  `gocognit` limit flags exceptional review targets only.
- Main-thread WASM execution was rejected because synchronous validation blocks
  rendering and cannot be interrupted. Worker termination is the cancellation
  boundary.
- Boolean-only WASM results and object-valued window globals were rejected. A
  tagged response distinguishes invalid documents from operational failures,
  while worker-owned limits keep the browser boundary explicit and isolated.

## Documentation Ownership

- `AGENTS.md` is the low-context execution router. It points to this contract
  and holds the project contract, repository-specific agent workflow, worktree
  safety, and the verification contract. It does not copy the package graph;
  design rules belong to **Change Protocol** and test-practice rules to
  **Tests And Enforcement**.
- This file owns cross-module boundaries, ownership, contracts, flows, and
  rejected alternatives. Package-local mechanisms, data layouts, constants,
  bounds, and invariants live in Go doc comments at their owning type,
  function, constant, or package `doc.go`; this file names those owners rather
  than restating them. Comments explain non-obvious intent, constraints, and
  consequences, not the code itself.
- `README.md` documents public usage, command workflows, and benchmark results.
- Default limit values are owned by the constants in
  `internal/validate/options.go` and `internal/schema/options.go`.
  `docs/RESOURCE_LIMITS.md` owns the public option reference (zero-value rules,
  meaning, limit interaction); `tests/resource_limits_doc_test.go` checks its
  tables against those constants through the public zero-value options.
- `docs/BENCHMARKS.md` owns benchmark methodology, reproduction commands, and
  comparison workloads.
- `docs` holds only documentation: `docs/BENCHMARKS.md`,
  `docs/RESOURCE_LIMITS.md`, and `docs/spec`. Review reports, comparison
  reports, completed plans, benchmark evidence, and verification logs do not
  belong there.
- `docs/spec` records XSD 1.0 source authority (editions, errata, and selected
  corrections), the corrected E1-56 reading, interpretation decisions, and
  attribution; it bundles no specification copies and does not define corpus
  coverage.
- `web` holds the browser validator: `web/index.html`, `web/js`, and the built
  `web/xsd.wasm` and `web/wasm_exec.js`. The Pages workflow publishes `web`,
  and `cmd/xsdweb` serves it by default.
- `tools/oracles` owns the cross-validator image: provider pins, offline
  adapters, and the `run.sh` interface. `tests/README.md` owns when and how its
  results enter diagnosis.
- `tests/README.md` documents corpus and harness operation. Counts and other
  cheap derived facts come from the manifest or test commands rather than
  manually maintained prose.
- This file is the sole architecture source of truth. Other documentation may
  link here but must not restate a competing package graph or lifecycle model.

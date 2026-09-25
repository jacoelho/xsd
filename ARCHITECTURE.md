# Architecture

This is the canonical architecture contract for contributors. It describes the
current target state, not migration history or alternative designs.

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
other: compilation never depends on validation, and validation never mutates or
reconstructs schema semantics.

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
| `internal/validate` | Document assessment, recovery, identity state, and reusable sessions |
| `internal/format` | Repository-internal formatting and its input/output limits |
| `internal/lex`, `internal/vocab`, `internal/uriref`, `xsderrors` | Shared lexical facts, vocabulary, URI-reference semantics, and structured errors |
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
| `ValidateOptions` | Errors, identity state, hints, depth, attributes, text, tokens, input bytes, and per-value work | Schema mutation or dynamic schema loading |
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
2. This file owns internal architecture, including module ownership, data flow,
   state transitions, lifecycle, resource policy, and rejected alternatives.
3. Code and tests prove the current implementation. A conflict with this file
   is architecture drift to resolve, not a second design.
4. `README.md` owns public usage. `docs/spec` is searchable local standards
   reference. `tests/README.md` owns corpus and test-harness operation.
5. Plans describe proposed work. Ledgers record revision-scoped evidence and
   decisions. Neither may silently redefine current architecture.

For a change, start at the public or package interface named in the routing
table below, trace the single execution path to its state owner, then inspect
all readers, writers, failure exits, limits, tests, and relevant history. Read
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
| WASM, worker, page, or local-server lifecycle | `cmd/wasmxsd`, `docs/js`, `cmd/xsdweb` | The narrowest listed adapter | Tagged response tests, worker generation/timeout tests, browser integration |
| A finite limit or retained allocation | Declaring option/constant, then allocating symbol | The module that first admits or retains the work | Below/at/above-limit tests, rollback/reset, opposing-shape allocation and retention evidence |

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
  and source identity. `Source.OpenInput` is the only source-opening path; its
  `Input` owns bounded reads and `Finish` drains unread bytes, closes once, and
  returns raw byte count, SHA-256 fingerprint, and stage-aware failures.
  `Source.ResolveFrom` is the only reference-resolution path. The package does
  not own XSD vocabulary or schema-graph policy. The compiler has one canonical
  source-loading path and may delegate within that path, but it must not replace,
  bypass, or duplicate either source boundary capability.
  Removing a custom resolver preserves the built-in file resolver's canonical
  context identity. The public adapter normalizes its known nil resolver forms
  before they enter source resolution.
  Per-source limit diagnostics acquire their source path before joining read or
  close failures, so wrapper-preserving presentation cannot lose that location.
- `internal/uriref` owns XSD 1.0 URI-reference validity after XLink escaping,
  raw and escaped projections, fragment syntax, and raw-preserving RFC 2396
  composition. Arbitrary source names and Unix paths do not enter this type.
- `internal/schema` owns schema syntax, source-aware diagnostics, opaque
  annotation consumption, graph loading and planning, chameleon contexts,
  component dependencies, names and declarations, complex derivation, identity
  declarations, content models, substitution groups, and immutable publication.
  Production schema input is read through `internal/xmlstream`; the parser
  retains compact `schemaNode` records with one closed typed semantic-source
  variant when a node owns compiler capability, one node-owned ID and
  `xml:base` projection, document-local IDs, and source positions. Syntax-only
  grammar nodes may carry only those common facts. Each variant owns its complete capability facts;
  particle data is embedded in the element, model, group, or wildcard variant
  that admits it. Simple-type declarations retain name and final constraints;
  their base, list-item, union-member, and complex-content facts belong to
  distinct child variants. Its syntax admission facts are transient and are never used
  as a generic XML tree by compilation. Schema
  QName references resolve while their namespace frame is live; literal values
  and XPath expressions that are intentionally deferred retain only the bounded
  namespace context needed by their owning compiler. Annotation payload is
  consumed without entering the retained semantic graph.
  The loader produces a closed graph before component compilation begins.
  Source loading, effective-context planning, and component resolution share a
  finite dependency-work budget; active expansion has a depth cap of 1024.
  Content analysis has a separate finite work budget shared by consistency,
  restriction, ambiguity checks, compilation, and sealing.
  The compiler owns one private `schemaBuild`. Registration and completion
  update correlated tables together. There is no exported mutable builder or
  forwarding layer between compiler and build state. Publication validates
  semantic source invariants, constructs immutable execution tables once, and
  consumes the build only on success. Failure preserves build data, while
  completed work remains charged. Read tables are derived state; comparing each
  table back to a duplicate runtime projection is not a second publication step.
  Names, typed references, element constraints, derivation indexes, wildcard
  policies, substitution membership, and content execution remain schema-owned.
  Published value constraints own application text and an immutable namespace
  projection for revalidation under a different actual type. Context-free
  constraints use canonical spelling; accepted QName/NOTATION values, including
  list items, retain source spelling because expanded-name text is not lexical
  XML. Rejected union members do not determine that choice. The projection
  captures each distinct prefix in QName-shaped application fields, including
  the default namespace and unresolved prefixes even when the declared type
  selected a string member. It retains only sorted bindings and shared immutable
  strings, never a source node, namespace history, or compiler closure. Source
  token/byte and instantiated-node limits bound this metadata. Publication
  validates coverage and consistency with captured resolution proofs; cloning
  owns the binding slice. Repeatable application lookup is distinct from the
  consuming proof replay used to audit original literal admission.
  Every constructed name table reserves namespace ID zero for the empty
  namespace independently of seed order; publication validates that invariant.
  Identity declarations compile into immutable selector/field path programs and
  exact-name, namespace, and wildcard dispatch indexes; validation owns only
  active document scopes and matching scratch.
  Large `xs:all` models use a bounded QName-to-term index; small models scan
  directly. Both preserve occurrence bits, substitution matching, and atomic
  transition failure. The index is derived once during sealing.
  One `ElementFrame` read combines effective-type simple content, text policy,
  and initial child content state. The original element declaration remains the
  owner of default/fixed constraints after `xsi:type` changes the actual type.
  Validation carries selected declaration metadata through start assessment
  instead of resolving the same declaration again. `ContentTransition.Commit`
  owns the stale-state check and applies the content transition once, after
  identity commit readiness has been checked.
- `internal/value` owns simple-type validation and its immutable program:
  lexical normalization, primitive values, lists, unions, facets, canonical
  text, typed equality, and ID/IDREF projections. Compile-time literals and
  instance values use the same value semantics. Construction capacity is admitted
  against the same conservative storage budget as type records; reserved slots
  consume that admission without a second fixed charge. Queued `Add` records
  prepay their complete metadata estimate; sealing cannot charge it again.
  Explicit `Reserve`/`Complete` admits the remaining metadata at completion.
  One program-owned completion table serves construction and evaluation until
  sealing removes it. Type construction validates dependencies and facets before
  publication. Completion derives one raw evaluation plan from the effective
  lexical rule and facets, including string enumeration, NMTOKEN lists, and
  integer sign/magnitude bounds. Builder literals can use completed plans before
  sealing. Integer eligibility follows the integer lexical rule; decimal types
  restricted to zero fractional digits still accept decimal spellings.
  Caller-owned scratch bounds
  reusable validation storage. Completed type dependencies are acyclic; runtime
  evaluation tracks only depth and cumulative lexical work; empty lists never
  visit an item type. The schema compiler
  supplies the builder's construction limit; each completed type's bounds and
  enumeration share one fresh facet-batch budget. Builder validation starts a
  fresh construction budget per value. Instance validation supplies its own
  normalized `MaxInstanceValueWork`, whose fixed default is 4,194,502,132,335
  work units and does not depend on compilation options. Each evaluation visit
  charges its lexical byte length plus one. Raw-byte attempts, shortcuts,
  list items, union attempts, and typed fallback share one per-value counter;
  a failed fast attempt does not replenish the budget. The published program
  retains neither caller limits nor work counters; session limits remain fixed
  for each validation and scratch retains no cumulative work. Limit failures keep their
  schema or instance diagnostic category at the owning boundary.
  Borrowed-byte and admitted-string validation consume UTF-8 XML 1.0 character
  data already checked by the stream boundary, including through typed fallback.
  Ordinary string validation admits external lexical input. All paths share
  value semantics and work accounting. A type retains its owning type
  through list and union evaluation. Equality uses the admitted value space,
  including duration
  month/second coordinates and resolved QName names. Text projections do not
  define equality: duration has no XSD 1.0 canonical representation and retains
  its whitespace-normalized lexical spelling. Patterns in one restriction step
  are alternatives; inherited restriction steps all apply. Union patterns inspect
  the selected member's normalized lexical output after ordered member assessment;
  evaluation returns that spelling explicitly to its caller, without mutable
  output pointers in options or enlarging each retained list item. A failed
  union pattern does not try a later member. Union enumeration compares the
  selected member's value. Compiled enumeration groups retain their maximum
  list-literal item count. Evaluation propagates the maximum applicable count
  through nested unions and combines it with each list's own groups. An input
  with N items retains all items only when N is within that bound K; otherwise
  it retains none. Temporary item storage is therefore at most K parsed values.
  Every item still validates and consumes work; omitting storage cannot change
  first-success member selection or bypass local facets. List equality requires
  equal counts and complete retained items. Enumeration-literal construction
  retains all items under its construction limits. Literal construction can omit
  its containing type's facets while they are being installed; member and item
  types always enforce their own facets, preserving normal value selection.
  Fixed ordered facets use the nearest declaration of the same bound kind;
  the complete inherited bound sequence remains available for partial orders.
  Schema declarations retain only component metadata and value-program
  references; they do not
  implement a second simple-value parser, facet evaluator, or equality model.
  Document identity is carried by each validated value: a union preserves the
  selected member's ID/IDREF projections, and a list collects its selected items'
  IDREFs. Static type identity metadata cannot replace these dynamic projections.
  Validation records them even when no key/unique/keyref field requested a value.
  The g* identity projection uses the normalized instant and timezone presence,
  matching typed equality independently of lexical calendar fields. QName
  resolution produces one expanded name before NOTATION declaration checking
  and typed assignment; absent namespace context cannot bypass that check.
  The value-owned `ExpandedName` is both the `QNameResolver` result and the
  retained QName/NOTATION payload. Compilation, proof replay, and document
  assessment share that resolver contract; value admission validates its result.
  Validation reads one scalar `InputRequirements` projection for QName context,
  document identity, and safe string interning. It does not clone diagnostic
  type views or interpret facet metadata.
- `internal/xsdregex` owns XSD 1.0 whole-input pattern semantics. One parsed
  expression selects literal, linear, or NFA execution based on its structure.
  Compilation and matching have explicit work/state limits. XML input admission
  belongs to `internal/xmlstream`; match callers provide valid UTF-8 XML text.
  Character classes merge sorted inputs through a heap of current ranges:
  total input ranges R across K nonempty sets require O(R log K) work and
  O(K + U) temporary storage for U output ranges, without retaining every
  repeated input range. Unit-stride Unicode rows remain contiguous ranges.
  A parser-local cache reuses admitted named categories and their complements;
  its keys are bounded by both polarities of the closed category/block catalogs,
  and it is released after compilation. No process-wide cache retains patterns.
  The recognized block catalog excludes the XSD 1.0 surrogate block names,
  including complemented escapes. Corpus expectations follow that contract;
  upstream disagreements remain provenance in the authoritative test manifest.
- `internal/validate` owns instance validation: finite default limits, option
  normalization, XML reader preflight, parser error classification, validation
  recovery, document structure, start/end element decisions, attributes,
  content, simple-content assessment, the concrete document-local identity
  evaluator and its lifecycle, XSI handling, and schemaLocation hint handling.
  XSI identity conversion propagates value-work limit diagnostics after
  invalidating its fields and releasing the pending target; ordinary lexical
  conversion errors remain owned by start assessment. Each URI item in an XSI
  location hint is a separate typed evaluation under the same per-value limit;
  existing token and hint limits bound aggregate processing.
  The document runner detaches its XML reader on every exit. Reusable sessions
  clear remaining document state before releasing the overlap guard; that
  cleanup does not repeat reader detachment. `SessionPool` owns a sealed schema
  and one atomic idle slot of raw validation scratch. It normalizes options
  before checking schema availability, and reapplies all limits on each checkout.
  A cache miss allocates isolated scratch without waiting. Normal returns reset
  state before attempting publication into the idle slot; a full slot drops the
  returning scratch. Panics skip publication after reader detachment. Existing
  buffer, map, and slice retention bounds apply to the single cached session.
  Explicit sessions are separately allocated guarded owners.
  Retained document element frames keep payload, expanded name, handle, and
  path metadata; lexical prefixes remain owned by the XML stream while start
  admission is live and are not copied into document state.
  All borrowed simple-value inputs share one admission path. QName/NOTATION
  spellings use the reader's bounded string cache, while resolution runs against
  the current namespace frame each time. Resolver callbacks belong to the
  reusable session; cached spellings never cache resolved QName values.
  Empty simple-content elements may instead obtain their value from a schema
  constraint. The same-owner path reuses the prevalidated value. A changed actual
  type validates the constraint's application text against its facets and the
  instance value-work limit, using the constraint's schema namespace projection
  and the sealed schema's NOTATION declarations. This supplied value records
  document identity without a second fixed-value comparison. Instance text,
  including whitespace-only text, uses instance namespaces and retains typed
  fixed-value equality. Untyped mixed-content constraints retain lexical equality.
  Both origins share identity record/capture/commit and failure rejection; nil,
  content recovery, and session cleanup keep their existing owners.
- `internal/format` owns repository-internal XML formatting and finite default
  input, token, processed-node, depth, and output bounds. Its output boundary
  rejects every incomplete `io.Writer` write, so success means the complete
  formatted document was written. It consumes the shared stream and namespace
  boundaries and exposes no root-package API.
- `internal/xmlstream` owns XML token streaming and declaration scanning shared by
  schema parsing, instance validation, and formatting. Its parser owns prolog
  preflight, the sole input buffer, XML 1.0 line-ending normalization and byte
  positions, and reader detachment for each stream. `Reader.Next` returns one
  reader-owned, read-only token pointer. Advancing, `Reset`, and `Detach`
  invalidate it; a rejected advance while admission is pending preserves it for
  retry. Start and end admission consume that current token without accepting
  caller-supplied copies. Literal CR and CRLF each
  advance one logical line in every parser mode; emitted payloads contain LF.
  Character-data tokens retain one stream-owned lexical origin: literal text,
  text containing references, or CDATA. Coalescing never erases reference origin.
  Contiguous ASCII text can borrow the input buffer until the next advance;
  references, normalization, Unicode, and refill use the existing token buffer.
  Both paths charge token limits before publishing data. Byte lookahead leaves
  positions unchanged, and bulk LF consumption preserves CRLF normalization.
  Compile, validate, and format use that origin for admitted character content;
  `Reader.Next` owns rejection of unsupported declarations and forbidden
  outside-root data, while consumers translate those neutral boundary failures.
  The tokenizer does not duplicate document topology.
  Only EOF at a token boundary completes a stream; EOF after consumed markup is
  an XML syntax error, while simultaneous non-EOF reader causes remain observable.
  Source offsets count admitted bytes; an overflow probe belongs to the terminal
  input-limit result and cannot shift spans of the admitted prefix. Terminal
  input errors remain latched so later advances cannot read beyond that result.
  The same XML stream owner admits namespaces and detects duplicate expanded
  attributes. Its append-only binding chain owns retained immutable contexts;
  an active-prefix index and scalar default-namespace head are reproducible
  frame-local projections. Retained contexts capture the default head alongside
  the binding-chain head, so later shadowing cannot change their resolution. Retained
  namespace frames keep only the lexical closing name and a nonzero serial;
  handle store ownership remains validated against the owning stack before the
  serial is checked. Admission, rollback, end, and reset update these together.
  Oversized maps and buffers are dropped at reset and detach; bounded caches may
  remain for session reuse. Each name/value cache admits at most 512 owned spellings of at
  most 256 bytes. A string map owns those spellings, and an eight-entry recent
  ring projects its values. Map keys and values share the same string storage;
  borrowed-byte lookups return that owned value without copying. Spellings
  beyond either bound are returned as owned strings without entering the cache.
  Comment mode
  selects syntax-only discard for instances, bounded discard for schemas, or
  emission for formatting. Bounded discard charges normalized payload bytes
  through the same token-limit owner without retaining or dispatching comments;
  schema comment limits remain unchanged.
- `internal/lex` owns low-level XML lexical helpers used by source and
  `internal/xmlstream` code.
- `internal/vocab` owns XML/XSD namespace and vocabulary constants.

Internal packages MUST NOT import root `xsd`. Compile-time packages MUST NOT
depend on validation packages. Value and lexical packages MUST remain below schema and validation packages.

## Data Flow

Compilation flow:

1. Public callers provide `xsd.SchemaSource` values.
2. Root `xsd` converts them to immutable or repeatable `internal/source.Source`
   values.
3. `internal/schema` applies document-local XSD admission before resolving a
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
   The canonical identity map owns one parsed document and a raw-content
   fingerprint (byte count plus SHA-256) per source key; resolution-context
   aliases remain attached to that entry. Opened source bytes are streamed into
   parsing and are never retained as a complete buffer. Explicit `Bytes` sources
   still own their caller-supplied immutable content. The source is finished
   before a parse result is accepted: unread bytes are drained within the same
   limit and close/read/byte-limit failures take precedence over early XML
   syntax errors, preserving acquisition diagnostics. Repeated identities are
   reopened and charged as before; only their fingerprint is compared. Identity
   relies on SHA-256 collision resistance, not mathematically exact byte
   equality. Streaming admission constructs typed source records; the source
   graph retains semantic declarations and references, not a generic XML tree.
   The loader owns one reader reused sequentially across sources. Each parse
   detaches it before returning; source records own retained names, values, and
   namespace contexts, so resetting the reader cannot change earlier documents.
   Content identification traverses its sorted keys, without a mirrored source
   list. Component and identity-declaration contexts derive from the same immutable
   plan document, without a separate context registry.
   The completed loader state becomes a closed loaded graph: planning may consume
   and update graph-owned state but performs no further I/O. Planning validates
   target namespaces and expands effective chameleon contexts without reopening
   or resolving sources. Annotation payload is consumed as
   namespace-well-formed opaque XML without entering retained source records.
   Source loading, graph planning, and component resolution consume one aggregate
   dependency-work budget. Component references also enter a bounded active
   expansion stack; cache hits remain charged, while cycles retain their specific
   schema diagnostics. Once every effective target namespace is known, the compiler indexes one
   declaration representative for each content fingerprint and effective
   namespace while retaining every source occurrence for resolver traversal and
   graph validation. It compiles schema components and populates a compiler-owned
   mutable `schemaBuild`. After all types and substitution affiliations are
   complete, element value constraints and effective substitution types are
   finalized atomically; the bounded transitive substitution table is the only
   retained substitution lookup.
4. Private schema publication checks source invariants before constructing the
   execution tables. Failure leaves the build retryable within its remaining
   work budget. Success seals and consumes that one build.
5. Root `xsd.Engine` stores a shared `validate.SessionPool` owning that sealed
   validation schema.

Validation flow:

1. Public callers validate through `Engine.Validate`, `ValidateWithOptions`, or
   a reusable `Session`.
2. Root `xsd` adapts public validation options.
3. Root `xsd` delegates construction and validation to `SessionPool.NewSession`
   and `SessionPool.Validate`; the pool owns schema and scratch lifecycle.
   A reusable public session is a handle to one guarded internal owner: copies
   alias that owner, and overlapping calls fail before reading the second input.
   That owner also holds bounded scalar scratch for type derivation and compiled
   string-pattern execution; immutable schema tables never hold validation work
   buffers. Every return path clears document-local state before releasing the
   overlap guard, while returned diagnostics remain caller-owned. The overlap
   guard is acquired before reading input, and cleanup completes before that guard
   is released.
4. `internal/validate` reads immutable schema facts through methods on the
   sealed `*schema.Schema`, then applies validation policy to those facts.
   Identity evaluation reads immutable schema-owned selector/field programs and
   their precomputed dispatch indexes. One concrete evaluator owns the element identity stack, matching
   path, per-element ID state, document IDs and IDREFs, key/unique/keyref scopes,
   pending selections, resource accounting, and reset/discard behavior. Value
   targets carry depth from the authoritative XML document stack. Without
   key/unique/keyref constraints, identity stays dormant until a validated value
   produces an ID or IDREF, including a selected union member or dynamic type.
   Activation extends the same identity element stack to that depth. Dormant
   starts retain only the minimal activation checkpoint; first activation and
   recorded identities roll back on fatal start failure. The XML transaction
   alone rolls back retained diagnostic paths. Document ID/IDREF checks remain
   independent of schema key constraints. Value
   capture uses one borrowed prepared target at a time: callers prepare, record,
   capture, then commit, or reject the target on validation failure. Element-end
   finalization is also evaluator-owned, including recoverable diagnostic
   reporting and the ordering of field completion, scope closure, ancestor
   invalidation, and path/stack release.
   Published key tuples retain their originating scope depth through ancestor
   propagation. A scope's local entry takes precedence over descendant entries;
   conflicting descendant nodes remain ambiguous, and duplicate local entries
   remain errors. Propagation merges smaller tables into larger ones without
   changing that provenance or charging already admitted entries again.
   Element frames distinguish assessed nodes, nodes admitted by a
   `processContents="skip"` wildcard, and validation-recovery containment.
   Identity fields distinguish an absent field from a validated value and a
   selected node that has no valid simple value. Skipped and lax-missing nodes
   therefore remain visible to identity XPath matching without being assigned
   declarations or types, while recovery invalidates affected ancestor fields
   without creating secondary identity diagnostics. A nillable element-field
   marker is enforced only when selection finalization has established a
   complete qualified key sequence. A pending selection retains its selected
   depth, not an eager copy of the document path. Immediate diagnostics project
   that depth from the active document path. IDs, IDREFs, and published tuples
   that need a path after an element pops retain references into document-owned
   parent-linked encoded suffix nodes, and the exact string is materialized only
   when a diagnostic is emitted. A node extends the nearest referenced active
   ancestor. Every newly encoded active element receives a reference to its
   complete segment boundary inside that node, so later descendants and siblings
   share a live prefix even when it ends inside the encoded text. These
   annotations are depth-bounded active state; they disappear when elements pop
   and are never overwritten while live. Each live element is therefore encoded
   and annotated at most once between rollback or pop boundaries, without one
   retained node per path element. Lexical path bytes are stored directly.
   Expanded-name records retain local-name bytes plus a suffix-local compact
   reference into one string header per distinct namespace in that suffix; they
   do not copy namespace URI bytes into every path occurrence. A transient
   namespace index is reused across suffixes, cleared after encoding, and dropped
   with oversized namespace storage at session reset. Retained paths cannot
   escape a validation call: fatal start rollback clears references into new
   nodes and truncates nodes and namespace headers, semantic discard releases
   them, and session reset clears references and drops oversized capacity. At
   element end, selections owned by that element's
   identity scope receive current nillable-field markers and finish before the
   scope closes. Scope-local failure is then folded into the element assessment
   and invalidates still-pending ancestor-owned fields before those selections
   finish. Root and child selection return one validation-owned `schemaStart`.
   Root selection first uses a global declaration, then a present `xsi:type`,
   then a matching schema hint, otherwise recoverable missing-root handling.
   Hint syntax is diagnosed before selection; recovery may continue when the
   error budget permits. An undeclared root resolves its type during selection;
   an operation-local type origin lets common assessment consume that same TypeID
   without resolving it again. No origin or second cached type reaches a frame.
   Ordinary declared starts extract `xsi:nil` and `xsi:type` before assessing nil
   then type, preserve successful results when the other fails, and own their
   diagnostics. Identity capture separately validates the lexical QName: a bound
   QName naming an unknown or non-derived type can still be an identity value,
   while malformed or unbound QNames invalidate the matched field. Identity
   capture cannot duplicate semantic XSI diagnostics.
   Every element start is a transaction across XML/namespace stacks,
   schema-location hints, parent-content state, content-model bits, and identity
   state. A fatal start rolls all of them back. A semantic-stop transition keeps
   the committed XML syntax state needed to parse the remainder while discarding
   semantic state. Identity field batches admit each value against the remaining
   document budget before growing one evaluator-owned staging workspace. They
   commit only complete batches, clear source references on every exit, and retain
   capacity only within the validation high-water bound. Identity dispatch resets
   and semantic discard apply that same bound to active-scope slices, selector
   hits, and the active-constraint map; the immutable schema dispatch index remains
   reusable. `MaxIdentityEntries`
   independently bounds stored identity entries, pending selector matches, and
   pending field-value slots; selection admission checks both pending dimensions
   before allocation or mutation. Retained diagnostic nodes are bounded by path
   publication extensions; encoded segments and suffix-local namespace headers
   are bounded by retained path occurrences, admitted document structure and
   bytes, identity limits, and the session high-water policy. Hint batches stage
   only new namespaces and copy the bounded retained map only when committing an
   actual change; no-op or duplicate hints do not snapshot accumulated state.
   Generic event-sink or matcher interfaces are intentionally absent: there is
   one evaluator implementation and one validation caller, while an interface
   would hide the required transaction and element-lifecycle sequencing without
   providing a real substitution boundary.
5. Schema table execution and metadata checks stay in `internal/schema`;
   instance-validation policy stays in `internal/validate`.

Compilation and validation are synchronous. Resource limits bound admitted input
and retained work, but the library does not own deadline or interruption policy.
Callers that require interruption of a blocked resolver, opener, file operation,
or arbitrary `io.Reader` MUST provide I/O that they can close or otherwise unblock.

Formatting flow:

1. The WASM adapter passes its existing bounded XML string to `internal/format`.
   The formatter accepts this immutable supplied input directly; it does not
   acquire or buffer a reader's complete contents.
2. One `internal/xmlstream` reader validates the document and records bounded
   source spans and per-element layout decisions. The flat span sequence contains
   offsets, positions, token kinds, and layout flags; it owns no decoded text,
   attribute values, or child tree. Input, token, node, and depth limits bound
   admission and retained metadata.
3. Rendering walks those spans in order with a depth-bounded layout stack and
   writes directly from the immutable caller-owned string. Shared XML lexical
   predicates decode admitted references; the renderer preserves normalized
   text and attribute values while escaping output. Validation completes before
   output begins, preserving empty output on malformed XML. Writer failures may
   leave partial output; success requires every write to complete. The WASM
   adapter discards its output builder on any error.
4. Formatting does not belong in the root public `xsd` package.

Diagnostics flow:

1. Internal packages return source-aware structured errors using public
   `xsderrors` types; schema diagnostics identify the originating schema through
   `xsderrors.Error.Path()`.
2. Public callers inspect `xsderrors.Error`, `xsderrors.Errors`,
   `xsderrors.Category`, and `xsderrors.Code`. Constructors reject invalid
   category/code combinations. Aggregate construction owns its input, accessors
   do not expose mutable storage, and `xsderrors.Flatten` is the sole top-level
   presentation projection.
   At diagnostic admission, known nil pointers and empty aggregate values or
   pointers represent absence. Construction normalizes direct absent causes;
   `xsderrors` traversal checks these shapes before invoking wrapper methods.
   External wrappers retain their identity and cause chain.
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
   time and headers. One run-owned `os.Root` contains both startup asset checks
   and request-time opens; replacing a file or parent directory cannot redirect
   reads outside that root. Each request serves only an opened regular file and
   closes it on return. The run closes the root after serving stops, including
   startup and shutdown failures. It does not compile or validate schemas.

## Change Protocol

Every architectural or behavioral change is one dependency-complete packet:

- **Contract:** the observable requirement and its oracle.
- **Owner:** the one module and state owner responsible for the decision.
- **Invariant:** what must always hold before and after the change.
- **Path:** entrypoint, canonical representation, commit or acknowledgement
  point, failure exits, and cleanup.
- **Bounds:** time, temporary memory, retained memory, recursion, I/O, and
  concurrency implications at the admitting owner.
- **Proof:** focused behavior tests, affected seam tests, and measurements for a
  changed hot path or retention shape.
- **Writeback:** the authoritative document, executable invariant, or local
  intent comment that prevents rediscovery.

Keep facts, inferences, and proposals distinct. Inspectable repository facts
need no prose cache. Expensive derived evidence may be recorded only with the
exact revision, command, environment, and status. A proposal remains in a plan
until implemented; once accepted, update this file and its enforcement in the
same change, then delete or mark the superseded plan text.

Test through the changed module's interface. Internal white-box tests are
appropriate for representations whose lifetime or corruption states cannot be
reached through a wider interface, but they supplement rather than replace
seam-level behavior. When a defect exposes split ownership, an invalid state,
or duplicated policy, repair that model and delete the obsolete path.

An agent should be able to finish a packet without loading the repository-wide
history. The packet is complete only when a future agent can recover the current
contract from this file, find the owner through the routing table, reproduce the
proof from tests or commands, and see no competing current representation.

## Tests And Enforcement

Architecture is enforced by behavior rather than convention:

| Enforcement owner | Contract enforced |
| --- | --- |
| `tests/phase_import_graph_test.go` | Package allowlists, dependency direction, context-free library code, source I/O ownership, and capability isolation |
| `tests/phase_boundary_test.go` | Required modules, single facade execution edges, session construction ownership, and retired public paths |
| `tests/root_public_shape_test.go` and `tests/external_api_smoke_test.go` | External callers use only the supported public packages and interface |
| `tests/stream_boundary_test.go` | Borrowed token data is consumed only in its valid lifetime and cannot escape through aliases or type erasure |
| `tests/schema_build_boundary_test.go` | Compiler topology mutation has one owner |
| `cmd/wasmxsd/api_test.go`, `cmd/wasmxsd/main_js_test.go`, `cmd/wasmxsd/build_test.go`, and `cmd/xsdweb/main_test.go` | Go adapter response/build contracts, input bounds, asset catalog, and server shutdown lifecycle |
| `docs/js/validation-worker.test.js` | Worker queue, cancellation, timeout, generation, and state ownership |
| `docs/js/browser/validator.spec.js` | Built WASM behavior, response branches, stale-input protection, and main-thread bounds |

The files own their exact test inventory. This contract records why each
enforcement family exists; it does not cache function names.

Root tests and benchmarks MUST use `package xsd_test`. Root tests MUST NOT
import `internal` packages. Implementation fuzz tests MUST live with the package
that owns the implementation being fuzzed.

Fuzz targets live with their implementation owner: stream parsing in
`internal/xmlstream`, schema parsing in `internal/schema`, regex syntax in
`internal/xsdregex`, and document validation in `internal/validate`. The
`Makefile` owns the executable smoke inventory.

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
- The benchmark smoke target spans every owning package selected by its bounded
  pattern; the exhaustive benchmark target remains separate.

## Rejected Alternatives

- A global or unbounded session pool was rejected because schema ownership and
  retained memory would become implicit. One idle slot per shared engine owner
  reuses the common sequential path without blocking concurrent validations or
  introducing a new memory budget. Splitting the parser's inline buffer alone
  would leave per-call allocation of the remaining reusable session state.
- A validator name cache and larger spelling caches were deferred because the
  profile did not isolate schema QName lookup from parser interning. They would
  add retained strings and invalidation policy without demonstrated benefit.

- Keeping source buffers for exact byte equality was rejected because it retains
  complete input streams. Reopening the earlier source cannot recover its exact
  prior contents, adds observable I/O, and can fail or return changed bytes.
  The rewrite explicitly uses raw byte length plus SHA-256 content identity.
  This is a collision-resistance assumption, not an exact-equality guarantee.
- Returning immediately on schema parse failure was rejected for streamed
  acquisition: it would hide later read, close, and source-limit failures that
  previously took precedence. `Input.Finish` owns bounded draining and cleanup.

- A separate root-start flag result was rejected because its sole consumer
  immediately translated it into the session's selected state. Resolving all XSI
  types eagerly was also rejected: undeclared-root selection and declared-start
  nil/type assessment have different diagnostic precedence. A transient origin
  on the canonical selected state removes repeated resolution without moving
  recovery or introducing another assessment owner.
- Adding document depth to the tokenizer was rejected for outside-root text
  validation: existing consumers already own topology. Preserving lexical text
  origin fixes the lost fact without a second document state machine.
- Refunding work after failed publication validation was rejected because the
  computation has already occurred; repeated failures would evade the aggregate
  work bound. Retryability preserves build data, not spent resources.
- An additional element-value slice projection was rejected because publication
  and validation already use the canonical packed element table. Tests observe
  published reads and independently corrupt packed projections instead of
  preserving a second representation. Private construction and semantic
  admission own correctness before the read tables are built; duplicate
  projection equality checks add no boundary.
- Path-based asset checks followed by unrestricted serving were rejected because
  symlinked ancestors and replacements could escape the configured directory.
  One standard-library root handle contains both operations.

- Raising the DFA state cap to admit avoidable expansion was rejected. Safe
  single-particle occurrence normalization prevents that expansion; the
  separately bounded scalar-work default is calibrated to the retained-state
  cap so valid counted models can reach it.
- Interleaving resolution, graph validation, and component compilation was
  rejected because partially known namespaces made ownership and failure state
  ambiguous. The loaded-graph-to-plan boundary is the only route into indexing.
- Context-bearing compilation and validation APIs were rejected because the
  library cannot interrupt arbitrary resolvers, openers, files, or readers.
  Callers own unblocking those operations; compatibility overloads would create
  parallel public APIs and execution paths without providing cancellation.
- A generic matcher/evaluator interface was rejected because there is
  one implementation and one caller; it obscured the required transactional
  sequencing without creating a substitution boundary.
- Canonical-text comparison for typed fixed values was rejected because
  equivalent duration and timezone-normalized g* values can have different
  lexical projections. Validation requests the value owner's identity
  projection despite its allocation cost; it does not add datatype-specific
  comparison rules or another parser.
- Re-resolving schema-supplied QNames in instance namespaces was rejected because
  missing or rebound prefixes change the constraint's value. Retaining the
  compiler's complete namespace history was rejected in favor of the bounded
  application-prefix projection. Capturing only names resolved by the declared
  type is insufficient when a string-first union or anyType later selects QName.
- Adding normalized spelling to every parsed value was rejected because it
  enlarges every retained list item. Returning it from evaluation keeps lexical
  output separate from value-space payloads and removes mutable result options.
- Lazy list growth or a cap on initial capacity alone was rejected because
  eventual storage still scales with input length. Rejecting a list immediately
  when it exceeds an enclosing enumeration's item bound was rejected because it
  can select a later union member and change acceptance. Complete-or-omitted
  retention bounds storage while preserving normal item and member assessment.
- Flattening repeated character-class ranges before merging was rejected
  because temporary storage scales with repeated inputs rather than their
  union. A range-head heap bounds that storage, accepting extra heap work.
  A global named-category cache was rejected because compilation already owns
  the required bounded lifetime; parser-local catalog caching suffices.
- Linear live namespace lookup was rejected because repeated resolution through
  a deep binding chain amplifies declaration churn. A second authoritative map
  was also rejected because retained contexts require immutable binding history;
  the active-prefix map is only a frame-owned projection of that history.
- A universal low cognitive-complexity limit was rejected because it fragments
  cohesive parsers, state machines, and invariant audits. The configured limit
  identifies exceptional review targets; ownership and invariants determine
  semantic boundaries.
- A one-pass pretty printer was rejected because later mixed content can change
  indentation decisions for earlier children, and malformed trailing input must
  produce no output. Validated source spans preserve these contracts without a
  tree, copied payloads, reader buffering, or temporary files. Repeating full XML
  tokenization was rejected after measurements showed avoidable parsing work;
  retained offsets refer only to the caller's existing immutable string.
- Unlimited zero-value formatter limits were rejected because formatting still
  retains source spans, per-element decisions, and nested frames. Finite defaults bound these
  dimensions, while positive options allow explicit smaller or larger budgets.
- Trusting a writer's nil error after an incomplete write was rejected because
  formatter success must imply complete XML output; the bounded writer checks
  every delegated write in one place.
- Per-element copies of the accumulated schema-location hint map were rejected
  because valid repeated or absent hints multiplied bounded state by document
  length. A staged delta preserves start-transaction rollback and copies only
  when a batch adds a namespace.
- Per-value heap staging and post-hoc limit checks for ID and IDREF batches were
  rejected because transactional validation then created garbage proportional to
  every value and could exceed the configured identity budget before failing. One
  bounded evaluator-owned workspace admits each field before growth, preserves
  all-or-nothing commits, and is cleared after success and failure.
- A separate `MaxIdentityFieldSlots` option was rejected because it would add a
  public tuning dimension while still requiring a finite default that rejects
  the same amplified inputs. `MaxIdentityEntries` owns retained identity
  cardinality through three independent ceilings; callers can raise that one
  budget deliberately when schemas require greater field fanout.
- Eager full-path strings were rejected because a long prefix was copied for
  every retained fact. One parent node per element and one expanded-name node per
  element were rejected because disjoint deep paths multiplied retained structs
  by depth and fact count. Rendered-text chunks were rejected because expanded
  paths copied a namespace URI at every occurrence. Fixed-size encoded chunks
  were rejected because a common prefix below the threshold still repeated its
  encoded bytes and suffix-local namespace headers per fact. Global full-name and
  namespace interners were rejected because unique local names or namespace URIs
  created another document-wide retained projection with rollback-sensitive
  state. The document instead owns encoded suffix nodes plus active element-
  boundary references, which share any live common prefix without retained per-
  element nodes. An identity-owned path arena and a separate retained-path limit
  were rejected because they would add a competing writer or public policy
  dimension; document structure, bytes, identity-entry limits, and session high-
  water retention already bound the chosen representation.
- Parser-mode-specific line counters were rejected because CR/CRLF behavior and
  diagnostics then diverge. The byte stream owns logical line positions, while
  token modes own normalization of their emitted payloads.
- Main-thread WASM execution was rejected because synchronous validation blocks
  rendering and cannot be interrupted. Worker termination is the cancellation
  boundary.
- Boolean-only WASM results and object-valued window globals were rejected. A
  tagged response distinguishes invalid documents from operational failures,
  while worker-owned limits keep the browser boundary explicit and isolated.
- Compatibility fields, aliases, and parallel constructors for diagnostics were
  rejected because they would preserve two representations and two location
  paths for the same fact.
- A semantic source struct with one nullable pointer per capability was rejected
  because it permitted contradictory records and forced every consumer through
  a nil chain. One closed variant owns at most one capability record; common ID
  and `xml:base` facts stay on the node source, and particle facts stay with the
  variant that admits them. Syntax-only grammar nodes intentionally retain no
  capability variant.

## Documentation Ownership

- `AGENTS.md` is the low-context execution router. It points to this contract and
  defines discovery, implementation, evidence, and writeback discipline without
  copying the package graph.
- README documents public usage and command workflows.
- `docs` contains XSD 1.0 reference documentation and the browser validator's
  deployable assets. Review reports, comparison reports, completed plans,
  benchmark evidence, and verification logs do not belong there.
- `docs/spec` contains searchable local specification material and an index; it
  does not define repository behavior.
- `tests/README.md` documents corpus and harness operation. Counts and other
  cheap derived facts come from the manifest or test commands rather than
  manually maintained prose.
- `.codex` plans and ledgers are ignored working memory. Plans own future work;
  ledgers own revision-scoped evidence. Closed history may be retained for
  provenance but is loaded only when its finding, revision, or rejected approach
  is relevant.
- Code comments own only non-obvious local intent, invariants, and consequences
  needed to change the adjacent implementation safely.
- This file is the sole architecture source of truth. Other documentation may
  link here but must not restate a competing package graph or lifecycle model.

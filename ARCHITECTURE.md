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
| `internal/corpus` | Conformance corpus file layout and grammar for tests and tools: inventory shards, observation rows and profile; read through `fs.FS`, imported by no library package |
| `tests/internal/compilework` | Test-only command that compiles one schema through the root facade and reports the least analysis and dependency limits under which it compiles; built with coverage counters by `TestContentModelCompileWorkGrowsWithChargedSteps` in `tests/content_model_compile_work_test.go` and `TestAttributeDerivationAuditDoesWorkInProportionToChargedSteps` in `tests/attribute_use_audit_work_test.go`. It is the one internal package that imports the facade: the depguard rule `internal-must-not-import-facade` exempts it by path, and no library package imports it |
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
| Schema source input | reading -> ended or failed -> finished (Finish or Discard) | One phase field; the frozen result is the only post-finish read; close failures join the read failure |
| XML stream | reset -> borrowed token -> advance/invalidate -> EOF/error -> detach | Borrowed bytes never survive advance; only token-boundary EOF is success |
| Namespace frame | prepare -> commit -> end or abort | The opaque top-frame capability is the sole pop authority |
| Schema publication | mutable build -> validate -> consume/seal | Validation preserves build data; work stays charged; successful consumption is the only commit |
| Browser client | idle -> loading -> ready -> running -> ready; running -> loading on request timeout; cancel -> loading; loading/ready/running -> failed -> loading on start or validate; any -> disposed | One discriminated state owns the worker, timer, active request, and readiness promise; leaving a state releases them and invalidates the worker generation; the limit catalog survives respawn and failure |

## Authority And Navigation

Use the narrowest authoritative source that answers the question:

1. Explicit requirements, XSD 1.0, XML 1.0, and exported behavior are binding.
   XSD 1.0 means the 28 October 2004 Second Edition with its approved errata
   and the corrections `docs/spec/SOURCES.md` adopts. XML 1.0 means the edition
   XSD 1.0 cites, except that name characters follow the Fifth Edition
   (26 November 2008) everywhere, including `xs:Name`, `xs:NCName` and the
   regex `\i` and `\c` classes; the Fifth Edition rule that reads any `1.x`
   version as 1.0 is not adopted. `docs/spec/SOURCES.md` owns each boundary.
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
`MaxContentModelAnalysisSteps` bounds the content-model work `workBudgetKind`
lists, and through that work the heap that automaton
construction uses (derived bounds for row edges and state keys, a measured
bound for counter vectors, and measured heap, in [RESOURCE_LIMITS.md](docs/RESOURCE_LIMITS.md);
derivations in the Go docs of `deterministicRow` and `dfaCounterVectors`). That bound is per content model. `MaxSchemaDependencySteps`
bounds the compiled tables retained across models (per-step bound in
[RESOURCE_LIMITS.md](docs/RESOURCE_LIMITS.md)): `contentModelCompiler` charges
each model record, row, edge and `xs:all` term before allocating it, and
`dfaRowIndexAnalysis` each name-index entry it installs. A direct sequence's
rows are windows of one edge run (`compiledModel.Run`) and share one index of
the run (`publishedDFARun`), so its tables grow linearly with its particles.
`compileContentModels` keeps only the published form (`compiledModelRead`) of
each complex type's root model, indexed in the model's scope, and the kind of
every other reached model, whose rows are still charged because its Unique
Particle Attribution check reads them; publication takes a table without
copying it when every element it names keeps its ID in the published
numbering, and copies it otherwise; `sealSchemaProgram` adds dense indexes,
which rewrite rows the build shares, only after the last step that can reject
the build. Spent work is never refunded. Retained derived records
are charged to `MaxSchemaDependencySteps` at their admission owner:
`appendModelParticles` is the only append to retained model particles and
charges one step per particle before allocating (the restriction normalizer
builds transient comparison nodes that it charges to
`MaxContentModelAnalysisSteps` and never retains); `addModel` (one step per record) and
`completeModel` write the model table, and `completeModel` installs only
particles already charged; `addAttributeUseSet` charges `len(Uses)` and
`appendWildcard` charges `len(Namespaces)`. Admitted models and wildcards are
immutable; `contentModelByID` and `wildcardByID` share them on read (see
`appendWildcard`). Wildcard set operations take the budget they charge as a
required argument (cost model in the `wildcard.go` comment on admitted namespace
lists): UPA overlap, sequence wildcard equivalence and content
restriction subset charge `MaxContentModelAnalysisSteps`; attribute-wildcard
intersection, union and restriction subset charge `MaxSchemaDependencySteps`.
The publication audit re-derives each attribute use set's wildcard exactly
once, through `admittedWildcardWork`, uncharged because compilation charged
the same comparison. A new retained table must charge in its admission function. Admission functions return unlocated budget errors. The
builtin components are seeded before the caller's `MaxSchemaDependencySteps`
budget is installed and spend none of it, so every dependency-limit failure
has a schema document to locate it.

The content model table holds declared components. Lowering drops zero-count
particles and keeps every declared group as its own particle; xs:anyType's
content is a 1..1 sequence holding its lax `##any` wildcard 0..unbounded,
and a {content type}'s variety alone says whether it is mixed. Where a model
group's occurrence is recorded is owned by `modelUse` and `compileCompositor`,
and `appendModelParticles` admits a model particle only when the record it
names has a unit occurrence. `applyModelGroupOccurrence` and
`compileModelParticle` make the records that share another model's particles,
and `chargedParticles` names the passes that read particles through it.
`explicitContentEmpty` alone decides from the source whether a complex type's
·effective content· is empty (Structures §3.4.2 complex content
clauses 2.1.1 to 2.1.3); `compileExplicitContent` is the one entry point the
direct, restriction and extension paths call, and returns the effective
content with the variety it gives the {content type}, with no model for empty
content, which mixed content stores as the synthetic empty sequence of clause
2.1.4 (an empty model). `extensionContentType` alone
builds an extension's {content type} (clause 3.2), and the publication audit
`complexContentExtendsBase` checks the same three forms. Each reader that
reduces a model owns one rule: the restriction normalizer
(`compareNormalizedRestriction`) owns Particle Valid (Restriction) clause 2.2
and decides pointlessness on the declared group; each comparison node it adds
carries the count range of the group it stands for, which the count analysis
reads for Particle Emptiable and the effective total range, and `emptyGroup`
decides from that range whether a group left with no members matches only the
empty sequence or nothing ([I-028](docs/spec/INTERPRETATIONS.md#i-028--content-model-checks),
[I-042](docs/spec/INTERPRETATIONS.md#i-042--empty-choice)). A group it finds
pointless that way is only marked: its parent ignores it, and a derived
{content type} particle so marked leaves nothing to compare and restricts only
an emptiable base (`validatePointlessRestriction`), as W3C mgE014 expects.
Each case of the Particle Valid (Restriction) table owns its own range rule
(NameAndTypeOK, NSCompat and NSSubset compare the particles' own ranges;
Recurse, RecurseLax and RecurseUnordered clause 1 the groups' ranges; MapAndSum
clause 2 its product range; NSRecurseCheckCardinality clause 2 the group's
effective total range), and no check compares the aggregate range of a
content model. `validateEmptyContentRestriction` owns Derivation Valid
(Restriction, Complex) clause 5.3 for an empty derived {content type}; the execution
projection owns language-preserving reduction without copying: direct
compilation walks the execution particles (`executionParticles`), which splice
every 1..1 child group of the same compositor in place, charging one step per
particle visited and stopping when compilation stops; `compileDirectModel`
walks them once, through `directModelAdmission`, which owns the particle
kinds direct compilation takes and its row count under
`MaxContentModelStates`, and builds the table in a second walk under a budget
of exactly the steps the first spent and no more rows than admission counted,
both internal invariants, so no row is built or charged for a model automaton
construction compiles; `executionModel`
lifts a model whose walk yields one particle, as `executionParticle` reduces
it, and otherwise hands the declared model to automaton construction, which
walks nested groups as declared; `executionParticle` reduces singleton
particles for direct compilation, and the projection recognizes lax-any
content in that reduced form by the shape `builtin.go` defines for
xs:anyType's content particle. The direct Unique
Particle Attribution check (`checkContentModelUPA`) also reads nested groups as
declared, through facts the content-model analysis owns:
`lastParticles` alone decides which particles of a nested group can match its
last element, and `ModelContinuations` computes the continuation set of each
model once, holding each particle once, so a group referenced N times adds
no copies; both direct checks iterate the shared sets. Unique Particle
Attribution applies to the content models of complex types ([I-050](docs/spec/INTERPRETATIONS.md#i-050--unique-particle-attribution-applies-to-the-content-models-of-complex-types)): the direct
check and automaton construction (`compileContentModels`) visit, in source-ID
order, the root model of each complex type and every model it reaches through
particles (`complexTypeReachableModels`), so a model no complex type reaches is
neither checked nor compiled and spends no automaton work or
`MaxContentModelStates` budget. The compiled table (`compiledContent`) holds
the reached models with their source IDs, and publication enters only the
roots of complex types that are not detached. Instance reachability decides
what is detached: an instance names only global components (root, `xsi:type`,
wildcard, substitution member), so a complex type or identity constraint no
walk from them reaches is never assessed. `detachUnreachableComponents` owns
that decision and charges no budget, because it reads only models
`complexTypeReachableModels` already charged. The same walk reaches the
simple types a published read can name: global types, global attribute
types, a reached element's type, a reached complex type's text type and
attribute-use types, and the base, list item and union members of a reached
simple type. It reads each model, attribute-use set and simple type once, so
an attribute group that many types share costs one read, not one per type. Publication keeps a detached type's ID and derivation record and a
detached constraint's ID for keyref `refer` and diagnostics, but no content
model, attribute uses or evaluation program. An unreached simple type keeps
its ID, derivation record and value-program slot, but `Builder.SealKeeping`
drops its value definition (facets, patterns, enumerations), and validating
it reports `value.ErrMetadata`. The inline simple type of a source item that
maps to no component, a zero-count element or a prohibited attribute, keeps
nothing once checked, because no component can hold its ID:
`compileUnheldSimpleType` drops it from the build and the value builder
(`dropSimpleTypesFrom`, `value.Builder.Truncate`) without changing a verdict
(`droppedSimpleTypeCharges`). Unreached element declarations
are not published, and validation sees only published element IDs, which
`publishedElements` owns. A published holder of element IDs is either a build
table that names only declarations whose published ID is their build ID or a
copy built over published IDs, never a table that mixes the two.
`validateReachableHolders`, `validatePublishedIdentityConstraints`,
`validatePublishedSimpleTypes`, `SealKeeping` and `publishedElements` reject a
published holder that names a detached or unreached component. Unique
Particle Attribution, restriction and automaton construction still cover
unreached components, so reachability changes no verdict.
Element Declarations Consistent still checks every model ([I-026](docs/spec/INTERPRETATIONS.md#i-026--element-declarations-consistent-counts-declarations)). The
count analysis
(`countRange`) represents a term that matches nothing, so such a member adds
no alternative to a choice. `baseIsUrType` alone decides the ur-type
exemption of Derivation Valid (Restriction, Complex) clauses 5.1 and 4.3, from
the restriction's base type.

Every diagnostic about schema document content carries the document path, line
and column. Two other kinds carry less: a failure to obtain an explicit
document (it cannot be opened or read, or exceeds `MaxSchemaSourceBytes`)
carries its path only, and a failure about the source set (no sources, more
than `MaxSchemaSources`, a source without a name) carries none; a referenced
document that cannot be obtained is located at its reference. The component
scope (`componentScope`, opened by `enterComponent`) is the compiler's locator:
its leave fills the missing location of any diagnostic leaving a component from
that component's node, so a failure is located at the innermost component being
compiled, not at the referring site; a charge made at a reference before the
scope opens keeps the reference location. The scope is opened to compile each
component and again, by a compiler loop, for each check made on it after
compilation, at the node `componentScope` names for each check; a content
model's checks open it at the model's source, which `addModel` requires of
every model but the builtin anyType model. Particles
carry no source, so a content-model violation inside a referenced group is
located at the first model in compilation order whose check reaches it (see
Rejected Alternatives). A `schema.limit` failure raised after loading and
outside every component scope is set-wide: target-context planning locates it
at the document being added or whose references it is walking, and
`compileMappedSources` locates the rest, the work `locateSetWideLimit` lists,
at the first explicit root document in plan order, which is
by document name. Loading locates its dependency charge at the reference being
resolved and a `MaxSchemaReferences` failure at the document whose references
exceed it. Publication consumes the
build only on success; `*schema.Schema` exposes only immutable derived reads.

Schema-document diagnostics have an order. A violation has one decision point,
and a compilation reports the violation whose decision point comes first. Two
tiers hold the decision points, and one criterion assigns every rule to one of
them.

A rule of any kind (Schema Representation Constraint, schema for schemas, or
Schema Component Constraint) is parse tier when every input it reads is on the
tag or in earlier content of the same document. A Schema Component Constraint
is parse tier only when, in addition, every value of the constrained property
that can violate it is written by one tag and never produced by resolution,
derivation or the including context; a value the processor synthesizes as a
constant (the 1..1 particle of an extension, a builtin component) cannot
violate it. Occurrence bounds, attribute declaration names and selector and
field paths qualify. All Group Limited through a group reference does not, and
neither does the XMLSchema-instance target namespace of an attribute
declaration, whose chameleon form takes the namespace of the including
document. A rule is classified once: when any form of it needs resolution or
set-wide context, the whole rule is compile tier, unless the specification
states the forms as separate constraints, as it does for the occurrence ranges
the schema for schemas gives `xs:all` and its elements and All Group Limited.

The parse tier decides while a document is read, at a start tag (`admitStart`)
or an end tag (`completeTypedNode`). Its violations are reported in document
order of their decision points and, at one decision point, in this stage order.
At a start tag:

1. the root element is `xs:schema`;
2. unsupported vocabulary (`rejectUnsupportedVocabulary`): an XSD 1.1 element
   or attribute, or `xs:redefine`;
3. character data earlier in the parent;
4. the parent's content admits the child: no foreign element, the annotation
   rules, then the parent's child order (`childOrderState.accept`), which also
   rejects a facet the restriction already holds (src-single-facet-value;
   `xs:pattern` and `xs:enumeration` may repeat);
5. the element's own attributes in the order written. For each attribute:
   that the element may have it in this placement (`schemaElementAttributeAllowed`,
   `misplacedSchemaAttributeError`), then its value: the name syntax of `name`
   and `id`, an `id` the document already declares, the lexical form
   (`checkSchemaAttributeValueSyntax`); for `minOccurs` and `maxOccurs` the
   lexical form, the count against uint32 and `MaxFiniteOccurs` and then the
   range the schema for schemas gives `xs:all` and an element directly in it
   (`occurrenceAdmission`); for the `value` of a size facet, `xs:whiteSpace`
   and `xs:pattern` its integer and uint32 cap, its mode, and its regular
   expression with the `MaxSchemaPatternRetainedBytes` charge
   (`admitFacetValue`); for the `fixed` of a facet its boolean form
   (`admitParsedAttributeValue`); for `xpath` the path grammar and the
   prefix bindings of its name tests, branch by branch from the left
   (`admitIdentityXPath`); the lexical form and prefix binding of a QName; and
   anyURI syntax;
6. the attributes the placement requires (`checkRequiredSchemaAttributes`),
   then the attributes that exclude each other
   (`checkValueConstraintAttributes`): `default` with `fixed`, and `default`
   with a `use` other than optional; then `minOccurs` not greater than
   `maxOccurs`, each written or by default (`occurrenceAdmission`); then an
   attribute declaration named `xmlns` whose `use` is not prohibited
   (`checkAttributeDeclarationName`).

At an end tag:

7. character data in the element;
8. the children the element requires (`childOrderState.missingChild`).

Malformed XML and the parse limits are reported where the reader meets them, in
the same document order. Documents are read one at a time, explicit sources in
name order and then referenced documents as the loader reaches them, so the
parse-tier violations of one document precede those of the next, and a loading
failure is reported when the loader reaches its reference.

The compile tier decides after every document of the set is read, so a
compile-tier violation is reported only when no document has a parse-tier
violation. Each compile-tier rule has a reason the criterion names: reference
resolution and import authorization (resolution); duplicate components and
attribute uses (set-wide uniqueness); cycles, derivation and `final`
(resolution); whether the base type admits a facet, a facet's value against
the base type, and a value constraint against its type (type-dependent value);
All Group Limited through a group reference (resolution); the content model
rules: Unique Particle Attribution, Element Declarations Consistent and
restriction (content-model analysis); and the XMLSchema-instance target
namespace of an attribute declaration (chameleon namespace).

Identity-constraint names are one schema-wide symbol space per target
namespace (Structures §3.11.1). `declareAllIdentityConstraints` declares
every constraint of every document context by expanded name, rejecting a
duplicate name, before any component compiles; the compiler's declarations,
the expanded name of each constraint ID, are the only record of the names
until they are interned, and compilation only looks them up. A keyref `refer`
therefore resolves against every name, whichever component holds the keyref or
the referenced constraint. Declaration interns nothing: a constraint's name is
interned by `nameIdentityConstraint`, the only writer of the constraint's
interned name and of the published global identity constraints, when the
element holding it compiles, or before global elements compile
(`nameAllIdentityConstraints`), and its selector and field names when it
compiles. A constraint is named once its interned name is set. Every name therefore takes its name ID in compilation order, which
is not only a diagnostic concern: the dense DFA index depends on the
local-name IDs of a content-model row's elements (`addDenseDFAIndexes`).
An anonymous type interns no name; the type declaration's scope, checked at
publication by `typeOwnership`, decides whether it has one. `MaxSchemaNames`
(`nameTable`) therefore does not count anonymous types, and
`MaxSchemaInstantiatedNodes` (schema loading) bounds them, since each takes at
least one source node occurrence of its own.

Each rule has one owner: one function, called at the tag that decides it,
whose product the tag's typed source record stores and compilation reads. The
child orders (`childOrderFor`) own where an element may appear; the placement
of a component (`schemaElementPlacementOf`) selects its attribute rules and
says nothing about whether its parent admits it. `schemaElementAttributeAllowed`
and `misplacedSchemaAttributeError` own the attributes an element may have,
`checkRequiredSchemaAttributes` the ones it must have,
`checkValueConstraintAttributes` the ones that exclude each other, and
`checkSchemaAttributeValueSyntax` the lexical form of a value other than the
ones below.

The tag stores these admitted values in the typed source record, and
compilation reads them without parsing again: the resolved names of QName
attributes (`admitSchemaQNameAttribute`), the occurrence of a particle
(`occurrenceAdmission`), and from a facet's value and `fixed` its size, its
whiteSpace mode, its compiled pattern and its fixed flag (`admitFacetValue`,
`admitParsedAttributeValue`). Patterns are compiled through `schemaPatterns`,
which the compiler owns and lends to start-tag admission while the set loads:
each distinct pattern text is compiled once, at the first facet that names it
in document load order, and charged against `MaxSchemaPatternRetainedBytes`
before it is kept, so a breach is reported at that facet's start tag.
A selector or field path is parsed once, by `admitIdentityXPath` at its start
tag: prefixes resolve against that tag's namespace scope and an unprefixed
name test has no namespace whatever the including target namespace. The tag
owns every syntax and prefix diagnostic of every path, and stores the form of
each branch with its name tests bound but not interned
(`identityXPathSyntax`); the parsed document stays immutable. The compiler
builds the interned paths from it the first time the node compiles
(`compiler.identityPaths`), after the keyref `refer`, so path names take name
IDs in compilation order, as every other component name does, and a
`MaxSchemaNames` breach on a path name is a compile diagnostic at the selector
or field, after every syntax diagnostic. The compiler keeps the paths once per
node, so every identity constraint compiled from the node, in each chameleon
context, shares them; a missing admitted path is an internal invariant. The other
attribute values that are single keywords or short lists (booleans, `form`,
`use`, derivation sets, `processContents`, wildcard namespaces,
`targetNamespace`) stay lexical in the record and are parsed again with the
same allocation-free parser where compilation uses them; a failure there is an
internal invariant (`admittedValueInvariant`), not a second schema diagnostic.

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
declarations are intrinsic; an unresolved `type`, `itemType` or `memberTypes`
loads and fails assessment of the items that use it, an unresolved
`substitutionGroup` loads and leaves the member without an affiliation, so a
member without its own type validates as `anyType`, and other unresolved
references are rejected
([I-023](docs/spec/INTERPRETATIONS.md#i-023--unresolved-references)).

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
  `Complete`, `SealKeeping`) and supplies its construction limits.
  `SealKeeping` keeps every type's ID but drops the definition of each type
  the compiler did not reach, after checking that no kept type's base, list
  item or union member is dropped; the dropped record's variety is one no
  `TypeSpec` can carry, so every evaluation switch rejects it without a check
  on the per-value type lookup. Enumeration and
  value-constraint literals (`Builder.Validate`) and instance values have the
  same value semantics. A bound literal is admitted by
  `Builder.AdmitBoundLiteral`: base value-space membership, or, for a
  `minExclusive` or `maxExclusive` equal to the base's effective same facet,
  only the base's patterns and other bounds; the base's non-bound facets are
  checked before its bounds. The compiler admits it while
  reading the facet, so the diagnostic names the literal; the
  valid-restriction clauses against inherited bounds and the
  lower-versus-upper and fixed-facet checks belong to `Builder.Complete`'s
  restriction derivation, located at the simple type.
  One order (`compareAtomic`) and one equality (`equalAtomic`) relate two
  values of a primitive wherever they are related: an instance value against
  a bound, enumeration, fixed values, identity constraints, a literal against
  the base's bounds, an exclusive restatement, a bound under a fixed base
  bound, the opposite-side restriction clauses, and the effective
  lower-versus-upper check. The order never separates equal values, and two
  values it ranks equal are equal, except durations equal only in order
  (Duration equality and order in `docs/spec/INTERPRETATIONS.md`). The doc
  comment of `atomicValue` names the owner of each primitive's order,
  equality and enumeration key and states what a value type may hold. An
  `xs:time` value is its timezone-normalized time of day, so `24:00:00` is
  `00:00:00` (I-045 and I-047 in `docs/spec/INTERPRETATIONS.md`). One
  relation needs more than the two values: the same-side restriction clause
  orders `xs:time` bounds by un-wrapped time. The day that the hour 24 or a
  timezone offset moves a bound to is a fact of the bound
  facet (`boundValue`), not of the value, and `compareDerivationBounds` is
  its only reader. Restriction derivation returns the effective facet
  program, and the compiler does not reconstruct that merge. `FacetAllowed`
  is the only facet-applicability rule for schema admission, value
  construction, and diagnostics.
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
  compile-time evaluation through `Builder.Validate`,
  `Builder.AdmitBoundLiteral` or `Builder.Complete` charges it, including a
  facet literal's base-type check. There is no uncharged evaluation path, so `MaxSchemaTotalPatternWork` bounds all
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
  keyed by pattern text belong to `internal/schema` (`schemaPatterns`), which
  admits each pattern at its facet's start tag in document load order;
  identical text is compiled and charged once.

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
authoritative test inventory under `tests/manifest`.

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
  without self branches, `SelectsSelf`, and `AttributeFieldBaseDepths` for
  every constraint a reachable element declaration owns; a detached constraint keeps
  its ID for keyref `refer` and diagnostics but has no program or branch;
  validate owns all matching state, scopes, and tables, and enables identity
  evaluation only when some reachable element declaration owns a constraint.
- `internal/schema` strips XSI attribute-use defaults and fixed values, so the
  four intrinsic XSI attributes use dedicated assessment only.
- Schema-location hints never participate in component selection. On assessed
  elements each hint item is checked as anyURI during attribute assessment
  (xsi:schemaLocation is a list of anyURI of any length; no pair-count rule);
  skip-processed content is not checked; no hint state is retained. An assessed
  element whose start fails fatally before attribute assessment gets no hint
  diagnostic. Missing roots, types, and
  wildcard declarations are ordinary assessment errors against the compiled
  component set; which unresolved schema references load as missing components
  is [I-023](docs/spec/INTERPRETATIONS.md#i-023--unresolved-references).
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
   `xml:base` per resolver context, accounts every source byte and read or close
   failure through `internal/source`, and parses through `internal/xmlstream`
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

   Name order is computed once by its owner and never by interned name ID.
   Expanded-name order, namespace URI then local name by text, has one
   definition (`compareExpandedName` and its namespace key
   `compareNamespaceURI`): the substitution table keeps each head's members in
   that order beside its name-ID lookup order, and a published wildcard keeps
   its namespaces in the namespace-key order. A check that stops
   at its first failure visits substitution-group members in that order
   (Element Declarations Consistent, the generated choice of a restriction,
   the Unique Particle Attribution name), so the failure reported and the work
   spent before it do not depend on name IDs. The expected-child projection
   inspects at most 256 alternatives: the edges of the DFA state or the
   all-group terms in compiled model order, and the substitution members of
   each element particle in expanded-name order, stopping a head's members at
   the first that sorts after eight kept element names. It displays at most
   eight inspected alternatives: wildcards, then element names, each kind in
   its diagnostic order (`compareContentExpectation`), element names in
   expanded-name order, each listed once however many particles render it. Wildcards take at most four of the eight
   while element names remain to show, and at least four while wildcards
   remain, so neither kind hides the other. A wildcard shows its first eight
   namespaces. The text therefore never
   depends on interning order, and it does not depend on the order of
   declarations unless the 256-alternative limit cuts across the edges of one
   state, whose compiled order follows element declaration IDs.

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
   termination, restart, and immutable state snapshots. The page renders those
   snapshots and keeps no readiness state of its own: input stays editable once
   any worker has announced the build's limit catalog, Validate respawns a
   failed worker, and a worker that never loaded offers Retry without clearing
   input. The page ignores results whose input epoch is no longer current. The page owns editor/file
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
| `internal/value/value_laws_contract_test.go` | For every primitive, one order and one equality: equality decides identity keys and enumeration membership, the bound facets agree on one order that never separates equal values and separates unequal ones outside the declared duration exception, and canonical text denotes its value; checked on declared pairs and on a generated grid of lexical forms |
| `schema_diagnostic_order_test.go` with the catalogue in `schema_diagnostic_catalogue_test.go` | The schema-document diagnostics order of `internal/schema`: a catalogue of single violations, each declaring its tier and stage from the contract and never from compiler output, and every ordered pair of them (as consecutive components, inside one component, on one start tag in both attribute orders, and across two documents) reports the violation that tier, decision point, stage and attribute order select. A rule decided later than the contract says is listed by name in `knownSchemaDiagnosticOrderGaps`, and a listed gap that holds fails as stale |
| `tests/harness_test.go` (`checkDiagnosticLocations`) | Every compile diagnostic of a corpus case, and every validation or unsupported diagnostic of an instance run, carries a path and a line; a compile pass that reports outside a component scope fails the corpus |
| `tests/oracle_observations_test.go` with `tests/oracle-backlog.txt` | Every measured corpus case, a project case or a W3C case whose effective expectation overrides the W3C one (`corpus.Case.Measured`), has an observation row for every provider and phase, as the `tests/README.md` Corpus Contract defines measured, and no other rows. Every row whose validity differs from the project expectation has a `differences` entry citing an interpretation record or anchored row that lists the case and names the provider; only cases listed in the backlog are exempt, and a listed case that is unknown or has every difference owned fails as stale |
| `tests/schema_export_surface_test.go` | The exported top-level identifiers of `internal/schema`, `internal/value` and `internal/validate` equal an explicit allowlist of the surface consumed across packages |
| `tests/resource_limits_doc_test.go` | `docs/RESOURCE_LIMITS.md` option tables list every public option with its code-owned default |
| `tests/root_public_shape_test.go` and `tests/external_api_smoke_test.go` | External callers use only the supported public packages and interface |
| `tests/stream_boundary_test.go` | Structural borrowed-type restrictions and intraprocedural retention checks, including raw byte fields and aliases erased to interfaces |
| `tests/schema_build_boundary_test.go` | Compiler topology mutation has one owner |
| `internal/schema/wildcard_work_external_test.go` with `internal/schema/wildcard_test.go` | The external test compiles interleaved namespace lists of 511 to 1,024 namespaces and requires at least 256 more steps of the owning budget when the lists are compared than when they are held apart, for UPA overlap, an element-wildcard restriction, an attribute-wildcard intersection and an attribute-wildcard restriction. `TestWildcardSetOperationsChargeEveryNamespaceVisited` requires each set operation to charge at least its visit count on lists of unequal length: overlap and intersection the shorter in-range run, subset the subset list, union both lists, intersection with `##other` the finite list, equality the list. Neither covers a consumer that passes a budget that does not charge, attribute-wildcard union through compilation, or sequence wildcard equivalence |
| `TestCompiledContentTablesChargeEveryRowEdgeAndIndexEntry` in `internal/schema/content_table_budget_external_test.go`, `TestCompileRootContentModelChargesItsRecordRowsEdgesAndTerms` in `internal/schema/content_compile_budget_test.go` and `TestRunIndexedRowsDispatchAsLinearScan` in `internal/schema/content_run_index_test.go` | A compiled model needs exactly two steps for its record plus one per row, edge and `xs:all` term; pairs of one-type schemas that differ only in one compiled table (optional against required sequence, with and without repeated names, repeated against single choice, `xs:all` against sequence) differ in their least `MaxSchemaDependencySteps` by exactly the rows, edges, `xs:all` terms and name-index entries of the two tables, so a direct sequence's index grows with its names, not its rows. Every row that shares a run index dispatches every input as a linear scan of its edges does. `TestCompileRootContentModelChargesOnlyAutomatonTablesWhenDirectCompilationDoesNotApply` requires a sequence or choice holding a group that direct compilation cannot splice to need exactly the records of its automaton table, `TestCompileDirectModelDeclinesWithinThreeStepsPerParticleAndNoRetainedSteps` requires direct compilation to decline it within three analysis steps per particle and no `MaxSchemaDependencySteps`, and `TestDirectBuildRejectsWhatItsAdmissionDidNotCount` requires a direct build to fail with an internal invariant when its walk or rows exceed what admission counted |
| `tests/content_model_particle_reads_test.go` | A content model's particle list is read through `chargedParticles`, except by the functions it lists, each with the owner that charges the particles it reads. The list is followed to a fixpoint from the `Particles` field into every holder: a parameter of a static in-package call, including a variadic parameter it is spread into with an ellipsis, and a variable, struct field or function result it is assigned, stored in a composite literal, or returned as; the result of `chargedParticles` holds none. A new function that reads a holder fails it, whether directly, through a package-level variable initializer, or through a list another function stored or returned, and so does a listed function that no longer exists or reads no particle list. A list passed to an interface method, a function value, or a variadic parameter without an ellipsis is read by the caller, and a list returned by a function that is exported, referenced other than as a callee, or a method named like an interface method is read by that function. It does not catch an uncharged loop added inside a listed function, a list stored as an element of an array, map or slice (the continuation sets and their cache) and read through it elsewhere, or a list whose particles are copied into another slice |
| `tests/content_model_compile_work_test.go` with `tests/internal/compilework` | For each content-model work family (restrictions of all, choice, sequence and wildcard bases, extension, wildcards in a sequence, group references, spliced groups, an emptiable base restricted to empty content, substitution groups in a choice, and a repeated sequence group), the statements of `internal/schema` per charged step of two added components may grow by at most the family's limit when the components grow sixteenfold; each limit lies between the family's measured growth and its growth under an uncharged loop one degree above its charged steps. Every function the particle-list guard allows must execute more often with the added components of some family, except one listed as reachable only from a rejected restriction. It catches only uncharged work that grows faster than the charged steps of its component, so a quadratic loop in a family whose charged steps are quadratic passes. It does not see work in a for-clause condition or post statement, inside a library or builtin call, or in another package, and measures only compilations that succeed |
| `tests/attribute_use_audit_work_test.go` with `tests/internal/compilework` | Pins the publication audit of a derived attribute-use set to index lookups: the statements of `internal/schema` per charged step of an added extension, or of an added restriction redeclaring every base attribute, may grow by at most 1.4 when the base grows from 128 to 2048 attributes. It has the measurement limits of the row above |
| `tests/shared_attribute_group_work_test.go` with `tests/internal/compilework` | Types that share one attribute-use set of at least 64 uses read it once: the statements of `internal/schema` per charged step of two added types referencing one attribute group may grow by at most 1.4 when the group grows from 64 to 2048 attributes, so neither the reachability walk nor the implicit restriction check of each type rereads the shared uses. The restriction check rereads a smaller shared set for each type, at most 63 uses per type, so a schema of small sets allocates no memo. It has the measurement limits of the content-model row above |
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
- The 32-bit target runs the library tests as linux/386 on linux hosts, the
  only hosts that execute them, and vets the linux/386 build elsewhere.
- WASM build/test targets own the Go/JavaScript bridge and matching Go runtime
  support file.
- Deterministic worker tests remain separate from the browser-level built-WASM
  integration target.
- The web target builds assets before starting the bounded loopback server.
- The CLI target builds `cmd/xmllint` directly; the Go build cache, not copied
  source prerequisites, decides when internal changes require rebuilding.
- The benchmark smoke target is separate from the timed exhaustive benchmark
  target.
- The libxml2 comparison target builds the CLI and runs the opt-in large
  benchmark harness against an explicitly selected libxml2 executable; it
  stays outside the verification contract.
- The oracle image target builds the pinned cross-validator container, and the
  oracle measurement target rewrites `tests/oracles` from it. Both need Docker
  and stay outside the verification contract.

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
- Deciding tag-decidable Schema Component Constraints when their component
  compiles (minOccurs above maxOccurs, the occurrence limits, the `xs:all`
  ranges, an attribute named `xmlns`, selector and field paths) and a repeated
  facet with the other facet rules was rejected: each then lost to a parse-tier
  violation later in the same document, so the reported violation depended on
  which tier owned a rule rather than on document order. The reason given for
  the `xs:all` ranges, one owner for All Group Limited, does not hold: the
  schema for schemas states the ranges of `xs:all` and its elements as a
  separate constraint from All Group Limited through a group reference, and
  each now has its own owner.
- Checking the syntax of selector and field paths only when the identity
  constraint compiles was rejected with the tier rule above.
- Checking each path at its tag without building it and parsing it again at
  compilation to intern its name tests was rejected: the second scan cost
  about 5–9% of compile time on `BenchmarkCompileIdentityPrograms` and kept
  the selector and field nodes' namespace context alive until compilation.
  Storing complete paths with namespace and local-name strings beside the
  interned paths was rejected because it allocated each path twice; the tag
  keeps only each branch's form and one slice of name tests per node, the
  compiler allocates the interned paths once, and no namespace context is kept
  for a selector or field (`schemaNodeNeedsDeferredQNameContext`).
- Interning the name tests into paths stored in the parsed document was
  rejected: the document is immutable and shared by chameleon views, and a
  second compiler of the same document would find the first compiler's name
  IDs there.
- Interning path names at the start tag was rejected. It gave them name IDs
  in document load order, ahead of every declaration, and the dense DFA index
  (`addDenseDFAIndexes`) admits a row only when the local-name IDs of its
  edges span a narrow range, so a content-model element named in a selector
  could cost its rows their dense index. It also put a `MaxSchemaNames` breach
  on one path's name ahead of the syntax and prefix errors of the paths after
  it.
- Checking a pattern at its facet's start tag and compiling it again when the
  restriction compiles was rejected: the tag compiled every occurrence of a
  text that the compiler cache would have compiled once, and compiled every
  distinct text before any `MaxSchemaPatternRetainedBytes` charge, so 200
  copies of one large pattern cost gigabytes instead of one compile. A
  syntax-only `xsdregex` check was rejected as well: the NFA state limit is
  raised while the program is built, and parsing is most of the compile cost.

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

- A synthetic interned name for each anonymous type (`$complexN`, `$simpleN`)
  was rejected: no lookup reads it, yet the published name table retained it,
  about 70 bytes per anonymous type, `MaxSchemaNames` counted it, and a
  diagnostic that formatted a type name printed it.

- Flattening nested groups during lowering was rejected: the declared groups
  are what Particle Valid (Restriction) clause 2.2 reads
  ([I-028](docs/spec/INTERPRETATIONS.md#i-028--content-model-checks)), and
  copying a shared group's particles into every referring group grows the model
  table exponentially in reference depth. A lowered model holds one particle
  per declared child, so a group referenced N times is stored once; only the
  analysis walks and the execution projection expand it, each charged to
  `MaxContentModelAnalysisSteps` per particle visited. Materializing the
  execution projection as a flat particle list was rejected for the same
  reason: one step per particle bought 32 bytes or more of a list that direct
  compilation and automaton construction can read in place, so a fan-out of
  references to a large group reached hundreds of MiB within the default
  analysis budget. For the same reason automaton construction does not store
  one composed follow entry per tail and first position it charges: a 56-byte
  entry per step let a choice of 256 elements doubled through ten group
  references reach 640 MiB of heap. Each tail instead records one segment
  naming the shared first set and its interned guard and action prefix
  (`followTable`), sibling entry sets are unioned by appending, and each
  distinct follow set is materialized once, into reused storage, when a state
  first reaches it. A queued state keeps only its key, a varint pair of
  position and prefix per entry, which its row decodes, so composed entries
  exist only while one row is built. Keeping each materialized set until its
  state's row was built was rejected: the start row of a sequence of optional
  elements reaches every position, so every follow set, at 56 bytes per entry,
  was live at once, 143 MiB for 1,700 positions. Direct compilation of a
  sequence gives each row a window of one shared edge run, since the rows an
  edge leaves are always a contiguous run ending at the newest row, instead of
  storing an edge per row and later element. Together these keep the measured
  heap of one model at about 4 bytes per step or less. Its dominant term in
  the shapes measured is row edges, bounded per step as the Go doc of
  `deterministicRow` derives (figures in `docs/RESOURCE_LIMITS.md`). Charging those entries more steps was
  rejected because it moves existing schemas toward or past
  `MaxContentModelAnalysisSteps`; a separate memory option was rejected because
  the analysis budget expresses the bound once each step admits bounded state.
  Building direct rows while deciding whether direct compilation applies was
  rejected: a sequence or choice that holds a group it cannot splice was
  charged `MaxSchemaDependencySteps` for rows it discarded, and spent work is
  never refunded. Collecting the walk's particles for the build was rejected
  as the flat list above; the build repeats the walk instead.
  Charging `len(Particles)` at `addModel` was rejected because the main path
  installs particles through `completeModel`, and shared variants would be
  charged for copies that do not happen. Cloning content models on read was
  rejected: it is uncharged work proportional to references times group size.
- Bounding the compiled tables retained across models with a schema-wide
  state cap was rejected because it does not bound edges or index entries and
  contradicts the per-model meaning of `MaxContentModelStates`; charging them
  to `MaxContentModelAnalysisSteps` by bytes was rejected because the largest
  corpus schema would keep only about 4× headroom against 68× for one
  dependency step per record; a new option was rejected as API that
  `MaxSchemaDependencySteps`, which already claims retained derived records,
  covers. Keeping a compiled table per model until publication and projecting
  it then was rejected: both forms were live at publication, about 2.5× the
  retained tables, and releasing compiled rows during publication still left
  the whole compiled table, larger than the published one in row-heavy shapes,
  live before publication starts. Projecting each model when it compiles
  makes the peak the retained tables plus one model's construction.
  Publishing a direct sequence with a copy of its edges and a name index per
  row was rejected: a sequence of n optional elements then retains about n²
  edges and index entries, so 800 types of 40 optional elements, which compile
  in under 170,000 steps with shared runs, exceeded the default
  `MaxSchemaDependencySteps`. Not charging those copies was rejected because
  the retained bytes would stay quadratic and unbounded. The run index resolves
  a name the run repeats by binary search over its positions, and a row whose
  window holds a name twice keeps linear dispatch, as a row with its own index
  does. Publication's per-row dense conversion skips run rows, which would
  need a dense table per row again; a run whose names allow it gets one dense
  table of run positions instead (`publishedDFARunDense`).
- Giving each {content type} particle its own root record was rejected. It
  would stop copying a group's record when a type's whole content is a
  repeated reference to that group. A full implementation kept verdicts,
  diagnostics and compiled tables identical and lowered steps on about 417
  corpus schemas, but it added 227 net production lines for a root table and
  its identity rules, still needed the clone of the base root and
  `chargedParticles`, and raised compile B/op 3.5% on the reachable
  benchmarks.
- Holding particle lists and continuation sets in an opaque generic list type
  in its own package, readable only through an iterator that charges one step
  per element or through a named uncharged escape, was rejected. A prototype
  kept compile time and B/op within noise and every corpus verdict,
  diagnostic and step count identical, for 102 net production lines. But 33
  functions still needed the escape because a callee charges what they read,
  so the guard's allowlist moved to the escape instead of going away. An
  uncharged loop added inside one of those functions still passed every test.
  The functions that iterate through the charge did so only because charges
  were moved out of three callees, which a new caller of those callees would
  silently skip.
- Skipping all validation below a zero-count compositor was rejected because
  the mapping removes only that compositor's component (see
  `compileDeferredModelChildren`): each child particle keeps its own mapping,
  decided by its own occurrence. A child `xs:element` with minOccurs and
  maxOccurs 0 is itself absent (`occurrence.IsAbsent`, Structures §3.3.2), so
  its own `ref`, `type` and value constraint are not resolved, authorized or
  checked. The parse tier still decides at its start tag and at every start
  tag inside it (the occurrence relation and the `xs:all` range, XPath,
  pattern, a repeated facet, size and `xs:whiteSpace` values, namespace
  declaration names), and the definitions nested in it keep their own
  mapping: its inline type definition, as for a prohibited attribute
  ([I-008](docs/spec/INTERPRETATIONS.md#i-008--prohibited-attribute-use-with-fixed)),
  and its identity constraints (§3.11.2). Instance reachability detaches the
  complex types and identity constraints among those definitions, the
  declarations nested in them and the children of a zero-count compositor,
  unless a live type also reaches them, as it does a declaration in a named
  group that a live type references; it drops the value definitions of the
  simple types among them and does not publish the element declarations (see
  `internal/schema`). Treating the whole subtree as absent was rejected:
  §3.11.2 maps every identity constraint regardless of its parent.
- Renumbering simple types densely over the reached ones, as element
  declarations are, was rejected: the simple-type ID is the value-program ID,
  named by the value program's own base, item and member references, the
  derivation records, the global type map and every value constraint, so
  every holder would need a copy. A per-type indirection in the value
  program's type lookup would free the 200-byte slot of an unreached type but
  adds a dependent load to every value evaluation. Only the inline simple type
  of a source item that maps to no component is freed, by truncation, because
  no component can hold its ID; a simple type nested in a detached complex
  type keeps its slot, since the complex type's attribute uses and text type
  hold it.
- Removing from the name table the names that only unreached element
  declarations use was rejected. A distinct name retains about 61 bytes plus
  its text: 79,000 element declarations, each under its own zero-count
  sequence, retain about 67 bytes each with distinct names of about six bytes,
  about 99 bytes with names 32 bytes longer, and nothing with one repeated
  name. But name IDs are held by declarations, identity-constraint paths,
  resolved QName values, diagnostics and the published name indexes, and a
  holder the removal missed would silently change a verdict instead of failing
  a check.
- Writing published element IDs while the compiled content tables are
  projected, before publication, was rejected: the name indexes built with
  them read build IDs against the declarations and the substitution table,
  the root tables of detached types name declarations that have no published
  ID, and publication's restriction checks read the substitution table in
  build IDs, so the build would hold two element ID spaces.
- Rewriting the element IDs of the build's content tables and substitution
  entries in place after the last step that can reject the build was
  rejected: its correctness rested on that ordering and on every particle
  being reached exactly once, so a table whose rows shared an edge array
  would be renumbered twice without any check failing.
- Copying every content table at publication was rejected: on a schema with
  no unreached declaration it raised the compilation peak and total
  allocation by about a fifth (a choice of 120 elements with `maxOccurs`
  16,000) to renumber nothing. Publication copies only a table that names a
  declaration whose published ID differs from its build ID, which every
  declaration below the first unreached one keeps.
- Marking detachment lexically, on the definitions nested under a zero-count
  element or compositor, was rejected: a local element declaration is
  compiled once per source node, so a named group referenced by both a
  detached and a live type shares its declarations, and marking them from the
  detached reference would stop evaluating keys the live type reaches.
- Declaring identity constraints as the components holding them compile,
  with a later pass for the rest, was rejected: types and named groups
  compiled first, so a keyref in them resolved its `refer` before a constraint
  on an element was declared and was rejected as unknown.
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
- Testing a group for pointlessness again after its children are reduced was
  rejected: the processor tie-break settles clause 2.2 on the declared group
  ([I-028](docs/spec/INTERPRETATIONS.md#i-028--content-model-checks)), and a
  re-test makes a declared two-member group a singleton. Only the choice
  clause 2.1 generates for a substitution group, which is not declared, is
  reduced by its member count (`substitutionChoice`). Deciding that a group
  left with no members is empty from its declared member count or its
  occurrence was rejected: a required choice of required empty choices
  matches nothing although it was declared with members, and an optional
  group of empty groups is not pointless as declared. `emptyGroup` decides
  from what the ignored members matched.
- Checking the aggregate count range and emptiability of the whole content,
  or the count range of each particle pair before the case table, was
  rejected: Particle Valid (Restriction) has no aggregate range rule, so those
  checks duplicated the per-case range rules and rejected restrictions the
  text admits, such as a choice of `a` and an empty sequence restricting a
  choice of `a` and `b`.
- Sending a derived {content type} particle that clause 2.2.1 makes pointless
  through the case table, as Xerces-J does, was rejected: accepted W3C mgE014
  expects such an empty sequence to restrict a sequence of one optional
  element, which the case table forbids once that sequence reduces to its
  element ([I-028](docs/spec/INTERPRETATIONS.md#i-028--content-model-checks)).
- Keying the ur-type exemption on a content model kind or wildcard identity was
  rejected: a type that inherits xs:anyType's content shares that identity
  without being xs:anyType, and clause 5.1 names the base type. Particle
  comparison has no ur-type concept.
- Discarding generated-choice provenance during flattening, or using global
  unordered flags with permutation search, was rejected: either reintroduces
  storage-order dependence or waives explicit sibling order (I-005).
- Refunding work after failed publication validation was rejected because the
  computation has already occurred; repeated failures would evade the aggregate
  work bound. Retryability preserves build data, not spent resources.
- Sharing UPA results across roots and eager DFA follow interning were
  rejected because each changes work accounting or limit-failure order. The
  analysis facts the checks read (emptiability, count ranges, continuation
  sets) are shared: each is computed once per model, charged to the root
  that first reads it.
- Materializing the continuation particles of each sequence member as a fresh
  list was rejected: a list per referring particle copies the lists of its
  nested groups, so shared emptiable groups nested N deep produce 2^N entries
  (277 MiB at N=16 on the doubling-groups schema). Continuation sets are
  memoized per model ID and deduplicated by particle, and the start particles
  of a repeated group are a shared prefix of its declared particles.
- Locating content-model diagnostics at the particle was rejected: particles
  carry no source node, and one per particle adds a pointer to every retained
  particle. The precision limit is that a violation inside a referenced group
  is located at the first model, in source-ID order, whose check reaches it;
  types compile before groups, so a referring type in another document than
  the group can be named
  (`TestUPAInsideReferencedGroupNamesTheLowestEnclosingModel`). The root of a
  group referenced as a type's whole content with a non-unit occurrence is a
  record of its own located at the reference (`applyModelGroupOccurrence`),
  so the location depends on the occurrence when no model with a lower ID
  reaches the group
  (`TestUPAInsideGroupIsLocatedAtTheFirstModelInSourceOrderThatReachesIt`). A
  root that wraps the reference as the one particle of a 1..1 sequence naming
  the group's own model was rejected: the group's model is then reached and
  compiled on its own as well, which raises analysis and dependency steps, and
  an all group cannot be a member of a model group such as that sequence.
- Locating a Unique Particle Attribution violation at the complex type, as
  Xerces-J and libxml2 do, was rejected: it would split content-model
  diagnostics between two location rules. The violation is located at the
  innermost compositor or component scope, and its message (`upaViolation`)
  names the complex type and both competing particles instead.
- Publishing unentered content models through a sparse table was rejected (a
  zero `CompiledModel` reads as Empty and a remap adds a lookup per read);
  skipping the compilation of a reached nested model was rejected because its
  errors and work charges belong to admission, and compiling it on its own
  stops a group fan-out at the first nested group over the DFA state cap.
  Compiling a model no complex type reaches was rejected: Unique Particle
  Attribution belongs to the content models of complex types ([I-050](docs/spec/INTERPRETATIONS.md#i-050--unique-particle-attribution-applies-to-the-content-models-of-complex-types)), as W3C
  `addB194` expects. A table indexed by source model with zero entries for the
  unreached models was rejected because a zero `compiledModel` reads as an
  empty model; `compiledContent` names the source of each compiled model.
- Raising the DFA state cap to admit avoidable expansion was rejected; safe
  single-particle occurrence normalization prevents that expansion.
- Per-row pointers for optional content dispatch were rejected because they
  enlarge every published row. Deferring all map cloning was rejected to keep
  the original projection lifecycle and small-schema construction path.
- Inferring empty complex content from an empty normalized child model was
  rejected because element-only particles can accept no children while still
  permitting whitespace.
- For the name index Element Declarations Consistent collects each content
  model with, a fresh map per model was rejected: on
  `CompileReachableContentModels/models_5000` it costs 235,952 allocs/op and
  33.75 MB/op, against 225,951 allocs/op and 34.12 MB/op when the map itself was
  the cached result. One index per nesting depth, emptied by deleting the names
  its model added, costs 215,965 allocs/op and 32.16 MB/op
  (`elementDeclarationConsistencyChecker.scratch`). A pool of cleared indexes
  measured the same 215,965 allocs/op and 32.16 MB/op and was rejected: `clear`
  costs a map its buckets rather than its entries, so the pool needed a cap on
  the size of a reused index, and a second cap on how many it kept, to stay
  bounded.

### Values

- Canonical-text comparison for typed fixed values was rejected: equivalent
  duration and timezone-normalized g* values have different lexical
  projections, and canonical spelling can select a different union member.
  A second equality API, lexical shortcut or identity-key cache would
  duplicate value-owned policy.
- Folding bound-literal admission into `Builder.Complete` was rejected:
  bound and enumeration literal errors are located at their facet element,
  while Complete's errors name no facet and are located at the simple type, so
  keeping that location would need a facet-identifying error and relocation in
  the schema compiler, and enumeration literals keep the same
  admit-then-complete shape. The cost is a second parse of each bound literal
  at compile time.
- Carrying the bound that `Builder.AdmitBoundLiteral` parsed to
  `Builder.Complete`, so each bound literal is parsed once, was rejected: the
  second parse allocates nothing, while a compiled bound carried in
  `BoundFacet` costs an allocation per bound and enlarges the `FacetSpec` of
  every simple type when held by pointer, and multiplies that size when held
  inline.
- Making the day a timezone moves an `xs:time` to part of its value was
  rejected: it contradicts the equality Xerces-J applies in enumerations,
  fixed values and identity constraints, where `00:30:00+14:00` is
  `10:30:00Z` (I-045 in `docs/spec/INTERPRETATIONS.md`).
- Keeping that day in `TimeValue` for the order while equality ignores it was
  rejected: order and equality then disagree, so a bound separates two equal
  values and a value can equal an enumerated literal that a bound of the same
  spelling rejects.
- Extending the un-wrapped `xs:time` order beyond the same-side restriction
  clauses was rejected: W3C `time_maxInclusive007_119` requires it only
  there, no W3C test requires it elsewhere, and Xerces-J, apart from its
  distinct `24:00:00`, orders every other relation by time of day.
- A distinct end-of-day midnight value for `24:00:00`, as in Xerces-J and
  libxml2, was rejected for three reasons. Datatypes §3.2.8.2 gives midnight
  the canonical representation `00:00:00`, and §2.3.1 makes canonical
  representations one-to-one with values. Structures applies a value
  constraint through its canonical representation, so a distinct value needs
  either a canonical text that §3.2.8.2 excludes or, as in Xerces-J, a
  `fixed="24:00:00"` that rejects `24:00:00`. XSD 1.1 Datatypes §3.3.8.2 and
  XPath Functions and Operators 3.1 `op:time-equal` state that both forms
  denote one value.
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
- A per-document memo of resolved schema names was rejected: a 64-entry
  two-way memo showed no significant gain on the element-start and
  content-dispatch benchmarks and cost 9% on
  `BenchmarkSessionElementStartWideVocabulary`, because every miss pays the
  hash, probe, and insert on top of the two schema map lookups it replaces.
- The reader's 8-entry ring in front of its spelling map is promoted, after 1024
  ring misses, to a two-way set-associative table of 32 sets indexed by a hash
  of the name bytes, and the ring entries move into it. A ring that inserts on a
  miss gets no hits on cyclic vocabularies wider than itself; the table gave
  5.8% on `BenchmarkLargeStreamingValidation/memory`, but each warm hit costs
  about 2 ns more than a ring hit. Allocating the full table from the start was
  rejected: a fresh session validating a one-row document was 2 to 4% slower on
  `BenchmarkIdentityDispatchFirstUse`, and every compile allocated 2 KiB more. A
  four-set hashed table that grows was rejected because its per-hit cost made
  small compiles 1 to 2% slower. Checking the last hit first was rejected: it
  slowed two-name rotations by 49%. Larger spelling caches are deferred: no
  qualified CPU result justifies their retained strings.
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
  dimension. Equal consecutive siblings share one path node through a slot on
  their parent's frame, not a document-wide index, so rollback clears it with
  the frame. Retained paths hold only a node reference; the store pointer they
  carried cost 8 bytes per ID, IDREF, and identity fact, and diagnostics
  receive the document explicitly. Together these cut the live heap at 99% of
  the 100k-row identity document from 55.0 to 44.8 MiB. Narrowing references
  to 32 bits was not needed for the RSS target and would require bounding
  `MaxIdentityEntries` at normalization rather than by a hidden cap.
- Matching `xmllint --stream` by skipping ID uniqueness and dangling-IDREF
  checks, and tuning `GOGC` or a memory limit in the CLI, were rejected as ways
  to close the libxml2 gap: the first weakens conformance, and the second hides
  validator memory behind runtime policy.
- A schema-wide switch that disables ID/IDREF evaluation when no declared
  type carries document identity was rejected: `xsi:type` can select `xs:ID`,
  `xs:IDREF`, or a type derived from them in any instance. The evaluator
  instead skips per value: with no identity constraint in scope and no
  selection pending, a value is recorded only when
  `value.Value.CarriesDocumentIdentity` reports an ID or IDREF fact. It gave
  2.5% on `BenchmarkLargeStreamingValidation/memory` and 1.8% on
  `BenchmarkLargeIdentityValidation`.
- A session-held check that reuses the previous name's namespace lookup was
  rejected: the namespace lookup is about 2% of streaming CPU, the check gave
  no significant change on `BenchmarkLargeStreamingValidation/memory`
  (n=24), and it cost 2.1% on a document that alternates namespaces.

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
  adapters, the `run.sh` interface, and the measurement command that writes
  `tests/oracles`. `tests/README.md` owns when and how its results enter
  diagnosis.
- `tests/README.md` documents corpus and harness operation. Counts and other
  cheap derived facts come from the inventory or test commands rather than
  manually maintained prose.
- This file is the sole architecture source of truth. Other documentation may
  link here but must not restate a competing package graph or lifecycle model.

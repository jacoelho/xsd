# Architecture improvement plan

> Historical record imported with baseline commit `2764e554`. Its completion
> statements concern the earlier architecture task.
> [ARCHITECTURE.md](ARCHITECTURE.md) owns the current architecture.

Status: P0–P3 implemented and verified, 2026-09-05. Three implementation adversaries
reviewed the result and re-reviewed both repairs; no material finding remains.
The charter, design comparisons and planning evidence below are historical.
Current completion evidence and IM1/IM2 amendments are in [implementation-ledger.md](implementation-ledger.md).

## Charter v1 and requirement lock

The requested outcome is to explore the codebase's architectural options, including
full rewrites and API changes where justified, write a detailed plan in `./plan.md`,
then iterate on the produced plan with three adversarial subagents.

| ID | Strength | Requirement and completion evidence |
| --- | --- | --- |
| R1 | Required | Explore all materially distinct options grounded in the codebase; record coverage, accepted and rejected designs, and reasons for closing each branch. |
| R2 | Required | Treat full rewrite and API-breaking designs as permitted alternatives, without assuming either is necessary. |
| R3 | Required | Produce an executable, detailed implementation plan in this file. |
| R4 | Required | After the initial complete plan exists, obtain three independent adversarial reviews, resolve findings, and obtain re-review of corrections. |
| R5 | Required | Preserve unrelated worktree changes and binding XML/XSD, resource, I/O, ownership, and diagnostic contracts. |
| R6 | Required | Keep `ARCHITECTURE.md` the current architecture authority; this file describes future work. |
| R7 | Required | Record verification evidence and distinguish planning completion from future implementation completion. |

Scope: the entire repository architecture, with deeper design work where an
observed seam exposes duplicated policy, competing representations, caller-owned
sequencing, or avoidable exported implementation detail. This task writes a plan;
it does not implement it, publish a GitHub issue, or alter the current architecture
contract. The user's requested local artifact supersedes the skill's issue-creation
workflow and its interactive candidate/interface selection stages.

No finite review can establish that no imaginable architecture exists. Exploration
closes only after every `ARCHITECTURE.md` navigation route has an evidence-backed
disposition, materially distinct design families have been compared, selected
designs have no decision-critical unknowns, and the three adversaries expose no
unresolved material alternative or execution gap. Renaming, file movement, and
equivalent wrappers do not count as new design families.

## Baseline dossier

Baseline revision: `72178fe6` plus the user's existing dirty worktree. The previous
review's findings are hypotheses until checked against that current state.

The baseline manifest covers 40,092 tracked and non-ignored untracked files,
excluding this plan. SHA-256 of the sorted path-to-content-hash manifest:
`b6a678cbde04787df7aa00a63f6d808d61820aa2b3cdb8c8c46e91e3dd246771`.
The task preserves existing changes in architecture/README, the web server,
compiler graph/context handling, formatter, stream, validation, their tests, and
the three untracked benchmark/document-test files. The initial status and hashes
are recorded in `/tmp/xsd-plan-baseline.json`; later steps must capture a fresh
snapshot if implementation starts from a different tree.

The compile-to-immutable-schema-to-document-session boundary is the calibration
design. Changing public APIs or replacing the implementation is permitted when it
produces a demonstrably better destination; preserving accidental internal shape
is not a constraint.

Initial candidate evidence:

- `internal/validate/start.go`: `RootInput`/`StartResult` expose a root-only
  assessment protocol. Its single production consumer in `session_runtime.go`
  converts `Skip`/`Recover` flags into `schemaStart`/`elementMode`.
- `rootTypeFromXSIType` resolves an undeclared root's type, and later
  `assessXSIType` resolves the same attribute again.
- `internal/runtime/schema_publish.go`: `PublishSchema` owns audit and successful
  build consumption, but several projection builders and validators remain
  exported without external production consumers. Their exact retention/deletion
  classification is recorded in the finite P2 inventory below and must be
  reconfirmed if the implementation baseline changes.

## Coverage and alternative closure

The dependency categories below follow the improve-codebase-architecture skill:
in-process computation and local-runnable boundaries. There are no required owned
remote services or third-party service integrations in the selected library paths.
Caller-supplied resolvers are explicit boundaries; they do not authorize library
network discovery.

| Route | Authoritative path and evidence | Alternatives examined | Disposition |
| --- | --- | --- | --- |
| Public compile, validation, source, options, sessions | `compile.go`, `session.go`, `source.go`; `public_api_test.go`, `tests/external_api_smoke_test.go` | API rename, merged options, expose internal types, replacement facade | Preserve small public facade unless a selected owner change needs an API break; names alone do not deepen it. |
| Diagnostics | `xsderrors/errors.go`; constructors, aggregates, location decoration, `xsderrors/errors_test.go` | Move diagnostics into root, mutable result envelopes, merge formatting/validation codes | Preserve: one catalog, immutable aggregate, one location decorator and public presentation projection already own the concern. |
| Source and URI handling | `Source.Acquire`, `Source.ResolveFrom`, `ReferenceBase`, `uriref.Reference`; cached-source resolver-context tests | Precompose bases once per parsed document; combine source identity with URI-reference spelling | Reject: cached schemas can have different resolver contexts, and opaque source identities are not raw URI references. |
| XML mechanics | `stream.Parser`; compile/validate/format consumers; `tests/stream_boundary_test.go` | Shared generic document cursor, tokenizer-owned depth, owned token copies | Reject: consumers own distinct topology/recovery; borrowed lifetime and lexical origin already have one owner. |
| Namespaces | `xmlns.Stack`/`Frame`; namespace transaction and churn tests | Merge with tokenizer, remove retained chain, duplicate authoritative lookup maps | Reject: retained contexts need immutable history; the active map is a bounded projection, not competing state. |
| Compiler graph and components | `CompileMappedSources` → loaded graph → plan → index → component compilation; `compilerSchemaBuild` | New phase interfaces, monolithic loader/compiler, opaque runtime builder | Compare opaque builder in broader design; phase-only wrappers add obligations around one existing orchestrator. |
| Published representation | `PublishSchema` → `newAuditedSchema` → projection audits → consume; publication corruption tests | Private helpers, aggregate projection owner, opaque builder, rewritten semantic kernel | Active candidate: close unnecessary projection construction/audit surface while preserving independent audit. |
| Datatypes, derivation, content models, substitution | `runtime` algorithms shared by compiler and validator; `derivation_test.go` compares mutable and indexed results across type pairs | Split by capability, move all algorithms to compiler, recompute at validation, compiled instruction VM, replace compiler callbacks | Preserve phase-specific mechanics and shared semantic owner; see the additional seam decisions below. |
| Element, XSI, recovery | `session.start` → start transaction → selection → assessment → frame/identity/attributes → commit | Local consolidation, concrete assessment owner, publication-precomputed start facts | Active candidate: one canonical start result and one semantic type-resolution path. |
| Identity and session lifecycle | Concrete identity evaluator, document-owned paths, guarded reusable session | Generic sink, combine identity with schema, atomic typed-value capture, separate path arena | Preserve: value paths differ in fixed-value/error ordering; the existing evaluator already owns target validity, rollback and retention. Add the missing integrated XSI identity case in P1. |
| Formatting | `format.XMLWithOptions` → shared token/namespace mechanics → bounded tree → checked writer | Share document topology, streaming rewrite, public formatting API | Preserve present owner; streaming would be a different formatting algorithm without demonstrated architectural need. |
| WASM, worker, page, HTTP, CLI | `cmd/wasmxsd/api.go`, worker/client JS, page epochs, `cmd/xsdweb`, `cmd/xmllint` | Unified adapter service, move cancellation into library, main-thread WASM | Reject: distinct execution contexts own lifecycle; worker termination is actual cancellation. Adapter tests already exercise boundaries. |
| Limits, allocation, verification and documentation | Owner options, allocation sites, `Makefile`, architecture/stream/build boundary tests, `ARCHITECTURE.md` | Global budget manager, structural-test removal, generated architecture hierarchy | Reject global ownership and blanket test deletion. Change only enforcement made obsolete by an accepted deletion and retain contract proof. |

Most existing boundaries already hide the required work. Broad changes must beat
that calibration on ownership and conceptual economy, not merely reduce file
count. All routes now have a disposition; selected changes are intentionally
concentrated where the evidence demonstrates an unnecessary seam.

## Preservation ledger

These observable invariants constrain every design. Existing private field names
and package/file layout are replaceable.

| ID | Invariant | Evidence / proof owner |
| --- | --- | --- |
| I1 | XSD 1.0, XML 1.0, UTF-8; XML 1.1 and unsupported external loading remain rejected. | `ARCHITECTURE.md`, stream/compile/corpus/public tests |
| I2 | Explicit repeatable sources; acquisition/resolution own I/O, identity, byte accounting and cleanup; resolver-context aliases remain distinct. | `internal/source`, schema source/context boundary tests |
| I3 | Synchronous, context-free library APIs; callers own unblocking arbitrary I/O. | Root APIs, import enforcement, CLI/worker lifecycle tests |
| I4 | Publish only after independent audit; failed publication preserves repairable build data, not spent work; success consumes build and owns immutable storage. | `schema_publish.go`, corruption/alias/retry tests |
| I5 | Fatal start rolls back XML, namespaces, hints, parent content, bit storage, identity and recovery state. Semantic stop retains syntax needed to detect later malformed XML. | `session_start_transaction_test.go`, `session_error_limit_test.go` |
| I6 | Root versus child, declared versus undeclared, wildcard skip/lax/recovery, absent versus explicit false, and lexical versus semantic XSI validity remain distinguishable. | Start, wildcard, XSI, identity and public validation tests |
| I7 | Diagnostic category/code, location/path, fatal/recoverable status, ordering and MaxErrors behavior remain unchanged unless a separate behavior defect is proven. | `start.go`, recovery owner, `xsderrors`, session boundary tests |
| I8 | Sessions own bounded scratch, reject overlap before reading, clear document references before releasing the guard; schema data never owns validation scratch. | Session construction/reset, public reuse/concurrency and allocation tests |
| I9 | Identity capture retains its lexical/value-space semantics; invalid semantic xsi:type does not automatically make a lexically valid QName field invalid. | `xsi_identity.go`, identity evaluator and XSI field tests |
| I10 | Input/work/retention budgets are charged at admission; no new unbounded caches, callback queues, goroutines or compensating work refunds. | Owner option/limit tests, work-budget tests, fuzz and opposing-shape benchmarks |

## Accepted design and preservation ledger

### Design families

These are alternative destinations, not cumulative layers. Signatures illustrate
the distinguishing seam; final names are subordinate to the ownership decision.

**A — Keep the current design (calibration).** Keep `RootStart`, its flag result,
the published read APIs, and private `newAuditedSchema`. This passes existing
behavior tests and introduces no migration. It leaves the verified root-only
transport representation, repeated semantic type lookup, exported publication
mechanics, and unused element-value table builders. The weakness is navigability
and unnecessary surface, not a claim that publication is currently bypassed.

**B — Deepen existing concrete owners.** `session.rootStartType(...) (schemaStart,
error)` directly performs root selection, and `session.assessElementStart(...)`
consumes its canonical state. Keep `PublishSchema(*SchemaBuild, ContentModelWork)
(*Schema, error)`; make construction and audit mechanics private. No package,
public API, registry, generic interface, or new long-lived state is required.
Caller example remains `engine.NewSession(options)` followed by
`session.Validate(reader)`. This removes the root helper protocol and obsolete
publication representations while keeping the already-complete transaction owner.

**C — Separate concrete assessment and opaque build owners.** An illustrative
`StartAssessor.Assess(StartInput) (StartDecision, error)` hides XSI checks; a
`runtime.Builder.Publish(work) (*Schema, error)` hides mutable table storage.
These are distinct proposals and can be evaluated separately. An assessor returning
one diagnostic is insufficient: one start can report several recoverable issues,
stop exactly at MaxErrors, or fail fatally before a later issue is assessed. A
corrected assessor needs direct access to session recovery or a resumable protocol.
It would relocate the same coupled transaction work and introduce another owner
without a second production consumer. Likewise, a naive Add/Replace/Bind builder
still requires callers to sequence correlated mutations; the stronger version
must own entire declaration registration/completion operations already owned by
the compiler's `schema_build.go` mutation boundary and private
`compilerSchemaBuild` storage. The current code confines topology writes to
`schema_build.go`; the premise that writes are scattered throughout the compiler
is false. Reject both additions for this plan: their repaired forms add concepts
without removing an unowned operation. Preserve the independent publication audit.

The measured replacement surface is 24 `*compiler` mutation/registration methods
writing through `c.rt.build`, plus 40 `compilerSchemaBuild` read/delegation/helper
methods. The mutation methods are not methods on the wrapper itself:

| Existing operation category | Exact operations an opaque owner must absorb or expose |
| --- | --- |
| Global/builtin registration (7) | `registerGlobalElement`, `registerGlobalAttribute`, `registerGlobalComplexType`, `registerGlobalSimpleType`, `registerGlobalIdentity`, `registerBuiltinSimpleType`, `registerBuiltinAnyType` |
| Component allocation/completion (7) | `addElement`, `completeElement`, `addComplexType`, `completeComplexType`, `addSimpleType`, `completeSimpleType`, `completeIdentity` |
| Auxiliary topology/model installation (7) | `appendWildcard`, `addAttributeUseSet`, `addModel`, `addModelAt`, `completeModel`, `installCompiledModels`, `installFinalizedElements` |
| Index/notation (2) | `indexGlobalAttribute`, `addNotation` |
| Publication (1) | `publishSchema` |

An opaque builder must either expose these coordinated operations plus necessary
semantic reads, or absorb the compiler policies deciding them. Merely replacing
21 direct field assignments with setters does not enforce registration/completion
invariants. The strongest builder absorbs full operations and hides storage; its
visibility benefit is real, but it relocates an existing single owner across the
compile/runtime dependency seam. No second producer or unsafe competing writer
justifies that destination here. P2 is a narrower representation/surface contraction.

**D — Publish aggregate element-start facts.** An illustrative
`Schema.ElementStart(id) (ElementStartRead, bool)` returns declaration and declared
type facts together; `GlobalElementStart(name)` serves roots. This can either join
existing immutable tables on demand or store type facts beside each element.
Runtime still cannot read instance attributes, decide recovery, or issue validation
diagnostics. The stored version increases schema memory for facts already present
in packed element/type tables; the on-demand version mainly packages existing
lookups. No measured bottleneck establishes a need for either. Reject additional
stored projections and per-session caches; use the existing sealed read methods.
An aggregate return may be reconsidered only with a demonstrated repeated caller
obligation that B cannot remove, not as an automatic follow-up optimization.

**E — Rewrite the schema kernel or the whole implementation.** The strongest
plausible destination merges compilation and the semantic kernel into one
`schema` capability: private construction IR, private audits, immutable exported
schema reads, and a separate document validator. It can delete cross-package
construction APIs and forwarding methods. It still needs incomplete component
graphs, finalized metadata, independently audited publication, packed runtime
reads, budgets, and distinct source contexts. Those representations have different
lifetimes; calling them one IR does not remove them. Merging compile and runtime
also combines schema XML parsing/source-graph orchestration with algorithms consumed
by instance validation. Splitting them again recreates the present seam.

The concrete replacement would expose `internal/schema.Compile(sources, limits)
(*Schema, error)` and keep `build`, `publish`, all projection constructors and
component compilation private in that package. Source acquisition remains in
`internal/source`; document scratch remains in `internal/validate`. Internally,
`compileComponents(graph, budget) (*build, error)` produces a mutable candidate;
`publish(*build, work) (*Schema, error)` audits it, consumes it only on success,
and transfers owned packed arrays to the immutable result. No build record escapes
the schema package. Failed source acquisition closes readers; failed compilation
discards local state; failed publication preserves the candidate for the owning
compiler's diagnostic/repair decision without refunding work.

This replacement deletes the cross-package `SchemaBuild` surface and the
`compilerSchemaBuild` forwarding boundary. That is a real visibility gain. Its
countervailing destination cost is that every schema compiler function now shares
package access to private sealed storage and publication internals. Retaining a
separate immutable-storage package restores an explicit mutation barrier but also
restores construction/transfer APIs. Keeping a private topology owner inside the
merged package preserves sequencing but does not eliminate that owner. Thus the
strongest rewrite trades an enforced package boundary for fewer exported internal
symbols; no currently unowned operation requires that trade. B retains the barrier
and removes the verified unnecessary exports and representation. Rejection does
not depend on migration size.

A public API replacement can be independently specified as
`Compile(options CompileOptions, sources ...SchemaSource) (*Schema, error)`,
`(*Schema).NewValidator(ValidateOptions) (*Validator, error)` and
`(*Validator).Validate(io.Reader) error`, with an optional allocating
`(*Schema).Validate(reader, options)` convenience. `Schema` owns immutable shared
metadata; `Validator` owns the same bounded reusable document scratch and overlap
guard currently owned by `Session`. Source descriptors retain identity, resolver
context, repeatable acquisition and close-error semantics. Options remain separate
because compilation and validation admit different resources. Diagnostics continue
through `xsderrors`, including immediate recovery limits and later XML fatal-error
precedence. The migration would replace all root examples, CLI/WASM consumers and
external API tests atomically, deleting `Engine`, `Session` and old overloads.
This gives names aligned with schema ownership and removes two default-options
entrypoints if desired, but introduces no new ownership or lifecycle guarantee:
current `Engine` is already immutable and `Session` already owns those resources.
Reject this independent API break as a naming/surface trade without a demonstrated
caller defect. It is permitted, not required, by R2.

An opaque IR plus table interpreter/VM is another rewrite variant. It adds an
instruction representation and audits while current compiled content models already
serve repeated validation. A streaming/event-pipeline validator requires ordered
rollback across namespace, content, identity, hints, diagnostics and paths; it must
either reconstruct the current transaction owner or introduce coordination. Neither
rewrite removes more semantic concepts than B. Reject on destination coherence and
unsupported extra machinery, not on change size or compatibility. Public API breaks
remain permitted but have no selected semantic purpose. Do not merge a replacement
alongside the existing engine or introduce migration wrappers.

**F — Concrete compiler phase contexts.** This is stronger than adding phase
interfaces. A `compileSession` would own limits, the one dependency-work budget,
the one content-work budget and `compilerSchemaBuild`. An indexing operation
consumes a closed `schemaPlan` and returns immutable raw indexes. A concrete
`componentCompiler` owns those indexes, component build state and cycle state,
borrowing the build and budgets. The existing `contentModelCompiler` owns model
depth, memoization, DFA construction and content analysis. Finalization drains
anonymous types, substitutions, restrictions, identity references and model/UPA
checks; a private final publication operation checks pending constraints and calls
`PublishSchema`. Transitions are loading → indexed → components → finalized →
published; source handles remain owned and closed by `schemaSetLoader`.

The current coordinator already partitions state into `compilerBuildState`,
`compilerCycleState`, `compilerIndexState` and `compilerModelState`, with a separate
loader and content compiler. The remaining components and models are recursively
coupled: compiling content resolves declarations, and compiling declarations
produces content. Separate concrete owners therefore need mutual references and
two-step initialization, or a combined recursive operation owner. The former adds
a lifecycle invariant and cyclic dependency; the latter largely reconstructs the
current coordinator. Interfaces do not remove this recursion. Moving maps alone
does not assign their policy to a deeper owner.

In the strongest safe variant, the coordinator continues to own that recursive
operation and passes the same budget pointers into its phases. Depth counters
unwind on return; consumed work never refunds. Any failure returns no schema and
discards private partial compilation state. Publication may preserve its candidate
for private repair, but no public partially built object escapes. This variant
improves phase-specific field visibility and test setup, yet removes no existing
semantic owner or verified caller sequencing defect. Reject the cyclic split;
retain the already concrete loader, model compiler and topology owner. Reject
further wrappers on destination economy, not aggregate line count. This closes
the phase-context alternative without claiming that future evidence could never
justify a different boundary.

### Additional seams found in the wider pass

**Simple-value callbacks.** `compiler.validateSimpleValue` caches five schema/error
callbacks and copies the bundle before assigning its sixth, per-call QName resolver. That
assignment is local, not shared mutable resolver state. The runtime uses one value
dispatcher with mutable-build and published metadata readers. The following options
were considered:

1. Keep the existing adapter. This preserves the compilation-owned completed-type
   facet cache and the runtime-owned dispatcher; QName context stays per operation.
2. Add `SchemaBuild.ValidateSimpleValue(id, lexical, resolve, needs)` with direct,
   uncached projection. It removes the bundle but repeatedly projects shared facet
   enumerations. That loses the allocation contract exercised by
   `TestSimpleValueFacetCacheSharedEnumerationAllocationsAreLinear` and the existing
   source-identity pooling tests. Do not trade a small callback adapter for that
   repeated work.
3. Add a runtime-owned `SimpleValueBuildReader{build *SchemaBuild, facetCache}`
   with `Validate(id, lexical, resolve, needs)`. Reuse the current facet projector
   and source-identity pooling; never retain `resolve`. To make this reader safe
   independently of compiler call order, route every type append/replacement
   through build mutations with per-type generations and cache generations. A
   changed generation reprojects the record. Discard the reader before publish.
   This removes callbacks but adds mutable generation/index state and moves cache
   lifecycle across the completion boundary. A weaker version borrowing only
   completed types avoids generations but retains the existing call-order contract
   while dividing completion and cache ownership. Neither improves that invariant.
4. The strongest cache-preserving alternative keeps the cache in compile and
   exports a runtime-consumed four-method `SimpleValueReader` contract:
   `ReadSimpleValueType`, `ReadSimpleValueFacets`, `ReadSimpleValueEnumeration`,
   and `ReadSimpleValueNotation`. A private compiler adapter implements these
   through the existing owners. A generic
   `ValidateSimpleValueWithReader[R SimpleValueReader](reader R, id, lexical,
   resolve, needs)` uses the existing dispatcher; QName stays an argument and
   unsupported-error classification becomes the common static classifier.
   Runtime's current lowercase reader methods are package-specific, so this
   requires exported methods/entrypoint and migration of both build and published
   readers, not just an exported alias of the private constraint.

   This variant deletes `compiler.simpleValues`, callback nil checks, the callback
   reader adapter and per-call bundle copying. It requires no new cache, generations
   or source copies. That is a legitimate small boundary simplification, and cache
   preservation is not a reason to reject it. It does not enforce completed-type
   validity: method results still carry metadata-validity booleans, and the same
   compiler ordering remains binding. A generic signature alone also proves no
   performance gain; allocation/dispatch comparisons would still be required.
   On the present evidence, retain the existing explicit adapter: there is one
   production assembly site and no demonstrated missing-callback or resolver-leak
   path. Replacing its transport with an exported four-capability contract removes
   optional-function plumbing but no duplicated semantic policy, retained
   representation or caller-owned lifecycle obligation. Unlike P1/P2, it does not
   resolve the identified architectural defect classes. This is a calibrated
   rejection of a modest cleanup, not a claim the interface design is invalid.

Decision: keep the adapter and its cache in this plan. This is an explicit
phase-boundary adapter with real metadata variation, not test-only indirection.
Current callers validate completed dependency types: restriction compilation
resolves its base before literals, and completes the derived type afterwards.
The completed-dependency precondition is verified from `compileRestriction`,
`compileLiteral`, `completeSimpleType`, and `derivedSimpleType`; no current violating
caller was found. No new callback framework, exported reader interface, or cached
build representation is selected. An incomplete-type caching test would invent a
new contract unless paired with an explicit type-completion redesign; do not add
such a change-detector test as a substitute for architecture work.

**Type derivation.** Retain the generic mutable-graph algorithm and the sealed
interval/index query, both owned by `runtime`. A universal walker loses the indexed
validation execution strategy; an index-only approach cannot evaluate incomplete
graphs without incremental indexing, invalidation and new lifecycle state. Building
another index late in compilation adds a second projection and still cannot replace
earlier queries. Existing all-pairs equivalence tests are useful independent proof,
not evidence that two graph lifetimes can be collapsed. Keep corruption and work
tests; do not remove the independent publication audit to eliminate repeated checks.

**Identity value application.** Keep the concrete evaluator's prepare/record/
capture/commit/reject operations. Declared attribute fixed checks occur after capture;
wildcard attributes and simple content check fixed values before capture. XSI identity
has different lexical rules and diagnostic ownership. A universal finalizer would
either change outcomes or add a policy strategy/callback protocol. Target generation,
single-outstanding-target admission, rollback, reset and budgets already have one
owner. No lifecycle defect justifies changing this model.

**Test-enforcement replacement.** Retain source-lifetime, import-direction, topology
ownership and public-boundary tests. Some assert concrete names because those are
the current enforced boundary. Update only names made obsolete by selected deletions;
do not remove guards wholesale to make a refactor pass. White-box corruption and
retention tests cover states unreachable through public validation and remain valid.

### Selection

B is the preferred destination for the two original seams. The root result and
publication helpers are separate packets; their implementations need not share a
new abstraction. D is the strongest performance-oriented counterproposal, and E
the strongest broad ownership counterproposal. Both require additional state or
coupling without eliminating the verified obligations B removes.

The strongest objection to B is that unexporting functions alone changes visibility,
not module depth. Therefore deleting the unused element-value table path and moving
its meaningful tests to actual published reads are required, not optional cleanup.
The start change also must eliminate the result conversion and duplicate semantic
resolution; merely renaming `RootStart` does not satisfy the plan.

### Selected start-state contract

Keep `schemaStart` as the only declaration/type/mode/assessment result. Root and
child selection can have different private functions because their policies differ;
both return that result into the existing transaction. Do not unify their observable
error precedence merely to make one helper easier to write.

Root branch order is binding: after hint syntax has been staged, first use a
matching global declaration. Otherwise, when xsi:type is present, resolve it now
and either select that type or return its fatal selection error. Only when no
xsi:type exists does a matching schema-location hint produce unsupported-loading;
without either, report the recoverable undeclared-root diagnostic. A valid
undeclared-root xsi:type therefore takes precedence over a root-namespace hint.
Malformed hint syntax is diagnosed before any root branch; recovery may continue
into selection when MaxErrors permits. Declared roots do not pre-resolve xsi:type
here.

An undeclared root must resolve xsi:type during root selection. Represent that
fact with a private, operation-local type-origin tag set only by the corresponding
constructor. The selected TypeID remains in `schemaStart.typ`; do not store a second
cached TypeID, lexical string, or error. Common assessment consumes that origin and
uses the selected type rather than resolving again. No origin tag enters `frame`,
the published schema, identity state, or the next token. Ordinary declared roots
and children retain their existing nil-before-type assessment sequence.

The resulting path remains:

```text
prepare XML/namespaces
  -> stage hints
  -> select root/child and stage content transition
  -> assess nil/type/effective state in existing diagnostic order
  -> create frame and commit XML
  -> start identity and validate attributes
  -> validate and commit parent-content/identity transaction
```

Fatal errors retain rollback; semantic stop retains only syntax continuation. XSI
identity conversion remains separate from semantic type availability/derivation.
The goal is one semantic type resolution per assessed start, not a blanket ban on
lexical QName conversion needed to produce identity values.

### Selected publication contract

`PublishSchema` remains the sole build-to-schema transition. It owns primary and
projection audits, independent comparison against build records, consumed work,
candidate storage, successful build consumption and alias isolation. Keep semantic
read types and the `Schema` methods used by validation; close only construction and
source-to-projection mechanics lacking real outside-runtime production consumers.

Delete the unused slice-based element-value lookup path. Retain the canonical
`elementReadTable.valueConstraints` path, `ElementValueConstraints`, and the private
construction/comparison helpers its packed-table audit still requires. In particular,
`NewElementValueConstraints` has production packed-table callers: privatize it,
do not delete its behavior. `ElementValueConstraintReadShape` also supports the
packed-table audit and is not automatically dead with the slice builders.

## Execution plan

The checked tasks below record implementation against accepted plan v2, with
IM1/IM2 documented in the implementation ledger. The original unchecked plan is
preserved in the implementation baseline snapshot. Use one writer, preserve the
user's baseline changes, and keep each packet dependency-complete. P1 and P2 are independent
conceptually; execute P1 first so semantic risk is isolated from mechanical API
contraction. A failing packet is repaired before starting the next one.

### P0 — Lock the implementation baseline and test oracles

- [x] Record HEAD, dirty diff, untracked inputs, toolchain and environment; verify
  that the reviewed worktree assumptions still hold. Do not reset the worktree or
  construct a comparison baseline from HEAD alone: it would omit user changes.
- [x] Use `go_workspace`, semantic references and the architecture router to confirm
  the complete `RootStart` consumer set and projection helper classifications.
  Include tests and build-tagged WASM sources, not just default production files.
- [x] Run the existing focused start/publication tests. Capture current error tuples
  `(category, code, path, line, column, cause)` and ordering for the P1 matrix before
  deleting helper tests. Use explicit expected outcomes, not a second copy of the
  production algorithm or a golden dump of private structs.
- [x] Capture repeated-document/XSI compilation and validation benchmarks at this
  exact baseline. Retain raw benchmark output and environment with its content hash.

Contract/owner: R5/R7 and I1–I10; implementation owner records revision evidence,
while tests remain owned by their capability. No architecture or production changes
occur here. Bounds: commands are finite; benchmark repetitions are explicitly set.
Proof: passing focused baseline, semantic reference inventory, raw measurement files.
Writeback: revision evidence in a ledger; do not copy current architecture into this
plan as a new authority. Rollback: none, since no production mutation is performed.

### P1 — Contract the element-start assessment path

- [x] Replace the body of `session.rootStartType` with direct global-root lookup,
  undeclared-root xsi:type selection, schema-hint handling and root diagnostic
  recovery. Return `schemaStart` directly; root and child selection remain within
  the existing transaction and retain their different policies.
- [x] Remove `RootInput`, `StartResult`, `RootStart`, `rootTypeFromXSIType`, and
  the root result flag-conversion branch. Do not leave aliases, wrappers or a second
  implementation for tests.
- [x] Add the operation-local type-origin discriminator to the canonical selected
  state. Its only non-default constructor is successful undeclared-root XSI
  selection. Common assessment consumes the existing TypeID for that case and
  still performs required metadata/effective-state checks. All other paths resolve
  the semantic override at their current position in the assessment sequence.
- [x] Keep raw xsi:nil/xsi:type extraction while the token is live; preserve
  absent versus explicit false. Do not eagerly resolve all XSI types before nil
  assessment or cache borrowed token data beyond the current start.
- [x] Preserve `sessionStartTransaction`, `acceptedChild` content-transition staging,
  `elementMode`, parent invalidation, start rollback, and semantic-stop behavior.
  Do not infer wildcard skip from missing declaration or infer recovery from TypeID.
- [x] Remove the two direct RootStart tests' flag assertions after replacing their
  behavioral coverage at `Validate`/`Session.Validate`. Keep or strengthen focused
  transaction and retention tests where public calls cannot expose rollback details.
- [x] Run the P1 matrix, allocation tests and baseline comparisons; update the
  element-start flow and rejection rationale in `ARCHITECTURE.md` in the same packet.

Contract/owner: R1/R3, decision B, I5–I10; `internal/validate` owns the selected
state, XSI policy, recovery and transaction. Runtime remains a provider of sealed
facts and derivation queries. Path: namespace preparation → staged hints → selection
→ existing nil/type checks → XML commit → identity/attributes → semantic commit.
Failure: root XSI selection errors remain fatal at that point; recoverable errors
call `s.recover` immediately; no delayed diagnostic batch may overrun MaxErrors.
Bounds: a fixed-size transient discriminator, no new per-element retained table,
no persistent cache and no extra token/attribute copies. Proof: boundary diagnostic
matrix, rollback/reset/overlap tests, unchanged fast-path allocations. Rollback:
revert the entire task-owned P1 packet, including its tests/documentation, to the P0
working-tree snapshot; never selectively restore only one side of the state model.

### P2 — Close publication construction and delete unused element-value tables

- [x] Classify every exported projection constructor, shape, comparer and audit
  touched by publication as: required cross-package semantic API; runtime-private
  production helper; meaningful test fixture; or obsolete representation. Record
  actual semantic consumers. Do not apply a blanket name-prefix rename.
- [x] Make runtime-private production helpers unexported using semantic rename.
  Keep `PublishSchema`, required read types, and `Schema` methods. Keep shared
  mutable-build semantic algorithms genuinely consumed by compilation.
- [x] Delete the unused element-value slice constructors, lookup and table-wide
  equality/audit helpers listed below. Migrate their valuable assertions to
  `Schema.ElementValueConstraints` and the packed `elementReadTable` audit.
- [x] Preserve the scalar `ElementValueConstraints` representation and its private
  constructor, comparer and declaration-shape projection needed by packed-table
  production code. Do not delete the canonical audit's reference computation.
- [x] Keep `newAuditedSchema`'s independent source-record and projection checks.
  Failed audits leave build contents repairable and work charged; successful audit
  alone clears the build. No lazy publication, skipping audit, or mutating shared
  schema tables during validation is permitted.
- [x] Replace tests that create a parallel slice table and compare it with the
  helper that produced it. Preserve explicit expected semantics, corruption,
  aliasing, retry and budget coverage through the actual publication/read path.
- [x] Remove the test-only element-start construction protocol and all four read
  facades listed in D9/D10. Migrate the two used simple-value facade calls to the
  existing Schema method first; preserve the separate identity-state fixture seam
  explicitly retained in the inventory.
- [x] Run runtime/compiler/publication/validation seam tests and publication
  benchmarks; update the publication contract in `ARCHITECTURE.md` in the same packet.

Contract/owner: R1/R3, decision B, I4/I8/I10; `internal/runtime` owns publication
construction and audit. Compile owns its mutable build and calls the existing
publication boundary. Path: build → candidate projections → independent audit →
successful consume/seal. Bounds: no new tables, cloned metadata, caches, graph walks,
or work-accounting changes; removal should only contract the surface. Proof: semantic
reference inventory, packed-table tests, independent corruption/alias/retry tests,
and compile→publish→validate behavior. Rollback: revert the complete P2 packet while
retaining a verified P1; no visibility shim or alternative table survives.

### P3 — Verify contraction across consumers and documentation

- [x] Re-run semantic references after P1/P2. Confirm the deleted root protocol and
  element-value slice table are absent from production and tests; no replacement
  `export_test.go` facade recreates them.
- [x] Verify root APIs, CLI, WASM, worker response tags and instance diagnostics
  still use their canonical paths. No selected change requires a public API break;
  do not add one simply because permission exists.
- [x] Preserve architecture enforcement of input lifetime, dependency direction,
  source I/O ownership and compiler topology. Update obsolete symbol expectations
  only when necessary, with equivalent enforcement of the substantive invariant.
- [x] Read `ARCHITECTURE.md`, README, public examples and affected tests together.
  Architecture writeback must reflect completed packets; README should change only
  if observable usage changed. Remove superseded plan text from current architecture
  documentation and retain this file as historical future-work evidence until done.
- [x] Run all required gates and inspect results. Review the final diff for every
  addition/removal, unexpected scope growth, test weakening and user-change loss.
- [x] Obtain a final implementation review of plan-to-code, code-to-plan and complete
  affected execution paths. This future review is separate from the three reviews
  of this planning deliverable.

Contract/owner: R3/R5–R7, all invariants; root implementer coordinates capability
owners without becoming a new runtime owner. Proof: full gates, reference/deletion
inventory, architecture and public documentation agreement, no incomplete migration.
Rollback: to the last complete verified packet using task-owned changes only.

### Exact deletion and migration ledger

| ID | Current symbol/path | Final disposition | Packet and proof |
| --- | --- | --- | --- |
| D1 | `RootInput`, `StartResult`, `RootStart` | Delete; move policy directly to session root selection. | P1; references currently include one production caller and two direct tests. |
| D2 | `rootTypeFromXSIType` and the `Skip`/`Recover` conversion | Delete; preserve one semantic XSI resolution and canonical selected state. | P1; start path and diagnostic matrix. |
| D3 | `NewNameReadView` and runtime-private projection construction/audit exports | Unexport where reference inventory confirms no external production consumer; preserve behavior. | P2; `NewNameReadView` references are publication and its runtime test. |
| D4 | `NewElementValueConstraintReads`, `NewElementValueConstraintReadsForDecls` | Delete unused alternative slice-table construction. | P2; the latter has only its definition and a runtime-test call. |
| D5 | `EqualElementValueConstraintReadProjection`, `EqualElementValueConstraintReadProjectionForDecls`, `ValidateElementValueConstraintReadProjectionForDecls` | Delete with the slice-table representation; preserve packed-table audit. | P2; migrate mismatch/corruption proof. |
| D6 | `ElementValueConstraintsByID` | Delete alternative slice lookup; use the published schema accessor. | P2; existing consumers are helper tests only. |
| D7 | `NewElementValueConstraints`, `EqualElementValueConstraints`, declaration-shape helper/type | Keep required private scalar behavior; unexport if consumer inventory permits. | P2; `element_read.go` uses constructor and equality in production and audit. |
| D8 | Direct RootStart flag tests and unused table-helper tests | Replace behavior coverage before deletion; keep independent pure and corruption tests. | P1/P2; no test-count or coverage-percentage target substitutes for contract proof. |
| D9 | `SimpleContentTypeForTest`, `ElementValueConstraintsForTest`, `ComplexAttributeUsesForTest`; `ValidateSimpleValueRuntimeBoundaryForTest` | Delete the first three unused facades. Replace the fourth's two datatype-test calls with `Schema.ValidateSimpleValue`, then delete it. | P2; preserve explicit decimal `5.0` versus integer `5` canonical-value expectations. |
| D10 | `NewElementStartInfo`, `EqualElementStartInfo`, `ElementStartInfoShape` | Delete unused constructor/comparer/shape after moving meaningful assertions to published `Schema.Element`/`RootElement` reads. | P2; preserve `ElementStartInfo` and packed start metadata behavior. |

The P2 inventory also covers attribute, identity, wildcard, name, notation and
derivation projection builders/auditors reached from `newSchemaRuntime` and
`validateRuntimeReadProjections`. Names are not sufficient evidence of deadness:
build-time semantic projections can have legitimate compiler consumers.

### P2 finite projection inventory

The semantic-reference sweep below covers exported construction/audit symbols
reached by `schema_publish.go:newSchemaRuntime` and
`schema_validate.go:validateRuntimeReadProjections`, their value/element helpers,
and associated read types. `RP/RT/OP/OT` means references from runtime production,
runtime tests, outside-runtime production, and outside-runtime tests. Grouped
counts follow symbol order. These are baseline evidence, not future assertions
about line numbers or reference totals. “Private” means an unexported semantic
rename; it does not change behavior. Name the private element shape type
`elementValueConstraintShape` because `elementValueConstraintReadShape` already
names its projection function. Retain method-based semantic read APIs.

| Symbols | Consumers / counts | Decision |
| --- | --- | --- |
| `NewNameReadView`; `ValidateNameReadProjection` | Publication/audit and runtime name tests; `1/1/0/0`; `1/2/0/0` | Private. |
| `NameReadView` | Seven runtime-only references | Private; no exported schema method exposes this type. |
| `NewNotationReadMap`; `EqualNotationReadMap`; `ValidateNotationReadMap` | Publication/audit and runtime notation tests; `1/3/0/0`; `1/6/0/0`; `1/2/0/0` | Private. |
| `NewAttributeUseReadForSimpleTypes`; `EqualAttributeUseSetReadProjectionForSetsWithSimpleTypes`; `ValidateAttributeUseSetReadProjectionForSetsWithSimpleTypes` | Runtime construction/audit/tests; each `1/2/0/0` | Private. |
| `AttributeUseReadShape` | `5/14/0/0` | Private. |
| `NewAttributeDeclRead`; `NewAttributeDeclReadForDecl`; `NewAttributeDeclReadsForDecls` | Runtime projection construction/tests; `1/7/0/0`; `2/0/0/0`; `1/3/0/0` | Private. |
| `AttributeDeclReadByID`; `EqualAttributeDeclReads`; `EqualAttributeDeclReadProjectionForDecls`; `ValidateAttributeDeclReadProjectionForDecls` | Runtime reads/audit/tests; `1/2/0/0`; `1/1/0/0`; `1/4/0/0`; `1/3/0/0` | Private. |
| `AttributeDeclReadShape` | `3/7/0/0` | Private. |
| `NewWildcardView`; `NewWildcardViews`; `EqualWildcardViews`; `EqualWildcardViewProjection`; `EqualWildcardViewProjectionTable`; `ValidateWildcardViewProjectionTable`; `WildcardViewByID` | Runtime construction/reads/audit/tests; RP/RT respectively `2/2`, `1/2`, `1/2`, `1/2`, `1/3`, `1/3`, `1/2`; all OP/OT zero | Private. |
| `NewSimpleTypeDerivationForSimpleType`; `NewComplexTypeDerivationForComplexType` | Runtime projection construction/tests; `1/2/0/0`; `2/2/0/0` | Private. |
| `EqualSimpleTypeDerivationForSimpleType`; `EqualComplexTypeDerivations`; `EqualComplexTypeDerivationForComplexType`; `ValidateTypeDerivationReadProjection` | Runtime equality/audit/tests; RP/RT `0/2`, `1/1`, `0/4`, `1/7`; all OP/OT zero | Private; retain independent audit behavior. |
| `TypeDerivationRead` | `21/8/0/0` | Private packed/indexed implementation type. |
| `NewElementStartInfo`; `EqualElementStartInfo`; `ElementStartInfoShape` | Two constructor and one comparer calls, all in `TestElementStartInfoProjection`; shape used only there and by constructor | Delete test-only construction protocol, retain published start-value type. |
| `NewTypeInfo`; `TypeInfoShape` | Production `complexTypeRead.typeInfo` and `Schema.TypeInfo`, plus runtime projection tests; five/six references | Private constructor/shape; retain `TypeInfo`. |
| `NewSimpleContentTypeRead`; `SimpleContentTypeReadShape`; `SimpleContentTypeRead` | Runtime `complexTypeRead.simpleContent` and focused tests only; five/six references for constructor/shape | Private all three; retain tuple-returning `Schema.SimpleContentType`. |
| `EqualIdentityConstraintReadProjection`; `EqualIdentityConstraintRead`; `ValidateIdentityConstraintReadProjection`; `IdentityConstraintReadByID` | Runtime audit/lookup/tests only; first three RP/RT `1/3`, `1/0`, `1/3` | Private. |
| `NewValueConstraintRead`; `NewValueConstraintReadFromConstraint`; `EqualValueConstraintReads` | Runtime scalar construction/audit/tests; `1/17/0/0`; `9/2/0/0`; `5/0/0/0` | Private. |
| `NewElementValueConstraints`; `ElementValueConstraintReadShape`; `EqualElementValueConstraints` | Packed-table construction/reference audit plus obsolete table/tests; `8/8/0/0`; `4/1/0/0`; `3/1/0/0` | Keep private scalar behavior and shape; no alternative audit rewrite. |
| `NewElementValueConstraintReads`; `NewElementValueConstraintReadsForDecls`; `EqualElementValueConstraintReadProjection`; `ValidateElementValueConstraintReadProjectionForDecls`; `ElementValueConstraintsByID` | No live production path; RP/RT `0/2`, `0/1`, `0/3`, `0/3`, `0/3`; OP/OT zero | Delete obsolete slice path and migrate meaningful tests. |
| `EqualElementValueConstraintReadProjectionForDecls` | `1/2/0/0`; its one non-test reference is the obsolete slice validator in `value_constraint.go` | Delete atomically with that validator. The canonical packed audit already exists; do not redirect or duplicate it. |
| `SimpleTypeDerivation`; `ComplexTypeDerivation` | Real compiler consumers in `schema_build.go`, plus outside tests | Retain exported build-phase semantic values. |
| `AttributeUseSetRead`; `AttributeUseRead`; `AttributeDeclRead` | `20/3/11/0`; `18/4/4/0`; `17/10/2/0` | Retain exported semantic read values. |
| `IdentityConstraintRead`; `WildcardView`; `ElementStartInfo`; `TypeInfo` | Identity has two outside production consumers; TypeInfo has ten; wildcard/element values are exposed by Schema semantic accessors | Retain exported read result types even when consumers use inferred types. |
| `IdentityConstraintIDs`, `IdentityPathRead`, `IdentityPathReads`, `IdentityFieldPathRead`, `CompiledIdentityFieldRead`, `CompiledIdentityFieldReads` | Nested semantic results returned by Schema/identity-read methods; validation consumes IDs and field paths | Retain exported. Identity read constructors are already private. |
| `ElementIdentityConstraintIDs` | Three external fixture sites in `internal/validate/identity_test.go`, including its scope helper; no production publication caller | Retain this isolated fixture seam unchanged. It constructs a borrowed ID view for state/limit tests; replacing it with schema compilation would add an unrelated prerequisite to those tests. It is not a second retained publication table or an audit path. |
| `ValueConstraintRead`; `ElementValueConstraints` | `32/6/4/0`; `24/11/8/0`; validation consumes both | Retain exported semantic reads and `Schema.ElementValueConstraints`. |
| `NewValueConstraintIdentity` | `internal/compile/compile_attributes.go` uses it for semantic fixed-value comparison | Retain exported; construction naming does not make it publication-only. |
| `ValueConstraintValidation`, value-constraint simple/complex metadata contracts and semantic validators | Shared compile/runtime validation operations, outside the removed projection path | Retain unchanged; do not broaden P2 into a semantic API redesign. |
| `SimpleContentTypeForTest`; `ElementValueConstraintsForTest`; `ComplexAttributeUsesForTest` | Each has zero semantic references | Delete. |
| `ValidateSimpleValueRuntimeBoundaryForTest` | Two calls in runtime external `TestDecimalAndIntegerCanonicalValuesDiverge` | Migrate those calls to the existing Schema method; delete facade. |

The finite list closes selection; P0 reconfirms it against baseline drift and
build tags. A newly discovered external consumer is evaluated at its actual owner,
not accommodated by a compatibility export. Remove unused test facades rather than
preserving the old projection surface under `ForTest` names.

### Predicted change surface

| File/symbol | Change | Role | Confidence |
| --- | --- | --- | --- |
| `internal/validate/start.go` | Delete root-only exported protocol; retain reusable semantic predicates. | P1 canonical assessment state | High |
| `internal/validate/session_runtime.go` | Direct root policy, transient type origin, reuse selected type. | P1 transaction integration | High |
| `internal/validate/session_model.go`, `session_namespaces.go` | Adjust private call sites only if affected. | Child selection and QName dependency | Medium; no owner change |
| `internal/validate/start_test.go`, session transaction/error-limit tests, root `xsd_test.go` | Replace shallow assertions and exercise boundary cases. | P1 proof | High |
| `internal/runtime/value_constraint.go` / `_test.go` | Remove unused slice APIs; retain scalar/packed behavior. | P2 contraction | High |
| `internal/runtime/element_read.go`, schema publication/read/corruption tests | Migrate helper names and retain independent audits. | P2 canonical representation proof | High |
| `internal/runtime/schema_publish.go`, `schema_validate.go`, projection owner files | Semantic renames and private helper calls. | P2 publication-only mechanics | High; finite inventory above |
| `internal/runtime/start_projection_test.go`, `export_test.go`, `datatype_external_test.go` | Delete unused start construction/facades and migrate the used facade's two calls. | P2 read-boundary proof | High |
| `internal/compile/schema_publication_corruption_external_test.go` and other compile seam tests | Preserve public runtime publication boundary tests; migrate only actual removed callers. | Cross-package publication proof | Medium |
| `ARCHITECTURE.md` | Update selected state/projection ownership after implementation. | Current architecture writeback | High |
| README/root APIs/adapters | Usually no edit; verify behavior and usage. | External contract | High confidence in preservation |

No new package, service, dependency, persistent format, global registry, public
option, worker, goroutine, runtime cache, compatibility layer or alternative
execution path belongs in the selected destination.

## Verification and completion audit

Required repository gates: `make test`, `make race`, `make wasm-test`,
`make web-test`, `make browser-test`, `make fuzz-smoke`, `make bench-smoke`,
`make staticcheck`, `make lint`, and `git diff --check`. Future changed hot paths
also require focused allocation and benchmark comparisons. Format changed Go
files with `gofmt` during implementation.

### P1 behavioral matrix

Use small explicit XSD/XML fixtures and assert observable error tuples and order.
Cross the cases below with both one-shot and reused sessions where failure/reset
matters. Root and child outcomes need not be identical; preserving the distinction
is part of the oracle.

| Family | Required opposing cases and invariant |
| --- | --- |
| Declaration selection | Declared root/child; undeclared root without XSI; undeclared root with valid XSI; strict missing child; lax missing child with/without XSI; skipped wildcard; recovery descendant. |
| XSI type | Valid same/derived type; unknown name; unresolved prefix; malformed QName; unavailable simple/complex/simple-content type; abstract type; blocked restriction/extension; illegal substitution relationship. |
| Nil independence | Absent, false, zero, true, one, invalid spelling; nillable/non-nillable; fixed declaration; nilled node with child/text; each crossed with valid/invalid XSI type and both attribute orders. |
| Diagnostic precedence | Undeclared-root XSI failure before nil assessment; declared starts' nil-before-type order; abstract/unavailable declaration precedence; malformed hints before start policy; matching unsupported hint versus ordinary missing declaration. |
| Recovery bounds | MaxErrors=1 and >1; trigger before and after XML commit; later malformed end tag, truncated EOF, outside-root character references/CDATA; no further semantic identity allocation after stop. |
| Identity | XSI diagnostics occur once; QName lexical validity is distinct from type existence/derivation; selected invalid/nilled fields, default/fixed values and key/keyref completion preserve current scope invalidation rules. |
| Integrated XSI identity | In the explicit fixture below, two bound but unknown or non-derived xsi:type QNames produce `validation.type`, `validation.type`, `validation.identity`, in that order. Malformed or unbound QNames produce only the two type diagnostics. Undeclared-root type failure exits before identity starts. |
| Transaction/lifetime | Fatal errors before/after XML commit roll back namespace bindings, hints, parent content, xs:all bits, identity targets and retained paths. Follow with valid input on the same session. Borrowed attributes cannot survive parser advance. |
| Concurrency | Independent sessions share one engine safely; copied session handles share the overlap guard; overlapping validation rejects before touching the second reader; cleanup precedes guard release. |

Retain primitive QName/nil predicate tests when they express an independent lexical
contract. A counter of private resolver calls is structural/performance evidence,
not the sole correctness test. The syntax-only checker and formatting consumers
remain outside the selected start-assessment change.

The integrated fixture declares `root` with repeated `row` of `xs:string` and an
`xs:unique` selecting `row`, with field `@xsi:type`. Bind `xs` to the schema
namespace, `xsi` to the instance namespace and `p` to `urn:missing`; do not bind
`u`. With `MaxErrors: 10`, validate two empty rows using respectively
`p:Missing`, `xs:int`, `p::Missing`, or `u:Missing` in both rows. The first two
cases have a valid lexical QName field despite a recoverable semantic type error,
so uniqueness still reports the duplicate. The latter two cannot supply a valid
QName field. Preserve `start.invalid` and the existing attribute-capture order;
element-end invalidation must not be broadened to erase those already captured
attribute fields. Put this regression at the public validation boundary in
`xsd_test.go`, alongside the existing XSI/identity cases, and repeat through a
reused session followed by valid input.

This oracle was executed against the planning baseline using the temporary public
API probe `/tmp/xsd-plan-xsi-probe.go`. For its one-line fixture, the unknown-type
case reports `(validation.type, /root, 1:127)`, `(validation.type, /root, 1:154)`,
then `(validation.identity, /root/row, 1:180)`; the non-derived case reports the
same codes/paths at `1:127`, `1:151`, `1:174`. Malformed/unbound cases report only
type errors. An undeclared root with `p:Missing` reports one type error at
`/, 1:1`. Future tests should use readable fixed fixtures with explicitly computed
locations, not these columns copied onto differently formatted XML. Also test
`MaxErrors: 1`: the first recoverable diagnostic stops further semantic capture,
while later malformed XML retains fatal precedence.

### P2 publication matrix

| Proof owner and boundary | Required explicit assertion |
| --- | --- |
| `internal/runtime/schema_publication_test.go`, new `TestPublishedElementValueConstraints` through `PublishSchema` then `Schema.ElementValueConstraints` | A valid declaration without a constraint returns its owner type, `present=true`, `valid=true`, and absent fixed/default values. A fixed declaration returns only fixed; a default declaration returns only default. Assert literal lexical/canonical text and typed value against fixture constants, never against the removed constructor. |
| Same published accessor fixture | Interleave unconstrained, fixed, unconstrained, default, and repeated shared-value declarations so declaration IDs differ from packed constraint offsets. Each declaration returns its own owner and expected value. Cover simple, complex simple-content and allowed emptiable mixed-content owners; mixed values retain the untyped lexical contract. |
| Same published accessor fixture | `NoElement` returns zero constraints, `present=false`, `valid=true`; out-of-range IDs return zero constraints, `false`, `false`. A real unconstrained declaration is distinct from both. |
| `internal/runtime/element_read_test.go`, `TestElementReadTableOwnsCanonicalFacts` | Preserve direct alias/ownership proof: mutate source names, identity slices and constraint records after construction; published facts remain the explicit original constants. Supplement with successful-publication alias tests; do not replace these with constructor/equality self-comparison. |
| `internal/runtime/element_read_test.go`, `TestElementReadTableAuditRejectsCorruption` | Preserve rejection of missing counts, altered type metadata/flags, identity offsets/values, constraint indices/values, and unreferenced identity/constraint storage. Add fixed/default discriminant coverage if absent. Each mutation must fail the owning packed audit. |
| `internal/runtime/schema_publication_test.go`, `TestProjectionAuditRejectsCorruption` and global-map corruption tests | Independent audit rejects projection/build disagreement, including element packed-table corruption routed through `validateRuntimeReadProjections`; preserve other attribute, identity, wildcard, derivation and compiled-model checks. |
| Existing publication lifecycle tests in runtime and `internal/compile/schema_publication_corruption_external_test.go` | Invalid source records fail `PublishSchema` without consuming the build; success consumes it and isolates retained aliases. Repair/retry uses remaining charged work, with no refund. Use current real lifecycle fixtures rather than inventing production injection hooks. |

These are complementary proofs: direct corruption tests establish that a malformed
packed projection is rejected; publication lifecycle tests establish consume-on-
success and failure preservation. Public compilation cannot normally inject a bad
projection between construction and audit. Do not claim a single end-to-end test
injects that state, and do not add a production hook solely to manufacture it.
External compile→publish→validate tests still establish observable fixed/default,
identity and content behavior across the complete seam. Delete the three unused
read facades in `internal/runtime/export_test.go`, and migrate the fourth facade's
two real datatype-test calls to `Schema.ValidateSimpleValue` before deleting it.
The published accessors already supply the required boundaries. Keep the narrow
identity-state fixture helper explicitly identified in the inventory: it allows
isolated state/budget tests without turning them into compiler integration tests.

### Measurement protocol

Use the same Go/tool versions, machine, GOMAXPROCS and input fixtures before/after;
avoid simultaneous benchmark workloads. Keep each raw sample with revision and
dirty-tree identity. For example, at each snapshot:

```sh
go test -run '^$' -bench '^BenchmarkSessionValidate(RepeatedSmallDocument|RepeatedXSIType|WideChoice)$' -benchmem -benchtime=1s -count=6 .
go test -run '^$' -bench '^BenchmarkCompile(SmallSchema|DeepSimpleTypeChain|RepeatedNestedUnionMembers|SubstitutionGroups)$' -benchmem -benchtime=1s -count=6 .
```

Verify those benchmark symbols against the implementation baseline before running;
do not accept an empty benchmark match as evidence. Use repository `benchstat` to
compare the saved output. Keep no-XSI and XSI-heavy shapes, known and undeclared
names, shallow/wide and deep paths, high identity fanout and many shared declarations.
Add a focused unknown-root-XSI benchmark only if existing inputs do not cover the
changed branch. No full benchmark result is claimed by this planning task.

Acceptance: no added allocs/op or B/op on unaffected no-XSI validation paths;
no new retained schema table or per-session schema cache; no worsening of explicit
resource bounds. Investigate reproducible time regressions above 5% with variance
and profiles; this is a review trigger, not proof of correctness or an automatic
license to trade allocations for speed. Any necessary regression changes the design
decision and must be resolved before declaring the packet complete.

### Requirement-to-evidence map

| Requirement/decision | Planning evidence | Future implementation evidence |
| --- | --- | --- |
| R1, R2 | Route closure table; alternatives A–F and additional seam decisions; adversarial ledger. | No unselected owner/state/API appears in the implementation. |
| R3 | P0–P3, removal ledger, predicted surface, behavior/publication matrices, measurement protocol. | All implementation boxes complete with packet-specific proof. |
| R4 | Three independent reviews of a complete plan plus correction re-reviews. | Not a requirement to implement code in this task. |
| R5, I1–I10 | Baseline evidence, preservation ledger, source hash comparison and passing baseline gates. | Focused/full checks, no user-change loss, unchanged external contracts. |
| R6 | Plan explicitly marked future work; current architecture left unchanged by planning. | P1/P2 architecture writeback in their atomic packets. |
| R7, D1–D10 | Honest test status, unchecked future work, explicit deletions and retained counterexamples. | Complete deletion/reference inventory and all required checks. |

Implementation is complete only when P0–P3 pass, required removals are present,
architecture and behavior agree, measurements satisfy the acceptance rules and no
temporary path or unowned follow-up remains. Planning is complete only when the
separate adversarial ledger below is closed and this file passes its content audit.

## Adversarial review and amendment ledger

Draft v1 is complete before review. Independent designers supplied the minimum
interface, strongest flexible-owner, and common-caller alternatives; the root
compared and corrected them against actual callers and lifecycle. In particular,
the flexible assessor's single-diagnostic/order-changing sketch was rejected, and
the common-caller proposal's extra stored type facts were not selected.

Three adversaries independently reviewed behavior/order (`adversary_behavior`),
publication/deletion (`adversary_publication`), and coverage/design necessity
(`adversary_scope`) after the complete draft existed. They received the complete
draft and repository, not each other's findings. Review completion requires evidence-backed adjudication
of every material finding and re-review of the resulting version. Same-model
agreement is correlated opinion; concrete repository evidence is the deciding
factor. No approval vote replaces a failed invariant or an unresolved factual gap.

### Planning verification evidence

On the unchanged reviewed source tree, the combined command
`make test race wasm-test web-test browser-test fuzz-smoke bench-smoke staticcheck lint`
completed with exit status 0. The worker suite passed 17 cases and browser suite
passed 5 cases. All four fuzz targets, benchmark smoke, staticcheck and lint passed;
lint reported zero issues. `git diff --check` passed. Raw output is
`/tmp/xsd-plan-checks.log`; these results establish baseline health, not proof of
unimplemented P1/P2 changes. The log SHA-256 is
`5b7e84f9eff8414393251ba87aef2ab1cc9facc8edcb44d277cf1b9ca66874c9`;
toolchains were Go `go1.27.0 darwin/arm64` and Node `v26.5.0`.

The final source-manifest comparison found zero changed baseline files and exactly
one new task file, `plan.md`. All 40,092 original hashes still match. The comparison
is recorded in `/tmp/xsd-plan-final-source-check.json`; no production, test,
architecture or public documentation file was edited by this planning task.

### Amendments

Charter v1 and R1–R7 remain unchanged. Draft v1 is archived at
`/tmp/xsd-plan-v1.md`. The following material findings were accepted and corrected
in v2; none changes the selected ownership destination.

| Finding | Reviewer and evidence | Adjudication and correction | Packet |
| --- | --- | --- | --- |
| AB1 | Behavior: integrated XSI identity outcome was left unresolved. | Accepted. Executed public API probe; specified the four lexical/semantic fixtures, exact diagnostic order, root fatal case and MaxErrors interaction. | P1 |
| AB2 | Behavior: root-selection/hint precedence was insufficiently explicit. | Accepted. Locked declaration → present XSI resolution → matching hint → recoverable missing-root order, with hint syntax staged beforehand. | P1 |
| AP1 / AS1 | Publication/scope: export inventory was deferred to implementation. | Accepted. Replaced an open-ended naming rule with a finite semantic-consumer inventory and explicit retain/private/delete outcomes. Baseline drift still requires reference reconfirmation. | P2 |
| AP2 | Publication: migrated element-read tests lacked an exact owner/oracle. | Accepted. Specified published accessor test, owner/presence/validity tuples, sparse offsets, fixed/default semantics and alias proof. | P2 |
| AP3 | Publication: corruption proof was conflated with publication failure lifecycle. | Accepted. Named complementary direct packed/projection audits and actual publication lifecycle tests; prohibited production injection hooks. | P2 |
| AP4 | Publication: unused `export_test.go` read facades would survive. | Accepted. Delete all three unused facades and migrate the fourth's two confirmed calls before deleting it. An initially incomplete search suggesting all four were unused was rejected by complete semantic references. | P2 |
| AP5 | Publication: visibility contraction was overstated; opaque builder comparison lacked actual mutation surface. | Accepted. Describe P2 as modest surface/representation contraction and compare the full existing topology-operation inventory with a real opaque owner. | P2 / C |
| AS2 | Scope: concrete compiler phase contexts were not examined. | Accepted. Added F with state, transfers, one-way phases, resource ownership and recursive component/model dependency; rejected added cyclic ownership. | F |
| AS3 | Scope: uncached reader was a weak callback counterproposal. | Accepted. Compare cache-preserving concrete and interface variants, including completion and QName ownership, rather than assume caching must be lost. | Additional seam |
| AS4 | Scope: rewrite/API break was acknowledged without a concrete destination. | Accepted. Added private schema-capability compilation/publication and replacement Schema/Validator signatures, lifecycle, consumers, migration and destination trade-offs. | E |

The reviewers independently retained B as the preferred candidate but disagreed
on the strength of its claimed depth gain. That disagreement is resolved narrowly:
P1 removes a protocol and repeated semantic resolution; P2 deletes an unused
representation and closes implementation exports. P2 is not presented as a
replacement publication architecture. Review votes are not performance evidence.

Correction re-review:

- `adversary_behavior`: AB1/AB2 closed; no remaining material behavior blocker.
  Its final wording refinement was applied: malformed hint syntax is diagnosed
  before selection, and recovery may continue when MaxErrors permits.
- `adversary_scope`: AS1–AS4 closed; no remaining material scope/design blocker.
  It rated economy 3/4, grounding 4/4, and destination/transition credibility 3/4;
  remaining baseline drift is an explicit implementation prerequisite, not an
  unresolved design choice.
- `adversary_publication`: AP1–AP5 closed after the complete v2 inventory and
  D9/D10 corrections; no remaining material publication finding. It confirmed
  consistency among the inventory, deletion ledger, test matrix and amendments.

Final content audit passed: R1–R7, I1–I10, D1–D10 and P0–P3 are present and
cross-referenced; all nine required gates are named; code fences, whitespace and
the final newline are valid; no unresolved TODO/TBD or provisional oracle remains.
`git diff --check` passed, and the untracked plan's whitespace check emitted no
diagnostics. The 25 unchecked boxes deliberately describe future implementation.

Exploration is closed under the charter's finite criterion: every architecture
route has a disposition, the materially distinct local/aggregate/opaque/phase/
rewrite/API alternatives have concrete trade-offs, and all three adversaries have
re-reviewed their corrections without a remaining material gap. No full rewrite,
public API break, callback-interface replacement or additional owner is selected.
The accepted implementation is P1's canonical start assessment plus P2's private
publication mechanics and removal of obsolete representations, completed through
P0/P3 baseline and verification work. At planning closure, implementation had not begun.


## Implementation completion — 2026-09-05

All 25 tasks in P0–P3 are complete. P1 removes the root-only result protocol and
reuses its selected XSI type in common assessment. P2 closes private publication
mechanics and deletes the alternative value table and test facades. P3 closes the
remaining helper export and retired stream allowance, preserves public behavior,
and verifies all consumers. ARCHITECTURE.md records the implemented ownership.

IM1 places the published value-constraint fixture in the existing external compile
publication suite. IM2 adds globalTypeByName to the private inventory and removes
the obsolete RootStart test allowance. Neither changes the accepted architecture
or external contract. Original requirements and rejected alternatives remain intact.

All nine required make gates passed after the last source repair. Focused tests,
semantic checks, controlled mutation failures, source-preservation audit, 66-symbol
deletion inventory and six-sample benchmark comparisons support closure. All three
implementation adversaries re-reviewed the repairs without a material finding.
See [the implementation evidence ledger](implementation-ledger.md) for exact proof,
measurement limits, review adjudication, source identity and raw-log locations.

# Fresh streaming rewrite

Status: replacement paths are integrated. The full R5 verification gates,
current corpus expectations, targeted differential probes, and bounded
flat-stream retention probe pass. The final paired performance matrix remains
open pending R5; complete XSD 1.0 coverage remains scoped by the explicit
unsupported allowlist. Baseline: `2764e554`. Branch:
`codex/fresh-rewrite`. `ARCHITECTURE.md` is the sole authority for current
ownership, data flow, lifecycle, limits, and rejected alternatives; this plan
records scope, evidence, and remaining work.

## Required result

Implement a fresh Go 1.27 library, compare it with libxml2, Xerces-J, xsdcpp,
and the requested pinned System.Xml sources, and continue until feature parity
and same or better performance are verified. Small cleanup or a renamed copy
of the old implementation does not fulfill this requirement.

Binding scope is XSD 1.0, XML 1.0, UTF-8, synchronous stream processing, explicit
schema sources, and no library-owned HTTP or other network transport. Preserve
source-resolution behavior, namespace semantics, datatypes/facets, derivation,
content models, substitutions, identity constraints, diagnostics, limits,
session isolation/reuse, and currently supported public operations. Existing
internal APIs and representations may be removed. Keep the corpus and external
behavior tests as independent evidence; rewrite structural tests when their
old representation is deliberately replaced.

Streaming applies to schema-source acquisition as well as instance documents.
The compiled schema necessarily retains semantic information; arbitrary scalar
values and identity constraints also require explicitly bounded retained state.
No complete input document or generic XML tree belongs in the final execution
path. `Bytes` may own the explicit immutable bytes supplied by its caller.

## Current architecture pointer

The current execution paths, package ownership, state transitions, resource
limits, and rejected alternatives are defined only in
[`ARCHITECTURE.md`](ARCHITECTURE.md). The replacement packets and evidence
below refer to that contract without restating it.

## Parity evidence inventory

| Capability | Required independent evidence |
| --- | --- |
| Sources and composition | Lazy/repeatable opening, cleanup and joined causes, byte/source/dependency limits, canonical identity conflicts, breadth-first resolver ordering, inherited `xml:base`, include/import and chameleon behavior |
| XML and namespaces | XML 1.0/UTF-8 acceptance, XML 1.1/DTD/entity/encoding exclusions, declarations/BOM, references/CDATA, CR normalization, positions, QName binding, duplicate attributes, malformed/truncated input |
| Schema components | Elements/attributes, groups, annotations/notations, defaults/fixed/form/nil/abstract, extension/restriction, final/block, wildcard and substitution semantics |
| Content models | Sequence/choice/all, finite/unbounded occurrences, UPA, restriction validity, wildcard overlap, model completion, bounded compilation |
| Values | All supported primitive/derived types, list/union, whitespace, regex and other facets, canonical typed equality, QName/NOTATION, ID/IDREF |
| Assessment | Declared/undeclared roots, XSI type/nil/hints, skip/lax/strict, attribute checks, value constraints, error ordering and recovery |
| Identity | Key/unique/keyref, selector/field paths, tuples, missing/nil/invalid fields, forward references, cardinality/byte limits, scope finalization |
| Lifecycle and consumers | Engine concurrency, session overlap rejection before reading, reset after every exit, bounded retention, CLI/WASM/browser public behavior |

At baseline the manifest contains 14,548 cases, 14,498 schema cases, and 25,199
instance runs (24,971 schema-backed). The 799 deliberate unsupported entries
cover regex forms, redefine, instance schema loading, XSD/XML 1.1, and non-UTF-8.
These counts describe `2764e554`; derive current inventory from the manifest
and allowlist instead of maintaining a second test catalog. Schema-less cases
remain outside the explicit precompiled-schema contract. The current
`tests/unsupported.txt` has 174 entries, including 138 `xs:redefine` cases.
Compared with the baseline, 625 entries were removed and none were added. The removed regex set was
individually adjudicated (624 valid cases and one intentional `schema.facet`
failure), and the adjudication also covered 937 regex-driven instance runs with
no mismatches.

## XSD improvement agenda

The rewrite will assess every XSD 1.0 capability, not just package layout or
XML throughput. Feature parity is the floor. Each row must reach an explicit
decision: implement an evidenced correction, adopt a simpler or faster design,
expand supported XSD 1.0 behavior where feasible, or retain the existing
semantics with a stated reason. Do not invent bugs or force a new abstraction
when the existing rule is already correct.

| Aspect | Improvement to investigate and implement when justified | Acceptance evidence |
| --- | --- | --- |
| Builtin and named simple types | Reserved builtin IDs and pure builtin metadata; one authoritative type graph and explicit compilation states instead of rebuilding and projecting builtin records for every schema | Every supported builtin and derivation relationship, redeclaration rejection, cycles/final restrictions, small-schema and deep-type-chain compilation |
| Lexical/value/canonical representations | Separate source spelling, normalized lexical input, typed value, and diagnostic rendering; parse once and reuse typed results for facets, fixed/default checks, and identities | Equivalent and distinct spellings across every primitive family; whitespace-sensitive values; canonical equality without lexical-equality shortcuts |
| Numeric types | Exact decimal/integer values with bounded digits, fast common integer cases, correct float/double precision and exceptional values; avoid repeated string parsing and unnecessary arbitrary-precision allocation | Sign/zero, huge integers, overflow boundaries, decimal digit/scale rules, NaN/infinities, cross-type value comparison, numeric-heavy benchmarks |
| Temporal and duration types | One owner for calendar normalization, timezone presence, fractional precision, and partial ordering; represent duration months separately from seconds | Leap/day/month limits, timezone boundaries, absent timezones, negative years, midnight rollover, duration comparison, date/duration facets and key equality |
| String/name/URI/binary/QName/NOTATION types | Specialize lexical checks without changing Unicode/XML rules; decode binary once; resolve QName values against their actual namespace context; keep URI validation separate from loading | Unicode name boundaries, normalizedString/token behavior, URI escaping, binary padding/whitespace, QName namespace changes, declared notation requirements |
| List and union types | Compact member programs, bounded member expansion, no temporary split arrays on common paths, and one typed result that preserves the selected member's value/identity semantics | List item versus lexical lengths, empty lists and builtin nonempty lists, illegal nested list constructions, union member order, nested unions, QName/ID-related members |
| Facet applicability and derivation | Compile applicable, inherited, fixed, and locally declared constraints into one validated facet program; detect contradictory or weakening restrictions at construction | Per-type facet applicability, fixed facets, inherited restrictions, mutually exclusive bounds, whitespace strengthening, invalid schema facets |
| Length and digit facets | Compute the correct measure once: Unicode characters, decoded octets, list items, or decimal digits/scale; specialize only after preserving that unit | Below/at/above each bound; multibyte strings, binary encodings, lists, leading/trailing zeros, large facet literals |
| Enumeration and ordered bounds | Prevalidate literals into the base type's value space; use typed equality and comparison; index sufficiently large enumerations where measurement justifies it | Equivalent lexical forms, QName/list/union values, incomparable temporal values, inclusive/exclusive edges, fixed/default equivalence, wide enumeration benchmarks |
| Pattern facets and XSD regular expressions | Preserve pattern derivation semantics; use the bounded `internal/xsdregex` matcher for character-class subtraction, XML name escapes, Unicode categories/blocks, and valid occurrence ranges | Primary-spec cases and regex corpus; same-step versus inherited pattern groups; adversarial patterns; compile/match work bounds; no catastrophic backtracking; 625 baseline regex rows adjudicated, 937 instance runs with no mismatches |
| Element and attribute declarations | Store prevalidated declaration facts and value constraints once; eliminate repeated type/name/default reconstruction in validation | Global/local scope, form defaults, refs, abstract/nillable, required/prohibited/default/fixed attributes, declaration ordering diagnostics |
| Complex types and derivation | One explicit derivation operation for simple/complex content, extension/restriction, attributes, and particles; private dependency states replace loosely coordinated build maps | Base-kind rules, cycles, final/block, mixed/simple content, valid content restriction, attribute-use merges, wildcard restriction, exact failure diagnostics |
| Content models and occurrences | Compact counted automata, specialized `all` state, and direct integer transitions; avoid unnecessary expansion and duplicated source/compiled/read models | Sequence/choice/all, nested/referenced groups, emptiable particles, finite/unbounded occurrence limits, UPA, extension concatenation, restriction, nested/adversarial model benchmarks |
| Wildcards | One namespace-set algebra shared by compilation and execution, with processContents rules enforced at the semantic owner | `##any`, `##other`, `##local`, target namespaces and lists; empty-namespace exclusion; overlap/subset/union/intersection; non-expressible attribute intersections; strict/lax/skip |
| Substitution groups | One owner for direct affiliation, inherited type finalization, cycle checks, and bounded effective closure; retain provenance only while compilation needs it | Transitive membership, missing heads, abstract declarations, type derivation, final/block masks, wildcard/UPA interactions, substitution fanout benchmarks |
| Identity constraints | Compile selector/field paths into compact programs; investigate indexing active matches by depth/name/path to avoid scanning every scope and selection on every event | Key/unique/keyref, compound typed tuples, missing/invalid/nil fields, descendant/union paths, skip/lax visibility, forward references, scope completion, rollback/reuse, identity-heavy allocation and throughput |
| Schema composition and reuse | Typed semantic documents plus effective-namespace views; no raw-tree cloning for chameleon includes; explicit context-aware symbol resolution and bounded graph work | Include/import namespace rules, chameleon QName adoption, cycles, same-content sources with distinct resolver contexts, deterministic traversal, context/fanout limits and benchmarks |
| `xs:redefine` | Assess adding the currently unsupported XSD 1.0 feature using scoped prior/new component bindings and the specification's derivation rules; reject an unbounded or partial implementation | Dedicated redefine corpus, self-reference and kind/derivation rules, include cycles, chameleon interaction, bounded compilation; remove unsupported entries only when cases pass |
| Annotations, notations, and schema syntax | Consume annotation payload without retention while preserving namespace/XML checks; retain only semantically relevant declarations and source locations | Annotation envelope/order rules, foreign markup, schema ID constraints, notation uniqueness, XML namespace behavior, streaming retention |
| Assessment and diagnostics | Precompute schema-only facts; keep XSI selection, nil/type checks, value assessment, recovery, and error ordering in one document transition owner | `xsi:type` derivation, `xsi:nil`, hints without loading, defaults/fixed, semantic-stop recovery, path/line/column/cause preservation, error budgets |

The current ownership and data-flow decisions for these improvements are
recorded in `ARCHITECTURE.md`; the evidence index in
[`docs/rewrite-assessment.md`](docs/rewrite-assessment.md) records the focused
proof for each agenda row. Neither document closes the remaining performance
gate.

The remaining explicit coverage candidate is the 138-entry `xs:redefine`
allowlist. The regex rows have been removed only after individual
specification-driven adjudication. XSD 1.1 assertions, alternatives,
override/open content, XML 1.1, non-UTF-8, DTD/entity expansion, and network or
instance-directed schema loading remain outside the selected contract.

The initial facet audit also identified an incorrect statement in the local
datatype guide: same-step patterns are ORed, while inherited groups across
derivation steps are ANDed (`src-multiple-patterns`). The guide is corrected;
the existing compiler/runtime grouping already follows that rule. Distinguish
documentation defects from validator defects when tracking improvements.

The datatype audit resolved both outstanding decisions:

- Duration parsing promotes month and second coordinates beyond `int64` to the
  arbitrary-precision `durationInteger` representation. The fast path remains
  inline, while promoted work and storage are linear in the admitted lexical
  length; checked arithmetic and duration comparison remain in `internal/value`.
- Float/double canonical rendering now uses the XSD lexical form, including
  exponent and zero formatting. Typed equality and identity projections remain
  separate from canonical lexical output.

For each accepted improvement, record the exact specification clause, current
behavior and evidence, selected representation/algorithm, owner, complexity
bounds, adversarial tests, and before/after measurements. Any added support must
use the same canonical compiler and validator. Update the local specification
guides when their shorthand disagrees with the primary specification.

## Replacement packets

Each packet replaces one complete execution path and deletes its predecessor.
Temporary use of unreplaced capabilities is migration state, not completion.
Concrete package boundaries for later packets remain subject to implementation
evidence and the independent design review.

The checkmarks below mean that the replacement path is integrated and has the
focused evidence named in the item. Full R4 verification gates, current corpus
expectations, targeted differential probes, and bounded flat-stream retention
evidence pass. The final paired-performance gate remains open pending R5; the
explicit unsupported allowlist continues to define the selected feature
boundary.

- [x] Import the actual current library, commit its complete state, and create a
  separate rewrite branch and immutable baseline worktree.
- [x] Run the baseline behavior suite and capture initial measurements.
- [x] Inspect all four references and record findings with pinned source links.
- [x] Replace full-buffer schema acquisition with source-owned bounded readers;
  stream into schema parsing, retain content identities without source buffers,
  and preserve I/O failure, cleanup, and byte-limit behavior.
- [x] Replace generic schema XML trees with typed semantic declarations and
  references, including source positions, annotation consumption, QName
  resolution, include/import closure, and chameleon contexts. Repository
  consumer-path tests and current corpus expectations pass; complete XSD 1.0
  coverage remains scoped by the explicit unsupported allowlist.
- [x] Replace cross-package mutable schema building and projection publication
  with private construction and one immutable compiled representation. Avoid
  rebuilding an entire builtin graph for each compilation; sealing consumes
  construction state only after validation succeeds.
- [x] Reimplement simple-type compilation and execution around one typed-value
  contract: all supported primitives, whitespace, list/union, restrictions,
  facets, regex semantics, fixed/default values, and identity equality. The
  repository integration and targeted datatype differential probe pass, while
  complete datatype/facet coverage remains subject to final evidence.
- [x] Reimplement bounded content-model compilation/execution, including
  sequence/choice/all, counted occurrences, wildcard overlap, substitution
  groups, UPA, and restriction rules. Repository integration and current
  corpus expectations pass; complete adversarial conformance and performance
  evidence remain open.
- [x] Reimplement the XML/namespace input boundary with one authoritative owner
  for XML 1.0 well-formedness, expanded names, borrowed data lifetime, positions,
  token/input limits, and cleanup. Focused package gates pass.
- [x] Reimplement document validation and identity assessment with explicit
  element transitions, syntax-only recovery, diagnostic ordering, bounded
  retention, and reusable-session failure isolation. Repository consumer gates,
  current corpus expectations, and targeted differential probes pass, while
  complete adversarial conformance and bounded-retention evidence remain open.
- [x] Migrate public entrypoints and CLI/WASM/browser consumers, replace any
  remaining tree-based formatting path, delete superseded production paths,
  and update canonical architecture and public documentation. Repository-wide
  verification, current corpus expectations, and targeted differential probes
  pass; final paired-performance evidence remains open.
- [ ] Complete the final paired performance matrix pending R5, then close any
  remaining adversarial review findings and update the evidence packet.

## Verification gates

### R4 verification state

The full R4 verification contract passes: `make test`, `make race`,
`make wasm-test`, `make web-test`, `make browser-test`, `make fuzz-smoke`,
`make bench-smoke`, `make staticcheck`, `make lint`, formatting, and
`git diff --check`. The current corpus expectations also pass. Raw gate output
and supporting measurements are retained under `.lab/rewrite`.

The baseline manifest had 799 unsupported entries. The current
`tests/unsupported.txt` has 174: 625 baseline entries were removed and 0 were
added. The removed regex set was individually adjudicated, including 937
dependent instance runs with no mismatches.

A targeted differential probe compared 16,830 builtin-type/lexical pairs with
the baseline. Acceptance and unsupported classification differ only at one
intended huge-duration acceptance delta, caused by arbitrary-precision duration
coordinates. Identity equivalence partitions match for all 3,103 jointly
accepted values. This probe is evidence for those inputs, not the complete
datatype or facet contract.

A warmed flat-stream retention probe at 1 MiB, 16 MiB, and 128 MiB measured
approximately 188,288 live bytes for the current tree, with a plateau across
input sizes, versus 238,944 bytes for the baseline. Total allocations after
warm-up were 800 B. This bounds the tested flat-stream shape; it does not close
the complete retained-state or performance gate.

The final paired performance matrix remains open pending R5. Do not mark the
rewrite complete until it reports matching workloads against baseline
`2764e554`, with throughput/time, B/op, allocs/op, and peak/live-retention
evidence where required.

The baseline at `/Users/jacoelho/Documents/ChatGPT/xsd-baseline` is a detached
worktree of `2764e554`. Compare equivalent tests and benchmarks against that
revision, using the same toolchain, input generation, options, and harness.
Keep raw measurement outputs with revision and environment identifiers.

For each replaced capability, run focused behavioral tests and its complete
consumer path. Use corpus expectations, explicit unsupported classifications,
and standards clauses to adjudicate differences. Do not weaken the unsupported
allowlist or remove failure cases to manufacture parity. Add adversarial input
shapes where the existing tests cannot prove the new invariant.

Performance evidence must cover compilation, first validation, repeated
session validation, independent/concurrent validation, large streamed inputs,
identity-heavy inputs, namespaces/attributes, scalar datatypes/facets, complex
content models, and failure paths. Report throughput/time, B/op, allocs/op,
and peak/live retention where allocation totals cannot prove streaming. Repeat
paired measurements sufficiently to separate regressions from noise. A faster
single microbenchmark cannot satisfy the repository-wide performance goal.

Before completion run the repository's complete verification contract:
`make test`, `make race`, `make wasm-test`, `make web-test`,
`make browser-test`, `make fuzz-smoke`, `make bench-smoke`,
`make staticcheck`, `make lint`, formatting, and `git diff --check`.

Completion also requires an inventory showing every old production capability
has been replaced or explicitly removed within scope, no parallel legacy
implementation remains, all required behavior has authoritative evidence, and
no performance claim depends on omitted validation work.

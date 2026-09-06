# XSD improvement assessment

This document records the implementation decision for every row in the
improvement agenda in [`rewrite-plan.md`](../rewrite-plan.md). It is an
evidence index, not a second architecture document. [`ARCHITECTURE.md`](../ARCHITECTURE.md)
owns module boundaries, state ownership, limits, lifecycle, and rejected
alternatives. [`reference-comparison.md`](reference-comparison.md) records the
reference implementations and the design consequences taken from them.

“Implemented” below means that the current tree has one owning path and focused
or seam-level evidence for the stated behavior. It does not mean that complete
XSD 1.0 coverage or the final paired-performance gate is closed.

Current evidence state: the full R4 verification gates and current corpus
expectations pass. The baseline unsupported inventory was 799 entries; the
current allowlist has 174, with 625 removed and 0 added. A targeted differential
probe compared 16,830 builtin-type/lexical pairs; acceptance and unsupported
classification differ only at one intended huge-duration acceptance delta, and
identity equivalence partitions match for all 3,103 jointly accepted values. A
warmed flat-stream retention probe measured approximately 188,288 live bytes at
1 MiB, 16 MiB, and 128 MiB, plateauing across sizes, versus 238,944 baseline
bytes; total allocations after warm-up were 800 B. The final paired-performance
matrix remains open pending R5.

## 1. Builtin and named simple types — implemented

`internal/value` owns the fixed builtin ID space and metadata. Its
[`builtins.go`](../internal/value/builtins.go) keeps the builtin chain,
primitive, whitespace, identity, list-item, and builtin-specific facets in one
table. `internal/schema` consumes those IDs through
[`compile_builtins.go`](../internal/schema/compile_builtins.go); it does not
rebuild a second value builtin graph. User types use reserved IDs and
dependency-first completion in `value.Builder`, while schema derivation checks
live in [`simple_derivation.go`](../internal/schema/simple_derivation.go) and
the shared derivation index.

The construction and dependency invariants are exercised by
[`builtin_contract_test.go`](../internal/value/builtin_contract_test.go),
[`builtin_test.go`](../internal/schema/builtin_test.go),
[`simple_derivation_test.go`](../internal/schema/simple_derivation_test.go),
and [`component_dependency_test.go`](../internal/schema/component_dependency_test.go).
These cover canonical IDs, forward bases, list reachability, final masks, and
dependency completion. Deep-chain and full-corpus parity remain completion
evidence rather than claims made by this row.

## 2. Lexical, value, and canonical representations — implemented

The value owner separates source spelling, normalized text, typed parsing, and
the projections needed by consumers. `parsedValue` is transient evaluation
state; the published [`Value`](../internal/value/eval.go) carries independent
canonical, value-space identity, selected-union, and dynamic ID/IDREF
projections. `Value.Equal` compares the typed identity projection, never source
lexical text. Schema literals enter through `compileLiteral` and the same value
program used for instance values.

The strongest anchors are [`eval.go`](../internal/value/eval.go),
[`identity_key.go`](../internal/value/identity_key.go),
[`program_test.go`](../internal/value/program_test.go),
[`identity_projection_contract_test.go`](../internal/value/identity_projection_contract_test.go),
and [`value_constraint_test.go`](../internal/schema/value_constraint_test.go).
The architecture records the type-specific rule that duration equality uses
month/second coordinates while duration has no XSD 1.0 canonical lexical form;
that is kept separate from canonical projections for types that define one.

## 3. Numeric types — implemented with specialized value representations

Decimal and integer-family values use exact digit-preserving representations in
[`decimal.go`](../internal/value/decimal.go) and
[`decimal_lexical.go`](../internal/value/decimal_lexical.go), including
canonical text, integer canonical text, total digits, fraction digits, and
value-space comparison. Float and double have their own bit-width parser,
exceptional-value handling, equality, ordered-facet relation, and canonical
rendering in [`float.go`](../internal/value/float.go). Borrowed XML bytes can
take the raw validation path when no typed projection is requested.

[`raw_validate_test.go`](../internal/value/raw_validate_test.go),
[`program_test.go`](../internal/value/program_test.go),
[`builtin_contract_test.go`](../internal/value/builtin_contract_test.go), and
the project cases
[`float-fixed-zero-value-space`](../tests/corpus/project/float-fixed-zero-value-space)
and
[`runtime-total-digits-leading-dot-decimal`](../tests/corpus/project/runtime-total-digits-leading-dot-decimal)
anchor zero/value-space, decimal-bound, integer-lexical, and digit-facet
behavior. Numeric benchmark superiority is intentionally not inferred from
these tests.

## 4. Temporal and duration types — implemented

`internal/value` is the sole owner of temporal normalization, timezone
presence, fractional text, partial ordering, and duration comparison. Date,
dateTime, time, and `g*` values are parsed into dedicated projections in
[`temporal.go`](../internal/value/temporal.go) and
[`gvalue.go`](../internal/value/gvalue.go). Duration keeps month and second
coordinates separate in [`duration.go`](../internal/value/duration.go);
coordinates beyond `int64` promote to the decimal-digit
[`durationInteger`](../internal/value/duration_integer.go) representation.
Promotion is linear in admitted lexical length and remains subject to the
normal input and evaluation limits.

[`duration_test.go`](../internal/value/duration_test.go) covers lexical space,
equality, identity, partial order, arbitrary magnitude, exact arithmetic, and
calendar-reference ordering. [`time_facet_contract_test.go`](../internal/value/time_facet_contract_test.go)
and the project cases
[`runtime-duration-partial-order-strict`](../tests/corpus/project/runtime-duration-partial-order-strict),
[`attribute-fixed-time-leap-second-offset-equivalent-lexical`](../tests/corpus/project/attribute-fixed-time-leap-second-offset-equivalent-lexical),
and [`element-fixed-time-leap-second-offset-equivalent-utc`](../tests/corpus/project/element-fixed-time-leap-second-offset-equivalent-utc)
anchor timezone, rollover, fixed-value, and duration facet behavior.

## 5. String, name, URI, binary, QName, and NOTATION types — implemented

Primitive lexical ownership is split by value kind: string/name rules are in
[`string.go`](../internal/value/string.go) and the lexical helpers, URI
validity is delegated to [`internal/uriref`](../internal/uriref), binary
length/decoding is in [`binary.go`](../internal/value/binary.go), and QName and
NOTATION resolution is performed by `parseNameAtomic` in
[`eval.go`](../internal/value/eval.go). The resolver is supplied by the schema
or validation boundary; URI validation does not load a source. Schema QName
attributes are resolved while their namespace frame is live, as required by
the architecture contract.

[`builtin_contract_test.go`](../internal/value/builtin_contract_test.go),
[`program_test.go`](../internal/value/program_test.go),
[`schema_namespace_external_test.go`](../internal/schema/schema_namespace_external_test.go),
and the project cases
[`runtime-any-uri-allows-space`](../tests/corpus/project/runtime-any-uri-allows-space),
[`notation-requires-declaration-invalid`](../tests/corpus/project/notation-requires-declaration-invalid),
[`notation-requires-declaration-valid`](../tests/corpus/project/notation-requires-declaration-valid),
and [`namespace-qname-enumeration-facet-uses-schema-namespaces`](../tests/corpus/project/namespace-qname-enumeration-facet-uses-schema-namespaces)
cover lexical names, URI behavior, resolver binding, declared notation, and
namespace-sensitive value constraints.

## 6. List and union types — implemented

`TypeSpec` and the sealed `Program` represent list item types and ordered union
members in one value program. Evaluation in [`eval.go`](../internal/value/eval.go)
keeps the selected union member and builds one typed result; list and union
evaluation charge depth and cumulative work. Dynamic identity projections come
from selected values: list items contribute IDREFs and unions preserve the
selected member's ID/IDREF semantics. Builtin non-empty list constraints remain
in the fixed metadata table.

The behavior is anchored by
[`program_test.go`](../internal/value/program_test.go),
[`raw_validate_test.go`](../internal/value/raw_validate_test.go),
[`identity_projection_contract_test.go`](../internal/value/identity_projection_contract_test.go),
[`type_source_test.go`](../internal/schema/type_source_test.go), and the
project cases
[`list-restriction-length-valid`](../tests/corpus/project/list-restriction-length-valid),
[`runtime-double-list-fast-path-lexical`](../tests/corpus/project/runtime-double-list-fast-path-lexical),
and [`runtime-identity-field-union-single-field`](../tests/corpus/project/runtime-identity-field-union-single-field).
The implementation is bounded by the value depth, type-storage, and evaluation
work limits; no unbounded member expansion is admitted.

## 7. Facet applicability and derivation — implemented

Facet legality, fixedness, inheritance, and derivation are compiled into the
value program. [`facet_derivation.go`](../internal/value/facet_derivation.go)
owns value-side restriction checks; schema source admission and facet syntax
are in [`compile_facets.go`](../internal/schema/compile_facets.go) and
[`facet_syntax.go`](../internal/schema/facet_syntax.go). A type cannot publish
until applicable facets, dependencies, and fixed restrictions validate. Pattern
groups use OR within one restriction step and AND across inherited steps.

[`facet_contract_test.go`](../internal/value/facet_contract_test.go),
[`facet_allowed_test.go`](../internal/value/facet_allowed_test.go),
[`facet_syntax_test.go`](../internal/schema/facet_syntax_test.go), and
[`schema_publication_corruption_external_test.go`](../internal/schema/schema_publication_corruption_external_test.go)
cover forbidden facets, fixed changes, widening restrictions, inherited
length/pattern rules, enumeration value-space checks, partial ordered bounds,
and publication-time constraint validation.

## 8. Length and digit facets — implemented

The value evaluator computes each facet in its owning value-space unit:
normalized character counts for text, decoded octets for binary values, item
counts for lists, and decimal total/fraction digits for decimal families.
QName and NOTATION length facets use their XSD-specific applicability rule.
The result is checked against the compiled facet program rather than by
reinterpreting the source spelling in validation.

Concrete anchors are `DecimalValue` and binary length functions in
[`decimal.go`](../internal/value/decimal.go) and
[`binary.go`](../internal/value/binary.go), the length branches in
[`eval.go`](../internal/value/eval.go), and
[`raw_validate_test.go`](../internal/value/raw_validate_test.go). The project
cases [`runtime-string-length-counts-codepoints`](../tests/corpus/project/runtime-string-length-counts-codepoints),
[`list-restriction-length-valid`](../tests/corpus/project/list-restriction-length-valid),
and [`runtime-total-digits-leading-dot-decimal`](../tests/corpus/project/runtime-total-digits-leading-dot-decimal)
exercise multibyte text, list items, and decimal digit boundaries.

## 9. Enumeration and ordered bounds — implemented

Enumeration and ordered bound literals are admitted through the owning base
type during value-program construction. Runtime comparison uses typed equality
and the primitive's ordered relation; temporal values can report incomparable
when timezone information does not establish an order. Fixed/default checks
therefore use value-space semantics and QName/NOTATION literals use their
resolved names.

[`facet_derivation.go`](../internal/value/facet_derivation.go),
[`partial_order_facet_test.go`](../internal/value/partial_order_facet_test.go),
[`facet_contract_test.go`](../internal/value/facet_contract_test.go),
[`value_constraint_test.go`](../internal/schema/value_constraint_test.go), and
the project cases
[`namespace-qname-enumeration-facet-uses-schema-namespaces`](../tests/corpus/project/namespace-qname-enumeration-facet-uses-schema-namespaces),
[`float-fixed-zero-value-space`](../tests/corpus/project/float-fixed-zero-value-space),
and [`runtime-duration-partial-order-strict`](../tests/corpus/project/runtime-duration-partial-order-strict)
anchor equivalent lexical forms, typed enumeration, zero equality, and
incomparable temporal bounds.

## 10. Pattern facets and XSD regular expressions — implemented for the supported XSD 1.0 matcher

`internal/xsdregex` owns parsing and whole-input matching. Its compiled pattern
selects literal, linear, or NFA execution in [`regex.go`](../internal/xsdregex/regex.go),
[`linear.go`](../internal/xsdregex/linear.go), and [`nfa.go`](../internal/xsdregex/nfa.go).
Character-class subtraction, XML name escapes, Unicode blocks/categories,
counted repeats, and explicit match/state work limits are exercised by
[`regex_test.go`](../internal/xsdregex/regex_test.go),
[`corpus_compile_test.go`](../internal/xsdregex/corpus_compile_test.go),
[`corpus_match_test.go`](../internal/xsdregex/corpus_match_test.go), and
[`regex_fuzz_test.go`](../internal/xsdregex/regex_fuzz_test.go). `value.Pattern`
is an alias to this matcher; value owns only facet grouping and inheritance.

The baseline 625 regex rows were individually adjudicated and the dependent
937 instance runs had no mismatches, as recorded in `rewrite-plan.md`; this is
corpus evidence for those rows, not a general performance claim. Final
conformance and paired performance evidence remain open.

## 11. Element and attribute declarations — implemented

Schema declaration compilation normalizes names, forms, references, types,
abstract/nillable flags, defaults, fixed values, and attribute-use policy into
typed declaration records. [`compile_elements.go`](../internal/schema/compile_elements.go)
and [`compile_attributes.go`](../internal/schema/compile_attributes.go) own
construction; [`declaration.go`](../internal/schema/declaration.go),
[`element_read.go`](../internal/schema/element_read.go), and
[`attribute_runtime.go`](../internal/schema/attribute_runtime.go) expose the
sealed reads consumed by validation. Value constraints are validated once at
the schema boundary and then reused.

[`declaration_test.go`](../internal/schema/declaration_test.go),
[`attribute_admission_test.go`](../internal/schema/attribute_admission_test.go),
[`attribute_runtime_test.go`](../internal/schema/attribute_runtime_test.go),
and [`start_test.go`](../internal/validate/start_test.go) cover declaration
shape, required/prohibited/fixed/default behavior, and start-time use. The
project cases [`attribute-group-prohibited-attribute`](../tests/corpus/project/attribute-group-prohibited-attribute),
[`attribute-required-fixed-idref-no-default`](../tests/corpus/project/attribute-required-fixed-idref-no-default),
and the fixed-time cases provide seam-level examples.

## 12. Complex types and derivation — implemented

Complex extension and restriction are compiled by one schema path through
[`compile_complex.go`](../internal/schema/compile_complex.go),
[`complex_derivation.go`](../internal/schema/complex_derivation.go),
[`attribute_use_merge.go`](../internal/schema/attribute_use_merge.go), and
[`content_restriction.go`](../internal/schema/content_restriction.go).
Dependency states and the shared derivation indexes make base kind, final/block,
simple/complex content, particle, attribute-use, and wildcard restrictions
explicit before publication.

[`complex_derivation_test.go`](../internal/schema/complex_derivation_test.go),
[`attribute_use_merge_test.go`](../internal/schema/attribute_use_merge_test.go),
[`simple_content_read_test.go`](../internal/schema/simple_content_read_test.go),
and [`content_model_test.go`](../internal/schema/content_model_test.go) cover
the derivation and restriction decisions. The project cases
[`mixed-extension-empty-base`](../tests/corpus/project/mixed-extension-empty-base),
[`mixed-extension-nonempty-base-invalid`](../tests/corpus/project/mixed-extension-nonempty-base-invalid),
and [`runtime-complex-content-extension-includes-base-particle`](../tests/corpus/project/runtime-complex-content-extension-includes-base-particle)
anchor extension and mixed/simple-content behavior.

## 13. Content models and occurrences — implemented

Schema owns particle normalization, occurrence handling, UPA/restriction
analysis, compiled transitions, and immutable reads. The path is split by
representation rather than ownership: [`content_model.go`](../internal/schema/content_model.go)
and [`content_compile.go`](../internal/schema/content_compile.go) admit and
compile models, [`content_model_dfa.go`](../internal/schema/content_model_dfa.go)
and [`content_model_lowering.go`](../internal/schema/content_model_lowering.go)
perform bounded lowering, and [`content_read.go`](../internal/schema/content_read.go)
serves validation. Large `xs:all` models use the sealed QName-to-term index;
small models use the direct path, with atomic transition failure.

The focused evidence is in [`content_model_test.go`](../internal/schema/content_model_test.go),
[`content_model_dfa_test.go`](../internal/schema/content_model_dfa_test.go),
[`content_model_lowering_test.go`](../internal/schema/content_model_lowering_test.go),
[`all_index_test.go`](../internal/schema/all_index_test.go), and
[`content_model_external_test.go`](../internal/schema/content_model_external_test.go).
The project cases [`architecture-repeated-choice-partitions-without-replay`](../tests/corpus/project/architecture-repeated-choice-partitions-without-replay),
[`architecture-repeated-group-choice-partitions`](../tests/corpus/project/architecture-repeated-group-choice-partitions),
and [`architecture-repeated-choice-duplicate-optional-upa`](../tests/corpus/project/architecture-repeated-choice-duplicate-optional-upa)
anchor repeated occurrence, partition, and UPA behavior. State/work limits
remain explicit; performance superiority is unclaimed.

## 14. Wildcards — implemented

`internal/schema` is the single owner of wildcard namespace algebra and
restriction. [`wildcard.go`](../internal/schema/wildcard.go),
[`wildcard_compile.go`](../internal/schema/wildcard_compile.go),
[`attribute_wildcard.go`](../internal/schema/attribute_wildcard.go), and
[`particle_overlap.go`](../internal/schema/particle_overlap.go) compile
`##any`, `##other`, `##local`, target/list namespace sets, subset/overlap, and
process-contents metadata. Validation reads those immutable policies in the
content and attribute paths; it does not reconstruct namespace sets.

[`wildcard_test.go`](../internal/schema/wildcard_test.go),
[`wildcard_compile_test.go`](../internal/schema/wildcard_compile_test.go),
[`attribute_wildcard_test.go`](../internal/schema/attribute_wildcard_test.go),
and [`content_model_external_test.go`](../internal/schema/content_model_external_test.go)
cover set operators, restriction, overlap, and attribute expressibility. The
project cases [`namespace-wildcard-allows-empty-namespace-list`](../tests/corpus/project/namespace-wildcard-allows-empty-namespace-list),
[`namespace-wildcard-rejects-all-token`](../tests/corpus/project/namespace-wildcard-rejects-all-token),
[`namespace-repeating-sequence-wildcard-overlap-is-upa-compile-error`](../tests/corpus/project/namespace-repeating-sequence-wildcard-overlap-is-upa-compile-error),
and [`runtime-any-attribute-skip-skips-declared-attributes`](../tests/corpus/project/runtime-any-attribute-skip-skips-declared-attributes)
anchor namespace and process-contents outcomes.

## 15. Substitution groups — implemented

Schema compilation owns direct affiliations, missing-head diagnostics, cycles,
effective types, final/block masks, and the bounded transitive closure. The
closure is built once in [`substitution_compile.go`](../internal/schema/substitution_compile.go)
and installed in the sealed schema; validation consumes the immutable table
through schema reads. Element consistency and content-model checks run after
effective substitution types are finalized.

[`substitution_test.go`](../internal/schema/substitution_test.go),
[`substitution_compile_test.go`](../internal/schema/substitution_compile_test.go),
[`element_finalization_external_test.go`](../internal/schema/element_finalization_external_test.go),
and [`content_compiled_test.go`](../internal/schema/content_compiled_test.go)
cover closure limits, cycles, direct-edge work, final/block checks, and runtime
member lookup. The project cases [`namespace-cyclic-substitution-groups-are-schema-errors`](../tests/corpus/project/namespace-cyclic-substitution-groups-are-schema-errors),
[`namespace-substitution-group-transitive-intermediate-block`](../tests/corpus/project/namespace-substitution-group-transitive-intermediate-block),
[`namespace-substitution-group-upa-is-checked-after-substitutions`](../tests/corpus/project/namespace-substitution-group-upa-is-checked-after-substitutions),
and [`namespace-substitution-group-member-defaults-to-head-type`](../tests/corpus/project/namespace-substitution-group-member-defaults-to-head-type)
anchor the cross-boundary behavior.

## 16. Identity constraints — implemented with bounded document-local state

Schema compiles selector and field paths into immutable programs and exact-name,
namespace, and wildcard dispatch indexes in
[`compile_identity.go`](../internal/schema/compile_identity.go),
[`identity_xpath.go`](../internal/schema/identity_xpath.go), and
[`identity_read.go`](../internal/schema/identity_read.go). Validation owns the
active scope stack, pending selections, field batches, document IDs/IDREFs,
tuple finalization, and reset in [`identity_evaluation.go`](../internal/validate/identity_evaluation.go),
[`identity_match.go`](../internal/validate/identity_match.go), and
[`session_identity.go`](../internal/validate/session_identity.go). Value
projections, including union/list IDREF behavior, feed that evaluator without
making static type metadata authoritative.

[`identity_compile_test.go`](../internal/schema/identity_compile_test.go),
[`identity_xpath_test.go`](../internal/schema/identity_xpath_test.go),
[`identity_evaluation_test.go`](../internal/validate/identity_evaluation_test.go),
[`identity_match_test.go`](../internal/validate/identity_match_test.go),
[`session_start_transaction_test.go`](../internal/validate/session_start_transaction_test.go),
and [`session_reset_test.go`](../internal/validate/session_reset_test.go) cover
path parsing, dispatch, atomic value batches, rollback, reset, and limits. The
project cases [`runtime-identity-selector-descendant-mid-path-duplicate-key`](../tests/corpus/project/runtime-identity-selector-descendant-mid-path-duplicate-key)
and [`runtime-keyref-missing-field-excluded`](../tests/corpus/project/runtime-keyref-missing-field-excluded)
anchor descendant matching and missing-field handling. Identity-heavy paired
allocation and throughput evidence remains part of the open performance gate.

## 17. Schema composition and reuse — implemented

`internal/source` owns repeatable opening, bounded reads, cleanup, canonical
identity, and explicit resolution. `internal/schema/schema_set.go` loads and
closes the graph before planning; `semantic_source.go` retains typed semantic
records and effective-namespace context rather than a generic XML tree.
`schemaTargetContexts` bounds chameleon context expansion, while dependency
work, source, node, and byte limits are charged at their admitting owners.

The source seam is covered by [`source_test.go`](../internal/source/source_test.go),
including limits, finish/close errors, canonical references, and resolver
ordering. Schema composition is covered by [`schema_set_limits_test.go`](../internal/schema/schema_set_limits_test.go),
[`schema_boundary_external_test.go`](../internal/schema/schema_boundary_external_test.go),
[`schema_namespace_external_test.go`](../internal/schema/schema_namespace_external_test.go),
and [`schema_compile_external_test.go`](../internal/schema/schema_compile_external_test.go),
including XML base, target-namespace checks, direct import rules, and chameleon
structural checks. This implements the architecture consequence identified in
the reference comparison: stream source admission, then compile a closed graph
with context-aware semantic records.

## 18. `xs:redefine` — retained unsupported baseline scope

The current decision is explicit: `xs:redefine` remains unsupported and stays
in the baseline allowlist. Schema admission identifies the child and returns
the public unsupported code through [`compile_children.go`](../internal/schema/compile_children.go);
the syntax and diagnostic contract is covered by
[`schema_syntax_test.go`](../internal/schema/schema_syntax_test.go) and
[`schema_compile_external_test.go`](../internal/schema/schema_compile_external_test.go).
The public scope also records the decision in [`README.md`](../README.md), and
the operational corpus retains the 138 `unsupported.xs_redefine` entries in
[`tests/unsupported.txt`](../tests/unsupported.txt).

This is a scope decision, not a claim that the feature is impossible or that
the allowlist is a standards verdict. A complete implementation would need
versioned component bindings for the prior and replacement declarations,
self-reference rules that distinguish the old component from the redefining
component, component-kind-specific derivation and restriction checks, and
reference rebinding across include/import closure. Those bindings would also
have to compose with effective-namespace/chameleon views, cyclic graph
diagnostics, and the single immutable publication step. A partial implementation
would require parallel symbol tables or post-publication mutation and could
silently select the wrong component; an unbounded implementation would evade
the existing dependency, component, and content work budgets. The repository
therefore keeps the explicit unsupported result until a bounded, full
component-version plan and dedicated corpus evidence exist. No current claim
is made about untested redefine cases beyond that operational scope.

## 19. Annotations, notations, and schema syntax — implemented

The typed schema parser consumes annotation payload as namespace-well-formed
opaque XML and retains only semantic fields, source positions, IDs, and the
bounded context needed by deferred QName/XPath interpretation. Syntax ordering,
schema IDs, annotation envelopes, and notation declarations are owned by
[`schema_syntax.go`](../internal/schema/schema_syntax.go),
[`ast_typed.go`](../internal/schema/ast_typed.go), and
[`children.go`](../internal/schema/children.go). Notation declarations are
published in the schema read view and consulted by the value resolver.

[`schema_boundary_external_test.go`](../internal/schema/schema_boundary_external_test.go),
[`schema_syntax_test.go`](../internal/schema/schema_syntax_test.go),
[`schema_fuzz_test.go`](../internal/schema/schema_fuzz_test.go),
[`value_constraint_test.go`](../internal/schema/value_constraint_test.go), and
the project cases [`schema-unknown-markup`](../tests/corpus/project/schema-unknown-markup),
[`notation-requires-declaration-invalid`](../tests/corpus/project/notation-requires-declaration-invalid),
and [`notation-requires-declaration-valid`](../tests/corpus/project/notation-requires-declaration-valid)
anchor opaque markup admission, ordering, source IDs, notation validity, and
value-constraint use. Annotation payload is not retained as a generic tree.

## 20. Assessment and diagnostics — implemented

`internal/validate` owns the one document assessment path: XML preflight,
root/start selection, XSI type and nil handling, attributes, content, simple
values, hints, recovery, identity finalization, and reusable-session reset.
Transactions in [`session_start_transaction_test.go`](../internal/validate/session_start_transaction_test.go)
roll back XML/namespace, content, identity, and staged hint state on fatal start
failure; semantic-stop recovery retains only syntax lifecycle state. Structured
diagnostic construction remains in `xsderrors`, while locations and causes are
attached at the owning boundary.

The strongest seam evidence is
[`start_test.go`](../internal/validate/start_test.go),
[`recovery_test.go`](../internal/validate/recovery_test.go),
[`hints_test.go`](../internal/validate/hints_test.go),
[`session_error_limit_test.go`](../internal/validate/session_error_limit_test.go),
[`session_reset_test.go`](../internal/validate/session_reset_test.go),
[`xml_document_test.go`](../internal/validate/xml_document_test.go), and
[`xsderrors/errors.go`](../xsderrors/errors.go). The project cases
[`namespace-undeclared-root-can-be-assessed-by-xsi-type`](../tests/corpus/project/namespace-undeclared-root-can-be-assessed-by-xsi-type),
[`namespace-xsi-type-validates-derived-content`](../tests/corpus/project/namespace-xsi-type-validates-derived-content),
and [`runtime-qname-namespace-ordering-xsi-type`](../tests/corpus/project/runtime-qname-namespace-ordering-xsi-type)
anchor XSI selection, derivation, and namespace ordering. The public behavior
and complete differential corpus remain final-gate evidence.

## Completion boundary

These decisions align the implementation with the ownership and data-flow
contract in `ARCHITECTURE.md` and the reference consequences recorded in
`reference-comparison.md`. The targeted differential and flat-stream retention
probes are recorded above, but complete feature parity, broader retained-state
evidence, and the final paired-performance matrix still require the evidence
packet tracked by the unchecked rewrite-plan item. The `xs:redefine` decision
remains an explicit unsupported-scope boundary until a bounded complete
implementation and its corpus proof are available.

# Reference comparison for the rewrite

Evidence baseline: this library at `2764e554`, including the source checkout's
previously uncommitted changes. The original checkout is untouched. This report
records findings and design reasoning; `ARCHITECTURE.md` owns implemented
architecture and `rewrite-plan.md` owns outstanding work.

## What the references establish

| Reference and inspected revision | Relevant evidence | Consequence for this library |
| --- | --- | --- |
| libxml2, `c63248941708bc1d2e3a4292954593312212f6ca` | Separate schema parser and validation contexts; streaming validation and SAX integration; compiled particle automata; depth-local element assessment and scope-local identity state. | Preserve compile-once semantics and isolated document execution. Pointer-heavy schema graphs and a broad XML platform are not prerequisites for validation. |
| Xerces-J, `eac1058ff35d12ae12f11f7ff8b465e5ba1e55d9` | Grammar loading precedes event validation; content models specialize `all` and DFA cases; validation retains element stacks, scalar text, and identity tuples. | Precompute schema decisions. Bound each unavoidable retained dimension explicitly. Streaming does not imply constant memory for arbitrary keys, keyrefs, or scalar values. |
| .NET System.Xml, `eabca98e78db9f203ae1aa9418c0ad0cd4170cab` | Schema compilation publishes compiled information; the validator accepts explicit element/attribute/text/end events; bounded DFA construction can fall back to NFA execution. | Keep compilation and execution distinct. Make automaton complexity an explicit admission decision, with correctness independent of optimization choices. |
| xsdcpp, `a74bee270f3c0b71587bf4562b076558cf6cd632` | Schema-to-C++ generation specializes parsing, but its documented limitations include namespace-insensitive matching, incomplete occurrence/facet checks, and missing sequence-order validation. | Useful specialization example, unsuitable as the correctness or feature-parity oracle. Do not equate a narrower parser's throughput with complete XSD validation. |

libxml2 exposes schema construction and per-run validation as separate
capabilities, including `xmlSchemaValidateStream` and `xmlSchemaSAXPlug`.
Its element processing advances content automata and finalizes constraints on
element close. The transferable idea is the separation of reusable schema
knowledge from mutable validation state, rather than its C object layout.
Sources: [schema/validation API](https://github.com/gnome/libxml2/blob/c63248941708bc1d2e3a4292954593312212f6ca/include/libxml/xmlschemas.h),
[schema implementation](https://github.com/gnome/libxml2/blob/c63248941708bc1d2e3a4292954593312212f6ca/xmlschemas.c).

Xerces' `XSDFACM` describes exponential worst-case determinization and builds
transition tables ahead of validation. `XMLSchemaValidator` maintains active
element and identity state; its identity stores also expose linear duplicate
searches. Those are reasons to measure adversarial schema shapes and identity
cardinality, not just repeated small documents.
Sources: [XSDFACM](https://github.com/apache/xerces-j/blob/eac1058ff35d12ae12f11f7ff8b465e5ba1e55d9/src/org/apache/xerces/impl/xs/models/XSDFACM.java),
[XMLSchemaValidator](https://github.com/apache/xerces-j/blob/eac1058ff35d12ae12f11f7ff8b465e5ba1e55d9/src/org/apache/xerces/impl/xs/XMLSchemaValidator.java).

.NET's content compiler bounds DFA construction and uses NFA execution when
determinization exceeds its threshold. The public validation engine has an
explicit event protocol, whereas the validating reader adds read-ahead and
replay to satisfy richer reader APIs. A validation-only Go API does not need
those reader conveniences. Its typed identity tuples demonstrate why lexical
string equality alone is insufficient for keys and keyrefs.
Sources: [content compiler and execution](https://github.com/dotnet/runtime/blob/eabca98e78db9f203ae1aa9418c0ad0cd4170cab/src/libraries/System.Private.Xml/src/System/Xml/Schema/ContentValidator.cs),
[validator](https://github.com/dotnet/runtime/blob/eabca98e78db9f203ae1aa9418c0ad0cd4170cab/src/libraries/System.Private.Xml/src/System/Xml/Schema/XmlSchemaValidator.cs),
[validating reader](https://github.com/dotnet/runtime/blob/eabca98e78db9f203ae1aa9418c0ad0cd4170cab/src/libraries/System.Private.Xml/src/System/Xml/Core/XsdValidatingReader.cs),
[identity values](https://github.com/dotnet/runtime/blob/eabca98e78db9f203ae1aa9418c0ad0cd4170cab/src/libraries/System.Private.Xml/src/System/Xml/Schema/ConstraintStruct.cs).

xsdcpp explicitly describes its validation limitations. Its parser accepts a
complete character buffer, recursively parses elements, and maps names into
generated metadata. That contract differs from a bounded `io.Reader` validator;
it cannot establish a performance target without accounting for omitted work.
Sources: [README](https://github.com/craflin/xsdcpp/blob/a74bee270f3c0b71587bf4562b076558cf6cd632/README.md),
[parser](https://github.com/craflin/xsdcpp/blob/a74bee270f3c0b71587bf4562b076558cf6cd632/src/XmlParser.cpp).

## Comparison with the baseline

The existing library already has the central architecture demonstrated by the
general-purpose validators: compilation produces an immutable schema and each
validation owns its mutable state. It already has typed IDs, specialized content
models, typed values, namespace handling, bounded identity state, and explicit
resource limits. None of the references establishes that a wholesale rewrite
will inherently improve its performance. The fresh rewrite is the requested
deliverable; its superiority remains a measurement requirement.

Concrete differences worth changing:

- `internal/source.Source.Acquire` materializes opener-backed schemas with
  `io.ReadAll`; `loadedSchemaDocument.data` retains source bytes for identity
  comparisons. Instance validation streams, but source acquisition does not.
- `internal/compile.rawNode` retains XML-shaped names, attributes, text, child
  pointers, and namespace contexts. Chameleon expansion clones those trees.
  A typed schema representation can retain semantic declarations and references
  without retaining generic XML structure.
- Compilation constructs exported mutable `runtime.SchemaBuild` data across a
  package boundary; publication derives and audits several read projections.
  Keeping private construction with its published schema owner can remove that
  protocol and prevent cross-package mutation by construction.
- Small-schema compilation still allocates roughly 206 KiB and 625 objects in
  the initial local sample. Builtin construction and publication are concrete
  targets for profiling, not yet proven explanations for all of that cost.

The baseline `make test` passed. The initial performance sample used Go 1.27.0
on darwin/arm64, Apple M2 Max, six 200 ms samples. Raw output is retained locally
at `.lab/rewrite/baseline-bench.txt`. This is baseline evidence, not a rewrite
performance result.

## Design implications

Use a synchronous pull XML stream, compiled schema knowledge, and one concrete
document evaluator. No DOM, network loader, schema discovery from instance
hints, goroutine pipeline, or callback framework is necessary. Keep lexical
normalization and typed equality authoritative; generics cannot replace XSD's
runtime type system.

Budget compilation by admitted source bytes, semantic components, dependency
work, and automaton work/state. Budget validation by depth, attributes, token
and scalar bytes, identity state, diagnostics, and total input. Arbitrary reader
interruption stays with the caller, as in the baseline contract.

Compare complete compile/validate paths with identical schemas, documents,
limits, and diagnostics. Keep conformance expectations distinct from baseline
behavior: reference implementations disagree, and preserving a baseline bug is
not proof of standards conformance.

# XML stream comparison with Go JSON v2

Reviewed XSD revision: `d497d859`. Reference: installed Go 1.27.0
`encoding/json/jsontext` and `encoding/json/v2`, checked against the official
[design discussion](https://go.dev/blog/jsonv2-exp) and
[versioned API](https://pkg.go.dev/encoding/json/jsontext@go1.27.0).
[ARCHITECTURE.md](../ARCHITECTURE.md) owns XSD's current design; this document
records comparison evidence and transfer decisions.

## Transferable design

Go separates syntactic streaming in `jsontext` from Go-value semantics in
`json/v2`. XSD has the corresponding boundary: `internal/xmlstream` owns XML
syntax, namespace admission, borrowing, and document topology; schema compilation,
instance assessment, and formatting own their respective interpretations.
Creating additional packages solely to mirror JSON's names would not deepen
this boundary. [Go JSON v2 design](https://go.dev/blog/jsonv2-exp)

| Concern | JSON reference | XSD decision |
| --- | --- | --- |
| Borrowing | Tokens and raw values expire on subsequent decoder operations; cloning makes ownership explicit. | Keep reader-owned tokens, explicit value materialization, and consumer lifetime enforcement. |
| Grammar state | The decoder owns grammar progress and validates operations before advancing structural state. | Keep pending-token admission and namespace commit/rollback under `Reader`; callers cannot supply replacement tokens. |
| Semantics | Reflection and Go-value conversion live above syntax. | Keep datatypes, facets, XSI decisions, identity tuples, and diagnostic paths out of `xmlstream`. |
| Effect boundary | A concrete decoder consumes `io.Reader`. | Keep `Reader` concrete; an additional stream interface has no current substitution requirement. |
| Errors | Syntactic location is distinct from richer semantic context. | Keep neutral stream error kinds and line/column; consumers own category, path, and recovery policy. |

These borrowing and decoder contracts are documented by the
[Go 1.27 API](https://pkg.go.dev/encoding/json/jsontext@go1.27.0). XSD's
implementations are [document.go](../internal/xmlstream/document.go),
[namespace.go](../internal/xmlstream/namespace.go), and
[xmlstream_token.go](../internal/xmlstream/xmlstream_token.go); their consumers
are additionally checked by
[stream_boundary_test.go](../tests/stream_boundary_test.go).

## Differences that should remain

JSON object names are lexical strings. XML names require namespace expansion,
separate default-namespace rules for elements and attributes, reserved prefixes,
and duplicate checks on expanded attribute names. XSD's binding history and
active-prefix index serve different lifetimes: retained immutable contexts
support deferred QName work, while the live projection supports element admission.
Combining those lifetimes into a single name map would lose required behavior.

JSON offers a contiguous complete raw-value operation alongside token reads.
XSD's consumers require incremental element admission for namespace scope,
identity matching, recovery, and formatter source spans. A generic complete
element value would add retention and a second ownership contract without a
present consumer need. [Go 1.27 raw-value API](https://pkg.go.dev/encoding/json/jsontext@go1.27.0#Decoder.ReadValue)

XSD also has explicit input, token, attribute, depth, retained-capacity, and cache
bounds. Those requirements remain binding independently of the reference
library's available limit options. Splitting tokenizer, namespaces, and document
protocol into separate packages would move their atomic sequencing into callers.

The flat token representation permits irrelevant field combinations, but it is
internal and consumed by token kind within an enforced borrowed lifetime.
Interface-backed variants would add allocation and dispatch without evidence
of a defect they prevent. Prefer retaining the compact representation unless a
concrete misuse demonstrates a stronger type boundary is needed.

## Measurement-gated candidate

A reader-owned skip operation for opaque schema annotation payloads might reduce
consumer dispatch. It would still have to validate every nested namespace,
attribute, limit, and closing name. The complete affected path is typed schema
parsing through `Reader.Start`, `MatchEnd`, and `CommitEnd`. No implementation is
justified by the comparison alone: require annotation-heavy measurements and
opposing malformed-input tests before adding a second traversal operation.

The comparison found no material architecture defect. It informs, but does not
replace, the subsequent XML contract reviews recorded in
[architecture-review-iterations.md](architecture-review-iterations.md).

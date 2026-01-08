# Streaming XML Validation Plan

## Goal
- Validate XML without building a full DOM, so very large instance documents can be processed with memory bounded by depth and validation state.
- Instance validation is streaming-only; DOM stays only for XSD parsing (schemas are small).
- Preserve current validation behavior and W3C error codes where practical.

## Current State (DOM-based instance validation)
- `xsd.Schema.Validate` parses XML into `internal/xml.Document` and validates against the full tree (this path will be removed).
- Validator relies on random access: `Children`, `Parent`, `TextContent`, and XPath evaluation for identity constraints.
- Content model validation consumes the full children slice per element.
- `xsi:schemaLocation` hints are collected by traversing the entire tree before validation.

## Target Architecture (Streaming-only)
Parse XML with `encoding/xml.Decoder` (pull tokens, SAX-style event loop) and validate incrementally using a stack of element frames.
Each frame owns the minimal state needed to validate the current element and its children.
No DOM fallback for instance validation.

```
XML Reader -> xml.StreamDecoder -> validator.StreamRun -> compiled schema -> errors
```

## Required Changes (By Area)

### Public API (`xsd.go`)
- `Validate` and `ValidateFile` run the streaming validator (no DOM path).
- Optional: `ValidateWithOptions` to configure:
  - SchemaLocation policy (default: root-only hints).
  - Error recovery strategy (stop-on-error for element subtree vs best-effort).
- Default error recovery: stop-on-error per element subtree (matches current behavior).
- SchemaLocation policy type:
```go
type SchemaLocationPolicy int

const (
	SchemaLocationRootOnly SchemaLocationPolicy = iota // default
	SchemaLocationDocument
	SchemaLocationIgnore
)
```

### XML Parsing (`internal/xml`)
- Use `encoding/xml.Decoder.Token()` as the SAX-like event source:
  - Drive validation from `StartElement`, `EndElement`, and `CharData` tokens.
  - Keep a namespace scope stack for QName value resolution (prefixes in values are not expanded by `Token()`).
  - Use `Decoder.Skip()` for `processContents=skip` subtrees.
  - Use `Decoder.InputPos()` for line/column if needed by errors.
- Keep `internal/xml.Document` only for XSD parsing and tests that still need it.

#### XML Stream Wrapper Shape
Define a small wrapper that normalizes namespaces and provides stable event data:
- `type StreamDecoder struct { dec *xml.Decoder; nsStack []nsScope; attrBuf []Attr }`
- `type Event struct { Kind EventKind; Name QName; Attrs []Attr; Text []byte; Line int; Column int }`
- `type Attr struct { Namespace string; Local string; Value string }`
- `type elementID uint64` (monotonic per document; assigned on `StartElement`).
- `StreamDecoder` maintains a `nextID` counter and assigns `elementID` on each `StartElement`.
- `func (d *StreamDecoder) Next() (Event, error)` returns the next event:
  - `StartElement`:
    - Normalize `xmlns` attributes into the namespace scope (`xmlns`, `xmlns:prefix`).
    - Copy attribute values into `attrBuf` (token buffers are invalid after the next `Token()`).
    - Return resolved element `QName` using `StartElement.Name.Space`/`Local`.
  - `EndElement`:
    - Pop namespace scope.
  - `CharData`:
    - Copy bytes if the validator needs to buffer text (string-value).
- `func (d *StreamDecoder) CurrentPos() (line, column int)` uses `Decoder.InputPos()`.

Notes:
- `Token()` resolves element/attribute names to namespace URIs when prefixes are in scope, but QName values (e.g., `xsi:type="p:T"`) still require explicit prefix resolution from the scope stack.
- `Token()` expands empty elements into start/end pairs; streaming content model logic must handle that as normal element open/close.
- `Decoder.InputPos()` returns the line/column of the end of the most recently returned token; for start positions, store the previous position or accept end-of-token locations in errors.
- Namespace scope is owned by `StreamDecoder`; element frames reference the current scope depth (no cloning).
- When `processContents=skip` is used, `Decoder.Skip()` consumes the subtree EndElement; the validator must pop the frame immediately and resume at the correct parent depth.

### Content Model Validation (`internal/grammar/contentmodel`)
- Introduce a streaming runner for DFA-based content models:
  - `Automaton.NewStreamValidator(matcher, wildcards)` returns stateful validator.
  - `Feed(childQName)` returns `MatchResult` (element vs wildcard, processContents).
  - `Close()` validates final min/max/group counters.
- Add streaming validator for all-groups (unordered):
  - Track per-element occurrences; detect duplicates and missing required elements on `Close()`.
- `Close()` is called at `EndElement` of the element that owns the content model (end of its content).
- On `Close()` errors, stop validating further children of that element (stop-on-error per subtree), but continue parsing to the element end to keep the stream balanced.

### Validator Core (`internal/validator`)
- Replace DOM validator with a streaming validator (`stream.go`) and event-driven flow:
  - Maintain a stack of `elementFrame`:
    - ID (monotonic `elementID` assigned at `StartElement`).
    - QName, declaration, effective type, `xsi:nil` state.
    - Content model runner (if any).
    - Flags for mixed/element-only content.
    - Text buffer for elements that need string-value (simple content, mixed+fixed, identity constraints).
    - Namespace scope for QName resolution (xsi:type, NOTATION, identity constraints).
  - On `StartElement`:
    - Resolve element declaration (including substitution groups).
    - Handle `xsi:type` and nillable logic.
    - Validate attributes and collect ID values.
    - Initialize content model runner for parent and validate this child against it.
  - On `CharData`:
    - Append to current element's text buffer.
    - Track non-whitespace direct text for element-only constraints.
  - On `EndElement`:
    - Finalize content model (`Close`) and report missing elements.
    - Validate simple content, fixed/default values, and IDREFs.
    - Pop frame and continue.
- If a subtree is skipped (wildcard `processContents=skip`), pop its frame immediately and do not expect an `EndElement` event for it.
- Retain helper logic for simple types, facets, and error formatting.
- Buffering rules (set `needStringValue` on `StartElement`):
  - Always for simple content types, complex types with simpleContent, and mixed types with fixed/default constraints.
  - For identity constraints:
    - If a selector has matched an ancestor and any field uses descendant axes or wildcards, set `needStringValue` for all elements in that subtree (safe, possibly over-buffering).
    - Otherwise, set `needStringValue` only when a field matcher marks this element as a potential field target.
  - For element-only content, only buffer when `needStringValue` is set (otherwise track direct text for whitespace checks only).

### Identity Constraints (`internal/validator/xpath`, `internal/validator/identity`)
- DOM XPath evaluator is not usable in streaming. Replace with streaming path matchers:
  - Precompile selector/field XPath expressions into step sequences (child/descendant axes).
  - Maintain active matches per constraint using the element stack.
  - Capture field values when selected elements close (string-value).
  - Keep key/unique tables per constraint scope and check keyrefs on scope end.
- Extend `grammar.CompiledConstraint` to store compiled selector/field steps to avoid re-parsing.
- XPath subset (must match current implementation, no predicates):
  - `Path  := Step ( "/" Step )*`
  - `Step  := Axis? NodeTest | AttributeStep`
  - `Axis  := "child::" | "descendant::" | "descendant-or-self::" | "self::" | "//" | ".//"`
  - `NodeTest := QName | "*" | "child::*" | "descendant::*" | "descendant-or-self::*" | "self::*"`
  - `AttributeStep := "@QName" | "@*" | "attribute::QName" | "attribute::*" | Path "/@QName" | Path "/attribute::QName"`
  - `Union := Path ( "|" Path )*`
  - Predicates (`[...]`), functions, and axes outside the list are rejected at schema load with a clear error.
  - Error includes constraint name, XPath, and schema location (file/line).
- Union handling: compile each branch and evaluate in parallel; de-duplicate by element instance ID on selection.
- Attribute defaults: when an attribute is selected but not present, use schema default/fixed if defined; otherwise treat as absent.
- Descendant axes: maintain active match states per open element to allow matches at arbitrary depth.
- Element instance ID for de-dup: use a monotonic `elementID` assigned at `StartElement` and stored in the frame; unions de-dup on this ID.

#### Constraint Scope Lifecycle
- Scope starts at the element instance whose declaration carries the constraint.
- Selector evaluation is relative to that element; field evaluation is relative to each selector match.
- Scope ends at `EndElement` of that element; key/unique tables are finalized, then keyrefs are checked.

### Schema Location Hints (`xsi:schemaLocation`)
Streaming-only validation cannot scan ahead without buffering, so a deterministic policy is required:
1. Default (`SchemaLocationRootOnly`):
   - Apply schemaLocation hints found on the root element only.
   - Single-pass, no seek required.
2. Document (`SchemaLocationDocument`):
   - If the reader implements `io.ReadSeeker`, run a prepass to collect hints, then validate.
   - If not seekable and hints are present, return an error.
3. Ignore (`SchemaLocationIgnore`):
   - Ignore all schemaLocation hints.

### Namespace Handling
- Replace ancestor scanning with a namespace scope stack:
  - Push namespace mappings on `StartElement`.
  - Pop on `EndElement`.
  - Use the stack to resolve QName values (xsi:type, NOTATION, identity constraint fields).
  - Track default namespace (`xmlns="..."`) separately from prefixed mappings.
  - Allow prefix redefinition; lookups walk the stack from top to bottom.

Example data model:
- `type nsScope struct { defaultNS string; prefixes map[string]string }`
- `type nsStack struct { scopes []nsScope }`
- `func (s *nsStack) Lookup(prefix string) (string, bool)`:
  - if prefix == "" then return nearest `defaultNS` (if set), else empty.
  - else search `prefixes` from top scope to bottom.
  - if not found, return empty and mark unresolved prefix error where required.
Frames store `scopeDepth` (index into `nsStack.scopes`) to resolve QName values at the correct scope.

### Error Reporting and Recovery
- Collect violations in a slice during validation.
- Stop-on-error per element subtree: once a subtree is invalid, stop validating further descendants in that subtree, continue parsing to the subtree end, then resume validation for siblings.
- Return `errors.ValidationList` with all collected violations encountered before any subtree was aborted.
- Line/column positions reflect end-of-token by default; document this or store start positions for better UX.

### Thread Safety
- `Schema` and compiled grammar are immutable after load.
- Each validation run allocates its own state (frames, matchers, ID tables).
- `Schema.Validate` is safe to call concurrently from multiple goroutines once a schema is loaded.

### Migration Notes
- Streaming-only validation is a breaking change; do it in the next alpha release (no compatibility path required).
- Default behavior is root-only hints; document-wide hints require `SchemaLocationDocument`.

## Implementation Steps
1. Add content model streaming runners (automaton + all-group) with unit tests.
2. Implement streaming XML decoder wrapper and namespace scope stack in `internal/xml`.
3. Build streaming validator core (element stack, attributes, text, content model) without identity constraints.
4. Add streaming ID/IDREF collection and simple type validation.
5. Identity constraints in phases:
   - 5a. XPath compiler with subset enforcement (reject unsupported syntax at schema load).
   - 5b. Streaming selector matchers (union + descendant axes).
   - 5c. Streaming field matchers and key/keyref/unique tables.
6. Implement schemaLocation policy (default error on non-seekable with hints) and switch `Validate` to streaming-only (remove DOM validator, no fallback).
7. Add large XML fixtures for memory and throughput validation.

## Testing Plan (No Mocks)
- Unit tests for content model streaming runners (automaton and all-group).
- Table-driven validator tests using streaming only with expected violations.
- W3C tests via `w3c/w3c_test.go` filtered for XSD 1.0 (same subset as current DOM path).
- Large XML fixture tests for memory usage and streaming correctness (real files, not mocks).
  - Track RSS and Go heap with `runtime.ReadMemStats` and OS-level RSS (documented in test output).
- Add tests:
  - Non-seekable reader with schemaLocation hints returns deterministic error.
  - Schema with unsupported XPath in identity constraints fails at load time with clear message.

## Gaps To Resolve (Streaming-only)
- String-value semantics: ensure concatenation of descendant text matches current `TextContent` behavior for simple content, mixed+fixed, and identity constraints.
- Wildcards with `processContents=skip`: skip validation of the entire subtree while still parsing for well-formedness.
- Identity constraints: streaming XPath matcher must support union, `//`, `.//`, wildcard steps, and attribute selections with defaults.
- `xsi:schemaLocation` hints: implement the error path for non-seekable readers with hints; ensure opt-out is explicit.
- Namespace resolution: default namespace and prefix redefinitions must be scoped correctly for `xsi:type`, NOTATION, and identity fields.

## String-Value Semantics (Test Matrix)
Definition: string-value is the concatenation of all character data nodes in document order
within the element subtree, matching `Document.TextContent`.
- `<a/>` => `""`
- `<a>text</a>` => `"text"`
- `<a>text<b/>more</a>` => `"textmore"`
- `<a> text <b/> more </a>` => `" text  more "`
- `<a><b>t</b><c>u</c></a>` => `"tu"`
Notes:
- Element-only content still checks direct text for non-whitespace; string-value is only used when the type allows text.
- Whitespace normalization happens after string-value assembly, via type-specific `whiteSpace` facet handling.

## Performance Considerations
- Avoid per-element allocations: pool `elementFrame` structs, reuse slices in stacks and matchers.
- Only buffer string-value when required; do not accumulate text for element-only content.
- Use `[]byte` buffers for text, convert to string only at validation boundaries.
- Keep content model runner state array-based (avoid maps in hot path).
- For ID/IDREF, consider a pending map to resolve refs as IDs appear to reduce memory.
- Keep identity constraint matchers incremental to avoid O(depth * constraints * steps) per element.
Targets (baseline goals, not guarantees):
- Validate a 100MB XML instance with < 150MB RSS increase over baseline.
- Sustained throughput >= 10 MB/s on a typical dev laptop for large, valid documents (lower bound; expect higher for simple docs).

## Open Questions / Tradeoffs
- Error recovery: default to stop-on-error per element subtree (matches current behavior), optional best-effort later.
- Identity constraints: full spec support in streaming-only or staged rollout with clear limits.

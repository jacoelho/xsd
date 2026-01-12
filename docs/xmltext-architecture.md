# xmltext architecture

xmltext is the zero-copy XML tokenizer used by the repository. It focuses on
XML 1.0 well-formedness and streaming performance and leaves namespace
resolution and semantic modeling to higher layers.

## Layering

io.Reader
  |
  v
+xmltext.Decoder (syntax, spans, well-formedness)
  |
  v
+internal/xml StreamDecoder (namespace resolution, events)
  |
  v
+validator / DOM parsing

The decoder is a low-level, allocation-light component. It does not resolve
namespaces or build DOM nodes. It only tokenizes and validates syntax.

## Buffer model and spans

- The decoder owns a sliding buffer and returns spans into that buffer.
- Spans are views: they are valid until the next decoder call.
- When the buffer compacts or grows, previous spans are invalidated.
- Scratch buffers are used for transformed text (entity expansion and
  coalesced char data) to avoid copying into the main buffer.

Compaction details:
- The buffer can compact to reclaim space once earlier bytes are no longer
  needed.
- Coalesced char data pins a minimum offset so raw spans stay anchored to the
  original input while the coalesce buffer accumulates text.

## Token model

Token fields are spans into decoder buffers:
- Raw spans are slices into the main input buffer and preserve original bytes.
- Text spans may point at the main buffer or a scratch buffer if entity
  expansion is enabled.
- When CoalesceCharData(true) is set, adjacent CharData/CDATA tokens are merged
  into a single CharData token; its Text span points to the coalesce buffer,
  while Raw spans remain anchored to the main buffer.

## Entity handling

- ResolveEntities(false): Text spans point into the original buffer and retain
  raw entity references. Raw spans match Text spans.
- ResolveEntities(true): Text spans contain unescaped bytes in a scratch buffer.
  Raw spans remain the original input bytes.
- Entity parsing applies to character data and attribute values. CDATA is never
  entity-expanded.

## Namespace handling

xmltext does not interpret prefixes or manage namespace scopes. Namespace
resolution is performed by internal/xml, which consumes xmltext tokens and
applies XML Namespaces rules.

## Error model

- Syntax errors return a typed error that includes line, column, and offset.
- After the first error, the decoder stays in an error state until Reset is
  called.

## Options and limits

Common options:
- ResolveEntities(bool)
- CoalesceCharData(bool)
- TrackLineColumn(bool)
- EmitComments(bool), EmitPI(bool), EmitDirectives(bool)
- MaxDepth(int), MaxAttrs(int), MaxTokenSize(int)
- MaxQNameInternEntries(int), MaxNamespaceInternEntries(int)

Limits are enforced during parsing to guard against hostile inputs.

## Performance notes

- ASCII lookup tables are used for name and whitespace classification.
- Char data scanning uses fast searches for '<' and entity markers.
- Tokenization is streaming and single-pass; no DOM allocation in xmltext.
- Scratch buffers and interning are reused within a decoder instance to keep
  allocations low.

## Scope

xmltext enforces XML 1.0 well-formedness. It does not implement DTD parsing or
external entity resolution. Charset support beyond UTF-8/UTF-16 requires an
explicit charset reader.

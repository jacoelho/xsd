# xmltext

xmltext is a streaming XML 1.0 tokenizer optimized for zero-copy parsing. It is
used by internal/xml and the validator to parse XML without building a DOM.

## Goals

- fast, streaming tokenization over io.Reader
- minimal allocations with span-based access
- explicit options for entity expansion and token emission

## Usage

```go
dec := xmltext.NewDecoder(r,
    xmltext.ResolveEntities(true),
    xmltext.CoalesceCharData(true),
)

for {
    tok, err := dec.ReadToken()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }

    if tok.Kind == xmltext.KindStartElement {
        name := dec.SpanBytes(tok.Name.Local)
        // use name within the lifetime of this token
        _ = name
    }
}
```

## Examples

On-demand entity expansion for text:

```go
dec := xmltext.NewDecoder(r,
    xmltext.ResolveEntities(false),
    xmltext.CoalesceCharData(true),
)

for {
    tok, err := dec.ReadToken()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    if tok.Kind != xmltext.KindCharData {
        continue
    }

    text := dec.SpanBytes(tok.Text)
    if tok.TextNeeds {
        var buf []byte
        buf, err = xmltext.UnescapeInto(buf, tok.Text)
        if err != nil {
            return err
        }
        text = buf
    }
    _ = text
}
```

Attribute values without forcing expansion:

```go
dec := xmltext.NewDecoder(r, xmltext.ResolveEntities(false))
for {
    tok, err := dec.ReadToken()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    if tok.Kind != xmltext.KindStartElement {
        continue
    }

    for i, attr := range tok.Attrs {
        name := dec.SpanBytes(attr.Name.Local)
        value := dec.SpanBytes(attr.ValueSpan)
        if tok.AttrNeeds[i] {
            var buf []byte
            buf, err = xmltext.UnescapeInto(buf, attr.ValueSpan)
            if err != nil {
                return err
            }
            value = buf
        }
        _ = name
        _ = value
    }
}
```

Retaining span data beyond the next decoder call:

```go
tok, err := dec.ReadToken()
if err != nil {
    return err
}
stable := xmltext.CopySpan(nil, tok.Name.Local)
_ = stable
```

## SAX-Style Struct Unmarshaling

Unlike `encoding/xml.Unmarshal` which builds a DOM, xmltext streams tokens for
manual struct population. Track element context and populate fields on events:

```go
type Book struct {
    Title  string
    Author string
    Year   string
}

func UnmarshalBook(r io.Reader) (Book, error) {
    dec := xmltext.NewDecoder(r,
        xmltext.ResolveEntities(true),
        xmltext.CoalesceCharData(true),
    )

    var book Book
    var current string // tracks current element

    for {
        tok, err := dec.ReadToken()
        if err == io.EOF {
            break
        }
        if err != nil {
            return Book{}, err
        }

        switch tok.Kind {
        case xmltext.KindStartElement:
            current = dec.SpanString(tok.Name.Local)
        case xmltext.KindCharData:
            text := dec.SpanString(tok.Text)
            switch current {
            case "title":
                book.Title = text
            case "author":
                book.Author = text
            case "year":
                book.Year = text
            }
        case xmltext.KindEndElement:
            current = ""
        }
    }
    return book, nil
}
```

For nested structures, use a stack or state machine to track depth:

```go
type Library struct {
    Books []Book
}

func UnmarshalLibrary(r io.Reader) (Library, error) {
    dec := xmltext.NewDecoder(r,
        xmltext.ResolveEntities(true),
        xmltext.CoalesceCharData(true),
    )

    var lib Library
    var current Book
    var inBook bool
    var field string

    for {
        tok, err := dec.ReadToken()
        if err == io.EOF {
            break
        }
        if err != nil {
            return Library{}, err
        }

        switch tok.Kind {
        case xmltext.KindStartElement:
            name := dec.SpanString(tok.Name.Local)
            if name == "book" {
                inBook = true
                current = Book{}
            } else if inBook {
                field = name
            }
        case xmltext.KindCharData:
            if !inBook {
                continue
            }
            text := dec.SpanString(tok.Text)
            switch field {
            case "title":
                current.Title = text
            case "author":
                current.Author = text
            case "year":
                current.Year = text
            }
        case xmltext.KindEndElement:
            name := dec.SpanString(tok.Name.Local)
            if name == "book" {
                lib.Books = append(lib.Books, current)
                inBook = false
            }
            field = ""
        }
    }
    return lib, nil
}
```

This approach avoids reflection and DOM allocation, giving full control over
parsing. Use `SkipValue()` to skip unwanted subtrees efficiently.

## Span lifetimes

Token spans are views into decoder buffers. They are valid only until the next
ReadToken/ReadValue/SkipValue call. If you need to keep data, copy it using
CopySpan or Clone.

Text and raw spans may reference different buffers when entity expansion or
char-data coalescing is enabled. Raw spans always refer to the original input
buffer.

## ReadValue

ReadValue returns a raw subtree or token payload. When ResolveEntities(true) is
set, entity expansion is applied while preserving original raw spans internally.

## Footguns

- spans and token slices are reused; do not keep them after the next call
- Text spans can point at scratch buffers when ResolveEntities(true) or
  CoalesceCharData(true) are enabled; Raw spans always point at the input buffer
- CDATA and CharData merge into a single CharData token when coalescing is on
- SpanString can allocate unless the backing buffer is marked stable

## Options

Common options include:
- ResolveEntities
- CoalesceCharData
- TrackLineColumn
- EmitComments, EmitPI, EmitDirectives
- MaxDepth, MaxAttrs, MaxTokenSize

MaxTokenSize is unlimited by default. Set it when parsing untrusted input to
cap memory growth; FastValidation does not set this limit.

See docs/xmltext-architecture.md for the design and buffer model.

# xmltext

xmltext is a streaming XML 1.0 tokenizer optimized for zero-copy parsing. It is
used by internal/xml and the validator to parse XML without building a DOM.

## Goals

- fast, streaming tokenization over io.Reader
- minimal allocations with caller-owned buffers
- explicit options for entity expansion and token emission

## Usage

```go
dec := xmltext.NewDecoder(r,
    xmltext.ResolveEntities(true),
    xmltext.CoalesceCharData(true),
)
var tok xmltext.Token
var buf xmltext.TokenBuffer

for {
    err := dec.ReadTokenInto(&tok, &buf)
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }

    if tok.Kind == xmltext.KindStartElement {
        name := tok.Name
        // use name within the lifetime of this buffer
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
var tok xmltext.Token
var buf xmltext.TokenBuffer
scratch := make([]byte, 256)

for {
    err := dec.ReadTokenInto(&tok, &buf)
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    if tok.Kind != xmltext.KindCharData {
        continue
    }

    text := tok.Text
    if tok.TextNeeds {
        for {
            n, err := dec.UnescapeInto(scratch, tok.Text)
            if err == io.ErrShortBuffer {
                scratch = make([]byte, len(scratch)*2+len(tok.Text))
                continue
            }
            if err != nil {
                return err
            }
            text = scratch[:n]
            break
        }
    }
    _ = text
}
```

Attribute values without forcing expansion:

```go
dec := xmltext.NewDecoder(r, xmltext.ResolveEntities(false))
var tok xmltext.Token
var buf xmltext.TokenBuffer
scratch := make([]byte, 256)

for {
    err := dec.ReadTokenInto(&tok, &buf)
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    if tok.Kind != xmltext.KindStartElement {
        continue
    }

    for _, attr := range tok.Attrs {
        name := attr.Name
        value := attr.Value
        if attr.ValueNeeds {
            for {
                n, err := dec.UnescapeInto(scratch, attr.Value)
                if err == io.ErrShortBuffer {
                    scratch = make([]byte, len(scratch)*2+len(attr.Value))
                    continue
                }
                if err != nil {
                    return err
                }
                value = scratch[:n]
                break
            }
        }
        _ = name
        _ = value
    }
}
```

Retaining token data beyond the next decoder call:

```go
var tok xmltext.Token
var buf xmltext.TokenBuffer
err := dec.ReadTokenInto(&tok, &buf)
if err != nil {
    return err
}
stable := append([]byte(nil), tok.Name...)
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
    var tok xmltext.Token
    var buf xmltext.TokenBuffer

    for {
        err := dec.ReadTokenInto(&tok, &buf)
        if err == io.EOF {
            break
        }
        if err != nil {
            return Book{}, err
        }

        switch tok.Kind {
        case xmltext.KindStartElement:
            current = string(tok.Name)
        case xmltext.KindCharData:
            text := string(tok.Text)
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
    var tok xmltext.Token
    var buf xmltext.TokenBuffer

    for {
        err := dec.ReadTokenInto(&tok, &buf)
        if err == io.EOF {
            break
        }
        if err != nil {
            return Library{}, err
        }

        switch tok.Kind {
        case xmltext.KindStartElement:
            name := string(tok.Name)
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
            text := string(tok.Text)
            switch field {
            case "title":
                current.Title = text
            case "author":
                current.Author = text
            case "year":
                current.Year = text
            }
        case xmltext.KindEndElement:
            name := string(tok.Name)
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

## Buffer lifetimes

Token slices are backed by the caller-provided TokenBuffer and are overwritten
on the next ReadTokenInto call that reuses the buffer. Copy slices if you need
to keep them.

## ReadValueInto

ReadValueInto writes the next subtree or token payload into dst and returns the
number of bytes written. When ResolveEntities(true) is set, entity expansion is
applied. It returns io.ErrShortBuffer if dst is too small.

## Footguns

- token slices are reused; copy them if you need to keep data past the next call
- ReadTokenInto overwrites the TokenBuffer contents every time
- ReadValueInto writes into dst; use the returned length to slice the buffer
- CDATA and CharData merge into a single CharData token when coalescing is on
- ResolveEntities(false) leaves entity references in Text/Attr values

## Options

Common options include:
- ResolveEntities
- Strict
- CoalesceCharData
- TrackLineColumn
- EmitComments, EmitPI, EmitDirectives
- MaxDepth, MaxAttrs, MaxTokenSize

MaxTokenSize is unlimited by default. Set it when parsing untrusted input to
cap memory growth; tokens exactly MaxTokenSize bytes long are allowed.
FastValidation does not set this limit.

Strict enforces XML declaration attribute ordering and values when present.

See docs/xmltext-architecture.md for the design and buffer model.

package xsdxml

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/pkg/xmltext"
)

// EventKind identifies the kind of streaming XML event.
type EventKind int

const (
	EventStartElement EventKind = iota
	EventEndElement
	EventCharData
)

const streamDecoderBufferSize = 256 * 1024
const streamDecoderAttrCapacity = 8

// elementID is a monotonic identifier assigned per document.
type elementID uint64

// Event represents a single streaming XML token.
// Attrs and Text are only valid until the next Next call.
type Event struct {
	Name       types.QName
	Attrs      []Attr
	Text       []byte
	Kind       EventKind
	Line       int
	Column     int
	ID         elementID
	ScopeDepth int
}

// StreamDecoder provides a streaming XML event interface with namespace tracking.
type StreamDecoder struct {
	dec        *xmltext.Decoder
	names      *qnameCache
	ns         nsStack
	attrBuf    []Attr
	elemStack  []types.QName
	valueBuf   []byte
	nsBuf      []byte
	tok        xmltext.Token
	nextID     elementID
	pendingPop bool
	lastLine   int
	lastColumn int
}

// NewStreamDecoder creates a new streaming decoder for the reader.
func NewStreamDecoder(r io.Reader) (*StreamDecoder, error) {
	if r == nil {
		return nil, fmt.Errorf("nil XML reader")
	}
	reader := bufio.NewReaderSize(r, streamDecoderBufferSize)
	dec := xmltext.NewDecoder(reader,
		xmltext.ResolveEntities(false),
		xmltext.CoalesceCharData(true),
		xmltext.EmitComments(false),
		xmltext.EmitPI(false),
		xmltext.EmitDirectives(false),
		xmltext.TrackLineColumn(true),
		xmltext.BufferSize(streamDecoderBufferSize),
	)
	if dec == nil {
		return nil, fmt.Errorf("nil XML decoder")
	}
	return &StreamDecoder{
		dec:      dec,
		names:    newQNameCache(),
		valueBuf: make([]byte, 0, 256),
		nsBuf:    make([]byte, 0, 128),
		tok: xmltext.Token{
			Attrs:        make([]xmltext.AttrSpan, 0, streamDecoderAttrCapacity),
			AttrNeeds:    make([]bool, 0, streamDecoderAttrCapacity),
			AttrRaw:      make([]xmltext.Span, 0, streamDecoderAttrCapacity),
			AttrRawNeeds: make([]bool, 0, streamDecoderAttrCapacity),
		},
	}, nil
}

// Next returns the next XML event.
func (d *StreamDecoder) Next() (Event, error) {
	if d == nil || d.dec == nil {
		return Event{}, fmt.Errorf("nil XML decoder")
	}
	if d.names == nil {
		d.names = newQNameCache()
	}
	if d.pendingPop {
		d.ns.pop()
		d.pendingPop = false
	}

	for {
		if err := d.dec.ReadTokenInto(&d.tok); err != nil {
			return Event{}, err
		}
		tok := &d.tok
		line, column := tok.Line, tok.Column
		d.lastLine = line
		d.lastColumn = column
		d.valueBuf = d.valueBuf[:0]

		switch tok.Kind {
		case xmltext.KindStartElement:
			scope, err := collectNamespaceScope(d, tok)
			if err != nil {
				return Event{}, wrapSyntaxError(d.dec, line, column, err)
			}
			scopeDepth := d.ns.push(scope)
			name, err := d.resolveElementName(tok.Name, scopeDepth, line, column)
			if err != nil {
				return Event{}, err
			}

			attrs := tok.Attrs
			if cap(d.attrBuf) < len(attrs) {
				d.attrBuf = make([]Attr, 0, len(attrs))
			} else {
				d.attrBuf = d.attrBuf[:0]
			}
			for i, attr := range attrs {
				attrNamespace, attrLocal, err := resolveAttrName(d.dec, &d.ns, attr.Name, scopeDepth, line, column)
				if err != nil {
					return Event{}, err
				}
				value, err := d.attrValueString(attr.ValueSpan, attrDecodeMode(tok, i))
				if err != nil {
					return Event{}, wrapSyntaxError(d.dec, line, column, err)
				}
				d.attrBuf = append(d.attrBuf, Attr{
					namespace: attrNamespace,
					local:     attrLocal,
					value:     value,
				})
			}

			id := d.nextID
			d.nextID++
			d.elemStack = append(d.elemStack, name)
			return Event{
				Kind:       EventStartElement,
				Name:       name,
				Attrs:      d.attrBuf,
				Line:       line,
				Column:     column,
				ID:         id,
				ScopeDepth: scopeDepth,
			}, nil

		case xmltext.KindEndElement:
			scopeDepth := d.ns.depth() - 1
			name, err := d.popElementName()
			if err != nil {
				return Event{}, err
			}
			d.pendingPop = true
			return Event{
				Kind:       EventEndElement,
				Name:       name,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}, nil

		case xmltext.KindCharData, xmltext.KindCDATA:
			text, err := d.textBytes(tok)
			if err != nil {
				return Event{}, wrapSyntaxError(d.dec, line, column, err)
			}
			return Event{
				Kind:       EventCharData,
				Text:       text,
				Line:       line,
				Column:     column,
				ScopeDepth: d.ns.depth() - 1,
			}, nil
		}
	}
}

// SkipSubtree skips the current element subtree after a StartElement event.
func (d *StreamDecoder) SkipSubtree() error {
	if d == nil || d.dec == nil {
		return fmt.Errorf("nil XML decoder")
	}
	if d.pendingPop {
		d.ns.pop()
		d.pendingPop = false
	}
	if err := d.dec.SkipValue(); err != nil {
		return err
	}
	if len(d.elemStack) > 0 {
		d.elemStack = d.elemStack[:len(d.elemStack)-1]
	}
	d.ns.pop()
	return nil
}

// CurrentPos returns the line and column of the most recent token.
func (d *StreamDecoder) CurrentPos() (line, column int) {
	if d == nil {
		return 0, 0
	}
	return d.lastLine, d.lastColumn
}

// LookupNamespace resolves a prefix at a given scope depth.
func (d *StreamDecoder) LookupNamespace(prefix string, depth int) (string, bool) {
	if d == nil {
		return "", false
	}
	return d.ns.lookup(prefix, depth)
}

func (d *StreamDecoder) resolveElementName(name xmltext.QNameSpan, depth, line, column int) (types.QName, error) {
	local := spanString(d.dec, name.Local)
	if name.HasPrefix {
		prefix := spanString(d.dec, name.Prefix)
		namespace, ok := d.ns.lookup(prefix, depth)
		if !ok {
			return types.QName{}, unboundPrefixError(d.dec, line, column)
		}
		return d.names.intern(namespace, local), nil
	}
	namespace, _ := d.ns.lookup("", depth)
	return d.names.intern(namespace, local), nil
}

func (d *StreamDecoder) popElementName() (types.QName, error) {
	if len(d.elemStack) == 0 {
		return types.QName{}, fmt.Errorf("unexpected end element")
	}
	name := d.elemStack[len(d.elemStack)-1]
	d.elemStack = d.elemStack[:len(d.elemStack)-1]
	return name, nil
}

func (d *StreamDecoder) attrValueString(span xmltext.Span, mode spanDecodeMode) (string, error) {
	var value string
	var err error
	if mode == spanDecodeCopy {
		if d.dec == nil {
			return "", nil
		}
		data := d.dec.SpanBytes(span)
		if len(data) == 0 {
			return "", nil
		}
		return unsafeString(data), nil
	}
	d.valueBuf, value, err = appendSpanString(d.valueBuf, span, mode)
	return value, err
}

func (d *StreamDecoder) namespaceValueString(span xmltext.Span, mode spanDecodeMode) (string, error) {
	var value string
	var err error
	d.nsBuf, value, err = appendSpanString(d.nsBuf, span, mode)
	return value, err
}

func (d *StreamDecoder) textBytes(tok *xmltext.Token) ([]byte, error) {
	if !tok.TextNeeds {
		return d.dec.SpanBytes(tok.Text), nil
	}
	out, err := xmltext.UnescapeInto(d.valueBuf, tok.Text)
	if err != nil {
		d.valueBuf = d.valueBuf[:0]
		return nil, err
	}
	d.valueBuf = out
	return d.valueBuf, nil
}

func attrDecodeMode(tok *xmltext.Token, index int) spanDecodeMode {
	if index >= 0 && index < len(tok.AttrNeeds) && tok.AttrNeeds[index] {
		return spanDecodeUnescape
	}
	return spanDecodeCopy
}

func wrapSyntaxError(dec *xmltext.Decoder, line, column int, err error) error {
	if err == nil {
		return nil
	}
	var syntaxErr *xmltext.SyntaxError
	if errors.As(err, &syntaxErr) {
		return err
	}
	if dec == nil {
		return err
	}
	return &xmltext.SyntaxError{
		Offset: dec.InputOffset(),
		Line:   line,
		Column: column,
		Path:   dec.StackPath(nil),
		Err:    err,
	}
}

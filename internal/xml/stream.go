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
	tokBuf     xmltext.TokenBuffer
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
	)
	if dec == nil {
		return nil, fmt.Errorf("nil XML decoder")
	}
	return &StreamDecoder{
		dec:      dec,
		names:    newQNameCache(),
		valueBuf: make([]byte, 0, 256),
		nsBuf:    make([]byte, 0, 128),
		tokBuf: xmltext.TokenBuffer{
			Attrs:     make([]xmltext.Attr, 0, streamDecoderAttrCapacity),
			AttrName:  make([]byte, 0, 256),
			AttrValue: make([]byte, 0, 256),
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
		if err := d.dec.ReadTokenInto(&d.tok, &d.tokBuf); err != nil {
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
			for _, attr := range attrs {
				attrNamespace, attrLocal, err := resolveAttrName(d.dec, &d.ns, attr.Name, scopeDepth, line, column)
				if err != nil {
					return Event{}, err
				}
				value, err := d.attrValueString(attr.Value, attr.ValueNeeds)
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

func (d *StreamDecoder) resolveElementName(name []byte, depth, line, column int) (types.QName, error) {
	prefix, local, hasPrefix := splitQName(name)
	localName := string(local)
	if hasPrefix {
		prefixName := string(prefix)
		namespace, ok := d.ns.lookup(prefixName, depth)
		if !ok {
			return types.QName{}, unboundPrefixError(d.dec, line, column)
		}
		return d.names.intern(namespace, localName), nil
	}
	namespace, _ := d.ns.lookup("", depth)
	return d.names.intern(namespace, localName), nil
}

func (d *StreamDecoder) popElementName() (types.QName, error) {
	if len(d.elemStack) == 0 {
		return types.QName{}, fmt.Errorf("unexpected end element")
	}
	name := d.elemStack[len(d.elemStack)-1]
	d.elemStack = d.elemStack[:len(d.elemStack)-1]
	return name, nil
}

func (d *StreamDecoder) attrValueString(value []byte, needsUnescape bool) (string, error) {
	if !needsUnescape {
		return unsafeString(value), nil
	}
	start := len(d.valueBuf)
	out, err := unescapeIntoBuffer(d.dec, d.valueBuf, start, value)
	if err != nil {
		d.valueBuf = d.valueBuf[:start]
		return "", err
	}
	d.valueBuf = out
	if len(out) == start {
		return "", nil
	}
	return unsafeString(out[start:]), nil
}

func (d *StreamDecoder) namespaceValueString(value []byte, needsUnescape bool) (string, error) {
	start := len(d.nsBuf)
	if needsUnescape {
		out, err := unescapeIntoBuffer(d.dec, d.nsBuf, start, value)
		if err != nil {
			d.nsBuf = d.nsBuf[:start]
			return "", err
		}
		d.nsBuf = out
	} else {
		d.nsBuf = append(d.nsBuf, value...)
	}
	if len(d.nsBuf) == start {
		return "", nil
	}
	return unsafeString(d.nsBuf[start:]), nil
}

func (d *StreamDecoder) textBytes(tok *xmltext.Token) ([]byte, error) {
	if !tok.TextNeeds {
		return tok.Text, nil
	}
	start := len(d.valueBuf)
	out, err := unescapeIntoBuffer(d.dec, d.valueBuf, start, tok.Text)
	if err != nil {
		d.valueBuf = d.valueBuf[:start]
		return nil, err
	}
	d.valueBuf = out
	if len(out) == start {
		return nil, nil
	}
	return out[start:], nil
}

func unescapeIntoBuffer(dec *xmltext.Decoder, buf []byte, start int, data []byte) ([]byte, error) {
	for {
		scratch := buf[start:cap(buf)]
		n, err := dec.UnescapeInto(scratch, data)
		if err == nil {
			end := start + n
			buf = buf[:end]
			return buf, nil
		}
		if !errors.Is(err, io.ErrShortBuffer) {
			return buf[:start], err
		}
		newCap := cap(buf) * 2
		minCap := start + len(data)
		if newCap < minCap {
			newCap = minCap
		}
		if newCap == 0 {
			newCap = minCap
		}
		next := make([]byte, start, newCap)
		copy(next, buf[:start])
		buf = next
	}
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
		Path:   dec.StackPointer(),
		Err:    err,
	}
}

package xmlstream

import (
	"bufio"
	"io"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

// StringAttr holds a namespace-qualified attribute with string value.
type StringAttr struct {
	namespace string
	local     string
	value     string
}

// NamespaceURI returns the attribute namespace URI.
func (a StringAttr) NamespaceURI() string {
	return a.namespace
}

// LocalName returns the attribute local name.
func (a StringAttr) LocalName() string {
	return a.local
}

// Value returns the attribute value.
func (a StringAttr) Value() string {
	return a.value
}

// StringEvent represents a streaming XML token with string attributes.
type StringEvent struct {
	Name       QName
	Attrs      []StringAttr
	Text       []byte
	Kind       EventKind
	Line       int
	Column     int
	ID         ElementID
	ScopeDepth int
}

// StringReader provides a streaming XML event interface with string attributes.
// It emits start/end/char events and retains xmlns attributes, without raw APIs.
type StringReader struct {
	dec          *xmltext.Decoder
	names        *qnameCache
	ns           nsStack
	attrBuf      []StringAttr
	elemStack    []QName
	valueBuf     []byte
	nsBuf        []byte
	tok          xmltext.Token
	nextID       ElementID
	lastLine     int
	lastColumn   int
	pendingPop   bool
	lastWasStart bool
}

// NewStringReader creates a new streaming reader for r that returns string attributes.
func NewStringReader(r io.Reader, opts ...Option) (*StringReader, error) {
	if r == nil {
		return nil, errNilReader
	}
	reader := bufio.NewReaderSize(r, readerBufferSize)
	options := buildOptions(opts...)
	dec := xmltext.NewDecoder(reader, options...)
	var tok xmltext.Token
	tok.Reserve(xmltext.TokenSizes{
		Attrs:     readerAttrCapacity,
		AttrName:  256,
		AttrValue: 256,
	})
	names := newQNameCache()
	names.setMaxEntries(qnameCacheLimit(options))
	return &StringReader{
		dec:      dec,
		names:    names,
		valueBuf: make([]byte, 0, 256),
		nsBuf:    make([]byte, 0, 128),
		tok:      tok,
	}, nil
}

// Reset prepares the reader for a new input stream.
func (r *StringReader) Reset(src io.Reader, opts ...Option) error {
	if r == nil {
		return errNilReader
	}
	if src == nil {
		return errNilReader
	}
	reader := bufio.NewReaderSize(src, readerBufferSize)
	options := buildOptions(opts...)
	if r.dec == nil {
		r.dec = xmltext.NewDecoder(reader, options...)
	} else {
		r.dec.Reset(reader, options...)
	}
	r.tok.Reserve(xmltext.TokenSizes{
		Attrs:     readerAttrCapacity,
		AttrName:  256,
		AttrValue: 256,
	})
	if r.names == nil {
		r.names = newQNameCache()
	} else {
		r.names.reset()
	}
	r.names.setMaxEntries(qnameCacheLimit(options))
	r.ns = nsStack{}
	r.attrBuf = r.attrBuf[:0]
	r.elemStack = r.elemStack[:0]
	r.valueBuf = r.valueBuf[:0]
	r.nsBuf = r.nsBuf[:0]
	r.nextID = 0
	r.pendingPop = false
	r.lastLine = 0
	r.lastColumn = 0
	r.lastWasStart = false
	return nil
}

// Next returns the next XML event.
func (r *StringReader) Next() (StringEvent, error) {
	if r == nil || r.dec == nil {
		return StringEvent{}, errNilReader
	}
	if r.names == nil {
		r.names = newQNameCache()
	}
	if r.pendingPop {
		r.ns.pop()
		r.pendingPop = false
	}
	r.lastWasStart = false

	for {
		if err := r.dec.ReadTokenInto(&r.tok); err != nil {
			return StringEvent{}, err
		}
		tok := &r.tok
		line, column := tok.Line, tok.Column
		r.lastLine = line
		r.lastColumn = column
		r.valueBuf = r.valueBuf[:0]

		switch tok.Kind {
		case xmltext.KindStartElement:
			scope, nsBuf, err := collectNamespaceScope(r.dec, r.nsBuf, tok)
			if err != nil {
				r.nsBuf = nsBuf
				return StringEvent{}, wrapSyntaxError(r.dec, line, column, err)
			}
			r.nsBuf = nsBuf
			scopeDepth := r.ns.push(scope)
			name, err := resolveElementName(r.names, &r.ns, r.dec, tok.Name, scopeDepth, line, column)
			if err != nil {
				return StringEvent{}, err
			}

			attrs := tok.Attrs
			if cap(r.attrBuf) < len(attrs) {
				r.attrBuf = make([]StringAttr, 0, len(attrs))
			} else {
				r.attrBuf = r.attrBuf[:0]
			}
			for _, attr := range attrs {
				attrNamespace, attrLocal, err := resolveAttrName(r.dec, &r.ns, attr.Name, scopeDepth, line, column)
				if err != nil {
					return StringEvent{}, err
				}
				value, err := r.attrValueString(attr.Value, attr.ValueNeeds)
				if err != nil {
					return StringEvent{}, wrapSyntaxError(r.dec, line, column, err)
				}
				r.attrBuf = append(r.attrBuf, StringAttr{
					namespace: attrNamespace,
					local:     string(attrLocal),
					value:     value,
				})
			}

			id := r.nextID
			r.nextID++
			r.elemStack = append(r.elemStack, name)
			r.lastWasStart = true
			return StringEvent{
				Kind:       EventStartElement,
				Name:       name,
				Attrs:      r.attrBuf,
				Line:       line,
				Column:     column,
				ID:         id,
				ScopeDepth: scopeDepth,
			}, nil

		case xmltext.KindEndElement:
			scopeDepth := r.ns.depth() - 1
			name, err := r.popElementName()
			if err != nil {
				return StringEvent{}, err
			}
			r.pendingPop = true
			return StringEvent{
				Kind:       EventEndElement,
				Name:       name,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}, nil

		case xmltext.KindCharData, xmltext.KindCDATA:
			text, err := r.textBytes(tok)
			if err != nil {
				return StringEvent{}, wrapSyntaxError(r.dec, line, column, err)
			}
			return StringEvent{
				Kind:       EventCharData,
				Text:       text,
				Line:       line,
				Column:     column,
				ScopeDepth: r.ns.depth() - 1,
			}, nil
		}
	}
}

// SkipSubtree skips the current element subtree after a StartElement event.
func (r *StringReader) SkipSubtree() error {
	if r == nil || r.dec == nil {
		return errNilReader
	}
	if !r.lastWasStart {
		return errNoStartElement
	}
	if r.pendingPop {
		r.ns.pop()
		r.pendingPop = false
	}
	if err := r.dec.SkipValue(); err != nil {
		return err
	}
	if len(r.elemStack) > 0 {
		r.elemStack = r.elemStack[:len(r.elemStack)-1]
	}
	r.ns.pop()
	r.lastWasStart = false
	return nil
}

// CurrentPos returns the line and column of the most recent token.
func (r *StringReader) CurrentPos() (line, column int) {
	if r == nil {
		return 0, 0
	}
	return r.lastLine, r.lastColumn
}

// InputOffset returns the current byte position in the input stream.
func (r *StringReader) InputOffset() int64 {
	if r == nil || r.dec == nil {
		return 0
	}
	return r.dec.InputOffset()
}

// LookupNamespace resolves a prefix in the current scope.
func (r *StringReader) LookupNamespace(prefix string) (string, bool) {
	if r == nil {
		return "", false
	}
	return r.ns.lookup(prefix, r.ns.depth()-1)
}

// LookupNamespaceAt resolves a prefix at the given scope depth.
func (r *StringReader) LookupNamespaceAt(prefix string, depth int) (string, bool) {
	if r == nil {
		return "", false
	}
	return r.ns.lookup(prefix, depth)
}

func (r *StringReader) popElementName() (QName, error) {
	var name QName
	var err error
	name, r.elemStack, err = popQName(r.elemStack, r.ns.depth())
	return name, err
}

func (r *StringReader) attrValueString(value []byte, needsUnescape bool) (string, error) {
	if !needsUnescape {
		return string(value), nil
	}
	var bytes []byte
	var err error
	r.valueBuf, bytes, err = decodeAttrValueBytes(r.dec, r.valueBuf, value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (r *StringReader) textBytes(tok *xmltext.Token) ([]byte, error) {
	if !tok.TextNeeds {
		return tok.Text, nil
	}
	var out []byte
	var err error
	r.valueBuf, out, err = decodeTextBytes(r.dec, r.valueBuf, tok.Text)
	if err != nil {
		return nil, err
	}
	return out, nil
}

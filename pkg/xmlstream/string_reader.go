package xmlstream

import (
	"bufio"
	"bytes"
	"fmt"
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
type StringReader struct {
	dec        *xmltext.Decoder
	names      *qnameCache
	ns         nsStack
	attrBuf    []StringAttr
	elemStack  []QName
	valueBuf   []byte
	nsBuf      []byte
	tok        xmltext.Token
	nextID     ElementID
	pendingPop bool
	lastLine   int
	lastColumn int
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
			scope, err := r.collectNamespaceScope(tok)
			if err != nil {
				return StringEvent{}, wrapSyntaxError(r.dec, line, column, err)
			}
			scopeDepth := r.ns.push(scope)
			name, err := r.resolveElementName(tok.Name, scopeDepth, line, column)
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
	return nil
}

// CurrentPos returns the line and column of the most recent token.
func (r *StringReader) CurrentPos() (line, column int) {
	if r == nil {
		return 0, 0
	}
	return r.lastLine, r.lastColumn
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

func (r *StringReader) collectNamespaceScope(tok *xmltext.Token) (nsScope, error) {
	scope := nsScope{}
	for _, attr := range tok.Attrs {
		if isDefaultNamespaceDecl(attr.Name) {
			value, err := r.namespaceValueString(attr.Value, attr.ValueNeeds)
			if err != nil {
				return nsScope{}, err
			}
			scope.defaultNS = value
			scope.defaultSet = true
			continue
		}
		if local, ok := prefixedNamespaceDecl(attr.Name); ok {
			if bytes.Equal(local, xmlBytes) || bytes.Equal(local, xmlnsBytes) {
				continue
			}
			value, err := r.namespaceValueString(attr.Value, attr.ValueNeeds)
			if err != nil {
				return nsScope{}, err
			}
			if scope.prefixes == nil {
				scope.prefixes = make(map[string]string, 1)
			}
			scope.prefixes[string(local)] = value
		}
	}
	return scope, nil
}

func (r *StringReader) resolveElementName(name []byte, depth, line, column int) (QName, error) {
	prefix, local, hasPrefix := splitQName(name)
	if hasPrefix {
		prefixName := unsafeString(prefix)
		namespace, ok := r.ns.lookup(prefixName, depth)
		if !ok {
			return QName{}, unboundPrefixError(r.dec, line, column)
		}
		return r.names.internBytes(namespace, local), nil
	}
	namespace, _ := r.ns.lookup("", depth)
	return r.names.internBytes(namespace, local), nil
}

func (r *StringReader) popElementName() (QName, error) {
	if len(r.elemStack) == 0 {
		return QName{}, fmt.Errorf("unexpected end element at depth %d", r.ns.depth())
	}
	name := r.elemStack[len(r.elemStack)-1]
	r.elemStack = r.elemStack[:len(r.elemStack)-1]
	return name, nil
}

func (r *StringReader) attrValueString(value []byte, needsUnescape bool) (string, error) {
	if !needsUnescape {
		return string(value), nil
	}
	start := len(r.valueBuf)
	out, err := unescapeIntoBuffer(r.dec, r.valueBuf, start, value)
	if err != nil {
		r.valueBuf = r.valueBuf[:start]
		return "", err
	}
	r.valueBuf = out
	if len(out) == start {
		return "", nil
	}
	return string(out[start:]), nil
}

func (r *StringReader) namespaceValueString(value []byte, needsUnescape bool) (string, error) {
	start := len(r.nsBuf)
	if needsUnescape {
		out, err := unescapeIntoBuffer(r.dec, r.nsBuf, start, value)
		if err != nil {
			r.nsBuf = r.nsBuf[:start]
			return "", err
		}
		r.nsBuf = out
	} else {
		r.nsBuf = append(r.nsBuf, value...)
	}
	if len(r.nsBuf) == start {
		return "", nil
	}
	return unsafeString(r.nsBuf[start:]), nil
}

func (r *StringReader) textBytes(tok *xmltext.Token) ([]byte, error) {
	if !tok.TextNeeds {
		return tok.Text, nil
	}
	start := len(r.valueBuf)
	out, err := unescapeIntoBuffer(r.dec, r.valueBuf, start, tok.Text)
	if err != nil {
		r.valueBuf = r.valueBuf[:start]
		return nil, err
	}
	r.valueBuf = out
	if len(out) == start {
		return nil, nil
	}
	return out[start:], nil
}

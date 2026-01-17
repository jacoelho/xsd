package xmlstream

import (
	"bufio"
	"errors"
	"io"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

const readerBufferSize = 256 * 1024
const readerAttrCapacity = 8

var errNilReader = errors.New("nil XML reader")

// Reader provides a streaming XML event interface with namespace tracking.
type Reader struct {
	dec          *xmltext.Decoder
	names        *qnameCache
	tok          xmltext.Token
	ns           nsStack
	attrBuf      []Attr
	rawAttrBuf   []RawAttr
	rawAttrInfo  []rawAttrInfo
	elemStack    []QName
	valueBuf     []byte
	nsBuf        []byte
	lastRawAttrs []RawAttr
	lastRawInfo  []rawAttrInfo
	lastStart    Event
	nextID       ElementID
	lastLine     int
	lastColumn   int
	pendingPop   bool
	lastWasStart bool
}

type rawAttrInfo struct {
	namespace string
	local     []byte
}

// NewReader creates a new streaming reader for r.
func NewReader(r io.Reader, opts ...Option) (*Reader, error) {
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
	return &Reader{
		dec:      dec,
		names:    names,
		valueBuf: make([]byte, 0, 256),
		nsBuf:    make([]byte, 0, 128),
		tok:      tok,
	}, nil
}

// Reset prepares the reader for a new input stream.
func (r *Reader) Reset(src io.Reader, opts ...Option) error {
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
	r.rawAttrBuf = r.rawAttrBuf[:0]
	r.rawAttrInfo = r.rawAttrInfo[:0]
	r.elemStack = r.elemStack[:0]
	r.valueBuf = r.valueBuf[:0]
	r.nsBuf = r.nsBuf[:0]
	r.nextID = 0
	r.pendingPop = false
	r.lastLine = 0
	r.lastColumn = 0
	r.lastWasStart = false
	r.lastStart = Event{}
	r.lastRawAttrs = nil
	r.lastRawInfo = nil
	return nil
}

// Next returns the next XML event.
func (r *Reader) Next() (Event, error) {
	ev, _, err := r.next(nextResolved)
	return ev, err
}

// NextRaw returns the next XML event with raw names.
// Raw name and value slices are valid until the next Next or NextRaw call.
func (r *Reader) NextRaw() (RawEvent, error) {
	_, ev, err := r.next(nextRaw)
	return ev, err
}

type nextMode uint8

const (
	nextResolved nextMode = iota
	nextRaw
)

func (r *Reader) next(mode nextMode) (Event, RawEvent, error) {
	if r == nil || r.dec == nil {
		return Event{}, RawEvent{}, errNilReader
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
			return Event{}, RawEvent{}, err
		}
		tok := &r.tok
		line, column := tok.Line, tok.Column
		r.lastLine = line
		r.lastColumn = column
		r.valueBuf = r.valueBuf[:0]

		switch tok.Kind {
		case xmltext.KindStartElement:
			return r.startEvent(mode, tok, line, column)

		case xmltext.KindEndElement:
			return r.endEvent(mode, tok, line, column)

		case xmltext.KindCharData, xmltext.KindCDATA:
			text, err := r.textBytes(tok)
			if err != nil {
				return Event{}, RawEvent{}, wrapSyntaxError(r.dec, line, column, err)
			}
			scopeDepth := r.currentScopeDepth()
			if mode == nextResolved {
				return Event{
					Kind:       EventCharData,
					Text:       text,
					Line:       line,
					Column:     column,
					ScopeDepth: scopeDepth,
				}, RawEvent{}, nil
			}
			return Event{}, RawEvent{
				Kind:       EventCharData,
				Text:       text,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}, nil

		case xmltext.KindComment:
			scopeDepth := r.currentScopeDepth()
			if mode == nextResolved {
				return Event{
					Kind:       EventComment,
					Text:       tok.Text,
					Line:       line,
					Column:     column,
					ScopeDepth: scopeDepth,
				}, RawEvent{}, nil
			}
			return Event{}, RawEvent{
				Kind:       EventComment,
				Text:       tok.Text,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}, nil

		case xmltext.KindPI:
			if tok.IsXMLDecl {
				continue
			}
			scopeDepth := r.currentScopeDepth()
			if mode == nextResolved {
				return Event{
					Kind:       EventPI,
					Text:       tok.Text,
					Line:       line,
					Column:     column,
					ScopeDepth: scopeDepth,
				}, RawEvent{}, nil
			}
			return Event{}, RawEvent{
				Kind:       EventPI,
				Text:       tok.Text,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}, nil

		case xmltext.KindDirective:
			scopeDepth := r.currentScopeDepth()
			if mode == nextResolved {
				return Event{
					Kind:       EventDirective,
					Text:       tok.Text,
					Line:       line,
					Column:     column,
					ScopeDepth: scopeDepth,
				}, RawEvent{}, nil
			}
			return Event{}, RawEvent{
				Kind:       EventDirective,
				Text:       tok.Text,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}, nil
		}
	}
}

func (r *Reader) startEvent(mode nextMode, tok *xmltext.Token, line, column int) (Event, RawEvent, error) {
	scope, nsBuf, err := collectNamespaceScope(r.dec, r.nsBuf, tok)
	if err != nil {
		r.nsBuf = nsBuf
		return Event{}, RawEvent{}, wrapSyntaxError(r.dec, line, column, err)
	}
	r.nsBuf = nsBuf
	scopeDepth := r.ns.push(scope)
	name, err := resolveElementName(r.names, &r.ns, r.dec, tok.Name, scopeDepth, line, column)
	if err != nil {
		return Event{}, RawEvent{}, err
	}

	attrs := tok.Attrs
	if mode == nextResolved {
		if cap(r.attrBuf) < len(attrs) {
			r.attrBuf = make([]Attr, 0, len(attrs))
		} else {
			r.attrBuf = r.attrBuf[:0]
		}
	} else {
		r.attrBuf = r.attrBuf[:0]
	}
	if mode == nextRaw {
		if cap(r.rawAttrBuf) < len(attrs) {
			r.rawAttrBuf = make([]RawAttr, 0, len(attrs))
		} else {
			r.rawAttrBuf = r.rawAttrBuf[:0]
		}
		if cap(r.rawAttrInfo) < len(attrs) {
			r.rawAttrInfo = make([]rawAttrInfo, 0, len(attrs))
		} else {
			r.rawAttrInfo = r.rawAttrInfo[:0]
		}
	}

	for _, attr := range attrs {
		if isDefaultNamespaceDecl(attr.Name) {
			continue
		}
		if _, ok := prefixedNamespaceDecl(attr.Name); ok {
			continue
		}
		attrNamespace, attrLocal, err := resolveAttrName(r.dec, &r.ns, attr.Name, scopeDepth, line, column)
		if err != nil {
			return Event{}, RawEvent{}, err
		}
		value, err := r.attrValueBytes(attr.Value, attr.ValueNeeds)
		if err != nil {
			return Event{}, RawEvent{}, wrapSyntaxError(r.dec, line, column, err)
		}
		if mode == nextResolved {
			r.attrBuf = append(r.attrBuf, Attr{
				Name:  r.names.internBytes(attrNamespace, attrLocal),
				Value: value,
			})
		}
		if mode == nextRaw {
			r.rawAttrBuf = append(r.rawAttrBuf, RawAttr{
				Name:  rawNameFromBytes(attr.Name),
				Value: value,
			})
			r.rawAttrInfo = append(r.rawAttrInfo, rawAttrInfo{
				namespace: attrNamespace,
				local:     attrLocal,
			})
		}
	}

	id := r.nextID
	r.nextID++
	r.elemStack = append(r.elemStack, name)
	event := Event{
		Kind:       EventStartElement,
		Name:       name,
		Attrs:      r.attrBuf,
		Line:       line,
		Column:     column,
		ID:         id,
		ScopeDepth: scopeDepth,
	}
	r.lastStart = event
	r.lastWasStart = true
	if mode == nextRaw {
		r.lastRawAttrs = r.rawAttrBuf
		r.lastRawInfo = r.rawAttrInfo
	} else {
		r.lastRawAttrs = nil
		r.lastRawInfo = nil
	}
	if mode == nextResolved {
		return event, RawEvent{}, nil
	}
	return Event{}, RawEvent{
		Kind:       EventStartElement,
		Name:       rawNameFromBytes(tok.Name),
		Attrs:      r.rawAttrBuf,
		Line:       line,
		Column:     column,
		ID:         id,
		ScopeDepth: scopeDepth,
	}, nil
}

func (r *Reader) endEvent(mode nextMode, tok *xmltext.Token, line, column int) (Event, RawEvent, error) {
	scopeDepth := r.ns.depth() - 1
	name, err := r.popElementName()
	if err != nil {
		return Event{}, RawEvent{}, err
	}
	r.pendingPop = true
	if mode == nextResolved {
		return Event{
			Kind:       EventEndElement,
			Name:       name,
			Line:       line,
			Column:     column,
			ScopeDepth: scopeDepth,
		}, RawEvent{}, nil
	}
	return Event{}, RawEvent{
		Kind:       EventEndElement,
		Name:       rawNameFromBytes(tok.Name),
		Line:       line,
		Column:     column,
		ScopeDepth: scopeDepth,
	}, nil
}

// SkipSubtree skips the current element subtree after a StartElement event.
func (r *Reader) SkipSubtree() error {
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
	r.lastRawAttrs = nil
	r.lastRawInfo = nil
	return nil
}

// CurrentPos returns the line and column of the most recent token.
func (r *Reader) CurrentPos() (line, column int) {
	if r == nil {
		return 0, 0
	}
	return r.lastLine, r.lastColumn
}

// InputOffset returns the current byte position in the input stream.
func (r *Reader) InputOffset() int64 {
	if r == nil || r.dec == nil {
		return 0
	}
	return r.dec.InputOffset()
}

// LookupNamespace resolves a prefix in the current scope.
func (r *Reader) LookupNamespace(prefix string) (string, bool) {
	if r == nil {
		return "", false
	}
	return r.ns.lookup(prefix, r.ns.depth()-1)
}

// LookupNamespaceBytes resolves a prefix in the current scope without allocation.
func (r *Reader) LookupNamespaceBytes(prefix []byte) (string, bool) {
	if r == nil {
		return "", false
	}
	return r.LookupNamespaceBytesAt(prefix, r.ns.depth()-1)
}

// LookupNamespaceBytesAt resolves a prefix at the given scope depth without allocation.
func (r *Reader) LookupNamespaceBytesAt(prefix []byte, depth int) (string, bool) {
	if r == nil {
		return "", false
	}
	return r.ns.lookup(unsafeString(prefix), depth)
}

// LookupNamespaceAt resolves a prefix at the given scope depth.
func (r *Reader) LookupNamespaceAt(prefix string, depth int) (string, bool) {
	if r == nil {
		return "", false
	}
	return r.ns.lookup(prefix, depth)
}

// NamespaceDecls returns namespace declarations in the current scope.
// The returned slice is valid until the next call to Next or NextRaw.
func (r *Reader) NamespaceDecls() []NamespaceDecl {
	if r == nil {
		return nil
	}
	return r.NamespaceDeclsAt(r.ns.depth() - 1)
}

// NamespaceDeclsAt returns namespace declarations at the given scope depth.
// The returned slice is valid until the next call to Next or NextRaw.
func (r *Reader) NamespaceDeclsAt(depth int) []NamespaceDecl {
	if r == nil {
		return nil
	}
	if len(r.ns.scopes) == 0 || depth < 0 {
		return nil
	}
	if depth >= len(r.ns.scopes) {
		depth = len(r.ns.scopes) - 1
	}
	return r.ns.scopes[depth].decls
}

func (r *Reader) popElementName() (QName, error) {
	var name QName
	var err error
	name, r.elemStack, err = popQName(r.elemStack, r.ns.depth())
	return name, err
}

func rawNameFromBytes(full []byte) RawName {
	if len(full) == 0 {
		return RawName{}
	}
	prefix, local, hasPrefix := splitQName(full)
	if !hasPrefix {
		prefix = nil
	}
	return RawName{
		Full:   full,
		Prefix: prefix,
		Local:  local,
	}
}

func (r *Reader) currentScopeDepth() int {
	depth := r.ns.depth() - 1
	if depth < 0 {
		return 0
	}
	return depth
}

func (r *Reader) attrValueBytes(value []byte, needsUnescape bool) ([]byte, error) {
	if !needsUnescape {
		return value, nil
	}
	var out []byte
	var err error
	r.valueBuf, out, err = decodeAttrValueBytes(r.dec, r.valueBuf, value)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Reader) namespaceValueString(value []byte, needsUnescape bool) (string, error) {
	if needsUnescape {
		var out string
		var err error
		r.nsBuf, out, err = decodeNamespaceValueString(r.dec, r.nsBuf, value)
		return out, err
	}
	var out string
	r.nsBuf, out = appendNamespaceValue(r.nsBuf, value)
	return out, nil
}

func (r *Reader) textBytes(tok *xmltext.Token) ([]byte, error) {
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

func qnameCacheLimit(opts []xmltext.Options) int {
	merged := xmltext.JoinOptions(opts...)
	if limit, ok := merged.QNameInternEntries(); ok {
		if limit < 0 {
			return 0
		}
		return limit
	}
	return qnameCacheMaxEntries
}

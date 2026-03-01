package xmlstream

import (
	"bufio"
	"errors"
	"io"
	"iter"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

const readerBufferSize = 256 * 1024
const readerAttrCapacity = 8

var errNilReader = errors.New("nil XML reader")
var errDuplicateAttribute = errors.New("duplicate attribute")

// Reader provides a streaming XML event interface with namespace tracking.
type Reader struct {
	nsBytes      namespaceBytesCache
	reader       *bufio.Reader
	nameIDs      *nameCache
	dec          *xmltext.Decoder
	names        *qnameCache
	ns           nsStack
	attrSeen     []uint32
	rawAttrInfo  []rawAttrInfo
	rawAttrBuf   []RawAttr
	resolvedAttr []ResolvedAttr
	attrBuf      []Attr
	lastRawInfo  []rawAttrInfo
	elemStack    []QName
	valueBuf     []byte
	nsBuf        []byte
	lastRawAttrs []RawAttr
	tok          xmltext.RawTokenSpan
	lastStart    Event
	nextID       ElementID
	lastLine     int
	lastColumn   int
	attrEpoch    uint32
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
	var tok xmltext.RawTokenSpan
	names := newQNameCache()
	names.setMaxEntries(qnameCacheLimit(options))
	return &Reader{
		reader:   reader,
		dec:      dec,
		names:    names,
		nameIDs:  newNameCache(),
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
	if r.reader == nil {
		r.reader = bufio.NewReaderSize(src, readerBufferSize)
	} else {
		r.reader.Reset(src)
	}
	options := buildOptions(opts...)
	if r.dec == nil {
		r.dec = xmltext.NewDecoder(r.reader, options...)
	} else {
		r.dec.Reset(r.reader, options...)
	}
	if r.names == nil {
		r.names = newQNameCache()
	} else {
		r.names.reset()
	}
	r.names.setMaxEntries(qnameCacheLimit(options))
	if r.nameIDs == nil {
		r.nameIDs = newNameCache()
	} else {
		r.nameIDs.reset()
	}
	r.nsBytes.reset()
	r.ns.reset()
	r.attrBuf = r.attrBuf[:0]
	r.resolvedAttr = r.resolvedAttr[:0]
	r.rawAttrBuf = r.rawAttrBuf[:0]
	r.rawAttrInfo = r.rawAttrInfo[:0]
	r.attrSeen = r.attrSeen[:0]
	r.attrEpoch = 0
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
	var ev Event
	err := r.next(nextEvent, &ev, nil, nil)
	return ev, err
}

// NextResolved returns the next XML event with namespace-resolved byte slices.
func (r *Reader) NextResolved() (ResolvedEvent, error) {
	var ev ResolvedEvent
	err := r.next(nextResolved, nil, nil, &ev)
	return ev, err
}

// NextRaw returns the next XML event with raw names.
// Raw name and value slices are valid until the next Next or NextRaw call.
func (r *Reader) NextRaw() (RawEvent, error) {
	var ev RawEvent
	err := r.next(nextRaw, nil, &ev, nil)
	return ev, err
}

type nextMode uint8

const (
	nextEvent nextMode = iota
	nextRaw
	nextResolved
)

type startElementCore struct {
	name       QName
	namespace  string
	local      []byte
	scopeDepth int
}

func (r *Reader) next(mode nextMode, event *Event, raw *RawEvent, resolved *ResolvedEvent) error {
	if r == nil || r.dec == nil {
		return errNilReader
	}
	if r.names == nil {
		r.names = newQNameCache()
	}
	if mode == nextResolved && r.nameIDs == nil {
		r.nameIDs = newNameCache()
	}
	if r.pendingPop {
		r.ns.pop()
		r.pendingPop = false
	}
	r.lastWasStart = false

	for {
		if err := r.dec.ReadTokenRawSpansInto(&r.tok); err != nil {
			return err
		}
		tok := &r.tok
		line, column := tok.Line, tok.Column
		r.lastLine = line
		r.lastColumn = column
		r.valueBuf = r.valueBuf[:0]

		switch tok.Kind {
		case xmltext.KindStartElement:
			return r.emitStart(mode, tok, line, column, event, raw, resolved)

		case xmltext.KindEndElement:
			return r.emitEnd(mode, tok, line, column, event, raw, resolved)

		case xmltext.KindPI:
			if tok.IsXMLDecl {
				continue
			}
			fallthrough

		case xmltext.KindCharData, xmltext.KindCDATA, xmltext.KindComment, xmltext.KindDirective:
			return r.emitTokenText(mode, tok, line, column, event, raw, resolved)
		}
	}
}

func (r *Reader) emitStart(mode nextMode, tok *xmltext.RawTokenSpan, line, column int, event *Event, raw *RawEvent, resolved *ResolvedEvent) error {
	switch mode {
	case nextEvent:
		ev, err := r.startEvent(tok, line, column)
		if err != nil {
			return err
		}
		if event != nil {
			*event = ev
		}
	case nextResolved:
		ev, err := r.startResolvedEvent(tok, line, column)
		if err != nil {
			return err
		}
		if resolved != nil {
			*resolved = ev
		}
	default:
		ev, err := r.startRawEvent(tok, line, column)
		if err != nil {
			return err
		}
		if raw != nil {
			*raw = ev
		}
	}
	return nil
}

func (r *Reader) emitEnd(mode nextMode, tok *xmltext.RawTokenSpan, line, column int, event *Event, raw *RawEvent, resolved *ResolvedEvent) error {
	switch mode {
	case nextEvent:
		ev, err := r.endEvent(tok, line, column)
		if err != nil {
			return err
		}
		if event != nil {
			*event = ev
		}
	case nextResolved:
		ev, err := r.endResolvedEvent(tok, line, column)
		if err != nil {
			return err
		}
		if resolved != nil {
			*resolved = ev
		}
	default:
		ev, err := r.endRawEvent(tok, line, column)
		if err != nil {
			return err
		}
		if raw != nil {
			*raw = ev
		}
	}
	return nil
}

func (r *Reader) emitTokenText(mode nextMode, tok *xmltext.RawTokenSpan, line, column int, event *Event, raw *RawEvent, resolved *ResolvedEvent) error {
	kind, ok := tokenEventKind(tok.Kind)
	if !ok {
		return nil
	}
	text, err := r.tokenText(tok)
	if err != nil {
		return wrapSyntaxError(r.dec, line, column, err)
	}
	scopeDepth := r.currentScopeDepth()
	r.projectTokenEvent(mode, kind, text, line, column, scopeDepth, event, raw, resolved)
	return nil
}

func tokenEventKind(kind xmltext.Kind) (EventKind, bool) {
	switch kind {
	case xmltext.KindCharData, xmltext.KindCDATA:
		return EventCharData, true
	case xmltext.KindComment:
		return EventComment, true
	case xmltext.KindPI:
		return EventPI, true
	case xmltext.KindDirective:
		return EventDirective, true
	default:
		return 0, false
	}
}

func (r *Reader) tokenText(tok *xmltext.RawTokenSpan) ([]byte, error) {
	switch tok.Kind {
	case xmltext.KindCharData, xmltext.KindCDATA:
		return r.textBytes(tok.Text, tok.TextNeeds)
	default:
		return tok.Text, nil
	}
}

func (r *Reader) projectTokenEvent(mode nextMode, kind EventKind, text []byte, line, column, scopeDepth int, event *Event, raw *RawEvent, resolved *ResolvedEvent) {
	switch mode {
	case nextEvent:
		if event != nil {
			*event = Event{
				Kind:       kind,
				Text:       text,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}
		}
	case nextResolved:
		if resolved != nil {
			*resolved = ResolvedEvent{
				Kind:       kind,
				Text:       text,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}
		}
	default:
		if raw != nil {
			*raw = RawEvent{
				Kind:       kind,
				Text:       text,
				Line:       line,
				Column:     column,
				ScopeDepth: scopeDepth,
			}
		}
	}
}

func (r *Reader) startEvent(tok *xmltext.RawTokenSpan, line, column int) (Event, error) {
	core, err := r.beginStartElementCore(tok, line, column)
	if err != nil {
		return Event{}, err
	}

	attrCount := tok.AttrCount()
	if cap(r.attrBuf) < attrCount {
		r.attrBuf = make([]Attr, 0, attrCount)
	} else {
		r.attrBuf = r.attrBuf[:0]
	}
	r.beginAttrDedup()

	err = r.scanStartAttributes(tok, core.scopeDepth, line, column, func(_ []byte, attrNamespace string, attrLocal, value []byte) error {
		if markErr := r.markAttrSeen(attrNamespace, attrLocal, line, column); markErr != nil {
			return markErr
		}
		r.attrBuf = append(r.attrBuf, Attr{
			Name:  r.names.internBytes(attrNamespace, attrLocal),
			Value: value,
		})
		return nil
	})
	if err != nil {
		return Event{}, err
	}

	_ = r.commitStartEvent(core.name, line, column, core.scopeDepth)
	r.lastStart.Attrs = r.attrBuf
	r.lastRawAttrs = nil
	r.lastRawInfo = nil
	return r.lastStart, nil
}

func (r *Reader) startRawEvent(tok *xmltext.RawTokenSpan, line, column int) (RawEvent, error) {
	core, err := r.beginStartElementCore(tok, line, column)
	if err != nil {
		return RawEvent{}, err
	}

	attrCount := tok.AttrCount()
	r.attrBuf = r.attrBuf[:0]
	if cap(r.rawAttrBuf) < attrCount {
		r.rawAttrBuf = make([]RawAttr, 0, attrCount)
	} else {
		r.rawAttrBuf = r.rawAttrBuf[:0]
	}
	if cap(r.rawAttrInfo) < attrCount {
		r.rawAttrInfo = make([]rawAttrInfo, 0, attrCount)
	} else {
		r.rawAttrInfo = r.rawAttrInfo[:0]
	}
	r.beginAttrDedup()

	err = r.scanStartAttributes(tok, core.scopeDepth, line, column, func(attrName []byte, attrNamespace string, attrLocal, value []byte) error {
		if markErr := r.markAttrSeen(attrNamespace, attrLocal, line, column); markErr != nil {
			return markErr
		}
		r.rawAttrBuf = append(r.rawAttrBuf, RawAttr{
			Name:  rawNameFromBytes(attrName),
			Value: value,
		})
		r.rawAttrInfo = append(r.rawAttrInfo, rawAttrInfo{
			namespace: attrNamespace,
			local:     attrLocal,
		})
		return nil
	})
	if err != nil {
		return RawEvent{}, err
	}

	id := r.commitStartEvent(core.name, line, column, core.scopeDepth)
	r.lastStart.Attrs = r.attrBuf
	r.lastRawAttrs = r.rawAttrBuf
	r.lastRawInfo = r.rawAttrInfo
	return RawEvent{
		Kind:       EventStartElement,
		Name:       rawNameFromBytes(tok.Name),
		Attrs:      r.rawAttrBuf,
		Line:       line,
		Column:     column,
		ID:         id,
		ScopeDepth: core.scopeDepth,
	}, nil
}

func (r *Reader) startResolvedEvent(tok *xmltext.RawTokenSpan, line, column int) (ResolvedEvent, error) {
	core, err := r.beginStartElementCore(tok, line, column)
	if err != nil {
		return ResolvedEvent{}, err
	}

	nameID := r.nameIDs.internBytes(core.namespace, core.local)
	nsBytes := r.nsBytes.intern(core.namespace)

	attrCount := tok.AttrCount()
	if cap(r.resolvedAttr) < attrCount {
		r.resolvedAttr = make([]ResolvedAttr, 0, attrCount)
	} else {
		r.resolvedAttr = r.resolvedAttr[:0]
	}
	r.beginAttrDedup()

	err = r.scanStartAttributes(tok, core.scopeDepth, line, column, func(_ []byte, attrNamespace string, attrLocal, value []byte) error {
		attrID, attrErr := r.attrID(attrNamespace, attrLocal)
		if attrErr != nil {
			return wrapSyntaxError(r.dec, line, column, attrErr)
		}
		attrNSBytes := r.nsBytes.intern(attrNamespace)
		r.resolvedAttr = append(r.resolvedAttr, ResolvedAttr{
			NameID: attrID,
			NS:     attrNSBytes,
			Local:  attrLocal,
			Value:  value,
		})
		return nil
	})
	if err != nil {
		return ResolvedEvent{}, err
	}

	id := r.commitStartEvent(core.name, line, column, core.scopeDepth)
	r.lastRawAttrs = nil
	r.lastRawInfo = nil

	return ResolvedEvent{
		Kind:       EventStartElement,
		NameID:     nameID,
		NS:         nsBytes,
		Local:      core.local,
		Attrs:      r.resolvedAttr,
		Line:       line,
		Column:     column,
		ID:         id,
		ScopeDepth: core.scopeDepth,
	}, nil
}

func (r *Reader) beginAttrDedup() {
	r.attrEpoch++
	if r.attrEpoch == 0 {
		clear(r.attrSeen)
		r.attrEpoch = 1
	}
}

func (r *Reader) attrID(namespace string, local []byte) (NameID, error) {
	attrID := r.nameIDs.internBytes(namespace, local)
	if attrID == 0 {
		return 0, nil
	}
	idx := int(attrID)
	if idx >= len(r.attrSeen) {
		r.attrSeen = append(r.attrSeen, make([]uint32, idx-len(r.attrSeen)+1)...)
	}
	if r.attrSeen[idx] == r.attrEpoch {
		return 0, errDuplicateAttribute
	}
	r.attrSeen[idx] = r.attrEpoch
	return attrID, nil
}

func (r *Reader) markAttrSeen(namespace string, local []byte, line, column int) error {
	if r.nameIDs == nil {
		r.nameIDs = newNameCache()
	}
	if _, err := r.attrID(namespace, local); err != nil {
		return wrapSyntaxError(r.dec, line, column, err)
	}
	return nil
}

func (r *Reader) commitStartEvent(name QName, line, column, scopeDepth int) ElementID {
	id := r.nextID
	r.nextID++
	r.elemStack = append(r.elemStack, name)
	r.lastWasStart = true
	r.lastStart = Event{
		Kind:       EventStartElement,
		Name:       name,
		Line:       line,
		Column:     column,
		ID:         id,
		ScopeDepth: scopeDepth,
	}
	return id
}

func (r *Reader) beginStartElement(tok *xmltext.RawTokenSpan, line, column int) (int, error) {
	declStart := len(r.ns.decls)
	scope, nsBuf, decls, err := collectNamespaceScope(r.dec, r.nsBuf, r.ns.decls, tok)
	if err != nil {
		r.nsBuf = nsBuf
		return 0, wrapSyntaxError(r.dec, line, column, err)
	}
	r.nsBuf = nsBuf
	r.ns.decls = decls
	scope.declStart = declStart
	scope.declLen = len(r.ns.decls) - declStart
	return r.ns.push(scope), nil
}

func (r *Reader) beginStartElementCore(tok *xmltext.RawTokenSpan, line, column int) (startElementCore, error) {
	scopeDepth, err := r.beginStartElement(tok, line, column)
	if err != nil {
		return startElementCore{}, err
	}
	namespace, local, err := resolveElementParts(&r.ns, r.dec, tok.Name, tok.NameColon, scopeDepth, line, column)
	if err != nil {
		return startElementCore{}, err
	}
	return startElementCore{
		name:       r.names.internBytes(namespace, local),
		namespace:  namespace,
		local:      local,
		scopeDepth: scopeDepth,
	}, nil
}

func (r *Reader) scanStartAttributes(
	tok *xmltext.RawTokenSpan,
	scopeDepth, line, column int,
	emit func(attrName []byte, namespace string, local, value []byte) error,
) error {
	for i := range tok.AttrCount() {
		attrName := tok.AttrName(i)
		if isDefaultNamespaceDecl(attrName) {
			continue
		}
		if _, ok := prefixedNamespaceDecl(attrName); ok {
			continue
		}
		attrNamespace, attrLocal, err := resolveAttrName(r.dec, &r.ns, attrName, tok.AttrNameColon(i), scopeDepth, line, column)
		if err != nil {
			return err
		}
		value, err := r.attrValueBytes(tok.AttrValue(i), tok.AttrValueNeeds(i))
		if err != nil {
			return wrapSyntaxError(r.dec, line, column, err)
		}
		if err := emit(attrName, attrNamespace, attrLocal, value); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reader) endEvent(_ *xmltext.RawTokenSpan, line, column int) (Event, error) {
	name, scopeDepth, err := r.beginEndElement()
	if err != nil {
		return Event{}, err
	}
	return Event{
		Kind:       EventEndElement,
		Name:       name,
		Line:       line,
		Column:     column,
		ScopeDepth: scopeDepth,
	}, nil
}

func (r *Reader) endRawEvent(tok *xmltext.RawTokenSpan, line, column int) (RawEvent, error) {
	_, scopeDepth, err := r.beginEndElement()
	if err != nil {
		return RawEvent{}, err
	}
	return RawEvent{
		Kind:       EventEndElement,
		Name:       rawNameFromBytes(tok.Name),
		Line:       line,
		Column:     column,
		ScopeDepth: scopeDepth,
	}, nil
}

func (r *Reader) endResolvedEvent(tok *xmltext.RawTokenSpan, line, column int) (ResolvedEvent, error) {
	name, scopeDepth, err := r.beginEndElement()
	if err != nil {
		return ResolvedEvent{}, err
	}

	_, local, _ := splitQNameWithColon(tok.Name, tok.NameColon)
	namespace := name.Namespace
	nameID := r.nameIDs.internBytes(namespace, local)
	nsBytes := r.nsBytes.intern(namespace)
	return ResolvedEvent{
		Kind:       EventEndElement,
		NameID:     nameID,
		NS:         nsBytes,
		Local:      local,
		Line:       line,
		Column:     column,
		ScopeDepth: scopeDepth,
	}, nil
}

func (r *Reader) beginEndElement() (QName, int, error) {
	scopeDepth := r.ns.depth() - 1
	name, err := r.popElementName()
	if err != nil {
		return QName{}, 0, err
	}
	r.pendingPop = true
	return name, scopeDepth, nil
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

// NamespaceDeclsSeq yields namespace declarations at the given scope depth.
func (r *Reader) NamespaceDeclsSeq(depth int) iter.Seq[NamespaceDecl] {
	return func(yield func(NamespaceDecl) bool) {
		decls := r.NamespaceDecls(depth)
		for _, decl := range decls {
			if !yield(decl) {
				return
			}
		}
	}
}

// NamespaceDecls returns namespace declarations at the given scope depth.
// The returned slice aliases reader-owned storage and is valid until the next read.
func (r *Reader) NamespaceDecls(depth int) []NamespaceDecl {
	if r == nil || len(r.ns.scopes) == 0 || depth < 0 {
		return nil
	}
	if depth >= len(r.ns.scopes) {
		depth = len(r.ns.scopes) - 1
	}
	scope := r.ns.scopes[depth]
	if scope.declLen == 0 {
		return nil
	}
	return r.ns.decls[scope.declStart : scope.declStart+scope.declLen]
}

// CurrentNamespaceDeclsSeq yields namespace declarations in the current scope.
func (r *Reader) CurrentNamespaceDeclsSeq() iter.Seq[NamespaceDecl] {
	if r == nil {
		return func(func(NamespaceDecl) bool) {}
	}
	return r.NamespaceDeclsSeq(r.ns.depth() - 1)
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

func (r *Reader) textBytes(text []byte, needsUnescape bool) ([]byte, error) {
	if !needsUnescape {
		return text, nil
	}
	var out []byte
	var err error
	r.valueBuf, out, err = decodeTextBytes(r.dec, r.valueBuf, text)
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

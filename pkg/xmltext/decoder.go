package xmltext

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"unsafe"
)

const defaultBufferSize = 32 * 1024
const attrSeenSmallMax = 8

// Decoder streams XML tokens with zero-copy spans.
type Decoder struct {
	buf                spanBuffer
	attrValueBuf       spanBuffer
	valueBuf           spanBuffer
	coalesce           spanBuffer
	scratch            spanBuffer
	r                  io.Reader
	err                error
	attrSeen           map[uint64]attrBucket
	interner           *nameInterner
	bufioReader        *bufio.Reader
	attrSeenSmall      [attrSeenSmallMax]attrSeenEntry
	entities           entityResolver
	pendingToken       Token
	pendingEndToken    Token
	optsRaw            Options
	stack              []stackEntry
	attrBuf            []AttrSpan
	attrNeeds          []bool
	attrRaw            []Span
	attrRawNeeds       []bool
	opts               decoderOptions
	pos                int
	attrSeenSmallCount int
	compactFloorAbs    int64
	baseOffset         int64
	column             int
	rootCount          int64
	line               int
	pendingCR          bool
	attrSeenGen        uint32
	xmlDeclSeen        bool
	afterRoot          bool
	seenNonXMLDecl     bool
	directiveSeen      bool
	rootSeen           bool
	eof                bool
	pendingSelfClosing bool
	pendingTokenValid  bool
	pendingEnd         bool
	compactFloorSet    bool
	lastKind           Kind
	lastSelfClosing    bool
}

type decoderOptions struct {
	entityMap             map[string]string
	charsetReader         func(label string, r io.Reader) (io.Reader, error)
	maxDepth              int
	bufferSize            int
	maxQNameInternEntries int
	maxTokenSize          int
	maxAttrs              int
	emitComments          bool
	coalesceCharData      bool
	trackLineColumn       bool
	emitDirectives        bool
	emitPI                bool
	debugPoisonSpans      bool
	resolveEntities       bool
}

// NewDecoder creates a new XML decoder for the reader.
func NewDecoder(r io.Reader, opts ...Options) *Decoder {
	dec := &Decoder{}
	dec.Reset(r, opts...)
	return dec
}

// Reset prepares the decoder for reading from r with new options.
func (d *Decoder) Reset(r io.Reader, opts ...Options) {
	if d == nil {
		return
	}
	joined := JoinOptions(opts...)
	d.optsRaw = joined
	d.opts = resolveOptions(joined)

	d.entities = newEntityResolver(d.opts.entityMap, d.opts.maxTokenSize)

	d.buf.poison = d.opts.debugPoisonSpans
	d.scratch.poison = d.opts.debugPoisonSpans
	d.coalesce.poison = d.opts.debugPoisonSpans
	d.attrValueBuf.poison = d.opts.debugPoisonSpans

	d.buf.entities = &d.entities
	d.scratch.entities = &d.entities
	d.coalesce.entities = &d.entities
	d.attrValueBuf.entities = &d.entities

	d.buf.data = d.buf.data[:0]
	d.scratch.data = d.scratch.data[:0]
	d.coalesce.data = d.coalesce.data[:0]
	d.valueBuf.data = d.valueBuf.data[:0]
	d.attrValueBuf.data = d.attrValueBuf.data[:0]
	d.pos = 0
	d.baseOffset = 0
	d.eof = false
	d.err = nil

	d.attrBuf = d.attrBuf[:0]
	d.attrNeeds = d.attrNeeds[:0]
	d.attrRaw = d.attrRaw[:0]
	d.attrRawNeeds = d.attrRawNeeds[:0]
	d.attrSeenSmallCount = 0
	d.stack = d.stack[:0]
	d.rootCount = 0
	d.rootSeen = false
	d.afterRoot = false
	d.xmlDeclSeen = false
	d.seenNonXMLDecl = false
	d.directiveSeen = false

	d.pendingTokenValid = false
	d.pendingSelfClosing = false
	d.pendingEnd = false
	d.pendingToken = Token{}
	d.pendingEndToken = Token{}
	d.lastKind = KindNone
	d.lastSelfClosing = false
	d.compactFloorSet = false
	d.compactFloorAbs = 0

	if d.opts.trackLineColumn {
		d.line = 1
		d.column = 1
	} else {
		d.line = 0
		d.column = 0
	}
	d.pendingCR = false

	if d.opts.debugPoisonSpans {
		d.buf.gen++
		d.scratch.gen++
		d.coalesce.gen++
		d.attrValueBuf.gen++
	}

	if d.interner == nil {
		d.interner = newNameInterner(d.opts.maxQNameInternEntries)
	} else {
		d.interner.setMax(d.opts.maxQNameInternEntries)
	}

	if r == nil {
		d.err = errNilReader
		return
	}
	var reader *bufio.Reader
	if bufioReader, ok := r.(*bufio.Reader); ok {
		reader = bufioReader
	} else {
		// reuse the internal bufio.Reader to avoid per-reset allocations.
		bufferSize := d.opts.bufferSize
		if bufferSize <= 0 {
			bufferSize = defaultBufferSize
		}
		if d.bufioReader == nil || d.bufioReader.Size() != bufferSize {
			d.bufioReader = bufio.NewReaderSize(r, bufferSize)
		} else {
			d.bufioReader.Reset(r)
		}
		reader = d.bufioReader
	}
	wrapped, err := wrapCharsetReaderFromBufio(reader, d.opts.charsetReader)
	if err != nil {
		d.err = err
		return
	}
	d.r = wrapped
}

// Options returns the immutable options snapshot.
func (d *Decoder) Options() Options {
	var zero Options
	if d == nil {
		return zero
	}
	return d.optsRaw
}

// PeekKind reports the kind of the next token without advancing input.
func (d *Decoder) PeekKind() Kind {
	if d == nil || d.err != nil {
		return KindNone
	}
	if d.pendingTokenValid {
		return d.pendingToken.Kind
	}
	if d.pendingEnd {
		return KindEndElement
	}
	kind, err := d.peekKind()
	if err != nil {
		return KindNone
	}
	if d.opts.coalesceCharData && (kind == KindCharData || kind == KindCDATA) {
		return KindCharData
	}
	return kind
}

// ReadTokenInto reads the next XML token into dst.
// Spans in dst are only valid until the next read call.
func (d *Decoder) ReadTokenInto(dst *Token) error {
	if dst == nil {
		return errNilToken
	}
	if d == nil {
		return errNilReader
	}
	if d.err != nil {
		return d.err
	}
	useTokenBuffers := dst.Attrs != nil || dst.AttrNeeds != nil || dst.AttrRaw != nil || dst.AttrRawNeeds != nil
	var (
		origAttrs        []AttrSpan
		origAttrNeeds    []bool
		origAttrRaw      []Span
		origAttrRawNeeds []bool
	)
	if useTokenBuffers {
		origAttrs = d.attrBuf
		origAttrNeeds = d.attrNeeds
		origAttrRaw = d.attrRaw
		origAttrRawNeeds = d.attrRawNeeds

		d.attrBuf = dst.Attrs[:0]
		d.attrNeeds = dst.AttrNeeds[:0]
		d.attrRaw = dst.AttrRaw[:0]
		d.attrRawNeeds = dst.AttrRawNeeds[:0]
	}
	err := d.readTokenInto(dst)
	if useTokenBuffers {
		d.attrBuf = origAttrs
		d.attrNeeds = origAttrNeeds
		d.attrRaw = origAttrRaw
		d.attrRawNeeds = origAttrRawNeeds
	}
	return err
}

// ReadToken returns the next XML token.
func (d *Decoder) ReadToken() (Token, error) {
	var tok Token
	if err := d.ReadTokenInto(&tok); err != nil {
		return Token{}, err
	}
	return tok, nil
}

func (d *Decoder) readTokenInto(dst *Token) error {
	d.bumpGen()
	if !d.opts.coalesceCharData {
		selfClosing, err := d.nextTokenInto(dst, true)
		if err != nil {
			return d.fail(err)
		}
		if d.opts.debugPoisonSpans {
			d.refreshToken(dst)
		}
		d.lastKind = dst.Kind
		d.lastSelfClosing = dst.Kind == KindStartElement && selfClosing
		return nil
	}

	firstSelfClosing, err := d.nextTokenInto(dst, true)
	if err != nil {
		return d.fail(err)
	}
	if dst.Kind != KindCharData && dst.Kind != KindCDATA {
		if d.opts.debugPoisonSpans {
			d.refreshToken(dst)
		}
		d.lastKind = dst.Kind
		d.lastSelfClosing = dst.Kind == KindStartElement && firstSelfClosing
		return nil
	}

	nextKind, err := d.peekKind()
	if err != nil && !errors.Is(err, io.EOF) {
		return d.fail(err)
	}
	if errors.Is(err, io.EOF) || (nextKind != KindCharData && nextKind != KindCDATA) {
		setCharDataToken(dst, dst.Text, dst.TextNeeds, dst.TextRawNeeds, dst.Line, dst.Column, dst.Raw)
		if d.opts.debugPoisonSpans {
			d.refreshToken(dst)
		}
		d.lastKind = KindCharData
		d.lastSelfClosing = false
		return nil
	}

	d.coalesce.data = d.coalesce.data[:0]
	needs := dst.TextNeeds
	rawNeeds := dst.TextRawNeeds
	startLine, startColumn := dst.Line, dst.Column
	rawStartAbs := d.baseOffset + int64(dst.Raw.Start)
	rawEndAbs := d.baseOffset + int64(dst.Raw.End)
	d.coalesce.data = append(d.coalesce.data, dst.Text.bytes()...)

	d.setCompactFloorAbs(rawStartAbs)
	defer d.clearCompactFloor()

	for {
		nextSelfClosing, err := d.nextTokenInto(&d.pendingToken, true)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return d.fail(err)
		}
		if d.pendingToken.Kind != KindCharData && d.pendingToken.Kind != KindCDATA {
			d.pendingSelfClosing = nextSelfClosing
			d.pendingTokenValid = true
			break
		}
		needs = needs || d.pendingToken.TextNeeds
		rawNeeds = rawNeeds || d.pendingToken.TextRawNeeds
		rawEndAbs = d.baseOffset + int64(d.pendingToken.Raw.End)
		d.coalesce.data = append(d.coalesce.data, d.pendingToken.Text.bytes()...)
	}

	span := makeSpan(&d.coalesce, 0, len(d.coalesce.data))
	rawStart := int(rawStartAbs - d.baseOffset)
	rawEnd := int(rawEndAbs - d.baseOffset)
	rawSpan := makeSpan(&d.buf, rawStart, rawEnd)
	setCharDataToken(dst, span, needs, rawNeeds, startLine, startColumn, rawSpan)
	if d.opts.debugPoisonSpans {
		d.refreshToken(dst)
	}
	d.lastKind = KindCharData
	d.lastSelfClosing = false
	return nil
}

// ReadValue returns the raw bytes for the next element subtree or token.
func (d *Decoder) ReadValue() (Value, error) {
	if d == nil {
		return nil, errNilReader
	}
	if d.err != nil {
		return nil, d.err
	}
	d.bumpGen()

	first, err := d.nextToken(false)
	if err != nil {
		return nil, d.fail(err)
	}
	if first.Kind != KindStartElement {
		switch first.Kind {
		case KindCharData:
			if d.opts.resolveEntities {
				return Value(first.Text.bytes()), nil
			}
			return Value(first.Raw.bytes()), nil
		case KindCDATA:
			return Value(first.Raw.bytes()), nil
		default:
			return Value(first.Raw.bytes()), nil
		}
	}

	rawStart := first.Raw.Start
	rawEnd := first.Raw.End
	depth := 1
	needsCopy := d.opts.resolveEntities && tokenNeedsExpansion(first)
	var out []byte
	cursor := rawStart
	if needsCopy {
		out = d.valueBuf.data[:0]
		if first.Raw.buf != nil && first.Raw.Start > cursor {
			out = append(out, d.buf.data[cursor:first.Raw.Start]...)
			cursor = first.Raw.Start
		}
		var appendErr error
		out, appendErr = d.appendTokenValue(out, first, &cursor)
		if appendErr != nil {
			return nil, d.fail(appendErr)
		}
	}

	for depth > 0 {
		next, err := d.nextToken(false)
		if err != nil {
			return nil, d.fail(err)
		}
		if next.Raw.buf != nil && next.Raw.End > rawEnd {
			rawEnd = next.Raw.End
		}
		switch next.Kind {
		case KindStartElement:
			depth++
		case KindEndElement:
			depth--
		}
		if !d.opts.resolveEntities {
			continue
		}
		if !needsCopy && tokenNeedsExpansion(next) {
			needsCopy = true
			out = d.valueBuf.data[:0]
			cursor = rawStart
			if next.Raw.buf != nil && next.Raw.Start > cursor {
				out = append(out, d.buf.data[cursor:next.Raw.Start]...)
				cursor = next.Raw.Start
			}
			var appendErr error
			out, appendErr = d.appendTokenValue(out, next, &cursor)
			if appendErr != nil {
				return nil, d.fail(appendErr)
			}
			continue
		}
		if needsCopy {
			if next.Raw.buf == nil {
				continue
			}
			if next.Raw.Start > cursor {
				out = append(out, d.buf.data[cursor:next.Raw.Start]...)
				cursor = next.Raw.Start
			}
			var appendErr error
			out, appendErr = d.appendTokenValue(out, next, &cursor)
			if appendErr != nil {
				return nil, d.fail(appendErr)
			}
		}
	}

	if !d.opts.resolveEntities || !needsCopy {
		if rawStart < 0 || rawEnd < rawStart || rawEnd > len(d.buf.data) {
			return nil, d.fail(errInvalidToken)
		}
		return Value(d.buf.data[rawStart:rawEnd]), nil
	}
	if cursor < rawEnd {
		out = append(out, d.buf.data[cursor:rawEnd]...)
	}
	d.valueBuf.data = out
	return Value(out), nil
}

func tokenNeedsExpansion(tok Token) bool {
	switch tok.Kind {
	case KindCharData:
		return tok.TextRawNeeds
	case KindStartElement:
		for _, needs := range tok.AttrRawNeeds {
			if needs {
				return true
			}
		}
	}
	return false
}

func (d *Decoder) appendTokenValue(dst []byte, tok Token, cursor *int) ([]byte, error) {
	if tok.Raw.buf == nil {
		return dst, nil
	}
	rawStart := tok.Raw.Start
	rawEnd := tok.Raw.End
	if rawStart < *cursor {
		return nil, errInvalidToken
	}
	switch tok.Kind {
	case KindCharData:
		if tok.TextRawNeeds {
			dst = append(dst, tok.Text.bytes()...)
			*cursor = rawEnd
			return dst, nil
		}
	case KindStartElement:
		if len(tok.AttrRaw) != len(tok.Attrs) {
			return nil, errInvalidToken
		}
		if hasAttrExpansion(tok) {
			pos := rawStart
			for i, rawSpan := range tok.AttrRaw {
				if rawSpan.Start < pos || rawSpan.End > rawEnd {
					return nil, errInvalidToken
				}
				dst = append(dst, d.buf.data[pos:rawSpan.Start]...)
				dst = append(dst, tok.Attrs[i].ValueSpan.bytes()...)
				pos = rawSpan.End
			}
			dst = append(dst, d.buf.data[pos:rawEnd]...)
			*cursor = rawEnd
			return dst, nil
		}
	}
	dst = append(dst, d.buf.data[rawStart:rawEnd]...)
	*cursor = rawEnd
	return dst, nil
}

func hasAttrExpansion(tok Token) bool {
	for _, needs := range tok.AttrRawNeeds {
		if needs {
			return true
		}
	}
	return false
}

// SkipValue skips the current value without materializing it.
func (d *Decoder) SkipValue() error {
	if d == nil {
		return errNilReader
	}
	if d.err != nil {
		return d.err
	}
	if d.lastKind == KindStartElement {
		if d.lastSelfClosing {
			_, err := d.ReadToken()
			return err
		}
		depth := 1
		for depth > 0 {
			next, err := d.ReadToken()
			if err != nil {
				return err
			}
			switch next.Kind {
			case KindStartElement:
				depth++
			case KindEndElement:
				depth--
			}
		}
		return nil
	}
	switch d.PeekKind() {
	case KindNone:
		_, err := d.ReadToken()
		return err
	case KindEndElement:
		return nil
	}

	tok, err := d.ReadToken()
	if err != nil {
		return err
	}
	if tok.Kind != KindStartElement {
		return nil
	}
	depth := 1
	for depth > 0 {
		next, err := d.ReadToken()
		if err != nil {
			return err
		}
		switch next.Kind {
		case KindStartElement:
			depth++
		case KindEndElement:
			depth--
		}
	}
	return nil
}

// UnreadBuffer exposes the unread portion of the internal buffer.
func (d *Decoder) UnreadBuffer() []byte {
	if d == nil || d.pos >= len(d.buf.data) {
		return nil
	}
	return d.buf.data[d.pos:]
}

// SpanBytes returns the bytes referenced by the span.
func (d *Decoder) SpanBytes(s Span) []byte {
	return s.bytes()
}

// SpanString returns a string view of the span when the backing buffer is stable.
func (d *Decoder) SpanString(s Span) string {
	data := s.bytes()
	if len(data) == 0 {
		return ""
	}
	if s.buf != nil && s.buf.stable {
		return unsafe.String(unsafe.SliceData(data), len(data))
	}
	return string(data)
}

// InputOffset reports the absolute byte offset of the next read position.
func (d *Decoder) InputOffset() int64 {
	if d == nil {
		return 0
	}
	return d.baseOffset + int64(d.pos)
}

// StackDepth reports the current element nesting depth.
func (d *Decoder) StackDepth() int {
	if d == nil {
		return 0
	}
	return len(d.stack)
}

// StackIndex returns the stack entry at depth i (0 is root).
func (d *Decoder) StackIndex(i int) StackEntry {
	if d == nil || i < 0 || i >= len(d.stack) {
		return StackEntry{}
	}
	entry := d.stack[i].StackEntry
	return entry
}

// StackPath copies the current stack into dst.
func (d *Decoder) StackPath(dst Path) Path {
	if d == nil {
		return dst[:0]
	}
	if cap(dst) < len(d.stack) {
		dst = make(Path, 0, len(d.stack))
	} else {
		dst = dst[:0]
	}
	for _, entry := range d.stack {
		dst = append(dst, entry.StackEntry)
	}
	return dst
}

// StackPointer renders the current stack path as a string.
func (d *Decoder) StackPointer() string {
	return d.StackPath(nil).String()
}

// InternStats reports QName interning statistics.
func (d *Decoder) InternStats() InternStats {
	if d == nil || d.interner == nil {
		return InternStats{}
	}
	return d.interner.stats
}

func resolveOptions(opts Options) decoderOptions {
	resolved := decoderOptions{trackLineColumn: true, bufferSize: defaultBufferSize}
	if value, ok := opts.CharsetReader(); ok {
		resolved.charsetReader = value
	}
	if value, ok := opts.EntityMap(); ok {
		resolved.entityMap = value
	}
	if value, ok := opts.ResolveEntities(); ok {
		resolved.resolveEntities = value
	}
	if value, ok := opts.EmitComments(); ok {
		resolved.emitComments = value
	}
	if value, ok := opts.EmitPI(); ok {
		resolved.emitPI = value
	}
	if value, ok := opts.EmitDirectives(); ok {
		resolved.emitDirectives = value
	}
	if value, ok := opts.TrackLineColumn(); ok {
		resolved.trackLineColumn = value
	}
	if value, ok := opts.CoalesceCharData(); ok {
		resolved.coalesceCharData = value
	}
	if value, ok := opts.MaxDepth(); ok {
		resolved.maxDepth = normalizeLimit(value)
	}
	if value, ok := opts.MaxAttrs(); ok {
		resolved.maxAttrs = normalizeLimit(value)
	}
	if value, ok := opts.MaxTokenSize(); ok {
		resolved.maxTokenSize = normalizeLimit(value)
	}
	if value, ok := opts.MaxQNameInternEntries(); ok {
		resolved.maxQNameInternEntries = normalizeLimit(value)
	}
	if value, ok := opts.DebugPoisonSpans(); ok {
		resolved.debugPoisonSpans = value
	}
	if value, ok := opts.BufferSize(); ok {
		resolved.bufferSize = normalizeLimit(value)
	}
	if resolved.bufferSize == 0 {
		resolved.bufferSize = defaultBufferSize
	}
	return resolved
}

func normalizeLimit(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func wrapCharsetReaderFromBufio(reader *bufio.Reader, charsetReader func(label string, r io.Reader) (io.Reader, error)) (io.Reader, error) {
	if err := discardUTF8BOM(reader); err != nil {
		return nil, err
	}
	label, err := detectEncoding(reader)
	if err != nil {
		return nil, err
	}
	if label == "" {
		return reader, nil
	}
	if charsetReader == nil {
		return nil, errUnsupportedEncoding
	}
	decoded, err := charsetReader(label, reader)
	if err != nil {
		return nil, err
	}
	if decoded == nil {
		return nil, errUnsupportedEncoding
	}
	return decoded, nil
}

func discardUTF8BOM(r *bufio.Reader) error {
	peek, err := r.Peek(3)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(peek) >= 3 && peek[0] == 0xEF && peek[1] == 0xBB && peek[2] == 0xBF {
		_, _ = r.Discard(3)
	}
	return nil
}

const maxDeclScan = 1024

func detectEncoding(r *bufio.Reader) (string, error) {
	peek, err := r.Peek(4)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return "", err
	}
	if len(peek) >= 2 {
		if peek[0] == 0xFE && peek[1] == 0xFF {
			return "utf-16", nil
		}
		if peek[0] == 0xFF && peek[1] == 0xFE {
			return "utf-16", nil
		}
	}
	if len(peek) >= 4 {
		if bytes.Equal(peek[:4], []byte{0x00, 0x3C, 0x00, 0x3F}) {
			return "utf-16be", nil
		}
		if bytes.Equal(peek[:4], []byte{0x3C, 0x00, 0x3F, 0x00}) {
			return "utf-16le", nil
		}
	}
	return detectXMLDeclEncoding(r)
}

func detectXMLDeclEncoding(r *bufio.Reader) (string, error) {
	const prefix = "<?xml"
	peek, err := r.Peek(len(prefix))
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return "", err
	}
	if len(peek) < len(prefix) || !bytes.Equal(peek, []byte(prefix)) {
		return "", nil
	}
	decl, err := r.Peek(maxDeclScan)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return "", err
	}
	end := bytes.Index(decl, []byte("?>"))
	if end < 0 {
		return "", nil
	}
	label := parseXMLDeclEncoding(decl[:end])
	if label == "" {
		return "", nil
	}
	if isUTF8Label(label) {
		return "", nil
	}
	return label, nil
}

func parseXMLDeclEncoding(decl []byte) string {
	const prefix = "<?xml"
	if !bytes.HasPrefix(decl, []byte(prefix)) {
		return ""
	}
	data := decl[len(prefix):]
	for {
		data = bytes.TrimLeft(data, " \t\r\n")
		if len(data) == 0 {
			return ""
		}
		name, rest := scanXMLDeclName(data)
		if len(name) == 0 {
			return ""
		}
		data = bytes.TrimLeft(rest, " \t\r\n")
		if len(data) == 0 || data[0] != '=' {
			return ""
		}
		data = bytes.TrimLeft(data[1:], " \t\r\n")
		if len(data) == 0 {
			return ""
		}
		quote := data[0]
		if quote != '\'' && quote != '"' {
			return ""
		}
		data = data[1:]
		end := bytes.IndexByte(data, quote)
		if end < 0 {
			return ""
		}
		value := data[:end]
		data = data[end+1:]
		if bytes.EqualFold(name, []byte("encoding")) {
			return string(value)
		}
	}
}

func scanXMLDeclName(data []byte) ([]byte, []byte) {
	if len(data) == 0 || !isNameStartByte(data[0]) {
		return nil, data
	}
	i := 1
	for i < len(data) && isNameByte(data[i]) {
		i++
	}
	return data[:i], data[i:]
}

func isUTF8Label(label string) bool {
	return strings.EqualFold(label, "utf-8") || strings.EqualFold(label, "utf8")
}

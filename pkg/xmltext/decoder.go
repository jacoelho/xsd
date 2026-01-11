package xmltext

import (
	"bufio"
	"bytes"
	"io"
)

const defaultBufferSize = 32 * 1024

// Decoder streams XML tokens with zero-copy spans.
type Decoder struct {
	r       io.Reader
	optsRaw Options
	opts    decoderOptions

	entities entityResolver

	buf          spanBuffer
	scratch      spanBuffer
	coalesce     spanBuffer
	valueBuf     spanBuffer
	attrValueBuf spanBuffer

	pos        int
	baseOffset int64
	eof        bool
	err        error

	line   int
	column int

	attrBuf      []AttrSpan
	attrNeeds    []bool
	attrRaw      []Span
	attrRawNeeds []bool
	attrSeen     map[uint64][]Span
	attrSeenKeys []uint64

	stack          []stackEntry
	rootCount      int64
	rootSeen       bool
	afterRoot      bool
	xmlDeclSeen    bool
	seenNonXMLDecl bool
	directiveSeen  bool

	interner *nameInterner

	pendingToken       Token
	pendingSelfClosing bool
	pendingTokenValid  bool

	pendingEnd      bool
	pendingEndToken Token

	lastKind        Kind
	lastSelfClosing bool
}

type decoderOptions struct {
	charsetReader             func(label string, r io.Reader) (io.Reader, error)
	entityMap                 map[string]string
	resolveEntities           bool
	emitComments              bool
	emitPI                    bool
	emitDirectives            bool
	trackLineColumn           bool
	coalesceCharData          bool
	maxDepth                  int
	maxAttrs                  int
	maxTokenSize              int
	maxQNameInternEntries     int
	maxNamespaceInternEntries int
	debugPoisonSpans          bool
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

	d.entities = entityResolver{custom: d.opts.entityMap, maxTokenSize: d.opts.maxTokenSize}

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
	d.attrSeenKeys = d.attrSeenKeys[:0]
	d.attrSeen = nil
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

	if d.opts.trackLineColumn {
		d.line = 1
		d.column = 1
	} else {
		d.line = 0
		d.column = 0
	}

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
	wrapped, err := wrapCharsetReader(r, d.opts.charsetReader)
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
		return d.pendingToken.kind
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

// ReadToken returns the next XML token.
func (d *Decoder) ReadToken() (Token, error) {
	if d == nil {
		return Token{}, errNilReader
	}
	if d.err != nil {
		return Token{}, d.err
	}
	d.bumpGen()
	if !d.opts.coalesceCharData {
		tok, selfClosing, err := d.nextToken(true)
		if err != nil {
			return Token{}, d.fail(err)
		}
		if d.opts.debugPoisonSpans {
			d.refreshToken(&tok)
		}
		d.lastKind = tok.kind
		d.lastSelfClosing = tok.kind == KindStartElement && selfClosing
		return tok, nil
	}

	first, firstSelfClosing, err := d.nextToken(true)
	if err != nil {
		return Token{}, d.fail(err)
	}
	if first.kind != KindCharData && first.kind != KindCDATA {
		if d.opts.debugPoisonSpans {
			d.refreshToken(&first)
		}
		d.lastKind = first.kind
		d.lastSelfClosing = first.kind == KindStartElement && firstSelfClosing
		return first, nil
	}

	d.coalesce.data = d.coalesce.data[:0]
	needs := first.textNeeds
	rawNeeds := first.textRawNeeds
	startLine, startColumn := first.line, first.column
	d.coalesce.data = append(d.coalesce.data, first.text.bytes()...)

	for {
		next, nextSelfClosing, err := d.nextToken(true)
		if err != nil {
			if err == io.EOF {
				break
			}
			return Token{}, d.fail(err)
		}
		if next.kind != KindCharData && next.kind != KindCDATA {
			d.pendingToken = next
			d.pendingSelfClosing = nextSelfClosing
			d.pendingTokenValid = true
			break
		}
		needs = needs || next.textNeeds
		rawNeeds = rawNeeds || next.textRawNeeds
		d.coalesce.data = append(d.coalesce.data, next.text.bytes()...)
	}

	span := makeSpan(&d.coalesce, 0, len(d.coalesce.data))
	coalesced := Token{
		kind:         KindCharData,
		text:         span,
		textNeeds:    needs,
		textRawNeeds: rawNeeds,
		line:         startLine,
		column:       startColumn,
		raw:          span,
	}
	if d.opts.debugPoisonSpans {
		d.refreshToken(&coalesced)
	}
	d.lastKind = KindCharData
	d.lastSelfClosing = false
	return coalesced, nil
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

	first, _, err := d.nextToken(false)
	if err != nil {
		return nil, d.fail(err)
	}
	if first.kind != KindStartElement {
		switch first.kind {
		case KindCharData:
			if d.opts.resolveEntities {
				return Value(first.text.bytes()), nil
			}
			return Value(first.raw.bytes()), nil
		case KindCDATA:
			return Value(first.raw.bytes()), nil
		default:
			return Value(first.raw.bytes()), nil
		}
	}

	rawStart := first.raw.Start
	rawEnd := first.raw.End
	depth := 1
	needsCopy := d.opts.resolveEntities && tokenNeedsExpansion(first)
	var out []byte
	cursor := rawStart
	if needsCopy {
		out = d.valueBuf.data[:0]
		if first.raw.buf != nil && first.raw.Start > cursor {
			out = append(out, d.buf.data[cursor:first.raw.Start]...)
			cursor = first.raw.Start
		}
		var appendErr error
		out, appendErr = d.appendTokenValue(out, first, &cursor)
		if appendErr != nil {
			return nil, d.fail(appendErr)
		}
	}

	for depth > 0 {
		next, _, err := d.nextToken(false)
		if err != nil {
			return nil, d.fail(err)
		}
		if next.raw.buf != nil && next.raw.End > rawEnd {
			rawEnd = next.raw.End
		}
		switch next.kind {
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
			if next.raw.buf != nil && next.raw.Start > cursor {
				out = append(out, d.buf.data[cursor:next.raw.Start]...)
				cursor = next.raw.Start
			}
			var appendErr error
			out, appendErr = d.appendTokenValue(out, next, &cursor)
			if appendErr != nil {
				return nil, d.fail(appendErr)
			}
			continue
		}
		if needsCopy {
			if next.raw.buf == nil {
				continue
			}
			if next.raw.Start > cursor {
				out = append(out, d.buf.data[cursor:next.raw.Start]...)
				cursor = next.raw.Start
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
	switch tok.kind {
	case KindCharData:
		return tok.textRawNeeds
	case KindStartElement:
		for _, needs := range tok.attrRawNeeds {
			if needs {
				return true
			}
		}
	}
	return false
}

func (d *Decoder) appendTokenValue(dst []byte, tok Token, cursor *int) ([]byte, error) {
	if tok.raw.buf == nil {
		return dst, nil
	}
	rawStart := tok.raw.Start
	rawEnd := tok.raw.End
	if rawStart < *cursor {
		return nil, errInvalidToken
	}
	switch tok.kind {
	case KindCharData:
		if tok.textRawNeeds {
			dst = append(dst, tok.text.bytes()...)
			*cursor = rawEnd
			return dst, nil
		}
	case KindStartElement:
		if len(tok.attrRaw) != len(tok.attrs) {
			return nil, errInvalidToken
		}
		if hasAttrExpansion(tok) {
			pos := rawStart
			for i, rawSpan := range tok.attrRaw {
				if rawSpan.Start < pos || rawSpan.End > rawEnd {
					return nil, errInvalidToken
				}
				dst = append(dst, d.buf.data[pos:rawSpan.Start]...)
				dst = append(dst, tok.attrs[i].ValueSpan.bytes()...)
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
	for _, needs := range tok.attrRawNeeds {
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
			switch next.kind {
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
	if tok.kind != KindStartElement {
		return nil
	}
	depth := 1
	for depth > 0 {
		next, err := d.ReadToken()
		if err != nil {
			return err
		}
		switch next.kind {
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
	resolved := decoderOptions{trackLineColumn: true}
	if value, ok := GetOption(opts, WithCharsetReader); ok {
		resolved.charsetReader = value
	}
	if value, ok := GetOption(opts, WithEntityMap); ok {
		resolved.entityMap = value
	}
	if value, ok := GetOption(opts, ResolveEntities); ok {
		resolved.resolveEntities = value
	}
	if value, ok := GetOption(opts, EmitComments); ok {
		resolved.emitComments = value
	}
	if value, ok := GetOption(opts, EmitPI); ok {
		resolved.emitPI = value
	}
	if value, ok := GetOption(opts, EmitDirectives); ok {
		resolved.emitDirectives = value
	}
	if value, ok := GetOption(opts, TrackLineColumn); ok {
		resolved.trackLineColumn = value
	}
	if value, ok := GetOption(opts, CoalesceCharData); ok {
		resolved.coalesceCharData = value
	}
	if value, ok := GetOption(opts, MaxDepth); ok {
		resolved.maxDepth = normalizeLimit(value)
	}
	if value, ok := GetOption(opts, MaxAttrs); ok {
		resolved.maxAttrs = normalizeLimit(value)
	}
	if value, ok := GetOption(opts, MaxTokenSize); ok {
		resolved.maxTokenSize = normalizeLimit(value)
	}
	if value, ok := GetOption(opts, MaxQNameInternEntries); ok {
		resolved.maxQNameInternEntries = normalizeLimit(value)
	}
	if value, ok := GetOption(opts, MaxNamespaceInternEntries); ok {
		resolved.maxNamespaceInternEntries = normalizeLimit(value)
	}
	if value, ok := GetOption(opts, DebugPoisonSpans); ok {
		resolved.debugPoisonSpans = value
	}
	return resolved
}

func normalizeLimit(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func wrapCharsetReader(r io.Reader, charsetReader func(label string, r io.Reader) (io.Reader, error)) (io.Reader, error) {
	reader := bufio.NewReader(r)
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
	if err != nil && err != io.EOF {
		return err
	}
	if len(peek) >= 3 && peek[0] == 0xEF && peek[1] == 0xBB && peek[2] == 0xBF {
		_, _ = r.Discard(3)
	}
	return nil
}

func detectEncoding(r *bufio.Reader) (string, error) {
	peek, err := r.Peek(4)
	if err != nil && err != io.EOF {
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
	return "", nil
}

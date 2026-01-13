package xmltext

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"unsafe"
)

const defaultBufferSize = 32 * 1024
const attrSeenSmallMax = 8

// Decoder streams XML tokens and copies token bytes into caller-provided buffers.
type Decoder struct {
	buf                spanBuffer
	attrValueBuf       spanBuffer
	coalesce           spanBuffer
	scratch            spanBuffer
	r                  io.Reader
	err                error
	attrSeen           map[uint64]attrBucket
	interner           *nameInterner
	bufioReader        *bufio.Reader
	attrSeenSmall      [attrSeenSmallMax]attrSeenEntry
	entities           entityResolver
	pendingToken       rawToken
	pendingEndToken    rawToken
	optsRaw            Options
	stack              []stackEntry
	attrBuf            []attrSpan
	attrNeeds          []bool
	attrRaw            []span
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
	strict                bool
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
	d.pendingToken = rawToken{}
	d.pendingEndToken = rawToken{}
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

// ReadTokenInto reads the next XML token into dst using buf for storage.
func (d *Decoder) ReadTokenInto(dst *Token, buf *TokenBuffer) error {
	if dst == nil {
		return errNilToken
	}
	if buf == nil {
		return errNilBuffer
	}
	if d == nil {
		return errNilReader
	}
	if d.err != nil {
		return d.err
	}
	var raw rawToken
	if err := d.readTokenIntoRaw(&raw); err != nil {
		return err
	}
	buf.Reset()
	dst.Kind = raw.kind
	dst.Line = raw.line
	dst.Column = raw.column
	dst.IsXMLDecl = raw.isXMLDecl
	dst.TextNeeds = raw.textNeeds
	buf.Name, dst.Name = appendSpanBytes(buf.Name, raw.name.Full)
	buf.Text, dst.Text = appendSpanBytes(buf.Text, raw.text)
	if raw.kind != KindStartElement {
		dst.Attrs = nil
		return nil
	}
	buf.Attrs = buf.Attrs[:0]
	for i, attr := range raw.attrs {
		var dstName []byte
		var dstValue []byte
		buf.AttrName, dstName = appendSpanBytes(buf.AttrName, attr.Name.Full)
		buf.AttrValue, dstValue = appendSpanBytes(buf.AttrValue, attr.ValueSpan)
		buf.Attrs = append(buf.Attrs, Attr{
			Name:       dstName,
			Value:      dstValue,
			ValueNeeds: raw.attrNeeds[i],
		})
	}
	dst.Attrs = buf.Attrs
	return nil
}

func appendSpanBytes(dst []byte, s span) ([]byte, []byte) {
	data := s.bytes()
	if len(data) == 0 {
		return dst, nil
	}
	start := len(dst)
	dst = append(dst, data...)
	return dst, dst[start:]
}

func (d *Decoder) readTokenIntoRaw(dst *rawToken) error {
	d.bumpGen()
	if !d.opts.coalesceCharData {
		selfClosing, err := d.nextTokenInto(dst, true)
		if err != nil {
			return d.fail(err)
		}
		if d.opts.debugPoisonSpans {
			d.refreshToken(dst)
		}
		d.lastKind = dst.kind
		d.lastSelfClosing = dst.kind == KindStartElement && selfClosing
		return nil
	}

	firstSelfClosing, err := d.nextTokenInto(dst, true)
	if err != nil {
		return d.fail(err)
	}
	if dst.kind != KindCharData && dst.kind != KindCDATA {
		if d.opts.debugPoisonSpans {
			d.refreshToken(dst)
		}
		d.lastKind = dst.kind
		d.lastSelfClosing = dst.kind == KindStartElement && firstSelfClosing
		return nil
	}

	nextKind, err := d.peekKind()
	if err != nil && !errors.Is(err, io.EOF) {
		return d.fail(err)
	}
	if errors.Is(err, io.EOF) || (nextKind != KindCharData && nextKind != KindCDATA) {
		setCharDataToken(dst, dst.text, dst.textNeeds, dst.textRawNeeds, dst.line, dst.column, dst.raw)
		if d.opts.debugPoisonSpans {
			d.refreshToken(dst)
		}
		d.lastKind = KindCharData
		d.lastSelfClosing = false
		return nil
	}

	d.coalesce.data = d.coalesce.data[:0]
	needs := dst.textNeeds
	rawNeeds := dst.textRawNeeds
	startLine, startColumn := dst.line, dst.column
	rawStartAbs := d.baseOffset + int64(dst.raw.Start)
	rawEndAbs := d.baseOffset + int64(dst.raw.End)
	d.coalesce.data = append(d.coalesce.data, dst.text.bytesUnsafe()...)

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
		if d.pendingToken.kind != KindCharData && d.pendingToken.kind != KindCDATA {
			d.pendingSelfClosing = nextSelfClosing
			d.pendingTokenValid = true
			break
		}
		needs = needs || d.pendingToken.textNeeds
		rawNeeds = rawNeeds || d.pendingToken.textRawNeeds
		rawEndAbs = d.baseOffset + int64(d.pendingToken.raw.End)
		d.coalesce.data = append(d.coalesce.data, d.pendingToken.text.bytesUnsafe()...)
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

func (d *Decoder) readTokenRaw() (rawToken, error) {
	var tok rawToken
	if err := d.readTokenIntoRaw(&tok); err != nil {
		return rawToken{}, err
	}
	return tok, nil
}

type bufferWriter struct {
	dst   []byte
	n     int
	short bool
}

func (w *bufferWriter) write(data []byte) {
	if w.short || len(data) == 0 {
		return
	}
	if w.n+len(data) > len(w.dst) {
		avail := len(w.dst) - w.n
		if avail > 0 {
			w.n += copy(w.dst[w.n:], data[:avail])
		}
		w.short = true
		return
	}
	w.n += copy(w.dst[w.n:], data)
}

// ReadValueInto writes the next element subtree or token into dst and returns
// the number of bytes written. It returns io.ErrShortBuffer if dst is too small
// and still consumes the value.
func (d *Decoder) ReadValueInto(dst []byte) (int, error) {
	if d == nil {
		return 0, errNilReader
	}
	if d.err != nil {
		return 0, d.err
	}
	d.bumpGen()

	first, err := d.nextToken(false)
	if err != nil {
		return 0, d.fail(err)
	}
	if first.kind != KindStartElement {
		writer := bufferWriter{dst: dst}
		var data []byte
		switch first.kind {
		case KindCharData:
			if d.opts.resolveEntities {
				data = first.text.bytesUnsafe()
			} else {
				data = first.raw.bytesUnsafe()
			}
		case KindCDATA:
			data = first.raw.bytesUnsafe()
		default:
			data = first.raw.bytesUnsafe()
		}
		writer.write(data)
		if writer.short {
			return writer.n, io.ErrShortBuffer
		}
		return writer.n, nil
	}

	writer := bufferWriter{dst: dst}
	rawStart := first.raw.Start
	rawEnd := first.raw.End
	depth := 1
	resolve := d.opts.resolveEntities
	needsCopy := resolve && tokenNeedsExpansion(first)
	cursor := rawStart
	if resolve && needsCopy {
		if first.raw.buf != nil && first.raw.Start > cursor {
			writer.write(d.buf.data[cursor:first.raw.Start])
			cursor = first.raw.Start
		}
		if appendErr := d.appendTokenValue(&writer, first, &cursor); appendErr != nil {
			return writer.n, d.fail(appendErr)
		}
	}

	for depth > 0 {
		next, err := d.nextToken(false)
		if err != nil {
			return writer.n, d.fail(err)
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
		if !resolve {
			continue
		}
		if !needsCopy && tokenNeedsExpansion(next) {
			needsCopy = true
			cursor = rawStart
			if next.raw.buf != nil && next.raw.Start > cursor {
				writer.write(d.buf.data[cursor:next.raw.Start])
				cursor = next.raw.Start
			}
			if appendErr := d.appendTokenValue(&writer, next, &cursor); appendErr != nil {
				return writer.n, d.fail(appendErr)
			}
			continue
		}
		if needsCopy {
			if next.raw.buf == nil {
				continue
			}
			if next.raw.Start > cursor {
				writer.write(d.buf.data[cursor:next.raw.Start])
				cursor = next.raw.Start
			}
			if appendErr := d.appendTokenValue(&writer, next, &cursor); appendErr != nil {
				return writer.n, d.fail(appendErr)
			}
		}
	}

	if !resolve || !needsCopy {
		if rawStart < 0 || rawEnd < rawStart || rawEnd > len(d.buf.data) {
			return writer.n, d.fail(errInvalidToken)
		}
		writer.write(d.buf.data[rawStart:rawEnd])
		if writer.short {
			return writer.n, io.ErrShortBuffer
		}
		return writer.n, nil
	}
	if cursor < rawEnd {
		writer.write(d.buf.data[cursor:rawEnd])
	}
	if writer.short {
		return writer.n, io.ErrShortBuffer
	}
	return writer.n, nil
}

// UnescapeInto expands entity references in data into dst and returns the
// number of bytes written. It returns io.ErrShortBuffer if dst is too small.
func (d *Decoder) UnescapeInto(dst []byte, data []byte) (int, error) {
	if d == nil {
		return 0, errNilReader
	}
	if len(data) == 0 {
		return 0, nil
	}
	return unescapeInto(dst, data, &d.entities, d.opts.maxTokenSize)
}

func tokenNeedsExpansion(tok rawToken) bool {
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

func (d *Decoder) appendTokenValue(writer *bufferWriter, tok rawToken, cursor *int) error {
	if tok.raw.buf == nil {
		return nil
	}
	rawStart := tok.raw.Start
	rawEnd := tok.raw.End
	if rawStart < *cursor {
		return errInvalidToken
	}
	switch tok.kind {
	case KindCharData:
		if tok.textRawNeeds {
			writer.write(tok.text.bytesUnsafe())
			*cursor = rawEnd
			return nil
		}
	case KindStartElement:
		if len(tok.attrRaw) != len(tok.attrs) {
			return errInvalidToken
		}
		if hasAttrExpansion(tok) {
			pos := rawStart
			for i, rawSpan := range tok.attrRaw {
				if rawSpan.Start < pos || rawSpan.End > rawEnd {
					return errInvalidToken
				}
				writer.write(d.buf.data[pos:rawSpan.Start])
				writer.write(tok.attrs[i].ValueSpan.bytesUnsafe())
				pos = rawSpan.End
			}
			writer.write(d.buf.data[pos:rawEnd])
			*cursor = rawEnd
			return nil
		}
	}
	writer.write(d.buf.data[rawStart:rawEnd])
	*cursor = rawEnd
	return nil
}

func hasAttrExpansion(tok rawToken) bool {
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
			_, err := d.readTokenRaw()
			return err
		}
		depth := 1
		for depth > 0 {
			next, err := d.readTokenRaw()
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
		_, err := d.readTokenRaw()
		return err
	case KindEndElement:
		return nil
	}

	tok, err := d.readTokenRaw()
	if err != nil {
		return err
	}
	if tok.kind != KindStartElement {
		return nil
	}
	depth := 1
	for depth > 0 {
		next, err := d.readTokenRaw()
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

func (d *Decoder) unreadBuffer() []byte {
	if d == nil || d.pos >= len(d.buf.data) {
		return nil
	}
	return d.buf.data[d.pos:]
}

func (d *Decoder) spanBytes(s span) []byte {
	return s.bytes()
}

func (d *Decoder) spanString(s span) string {
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

// StackPointer renders the current stack path using local names.
func (d *Decoder) StackPointer() string {
	if d == nil {
		return ""
	}
	return string(d.appendStackPointer(nil))
}

func (d *Decoder) appendStackPointer(dst []byte) []byte {
	for _, entry := range d.stack {
		dst = append(dst, '/')
		name := entry.name.Local.bytes()
		dst = append(dst, name...)
		dst = append(dst, '[')
		dst = strconv.AppendInt(dst, entry.index, 10)
		dst = append(dst, ']')
	}
	return dst
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
	if value, ok := opts.Strict(); ok {
		resolved.strict = value
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

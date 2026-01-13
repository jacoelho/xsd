package xmltext

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

// Pre-computed byte slices to avoid allocations in hot paths
var (
	litXML      = []byte("xml")
	litPIEnd    = []byte("?>")
	litComStart = []byte("<!--")
	litComEnd   = []byte("-->")
	litDDash    = []byte("--")
	litCDStart  = []byte("<![CDATA[")
	litCDEnd    = []byte("]]>")
)

var whitespaceLUT = [256]bool{
	'\t': true,
	'\n': true,
	'\r': true,
	' ':  true,
}

type charDataByteClass uint8

const (
	charDataByteOK charDataByteClass = iota
	charDataByteAmp
	charDataByteRightBracket
	charDataByteGreater
	charDataByteInvalid
	charDataByteNonASCII
)

var charDataByteClassLUT = func() [256]charDataByteClass {
	var lut [256]charDataByteClass
	for i := 0; i < len(lut); i++ {
		b := byte(i)
		switch {
		case b >= utf8.RuneSelf:
			lut[i] = charDataByteNonASCII
		case b == '&':
			lut[i] = charDataByteAmp
		case b == ']':
			lut[i] = charDataByteRightBracket
		case b == '>':
			lut[i] = charDataByteGreater
		case b < 0x20 && b != 0x9 && b != 0xA && b != 0xD:
			lut[i] = charDataByteInvalid
		default:
			lut[i] = charDataByteOK
		}
	}
	return lut
}()

type stackEntry struct {
	name       qnameSpan
	index      int64
	childCount int64
}

type attrBucket struct {
	spans []span
	gen   uint32
}

type attrSeenEntry struct {
	span span
	hash uint64
}

func (d *Decoder) bumpGen() {
	if !d.opts.debugPoisonSpans {
		return
	}
	d.buf.gen++
	d.scratch.gen++
	d.coalesce.gen++
	d.attrValueBuf.gen++
}

func (d *Decoder) fail(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) {
		d.err = io.EOF
		return io.EOF
	}
	var syntaxErr *SyntaxError
	if errors.As(err, &syntaxErr) {
		d.err = err
		return err
	}
	syntax := d.syntaxError(err)
	d.err = syntax
	return syntax
}

func (d *Decoder) syntaxError(err error) error {
	if d == nil {
		return err
	}
	offset := d.baseOffset + int64(d.pos)
	line := 0
	column := 0
	if d.opts.trackLineColumn {
		line = d.line
		column = d.column
	}
	return &SyntaxError{
		Offset:  offset,
		Line:    line,
		Column:  column,
		Path:    d.StackPointer(),
		Snippet: d.snippet(d.pos),
		Err:     err,
	}
}

func (d *Decoder) snippet(pos int) []byte {
	const window = 32
	if pos < 0 || pos > len(d.buf.data) {
		return nil
	}
	start := pos - window
	if start < 0 {
		start = 0
	}
	end := pos + window
	if end > len(d.buf.data) {
		end = len(d.buf.data)
	}
	if start >= end {
		return nil
	}
	out := make([]byte, end-start)
	copy(out, d.buf.data[start:end])
	return out
}

func (d *Decoder) refreshToken(tok *rawToken) {
	if tok == nil {
		return
	}
	refreshSpan(&tok.name.Full)
	refreshSpan(&tok.name.Prefix)
	refreshSpan(&tok.name.Local)
	refreshSpan(&tok.text)
	refreshSpan(&tok.raw)
	for i := range tok.attrRaw {
		refreshSpan(&tok.attrRaw[i])
	}
	for i := range tok.attrs {
		attr := &tok.attrs[i]
		refreshSpan(&attr.Name.Full)
		refreshSpan(&attr.Name.Prefix)
		refreshSpan(&attr.Name.Local)
		refreshSpan(&attr.ValueSpan)
	}
}

func refreshSpan(span *span) {
	if span == nil || span.buf == nil {
		return
	}
	span.gen = span.buf.gen
}

func (d *Decoder) nextToken(allowCompact bool) (rawToken, error) {
	var tok rawToken
	if _, err := d.nextTokenInto(&tok, allowCompact); err != nil {
		return rawToken{}, err
	}
	return tok, nil
}

func (d *Decoder) nextTokenInto(dst *rawToken, allowCompact bool) (bool, error) {
	if d.pendingTokenValid {
		copyToken(dst, &d.pendingToken)
		selfClosing := d.pendingSelfClosing
		d.pendingTokenValid = false
		d.pendingSelfClosing = false
		return selfClosing, nil
	}
	if d.pendingEnd {
		copyToken(dst, &d.pendingEndToken)
		d.pendingEnd = false
		if err := d.popStackInterned(dst.name); err != nil {
			return false, err
		}
		return false, nil
	}
	for {
		selfClosing, err := d.scanTokenInto(dst, allowCompact)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(d.stack) > 0 {
					return false, errUnexpectedEOF
				}
				if !d.rootSeen {
					return false, errMissingRoot
				}
				return false, io.EOF
			}
			return false, err
		}
		if err := d.applyToken(dst, selfClosing); err != nil {
			return false, err
		}
		switch dst.kind {
		case KindComment:
			if !d.opts.emitComments {
				continue
			}
		case KindPI:
			if !d.opts.emitPI {
				continue
			}
		case KindDirective:
			if !d.opts.emitDirectives {
				continue
			}
		}
		return selfClosing, nil
	}
}

func (d *Decoder) applyToken(tok *rawToken, selfClosing bool) error {
	if err := d.checkTokenPlacement(tok); err != nil {
		return err
	}
	switch tok.kind {
	case KindStartElement:
		interned := d.internQName(tok.name)
		tok.name = interned
		if err := d.pushStack(interned); err != nil {
			return err
		}
		if selfClosing {
			d.pendingEnd = true
			d.pendingEndToken = rawToken{
				kind:   KindEndElement,
				name:   interned,
				line:   d.line,
				column: d.column,
			}
		}
	case KindEndElement:
		interned, err := d.popStackRaw(tok.name)
		if err != nil {
			return err
		}
		tok.name = interned
		if len(d.stack) == 0 {
			d.afterRoot = true
		}
	}
	return nil
}

func (d *Decoder) checkTokenPlacement(tok *rawToken) error {
	if tok == nil {
		return nil
	}
	if tok.isXMLDecl {
		if d.xmlDeclSeen {
			return errDuplicateXMLDecl
		}
		if d.seenNonXMLDecl {
			return errMisplacedXMLDecl
		}
		d.xmlDeclSeen = true
	} else {
		d.seenNonXMLDecl = true
	}
	switch tok.kind {
	case KindDirective:
		if d.rootSeen || d.afterRoot {
			return errMisplacedDirective
		}
		if d.directiveSeen {
			return errDuplicateDirective
		}
		d.directiveSeen = true
	case KindCharData:
		if len(d.stack) == 0 {
			whitespace, err := d.isWhitespaceCharData(tok)
			if err != nil {
				return err
			}
			if !whitespace {
				return errContentOutsideRoot
			}
		}
	case KindCDATA:
		if len(d.stack) == 0 {
			return errContentOutsideRoot
		}
	case KindStartElement:
		if d.afterRoot || (d.rootSeen && len(d.stack) == 0) {
			return errMultipleRoots
		}
		if len(d.stack) == 0 {
			d.rootSeen = true
		}
	}
	return nil
}

func unescapeIntoSpanBuffer(buf *spanBuffer, start int, data []byte, resolver *entityResolver, maxTokenSize int) ([]byte, error) {
	for {
		scratch := buf.data[start:cap(buf.data)]
		n, err := unescapeInto(scratch, data, resolver, maxTokenSize)
		if err == nil {
			end := start + n
			buf.data = buf.data[:end]
			return buf.data, nil
		}
		if !errors.Is(err, io.ErrShortBuffer) {
			return nil, err
		}
		newCap := cap(buf.data) * 2
		minCap := start + len(data)
		if maxTokenSize > 0 {
			limit := start + maxTokenSize
			if minCap > limit {
				minCap = limit
			}
		}
		if newCap < minCap {
			newCap = minCap
		}
		if newCap == 0 {
			newCap = minCap
		}
		next := make([]byte, start, newCap)
		copy(next, buf.data[:start])
		buf.data = next
	}
}

func (d *Decoder) isWhitespaceCharData(tok *rawToken) (bool, error) {
	if tok == nil {
		return true, nil
	}
	data := tok.text.bytesUnsafe()
	if len(data) == 0 {
		return true, nil
	}
	if !tok.textNeeds {
		return isWhitespaceBytes(data), nil
	}
	out, err := unescapeIntoSpanBuffer(&d.scratch, 0, data, &d.entities, d.opts.maxTokenSize)
	if err != nil {
		return false, err
	}
	return isWhitespaceBytes(out), nil
}

func (d *Decoder) internQName(name qnameSpan) qnameSpan {
	if d.interner == nil {
		d.interner = newNameInterner(d.opts.maxQNameInternEntries)
	}
	return d.interner.internQName(name)
}

func (d *Decoder) internQNameHash(name qnameSpan, hash uint64) qnameSpan {
	if d.interner == nil {
		d.interner = newNameInterner(d.opts.maxQNameInternEntries)
	}
	return d.interner.internQNameHash(name, hash)
}

func (d *Decoder) pushStack(name qnameSpan) error {
	if d.opts.maxDepth > 0 && len(d.stack)+1 > d.opts.maxDepth {
		return errDepthLimit
	}
	var index int64
	if len(d.stack) == 0 {
		d.rootCount++
		index = d.rootCount
	} else {
		parent := &d.stack[len(d.stack)-1]
		parent.childCount++
		index = parent.childCount
	}
	d.stack = append(d.stack, stackEntry{
		name:       name,
		index:      index,
		childCount: 0,
	})
	return nil
}

func (d *Decoder) popStackRaw(name qnameSpan) (qnameSpan, error) {
	if len(d.stack) == 0 {
		return qnameSpan{}, errMismatchedEndTag
	}
	top := d.stack[len(d.stack)-1]
	if !bytes.Equal(name.Full.bytesUnsafe(), top.name.Full.bytesUnsafe()) {
		return qnameSpan{}, errMismatchedEndTag
	}
	d.stack = d.stack[:len(d.stack)-1]
	return top.name, nil
}

func (d *Decoder) popStackInterned(name qnameSpan) error {
	if len(d.stack) == 0 {
		return errMismatchedEndTag
	}
	top := d.stack[len(d.stack)-1]
	if !bytes.Equal(name.Full.bytesUnsafe(), top.name.Full.bytesUnsafe()) {
		return errMismatchedEndTag
	}
	d.stack = d.stack[:len(d.stack)-1]
	return nil
}

func (d *Decoder) scanTokenInto(dst *rawToken, allowCompact bool) (bool, error) {
	if allowCompact {
		d.compactIfNeeded()
	}
	d.scratch.data = d.scratch.data[:0]
	d.attrBuf = d.attrBuf[:0]
	d.attrNeeds = d.attrNeeds[:0]
	d.attrRaw = d.attrRaw[:0]
	d.attrRawNeeds = d.attrRawNeeds[:0]
	d.attrValueBuf.data = d.attrValueBuf.data[:0]
	d.resetAttrSeen()

	if err := d.ensureIndex(d.pos, allowCompact); err != nil {
		return false, err
	}
	if d.pos >= len(d.buf.data) {
		return false, io.EOF
	}
	if d.buf.data[d.pos] != '<' {
		return d.scanCharDataInto(dst, allowCompact)
	}

	if err := d.ensureIndex(d.pos+1, allowCompact); err != nil {
		if errors.Is(err, io.EOF) {
			return false, errUnexpectedEOF
		}
		return false, err
	}

	switch d.buf.data[d.pos+1] {
	case '/':
		return d.scanEndTagInto(dst, allowCompact)
	case '?':
		return d.scanPIInto(dst, allowCompact)
	case '!':
		return d.scanBangInto(dst, allowCompact)
	default:
		return d.scanStartTagInto(dst, allowCompact)
	}
}

func setCharDataToken(dst *rawToken, text span, needs, rawNeeds bool, line, column int, raw span) {
	dst.kind = KindCharData
	dst.name = qnameSpan{}
	dst.attrs = nil
	dst.attrNeeds = nil
	dst.attrRaw = nil
	dst.attrRawNeeds = nil
	dst.text = text
	dst.textNeeds = needs
	dst.textRawNeeds = rawNeeds
	dst.line = line
	dst.column = column
	dst.raw = raw
	dst.isXMLDecl = false
}

func copyToken(dst *rawToken, src *rawToken) {
	if dst == nil || src == nil {
		return
	}
	dst.kind = src.kind
	dst.name = src.name
	dst.attrs = src.attrs
	dst.attrNeeds = src.attrNeeds
	dst.attrRaw = src.attrRaw
	dst.attrRawNeeds = src.attrRawNeeds
	dst.text = src.text
	dst.textNeeds = src.textNeeds
	dst.textRawNeeds = src.textRawNeeds
	dst.line = src.line
	dst.column = src.column
	dst.raw = src.raw
	dst.isXMLDecl = src.isXMLDecl
}

func setTextToken(dst *rawToken, kind Kind, text span, line, column int, raw span, isXMLDecl bool) {
	dst.kind = kind
	dst.name = qnameSpan{}
	dst.attrs = nil
	dst.attrNeeds = nil
	dst.attrRaw = nil
	dst.attrRawNeeds = nil
	dst.text = text
	dst.textNeeds = false
	dst.textRawNeeds = false
	dst.line = line
	dst.column = column
	dst.raw = raw
	dst.isXMLDecl = isXMLDecl
}

func (d *Decoder) scanCharDataInto(dst *rawToken, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	start := d.pos
	for {
		idx := bytes.IndexByte(d.buf.data[d.pos:], '<')
		if idx >= 0 {
			end := d.pos + idx
			if d.opts.maxTokenSize > 0 && end-start > d.opts.maxTokenSize {
				return false, errTokenTooLarge
			}
			d.advanceTo(end)
			span := makeSpan(&d.buf, start, end)
			textSpan, needs, rawNeeds, err := d.resolveText(span)
			if err != nil {
				return false, err
			}
			setCharDataToken(dst, textSpan, needs, rawNeeds, startLine, startColumn, span)
			return false, nil
		}
		if d.eof {
			end := len(d.buf.data)
			if end == start {
				return false, io.EOF
			}
			if d.opts.maxTokenSize > 0 && end-start > d.opts.maxTokenSize {
				return false, errTokenTooLarge
			}
			d.advanceTo(end)
			span := makeSpan(&d.buf, start, end)
			textSpan, needs, rawNeeds, err := d.resolveText(span)
			if err != nil {
				return false, err
			}
			setCharDataToken(dst, textSpan, needs, rawNeeds, startLine, startColumn, span)
			return false, nil
		}
		if err := d.readMore(allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				d.eof = true
				continue
			}
			return false, err
		}
	}
}

func scanCharDataSpanUntilEntity(data []byte, start int) (int, error) {
	if start < 0 {
		return -1, errInvalidChar
	}
	size := len(data)
	if start >= size {
		return -1, nil
	}
	bracketRun := 0
	for i := start; i < size; {
		switch charDataByteClassLUT[data[i]] {
		case charDataByteOK:
			bracketRun = 0
			i++
		case charDataByteAmp:
			return i, nil
		case charDataByteRightBracket:
			bracketRun++
			i++
		case charDataByteGreater:
			if bracketRun >= 2 {
				return -1, errInvalidToken
			}
			bracketRun = 0
			i++
		case charDataByteInvalid:
			return -1, errInvalidChar
		case charDataByteNonASCII:
			bracketRun = 0
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size == 1 {
				return -1, errInvalidChar
			}
			if !isValidXMLChar(r) {
				return -1, errInvalidChar
			}
			i += size
		default:
			bracketRun = 0
			i++
		}
	}
	return -1, nil
}

func unescapeCharDataInto(dst []byte, data []byte, resolver *entityResolver, maxTokenSize int) ([]byte, bool, error) {
	rawNeeds := false
	bracketRun := 0
	start := 0
	for i := 0; i < len(data); {
		switch charDataByteClassLUT[data[i]] {
		case charDataByteOK:
			bracketRun = 0
			i++
		case charDataByteAmp:
			if !rawNeeds {
				required := len(dst) + len(data)
				if cap(dst) < required {
					next := make([]byte, len(dst), required)
					copy(next, dst)
					dst = next
				}
			}
			rawNeeds = true
			if start < i {
				dst = append(dst, data[start:i]...)
				if maxTokenSize > 0 && len(dst) > maxTokenSize {
					return nil, rawNeeds, errTokenTooLarge
				}
			}
			consumed, replacement, r, isNumeric, err := parseEntityRef(data, i, resolver)
			if err != nil {
				return nil, rawNeeds, err
			}
			if isNumeric {
				dst = utf8.AppendRune(dst, r)
			} else {
				dst = append(dst, replacement...)
			}
			if maxTokenSize > 0 && len(dst) > maxTokenSize {
				return nil, rawNeeds, errTokenTooLarge
			}
			i += consumed
			start = i
			bracketRun = 0
		case charDataByteRightBracket:
			bracketRun++
			i++
		case charDataByteGreater:
			if bracketRun >= 2 {
				return nil, rawNeeds, errInvalidToken
			}
			bracketRun = 0
			i++
		case charDataByteInvalid:
			return nil, rawNeeds, errInvalidChar
		case charDataByteNonASCII:
			bracketRun = 0
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size == 1 {
				return nil, rawNeeds, errInvalidChar
			}
			if !isValidXMLChar(r) {
				return nil, rawNeeds, errInvalidChar
			}
			i += size
		default:
			bracketRun = 0
			i++
		}
	}
	if !rawNeeds {
		return dst, false, nil
	}
	if start < len(data) {
		dst = append(dst, data[start:]...)
		if maxTokenSize > 0 && len(dst) > maxTokenSize {
			return nil, rawNeeds, errTokenTooLarge
		}
	}
	return dst, rawNeeds, nil
}

func scanCharDataSpanParse(data []byte, resolver *entityResolver) (bool, error) {
	rawNeeds := false
	for i := 0; i < len(data); {
		ampIdx, err := scanCharDataSpanUntilEntity(data, i)
		if err != nil {
			return rawNeeds, err
		}
		if ampIdx < 0 {
			return rawNeeds, nil
		}
		rawNeeds = true
		consumed, _, _, _, err := parseEntityRef(data, ampIdx, resolver)
		if err != nil {
			return rawNeeds, err
		}
		i = ampIdx + consumed
	}
	return rawNeeds, nil
}

func (d *Decoder) resolveText(textSpan span) (span, bool, bool, error) {
	data := textSpan.bytesUnsafe()
	if len(data) == 0 {
		return textSpan, false, false, nil
	}
	if !d.opts.resolveEntities {
		rawNeeds, err := scanCharDataSpanParse(data, &d.entities)
		if err != nil {
			return span{}, false, false, err
		}
		if !rawNeeds {
			return textSpan, false, false, nil
		}
		return textSpan, true, true, nil
	}
	out, rawNeeds, err := unescapeCharDataInto(d.scratch.data[:0], data, &d.entities, d.opts.maxTokenSize)
	if err != nil {
		return span{}, false, false, err
	}
	if !rawNeeds {
		return textSpan, false, false, nil
	}
	d.scratch.data = out
	return makeSpan(&d.scratch, 0, len(out)), false, true, nil
}

func (d *Decoder) scanStartTagInto(dst *rawToken, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advanceRaw(1)

	name, err := d.scanQName(allowCompact)
	if err != nil {
		return false, err
	}

	space := d.skipWhitespace(allowCompact)
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return false, errUnexpectedEOF
			}
			return false, err
		}
		b := d.buf.data[d.pos]
		if b == '/' || b == '>' {
			break
		}
		if !space {
			return false, errInvalidToken
		}
		attrName, err := d.scanQName(allowCompact)
		if err != nil {
			return false, err
		}
		hash, err := d.markAttrSeen(attrName)
		if err != nil {
			return false, err
		}
		d.skipWhitespace(allowCompact)
		if err := d.expectByte('=', allowCompact); err != nil {
			return false, err
		}
		d.skipWhitespace(allowCompact)
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return false, errUnexpectedEOF
			}
			return false, err
		}
		quote := d.buf.data[d.pos]
		if quote != '\'' && quote != '"' {
			return false, errInvalidToken
		}
		d.advance(1)
		rawSpan, valueSpan, needs, rawNeeds, err := d.scanAttrValue(quote, allowCompact)
		if err != nil {
			return false, err
		}

		attrName = d.internQNameHash(attrName, hash)
		d.attrBuf = append(d.attrBuf, attrSpan{Name: attrName, ValueSpan: valueSpan})
		d.attrNeeds = append(d.attrNeeds, needs)
		d.attrRaw = append(d.attrRaw, rawSpan)
		d.attrRawNeeds = append(d.attrRawNeeds, rawNeeds)
		if d.opts.maxAttrs > 0 && len(d.attrBuf) > d.opts.maxAttrs {
			return false, errAttrLimit
		}
		space = d.skipWhitespace(allowCompact)
	}

	selfClosing := false
	switch d.buf.data[d.pos] {
	case '/':
		selfClosing = true
		d.advance(1)
		if err := d.expectByte('>', allowCompact); err != nil {
			return false, err
		}
	case '>':
		d.advance(1)
	default:
		return false, errInvalidToken
	}

	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return false, errTokenTooLarge
	}

	dst.kind = KindStartElement
	dst.name = name
	dst.attrs = d.attrBuf
	dst.attrNeeds = d.attrNeeds
	dst.attrRaw = d.attrRaw
	dst.attrRawNeeds = d.attrRawNeeds
	dst.text = span{}
	dst.textNeeds = false
	dst.textRawNeeds = false
	dst.line = startLine
	dst.column = startColumn
	dst.raw = makeSpan(&d.buf, rawStart, rawEnd)
	dst.isXMLDecl = false
	return selfClosing, nil
}

func (d *Decoder) scanAttrValue(quote byte, allowCompact bool) (span, span, bool, bool, error) {
	start := d.pos
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return span{}, span{}, false, false, errUnexpectedEOF
			}
			return span{}, span{}, false, false, err
		}
		data := d.buf.data[d.pos:]
		termIdx := -1
		for i := 0; i < len(data); i++ {
			b := data[i]
			if b == '<' || b == quote {
				termIdx = i
				break
			}
		}

		if termIdx >= 0 {
			if data[termIdx] == '<' {
				return span{}, span{}, false, false, errInvalidToken
			}
			if d.opts.maxTokenSize > 0 && d.pos-start+termIdx > d.opts.maxTokenSize {
				return span{}, span{}, false, false, errTokenTooLarge
			}
			d.advanceTo(d.pos + termIdx)
			break
		}

		d.advanceTo(len(d.buf.data))
		if d.opts.maxTokenSize > 0 && d.pos-start > d.opts.maxTokenSize {
			return span{}, span{}, false, false, errTokenTooLarge
		}

		if err := d.readMore(allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return span{}, span{}, false, false, errUnexpectedEOF
			}
			return span{}, span{}, false, false, err
		}
	}

	end := d.pos
	d.advanceRaw(1)
	rawSpan := makeSpan(&d.buf, start, end)
	data := rawSpan.bytesUnsafe()
	rawNeeds, err := scanXMLCharsUntilEntity(data)
	if err != nil {
		return span{}, span{}, false, false, err
	}
	if !rawNeeds {
		return rawSpan, rawSpan, false, false, nil
	}
	if d.opts.resolveEntities {
		startOut := len(d.attrValueBuf.data)
		out, err := unescapeIntoSpanBuffer(&d.attrValueBuf, startOut, data, &d.entities, d.opts.maxTokenSize)
		if err != nil {
			return span{}, span{}, false, false, err
		}
		if err := validateXMLChars(out[startOut:]); err != nil {
			return span{}, span{}, false, false, err
		}
		valueSpan := makeSpan(&d.attrValueBuf, startOut, len(out))
		return rawSpan, valueSpan, false, true, nil
	}
	if err := validateXMLText(data, &d.entities); err != nil {
		return span{}, span{}, false, false, err
	}
	return rawSpan, rawSpan, true, true, nil
}

func (d *Decoder) resetAttrSeen() {
	d.attrSeenSmallCount = 0
	d.attrSeenGen++
	if d.attrSeenGen != 0 {
		return
	}
	if len(d.attrSeen) == 0 {
		return
	}
	for key := range d.attrSeen {
		delete(d.attrSeen, key)
	}
	d.attrSeenGen = 1
}

func (d *Decoder) markAttrSeen(name qnameSpan) (uint64, error) {
	data := name.Full.bytesUnsafe()
	if len(data) == 0 {
		return 0, errInvalidName
	}
	hash := hashBytes(data)
	if d.attrSeenSmallCount < attrSeenSmallMax {
		for i := 0; i < d.attrSeenSmallCount; i++ {
			entry := d.attrSeenSmall[i]
			if entry.hash == hash && bytes.Equal(entry.span.bytesUnsafe(), data) {
				return 0, errDuplicateAttr
			}
		}
		d.attrSeenSmall[d.attrSeenSmallCount] = attrSeenEntry{span: name.Full, hash: hash}
		d.attrSeenSmallCount++
		return hash, nil
	}
	if d.attrSeenSmallCount == attrSeenSmallMax {
		if d.attrSeen == nil {
			d.attrSeen = make(map[uint64]attrBucket, attrSeenSmallMax*2)
		}
		for i := 0; i < d.attrSeenSmallCount; i++ {
			entry := d.attrSeenSmall[i]
			if len(entry.span.bytesUnsafe()) == 0 {
				continue
			}
			bucket := d.attrSeen[entry.hash]
			if bucket.gen != d.attrSeenGen {
				bucket.gen = d.attrSeenGen
				bucket.spans = bucket.spans[:0]
			}
			bucket.spans = append(bucket.spans, entry.span)
			d.attrSeen[entry.hash] = bucket
		}
		d.attrSeenSmallCount++
	}
	if d.attrSeen == nil {
		d.attrSeen = make(map[uint64]attrBucket, 8)
	}
	bucket := d.attrSeen[hash]
	if bucket.gen != d.attrSeenGen {
		bucket.gen = d.attrSeenGen
		bucket.spans = bucket.spans[:0]
	}
	for _, span := range bucket.spans {
		if bytes.Equal(span.bytesUnsafe(), data) {
			return 0, errDuplicateAttr
		}
	}
	bucket.spans = append(bucket.spans, name.Full)
	d.attrSeen[hash] = bucket
	return hash, nil
}

func (d *Decoder) scanEndTagInto(dst *rawToken, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advanceRaw(2)
	name, err := d.scanQName(allowCompact)
	if err != nil {
		return false, err
	}
	d.skipWhitespace(allowCompact)
	if err := d.expectByte('>', allowCompact); err != nil {
		return false, err
	}
	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return false, errTokenTooLarge
	}
	dst.kind = KindEndElement
	dst.name = name
	dst.attrs = nil
	dst.attrNeeds = nil
	dst.attrRaw = nil
	dst.attrRawNeeds = nil
	dst.text = span{}
	dst.textNeeds = false
	dst.textRawNeeds = false
	dst.line = startLine
	dst.column = startColumn
	dst.raw = makeSpan(&d.buf, rawStart, rawEnd)
	dst.isXMLDecl = false
	return false, nil
}

func (d *Decoder) scanPIInto(dst *rawToken, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advanceRaw(2)
	textStart := d.pos
	targetSpan, err := d.scanName(allowCompact)
	if err != nil {
		return false, err
	}
	isXMLDecl := bytes.EqualFold(targetSpan.bytesUnsafe(), litXML)
	hasSpace := d.skipWhitespace(allowCompact)
	if !hasSpace {
		ok, err := d.matchLiteral(litPIEnd, allowCompact)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, errInvalidPI
		}
		if isXMLDecl {
			return false, errInvalidPI
		}
		textEnd := d.pos
		d.advanceRaw(2)
		rawEnd := d.pos
		if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
			return false, errTokenTooLarge
		}
		textSpan := makeSpan(&d.buf, textStart, textEnd)
		if err := validateXMLChars(textSpan.bytesUnsafe()); err != nil {
			return false, err
		}
		setTextToken(dst, KindPI, textSpan, startLine, startColumn, makeSpan(&d.buf, rawStart, rawEnd), isXMLDecl)
		return false, nil
	}
	endIdx, err := d.scanUntil(litPIEnd, allowCompact)
	if err != nil {
		return false, err
	}
	textEnd := endIdx
	d.advanceTo(endIdx + 2)
	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return false, errTokenTooLarge
	}
	textSpan := makeSpan(&d.buf, textStart, textEnd)
	if err := validateXMLChars(textSpan.bytesUnsafe()); err != nil {
		return false, err
	}
	setTextToken(dst, KindPI, textSpan, startLine, startColumn, makeSpan(&d.buf, rawStart, rawEnd), isXMLDecl)
	return false, nil
}

func (d *Decoder) scanBangInto(dst *rawToken, allowCompact bool) (bool, error) {
	if ok, err := d.matchLiteral(litComStart, allowCompact); err != nil {
		return false, err
	} else if ok {
		return d.scanCommentInto(dst, allowCompact)
	}
	if ok, err := d.matchLiteral(litCDStart, allowCompact); err != nil {
		return false, err
	} else if ok {
		return d.scanCDATAInto(dst, allowCompact)
	}
	return d.scanDirectiveInto(dst, allowCompact)
}

func (d *Decoder) scanCommentInto(dst *rawToken, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advanceRaw(len("<!--"))
	textStart := d.pos
	endIdx, err := d.scanUntil(litComEnd, allowCompact)
	if err != nil {
		return false, err
	}
	textEnd := endIdx
	d.advanceTo(endIdx + len("-->"))
	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return false, errTokenTooLarge
	}
	textSpan := makeSpan(&d.buf, textStart, textEnd)
	textData := textSpan.bytesUnsafe()
	if bytes.Contains(textData, litDDash) || (len(textData) > 0 && textData[len(textData)-1] == '-') {
		return false, errInvalidComment
	}
	if err := validateXMLChars(textData); err != nil {
		return false, err
	}
	setTextToken(dst, KindComment, textSpan, startLine, startColumn, makeSpan(&d.buf, rawStart, rawEnd), false)
	return false, nil
}

func (d *Decoder) scanCDATAInto(dst *rawToken, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advanceRaw(len("<![CDATA["))
	textStart := d.pos
	endIdx, err := d.scanUntil(litCDEnd, allowCompact)
	if err != nil {
		return false, err
	}
	textEnd := endIdx
	d.advanceTo(endIdx + len("]]>"))
	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return false, errTokenTooLarge
	}
	textSpan := makeSpan(&d.buf, textStart, textEnd)
	if err := validateXMLChars(textSpan.bytesUnsafe()); err != nil {
		return false, err
	}
	setTextToken(dst, KindCDATA, textSpan, startLine, startColumn, makeSpan(&d.buf, rawStart, rawEnd), false)
	return false, nil
}

func (d *Decoder) scanDirectiveInto(dst *rawToken, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advanceRaw(2)
	textStart := d.pos
	depth := 0
	quote := byte(0)
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return false, errUnexpectedEOF
			}
			return false, err
		}
		b := d.buf.data[d.pos]
		if quote != 0 {
			if b == quote {
				quote = 0
			}
			d.advance(1)
			continue
		}
		switch b {
		case '\'', '"':
			quote = b
			d.advance(1)
		case '[':
			depth++
			d.advance(1)
		case ']':
			if depth > 0 {
				depth--
			}
			d.advance(1)
		case '>':
			if depth == 0 {
				textEnd := d.pos
				d.advance(1)
				rawEnd := d.pos
				if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
					return false, errTokenTooLarge
				}
				textSpan := makeSpan(&d.buf, textStart, textEnd)
				if err := validateXMLChars(textSpan.bytesUnsafe()); err != nil {
					return false, err
				}
				setTextToken(dst, KindDirective, textSpan, startLine, startColumn, makeSpan(&d.buf, rawStart, rawEnd), false)
				return false, nil
			}
			d.advance(1)
		default:
			d.advance(1)
		}
		if d.opts.maxTokenSize > 0 && d.pos-rawStart > d.opts.maxTokenSize {
			return false, errTokenTooLarge
		}
	}
}

func (d *Decoder) scanUntil(seq []byte, allowCompact bool) (int, error) {
	for {
		idx := bytes.Index(d.buf.data[d.pos:], seq)
		if idx >= 0 {
			return d.pos + idx, nil
		}
		if d.eof {
			return 0, errUnexpectedEOF
		}
		if err := d.readMore(allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				d.eof = true
				continue
			}
			return 0, err
		}
	}
}

func (d *Decoder) scanQName(allowCompact bool) (qnameSpan, error) {
	start := d.pos
	first := true
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return qnameSpan{}, errUnexpectedEOF
			}
			return qnameSpan{}, err
		}
		buf := d.buf.data
		b := buf[d.pos]
		if b < utf8.RuneSelf {
			if first {
				if !nameStartByteLUT[b] {
					return qnameSpan{}, errInvalidName
				}
			} else if !isNameByte(b) {
				break
			}
			i := d.pos
			for i < len(buf) {
				b = buf[i]
				if b >= utf8.RuneSelf || !nameByteLUT[b] {
					break
				}
				i++
			}
			d.advanceName(i - d.pos)
			first = false
			if i == len(buf) {
				continue
			}
			if buf[i] < utf8.RuneSelf {
				break
			}
		}
		r, size, err := d.peekRune(allowCompact)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return qnameSpan{}, errUnexpectedEOF
			}
			return qnameSpan{}, err
		}
		if first {
			if !isNameStartRune(r) {
				return qnameSpan{}, errInvalidName
			}
		} else if !isNameRune(r) {
			break
		}
		d.advanceName(size)
		first = false
	}
	end := d.pos
	colonIndex := -1
	data := d.buf.data[start:end]
	if offset := bytes.IndexByte(data, ':'); offset >= 0 {
		if offset == 0 || offset == len(data)-1 {
			return qnameSpan{}, errInvalidName
		}
		if bytes.IndexByte(data[offset+1:], ':') >= 0 {
			return qnameSpan{}, errInvalidName
		}
		colonIndex = start + offset
	}
	return makeQNameSpan(&d.buf, start, end, colonIndex), nil
}

func (d *Decoder) scanName(allowCompact bool) (span, error) {
	if err := d.ensureIndex(d.pos, allowCompact); err != nil {
		if errors.Is(err, io.EOF) {
			return span{}, errUnexpectedEOF
		}
		return span{}, err
	}
	start := d.pos
	b := d.buf.data[d.pos]
	if b < utf8.RuneSelf {
		if !isNameStartByte(b) {
			return span{}, errInvalidName
		}
		buf := d.buf.data
		i := d.pos + 1
		for i < len(buf) {
			b = buf[i]
			if b >= utf8.RuneSelf || !isNameByte(b) {
				break
			}
			i++
		}
		d.advanceName(i - d.pos)
		if i < len(buf) && buf[i] < utf8.RuneSelf {
			return makeSpan(&d.buf, start, d.pos), nil
		}
	} else {
		r, size, err := d.peekRune(allowCompact)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return span{}, errUnexpectedEOF
			}
			return span{}, err
		}
		if !isNameStartRune(r) {
			return span{}, errInvalidName
		}
		d.advanceName(size)
	}
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return span{}, errUnexpectedEOF
			}
			return span{}, err
		}
		b = d.buf.data[d.pos]
		if b < utf8.RuneSelf {
			if !isNameByte(b) {
				break
			}
			d.advanceName(1)
			continue
		}
		r, size, err := d.peekRune(allowCompact)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return span{}, errUnexpectedEOF
			}
			return span{}, err
		}
		if !isNameRune(r) {
			break
		}
		d.advanceName(size)
	}
	return makeSpan(&d.buf, start, d.pos), nil
}

func (d *Decoder) peekRune(allowCompact bool) (rune, int, error) {
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			return 0, 0, err
		}
		b := d.buf.data[d.pos]
		if b < utf8.RuneSelf {
			return rune(b), 1, nil
		}
		data := d.buf.data[d.pos:]
		if utf8.FullRune(data) {
			r, size := utf8.DecodeRune(data)
			if r == utf8.RuneError && size == 1 {
				return 0, 0, errInvalidChar
			}
			return r, size, nil
		}
		if d.eof {
			return 0, 0, errInvalidChar
		}
		if err := d.readMore(allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				d.eof = true
				continue
			}
			return 0, 0, err
		}
	}
}

func (d *Decoder) skipWhitespace(allowCompact bool) bool {
	consumed := false
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			return consumed
		}
		data := d.buf.data[d.pos:]
		i := 0
		for i < len(data) && isWhitespace(data[i]) {
			i++
		}
		if i == 0 {
			return consumed
		}
		consumed = true
		d.advance(i)
		if i < len(data) {
			return consumed
		}
	}
}

func (d *Decoder) expectByte(value byte, allowCompact bool) error {
	if err := d.ensureIndex(d.pos, allowCompact); err != nil {
		if errors.Is(err, io.EOF) {
			return errUnexpectedEOF
		}
		return err
	}
	if d.buf.data[d.pos] != value {
		return errInvalidToken
	}
	d.advance(1)
	return nil
}

func (d *Decoder) matchLiteral(lit []byte, allowCompact bool) (bool, error) {
	end := d.pos + len(lit)
	for end > len(d.buf.data) {
		if err := d.readMore(allowCompact); err != nil {
			if errors.Is(err, io.EOF) {
				return false, errUnexpectedEOF
			}
			return false, err
		}
	}
	return bytes.Equal(d.buf.data[d.pos:end], lit), nil
}

func (d *Decoder) ensureIndex(idx int, allowCompact bool) error {
	for idx >= len(d.buf.data) {
		if d.eof {
			return io.EOF
		}
		if err := d.readMore(allowCompact); err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) readMore(allowCompact bool) error {
	if d.eof {
		return io.EOF
	}
	if allowCompact && d.pos == 0 {
		d.compactIfNeeded()
	}
	if len(d.buf.data) == cap(d.buf.data) {
		if err := d.growBuffer(); err != nil {
			return err
		}
	}
	space := cap(d.buf.data) - len(d.buf.data)
	if space == 0 {
		return io.EOF
	}
	buf := d.buf.data
	n, err := d.r.Read(buf[len(buf) : len(buf)+space])
	if n > 0 {
		d.buf.data = buf[:len(buf)+n]
		return nil
	}
	if errors.Is(err, io.EOF) {
		d.eof = true
		return io.EOF
	}
	return err
}

func (d *Decoder) compactIfNeeded() {
	if d.pos == 0 {
		return
	}
	keepIndex := d.pos
	if d.compactFloorSet {
		floorIndex := int(d.compactFloorAbs - d.baseOffset)
		if floorIndex < 0 {
			return
		}
		if floorIndex < keepIndex {
			keepIndex = floorIndex
		}
	}
	if keepIndex >= len(d.buf.data) {
		d.baseOffset += int64(keepIndex)
		d.buf.data = d.buf.data[:0]
		d.pos -= keepIndex
		return
	}
	remaining := len(d.buf.data) - keepIndex
	if remaining >= cap(d.buf.data)/4 {
		return
	}
	if cap(d.buf.data)-len(d.buf.data) >= cap(d.buf.data)/4 {
		return
	}
	if keepIndex == d.pos {
		d.compact()
		return
	}
	copy(d.buf.data, d.buf.data[keepIndex:])
	d.buf.data = d.buf.data[:len(d.buf.data)-keepIndex]
	d.baseOffset += int64(keepIndex)
	d.pos -= keepIndex
}

func (d *Decoder) setCompactFloorAbs(offset int64) {
	d.compactFloorAbs = offset
	d.compactFloorSet = true
}

func (d *Decoder) clearCompactFloor() {
	d.compactFloorSet = false
	d.compactFloorAbs = 0
}

func (d *Decoder) growBuffer() error {
	capNow := cap(d.buf.data)
	newCap := capNow * 2
	minCap := d.opts.bufferSize
	if minCap <= 0 {
		minCap = defaultBufferSize
	}
	if newCap < minCap {
		newCap = minCap
	}
	if d.opts.maxTokenSize > 0 && newCap > d.opts.maxTokenSize {
		newCap = d.opts.maxTokenSize
	}
	if newCap <= capNow {
		return errTokenTooLarge
	}
	newBuf := make([]byte, len(d.buf.data), newCap)
	copy(newBuf, d.buf.data)
	d.buf.data = newBuf
	return nil
}

func (d *Decoder) compact() {
	if d.pos == 0 {
		return
	}
	if d.pos >= len(d.buf.data) {
		d.baseOffset += int64(d.pos)
		d.buf.data = d.buf.data[:0]
		d.pos = 0
		return
	}
	copy(d.buf.data, d.buf.data[d.pos:])
	d.buf.data = d.buf.data[:len(d.buf.data)-d.pos]
	d.baseOffset += int64(d.pos)
	d.pos = 0
}

func (d *Decoder) advance(n int) {
	if n <= 0 {
		return
	}
	if d.opts.trackLineColumn {
		data := d.buf.data[d.pos : d.pos+n]
		for i := 0; i < len(data); i++ {
			b := data[i]
			if b == '\n' || b == '\r' {
				d.advanceWithNewlines(data)
				return
			}
		}
		d.pendingCR = false
		// No newlines - just update column
		d.column += n
	}
	d.pos += n
}

// advanceWithNewlines handles line tracking when newlines are present (slow path).
func (d *Decoder) advanceWithNewlines(data []byte) {
	i := 0
	if d.pendingCR {
		d.pendingCR = false
		if len(data) > 0 && data[0] == '\n' {
			i = 1
		}
	}
	for ; i < len(data); i++ {
		switch data[i] {
		case '\n':
			d.line++
			d.column = 1
		case '\r':
			d.line++
			d.column = 1
			if i+1 < len(data) && data[i+1] == '\n' {
				i++
			} else if i+1 == len(data) {
				d.pendingCR = true
			}
		default:
			d.column++
		}
	}
	d.pos += len(data)
}

// advanceName assumes name characters cannot include line breaks.
func (d *Decoder) advanceName(n int) {
	if n <= 0 {
		return
	}
	if d.opts.trackLineColumn {
		d.column += n
	}
	d.pos += n
}

func (d *Decoder) advanceTo(pos int) {
	d.advance(pos - d.pos)
}

// advanceRaw increments position without line tracking.
// Use only for content known not to contain newlines (delimiters, tags, etc).
func (d *Decoder) advanceRaw(n int) {
	if n <= 0 {
		return
	}
	if d.opts.trackLineColumn {
		if d.opts.debugPoisonSpans {
			end := d.pos + n
			if end > len(d.buf.data) {
				panic("xmltext: advanceRaw beyond buffer")
			}
			if bytes.ContainsAny(d.buf.data[d.pos:end], "\n\r") {
				panic("xmltext: advanceRaw consumed newline")
			}
		}
		d.pendingCR = false
		d.column += n
	}
	d.pos += n
}

func isWhitespace(b byte) bool {
	return whitespaceLUT[b]
}

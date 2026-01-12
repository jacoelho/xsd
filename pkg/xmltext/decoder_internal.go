package xmltext

import (
	"bytes"
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
	StackEntry
	childCount int64
}

type attrBucket struct {
	spans []Span
	gen   uint32
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
	if err == io.EOF {
		d.err = io.EOF
		return io.EOF
	}
	if _, ok := err.(*SyntaxError); ok {
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
		Path:    d.StackPath(nil),
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

func (d *Decoder) refreshToken(tok *Token) {
	if tok == nil {
		return
	}
	refreshSpan(&tok.Name.Full)
	refreshSpan(&tok.Name.Prefix)
	refreshSpan(&tok.Name.Local)
	refreshSpan(&tok.Text)
	refreshSpan(&tok.Raw)
	for i := range tok.AttrRaw {
		refreshSpan(&tok.AttrRaw[i])
	}
	for i := range tok.Attrs {
		attr := &tok.Attrs[i]
		refreshSpan(&attr.Name.Full)
		refreshSpan(&attr.Name.Prefix)
		refreshSpan(&attr.Name.Local)
		refreshSpan(&attr.ValueSpan)
	}
}

func refreshSpan(span *Span) {
	if span == nil || span.buf == nil {
		return
	}
	span.gen = span.buf.gen
}

func (d *Decoder) nextToken(allowCompact bool) (Token, bool, error) {
	var tok Token
	selfClosing, err := d.nextTokenInto(&tok, allowCompact)
	if err != nil {
		return Token{}, false, err
	}
	return tok, selfClosing, nil
}

func (d *Decoder) nextTokenInto(dst *Token, allowCompact bool) (bool, error) {
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
		if err := d.popStackInterned(dst.Name); err != nil {
			return false, err
		}
		return false, nil
	}
	for {
		selfClosing, err := d.scanTokenInto(dst, allowCompact)
		if err != nil {
			if err == io.EOF {
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
		switch dst.Kind {
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

func (d *Decoder) applyToken(tok *Token, selfClosing bool) error {
	if err := d.checkTokenPlacement(tok); err != nil {
		return err
	}
	switch tok.Kind {
	case KindStartElement:
		interned := d.internQName(tok.Name)
		tok.Name = interned
		if err := d.pushStack(interned); err != nil {
			return err
		}
		if selfClosing {
			d.pendingEnd = true
			d.pendingEndToken = Token{
				Kind:   KindEndElement,
				Name:   interned,
				Line:   d.line,
				Column: d.column,
			}
		}
	case KindEndElement:
		interned, err := d.popStackRaw(tok.Name)
		if err != nil {
			return err
		}
		tok.Name = interned
		if len(d.stack) == 0 {
			d.afterRoot = true
		}
	}
	return nil
}

func (d *Decoder) checkTokenPlacement(tok *Token) error {
	if tok == nil {
		return nil
	}
	if tok.IsXMLDecl {
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
	switch tok.Kind {
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

func (d *Decoder) isWhitespaceCharData(tok *Token) (bool, error) {
	if tok == nil {
		return true, nil
	}
	data := tok.Text.bytes()
	if len(data) == 0 {
		return true, nil
	}
	if !tok.TextNeeds {
		return isWhitespaceBytes(data), nil
	}
	out, err := unescapeInto(d.scratch.data[:0], data, &d.entities, d.opts.maxTokenSize)
	if err != nil {
		return false, err
	}
	d.scratch.data = out
	return isWhitespaceBytes(out), nil
}

func (d *Decoder) internQName(name QNameSpan) QNameSpan {
	if d.interner == nil {
		d.interner = newNameInterner(d.opts.maxQNameInternEntries)
	}
	return d.interner.internQName(name)
}

func (d *Decoder) internQNameHash(name QNameSpan, hash uint64) QNameSpan {
	if d.interner == nil {
		d.interner = newNameInterner(d.opts.maxQNameInternEntries)
	}
	return d.interner.internQNameHash(name, hash)
}

func (d *Decoder) pushStack(name QNameSpan) error {
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
		StackEntry: StackEntry{Name: name, Index: index},
		childCount: 0,
	})
	return nil
}

func (d *Decoder) popStackRaw(name QNameSpan) (QNameSpan, error) {
	if len(d.stack) == 0 {
		return QNameSpan{}, errMismatchedEndTag
	}
	top := d.stack[len(d.stack)-1]
	if !bytes.Equal(name.Full.bytes(), top.Name.Full.bytes()) {
		return QNameSpan{}, errMismatchedEndTag
	}
	d.stack = d.stack[:len(d.stack)-1]
	return top.Name, nil
}

func (d *Decoder) popStackInterned(name QNameSpan) error {
	if len(d.stack) == 0 {
		return errMismatchedEndTag
	}
	top := d.stack[len(d.stack)-1]
	if !bytes.Equal(name.Full.bytes(), top.Name.Full.bytes()) {
		return errMismatchedEndTag
	}
	d.stack = d.stack[:len(d.stack)-1]
	return nil
}

func (d *Decoder) scanTokenInto(dst *Token, allowCompact bool) (bool, error) {
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
		if err == io.EOF {
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

func setCharDataToken(dst *Token, text Span, needs, rawNeeds bool, line, column int, raw Span) {
	dst.Kind = KindCharData
	dst.Name = QNameSpan{}
	dst.Attrs = nil
	dst.AttrNeeds = nil
	dst.AttrRaw = nil
	dst.AttrRawNeeds = nil
	dst.Text = text
	dst.TextNeeds = needs
	dst.TextRawNeeds = rawNeeds
	dst.Line = line
	dst.Column = column
	dst.Raw = raw
	dst.IsXMLDecl = false
}

func copyToken(dst *Token, src *Token) {
	if dst == nil || src == nil {
		return
	}
	dst.Kind = src.Kind
	dst.Name = src.Name
	dst.Attrs = src.Attrs
	dst.AttrNeeds = src.AttrNeeds
	dst.AttrRaw = src.AttrRaw
	dst.AttrRawNeeds = src.AttrRawNeeds
	dst.Text = src.Text
	dst.TextNeeds = src.TextNeeds
	dst.TextRawNeeds = src.TextRawNeeds
	dst.Line = src.Line
	dst.Column = src.Column
	dst.Raw = src.Raw
	dst.IsXMLDecl = src.IsXMLDecl
}

func setTextToken(dst *Token, kind Kind, text Span, line, column int, raw Span, isXMLDecl bool) {
	dst.Kind = kind
	dst.Name = QNameSpan{}
	dst.Attrs = nil
	dst.AttrNeeds = nil
	dst.AttrRaw = nil
	dst.AttrRawNeeds = nil
	dst.Text = text
	dst.TextNeeds = false
	dst.TextRawNeeds = false
	dst.Line = line
	dst.Column = column
	dst.Raw = raw
	dst.IsXMLDecl = isXMLDecl
}

func (d *Decoder) scanCharDataInto(dst *Token, allowCompact bool) (bool, error) {
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
			if err == io.EOF {
				d.eof = true
				continue
			}
			return false, err
		}
	}
}

func scanCharDataSpanUntilEntity(data []byte, start int) (int, error) {
	bracketRun := 0
	for i := start; i < len(data); {
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

func (d *Decoder) resolveText(span Span) (Span, bool, bool, error) {
	data := span.bytes()
	if len(data) == 0 {
		return span, false, false, nil
	}
	if !d.opts.resolveEntities {
		rawNeeds, err := scanCharDataSpanParse(data, &d.entities)
		if err != nil {
			return Span{}, false, false, err
		}
		if !rawNeeds {
			return span, false, false, nil
		}
		return span, true, true, nil
	}
	out, rawNeeds, err := unescapeCharDataInto(d.scratch.data[:0], data, &d.entities, d.opts.maxTokenSize)
	if err != nil {
		return Span{}, false, false, err
	}
	if !rawNeeds {
		return span, false, false, nil
	}
	d.scratch.data = out
	return makeSpan(&d.scratch, 0, len(out)), false, true, nil
}

func (d *Decoder) scanStartTagInto(dst *Token, allowCompact bool) (bool, error) {
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
			if err == io.EOF {
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
			if err == io.EOF {
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
		d.attrBuf = append(d.attrBuf, AttrSpan{Name: attrName, ValueSpan: valueSpan})
		d.attrNeeds = append(d.attrNeeds, needs)
		d.attrRaw = append(d.attrRaw, rawSpan)
		d.attrRawNeeds = append(d.attrRawNeeds, rawNeeds)
		if d.opts.maxAttrs > 0 && len(d.attrBuf) > d.opts.maxAttrs {
			return false, errAttrLimit
		}
		space = d.skipWhitespace(allowCompact)
	}

	selfClosing := false
	if d.buf.data[d.pos] == '/' {
		selfClosing = true
		d.advance(1)
		if err := d.expectByte('>', allowCompact); err != nil {
			return false, err
		}
	} else if d.buf.data[d.pos] == '>' {
		d.advance(1)
	} else {
		return false, errInvalidToken
	}

	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return false, errTokenTooLarge
	}

	dst.Kind = KindStartElement
	dst.Name = name
	dst.Attrs = d.attrBuf
	dst.AttrNeeds = d.attrNeeds
	dst.AttrRaw = d.attrRaw
	dst.AttrRawNeeds = d.attrRawNeeds
	dst.Text = Span{}
	dst.TextNeeds = false
	dst.TextRawNeeds = false
	dst.Line = startLine
	dst.Column = startColumn
	dst.Raw = makeSpan(&d.buf, rawStart, rawEnd)
	dst.IsXMLDecl = false
	return selfClosing, nil
}

func (d *Decoder) scanAttrValue(quote byte, allowCompact bool) (Span, Span, bool, bool, error) {
	start := d.pos
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if err == io.EOF {
				return Span{}, Span{}, false, false, errUnexpectedEOF
			}
			return Span{}, Span{}, false, false, err
		}
		data := d.buf.data[d.pos:]
		quoteIdx := bytes.IndexByte(data, quote)
		ltIdx := bytes.IndexByte(data, '<')

		if quoteIdx >= 0 {
			if ltIdx >= 0 && ltIdx < quoteIdx {
				return Span{}, Span{}, false, false, errInvalidToken
			}
			if d.opts.maxTokenSize > 0 && d.pos-start+quoteIdx > d.opts.maxTokenSize {
				return Span{}, Span{}, false, false, errTokenTooLarge
			}
			d.advanceTo(d.pos + quoteIdx)
			break
		}

		if ltIdx >= 0 {
			return Span{}, Span{}, false, false, errInvalidToken
		}

		d.advanceTo(len(d.buf.data))
		if d.opts.maxTokenSize > 0 && d.pos-start > d.opts.maxTokenSize {
			return Span{}, Span{}, false, false, errTokenTooLarge
		}

		if err := d.readMore(allowCompact); err != nil {
			if err == io.EOF {
				return Span{}, Span{}, false, false, errUnexpectedEOF
			}
			return Span{}, Span{}, false, false, err
		}
	}

	end := d.pos
	d.advanceRaw(1)
	rawSpan := makeSpan(&d.buf, start, end)
	data := rawSpan.bytes()
	rawNeeds := bytes.IndexByte(data, '&') >= 0
	if !rawNeeds {
		if err := validateXMLChars(data); err != nil {
			return Span{}, Span{}, false, false, err
		}
		return rawSpan, rawSpan, false, false, nil
	}
	if d.opts.resolveEntities {
		startOut := len(d.attrValueBuf.data)
		maxSize := 0
		if d.opts.maxTokenSize > 0 {
			maxSize = startOut + d.opts.maxTokenSize
		}
		out, err := unescapeInto(d.attrValueBuf.data, data, &d.entities, maxSize)
		if err != nil {
			return Span{}, Span{}, false, false, err
		}
		if err := validateXMLChars(out[startOut:]); err != nil {
			return Span{}, Span{}, false, false, err
		}
		d.attrValueBuf.data = out
		valueSpan := makeSpan(&d.attrValueBuf, startOut, len(out))
		return rawSpan, valueSpan, false, true, nil
	}
	if err := validateXMLText(data, &d.entities); err != nil {
		return Span{}, Span{}, false, false, err
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

func (d *Decoder) markAttrSeen(name QNameSpan) (uint64, error) {
	data := name.Full.bytes()
	if len(data) == 0 {
		return 0, errInvalidName
	}
	hash := hashBytes(data)
	if d.attrSeenSmallCount < attrSeenSmallMax {
		for i := 0; i < d.attrSeenSmallCount; i++ {
			if bytes.Equal(d.attrSeenSmall[i].bytes(), data) {
				return 0, errDuplicateAttr
			}
		}
		d.attrSeenSmall[d.attrSeenSmallCount] = name.Full
		d.attrSeenSmallCount++
		return hash, nil
	}
	if d.attrSeenSmallCount == attrSeenSmallMax {
		if d.attrSeen == nil {
			d.attrSeen = make(map[uint64]attrBucket, attrSeenSmallMax*2)
		}
		for i := 0; i < d.attrSeenSmallCount; i++ {
			span := d.attrSeenSmall[i]
			spanData := span.bytes()
			if len(spanData) == 0 {
				continue
			}
			spanHash := hashBytes(spanData)
			bucket := d.attrSeen[spanHash]
			if bucket.gen != d.attrSeenGen {
				bucket.gen = d.attrSeenGen
				bucket.spans = bucket.spans[:0]
			}
			bucket.spans = append(bucket.spans, span)
			d.attrSeen[spanHash] = bucket
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
		if bytes.Equal(span.bytes(), data) {
			return 0, errDuplicateAttr
		}
	}
	bucket.spans = append(bucket.spans, name.Full)
	d.attrSeen[hash] = bucket
	return hash, nil
}

func (d *Decoder) scanEndTagInto(dst *Token, allowCompact bool) (bool, error) {
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
	dst.Kind = KindEndElement
	dst.Name = name
	dst.Attrs = nil
	dst.AttrNeeds = nil
	dst.AttrRaw = nil
	dst.AttrRawNeeds = nil
	dst.Text = Span{}
	dst.TextNeeds = false
	dst.TextRawNeeds = false
	dst.Line = startLine
	dst.Column = startColumn
	dst.Raw = makeSpan(&d.buf, rawStart, rawEnd)
	dst.IsXMLDecl = false
	return false, nil
}

func (d *Decoder) scanPIInto(dst *Token, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advanceRaw(2)
	textStart := d.pos
	targetSpan, err := d.scanName(allowCompact)
	if err != nil {
		return false, err
	}
	isXMLDecl := bytes.EqualFold(targetSpan.bytes(), litXML)
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
		if err := validateXMLChars(textSpan.bytes()); err != nil {
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
	if err := validateXMLChars(textSpan.bytes()); err != nil {
		return false, err
	}
	setTextToken(dst, KindPI, textSpan, startLine, startColumn, makeSpan(&d.buf, rawStart, rawEnd), isXMLDecl)
	return false, nil
}

func (d *Decoder) scanBangInto(dst *Token, allowCompact bool) (bool, error) {
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

func (d *Decoder) scanCommentInto(dst *Token, allowCompact bool) (bool, error) {
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
	textData := textSpan.bytes()
	if bytes.Contains(textData, litDDash) || (len(textData) > 0 && textData[len(textData)-1] == '-') {
		return false, errInvalidComment
	}
	if err := validateXMLChars(textData); err != nil {
		return false, err
	}
	setTextToken(dst, KindComment, textSpan, startLine, startColumn, makeSpan(&d.buf, rawStart, rawEnd), false)
	return false, nil
}

func (d *Decoder) scanCDATAInto(dst *Token, allowCompact bool) (bool, error) {
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
	if err := validateXMLChars(textSpan.bytes()); err != nil {
		return false, err
	}
	setTextToken(dst, KindCDATA, textSpan, startLine, startColumn, makeSpan(&d.buf, rawStart, rawEnd), false)
	return false, nil
}

func (d *Decoder) scanDirectiveInto(dst *Token, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advanceRaw(2)
	textStart := d.pos
	depth := 0
	quote := byte(0)
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if err == io.EOF {
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
				if err := validateXMLChars(textSpan.bytes()); err != nil {
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
			if err == io.EOF {
				d.eof = true
				continue
			}
			return 0, err
		}
	}
}

func (d *Decoder) scanQName(allowCompact bool) (QNameSpan, error) {
	start := d.pos
	colonIndex := -1
	first := true
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if err == io.EOF {
				return QNameSpan{}, errUnexpectedEOF
			}
			return QNameSpan{}, err
		}
		buf := d.buf.data
		b := buf[d.pos]
		if b < utf8.RuneSelf {
			if first {
				if !nameStartByteLUT[b] {
					return QNameSpan{}, errInvalidName
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
				if b == ':' {
					if colonIndex >= 0 {
						return QNameSpan{}, errInvalidName
					}
					colonIndex = i
				}
				i++
			}
			d.advanceName(i - d.pos)
			first = false
			if i < len(buf) {
				if buf[i] < utf8.RuneSelf {
					break
				}
			} else {
				continue
			}
		}
		r, size, err := d.peekRune(allowCompact)
		if err != nil {
			if err == io.EOF {
				return QNameSpan{}, errUnexpectedEOF
			}
			return QNameSpan{}, err
		}
		if first {
			if !isNameStartRune(r) {
				return QNameSpan{}, errInvalidName
			}
		} else if !isNameRune(r) {
			break
		}
		if r == ':' {
			if colonIndex >= 0 {
				return QNameSpan{}, errInvalidName
			}
			colonIndex = d.pos
		}
		d.advanceName(size)
		first = false
	}
	end := d.pos
	if colonIndex == start || colonIndex == end-1 {
		return QNameSpan{}, errInvalidName
	}
	return makeQNameSpan(&d.buf, start, end, colonIndex), nil
}

func (d *Decoder) scanName(allowCompact bool) (Span, error) {
	if err := d.ensureIndex(d.pos, allowCompact); err != nil {
		if err == io.EOF {
			return Span{}, errUnexpectedEOF
		}
		return Span{}, err
	}
	start := d.pos
	b := d.buf.data[d.pos]
	if b < utf8.RuneSelf {
		if !isNameStartByte(b) {
			return Span{}, errInvalidName
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
			if err == io.EOF {
				return Span{}, errUnexpectedEOF
			}
			return Span{}, err
		}
		if !isNameStartRune(r) {
			return Span{}, errInvalidName
		}
		d.advanceName(size)
	}
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if err == io.EOF {
				return Span{}, errUnexpectedEOF
			}
			return Span{}, err
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
			if err == io.EOF {
				return Span{}, errUnexpectedEOF
			}
			return Span{}, err
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
			if err == io.EOF {
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
		if err == io.EOF {
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
			if err == io.EOF {
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
	if err == io.EOF {
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
		// Single scan for newlines (fast path)
		for _, b := range data {
			if b == '\n' || b == '\r' {
				d.advanceWithNewlines(data)
				return
			}
		}
		// No newlines - just update column
		d.column += n
	}
	d.pos += n
}

// advanceWithNewlines handles line tracking when newlines are present (slow path).
func (d *Decoder) advanceWithNewlines(data []byte) {
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\n':
			d.line++
			d.column = 1
		case '\r':
			d.line++
			d.column = 1
			if i+1 < len(data) && data[i+1] == '\n' {
				i++
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
//
//go:inline
func (d *Decoder) advanceRaw(n int) {
	if n <= 0 {
		return
	}
	if d.opts.trackLineColumn {
		d.column += n
	}
	d.pos += n
}

func isWhitespace(b byte) bool {
	return whitespaceLUT[b]
}

package xmltext

import (
	"bytes"
	"errors"
	"hash/maphash"
	"io"
	"unicode/utf8"
)

var (
	litXML       = []byte("xml")
	litVersion   = []byte("version")
	litEncoding  = []byte("encoding")
	litStand     = []byte("standalone")
	litVersion10 = []byte("1.0")
	litYes       = []byte("yes")
	litNo        = []byte("no")
	litPIEnd     = []byte("?>")
	litComStart  = []byte("<!--")
	litComEnd    = []byte("-->")
	litDDash     = []byte("--")
	litCDStart   = []byte("<![CDATA[")
	litCDEnd     = []byte("]]>")
)

func (d *Decoder) scanStartTagInto(dst *rawToken, allowCompact bool) (bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advanceRaw(1)

	d.attrBuf = d.attrBuf[:0]
	d.attrNeeds = d.attrNeeds[:0]
	d.attrRaw = d.attrRaw[:0]
	d.attrRawNeeds = d.attrRawNeeds[:0]
	d.attrValueBuf.data = d.attrValueBuf.data[:0]
	d.resetAttrSeen()

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
		hash, err := d.markAttrSeen(&attrName)
		if err != nil {
			return false, err
		}
		d.skipWhitespace(allowCompact)
		err = d.expectByte('=', allowCompact)
		if err != nil {
			return false, err
		}
		d.skipWhitespace(allowCompact)
		err = d.ensureIndex(d.pos, allowCompact)
		if err != nil {
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

		attrName = d.internQNameHash(&attrName, hash)
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
		for i := range data {
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

func (d *Decoder) markAttrSeen(name *qnameSpan) (uint64, error) {
	if name == nil {
		return 0, errInvalidName
	}
	data := name.Full.bytesUnsafe()
	if len(data) == 0 {
		return 0, errInvalidName
	}
	hash := maphash.Bytes(hashSeed, data)
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
	var (
		name qnameSpan
		err  error
	)
	if len(d.stack) > 0 {
		name = d.stack[len(d.stack)-1].name
		matched, matchErr := d.matchExpectedQName(name.Full.bytesUnsafe(), allowCompact)
		if matchErr != nil {
			return false, matchErr
		}
		if matched {
			d.advanceName(len(name.Full.bytesUnsafe()))
		} else {
			name, err = d.scanQName(allowCompact)
			if err != nil {
				return false, err
			}
		}
	} else {
		name, err = d.scanQName(allowCompact)
		if err != nil {
			return false, err
		}
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

func (d *Decoder) matchExpectedQName(expected []byte, allowCompact bool) (bool, error) {
	if len(expected) == 0 {
		return false, nil
	}
	end := d.pos + len(expected)
	if err := d.ensureIndex(end-1, allowCompact); err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}
	if !bytes.Equal(d.buf.data[d.pos:end], expected) {
		return false, nil
	}
	if err := d.ensureIndex(end, allowCompact); err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}
	next := d.buf.data[end]
	if next >= utf8.RuneSelf {
		return false, nil
	}
	return !nameByteLUT[next], nil
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
		var ok bool
		ok, err = d.matchLiteral(litPIEnd, allowCompact)
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
		err = validateXMLChars(textSpan.bytesUnsafe())
		if err != nil {
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
	if isXMLDecl && d.opts.strict {
		if err := validateXMLDecl(textSpan.bytesUnsafe()); err != nil {
			return false, err
		}
	}
	setTextToken(dst, KindPI, textSpan, startLine, startColumn, makeSpan(&d.buf, rawStart, rawEnd), isXMLDecl)
	return false, nil
}

func validateXMLDecl(text []byte) error {
	if len(text) < len(litXML) || !bytes.EqualFold(text[:len(litXML)], litXML) {
		return errInvalidPI
	}
	data, ok := consumeXMLDeclSpace(text[len(litXML):])
	if !ok {
		return errInvalidPI
	}
	name, value, rest, err := scanXMLDeclAttr(data)
	if err != nil {
		return err
	}
	if !bytes.Equal(name, litVersion) || !bytes.Equal(value, litVersion10) {
		return errInvalidPI
	}
	if len(rest) == 0 {
		return nil
	}
	data, ok = consumeXMLDeclSpace(rest)
	if !ok {
		return errInvalidPI
	}
	name, value, rest, err = scanXMLDeclAttr(data)
	if err != nil {
		return err
	}
	if bytes.Equal(name, litEncoding) {
		if !isXMLDeclEncoding(value) {
			return errInvalidPI
		}
		if len(rest) == 0 {
			return nil
		}
		data, ok = consumeXMLDeclSpace(rest)
		if !ok {
			return errInvalidPI
		}
		name, value, rest, err = scanXMLDeclAttr(data)
		if err != nil {
			return err
		}
		if !bytes.Equal(name, litStand) {
			return errInvalidPI
		}
		if !isXMLDeclStandalone(value) {
			return errInvalidPI
		}
		if len(trimXMLDeclSpace(rest)) != 0 {
			return errInvalidPI
		}
		return nil
	}
	if !bytes.Equal(name, litStand) {
		return errInvalidPI
	}
	if !isXMLDeclStandalone(value) {
		return errInvalidPI
	}
	if len(trimXMLDeclSpace(rest)) != 0 {
		return errInvalidPI
	}
	return nil
}

func scanXMLDeclAttr(data []byte) ([]byte, []byte, []byte, error) {
	name, rest := scanXMLDeclName(data)
	if len(name) == 0 {
		return nil, nil, data, errInvalidPI
	}
	rest = trimXMLDeclSpace(rest)
	if len(rest) == 0 || rest[0] != '=' {
		return nil, nil, rest, errInvalidPI
	}
	rest = trimXMLDeclSpace(rest[1:])
	if len(rest) == 0 {
		return nil, nil, rest, errInvalidPI
	}
	quote := rest[0]
	if quote != '"' && quote != '\'' {
		return nil, nil, rest, errInvalidPI
	}
	rest = rest[1:]
	end := bytes.IndexByte(rest, quote)
	if end < 0 {
		return nil, nil, rest, errInvalidPI
	}
	value := rest[:end]
	rest = rest[end+1:]
	return name, value, rest, nil
}

func consumeXMLDeclSpace(data []byte) ([]byte, bool) {
	i := 0
	for i < len(data) && isWhitespace(data[i]) {
		i++
	}
	if i == 0 {
		return data, false
	}
	return data[i:], true
}

func trimXMLDeclSpace(data []byte) []byte {
	i := 0
	for i < len(data) && isWhitespace(data[i]) {
		i++
	}
	return data[i:]
}

func isXMLDeclEncoding(value []byte) bool {
	if len(value) == 0 || !isASCIIAlpha(value[0]) {
		return false
	}
	for i := 1; i < len(value); i++ {
		b := value[i]
		if isASCIIAlpha(b) || (b >= '0' && b <= '9') || b == '.' || b == '_' || b == '-' {
			continue
		}
		return false
	}
	return true
}

func isXMLDeclStandalone(value []byte) bool {
	return bytes.Equal(value, litYes) || bytes.Equal(value, litNo)
}

func isASCIIAlpha(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
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

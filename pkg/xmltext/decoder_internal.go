package xmltext

import (
	"bytes"
	"io"
	"unicode/utf8"
)

type stackEntry struct {
	StackEntry
	childCount int64
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

func refreshSpan(span *Span) {
	if span == nil || span.buf == nil {
		return
	}
	span.gen = span.buf.gen
}

func (d *Decoder) nextToken(allowCompact bool) (Token, bool, error) {
	if d.pendingTokenValid {
		tok := d.pendingToken
		selfClosing := d.pendingSelfClosing
		d.pendingTokenValid = false
		d.pendingSelfClosing = false
		d.pendingToken = Token{}
		return tok, selfClosing, nil
	}
	if d.pendingEnd {
		tok := d.pendingEndToken
		d.pendingEnd = false
		d.pendingEndToken = Token{}
		if err := d.popStackInterned(tok.name); err != nil {
			return Token{}, false, err
		}
		return tok, false, nil
	}
	for {
		tok, selfClosing, err := d.scanToken(allowCompact)
		if err != nil {
			if err == io.EOF {
				if len(d.stack) > 0 {
					return Token{}, false, errUnexpectedEOF
				}
				if !d.rootSeen {
					return Token{}, false, errMissingRoot
				}
				return Token{}, false, io.EOF
			}
			return Token{}, false, err
		}
		if err := d.applyToken(&tok, selfClosing); err != nil {
			return Token{}, false, err
		}
		switch tok.kind {
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
		return tok, selfClosing, nil
	}
}

func (d *Decoder) applyToken(tok *Token, selfClosing bool) error {
	if err := d.checkTokenPlacement(tok); err != nil {
		return err
	}
	switch tok.kind {
	case KindStartElement:
		interned := d.internName(tok.name.Full.bytes())
		tok.name = interned
		if err := d.pushStack(interned); err != nil {
			return err
		}
		if selfClosing {
			d.pendingEnd = true
			d.pendingEndToken = Token{
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

func (d *Decoder) checkTokenPlacement(tok *Token) error {
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

func (d *Decoder) isWhitespaceCharData(tok *Token) (bool, error) {
	if tok == nil {
		return true, nil
	}
	data := tok.text.bytes()
	if len(data) == 0 {
		return true, nil
	}
	if !tok.textNeeds {
		return isWhitespaceBytes(data), nil
	}
	out, err := unescapeInto(d.scratch.data[:0], data, &d.entities, d.opts.maxTokenSize)
	if err != nil {
		return false, err
	}
	d.scratch.data = out
	return isWhitespaceBytes(out), nil
}

func (d *Decoder) internName(data []byte) QNameSpan {
	if d.interner == nil {
		d.interner = newNameInterner(d.opts.maxQNameInternEntries)
	}
	return d.interner.intern(data)
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

func (d *Decoder) scanToken(allowCompact bool) (Token, bool, error) {
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
		return Token{}, false, err
	}
	if d.pos >= len(d.buf.data) {
		return Token{}, false, io.EOF
	}
	if d.buf.data[d.pos] != '<' {
		return d.scanCharData(allowCompact)
	}

	if err := d.ensureIndex(d.pos+1, allowCompact); err != nil {
		if err == io.EOF {
			return Token{}, false, errUnexpectedEOF
		}
		return Token{}, false, err
	}

	switch d.buf.data[d.pos+1] {
	case '/':
		return d.scanEndTag(allowCompact)
	case '?':
		return d.scanPI(allowCompact)
	case '!':
		return d.scanBang(allowCompact)
	default:
		return d.scanStartTag(allowCompact)
	}
}

func (d *Decoder) scanCharData(allowCompact bool) (Token, bool, error) {
	startLine, startColumn := d.line, d.column
	start := d.pos
	for {
		idx := bytes.IndexByte(d.buf.data[d.pos:], '<')
		if idx >= 0 {
			end := d.pos + idx
			if d.opts.maxTokenSize > 0 && end-start > d.opts.maxTokenSize {
				return Token{}, false, errTokenTooLarge
			}
			if bytes.Contains(d.buf.data[start:end], []byte("]]>")) {
				return Token{}, false, errInvalidToken
			}
			d.advanceTo(end)
			span := makeSpan(&d.buf, start, end)
			rawNeeds := bytes.IndexByte(span.bytes(), '&') >= 0
			textSpan, needs, err := d.resolveText(span, rawNeeds)
			if err != nil {
				return Token{}, false, err
			}
			return Token{
				kind:         KindCharData,
				text:         textSpan,
				textNeeds:    needs,
				textRawNeeds: rawNeeds,
				line:         startLine,
				column:       startColumn,
				raw:          span,
			}, false, nil
		}
		if d.eof {
			end := len(d.buf.data)
			if end == start {
				return Token{}, false, io.EOF
			}
			if d.opts.maxTokenSize > 0 && end-start > d.opts.maxTokenSize {
				return Token{}, false, errTokenTooLarge
			}
			if bytes.Contains(d.buf.data[start:end], []byte("]]>")) {
				return Token{}, false, errInvalidToken
			}
			d.advanceTo(end)
			span := makeSpan(&d.buf, start, end)
			rawNeeds := bytes.IndexByte(span.bytes(), '&') >= 0
			textSpan, needs, err := d.resolveText(span, rawNeeds)
			if err != nil {
				return Token{}, false, err
			}
			return Token{
				kind:         KindCharData,
				text:         textSpan,
				textNeeds:    needs,
				textRawNeeds: rawNeeds,
				line:         startLine,
				column:       startColumn,
				raw:          span,
			}, false, nil
		}
		if err := d.readMore(allowCompact); err != nil {
			if err == io.EOF {
				d.eof = true
				continue
			}
			return Token{}, false, err
		}
	}
}

func (d *Decoder) resolveText(span Span, rawNeeds bool) (Span, bool, error) {
	data := span.bytes()
	if len(data) == 0 {
		return span, false, nil
	}
	if !rawNeeds {
		if err := validateXMLChars(data); err != nil {
			return Span{}, false, err
		}
		return span, false, nil
	}
	if d.opts.resolveEntities {
		out, err := unescapeInto(d.scratch.data[:0], data, &d.entities, d.opts.maxTokenSize)
		if err != nil {
			return Span{}, false, err
		}
		if err := validateXMLChars(out); err != nil {
			return Span{}, false, err
		}
		d.scratch.data = out
		return makeSpan(&d.scratch, 0, len(out)), false, nil
	}
	if err := validateXMLText(data, &d.entities); err != nil {
		return Span{}, false, err
	}
	return span, true, nil
}

func (d *Decoder) scanStartTag(allowCompact bool) (Token, bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advance(1)

	name, err := d.scanQName(allowCompact)
	if err != nil {
		return Token{}, false, err
	}

	space := d.skipWhitespace(allowCompact)
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if err == io.EOF {
				return Token{}, false, errUnexpectedEOF
			}
			return Token{}, false, err
		}
		b := d.buf.data[d.pos]
		if b == '/' || b == '>' {
			break
		}
		if !space {
			return Token{}, false, errInvalidToken
		}
		attrName, err := d.scanQName(allowCompact)
		if err != nil {
			return Token{}, false, err
		}
		if err := d.markAttrSeen(attrName); err != nil {
			return Token{}, false, err
		}
		d.skipWhitespace(allowCompact)
		if err := d.expectByte('=', allowCompact); err != nil {
			return Token{}, false, err
		}
		d.skipWhitespace(allowCompact)
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if err == io.EOF {
				return Token{}, false, errUnexpectedEOF
			}
			return Token{}, false, err
		}
		quote := d.buf.data[d.pos]
		if quote != '\'' && quote != '"' {
			return Token{}, false, errInvalidToken
		}
		d.advance(1)
		rawSpan, valueSpan, needs, rawNeeds, err := d.scanAttrValue(quote, allowCompact)
		if err != nil {
			return Token{}, false, err
		}

		d.attrBuf = append(d.attrBuf, AttrSpan{Name: attrName, ValueSpan: valueSpan})
		d.attrNeeds = append(d.attrNeeds, needs)
		d.attrRaw = append(d.attrRaw, rawSpan)
		d.attrRawNeeds = append(d.attrRawNeeds, rawNeeds)
		if d.opts.maxAttrs > 0 && len(d.attrBuf) > d.opts.maxAttrs {
			return Token{}, false, errAttrLimit
		}
		space = d.skipWhitespace(allowCompact)
	}

	selfClosing := false
	if d.buf.data[d.pos] == '/' {
		selfClosing = true
		d.advance(1)
		if err := d.expectByte('>', allowCompact); err != nil {
			return Token{}, false, err
		}
	} else if d.buf.data[d.pos] == '>' {
		d.advance(1)
	} else {
		return Token{}, false, errInvalidToken
	}

	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return Token{}, false, errTokenTooLarge
	}

	return Token{
		kind:         KindStartElement,
		name:         name,
		attrs:        d.attrBuf,
		attrNeeds:    d.attrNeeds,
		attrRaw:      d.attrRaw,
		attrRawNeeds: d.attrRawNeeds,
		line:         startLine,
		column:       startColumn,
		raw:          makeSpan(&d.buf, rawStart, rawEnd),
	}, selfClosing, nil
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
		b := d.buf.data[d.pos]
		if b == quote {
			break
		}
		if b == '<' {
			return Span{}, Span{}, false, false, errInvalidToken
		}
		d.advance(1)
		if d.opts.maxTokenSize > 0 && d.pos-start > d.opts.maxTokenSize {
			return Span{}, Span{}, false, false, errTokenTooLarge
		}
	}

	end := d.pos
	d.advance(1)
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
	for _, key := range d.attrSeenKeys {
		delete(d.attrSeen, key)
	}
	d.attrSeenKeys = d.attrSeenKeys[:0]
}

func (d *Decoder) markAttrSeen(name QNameSpan) error {
	data := name.Full.bytes()
	if len(data) == 0 {
		return errInvalidName
	}
	if d.attrSeen == nil {
		d.attrSeen = make(map[uint64][]Span, 8)
	}
	hash := hashBytes(data)
	bucket := d.attrSeen[hash]
	for _, span := range bucket {
		if bytes.Equal(span.bytes(), data) {
			return errDuplicateAttr
		}
	}
	if bucket == nil {
		d.attrSeenKeys = append(d.attrSeenKeys, hash)
	}
	d.attrSeen[hash] = append(bucket, name.Full)
	return nil
}

func (d *Decoder) scanEndTag(allowCompact bool) (Token, bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advance(2)
	name, err := d.scanQName(allowCompact)
	if err != nil {
		return Token{}, false, err
	}
	d.skipWhitespace(allowCompact)
	if err := d.expectByte('>', allowCompact); err != nil {
		return Token{}, false, err
	}
	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return Token{}, false, errTokenTooLarge
	}
	return Token{
		kind:   KindEndElement,
		name:   name,
		line:   startLine,
		column: startColumn,
		raw:    makeSpan(&d.buf, rawStart, rawEnd),
	}, false, nil
}

func (d *Decoder) scanPI(allowCompact bool) (Token, bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advance(2)
	textStart := d.pos
	targetSpan, err := d.scanName(allowCompact)
	if err != nil {
		return Token{}, false, err
	}
	isXMLDecl := bytes.EqualFold(targetSpan.bytes(), []byte("xml"))
	hasSpace := d.skipWhitespace(allowCompact)
	if !hasSpace {
		ok, err := d.matchLiteral("?>", allowCompact)
		if err != nil {
			return Token{}, false, err
		}
		if !ok {
			return Token{}, false, errInvalidPI
		}
		if isXMLDecl {
			return Token{}, false, errInvalidPI
		}
		textEnd := d.pos
		d.advance(2)
		rawEnd := d.pos
		if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
			return Token{}, false, errTokenTooLarge
		}
		textSpan := makeSpan(&d.buf, textStart, textEnd)
		if err := validateXMLChars(textSpan.bytes()); err != nil {
			return Token{}, false, err
		}
		return Token{
			kind:      KindPI,
			text:      textSpan,
			line:      startLine,
			column:    startColumn,
			raw:       makeSpan(&d.buf, rawStart, rawEnd),
			isXMLDecl: isXMLDecl,
		}, false, nil
	}
	endIdx, err := d.scanUntil([]byte("?>"), allowCompact)
	if err != nil {
		return Token{}, false, err
	}
	textEnd := endIdx
	d.advanceTo(endIdx + 2)
	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return Token{}, false, errTokenTooLarge
	}
	textSpan := makeSpan(&d.buf, textStart, textEnd)
	if err := validateXMLChars(textSpan.bytes()); err != nil {
		return Token{}, false, err
	}
	return Token{
		kind:      KindPI,
		text:      textSpan,
		line:      startLine,
		column:    startColumn,
		raw:       makeSpan(&d.buf, rawStart, rawEnd),
		isXMLDecl: isXMLDecl,
	}, false, nil
}

func (d *Decoder) scanBang(allowCompact bool) (Token, bool, error) {
	if ok, err := d.matchLiteral("<!--", allowCompact); err != nil {
		return Token{}, false, err
	} else if ok {
		return d.scanComment(allowCompact)
	}
	if ok, err := d.matchLiteral("<![CDATA[", allowCompact); err != nil {
		return Token{}, false, err
	} else if ok {
		return d.scanCDATA(allowCompact)
	}
	return d.scanDirective(allowCompact)
}

func (d *Decoder) scanComment(allowCompact bool) (Token, bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advance(len("<!--"))
	textStart := d.pos
	endIdx, err := d.scanUntil([]byte("-->"), allowCompact)
	if err != nil {
		return Token{}, false, err
	}
	textEnd := endIdx
	d.advanceTo(endIdx + len("-->"))
	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return Token{}, false, errTokenTooLarge
	}
	textSpan := makeSpan(&d.buf, textStart, textEnd)
	textData := textSpan.bytes()
	if bytes.Contains(textData, []byte("--")) || (len(textData) > 0 && textData[len(textData)-1] == '-') {
		return Token{}, false, errInvalidComment
	}
	if err := validateXMLChars(textData); err != nil {
		return Token{}, false, err
	}
	return Token{
		kind:   KindComment,
		text:   textSpan,
		line:   startLine,
		column: startColumn,
		raw:    makeSpan(&d.buf, rawStart, rawEnd),
	}, false, nil
}

func (d *Decoder) scanCDATA(allowCompact bool) (Token, bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advance(len("<![CDATA["))
	textStart := d.pos
	endIdx, err := d.scanUntil([]byte("]]>"), allowCompact)
	if err != nil {
		return Token{}, false, err
	}
	textEnd := endIdx
	d.advanceTo(endIdx + len("]]>"))
	rawEnd := d.pos
	if d.opts.maxTokenSize > 0 && rawEnd-rawStart > d.opts.maxTokenSize {
		return Token{}, false, errTokenTooLarge
	}
	textSpan := makeSpan(&d.buf, textStart, textEnd)
	if err := validateXMLChars(textSpan.bytes()); err != nil {
		return Token{}, false, err
	}
	return Token{
		kind:   KindCDATA,
		text:   textSpan,
		line:   startLine,
		column: startColumn,
		raw:    makeSpan(&d.buf, rawStart, rawEnd),
	}, false, nil
}

func (d *Decoder) scanDirective(allowCompact bool) (Token, bool, error) {
	startLine, startColumn := d.line, d.column
	rawStart := d.pos
	d.advance(2)
	textStart := d.pos
	depth := 0
	quote := byte(0)
	for {
		if err := d.ensureIndex(d.pos, allowCompact); err != nil {
			if err == io.EOF {
				return Token{}, false, errUnexpectedEOF
			}
			return Token{}, false, err
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
					return Token{}, false, errTokenTooLarge
				}
				textSpan := makeSpan(&d.buf, textStart, textEnd)
				if err := validateXMLChars(textSpan.bytes()); err != nil {
					return Token{}, false, err
				}
				return Token{
					kind:   KindDirective,
					text:   textSpan,
					line:   startLine,
					column: startColumn,
					raw:    makeSpan(&d.buf, rawStart, rawEnd),
				}, false, nil
			}
			d.advance(1)
		default:
			d.advance(1)
		}
		if d.opts.maxTokenSize > 0 && d.pos-rawStart > d.opts.maxTokenSize {
			return Token{}, false, errTokenTooLarge
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
	if err := d.ensureIndex(d.pos, allowCompact); err != nil {
		if err == io.EOF {
			return QNameSpan{}, errUnexpectedEOF
		}
		return QNameSpan{}, err
	}
	start := d.pos
	r, size, err := d.peekRune(allowCompact)
	if err != nil {
		if err == io.EOF {
			return QNameSpan{}, errUnexpectedEOF
		}
		return QNameSpan{}, err
	}
	if !isNameStartRune(r) {
		return QNameSpan{}, errInvalidName
	}
	colonIndex := -1
	if r == ':' {
		colonIndex = start
	}
	d.advance(size)
	for {
		r, size, err = d.peekRune(allowCompact)
		if err != nil {
			if err == io.EOF {
				return QNameSpan{}, errUnexpectedEOF
			}
			return QNameSpan{}, err
		}
		if !isNameRune(r) {
			break
		}
		if r == ':' {
			if colonIndex >= 0 {
				return QNameSpan{}, errInvalidName
			}
			colonIndex = d.pos
		}
		d.advance(size)
	}
	end := d.pos
	if colonIndex == start || colonIndex == end-1 {
		return QNameSpan{}, errInvalidName
	}
	return newQNameSpan(&d.buf, start, end), nil
}

func (d *Decoder) scanName(allowCompact bool) (Span, error) {
	if err := d.ensureIndex(d.pos, allowCompact); err != nil {
		if err == io.EOF {
			return Span{}, errUnexpectedEOF
		}
		return Span{}, err
	}
	start := d.pos
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
	d.advance(size)
	for {
		r, size, err = d.peekRune(allowCompact)
		if err != nil {
			if err == io.EOF {
				return Span{}, errUnexpectedEOF
			}
			return Span{}, err
		}
		if !isNameRune(r) {
			break
		}
		d.advance(size)
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
		if !isWhitespace(d.buf.data[d.pos]) {
			return consumed
		}
		consumed = true
		d.advance(1)
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

func (d *Decoder) matchLiteral(lit string, allowCompact bool) (bool, error) {
	end := d.pos + len(lit)
	for end > len(d.buf.data) {
		if err := d.readMore(allowCompact); err != nil {
			if err == io.EOF {
				return false, errUnexpectedEOF
			}
			return false, err
		}
	}
	return bytes.Equal(d.buf.data[d.pos:end], []byte(lit)), nil
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
	if (len(d.buf.data) - d.pos) >= cap(d.buf.data)/2 {
		return
	}
	d.compact()
}

func (d *Decoder) growBuffer() error {
	capNow := cap(d.buf.data)
	newCap := capNow * 2
	if newCap < defaultBufferSize {
		newCap = defaultBufferSize
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
	}
	d.pos += n
}

func (d *Decoder) advanceTo(pos int) {
	d.advance(pos - d.pos)
}

func isWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

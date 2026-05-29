package xsd

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type streamTokenKind uint8

const (
	streamTokenStart streamTokenKind = iota
	streamTokenEnd
	streamTokenCharData
	streamTokenDirective
	streamTokenComment
	streamTokenPI
)

// byteStringCache interns short parser strings per validation session. Each
// cache keeps at most maxByteStringCacheEntries strings no longer than
// maxByteStringCacheLen bytes.
const (
	maxByteStringCacheEntries = 512
	maxByteStringCacheLen     = 256
)

type streamToken struct {
	end       xml.EndElement
	start     streamStartElement
	data      []byte
	directive []byte
	line      int
	col       int
	kind      streamTokenKind
	cdata     bool
}

var (
	errUnsupportedEntityReference = errors.New("unsupported entity reference")
	errXMLAttributeLimit          = errors.New("XML attribute count limit exceeded")
	errXMLTokenLimit              = errors.New("XML token byte limit exceeded")
)

var (
	doctypeDirective = []byte("DOCTYPE")
	xmlPITarget      = []byte(xmlPrefix)
	entityLT         = []byte("lt")
	entityGT         = []byte("gt")
	entityAMP        = []byte("amp")
	entityAPOS       = []byte("apos")
	entityQUOT       = []byte("quot")
)

type xmlStreamParser struct {
	names         *byteStringCache
	values        *byteStringCache
	pendingEnd    xml.EndElement
	nameBuf       []byte
	valueBuf      []byte
	attrValueBuf  []byte
	entityBuf     []byte
	textBuf       []byte
	directive     []byte
	attrs         []streamAttr
	br            byteStream
	maxAttrs      int
	maxTokenBytes int64
	cdataMatched  int
	hasEnd        bool
	inCDATA       bool
	atStart       bool
	emitComments  bool
	emitPI        bool
	lazyAttrValue bool
}

func (p *xmlStreamParser) reset(r io.Reader, names, values *byteStringCache) {
	p.resetWithLimit(r, names, values, 0)
}

func (p *xmlStreamParser) resetWithLimit(r io.Reader, names, values *byteStringCache, maxTokenBytes int64) {
	p.names = names
	p.values = values
	p.pendingEnd = xml.EndElement{}
	p.nameBuf = resetRetainedBytes(p.nameBuf)
	p.valueBuf = resetRetainedBytes(p.valueBuf)
	p.attrValueBuf = resetRetainedBytes(p.attrValueBuf)
	p.entityBuf = resetRetainedBytes(p.entityBuf)
	p.textBuf = resetRetainedBytes(p.textBuf)
	p.directive = resetRetainedBytes(p.directive)
	p.attrs = resetRetainedSlice(p.attrs)
	p.br.reset(r)
	p.maxAttrs = 0
	p.maxTokenBytes = maxTokenBytes
	p.cdataMatched = 0
	p.hasEnd = false
	p.inCDATA = false
	p.atStart = true
	p.emitComments = false
	p.emitPI = false
	p.lazyAttrValue = false
}

func (p *xmlStreamParser) next() (streamToken, error) {
	if p.inCDATA {
		return p.readCDATAChunk(0, 0)
	}
	if p.hasEnd {
		p.hasEnd = false
		line, col := p.br.pos()
		return streamToken{kind: streamTokenEnd, end: p.pendingEnd, line: line, col: col}, nil
	}
	for {
		b, err := p.br.readByte()
		if err != nil {
			return streamToken{}, err
		}
		if b != '<' {
			p.atStart = false
			return p.readCharData(b)
		}
		line, col := p.br.pos()
		next, err := p.br.readByte()
		if err != nil {
			return streamToken{}, p.syntaxError("unexpected EOF after <", err)
		}
		switch next {
		case '/':
			end, err := p.readEndElement()
			if err != nil {
				return streamToken{}, err
			}
			p.atStart = false
			return streamToken{kind: streamTokenEnd, end: end, line: line, col: col}, nil
		case '!':
			tok, skip, err := p.readMarkup(line, col)
			if err != nil {
				return streamToken{}, err
			}
			p.atStart = false
			if skip {
				continue
			}
			return tok, nil
		case '?':
			tok, skip, err := p.readPI(p.atStart, line, col)
			if err != nil {
				return streamToken{}, err
			}
			p.atStart = false
			if skip {
				continue
			}
			return tok, nil
		default:
			start, selfClosing, err := p.readStartElement(next)
			if err != nil {
				return streamToken{}, err
			}
			p.atStart = false
			if selfClosing {
				p.pendingEnd = xml.EndElement{Name: start.Name}
				p.hasEnd = true
			}
			return streamToken{kind: streamTokenStart, start: start, line: line, col: col}, nil
		}
	}
}

func (p *xmlStreamParser) readCharData(first byte) (streamToken, error) {
	line, col := p.br.pos()
	p.textBuf = p.textBuf[:0]
	cdataEnd := 0
	switch first {
	case '&':
		if err := p.readEntity(&p.textBuf); err != nil {
			return streamToken{}, err
		}
	case '\r':
		p.consumeLineFeed()
		if err := p.appendTokenByte(&p.textBuf, '\n'); err != nil {
			return streamToken{}, err
		}
	default:
		cdataEnd = advanceCDataEnd(cdataEnd, first)
		if cdataEnd == len(cdataEndTerm) {
			return streamToken{}, fmt.Errorf("]]> cannot appear in character data")
		}
		if err := p.appendXMLRune(&p.textBuf, first); err != nil {
			return streamToken{}, err
		}
	}
	for {
		chunk, err := p.br.buffered()
		if errors.Is(err, io.EOF) {
			return streamToken{kind: streamTokenCharData, data: p.textBuf, line: line, col: col}, nil
		}
		if err != nil {
			return streamToken{}, err
		}
		n, nextCDataEnd, err := scanCharDataChunk(chunk, cdataEnd)
		if err != nil {
			return streamToken{}, err
		}
		if n > 0 {
			if appendErr := p.appendTokenBytes(&p.textBuf, chunk[:n]); appendErr != nil {
				return streamToken{}, appendErr
			}
			p.br.consumeBuffered(n)
			cdataEnd = nextCDataEnd
			continue
		}
		b, err := p.br.readByte()
		if err != nil {
			return streamToken{}, err
		}
		if b == '<' {
			p.br.unreadByte()
			return streamToken{kind: streamTokenCharData, data: p.textBuf, line: line, col: col}, nil
		}
		if b == '\r' {
			p.consumeLineFeed()
			if err := p.appendTokenByte(&p.textBuf, '\n'); err != nil {
				return streamToken{}, err
			}
			cdataEnd = 0
			continue
		}
		if b == '&' {
			if err := p.readEntity(&p.textBuf); err != nil {
				return streamToken{}, err
			}
			cdataEnd = 0
			continue
		}
		cdataEnd = advanceCDataEnd(cdataEnd, b)
		if cdataEnd == len(cdataEndTerm) {
			return streamToken{}, fmt.Errorf("]]> cannot appear in character data")
		}
		if err := p.appendXMLRune(&p.textBuf, b); err != nil {
			return streamToken{}, err
		}
	}
}

func scanCharDataChunk(data []byte, cdataEnd int) (int, int, error) {
	i := 0
	for cdataEnd == 0 && len(data)-i >= 8 {
		x := binary.LittleEndian.Uint64(data[i:])
		if x&asciiHighBits != 0 ||
			hasByteLessThan(x, 0x20) ||
			hasByte(x, '<') ||
			hasByte(x, '&') ||
			hasByte(x, ']') {
			break
		}
		i += 8
	}
	for i < len(data) {
		b := data[i]
		if b >= 0x20 && b < utf8.RuneSelf {
			if b == '<' || b == '&' {
				return i, cdataEnd, nil
			}
			cdataEnd = advanceCDataEnd(cdataEnd, b)
			if cdataEnd == len(cdataEndTerm) {
				return i, cdataEnd, fmt.Errorf("]]> cannot appear in character data")
			}
			i++
			continue
		}
		if b == '\n' || b == '\r' {
			return i, cdataEnd, nil
		}
		if b == '\t' {
			cdataEnd = 0
			i++
			continue
		}
		if b < utf8.RuneSelf {
			return i, cdataEnd, fmt.Errorf("invalid XML character")
		}
		if !utf8.FullRune(data[i:]) {
			return i, cdataEnd, nil
		}
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			return i, cdataEnd, fmt.Errorf("invalid UTF-8")
		}
		if !isXMLChar(r) {
			return i, cdataEnd, fmt.Errorf("invalid XML character")
		}
		cdataEnd = 0
		i += size
	}
	return len(data), cdataEnd, nil
}

const cdataEndTerm = "]]>"
const maxEntityReferenceLength = 4 << 20

func advanceCDataEnd(matched int, b byte) int {
	switch matched {
	case 0:
		if b == ']' {
			return 1
		}
	case 1:
		if b == ']' {
			return 2
		}
	case 2:
		if b == '>' {
			return 3
		}
		if b == ']' {
			return 2
		}
	}
	return 0
}

const (
	asciiLowBits  uint64 = 0x0101010101010101
	asciiHighBits uint64 = 0x8080808080808080
)

func hasByte(x uint64, b byte) bool {
	return hasZeroByte(x ^ (asciiLowBits * uint64(b)))
}

// hasByteLessThan uses the standard zero-byte trick across eight ASCII bytes.
func hasByteLessThan(x uint64, b byte) bool {
	return ((x - asciiLowBits*uint64(b)) &^ x & asciiHighBits) != 0
}

// hasZeroByte is the shared bit-parallel zero-byte primitive.
func hasZeroByte(x uint64) bool {
	return ((x - asciiLowBits) &^ x & asciiHighBits) != 0
}

func (p *xmlStreamParser) readMarkup(line, col int) (streamToken, bool, error) {
	b, err := p.br.readByte()
	if err != nil {
		return streamToken{}, false, p.syntaxError("unexpected EOF after <!", err)
	}
	switch b {
	case '-':
		next, err := p.br.readByte()
		if err != nil {
			return streamToken{}, false, p.syntaxError("unexpected EOF in comment", err)
		}
		if next != '-' {
			return streamToken{}, false, fmt.Errorf("invalid XML comment")
		}
		if !p.emitComments {
			if commentErr := p.skipComment(); commentErr != nil {
				return streamToken{}, false, commentErr
			}
			return streamToken{}, true, nil
		}
		p.directive = p.directive[:0]
		data, err := p.readComment(p.directive)
		if err != nil {
			return streamToken{}, false, err
		}
		p.directive = data
		return streamToken{kind: streamTokenComment, directive: data, line: line, col: col}, false, nil
	case '[':
		if err := p.expectString("CDATA["); err != nil {
			return streamToken{}, false, err
		}
		p.inCDATA = true
		p.cdataMatched = 0
		tok, err := p.readCDATAChunk(line, col)
		return tok, false, err
	default:
		p.directive = p.directive[:0]
		p.directive = append(p.directive, b)
		for len(p.directive) <= len(doctypeDirective) {
			next, err := p.br.readByte()
			if err != nil {
				return streamToken{}, false, p.syntaxError("unexpected EOF in markup declaration", err)
			}
			p.directive = append(p.directive, next)
		}
		if !isDOCTYPEDeclaration(p.directive) {
			return streamToken{}, false, fmt.Errorf("invalid markup declaration")
		}
		return streamToken{kind: streamTokenDirective, directive: p.directive, line: line, col: col}, false, nil
	}
}

func (p *xmlStreamParser) readCDATAChunk(line, col int) (streamToken, error) {
	if line == 0 {
		line, col = p.br.pos()
	}
	p.textBuf = p.textBuf[:0]
	matched := p.cdataMatched
	appendPending := func() error {
		for ; matched > 0; matched-- {
			if err := p.appendTokenByte(&p.textBuf, ']'); err != nil {
				return err
			}
		}
		return nil
	}
	for {
		b, err := p.br.readByte()
		if err != nil {
			return streamToken{}, p.syntaxError("unexpected EOF in CDATA section", err)
		}
		switch b {
		case ']':
			if matched == 2 {
				if err := p.appendTokenByte(&p.textBuf, ']'); err != nil {
					return streamToken{}, err
				}
			} else {
				matched++
			}
		case '>':
			if matched == 2 {
				p.inCDATA = false
				p.cdataMatched = 0
				return streamToken{kind: streamTokenCharData, data: p.textBuf, cdata: true, line: line, col: col}, nil
			}
			if err := appendPending(); err != nil {
				return streamToken{}, err
			}
			if err := p.appendTokenByte(&p.textBuf, '>'); err != nil {
				return streamToken{}, err
			}
		case '\r':
			if err := appendPending(); err != nil {
				return streamToken{}, err
			}
			p.consumeLineFeed()
			if err := p.appendTokenByte(&p.textBuf, '\n'); err != nil {
				return streamToken{}, err
			}
		default:
			if err := appendPending(); err != nil {
				return streamToken{}, err
			}
			if err := p.appendXMLRune(&p.textBuf, b); err != nil {
				return streamToken{}, err
			}
		}
		if len(p.textBuf) >= len(p.br.buf) {
			p.cdataMatched = matched
			return streamToken{kind: streamTokenCharData, data: p.textBuf, cdata: true, line: line, col: col}, nil
		}
	}
}

func (p *xmlStreamParser) readStartElement(first byte) (streamStartElement, bool, error) {
	name, err := p.readName(first)
	if err != nil {
		return streamStartElement{}, false, err
	}
	p.attrs = p.attrs[:0]
	p.attrValueBuf = p.attrValueBuf[:0]
	for {
		b, hadSpace, err := p.readPastSpace()
		if err != nil {
			return streamStartElement{}, false, err
		}
		switch b {
		case '>':
			return streamStartElement{Name: name, Attr: p.attrs}, false, nil
		case '/':
			next, err := p.br.readByte()
			if err != nil {
				return streamStartElement{}, false, p.syntaxError("unexpected EOF in empty element tag", err)
			}
			if next != '>' {
				return streamStartElement{}, false, fmt.Errorf("expected > after / in empty element tag")
			}
			return streamStartElement{Name: name, Attr: p.attrs}, true, nil
		default:
			if !hadSpace {
				return streamStartElement{}, false, fmt.Errorf("expected whitespace before attribute")
			}
			attrName, err := p.readName(b)
			if err != nil {
				return streamStartElement{}, false, err
			}
			if p.maxAttrs > 0 && len(p.attrs)+1 > p.maxAttrs {
				return streamStartElement{}, false, errXMLAttributeLimit
			}
			if b, _, err = p.readPastSpace(); err != nil {
				return streamStartElement{}, false, err
			}
			if b != '=' {
				return streamStartElement{}, false, fmt.Errorf("expected = after attribute name")
			}
			if b, _, err = p.readPastSpace(); err != nil {
				return streamStartElement{}, false, err
			}
			if b != '"' && b != '\'' {
				return streamStartElement{}, false, fmt.Errorf("attribute value must be quoted")
			}
			value, err := p.readAttributeValueBytes(b)
			if err != nil {
				return streamStartElement{}, false, err
			}
			attr := streamAttr{Name: attrName}
			if p.lazyAttrValue {
				start := len(p.attrValueBuf)
				p.attrValueBuf = append(p.attrValueBuf, value...)
				attr.Raw = p.attrValueBuf[start:]
			} else {
				attr.Value = p.values.intern(value)
			}
			p.attrs = append(p.attrs, attr)
		}
	}
}

func (p *xmlStreamParser) readEndElement() (xml.EndElement, error) {
	b, err := p.br.readByte()
	if err != nil {
		return xml.EndElement{}, err
	}
	if isXMLWhitespaceByte(b) {
		return xml.EndElement{}, fmt.Errorf("unexpected whitespace after </")
	}
	name, err := p.readName(b)
	if err != nil {
		return xml.EndElement{}, err
	}
	b, _, err = p.readPastSpace()
	if err != nil {
		return xml.EndElement{}, err
	}
	if b != '>' {
		return xml.EndElement{}, fmt.Errorf("expected > after end element name")
	}
	return xml.EndElement{Name: name}, nil
}

func (p *xmlStreamParser) readName(first byte) (xml.Name, error) {
	p.nameBuf = p.nameBuf[:0]
	if err := p.appendTokenByte(&p.nameBuf, first); err != nil {
		return xml.Name{}, err
	}
	for {
		chunk, err := p.br.buffered()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return xml.Name{}, fmt.Errorf("unexpected EOF in XML name")
			}
			return xml.Name{}, err
		}
		n := nameChunkLen(chunk)
		if n > 0 {
			if err := p.appendTokenBytes(&p.nameBuf, chunk[:n]); err != nil {
				return xml.Name{}, err
			}
			p.br.consumeBuffered(n)
		}
		if n < len(chunk) {
			break
		}
	}
	if len(p.nameBuf) == 0 {
		return xml.Name{}, fmt.Errorf("empty XML name")
	}
	if prefix, local, ascii, ok := splitASCIIQNameBytes(p.nameBuf); ascii {
		if !ok {
			return xml.Name{}, fmt.Errorf("invalid XML qualified name")
		}
		if prefix == nil {
			return xml.Name{Local: p.names.intern(local)}, nil
		}
		return xml.Name{
			Space: p.names.intern(prefix),
			Local: p.names.intern(local),
		}, nil
	}
	colon := bytes.IndexByte(p.nameBuf, ':')
	if colon < 0 {
		if !isNCNameBytes(p.nameBuf) {
			return xml.Name{}, fmt.Errorf("invalid XML qualified name")
		}
		return xml.Name{Local: p.names.intern(p.nameBuf)}, nil
	}
	if colon == 0 || colon == len(p.nameBuf)-1 || bytes.IndexByte(p.nameBuf[colon+1:], ':') >= 0 {
		return xml.Name{}, fmt.Errorf("invalid XML qualified name")
	}
	if !isNCNameBytes(p.nameBuf[:colon]) || !isNCNameBytes(p.nameBuf[colon+1:]) {
		return xml.Name{}, fmt.Errorf("invalid XML qualified name")
	}
	return xml.Name{
		Space: p.names.intern(p.nameBuf[:colon]),
		Local: p.names.intern(p.nameBuf[colon+1:]),
	}, nil
}

func nameChunkLen(chunk []byte) int {
	for i, b := range chunk {
		if isNameTerminator(b) {
			return i
		}
	}
	return len(chunk)
}

func (p *xmlStreamParser) readAttributeValueBytes(quote byte) ([]byte, error) {
	p.valueBuf = p.valueBuf[:0]
	for {
		chunk, err := p.br.buffered()
		if errors.Is(err, io.EOF) {
			return nil, p.syntaxError("unexpected EOF in attribute value", err)
		}
		if err != nil {
			return nil, err
		}
		if n := attributeValueChunkLen(chunk, quote); n > 0 {
			if appendErr := p.appendTokenBytes(&p.valueBuf, chunk[:n]); appendErr != nil {
				return nil, appendErr
			}
			p.br.consumeBuffered(n)
			continue
		}
		b, err := p.br.readByte()
		if err != nil {
			return nil, p.syntaxError("unexpected EOF in attribute value", err)
		}
		switch b {
		case quote:
			return p.valueBuf, nil
		case '<':
			return nil, fmt.Errorf("attribute value cannot contain <")
		case '\r':
			p.consumeLineFeed()
			if err := p.appendTokenByte(&p.valueBuf, ' '); err != nil {
				return nil, err
			}
		case '\n', '\t':
			if err := p.appendTokenByte(&p.valueBuf, ' '); err != nil {
				return nil, err
			}
		case '&':
			if err := p.readEntity(&p.valueBuf); err != nil {
				return nil, err
			}
		default:
			if err := p.appendXMLRune(&p.valueBuf, b); err != nil {
				return nil, err
			}
		}
	}
}

func attributeValueChunkLen(chunk []byte, quote byte) int {
	for i, b := range chunk {
		if b >= utf8.RuneSelf || b < 0x20 || b == quote || b == '<' || b == '&' {
			return i
		}
	}
	return len(chunk)
}

func (p *xmlStreamParser) readComment(dst []byte) ([]byte, error) {
	prevDash := false
	for {
		b, err := p.br.readByte()
		if err != nil {
			return nil, p.syntaxError("unexpected EOF in comment", err)
		}
		if b == '-' {
			if prevDash {
				if err := p.finishCommentAfterDoubleDash(); err != nil {
					return nil, err
				}
				return dst, nil
			}
			prevDash = true
			continue
		}
		if prevDash {
			if err := p.appendTokenByte(&dst, '-'); err != nil {
				return nil, err
			}
			prevDash = false
		}
		if err := p.appendXMLRune(&dst, b); err != nil {
			return nil, err
		}
	}
}

func (p *xmlStreamParser) skipComment() error {
	prevDash := false
	for {
		b, err := p.br.readByte()
		if err != nil {
			return p.syntaxError("unexpected EOF in comment", err)
		}
		if b == '-' {
			if prevDash {
				return p.finishCommentAfterDoubleDash()
			}
			prevDash = true
			continue
		}
		if prevDash {
			prevDash = false
		}
		if err := p.consumeXMLRune(b); err != nil {
			return err
		}
	}
}

func (p *xmlStreamParser) finishCommentAfterDoubleDash() error {
	next, err := p.br.readByte()
	if err != nil {
		return p.syntaxError("unexpected EOF in comment", err)
	}
	if next != '>' {
		return fmt.Errorf("invalid XML comment")
	}
	return nil
}

func (p *xmlStreamParser) readPI(atDocumentStart bool, line, col int) (streamToken, bool, error) {
	p.nameBuf = p.nameBuf[:0]
	for {
		b, err := p.br.readByte()
		if err != nil {
			return streamToken{}, false, p.syntaxError("unexpected EOF in processing instruction", err)
		}
		if b == '?' {
			return p.finishPIWithoutContent(atDocumentStart, line, col)
		}
		if isXMLWhitespaceByte(b) {
			return p.finishPIWithContent(atDocumentStart, line, col)
		}
		if err := p.appendTokenByte(&p.nameBuf, b); err != nil {
			return streamToken{}, false, err
		}
	}
}

func (p *xmlStreamParser) finishPIWithoutContent(atDocumentStart bool, line, col int) (streamToken, bool, error) {
	isXMLDecl, err := p.validatePITarget(atDocumentStart)
	if err != nil {
		return streamToken{}, false, err
	}
	if isXMLDecl {
		return streamToken{}, false, fmt.Errorf("invalid XML declaration")
	}
	next, err := p.br.readByte()
	if err != nil {
		return streamToken{}, false, p.syntaxError("unexpected EOF in processing instruction", err)
	}
	if next != '>' {
		return streamToken{}, false, fmt.Errorf("processing instruction target must be followed by whitespace or ?>")
	}
	if !p.emitPI {
		return streamToken{}, true, nil
	}
	return streamToken{kind: streamTokenPI, data: p.nameBuf, line: line, col: col}, false, nil
}

func (p *xmlStreamParser) finishPIWithContent(atDocumentStart bool, line, col int) (streamToken, bool, error) {
	isXMLDecl, err := p.validatePITarget(atDocumentStart)
	if err != nil {
		return streamToken{}, false, err
	}
	if isXMLDecl {
		return streamToken{}, true, p.readXMLDeclContent()
	}
	if !p.emitPI {
		return streamToken{}, true, p.skipPIContent()
	}
	p.directive = p.directive[:0]
	data, err := p.readPIContent(p.directive)
	if err != nil {
		return streamToken{}, false, err
	}
	p.directive = data
	return streamToken{kind: streamTokenPI, data: p.nameBuf, directive: data, line: line, col: col}, false, nil
}

func (p *xmlStreamParser) readXMLDeclContent() error {
	p.directive = p.directive[:0]
	data, err := p.readPIContent(p.directive)
	p.directive = data
	if err != nil {
		return err
	}
	return validateXMLDeclContent(p.directive)
}

func (p *xmlStreamParser) skipPIContent() error {
	if err := p.skipUntil("?>"); err != nil {
		if errors.Is(err, io.EOF) {
			return p.syntaxError("unexpected EOF in processing instruction", err)
		}
		return err
	}
	return nil
}

func (p *xmlStreamParser) validatePITarget(atDocumentStart bool) (bool, error) {
	if !isXMLNameBytes(p.nameBuf) {
		return false, fmt.Errorf("invalid processing instruction target")
	}
	if bytes.EqualFold(p.nameBuf, xmlPITarget) {
		if !atDocumentStart || !bytes.Equal(p.nameBuf, xmlPITarget) {
			return false, fmt.Errorf("xml processing instruction target is reserved")
		}
		return true, nil
	}
	return false, nil
}

func (p *xmlStreamParser) readPIContent(dst []byte) ([]byte, error) {
	data, err := p.readUntil("?>", dst)
	if err != nil {
		return nil, p.syntaxError("unexpected EOF in processing instruction", err)
	}
	if valid, err := validXMLPrefix(data); err != nil {
		return nil, err
	} else if valid != len(data) {
		return nil, fmt.Errorf("invalid UTF-8")
	}
	return data, nil
}

func (p *xmlStreamParser) skipUntil(term string) error {
	prefix := termPrefix(term)
	matched := 0
	for {
		b, err := p.br.readByte()
		if err != nil {
			return err
		}
		if b < utf8.RuneSelf {
			if !isXMLChar(rune(b)) {
				return fmt.Errorf("invalid XML character")
			}
			matched = advanceTermMatch(term, prefix, matched, b)
			if matched == len(term) {
				return nil
			}
			continue
		}
		matched = 0
		if err := p.consumeXMLRune(b); err != nil {
			return err
		}
	}
}

func validateXMLDeclContent(content []byte) error {
	rest := content
	version, rest, ok := parseXMLDeclAttr(rest, xsdAttrVersion, xmlDeclFirstAttr)
	if !ok || version != xmlVersion10 {
		return fmt.Errorf("invalid XML declaration")
	}
	if encoding, next, ok := parseXMLDeclAttr(rest, "encoding", xmlDeclNextAttr); ok {
		if !strings.EqualFold(encoding, "UTF-8") && !strings.EqualFold(encoding, "UTF8") {
			return fmt.Errorf("invalid XML declaration")
		}
		rest = next
	}
	if standalone, next, ok := parseXMLDeclAttr(rest, "standalone", xmlDeclNextAttr); ok {
		if standalone != "yes" && standalone != "no" {
			return fmt.Errorf("invalid XML declaration")
		}
		rest = next
	}
	if len(trimXMLWhitespaceBytes(rest)) != 0 {
		return fmt.Errorf("invalid XML declaration")
	}
	return nil
}

type xmlDeclAttrPosition uint8

const (
	xmlDeclFirstAttr xmlDeclAttrPosition = iota
	xmlDeclNextAttr
)

func parseXMLDeclAttr(content []byte, name string, pos xmlDeclAttrPosition) (string, []byte, bool) {
	if pos == xmlDeclNextAttr && (len(content) == 0 || !isXMLWhitespaceByte(content[0])) {
		return "", content, false
	}
	content = bytes.TrimLeft(content, " \t\r\n")
	if !bytes.HasPrefix(content, []byte(name)) {
		return "", content, false
	}
	content = content[len(name):]
	content = bytes.TrimLeft(content, " \t\r\n")
	if len(content) == 0 || content[0] != '=' {
		return "", content, false
	}
	content = bytes.TrimLeft(content[1:], " \t\r\n")
	if len(content) == 0 || content[0] != '"' && content[0] != '\'' {
		return "", content, false
	}
	quote := content[0]
	content = content[1:]
	end := bytes.IndexByte(content, quote)
	if end < 0 {
		return "", content, false
	}
	return string(content[:end]), content[end+1:], true
}

func (p *xmlStreamParser) appendXMLRune(dst *[]byte, first byte) error {
	if first < utf8.RuneSelf {
		if !isXMLChar(rune(first)) {
			return fmt.Errorf("invalid XML character")
		}
		return p.appendTokenByte(dst, first)
	}
	var buf [utf8.UTFMax]byte
	buf[0] = first
	n := 1
	for !utf8.FullRune(buf[:n]) {
		if n == len(buf) {
			return fmt.Errorf("invalid UTF-8")
		}
		b, err := p.br.readByte()
		if err != nil {
			return p.syntaxError("unexpected EOF in UTF-8 sequence", err)
		}
		buf[n] = b
		n++
	}
	r, size := utf8.DecodeRune(buf[:n])
	if r == utf8.RuneError && size == 1 {
		return fmt.Errorf("invalid UTF-8")
	}
	if !isXMLChar(r) {
		return fmt.Errorf("invalid XML character")
	}
	return p.appendTokenBytes(dst, buf[:size])
}

func (p *xmlStreamParser) consumeXMLRune(first byte) error {
	if first < utf8.RuneSelf {
		if !isXMLChar(rune(first)) {
			return fmt.Errorf("invalid XML character")
		}
		return nil
	}
	var buf [utf8.UTFMax]byte
	buf[0] = first
	n := 1
	for !utf8.FullRune(buf[:n]) {
		if n == len(buf) {
			return fmt.Errorf("invalid UTF-8")
		}
		b, err := p.br.readByte()
		if err != nil {
			return p.syntaxError("unexpected EOF in UTF-8 sequence", err)
		}
		buf[n] = b
		n++
	}
	r, size := utf8.DecodeRune(buf[:n])
	if r == utf8.RuneError && size == 1 {
		return fmt.Errorf("invalid UTF-8")
	}
	if !isXMLChar(r) {
		return fmt.Errorf("invalid XML character")
	}
	return nil
}

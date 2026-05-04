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
)

type streamToken struct {
	end       xml.EndElement
	start     xml.StartElement
	data      []byte
	directive []byte
	line      int
	col       int
	kind      streamTokenKind
	cdata     bool
}

var errUnsupportedEntityReference = errors.New("unsupported entity reference")

var (
	doctypeDirective = []byte("DOCTYPE")
	xmlPITarget      = []byte("xml")
	entityLT         = []byte("lt")
	entityGT         = []byte("gt")
	entityAMP        = []byte("amp")
	entityAPOS       = []byte("apos")
	entityQUOT       = []byte("quot")
)

type xmlStreamParser struct {
	names      *byteStringCache
	values     *byteStringCache
	pendingEnd xml.EndElement
	nameBuf    []byte
	valueBuf   []byte
	entityBuf  []byte
	textBuf    []byte
	directive  []byte
	attrs      []xml.Attr
	br         byteStream
	hasEnd     bool
	atStart    bool
}

func newXMLStreamParser(r io.Reader, names, values *byteStringCache) *xmlStreamParser {
	return &xmlStreamParser{
		br:      byteStream{r: r, line: 1},
		names:   names,
		values:  values,
		atStart: true,
	}
}

func (p *xmlStreamParser) next() (streamToken, error) {
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
			if err := p.skipPI(p.atStart); err != nil {
				return streamToken{}, err
			}
			p.atStart = false
			continue
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
		p.textBuf = append(p.textBuf, '\n')
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
		if err == io.EOF {
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
			p.textBuf = append(p.textBuf, chunk[:n]...)
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
			p.textBuf = append(p.textBuf, '\n')
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

func hasByteLessThan(x uint64, b byte) bool {
	return ((x - asciiLowBits*uint64(b)) &^ x & asciiHighBits) != 0
}

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
		return streamToken{}, true, p.skipComment()
	case '[':
		if err := p.expectString("CDATA["); err != nil {
			return streamToken{}, false, err
		}
		p.textBuf = p.textBuf[:0]
		data, err := p.readUntil("]]>", &p.textBuf)
		if err != nil {
			return streamToken{}, false, err
		}
		if valid, err := validXMLPrefix(data); err != nil {
			return streamToken{}, false, err
		} else if valid != len(data) {
			return streamToken{}, false, fmt.Errorf("invalid UTF-8")
		}
		return streamToken{kind: streamTokenCharData, data: data, cdata: true, line: line, col: col}, false, nil
	default:
		p.directive = p.directive[:0]
		p.directive = append(p.directive, b)
		data, err := p.readUntil(">", &p.directive)
		if err != nil {
			return streamToken{}, false, err
		}
		if !isDOCTYPEDeclaration(data) {
			return streamToken{}, false, fmt.Errorf("invalid markup declaration")
		}
		return streamToken{kind: streamTokenDirective, directive: data, line: line, col: col}, false, nil
	}
}

func (p *xmlStreamParser) readStartElement(first byte) (xml.StartElement, bool, error) {
	name, err := p.readName(first)
	if err != nil {
		return xml.StartElement{}, false, err
	}
	p.attrs = p.attrs[:0]
	for {
		b, hadSpace, err := p.readPastSpace()
		if err != nil {
			return xml.StartElement{}, false, err
		}
		switch b {
		case '>':
			return xml.StartElement{Name: name, Attr: p.attrs}, false, nil
		case '/':
			next, err := p.br.readByte()
			if err != nil {
				return xml.StartElement{}, false, p.syntaxError("unexpected EOF in empty element tag", err)
			}
			if next != '>' {
				return xml.StartElement{}, false, fmt.Errorf("expected > after / in empty element tag")
			}
			return xml.StartElement{Name: name, Attr: p.attrs}, true, nil
		default:
			if !hadSpace {
				return xml.StartElement{}, false, fmt.Errorf("expected whitespace before attribute")
			}
			attrName, err := p.readName(b)
			if err != nil {
				return xml.StartElement{}, false, err
			}
			if b, _, err = p.readPastSpace(); err != nil {
				return xml.StartElement{}, false, err
			}
			if b != '=' {
				return xml.StartElement{}, false, fmt.Errorf("expected = after attribute name")
			}
			if b, _, err = p.readPastSpace(); err != nil {
				return xml.StartElement{}, false, err
			}
			if b != '"' && b != '\'' {
				return xml.StartElement{}, false, fmt.Errorf("attribute value must be quoted")
			}
			value, err := p.readAttributeValue(b)
			if err != nil {
				return xml.StartElement{}, false, err
			}
			p.attrs = append(p.attrs, xml.Attr{Name: attrName, Value: value})
		}
	}
}

func (p *xmlStreamParser) readEndElement() (xml.EndElement, error) {
	b, err := p.br.readByte()
	if err != nil {
		return xml.EndElement{}, err
	}
	if isXMLSpaceByte(b) {
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
	p.nameBuf = append(p.nameBuf, first)
	for {
		b, err := p.br.readByte()
		if err != nil {
			if err == io.EOF {
				return xml.Name{}, fmt.Errorf("unexpected EOF in XML name")
			}
			return xml.Name{}, err
		}
		if isNameTerminator(b) {
			p.br.unreadByte()
			break
		}
		p.nameBuf = append(p.nameBuf, b)
	}
	if len(p.nameBuf) == 0 {
		return xml.Name{}, fmt.Errorf("empty XML name")
	}
	if !utf8.Valid(p.nameBuf) {
		return xml.Name{}, fmt.Errorf("invalid XML qualified name")
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

func (p *xmlStreamParser) readAttributeValue(quote byte) (string, error) {
	p.valueBuf = p.valueBuf[:0]
	for {
		b, err := p.br.readByte()
		if err != nil {
			return "", p.syntaxError("unexpected EOF in attribute value", err)
		}
		if b == quote {
			return p.values.intern(p.valueBuf), nil
		}
		if b == '<' {
			return "", fmt.Errorf("attribute value cannot contain <")
		}
		if b == '\r' {
			p.consumeLineFeed()
			p.valueBuf = append(p.valueBuf, ' ')
			continue
		}
		if b == '\n' || b == '\t' {
			p.valueBuf = append(p.valueBuf, ' ')
			continue
		}
		if b == '&' {
			if err := p.readEntity(&p.valueBuf); err != nil {
				return "", err
			}
			continue
		}
		if err := p.appendXMLRune(&p.valueBuf, b); err != nil {
			return "", err
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
				next, err := p.br.readByte()
				if err != nil {
					return p.syntaxError("unexpected EOF in comment", err)
				}
				if next == '>' {
					return nil
				}
				return fmt.Errorf("invalid XML comment")
			}
			prevDash = true
			continue
		}
		if err := p.consumeXMLRune(b); err != nil {
			return err
		}
		prevDash = false
	}
}

func (p *xmlStreamParser) skipPI(atDocumentStart bool) error {
	p.nameBuf = p.nameBuf[:0]
	for {
		b, err := p.br.readByte()
		if err != nil {
			return p.syntaxError("unexpected EOF in processing instruction", err)
		}
		if b == '?' {
			isXMLDecl, err := p.validatePITarget(atDocumentStart)
			if err != nil {
				return err
			}
			if isXMLDecl {
				return fmt.Errorf("invalid XML declaration")
			}
			next, err := p.br.readByte()
			if err != nil {
				return p.syntaxError("unexpected EOF in processing instruction", err)
			}
			if next != '>' {
				return fmt.Errorf("processing instruction target must be followed by whitespace or ?>")
			}
			return nil
		}
		if isXMLSpaceByte(b) {
			isXMLDecl, err := p.validatePITarget(atDocumentStart)
			if err != nil {
				return err
			}
			if isXMLDecl {
				p.directive = p.directive[:0]
				if _, readErr := p.readPIContent(&p.directive); readErr != nil {
					return readErr
				}
				return validateXMLDeclContent(p.directive)
			}
			p.directive = p.directive[:0]
			_, err = p.readPIContent(&p.directive)
			return err
		}
		p.nameBuf = append(p.nameBuf, b)
	}
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

func (p *xmlStreamParser) readPIContent(dst *[]byte) ([]byte, error) {
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

func validateXMLDeclContent(content []byte) error {
	rest := content
	version, rest, ok := parseXMLDeclAttr(rest, "version", false)
	if !ok || version != "1.0" {
		return fmt.Errorf("invalid XML declaration")
	}
	if hasXMLDeclAttr(rest, "encoding") {
		encoding, next, ok := parseXMLDeclAttr(rest, "encoding", true)
		if !ok || !strings.EqualFold(encoding, "UTF-8") && !strings.EqualFold(encoding, "UTF8") {
			return fmt.Errorf("invalid XML declaration")
		}
		rest = next
	}
	if hasXMLDeclAttr(rest, "standalone") {
		standalone, next, ok := parseXMLDeclAttr(rest, "standalone", true)
		if !ok || standalone != "yes" && standalone != "no" {
			return fmt.Errorf("invalid XML declaration")
		}
		rest = next
	}
	if len(bytes.TrimSpace(rest)) != 0 {
		return fmt.Errorf("invalid XML declaration")
	}
	return nil
}

func hasXMLDeclAttr(content []byte, name string) bool {
	if len(content) == 0 || !isXMLSpaceByte(content[0]) {
		return false
	}
	content = bytes.TrimLeft(content, " \t\r\n")
	return bytes.HasPrefix(content, []byte(name))
}

func parseXMLDeclAttr(content []byte, name string, requireLeadingSpace bool) (string, []byte, bool) {
	if requireLeadingSpace && (len(content) == 0 || !isXMLSpaceByte(content[0])) {
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
		*dst = append(*dst, first)
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
	*dst = append(*dst, buf[:size]...)
	return nil
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

func validXMLPrefix(data []byte) (int, error) {
	for i := 0; i < len(data); {
		if data[i] < utf8.RuneSelf {
			if !isXMLChar(rune(data[i])) {
				return i, fmt.Errorf("invalid XML character")
			}
			i++
			continue
		}
		if !utf8.FullRune(data[i:]) {
			return i, nil
		}
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			return i, fmt.Errorf("invalid UTF-8")
		}
		if !isXMLChar(r) {
			return i, fmt.Errorf("invalid XML character")
		}
		i += size
	}
	return len(data), nil
}

func (p *xmlStreamParser) consumeLineFeed() {
	b, err := p.br.readByte()
	if err != nil {
		return
	}
	if b != '\n' {
		p.br.unreadByte()
	}
}

func (p *xmlStreamParser) readEntity(dst *[]byte) error {
	p.entityBuf = p.entityBuf[:0]
	for {
		b, err := p.br.readByte()
		if err != nil {
			return p.syntaxError("unexpected EOF in entity reference", err)
		}
		if b == ';' {
			break
		}
		p.entityBuf = append(p.entityBuf, b)
		if len(p.entityBuf) > maxEntityReferenceLength {
			return fmt.Errorf("invalid character entity")
		}
	}
	switch {
	case bytes.Equal(p.entityBuf, entityLT):
		*dst = append(*dst, '<')
	case bytes.Equal(p.entityBuf, entityGT):
		*dst = append(*dst, '>')
	case bytes.Equal(p.entityBuf, entityAMP):
		*dst = append(*dst, '&')
	case bytes.Equal(p.entityBuf, entityAPOS):
		*dst = append(*dst, '\'')
	case bytes.Equal(p.entityBuf, entityQUOT):
		*dst = append(*dst, '"')
	default:
		if len(p.entityBuf) == 0 || p.entityBuf[0] != '#' {
			if isXMLNameBytes(p.entityBuf) {
				return errUnsupportedEntityReference
			}
			return fmt.Errorf("invalid character entity")
		}
		r, ok := parseCharRef(p.entityBuf[1:])
		if !ok {
			return fmt.Errorf("invalid character entity")
		}
		var buf [utf8.UTFMax]byte
		n := utf8.EncodeRune(buf[:], r)
		*dst = append(*dst, buf[:n]...)
	}
	return nil
}

func parseCharRef(s []byte) (rune, bool) {
	if len(s) == 0 {
		return 0, false
	}
	base := 10
	if s[0] == 'x' || s[0] == 'X' {
		base = 16
		s = s[1:]
		if len(s) == 0 {
			return 0, false
		}
	}
	var v rune
	for _, b := range s {
		var d byte
		switch {
		case b >= '0' && b <= '9':
			d = b - '0'
		case base == 16 && b >= 'a' && b <= 'f':
			d = b - 'a' + 10
		case base == 16 && b >= 'A' && b <= 'F':
			d = b - 'A' + 10
		default:
			return 0, false
		}
		if int(d) >= base {
			return 0, false
		}
		v = v*rune(base) + rune(d)
		if v > utf8.MaxRune {
			return 0, false
		}
	}
	if v > utf8.MaxRune || !utf8.ValidRune(v) || !isXMLChar(v) {
		return 0, false
	}
	return v, true
}

func (p *xmlStreamParser) readPastSpace() (byte, bool, error) {
	hadSpace := false
	for {
		b, err := p.br.readByte()
		if err != nil {
			return 0, hadSpace, err
		}
		if !isXMLSpaceByte(b) {
			return b, hadSpace, nil
		}
		hadSpace = true
	}
}

func (p *xmlStreamParser) readUntil(term string, dst *[]byte) ([]byte, error) {
	prefix := termPrefix(term)
	matched := 0
	for {
		b, err := p.br.readByte()
		if err != nil {
			return nil, p.syntaxError("unexpected EOF", err)
		}
		*dst = append(*dst, b)
		matched = advanceTermMatch(term, prefix, matched, b)
		if matched == len(term) {
			*dst = (*dst)[:len(*dst)-len(term)]
			return *dst, nil
		}
	}
}

func termPrefix(term string) []int {
	prefix := make([]int, len(term))
	for i, j := 1, 0; i < len(term); i++ {
		for j > 0 && term[i] != term[j] {
			j = prefix[j-1]
		}
		if term[i] == term[j] {
			j++
			prefix[i] = j
		}
	}
	return prefix
}

func advanceTermMatch(term string, prefix []int, matched int, b byte) int {
	for matched > 0 && b != term[matched] {
		matched = prefix[matched-1]
	}
	if b == term[matched] {
		matched++
	}
	return matched
}

func (p *xmlStreamParser) expectString(s string) error {
	for i := 0; i < len(s); i++ {
		b, err := p.br.readByte()
		if err != nil {
			return p.syntaxError("unexpected EOF", err)
		}
		if b != s[i] {
			return fmt.Errorf("invalid markup declaration")
		}
	}
	return nil
}

func (p *xmlStreamParser) syntaxError(msg string, err error) error {
	if err == io.EOF {
		return errors.New(msg)
	}
	return err
}

func isNameTerminator(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '/', '>', '=':
		return true
	default:
		return false
	}
}

func isXMLSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func isXMLNameBytes(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	first := true
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if first {
			if !isXMLNameStartChar(r) {
				return false
			}
			first = false
		} else if !isXMLNameChar(r) {
			return false
		}
		b = b[size:]
	}
	return true
}

func isNCNameBytes(b []byte) bool {
	return bytes.IndexByte(b, ':') < 0 && isXMLNameBytes(b)
}

type byteStream struct {
	r       io.Reader
	buf     [64 * 1024]byte
	off     int
	end     int
	line    int
	col     int
	prev    bytePosition
	unread  bool
	last    byte
	lastPos bytePosition
}

type bytePosition struct {
	line int
	col  int
}

func (b *byteStream) readByte() (byte, error) {
	if b.unread {
		b.unread = false
		b.prev = b.lastPos
		b.advance(b.last)
		return b.last, nil
	}
	if b.off == b.end {
		n, err := b.r.Read(b.buf[:])
		if err != nil {
			return 0, err
		}
		b.off = 0
		b.end = n
	}
	c := b.buf[b.off]
	b.off++
	b.last = c
	b.lastPos = bytePosition{line: b.line, col: b.col}
	b.prev = b.lastPos
	b.advance(c)
	return c, nil
}

func (b *byteStream) buffered() ([]byte, error) {
	if b.unread {
		return []byte{b.last}, nil
	}
	if b.off == b.end {
		n, err := b.r.Read(b.buf[:])
		if err != nil {
			return nil, err
		}
		b.off = 0
		b.end = n
	}
	return b.buf[b.off:b.end], nil
}

func (b *byteStream) consumeBuffered(n int) {
	if n <= 0 {
		return
	}
	b.off += n
	b.col += n
}

func (b *byteStream) unreadByte() {
	if b.unread {
		panic("double unread")
	}
	b.unread = true
	b.line = b.lastPos.line
	b.col = b.lastPos.col
}

func (b *byteStream) advance(c byte) {
	if c == '\n' {
		b.line++
		b.col = 0
		return
	}
	b.col++
}

func (b *byteStream) pos() (int, int) {
	return b.line, b.col
}

type byteStringCache struct {
	buckets    map[uint64][]int
	entries    []byteStringEntry
	maxEntries int
	maxLen     int
}

type byteStringEntry struct {
	text string
	hash uint64
}

func newByteStringCache(maxEntries, maxLen int) byteStringCache {
	return byteStringCache{buckets: make(map[uint64][]int), maxEntries: maxEntries, maxLen: maxLen}
}

func (c *byteStringCache) intern(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if c.buckets == nil {
		*c = newByteStringCache(512, 256)
	}
	if c.maxLen > 0 && len(b) > c.maxLen {
		return string(b)
	}
	h := hashBytes(b)
	for _, idx := range c.buckets[h] {
		if stringBytesEqual(c.entries[idx].text, b) {
			return c.entries[idx].text
		}
	}
	s := string(b)
	if c.maxEntries <= 0 || len(c.entries) >= c.maxEntries {
		return s
	}
	idx := len(c.entries)
	c.entries = append(c.entries, byteStringEntry{hash: h, text: s})
	c.buckets[h] = append(c.buckets[h], idx)
	return s
}

func hashBytes(b []byte) uint64 {
	const offset = 14695981039346656037
	const prime = 1099511628211
	h := uint64(offset)
	for _, c := range b {
		h ^= uint64(c)
		h *= prime
	}
	return h
}

func stringBytesEqual(s string, b []byte) bool {
	if len(s) != len(b) {
		return false
	}
	for i := range b {
		if s[i] != b[i] {
			return false
		}
	}
	return true
}

func isDOCTYPEDeclaration(b []byte) bool {
	if len(b) <= len(doctypeDirective) {
		return false
	}
	return bytes.HasPrefix(b, doctypeDirective) && isXMLSpaceByte(b[len(doctypeDirective)])
}

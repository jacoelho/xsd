package stream

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/lex"
)

// TokenKind identifies the parser token variant.
type TokenKind uint8

const (
	// KindStart is a start-element token.
	KindStart TokenKind = iota
	// KindEnd is an end-element token.
	KindEnd
	// KindCharData is a character-data token.
	KindCharData
	// KindDirective is a markup directive token.
	KindDirective
	// KindComment is an XML comment token.
	KindComment
	// KindPI is a processing-instruction token.
	KindPI
)

const (
	maxByteStringCacheEntries = 512
	maxByteStringCacheLen     = 256
)

// Token is one borrowed parser token. Byte slices in token fields are valid
// only until the next parser call.
type Token struct {
	End       EndElement
	Start     StartElement
	Data      []byte
	Directive []byte
	Line      int
	Column    int
	Kind      TokenKind
	CDATA     bool
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

// Parser tokenizes XML into borrowed stream tokens.
type Parser struct {
	names         *Cache
	values        *Cache
	pendingEnd    EndElement
	nameBuf       []byte
	attrValueBuf  []byte
	attrValueEnds []int
	entityBuf     []byte
	textBuf       []byte
	directive     []byte
	attrs         []Attr
	br            byteStream
	maxAttrs      int
	maxTokenBytes int64
	retainedBytes int64
	cdataMatched  int
	hasEnd        bool
	inCDATA       bool
	atStart       bool
	emitComments  bool
	emitPI        bool
	lazyAttrValue bool
}

// Reset prepares p to read r using the supplied string caches.
func (p *Parser) Reset(r io.Reader, names, values *Cache) error {
	return p.ResetWithLimit(r, names, values, 0)
}

// ResetWithLimit prepares p to read r and enforces maxTokenBytes when positive.
func (p *Parser) ResetWithLimit(r io.Reader, names, values *Cache, maxTokenBytes int64) error {
	if isNilReader(r) {
		r = nil
	}
	p.names = names
	p.values = values
	p.pendingEnd = EndElement{}
	p.nameBuf = resetRetainedBytes(p.nameBuf)
	p.attrValueBuf = resetRetainedBytes(p.attrValueBuf)
	p.attrValueEnds = resetRetainedSlice(p.attrValueEnds)
	p.entityBuf = resetRetainedBytes(p.entityBuf)
	p.textBuf = resetRetainedBytes(p.textBuf)
	p.directive = resetRetainedBytes(p.directive)
	p.attrs = resetRetainedSlice(p.attrs)
	p.br.reset(r)
	p.maxAttrs = 0
	p.maxTokenBytes = maxTokenBytes
	p.retainedBytes = 0
	p.cdataMatched = 0
	p.hasEnd = false
	p.inCDATA = false
	p.atStart = true
	p.emitComments = false
	p.emitPI = false
	p.lazyAttrValue = false
	if err := p.prepareXMLProlog(); err != nil {
		p.Detach()
		return err
	}
	return nil
}

func isNilReader(r io.Reader) bool {
	if r == nil {
		return true
	}
	v := reflect.ValueOf(r)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// Detach drops references to the current input while retaining bounded parser
// buffers for reuse.
func (p *Parser) Detach() {
	p.br.detach()
	p.names = nil
	p.values = nil
	p.pendingEnd = EndElement{}
	clear(p.attrs)
	p.attrs = p.attrs[:0]
}

// Next returns the next token. Returned token byte slices are valid until the
// next call to Next or Reset.
func (p *Parser) Next() (Token, error) {
	clear(p.attrs)
	p.attrs = p.attrs[:0]
	if p.inCDATA {
		p.retainedBytes = 0
		return p.readCDATAChunk(0, 0)
	}
	if p.hasEnd {
		p.hasEnd = false
		end := p.pendingEnd
		p.pendingEnd = EndElement{}
		line, col := p.br.pos()
		return Token{Kind: KindEnd, End: end, Line: line, Column: col}, nil
	}
	for {
		p.retainedBytes = 0
		b, err := p.br.readByte()
		if err != nil {
			return Token{}, err
		}
		if b != '<' {
			p.atStart = false
			return p.readCharData(b)
		}
		line, col := p.br.pos()
		next, err := p.br.readByte()
		if err != nil {
			return Token{}, p.syntaxError("unexpected EOF after <", err)
		}
		switch next {
		case '/':
			end, err := p.readEndElement()
			if err != nil {
				return Token{}, err
			}
			p.atStart = false
			return Token{Kind: KindEnd, End: end, Line: line, Column: col}, nil
		case '!':
			tok, skip, err := p.readMarkup(line, col)
			if err != nil {
				return Token{}, err
			}
			p.atStart = false
			if skip {
				continue
			}
			return tok, nil
		case '?':
			tok, skip, err := p.readPI(p.atStart, line, col)
			if err != nil {
				return Token{}, err
			}
			p.atStart = false
			if skip {
				continue
			}
			return tok, nil
		default:
			start, selfClosing, err := p.readStartElement(next)
			if err != nil {
				return Token{}, err
			}
			p.atStart = false
			if selfClosing {
				p.pendingEnd = EndElement{Name: start.Name}
				p.hasEnd = true
			}
			return Token{Kind: KindStart, Start: start, Line: line, Column: col}, nil
		}
	}
}

// Pos returns the current parser line and byte column.
func (p *Parser) Pos() (int, int) {
	return p.br.pos()
}

// SetMaxAttrs sets the maximum attributes allowed on one element. Zero means
// unlimited.
func (p *Parser) SetMaxAttrs(n int) {
	p.maxAttrs = n
}

// SetEmitComments controls whether comment tokens are emitted.
func (p *Parser) SetEmitComments(enabled bool) {
	p.emitComments = enabled
}

// SetEmitPI controls whether processing-instruction tokens are emitted.
func (p *Parser) SetEmitPI(enabled bool) {
	p.emitPI = enabled
}

// SetLazyAttrValue controls whether attributes keep parser-owned raw bytes
// until callers ask for an owned string.
func (p *Parser) SetLazyAttrValue(enabled bool) {
	p.lazyAttrValue = enabled
}

// IsTokenLimit reports whether err is the parser token-byte limit error.
func IsTokenLimit(err error) bool {
	return errors.Is(err, errXMLTokenLimit)
}

// IsAttributeLimit reports whether err is the parser attribute-count limit error.
func IsAttributeLimit(err error) bool {
	return errors.Is(err, errXMLAttributeLimit)
}

// IsUnsupportedEntityReference reports whether err came from an unresolved
// XML entity reference.
func IsUnsupportedEntityReference(err error) bool {
	return errors.Is(err, errUnsupportedEntityReference)
}

func (p *Parser) readCharData(first byte) (Token, error) {
	line, col := p.br.pos()
	p.textBuf = p.textBuf[:0]
	cdataEnd := 0
	switch first {
	case '&':
		if err := p.readEntity(&p.textBuf); err != nil {
			return Token{}, err
		}
	case '\r':
		p.consumeLineFeed()
		if err := p.appendTokenByte(&p.textBuf, '\n'); err != nil {
			return Token{}, err
		}
	default:
		cdataEnd = advanceCDataEnd(cdataEnd, first)
		if cdataEnd == len(cdataEndTerm) {
			return Token{}, fmt.Errorf("]]> cannot appear in character data")
		}
		if err := p.appendXMLRune(&p.textBuf, first); err != nil {
			return Token{}, err
		}
	}
	for {
		chunk, err := p.br.buffered()
		if errors.Is(err, io.EOF) {
			return Token{Kind: KindCharData, Data: p.textBuf, Line: line, Column: col}, nil
		}
		if err != nil {
			return Token{}, err
		}
		n, nextCDataEnd, err := scanCharDataChunk(chunk, cdataEnd)
		if err != nil {
			return Token{}, err
		}
		if n > 0 {
			if appendErr := p.appendTokenBytes(&p.textBuf, chunk[:n]); appendErr != nil {
				return Token{}, appendErr
			}
			p.br.consumeBuffered(n)
			cdataEnd = nextCDataEnd
			continue
		}
		b, err := p.br.readByte()
		if err != nil {
			return Token{}, err
		}
		if b == '<' {
			p.br.unreadByte()
			return Token{Kind: KindCharData, Data: p.textBuf, Line: line, Column: col}, nil
		}
		if b == '\r' {
			p.consumeLineFeed()
			if err := p.appendTokenByte(&p.textBuf, '\n'); err != nil {
				return Token{}, err
			}
			cdataEnd = 0
			continue
		}
		if b == '&' {
			if err := p.readEntity(&p.textBuf); err != nil {
				return Token{}, err
			}
			cdataEnd = 0
			continue
		}
		cdataEnd = advanceCDataEnd(cdataEnd, b)
		if cdataEnd == len(cdataEndTerm) {
			return Token{}, fmt.Errorf("]]> cannot appear in character data")
		}
		if err := p.appendXMLRune(&p.textBuf, b); err != nil {
			return Token{}, err
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
		if !lex.IsXMLChar(r) {
			return i, cdataEnd, fmt.Errorf("invalid XML character")
		}
		cdataEnd = 0
		i += size
	}
	return len(data), cdataEnd, nil
}

const cdataEndTerm = "]]>"
const maxEntityReferenceLength = 4 << 20

// LazyAttrRawMinAttrs is the threshold where parser attributes keep borrowed
// raw values instead of interning all values during tokenization.
const LazyAttrRawMinAttrs = 1

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

func (p *Parser) readMarkup(line, col int) (Token, bool, error) {
	b, err := p.br.readByte()
	if err != nil {
		return Token{}, false, p.syntaxError("unexpected EOF after <!", err)
	}
	switch b {
	case '-':
		next, err := p.br.readByte()
		if err != nil {
			return Token{}, false, p.syntaxError("unexpected EOF in comment", err)
		}
		if next != '-' {
			return Token{}, false, fmt.Errorf("invalid XML comment")
		}
		if !p.emitComments {
			if commentErr := p.skipComment(); commentErr != nil {
				return Token{}, false, commentErr
			}
			return Token{}, true, nil
		}
		p.directive = p.directive[:0]
		data, err := p.readComment(p.directive)
		if err != nil {
			return Token{}, false, err
		}
		p.directive = data
		return Token{Kind: KindComment, Directive: data, Line: line, Column: col}, false, nil
	case '[':
		if err := p.expectString("CDATA["); err != nil {
			return Token{}, false, err
		}
		p.inCDATA = true
		p.cdataMatched = 0
		tok, err := p.readCDATAChunk(line, col)
		return tok, false, err
	default:
		p.directive = p.directive[:0]
		if err := p.appendTokenByte(&p.directive, b); err != nil {
			return Token{}, false, err
		}
		for len(p.directive) <= len(doctypeDirective) {
			next, err := p.br.readByte()
			if err != nil {
				return Token{}, false, p.syntaxError("unexpected EOF in markup declaration", err)
			}
			if err := p.appendTokenByte(&p.directive, next); err != nil {
				return Token{}, false, err
			}
		}
		if !IsDOCTYPEDeclaration(p.directive) {
			return Token{}, false, fmt.Errorf("invalid markup declaration")
		}
		return Token{Kind: KindDirective, Directive: p.directive, Line: line, Column: col}, false, nil
	}
}

func (p *Parser) readCDATAChunk(line, col int) (Token, error) {
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
			return Token{}, p.syntaxError("unexpected EOF in CDATA section", err)
		}
		switch b {
		case ']':
			if matched == 2 {
				if err := p.appendTokenByte(&p.textBuf, ']'); err != nil {
					return Token{}, err
				}
			} else {
				matched++
			}
		case '>':
			if matched == 2 {
				p.inCDATA = false
				p.cdataMatched = 0
				return Token{Kind: KindCharData, Data: p.textBuf, CDATA: true, Line: line, Column: col}, nil
			}
			if err := appendPending(); err != nil {
				return Token{}, err
			}
			if err := p.appendTokenByte(&p.textBuf, '>'); err != nil {
				return Token{}, err
			}
		case '\r':
			if err := appendPending(); err != nil {
				return Token{}, err
			}
			p.consumeLineFeed()
			if err := p.appendTokenByte(&p.textBuf, '\n'); err != nil {
				return Token{}, err
			}
		default:
			if err := appendPending(); err != nil {
				return Token{}, err
			}
			if err := p.appendXMLRune(&p.textBuf, b); err != nil {
				return Token{}, err
			}
		}
		if len(p.textBuf) >= len(p.br.buf) {
			p.cdataMatched = matched
			return Token{Kind: KindCharData, Data: p.textBuf, CDATA: true, Line: line, Column: col}, nil
		}
	}
}

func (p *Parser) readStartElement(first byte) (StartElement, bool, error) {
	name, err := p.readName(first)
	if err != nil {
		return StartElement{}, false, err
	}
	p.attrValueBuf = p.attrValueBuf[:0]
	p.attrValueEnds = p.attrValueEnds[:0]
	for {
		b, hadSpace, err := p.readPastSpace()
		if err != nil {
			return StartElement{}, false, err
		}
		switch b {
		case '>':
			p.finishLazyAttrValues()
			return StartElement{Name: name, Attr: p.attrs}, false, nil
		case '/':
			next, err := p.br.readByte()
			if err != nil {
				return StartElement{}, false, p.syntaxError("unexpected EOF in empty element tag", err)
			}
			if next != '>' {
				return StartElement{}, false, fmt.Errorf("expected > after / in empty element tag")
			}
			p.finishLazyAttrValues()
			return StartElement{Name: name, Attr: p.attrs}, true, nil
		default:
			if !hadSpace {
				return StartElement{}, false, fmt.Errorf("expected whitespace before attribute")
			}
			attrName, err := p.readName(b)
			if err != nil {
				return StartElement{}, false, err
			}
			if p.maxAttrs > 0 && len(p.attrs)+1 > p.maxAttrs {
				return StartElement{}, false, errXMLAttributeLimit
			}
			if b, _, err = p.readPastSpace(); err != nil {
				return StartElement{}, false, err
			}
			if b != '=' {
				return StartElement{}, false, fmt.Errorf("expected = after attribute name")
			}
			if b, _, err = p.readPastSpace(); err != nil {
				return StartElement{}, false, err
			}
			if b != '"' && b != '\'' {
				return StartElement{}, false, fmt.Errorf("attribute value must be quoted")
			}
			if !p.lazyAttrValue {
				p.attrValueBuf = p.attrValueBuf[:0]
			}
			value, err := p.readAttributeValueBytes(b, &p.attrValueBuf)
			if err != nil {
				return StartElement{}, false, err
			}
			attr := Attr{Name: attrName}
			if p.lazyAttrValue {
				p.attrValueEnds = append(p.attrValueEnds, len(p.attrValueBuf))
			} else {
				attr.Value = p.values.Intern(value)
			}
			p.attrs = append(p.attrs, attr)
		}
	}
}

func (p *Parser) finishLazyAttrValues() {
	if !p.lazyAttrValue {
		return
	}
	start := 0
	for i := range p.attrs {
		end := p.attrValueEnds[i]
		p.attrs[i].raw = p.attrValueBuf[start:end]
		start = end
	}
	if len(p.attrs) < LazyAttrRawMinAttrs {
		for i := range p.attrs {
			p.attrs[i].Value = p.values.Intern(p.attrs[i].raw)
			p.attrs[i].raw = nil
		}
		return
	}
}

func (p *Parser) readEndElement() (EndElement, error) {
	b, err := p.br.readByte()
	if err != nil {
		return EndElement{}, err
	}
	if lex.IsXMLWhitespaceByte(b) {
		return EndElement{}, fmt.Errorf("unexpected whitespace after </")
	}
	name, err := p.readName(b)
	if err != nil {
		return EndElement{}, err
	}
	b, _, err = p.readPastSpace()
	if err != nil {
		return EndElement{}, err
	}
	if b != '>' {
		return EndElement{}, fmt.Errorf("expected > after end element name")
	}
	return EndElement{Name: name}, nil
}

func (p *Parser) readName(first byte) (xml.Name, error) {
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
	if prefix, local, ascii, ok := lex.SplitASCIIQNameBytes(p.nameBuf); ascii {
		if !ok {
			return xml.Name{}, fmt.Errorf("invalid XML qualified name")
		}
		if prefix == nil {
			return xml.Name{Local: p.names.Intern(local)}, nil
		}
		return xml.Name{
			Space: p.names.Intern(prefix),
			Local: p.names.Intern(local),
		}, nil
	}
	colon := bytes.IndexByte(p.nameBuf, ':')
	if colon < 0 {
		if !lex.IsNCNameBytes(p.nameBuf) {
			return xml.Name{}, fmt.Errorf("invalid XML qualified name")
		}
		return xml.Name{Local: p.names.Intern(p.nameBuf)}, nil
	}
	if colon == 0 || colon == len(p.nameBuf)-1 || bytes.IndexByte(p.nameBuf[colon+1:], ':') >= 0 {
		return xml.Name{}, fmt.Errorf("invalid XML qualified name")
	}
	if !lex.IsNCNameBytes(p.nameBuf[:colon]) || !lex.IsNCNameBytes(p.nameBuf[colon+1:]) {
		return xml.Name{}, fmt.Errorf("invalid XML qualified name")
	}
	return xml.Name{
		Space: p.names.Intern(p.nameBuf[:colon]),
		Local: p.names.Intern(p.nameBuf[colon+1:]),
	}, nil
}

func nameChunkLen(chunk []byte) int {
	for i, b := range chunk {
		if lex.IsNameTerminator(b) {
			return i
		}
	}
	return len(chunk)
}

func (p *Parser) readAttributeValueBytes(quote byte, dst *[]byte) ([]byte, error) {
	start := len(*dst)
	for {
		chunk, err := p.br.buffered()
		if errors.Is(err, io.EOF) {
			return nil, p.syntaxError("unexpected EOF in attribute value", err)
		}
		if err != nil {
			return nil, err
		}
		if n := attributeValueChunkLen(chunk, quote); n > 0 {
			if appendErr := p.appendTokenBytes(dst, chunk[:n]); appendErr != nil {
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
			return (*dst)[start:], nil
		case '<':
			return nil, fmt.Errorf("attribute value cannot contain <")
		case '\r':
			p.consumeLineFeed()
			if err := p.appendTokenByte(dst, ' '); err != nil {
				return nil, err
			}
		case '\n', '\t':
			if err := p.appendTokenByte(dst, ' '); err != nil {
				return nil, err
			}
		case '&':
			if err := p.readEntity(dst); err != nil {
				return nil, err
			}
		default:
			if err := p.appendXMLRune(dst, b); err != nil {
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

func (p *Parser) readComment(dst []byte) ([]byte, error) {
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

func (p *Parser) skipComment() error {
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

func (p *Parser) finishCommentAfterDoubleDash() error {
	next, err := p.br.readByte()
	if err != nil {
		return p.syntaxError("unexpected EOF in comment", err)
	}
	if next != '>' {
		return fmt.Errorf("invalid XML comment")
	}
	return nil
}

func (p *Parser) readPI(atDocumentStart bool, line, col int) (Token, bool, error) {
	p.nameBuf = p.nameBuf[:0]
	for {
		b, err := p.br.readByte()
		if err != nil {
			return Token{}, false, p.syntaxError("unexpected EOF in processing instruction", err)
		}
		if b == '?' {
			return p.finishPIWithoutContent(atDocumentStart, line, col)
		}
		if lex.IsXMLWhitespaceByte(b) {
			return p.finishPIWithContent(atDocumentStart, line, col)
		}
		if err := p.appendTokenByte(&p.nameBuf, b); err != nil {
			return Token{}, false, err
		}
	}
}

func (p *Parser) finishPIWithoutContent(atDocumentStart bool, line, col int) (Token, bool, error) {
	isXMLDecl, err := p.validatePITarget(atDocumentStart)
	if err != nil {
		return Token{}, false, err
	}
	if isXMLDecl {
		return Token{}, false, fmt.Errorf("invalid XML declaration")
	}
	next, err := p.br.readByte()
	if err != nil {
		return Token{}, false, p.syntaxError("unexpected EOF in processing instruction", err)
	}
	if next != '>' {
		return Token{}, false, fmt.Errorf("processing instruction target must be followed by whitespace or ?>")
	}
	if !p.emitPI {
		return Token{}, true, nil
	}
	return Token{Kind: KindPI, Data: p.nameBuf, Line: line, Column: col}, false, nil
}

func (p *Parser) finishPIWithContent(atDocumentStart bool, line, col int) (Token, bool, error) {
	isXMLDecl, err := p.validatePITarget(atDocumentStart)
	if err != nil {
		return Token{}, false, err
	}
	if isXMLDecl {
		return Token{}, true, p.readXMLDeclContent()
	}
	if !p.emitPI {
		return Token{}, true, p.skipPIContent()
	}
	p.directive = p.directive[:0]
	data, err := p.readPIContent(p.directive)
	if err != nil {
		return Token{}, false, err
	}
	p.directive = data
	return Token{Kind: KindPI, Data: p.nameBuf, Directive: data, Line: line, Column: col}, false, nil
}

func (p *Parser) readXMLDeclContent() error {
	p.directive = p.directive[:0]
	data, err := p.readPIContent(p.directive)
	p.directive = data
	if err != nil {
		return err
	}
	return ValidateXMLDeclContent(p.directive)
}

func (p *Parser) skipPIContent() error {
	if err := p.skipUntil("?>"); err != nil {
		if errors.Is(err, io.EOF) {
			return p.syntaxError("unexpected EOF in processing instruction", err)
		}
		return err
	}
	return nil
}

func (p *Parser) validatePITarget(atDocumentStart bool) (bool, error) {
	if !lex.IsXMLNameBytes(p.nameBuf) {
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

func (p *Parser) readPIContent(dst []byte) ([]byte, error) {
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

func (p *Parser) skipUntil(term string) error {
	prefix := termPrefix(term)
	matched := 0
	for {
		b, err := p.br.readByte()
		if err != nil {
			return err
		}
		if b < utf8.RuneSelf {
			if !lex.IsXMLChar(rune(b)) {
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

// ValidateXMLDeclContent validates the content inside an XML declaration.
func ValidateXMLDeclContent(content []byte) error {
	name, version, rest, ok := ScanXMLDeclAttr(content, XMLDeclFirstAttr)
	if !ok || name != xsdAttrVersion {
		return fmt.Errorf("invalid XML declaration")
	}
	if version != xmlVersion10 {
		return UnsupportedXMLVersionError{Version: version}
	}
	name, value, next, ok := ScanXMLDeclAttr(rest, XMLDeclNextAttr)
	if ok && name == "encoding" {
		if !strings.EqualFold(value, "UTF-8") && !strings.EqualFold(value, "UTF8") {
			return ErrUnsupportedNonUTF8
		}
		rest = next
		name, value, next, ok = ScanXMLDeclAttr(rest, XMLDeclNextAttr)
	}
	if ok && name == "standalone" {
		if value != "yes" && value != "no" {
			return fmt.Errorf("invalid XML declaration")
		}
		rest = next
	}
	if len(lex.TrimXMLWhitespaceBytes(rest)) != 0 {
		return fmt.Errorf("invalid XML declaration")
	}
	return nil
}

// XMLDeclAttrPosition identifies whether an XML declaration attribute is first
// or follows a prior declaration attribute.
type XMLDeclAttrPosition uint8

const (
	// XMLDeclFirstAttr allows optional leading whitespace.
	XMLDeclFirstAttr XMLDeclAttrPosition = iota
	// XMLDeclNextAttr requires leading whitespace.
	XMLDeclNextAttr
)

// ScanXMLDeclAttr scans the next name="value" pair of an XML declaration.
// The first attribute may have optional leading whitespace; later attributes
// require it.
func ScanXMLDeclAttr(content []byte, pos XMLDeclAttrPosition) (string, string, []byte, bool) {
	if pos == XMLDeclNextAttr && (len(content) == 0 || !lex.IsXMLWhitespaceByte(content[0])) {
		return "", "", content, false
	}
	content = bytes.TrimLeft(content, " \t\r\n")
	nameLen := 0
	for nameLen < len(content) && content[nameLen] != '=' && content[nameLen] != '"' && content[nameLen] != '\'' && !lex.IsXMLWhitespaceByte(content[nameLen]) {
		nameLen++
	}
	if nameLen == 0 {
		return "", "", content, false
	}
	name := string(content[:nameLen])
	content = bytes.TrimLeft(content[nameLen:], " \t\r\n")
	if len(content) == 0 || content[0] != '=' {
		return "", "", content, false
	}
	content = bytes.TrimLeft(content[1:], " \t\r\n")
	if len(content) == 0 || content[0] != '"' && content[0] != '\'' {
		return "", "", content, false
	}
	quote := content[0]
	content = content[1:]
	end := bytes.IndexByte(content, quote)
	if end < 0 {
		return "", "", content, false
	}
	return name, string(content[:end]), content[end+1:], true
}

func (p *Parser) appendXMLRune(dst *[]byte, first byte) error {
	if first < utf8.RuneSelf {
		if !lex.IsXMLChar(rune(first)) {
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
	if !lex.IsXMLChar(r) {
		return fmt.Errorf("invalid XML character")
	}
	return p.appendTokenBytes(dst, buf[:size])
}

func (p *Parser) consumeXMLRune(first byte) error {
	if first < utf8.RuneSelf {
		if !lex.IsXMLChar(rune(first)) {
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
	if !lex.IsXMLChar(r) {
		return fmt.Errorf("invalid XML character")
	}
	return nil
}

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
	errXMLInputLimit              = errors.New("XML input byte limit exceeded")
	errXMLTokenLimit              = errors.New("XML token byte limit exceeded")
	errXMLNilCache                = errors.New("XML string cache is nil")
	errXMLNegativeLimit           = errors.New("XML parser limit cannot be negative")
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
	inCDATA       bool
	atStart       bool
	emitComments  bool
	emitPI        bool
	lazyAttrValue bool
}

// Limits bounds parser-owned input and token state. Zero disables a limit;
// production callers are responsible for supplying normalized finite values.
type Limits struct {
	MaxInputBytes int64
	MaxTokenBytes int64
	MaxAttrs      int
}

// Config defines one parser input lifecycle. Modes cannot change after reset.
type Config struct {
	Limits         Limits
	EmitComments   bool
	EmitPI         bool
	LazyAttrValues bool
}

// Reset prepares p to read r using the supplied string caches.
func (p *Parser) Reset(r io.Reader, names, values *Cache) error {
	return p.ResetWithConfig(r, names, values, Config{})
}

// ResetWithConfig atomically prepares p to read r with one validated configuration.
func (p *Parser) ResetWithConfig(r io.Reader, names, values *Cache, config Config) error {
	limits := config.Limits
	if isNilReader(r) {
		p.Detach()
		return ErrXMLInputNilReader
	}
	if names == nil || values == nil {
		p.Detach()
		return errXMLNilCache
	}
	if limits.MaxInputBytes < 0 || limits.MaxTokenBytes < 0 || limits.MaxAttrs < 0 {
		p.Detach()
		return errXMLNegativeLimit
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
	p.br.reset(r, limits.MaxInputBytes)
	p.maxAttrs = limits.MaxAttrs
	p.maxTokenBytes = limits.MaxTokenBytes
	p.retainedBytes = 0
	p.cdataMatched = 0
	p.inCDATA = false
	p.atStart = true
	p.emitComments = config.EmitComments
	p.emitPI = config.EmitPI
	p.lazyAttrValue = config.LazyAttrValues
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
	p.inCDATA = false
	p.atStart = false
	p.cdataMatched = 0
	p.retainedBytes = 0
	p.maxAttrs = 0
	p.maxTokenBytes = 0
	p.emitComments = false
	p.emitPI = false
	p.lazyAttrValue = false
	clear(p.attrs)
	p.attrs = p.attrs[:0]
}

// Next returns the next token. Returned token byte slices are valid until the
// next call to Next or Reset.
func (p *Parser) Next() (Token, error) {
	clear(p.attrs)
	p.attrs = p.attrs[:0]
	if tok, ready, err := p.pendingToken(); ready {
		return tok, err
	}
	for {
		tok, skip, err := p.readNextToken()
		if err != nil || !skip {
			return tok, err
		}
	}
}

func (p *Parser) pendingToken() (Token, bool, error) {
	if p.inCDATA {
		p.retainedBytes = 0
		tok, err := p.readCDATAChunk(0, 0)
		return tok, true, err
	}
	if p.pendingEnd.Name.Local == "" {
		return Token{}, false, nil
	}
	end := p.pendingEnd
	p.pendingEnd = EndElement{}
	line, col := p.br.pos()
	return Token{Kind: KindEnd, End: end, Line: line, Column: col}, true, nil
}

func (p *Parser) readNextToken() (Token, bool, error) {
	p.retainedBytes = 0
	b, err := p.br.readByte()
	if err != nil {
		return Token{}, false, err
	}
	if b != '<' {
		p.atStart = false
		tok, charErr := p.readCharData(b)
		return tok, false, charErr
	}
	line, col := p.br.pos()
	next, err := p.br.readByte()
	if err != nil {
		return Token{}, false, p.syntaxError("unexpected EOF after <", err)
	}
	return p.readMarkupToken(next, line, col)
}

func (p *Parser) readMarkupToken(next byte, line, col int) (Token, bool, error) {
	atDocumentStart := p.atStart
	p.atStart = false
	switch next {
	case '/':
		return p.readEndToken(line, col)
	case '!':
		return p.readMarkup(line, col)
	case '?':
		return p.readPI(atDocumentStart, line, col)
	default:
		return p.readStartToken(next, line, col)
	}
}

func (p *Parser) readEndToken(line, col int) (Token, bool, error) {
	end, err := p.readEndElement()
	return Token{Kind: KindEnd, End: end, Line: line, Column: col}, false, err
}

func (p *Parser) readStartToken(first byte, line, col int) (Token, bool, error) {
	start, selfClosing, err := p.readStartElement(first)
	if err != nil {
		return Token{}, false, err
	}
	if selfClosing {
		p.pendingEnd = EndElement{Name: start.Name}
	}
	return Token{Kind: KindStart, Start: start, Line: line, Column: col}, false, nil
}

// Pos returns the current parser line and byte column.
func (p *Parser) Pos() (int, int) {
	return p.br.pos()
}

// IsTokenLimit reports whether err is the parser token-byte limit error.
func IsTokenLimit(err error) bool {
	return errors.Is(err, errXMLTokenLimit)
}

// IsInputLimit reports whether err is the parser aggregate input-byte limit.
func IsInputLimit(err error) bool {
	return errors.Is(err, errXMLInputLimit)
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

// IsOnlyEOF reports whether every cause in err is io.EOF. Joined errors with
// any non-EOF cause must not be treated as successful stream completion.
func IsOnlyEOF(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF { //nolint:errorlint // Exact identity is required before inspecting every wrapped or joined cause.
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joinedErrorsAreOnlyEOF(joined.Unwrap())
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return IsOnlyEOF(wrapped.Unwrap())
	}
	return false
}

func joinedErrorsAreOnlyEOF(causes []error) bool {
	if len(causes) == 0 {
		return false
	}
	for _, cause := range causes {
		if !IsOnlyEOF(cause) {
			return false
		}
	}
	return true
}

func (p *Parser) readCharData(first byte) (Token, error) {
	line, col := p.br.pos()
	p.textBuf = p.textBuf[:0]
	cdataEnd := 0
	if err := p.appendCharDataByte(first, &cdataEnd); err != nil {
		return Token{}, err
	}
	for {
		done, err := p.readCharDataStep(&cdataEnd)
		if err != nil || done {
			return Token{Kind: KindCharData, Data: p.textBuf, Line: line, Column: col}, err
		}
	}
}

func (p *Parser) readCharDataStep(cdataEnd *int) (bool, error) {
	progressed, eof, err := p.appendBufferedCharData(cdataEnd)
	if err != nil || eof {
		return eof, err
	}
	if progressed {
		return false, nil
	}
	b, err := p.br.readByte()
	if err != nil {
		return false, err
	}
	if b == '<' {
		p.br.unreadByte()
		return true, nil
	}
	return false, p.appendCharDataByte(b, cdataEnd)
}

func (p *Parser) appendBufferedCharData(cdataEnd *int) (bool, bool, error) {
	chunk, err := p.br.buffered()
	if IsOnlyEOF(err) {
		return false, true, nil
	}
	if err != nil {
		return false, false, err
	}
	n, nextCDataEnd, err := scanCharDataChunk(chunk, *cdataEnd)
	if err != nil || n == 0 {
		return false, false, err
	}
	if err := p.appendTokenBytes(&p.textBuf, chunk[:n]); err != nil {
		return false, false, err
	}
	p.br.consumeBuffered(n)
	*cdataEnd = nextCDataEnd
	return true, false, nil
}

func (p *Parser) appendCharDataByte(b byte, cdataEnd *int) error {
	switch b {
	case '&':
		*cdataEnd = 0
		return p.readEntity(&p.textBuf)
	case '\r':
		*cdataEnd = 0
		return p.appendNormalizedLineFeed(&p.textBuf)
	default:
		*cdataEnd = advanceCDataEnd(*cdataEnd, b)
		if *cdataEnd == len(cdataEndTerm) {
			return fmt.Errorf("]]> cannot appear in character data")
		}
		return p.appendXMLRune(&p.textBuf, b)
	}
}

func (p *Parser) appendNormalizedLineFeed(dst *[]byte) error {
	if err := p.consumeLineFeed(); err != nil {
		return err
	}
	return p.appendTokenByte(dst, '\n')
}

func scanCharDataChunk(data []byte, cdataEnd int) (int, int, error) {
	i := scanFastCharData(data, cdataEnd)
	for i < len(data) {
		size, next, stop, err := scanCharDataUnit(data[i:], cdataEnd)
		if err != nil || stop {
			return i, next, err
		}
		i += size
		cdataEnd = next
	}
	return len(data), cdataEnd, nil
}

func scanFastCharData(data []byte, cdataEnd int) int {
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
	return i
}

func scanCharDataUnit(data []byte, cdataEnd int) (int, int, bool, error) {
	if data[0] < utf8.RuneSelf {
		return scanASCIICharDataByte(data[0], cdataEnd)
	}
	return scanUTF8CharData(data)
}

func scanASCIICharDataByte(b byte, cdataEnd int) (int, int, bool, error) {
	switch {
	case b == '<' || b == '&' || b == '\n' || b == '\r':
		return 0, cdataEnd, true, nil
	case b == '\t':
		return 1, 0, false, nil
	case b < 0x20:
		return 0, cdataEnd, false, fmt.Errorf("invalid XML character")
	default:
		next := advanceCDataEnd(cdataEnd, b)
		if next == len(cdataEndTerm) {
			return 0, next, false, fmt.Errorf("]]> cannot appear in character data")
		}
		return 1, next, false, nil
	}
}

func scanUTF8CharData(data []byte) (int, int, bool, error) {
	if !utf8.FullRune(data) {
		return 0, 0, true, nil
	}
	r, size := utf8.DecodeRune(data)
	if r == utf8.RuneError && size == 1 {
		return 0, 0, false, fmt.Errorf("invalid UTF-8")
	}
	if !lex.IsXMLChar(r) {
		return 0, 0, false, fmt.Errorf("invalid XML character")
	}
	return size, 0, false, nil
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
		return p.readCommentMarkup(line, col)
	case '[':
		return p.readCDATAMarkup(line, col)
	default:
		return p.readDirectiveMarkup(b, line, col)
	}
}

func (p *Parser) readCommentMarkup(line, col int) (Token, bool, error) {
	next, err := p.br.readByte()
	if err != nil {
		return Token{}, false, p.syntaxError("unexpected EOF in comment", err)
	}
	if next != '-' {
		return Token{}, false, fmt.Errorf("invalid XML comment")
	}
	if !p.emitComments {
		return Token{}, true, p.skipComment()
	}
	p.directive = p.directive[:0]
	data, err := p.readComment(p.directive)
	p.directive = data
	return Token{Kind: KindComment, Directive: data, Line: line, Column: col}, false, err
}

func (p *Parser) readCDATAMarkup(line, col int) (Token, bool, error) {
	if err := p.expectString("CDATA["); err != nil {
		return Token{}, false, err
	}
	p.inCDATA = true
	p.cdataMatched = 0
	tok, err := p.readCDATAChunk(line, col)
	return tok, false, err
}

func (p *Parser) readDirectiveMarkup(first byte, line, col int) (Token, bool, error) {
	p.directive = p.directive[:0]
	if err := p.appendTokenByte(&p.directive, first); err != nil {
		return Token{}, false, err
	}
	if err := p.readDirectivePrefix(); err != nil {
		return Token{}, false, err
	}
	if !IsDOCTYPEDeclaration(p.directive) {
		return Token{}, false, fmt.Errorf("invalid markup declaration")
	}
	return Token{Kind: KindDirective, Directive: p.directive, Line: line, Column: col}, false, nil
}

func (p *Parser) readDirectivePrefix() error {
	for len(p.directive) <= len(doctypeDirective) {
		next, err := p.br.readByte()
		if err != nil {
			return p.syntaxError("unexpected EOF in markup declaration", err)
		}
		if err := p.appendTokenByte(&p.directive, next); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) readCDATAChunk(line, col int) (Token, error) {
	if line == 0 {
		line, col = p.br.pos()
	}
	p.textBuf = p.textBuf[:0]
	matched := p.cdataMatched
	for {
		done, err := p.readCDATAByte(&matched)
		if err != nil {
			return Token{}, err
		}
		if done {
			p.inCDATA = false
			p.cdataMatched = 0
			return p.cdataToken(line, col), nil
		}
		if len(p.textBuf) >= len(p.br.buf) {
			p.cdataMatched = matched
			return p.cdataToken(line, col), nil
		}
	}
}

func (p *Parser) cdataToken(line, col int) Token {
	return Token{Kind: KindCharData, Data: p.textBuf, CDATA: true, Line: line, Column: col}
}

func (p *Parser) readCDATAByte(matched *int) (bool, error) {
	b, err := p.br.readByte()
	if err != nil {
		return false, p.syntaxError("unexpected EOF in CDATA section", err)
	}
	switch b {
	case ']':
		return false, p.appendCDATABracket(matched)
	case '>':
		return p.appendCDATAGreaterThan(matched)
	case '\r':
		return false, p.appendNormalizedCDATA(matched)
	default:
		return false, p.appendCDATARune(matched, b)
	}
}

func (p *Parser) appendCDATABracket(matched *int) error {
	if *matched == 2 {
		return p.appendTokenByte(&p.textBuf, ']')
	}
	*matched++
	return nil
}

func (p *Parser) appendCDATAGreaterThan(matched *int) (bool, error) {
	if *matched == 2 {
		return true, nil
	}
	if err := p.appendPendingCDATA(matched); err != nil {
		return false, err
	}
	return false, p.appendTokenByte(&p.textBuf, '>')
}

func (p *Parser) appendNormalizedCDATA(matched *int) error {
	if err := p.appendPendingCDATA(matched); err != nil {
		return err
	}
	return p.appendNormalizedLineFeed(&p.textBuf)
}

func (p *Parser) appendCDATARune(matched *int, first byte) error {
	if err := p.appendPendingCDATA(matched); err != nil {
		return err
	}
	return p.appendXMLRune(&p.textBuf, first)
}

func (p *Parser) appendPendingCDATA(matched *int) error {
	for *matched > 0 {
		if err := p.appendTokenByte(&p.textBuf, ']'); err != nil {
			return err
		}
		*matched--
	}
	return nil
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
			return p.completedStartElement(name, false)
		case '/':
			return p.completeEmptyElement(name)
		default:
			if err := p.readSpacedStartAttribute(b, hadSpace); err != nil {
				return StartElement{}, false, err
			}
		}
	}
}

func (p *Parser) completedStartElement(name xml.Name, selfClosing bool) (StartElement, bool, error) {
	p.finishLazyAttrValues()
	return StartElement{Name: name, Attr: p.attrs}, selfClosing, nil
}

func (p *Parser) completeEmptyElement(name xml.Name) (StartElement, bool, error) {
	next, err := p.br.readByte()
	if err != nil {
		return StartElement{}, false, p.syntaxError("unexpected EOF in empty element tag", err)
	}
	if next != '>' {
		return StartElement{}, false, fmt.Errorf("expected > after / in empty element tag")
	}
	return p.completedStartElement(name, true)
}

func (p *Parser) readSpacedStartAttribute(first byte, hadSpace bool) error {
	if !hadSpace {
		return fmt.Errorf("expected whitespace before attribute")
	}
	return p.readStartAttribute(first)
}

func (p *Parser) readStartAttribute(first byte) error {
	name, err := p.readName(first)
	if err != nil {
		return err
	}
	if p.maxAttrs > 0 && len(p.attrs)+1 > p.maxAttrs {
		return errXMLAttributeLimit
	}
	quote, err := p.readAttributeQuote()
	if err != nil {
		return err
	}
	value, err := p.readStartAttributeValue(quote)
	if err != nil {
		return err
	}
	p.recordStartAttribute(name, value)
	return nil
}

func (p *Parser) readAttributeQuote() (byte, error) {
	b, _, err := p.readPastSpace()
	if err != nil {
		return 0, err
	}
	if b != '=' {
		return 0, fmt.Errorf("expected = after attribute name")
	}
	b, _, err = p.readPastSpace()
	if err != nil {
		return 0, err
	}
	if b != '"' && b != '\'' {
		return 0, fmt.Errorf("attribute value must be quoted")
	}
	return b, nil
}

func (p *Parser) readStartAttributeValue(quote byte) ([]byte, error) {
	if !p.lazyAttrValue {
		p.attrValueBuf = p.attrValueBuf[:0]
	}
	return p.readAttributeValueBytes(quote, &p.attrValueBuf)
}

func (p *Parser) recordStartAttribute(name xml.Name, value []byte) {
	attr := Attr{Name: name}
	if p.lazyAttrValue {
		p.attrValueEnds = append(p.attrValueEnds, len(p.attrValueBuf))
	} else {
		attr.Value = p.values.Intern(value)
	}
	p.attrs = append(p.attrs, attr)
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
		done, err := p.appendBufferedName()
		if err != nil {
			return xml.Name{}, err
		}
		if done {
			break
		}
	}
	return p.internQName(p.nameBuf)
}

func (p *Parser) appendBufferedName() (bool, error) {
	chunk, err := p.br.buffered()
	if err != nil {
		return false, xmlNameReadError(err)
	}
	n := nameChunkLen(chunk)
	if err := p.appendTokenBytes(&p.nameBuf, chunk[:n]); err != nil {
		return false, err
	}
	p.br.consumeBuffered(n)
	return n < len(chunk), nil
}

func xmlNameReadError(err error) error {
	if IsOnlyEOF(err) {
		return fmt.Errorf("unexpected EOF in XML name")
	}
	return err
}

func (p *Parser) internQName(name []byte) (xml.Name, error) {
	if len(name) == 0 {
		return xml.Name{}, fmt.Errorf("empty XML name")
	}
	if prefix, local, ascii, ok := lex.SplitASCIIQNameBytes(p.nameBuf); ascii {
		if !ok {
			return xml.Name{}, fmt.Errorf("invalid XML qualified name")
		}
		return p.internQNameParts(prefix, local), nil
	}
	prefix, local, ok := splitUnicodeQName(name)
	if !ok {
		return xml.Name{}, fmt.Errorf("invalid XML qualified name")
	}
	return p.internQNameParts(prefix, local), nil
}

func splitUnicodeQName(name []byte) ([]byte, []byte, bool) {
	colon := bytes.IndexByte(name, ':')
	if colon < 0 {
		return nil, name, lex.IsNCNameBytes(name)
	}
	if colon == 0 || colon == len(name)-1 || bytes.IndexByte(name[colon+1:], ':') >= 0 {
		return nil, nil, false
	}
	prefix, local := name[:colon], name[colon+1:]
	return prefix, local, lex.IsNCNameBytes(prefix) && lex.IsNCNameBytes(local)
}

func (p *Parser) internQNameParts(prefix, local []byte) xml.Name {
	if prefix == nil {
		return xml.Name{Local: p.names.Intern(local)}
	}
	return xml.Name{
		Space: p.names.Intern(prefix),
		Local: p.names.Intern(local),
	}
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
		progressed, err := p.appendBufferedAttributeValue(quote, dst)
		if err != nil {
			return nil, err
		}
		if progressed {
			continue
		}
		done, err := p.readAttributeValueByte(quote, dst)
		if err != nil {
			return nil, err
		}
		if done {
			return (*dst)[start:], nil
		}
	}
}

func (p *Parser) appendBufferedAttributeValue(quote byte, dst *[]byte) (bool, error) {
	chunk, err := p.br.buffered()
	if IsOnlyEOF(err) {
		return false, p.syntaxError("unexpected EOF in attribute value", err)
	}
	if err != nil {
		return false, err
	}
	n := attributeValueChunkLen(chunk, quote)
	if n == 0 {
		return false, nil
	}
	if err := p.appendTokenBytes(dst, chunk[:n]); err != nil {
		return false, err
	}
	p.br.consumeBuffered(n)
	return true, nil
}

func (p *Parser) readAttributeValueByte(quote byte, dst *[]byte) (bool, error) {
	b, err := p.br.readByte()
	if err != nil {
		return false, p.syntaxError("unexpected EOF in attribute value", err)
	}
	switch b {
	case quote:
		return true, nil
	case '<':
		return false, fmt.Errorf("attribute value cannot contain <")
	case '\r':
		return false, p.appendNormalizedAttributeSpace(dst)
	case '\n', '\t':
		return false, p.appendTokenByte(dst, ' ')
	case '&':
		return false, p.readEntity(dst)
	default:
		return false, p.appendXMLRune(dst, b)
	}
}

func (p *Parser) appendNormalizedAttributeSpace(dst *[]byte) error {
	if err := p.consumeLineFeed(); err != nil {
		return err
	}
	return p.appendTokenByte(dst, ' ')
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
	if err := p.scanComment(&dst); err != nil {
		return nil, err
	}
	return dst, nil
}

func (p *Parser) skipComment() error {
	return p.scanComment(nil)
}

func (p *Parser) scanComment(dst *[]byte) error {
	prevDash := false
	for {
		b, err := p.br.readByte()
		if err != nil {
			return p.syntaxError("unexpected EOF in comment", err)
		}
		done, err := p.consumeCommentByte(dst, b, &prevDash)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func (p *Parser) consumeCommentByte(dst *[]byte, b byte, prevDash *bool) (bool, error) {
	switch {
	case b == '-' && *prevDash:
		return true, p.finishCommentAfterDoubleDash()
	case b == '-':
		*prevDash = true
		return false, nil
	default:
		if err := p.appendPendingCommentDash(dst, prevDash); err != nil {
			return false, err
		}
		return false, p.appendCommentRune(dst, b)
	}
}

func (p *Parser) appendPendingCommentDash(dst *[]byte, pending *bool) error {
	if !*pending {
		return nil
	}
	*pending = false
	if dst == nil {
		return nil
	}
	return p.appendTokenByte(dst, '-')
}

func (p *Parser) appendCommentRune(dst *[]byte, first byte) error {
	if dst == nil {
		return p.consumeXMLRune(first)
	}
	return p.appendXMLRune(dst, first)
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
		if IsOnlyEOF(err) {
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
		done, err := p.skipUntilByte(term, prefix, b, &matched)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func (p *Parser) skipUntilByte(term string, prefix []int, b byte, matched *int) (bool, error) {
	if b >= utf8.RuneSelf {
		*matched = 0
		return false, p.consumeXMLRune(b)
	}
	if !lex.IsXMLChar(rune(b)) {
		return false, fmt.Errorf("invalid XML character")
	}
	*matched = advanceTermMatch(term, prefix, *matched, b)
	return *matched == len(term), nil
}

// ValidateXMLDeclContent validates the content inside an XML declaration.
func ValidateXMLDeclContent(content []byte) error {
	rest, err := validateXMLVersionAttribute(content)
	if err != nil {
		return err
	}
	rest, err = validateOptionalXMLEncoding(rest)
	if err != nil {
		return err
	}
	rest, err = validateOptionalXMLStandalone(rest)
	if err != nil {
		return err
	}
	if len(lex.TrimXMLWhitespaceBytes(rest)) != 0 {
		return fmt.Errorf("invalid XML declaration")
	}
	return nil
}

func validateXMLVersionAttribute(content []byte) ([]byte, error) {
	name, version, rest, ok := ScanXMLDeclAttr(content, XMLDeclFirstAttr)
	if !ok || name != xsdAttrVersion {
		return nil, fmt.Errorf("invalid XML declaration")
	}
	if version != xmlVersion10 {
		return nil, UnsupportedXMLVersionError{Version: version}
	}
	return rest, nil
}

func validateOptionalXMLEncoding(content []byte) ([]byte, error) {
	name, value, rest, ok := ScanXMLDeclAttr(content, XMLDeclNextAttr)
	if !ok || name != "encoding" {
		return content, nil
	}
	if !strings.EqualFold(value, "UTF-8") && !strings.EqualFold(value, "UTF8") {
		return nil, ErrUnsupportedNonUTF8
	}
	return rest, nil
}

func validateOptionalXMLStandalone(content []byte) ([]byte, error) {
	name, value, rest, ok := ScanXMLDeclAttr(content, XMLDeclNextAttr)
	if !ok || name != "standalone" {
		return content, nil
	}
	if value != "yes" && value != "no" {
		return nil, fmt.Errorf("invalid XML declaration")
	}
	return rest, nil
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
	content, ok := startXMLDeclAttribute(content, pos)
	if !ok {
		return "", "", content, false
	}
	name, content, ok := scanXMLDeclAttributeName(content)
	if !ok {
		return "", "", content, false
	}
	value, rest, ok := scanXMLDeclAttributeValue(content)
	if !ok {
		return "", "", content, false
	}
	return name, value, rest, true
}

func startXMLDeclAttribute(content []byte, pos XMLDeclAttrPosition) ([]byte, bool) {
	if pos == XMLDeclNextAttr && (len(content) == 0 || !lex.IsXMLWhitespaceByte(content[0])) {
		return content, false
	}
	return bytes.TrimLeft(content, " \t\r\n"), true
}

func scanXMLDeclAttributeName(content []byte) (string, []byte, bool) {
	nameLen := 0
	for nameLen < len(content) && !isXMLDeclNameTerminator(content[nameLen]) {
		nameLen++
	}
	if nameLen == 0 {
		return "", content, false
	}
	return string(content[:nameLen]), bytes.TrimLeft(content[nameLen:], " \t\r\n"), true
}

func isXMLDeclNameTerminator(b byte) bool {
	return b == '=' || b == '"' || b == '\'' || lex.IsXMLWhitespaceByte(b)
}

func scanXMLDeclAttributeValue(content []byte) (string, []byte, bool) {
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

func (p *Parser) appendXMLRune(dst *[]byte, first byte) error {
	runeBytes, err := p.readXMLRuneBytes(first)
	if err != nil {
		return err
	}
	return p.appendTokenBytes(dst, runeBytes.buf[:runeBytes.size])
}

func (p *Parser) consumeXMLRune(first byte) error {
	_, err := p.readXMLRuneBytes(first)
	return err
}

type xmlRuneBytes struct {
	buf  [utf8.UTFMax]byte
	size int
}

func (p *Parser) readXMLRuneBytes(first byte) (xmlRuneBytes, error) {
	if first < utf8.RuneSelf {
		if !lex.IsXMLChar(rune(first)) {
			return xmlRuneBytes{}, fmt.Errorf("invalid XML character")
		}
		return xmlRuneBytes{buf: [utf8.UTFMax]byte{first}, size: 1}, nil
	}
	return p.readMultibyteXMLRune(first)
}

func (p *Parser) readMultibyteXMLRune(first byte) (xmlRuneBytes, error) {
	var result xmlRuneBytes
	result.buf[0] = first
	n := 1
	for !utf8.FullRune(result.buf[:n]) {
		if n == len(result.buf) {
			return xmlRuneBytes{}, fmt.Errorf("invalid UTF-8")
		}
		b, err := p.br.readByte()
		if err != nil {
			return xmlRuneBytes{}, p.syntaxError("unexpected EOF in UTF-8 sequence", err)
		}
		result.buf[n] = b
		n++
	}
	r, size := utf8.DecodeRune(result.buf[:n])
	if r == utf8.RuneError && size == 1 {
		return xmlRuneBytes{}, fmt.Errorf("invalid UTF-8")
	}
	if !lex.IsXMLChar(r) {
		return xmlRuneBytes{}, fmt.Errorf("invalid XML character")
	}
	result.size = size
	return result, nil
}

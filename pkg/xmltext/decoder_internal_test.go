package xmltext

import (
	"bytes"
	"errors"
	"hash/maphash"
	"io"
	"strings"
	"testing"
)

func TestDecoderFailBehavior(t *testing.T) {
	dec := &Decoder{}
	if err := dec.fail(nil); err != nil {
		t.Fatalf("fail(nil) = %v, want nil", err)
	}

	dec = &Decoder{}
	if err := dec.fail(io.EOF); !errors.Is(err, io.EOF) {
		t.Fatalf("fail(io.EOF) = %v, want io.EOF", err)
	}
	if !errors.Is(dec.err, io.EOF) {
		t.Fatalf("decoder err = %v, want io.EOF", dec.err)
	}

	dec = &Decoder{}
	syntax := &SyntaxError{Err: errInvalidToken}
	if err := dec.fail(syntax); !errors.Is(err, syntax) {
		t.Fatalf("fail(SyntaxError) = %v, want %v", err, syntax)
	}
	if !errors.Is(dec.err, syntax) {
		t.Fatalf("decoder err = %v, want %v", dec.err, syntax)
	}

	dec = &Decoder{
		opts:   decoderOptions{trackLineColumn: true},
		buf:    spanBuffer{data: []byte("abc")},
		line:   4,
		column: 2,
		pos:    1,
	}
	err := dec.fail(errInvalidName)
	var syn *SyntaxError
	if !errors.As(err, &syn) {
		t.Fatalf("fail error = %T, want *SyntaxError", err)
	}
	if !errors.Is(err, errInvalidName) {
		t.Fatalf("fail error = %v, want %v", err, errInvalidName)
	}
	if syn.Line != dec.line || syn.Column != dec.column {
		t.Fatalf("syntax line/column = %d/%d, want %d/%d", syn.Line, syn.Column, dec.line, dec.column)
	}
}

func TestScanCharDataSpanUntilEntity(t *testing.T) {
	idx, err := scanCharDataSpanUntilEntity([]byte("abc&def"), 0)
	if err != nil {
		t.Fatalf("scanCharDataSpanUntilEntity error = %v", err)
	}
	if idx != 3 {
		t.Fatalf("scanCharDataSpanUntilEntity idx = %d, want 3", idx)
	}

	idx, err = scanCharDataSpanUntilEntity([]byte("abc"), 0)
	if err != nil {
		t.Fatalf("scanCharDataSpanUntilEntity no-entity error = %v", err)
	}
	if idx != -1 {
		t.Fatalf("scanCharDataSpanUntilEntity idx = %d, want -1", idx)
	}

	_, err = scanCharDataSpanUntilEntity([]byte("abc"), -1)
	if !errors.Is(err, errInvalidChar) {
		t.Fatalf("scanCharDataSpanUntilEntity negative error = %v, want %v", err, errInvalidChar)
	}

	_, err = scanCharDataSpanUntilEntity([]byte("]]>"), 0)
	if !errors.Is(err, errInvalidToken) {
		t.Fatalf("scanCharDataSpanUntilEntity token error = %v, want %v", err, errInvalidToken)
	}

	_, err = scanCharDataSpanUntilEntity([]byte{0x01}, 0)
	if !errors.Is(err, errInvalidChar) {
		t.Fatalf("scanCharDataSpanUntilEntity char error = %v, want %v", err, errInvalidChar)
	}

	_, err = scanCharDataSpanUntilEntity([]byte{0xff}, 0)
	if !errors.Is(err, errInvalidChar) {
		t.Fatalf("scanCharDataSpanUntilEntity utf8 error = %v, want %v", err, errInvalidChar)
	}

	idx, err = scanCharDataSpanUntilEntity([]byte("a\u00e9b"), 0)
	if err != nil {
		t.Fatalf("scanCharDataSpanUntilEntity unicode error = %v", err)
	}
	if idx != -1 {
		t.Fatalf("scanCharDataSpanUntilEntity unicode idx = %d, want -1", idx)
	}
}

func TestUnescapeCharDataInto(t *testing.T) {
	out, rawNeeds, err := unescapeCharDataInto(nil, []byte("ok"), &entityResolver{}, 0)
	if err != nil {
		t.Fatalf("unescapeCharDataInto ok error = %v", err)
	}
	if rawNeeds {
		t.Fatalf("unescapeCharDataInto rawNeeds = true, want false")
	}
	if len(out) != 0 {
		t.Fatalf("unescapeCharDataInto = %q, want empty", out)
	}

	out, rawNeeds, err = unescapeCharDataInto(nil, []byte("a&amp;b"), &entityResolver{}, 0)
	if err != nil {
		t.Fatalf("unescapeCharDataInto entity error = %v", err)
	}
	if !rawNeeds {
		t.Fatalf("unescapeCharDataInto rawNeeds = false, want true")
	}
	if string(out) != "a&b" {
		t.Fatalf("unescapeCharDataInto = %q, want a&b", out)
	}

	_, _, err = unescapeCharDataInto(nil, []byte("]]>"), &entityResolver{}, 0)
	if !errors.Is(err, errInvalidToken) {
		t.Fatalf("unescapeCharDataInto token error = %v, want %v", err, errInvalidToken)
	}

	_, _, err = unescapeCharDataInto(nil, []byte{0x00}, &entityResolver{}, 0)
	if !errors.Is(err, errInvalidChar) {
		t.Fatalf("unescapeCharDataInto char error = %v, want %v", err, errInvalidChar)
	}

	_, _, err = unescapeCharDataInto(nil, []byte("a&bogus;"), &entityResolver{}, 0)
	if !errors.Is(err, errInvalidEntity) {
		t.Fatalf("unescapeCharDataInto entity error = %v, want %v", err, errInvalidEntity)
	}

	_, _, err = unescapeCharDataInto(nil, []byte("a&amp;b"), &entityResolver{}, 2)
	if !errors.Is(err, errTokenTooLarge) {
		t.Fatalf("unescapeCharDataInto size error = %v, want %v", err, errTokenTooLarge)
	}
}

func TestScanAttrValueBranches(t *testing.T) {
	dec := newAttrValueDecoder([]byte("abc\""))
	rawSpan, valueSpan, needs, rawNeeds, err := dec.scanAttrValue('"', false)
	if err != nil {
		t.Fatalf("scanAttrValue raw error = %v", err)
	}
	if needs || rawNeeds {
		t.Fatalf("scanAttrValue needs/rawNeeds = %v/%v, want false/false", needs, rawNeeds)
	}
	if got := string(rawSpan.bytes()); got != "abc" {
		t.Fatalf("rawSpan = %q, want abc", got)
	}
	if got := string(valueSpan.bytes()); got != "abc" {
		t.Fatalf("valueSpan = %q, want abc", got)
	}

	dec = newAttrValueDecoder([]byte("a&amp;b\""), ResolveEntities(true))
	rawSpan, valueSpan, needs, rawNeeds, err = dec.scanAttrValue('"', false)
	if err != nil {
		t.Fatalf("scanAttrValue resolve error = %v", err)
	}
	if needs || !rawNeeds {
		t.Fatalf("scanAttrValue resolve needs/rawNeeds = %v/%v, want false/true", needs, rawNeeds)
	}
	if got := string(rawSpan.bytes()); got != "a&amp;b" {
		t.Fatalf("rawSpan = %q, want a&amp;b", got)
	}
	if got := string(valueSpan.bytes()); got != "a&b" {
		t.Fatalf("valueSpan = %q, want a&b", got)
	}

	dec = newAttrValueDecoder([]byte("a&amp;b\""))
	_, valueSpan, needs, rawNeeds, err = dec.scanAttrValue('"', false)
	if err != nil {
		t.Fatalf("scanAttrValue text error = %v", err)
	}
	if !needs || !rawNeeds {
		t.Fatalf("scanAttrValue text needs/rawNeeds = %v/%v, want true/true", needs, rawNeeds)
	}
	if got := string(valueSpan.bytes()); got != "a&amp;b" {
		t.Fatalf("valueSpan = %q, want a&amp;b", got)
	}

	dec = newAttrValueDecoder([]byte("a<\""))
	if _, _, _, _, err := dec.scanAttrValue('"', false); !errors.Is(err, errInvalidToken) {
		t.Fatalf("scanAttrValue token error = %v, want %v", err, errInvalidToken)
	}

	dec = newAttrValueDecoder([]byte{0x01, '"'})
	if _, _, _, _, err := dec.scanAttrValue('"', false); !errors.Is(err, errInvalidChar) {
		t.Fatalf("scanAttrValue char error = %v, want %v", err, errInvalidChar)
	}

	dec = newAttrValueDecoder([]byte("a&bogus;\""))
	if _, _, _, _, err := dec.scanAttrValue('"', false); !errors.Is(err, errInvalidEntity) {
		t.Fatalf("scanAttrValue entity error = %v, want %v", err, errInvalidEntity)
	}

	dec = newAttrValueDecoder([]byte("abcd\""), MaxTokenSize(2))
	if _, _, _, _, err := dec.scanAttrValue('"', false); !errors.Is(err, errTokenTooLarge) {
		t.Fatalf("scanAttrValue size error = %v, want %v", err, errTokenTooLarge)
	}

	dec = newAttrValueDecoder([]byte("abc"))
	dec.eof = true
	if _, _, _, _, err := dec.scanAttrValue('"', false); !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("scanAttrValue EOF error = %v, want %v", err, errUnexpectedEOF)
	}
}

func TestWhitespaceCharData(t *testing.T) {
	dec := NewDecoder(strings.NewReader(""))
	ok, err := dec.isWhitespaceCharData(nil)
	if err != nil || !ok {
		t.Fatalf("isWhitespaceCharData(nil) = %v/%v, want true/nil", ok, err)
	}

	buf := spanBuffer{data: []byte(" \t\r\n")}
	tok := &rawToken{text: makeSpan(&buf, 0, len(buf.data))}
	ok, err = dec.isWhitespaceCharData(tok)
	if err != nil || !ok {
		t.Fatalf("isWhitespaceCharData whitespace = %v/%v, want true/nil", ok, err)
	}

	buf = spanBuffer{data: []byte("&#x20;")}
	tok = &rawToken{text: makeSpan(&buf, 0, len(buf.data)), textNeeds: true}
	ok, err = dec.isWhitespaceCharData(tok)
	if err != nil || !ok {
		t.Fatalf("isWhitespaceCharData entity = %v/%v, want true/nil", ok, err)
	}

	buf = spanBuffer{data: []byte("&bogus;")}
	tok = &rawToken{text: makeSpan(&buf, 0, len(buf.data)), textNeeds: true}
	if _, err := dec.isWhitespaceCharData(tok); !errors.Is(err, errInvalidEntity) {
		t.Fatalf("isWhitespaceCharData error = %v, want %v", err, errInvalidEntity)
	}
}

func TestMatchLiteral(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("<?xml")}}
	ok, err := dec.matchLiteral([]byte("<?xml"), false)
	if err != nil {
		t.Fatalf("matchLiteral error = %v", err)
	}
	if !ok {
		t.Fatalf("matchLiteral = false, want true")
	}

	dec = &Decoder{buf: spanBuffer{data: []byte("<!")}, eof: true}
	_, err = dec.matchLiteral(litComStart, false)
	if !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("matchLiteral error = %v, want %v", err, errUnexpectedEOF)
	}

	dec = &Decoder{buf: spanBuffer{data: []byte("<!x-")}}
	ok, err = dec.matchLiteral(litComStart, false)
	if err != nil {
		t.Fatalf("matchLiteral mismatch error = %v", err)
	}
	if ok {
		t.Fatalf("matchLiteral = true, want false")
	}
}

func TestAdvanceHelpers(t *testing.T) {
	dec := &Decoder{opts: decoderOptions{trackLineColumn: true}, line: 1, column: 1}
	dec.advanceName(0)
	dec.advanceRaw(-1)
	if dec.pos != 0 || dec.column != 1 {
		t.Fatalf("advance helpers pos/column = %d/%d, want 0/1", dec.pos, dec.column)
	}
	dec.advanceName(2)
	if dec.pos != 2 || dec.column != 3 {
		t.Fatalf("advanceName pos/column = %d/%d, want 2/3", dec.pos, dec.column)
	}
	dec.advanceRaw(1)
	if dec.pos != 3 || dec.column != 4 {
		t.Fatalf("advanceRaw pos/column = %d/%d, want 3/4", dec.pos, dec.column)
	}
}

func TestAdvanceCRLFAcrossChunks(t *testing.T) {
	dec := &Decoder{opts: decoderOptions{trackLineColumn: true}, line: 1, column: 1}
	dec.buf.data = []byte("\r\nx")

	dec.advance(1)
	if dec.pos != 1 || dec.line != 2 || dec.column != 1 {
		t.Fatalf("after CR pos/line/column = %d/%d/%d, want 1/2/1", dec.pos, dec.line, dec.column)
	}

	dec.advance(1)
	if dec.pos != 2 || dec.line != 2 || dec.column != 1 {
		t.Fatalf("after LF pos/line/column = %d/%d/%d, want 2/2/1", dec.pos, dec.line, dec.column)
	}

	dec.advance(1)
	if dec.pos != 3 || dec.line != 2 || dec.column != 2 {
		t.Fatalf("after char pos/line/column = %d/%d/%d, want 3/2/2", dec.pos, dec.line, dec.column)
	}
}

func TestPopStackInterned(t *testing.T) {
	dec := &Decoder{}
	if err := dec.popStackInterned(&qnameSpan{}); !errors.Is(err, errMismatchedEndTag) {
		t.Fatalf("popStackInterned empty error = %v, want %v", err, errMismatchedEndTag)
	}
	buf := spanBuffer{data: []byte("root")}
	name := newQNameSpan(&buf, 0, len(buf.data))
	dec.stack = []stackEntry{{name: name}}
	otherBuf := spanBuffer{data: []byte("other")}
	other := newQNameSpan(&otherBuf, 0, len(otherBuf.data))
	if err := dec.popStackInterned(&other); !errors.Is(err, errMismatchedEndTag) {
		t.Fatalf("popStackInterned mismatch error = %v, want %v", err, errMismatchedEndTag)
	}
}

func TestScanName(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("name rest")}}
	nameSpan, err := dec.scanName(false)
	if err != nil {
		t.Fatalf("scanName error = %v", err)
	}
	if got := string(nameSpan.bytes()); got != "name" {
		t.Fatalf("scanName = %q, want name", got)
	}

	dec = &Decoder{buf: spanBuffer{data: []byte("1bad")}}
	_, err = dec.scanName(false)
	if !errors.Is(err, errInvalidName) {
		t.Fatalf("scanName invalid error = %v, want %v", err, errInvalidName)
	}

	dec = &Decoder{buf: spanBuffer{data: []byte("\u00e9x ")}, eof: true}
	nameSpan, err = dec.scanName(false)
	if err != nil {
		t.Fatalf("scanName unicode error = %v", err)
	}
	if got := string(nameSpan.bytes()); got != "\u00e9x" {
		t.Fatalf("scanName unicode = %q, want \\u00e9x", got)
	}
}

func TestScanNameUnexpectedEOF(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: nil}, eof: true}
	if _, err := dec.scanName(false); !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("scanName error = %v, want %v", err, errUnexpectedEOF)
	}
}

func TestPeekRuneErrors(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte{0xff}}}
	if _, _, err := dec.peekRune(false); !errors.Is(err, errInvalidChar) {
		t.Fatalf("peekRune invalid utf8 error = %v, want %v", err, errInvalidChar)
	}

	dec = &Decoder{buf: spanBuffer{data: []byte{0xc3}}, eof: true}
	if _, _, err := dec.peekRune(false); !errors.Is(err, errInvalidChar) {
		t.Fatalf("peekRune incomplete error = %v, want %v", err, errInvalidChar)
	}

	dec = &Decoder{buf: spanBuffer{data: []byte("a")}}
	r, size, err := dec.peekRune(false)
	if err != nil {
		t.Fatalf("peekRune ascii error = %v", err)
	}
	if r != 'a' || size != 1 {
		t.Fatalf("peekRune = %q/%d, want a/1", r, size)
	}
}

func TestPeekRuneReadMore(t *testing.T) {
	dec := NewDecoder(strings.NewReader("\u20ac"), bufferSize(1))
	r, size, err := dec.peekRune(false)
	if err != nil {
		t.Fatalf("peekRune error = %v", err)
	}
	if r != '\u20ac' || size != len([]byte("\u20ac")) {
		t.Fatalf("peekRune = %q/%d, want \\u20ac/3", r, size)
	}
}

func TestPeekRuneIncompleteRead(t *testing.T) {
	reader := io.LimitReader(strings.NewReader("\u20ac"), 1)
	dec := NewDecoder(reader, bufferSize(1))
	if _, _, err := dec.peekRune(false); !errors.Is(err, errInvalidChar) {
		t.Fatalf("peekRune error = %v, want %v", err, errInvalidChar)
	}
}

func TestResetAttrSeenOverflow(t *testing.T) {
	dec := &Decoder{}
	dec.attrSeen = map[uint64]attrBucket{
		1: {gen: 1, spans: []span{{}}},
	}
	dec.attrSeenGen = ^uint32(0)
	dec.resetAttrSeen()
	if dec.attrSeenGen != 1 {
		t.Fatalf("attrSeenGen = %d, want 1", dec.attrSeenGen)
	}
	if len(dec.attrSeen) != 0 {
		t.Fatalf("attrSeen len = %d, want 0", len(dec.attrSeen))
	}
}

func TestRefreshToken(t *testing.T) {
	buf := spanBuffer{data: []byte("root attr val"), gen: 2}
	name := newQNameSpan(&buf, 0, 4)
	attrName := newQNameSpan(&buf, 5, 9)
	attrValue := makeSpan(&buf, 10, 13)
	tok := rawToken{
		kind: KindStartElement,
		name: name,
		attrs: []attrSpan{{
			Name:      attrName,
			ValueSpan: attrValue,
		}},
	}
	tok.name.Full.gen = 0
	tok.name.Local.gen = 0
	tok.attrs[0].Name.Full.gen = 0
	tok.attrs[0].ValueSpan.gen = 0

	dec := &Decoder{}
	dec.refreshToken(&tok)
	if tok.name.Full.gen != buf.gen {
		t.Fatalf("name gen = %d, want %d", tok.name.Full.gen, buf.gen)
	}
	if tok.attrs[0].ValueSpan.gen != buf.gen {
		t.Fatalf("attr value gen = %d, want %d", tok.attrs[0].ValueSpan.gen, buf.gen)
	}
}

func TestCompact(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("abcd")}, pos: 2, baseOffset: 5}
	dec.compact()
	if got := string(dec.buf.data); got != "cd" {
		t.Fatalf("compact data = %q, want cd", got)
	}
	if dec.pos != 0 {
		t.Fatalf("compact pos = %d, want 0", dec.pos)
	}
	if dec.baseOffset != 7 {
		t.Fatalf("compact baseOffset = %d, want 7", dec.baseOffset)
	}

	dec.buf.data = []byte("abcd")
	dec.pos = 4
	dec.baseOffset = 0
	dec.compact()
	if len(dec.buf.data) != 0 {
		t.Fatalf("compact cleared len = %d, want 0", len(dec.buf.data))
	}
	if dec.pos != 0 {
		t.Fatalf("compact cleared pos = %d, want 0", dec.pos)
	}
	if dec.baseOffset != 4 {
		t.Fatalf("compact cleared baseOffset = %d, want 4", dec.baseOffset)
	}
}

func TestScanPIInto(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<?pi?><root/>"), EmitPI(true))
	reader := newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken PI error = %v", err)
	}
	if tok.Kind != KindPI {
		t.Fatalf("PI kind = %v, want %v", tok.Kind, KindPI)
	}
	if got := string(tok.Text); got != "pi" {
		t.Fatalf("PI text = %q, want pi", got)
	}

	dec = NewDecoder(strings.NewReader("<?pi test?><root/>"), EmitPI(true))
	reader = newTokenReader(dec)
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken PI data error = %v", err)
	}
	if tok.Kind != KindPI {
		t.Fatalf("PI data kind = %v, want %v", tok.Kind, KindPI)
	}

	dec = NewDecoder(strings.NewReader("<?xml?>"), EmitPI(true))
	reader = newTokenReader(dec)
	if _, err := reader.Next(); !errors.Is(err, errInvalidPI) {
		t.Fatalf("ReadToken xml PI error = %v, want %v", err, errInvalidPI)
	}
}

func TestScanDirectiveInto(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<!DOCTYPE root><root/>"), EmitDirectives(true))
	reader := newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken directive error = %v", err)
	}
	if tok.Kind != KindDirective {
		t.Fatalf("directive kind = %v, want %v", tok.Kind, KindDirective)
	}
}

func TestCompactIfNeededFloor(t *testing.T) {
	dec := &Decoder{
		buf:             spanBuffer{data: []byte("abcd")},
		pos:             2,
		baseOffset:      10,
		compactFloorSet: true,
		compactFloorAbs: 5,
	}
	dec.compactIfNeeded()
	if dec.pos != 2 {
		t.Fatalf("pos = %d, want 2", dec.pos)
	}
	if string(dec.buf.data) != "abcd" {
		t.Fatalf("data = %q, want abcd", dec.buf.data)
	}
	if dec.baseOffset != 10 {
		t.Fatalf("baseOffset = %d, want 10", dec.baseOffset)
	}
}

func TestCompactIfNeededClear(t *testing.T) {
	dec := &Decoder{
		buf:        spanBuffer{data: []byte("abcd")},
		pos:        4,
		baseOffset: 7,
	}
	dec.compactIfNeeded()
	if dec.pos != 0 {
		t.Fatalf("pos = %d, want 0", dec.pos)
	}
	if dec.baseOffset != 11 {
		t.Fatalf("baseOffset = %d, want 11", dec.baseOffset)
	}
	if len(dec.buf.data) != 0 {
		t.Fatalf("data len = %d, want 0", len(dec.buf.data))
	}
}

func TestScanNameAcrossBuffer(t *testing.T) {
	dec := newStreamingDecoder([]byte{0xCF}, []byte{0x80, 'x', ' '})
	nameSpan, err := dec.scanName(false)
	if err != nil {
		t.Fatalf("scanName error = %v", err)
	}
	if got := string(nameSpan.bytes()); got != "\u03c0x" {
		t.Fatalf("scanName = %q, want \\u03c0x", got)
	}
}

func TestScanQNameAcrossBuffer(t *testing.T) {
	dec := newStreamingDecoder([]byte("p:"), []byte{0xCF, 0x80, ' '})
	qname, err := dec.scanQName(false)
	if err != nil {
		t.Fatalf("scanQName error = %v", err)
	}
	if got := string(qname.Full.bytes()); got != "p:\u03c0" {
		t.Fatalf("scanQName = %q, want p:\\u03c0", got)
	}
	if got := string(qname.Prefix.bytes()); got != "p" {
		t.Fatalf("prefix = %q, want p", got)
	}
	if got := string(qname.Local.bytes()); got != "\u03c0" {
		t.Fatalf("local = %q, want \\u03c0", got)
	}
}

func TestScanQNameReadMoreWithoutCompaction(t *testing.T) {
	const bufLen = 64 * 1024
	const prefix = "prefix"
	const suffix = "suffix"

	buf := bytes.Repeat([]byte{'x'}, bufLen)
	start := bufLen - len(prefix)
	copy(buf[start:], prefix)

	dec := NewDecoder(bytes.NewReader([]byte(suffix + ">")))
	dec.buf.data = buf
	dec.pos = start

	span, err := dec.scanQName(true)
	if err != nil {
		t.Fatalf("scanQName error = %v", err)
	}
	if got := string(span.Full.bytes()); got != prefix+suffix {
		t.Fatalf("scanQName = %q, want %q", got, prefix+suffix)
	}
}

func TestDecoderInternQNameHelpers(t *testing.T) {
	buf := spanBuffer{data: []byte("ns:local")}
	name := makeQNameSpan(&buf, 0, len(buf.data), 2)

	dec := &Decoder{}
	interned := dec.internQName(&name)
	if got := string(interned.Full.bytes()); got != "ns:local" {
		t.Fatalf("internQName full = %q, want ns:local", got)
	}
	if got := string(interned.Local.bytes()); got != "local" {
		t.Fatalf("internQName local = %q, want local", got)
	}

	hash := maphash.Bytes(hashSeed, buf.data)
	dec = &Decoder{}
	interned = dec.internQNameHash(&name, hash)
	if got := string(interned.Prefix.bytes()); got != "ns" {
		t.Fatalf("internQNameHash prefix = %q, want ns", got)
	}
}

func newStreamingDecoder(prefix, rest []byte) *Decoder {
	data := make([]byte, len(prefix), len(prefix)+len(rest))
	copy(data, prefix)
	return &Decoder{
		buf: spanBuffer{data: data},
		r:   bytes.NewReader(rest),
	}
}

func newAttrValueDecoder(data []byte, opts ...Options) *Decoder {
	dec := NewDecoder(strings.NewReader(""), opts...)
	dec.buf.data = data
	dec.pos = 0
	dec.eof = true
	return dec
}

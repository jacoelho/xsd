package xmltext

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDecoderNilAccessors(t *testing.T) {
	var dec *Decoder
	if got := dec.PeekKind(); got != KindNone {
		t.Fatalf("PeekKind = %v, want %v", got, KindNone)
	}
	if got := dec.InputOffset(); got != 0 {
		t.Fatalf("InputOffset = %d, want 0", got)
	}
	if got := dec.StackDepth(); got != 0 {
		t.Fatalf("StackDepth = %d, want 0", got)
	}
	if got := dec.InternStats(); got != (InternStats{}) {
		t.Fatalf("InternStats = %v, want zero", got)
	}
	if err := dec.SkipValue(); !errors.Is(err, errNilReader) {
		t.Fatalf("SkipValue error = %v, want %v", err, errNilReader)
	}
	if _, ok := GetOption(dec.Options(), ResolveEntities); ok {
		t.Fatalf("Options ResolveEntities = true, want false")
	}
}

func TestDecoderFailBehavior(t *testing.T) {
	dec := &Decoder{}
	if err := dec.fail(nil); err != nil {
		t.Fatalf("fail(nil) = %v, want nil", err)
	}

	dec = &Decoder{}
	if err := dec.fail(io.EOF); err != io.EOF {
		t.Fatalf("fail(io.EOF) = %v, want io.EOF", err)
	}
	if dec.err != io.EOF {
		t.Fatalf("decoder err = %v, want io.EOF", dec.err)
	}

	dec = &Decoder{}
	syntax := &SyntaxError{Err: errInvalidToken}
	if err := dec.fail(syntax); err != syntax {
		t.Fatalf("fail(SyntaxError) = %v, want %v", err, syntax)
	}
	if dec.err != syntax {
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

func TestIsValidXMLCharRanges(t *testing.T) {
	tests := []struct {
		r     rune
		valid bool
	}{
		{0x9, true},
		{0xA, true},
		{0xD, true},
		{0x20, true},
		{0xD7FF, true},
		{0xE000, true},
		{0xFFFD, true},
		{0x10000, true},
		{0x10FFFF, true},
		{0x0, false},
		{0xD800, false},
		{0x110000, false},
	}
	for _, tt := range tests {
		if got := isValidXMLChar(tt.r); got != tt.valid {
			t.Fatalf("isValidXMLChar(%#x) = %v, want %v", tt.r, got, tt.valid)
		}
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
	rawSpan, valueSpan, needs, rawNeeds, err = dec.scanAttrValue('"', false)
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
	tok := &Token{Text: makeSpan(&buf, 0, len(buf.data))}
	ok, err = dec.isWhitespaceCharData(tok)
	if err != nil || !ok {
		t.Fatalf("isWhitespaceCharData whitespace = %v/%v, want true/nil", ok, err)
	}

	buf = spanBuffer{data: []byte("&#x20;")}
	tok = &Token{Text: makeSpan(&buf, 0, len(buf.data)), TextNeeds: true}
	ok, err = dec.isWhitespaceCharData(tok)
	if err != nil || !ok {
		t.Fatalf("isWhitespaceCharData entity = %v/%v, want true/nil", ok, err)
	}

	buf = spanBuffer{data: []byte("&bogus;")}
	tok = &Token{Text: makeSpan(&buf, 0, len(buf.data)), TextNeeds: true}
	if _, err := dec.isWhitespaceCharData(tok); !errors.Is(err, errInvalidEntity) {
		t.Fatalf("isWhitespaceCharData error = %v, want %v", err, errInvalidEntity)
	}
}

func TestValidateXMLTextExtras(t *testing.T) {
	if err := validateXMLText([]byte("x&#x20;y"), nil); err != nil {
		t.Fatalf("validateXMLText numeric error = %v", err)
	}
	if err := validateXMLText([]byte("\u20ac"), nil); err != nil {
		t.Fatalf("validateXMLText unicode error = %v", err)
	}
}

func TestPeekKindBranches(t *testing.T) {
	dec := &Decoder{pendingTokenValid: true, pendingToken: Token{Kind: KindPI}}
	if got := dec.PeekKind(); got != KindPI {
		t.Fatalf("PeekKind pending = %v, want %v", got, KindPI)
	}

	dec = &Decoder{pendingEnd: true}
	if got := dec.PeekKind(); got != KindEndElement {
		t.Fatalf("PeekKind pending end = %v, want %v", got, KindEndElement)
	}

	dec = &Decoder{err: errInvalidToken}
	if got := dec.PeekKind(); got != KindNone {
		t.Fatalf("PeekKind error = %v, want %v", got, KindNone)
	}

	dec = NewDecoder(strings.NewReader("<root><![CDATA[x]]></root>"), CoalesceCharData(true))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	if got := dec.PeekKind(); got != KindCharData {
		t.Fatalf("PeekKind CDATA = %v, want %v", got, KindCharData)
	}
}

func TestPeekSkipDirective(t *testing.T) {
	input := "<!DOCTYPE root [<!ENTITY x '>'> [ignored] \"x\"]><root/>"
	dec := NewDecoder(strings.NewReader(input))
	pos, err := dec.peekSkipDirective(0)
	if err != nil {
		t.Fatalf("peekSkipDirective error = %v", err)
	}
	want := strings.Index(input, "<root/>")
	if pos != want {
		t.Fatalf("peekSkipDirective pos = %d, want %d", pos, want)
	}
}

func TestPeekKindTokens(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<!--c--><root/>"), EmitComments(true))
	if got := dec.PeekKind(); got != KindComment {
		t.Fatalf("PeekKind comment = %v, want %v", got, KindComment)
	}

	dec = NewDecoder(strings.NewReader("<?pi?><root/>"), EmitPI(true))
	if got := dec.PeekKind(); got != KindPI {
		t.Fatalf("PeekKind PI = %v, want %v", got, KindPI)
	}

	dec = NewDecoder(strings.NewReader("<!DOCTYPE root><root/>"), EmitDirectives(true))
	if got := dec.PeekKind(); got != KindDirective {
		t.Fatalf("PeekKind directive = %v, want %v", got, KindDirective)
	}

	dec = NewDecoder(strings.NewReader("<![CDATA[x]]><root/>"))
	if got := dec.PeekKind(); got != KindCDATA {
		t.Fatalf("PeekKind CDATA = %v, want %v", got, KindCDATA)
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

func TestInterningHelpers(t *testing.T) {
	interner := newNameInterner(2)
	if got := interner.internBytes(nil, -1); got.Full.buf != nil {
		t.Fatalf("internBytes empty = %v, want zero", got)
	}
	first := interner.intern([]byte("name"))
	second := interner.intern([]byte("name"))
	if first.Full.buf == nil || second.Full.buf == nil {
		t.Fatalf("interned spans missing buffers")
	}
	if interner.stats.Hits == 0 {
		t.Fatalf("intern hits = 0, want > 0")
	}

	for i := 0; i < nameInternerRecentSize+1; i++ {
		name := []byte{byte('a' + i)}
		_ = interner.intern(name)
	}

	limit := &nameInterner{maxEntries: -1}
	_ = limit.internBytesHash([]byte("x"), -1, hashBytes([]byte("x")))
	if limit.maxEntries != 0 {
		t.Fatalf("maxEntries = %d, want 0", limit.maxEntries)
	}
	limit.maxEntries = 1
	_ = limit.internBytesHash([]byte("a"), -1, hashBytes([]byte("a")))
	_ = limit.internBytesHash([]byte("b"), -1, hashBytes([]byte("b")))
	if limit.stats.Count != 1 {
		t.Fatalf("intern count = %d, want 1", limit.stats.Count)
	}
}

func TestDecoderInternQNameHelpers(t *testing.T) {
	buf := spanBuffer{data: []byte("ns:local")}
	name := makeQNameSpan(&buf, 0, len(buf.data), 2)

	dec := &Decoder{}
	interned := dec.internQName(name)
	if got := string(interned.Full.bytes()); got != "ns:local" {
		t.Fatalf("internQName full = %q, want ns:local", got)
	}
	if got := string(interned.Local.bytes()); got != "local" {
		t.Fatalf("internQName local = %q, want local", got)
	}

	hash := hashBytes(buf.data)
	dec = &Decoder{}
	interned = dec.internQNameHash(name, hash)
	if got := string(interned.Prefix.bytes()); got != "ns" {
		t.Fatalf("internQNameHash prefix = %q, want ns", got)
	}
}

func TestXMLDeclEncodingHelpers(t *testing.T) {
	decl := []byte(`<?xml version="1.0" encoding="ISO-8859-1"?>`)
	if got := parseXMLDeclEncoding(decl); got != "ISO-8859-1" {
		t.Fatalf("parseXMLDeclEncoding = %q, want ISO-8859-1", got)
	}
	if got := parseXMLDeclEncoding([]byte(`<?xml version="1.0"?>`)); got != "" {
		t.Fatalf("parseXMLDeclEncoding = %q, want empty", got)
	}
	if got := parseXMLDeclEncoding([]byte(`<?xml version="1.0" encoding=UTF-8?>`)); got != "" {
		t.Fatalf("parseXMLDeclEncoding = %q, want empty", got)
	}

	bufReader := bufio.NewReader(strings.NewReader(`<?xml version="1.0" encoding="ISO-8859-1"?><root/>`))
	label, err := detectXMLDeclEncoding(bufReader)
	if err != nil {
		t.Fatalf("detectXMLDeclEncoding error = %v", err)
	}
	if label != "ISO-8859-1" {
		t.Fatalf("detectXMLDeclEncoding label = %q, want ISO-8859-1", label)
	}

	bufReader = bufio.NewReader(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?><root/>`))
	label, err = detectXMLDeclEncoding(bufReader)
	if err != nil {
		t.Fatalf("detectXMLDeclEncoding UTF-8 error = %v", err)
	}
	if label != "" {
		t.Fatalf("detectXMLDeclEncoding UTF-8 label = %q, want empty", label)
	}

	name, rest := scanXMLDeclName([]byte("version=\"1.0\""))
	if string(name) != "version" {
		t.Fatalf("scanXMLDeclName name = %q, want version", name)
	}
	if len(rest) == 0 || rest[0] != '=' {
		t.Fatalf("scanXMLDeclName rest = %q, want prefix '='", rest)
	}
	name, _ = scanXMLDeclName([]byte("1bad"))
	if len(name) != 0 {
		t.Fatalf("scanXMLDeclName invalid = %q, want empty", name)
	}

	input := `<?xml version="1.0" encoding="ISO-8859-1"?><root/>`
	bufioReader := bufio.NewReader(strings.NewReader(input))
	called := false
	wrapped, err := wrapCharsetReaderFromBufio(bufioReader, func(label string, r io.Reader) (io.Reader, error) {
		called = true
		if label != "ISO-8859-1" {
			t.Fatalf("charset label = %q, want ISO-8859-1", label)
		}
		return r, nil
	})
	if err != nil {
		t.Fatalf("wrapCharsetReaderFromBufio error = %v", err)
	}
	if !called {
		t.Fatalf("charset reader not called")
	}
	out, err := io.ReadAll(wrapped)
	if err != nil {
		t.Fatalf("ReadAll error = %v", err)
	}
	if !bytes.HasPrefix(out, []byte("<?xml")) {
		t.Fatalf("ReadAll prefix = %q, want xml decl", out)
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

func TestPopStackInterned(t *testing.T) {
	dec := &Decoder{}
	if err := dec.popStackInterned(QNameSpan{}); !errors.Is(err, errMismatchedEndTag) {
		t.Fatalf("popStackInterned empty error = %v, want %v", err, errMismatchedEndTag)
	}
	buf := spanBuffer{data: []byte("root")}
	name := newQNameSpan(&buf, 0, len(buf.data))
	dec.stack = []stackEntry{{StackEntry: StackEntry{Name: name}}}
	otherBuf := spanBuffer{data: []byte("other")}
	other := newQNameSpan(&otherBuf, 0, len(otherBuf.data))
	if err := dec.popStackInterned(other); !errors.Is(err, errMismatchedEndTag) {
		t.Fatalf("popStackInterned mismatch error = %v, want %v", err, errMismatchedEndTag)
	}
}

func TestScanName(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("name rest")}}
	span, err := dec.scanName(false)
	if err != nil {
		t.Fatalf("scanName error = %v", err)
	}
	if got := string(span.bytes()); got != "name" {
		t.Fatalf("scanName = %q, want name", got)
	}

	dec = &Decoder{buf: spanBuffer{data: []byte("1bad")}}
	if _, err := dec.scanName(false); !errors.Is(err, errInvalidName) {
		t.Fatalf("scanName invalid error = %v, want %v", err, errInvalidName)
	}

	dec = &Decoder{buf: spanBuffer{data: []byte("\u00e9x ")}, eof: true}
	span, err = dec.scanName(false)
	if err != nil {
		t.Fatalf("scanName unicode error = %v", err)
	}
	if got := string(span.bytes()); got != "\u00e9x" {
		t.Fatalf("scanName unicode = %q, want \\u00e9x", got)
	}
}

func TestHasAttrExpansion(t *testing.T) {
	tok := Token{AttrRawNeeds: []bool{false, false}}
	if hasAttrExpansion(tok) {
		t.Fatalf("hasAttrExpansion = true, want false")
	}
	tok.AttrRawNeeds[1] = true
	if !hasAttrExpansion(tok) {
		t.Fatalf("hasAttrExpansion = false, want true")
	}
}

func TestSpanStringEmpty(t *testing.T) {
	dec := &Decoder{}
	if got := dec.SpanString(Span{}); got != "" {
		t.Fatalf("SpanString = %q, want empty", got)
	}
}

func TestSkipValueExtraBranches(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root/>"))
	dec.err = errInvalidToken
	if err := dec.SkipValue(); !errors.Is(err, errInvalidToken) {
		t.Fatalf("SkipValue error = %v, want %v", err, errInvalidToken)
	}

	dec = NewDecoder(strings.NewReader(""))
	if err := dec.SkipValue(); !errors.Is(err, errMissingRoot) {
		t.Fatalf("SkipValue empty error = %v, want %v", err, errMissingRoot)
	}

	dec = NewDecoder(strings.NewReader("<!--c-->"), EmitComments(true))
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue comment error = %v, want nil", err)
	}

	dec = NewDecoder(strings.NewReader("<root><child/></root>"))
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue root error = %v", err)
	}
	if _, err := dec.ReadToken(); err != io.EOF {
		t.Fatalf("ReadToken after SkipValue = %v, want io.EOF", err)
	}
}

func TestPeekRuneReadMore(t *testing.T) {
	dec := NewDecoder(strings.NewReader("\u20ac"), BufferSize(1))
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
	dec := NewDecoder(reader, BufferSize(1))
	if _, _, err := dec.peekRune(false); !errors.Is(err, errInvalidChar) {
		t.Fatalf("peekRune error = %v, want %v", err, errInvalidChar)
	}
}

func TestWrapCharsetReaderFromBufioError(t *testing.T) {
	sentinel := errors.New("boom")
	reader := bufio.NewReader(errReader{err: sentinel})
	if _, err := wrapCharsetReaderFromBufio(reader, nil); !errors.Is(err, sentinel) {
		t.Fatalf("wrapCharsetReaderFromBufio error = %v, want %v", err, sentinel)
	}
}

func TestWrapCharsetReaderFromBufioErrors(t *testing.T) {
	input := `<?xml version="1.0" encoding="ISO-8859-1"?><root/>`
	reader := bufio.NewReader(strings.NewReader(input))
	sentinel := errors.New("decode")
	if _, err := wrapCharsetReaderFromBufio(reader, func(label string, r io.Reader) (io.Reader, error) {
		return nil, sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("wrapCharsetReaderFromBufio decode error = %v, want %v", err, sentinel)
	}

	reader = bufio.NewReader(strings.NewReader(input))
	if _, err := wrapCharsetReaderFromBufio(reader, func(label string, r io.Reader) (io.Reader, error) {
		return nil, nil
	}); !errors.Is(err, errUnsupportedEncoding) {
		t.Fatalf("wrapCharsetReaderFromBufio nil error = %v, want %v", err, errUnsupportedEncoding)
	}
}

func TestDetectEncodingBOM(t *testing.T) {
	reader := bufio.NewReader(bytes.NewReader([]byte{0xFF, 0xFE, 0x00, 0x00}))
	label, err := detectEncoding(reader)
	if err != nil {
		t.Fatalf("detectEncoding error = %v", err)
	}
	if label != "utf-16" {
		t.Fatalf("detectEncoding label = %q, want utf-16", label)
	}
}

func TestDetectXMLDeclEncodingNoPrefix(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("<root/>"))
	label, err := detectXMLDeclEncoding(reader)
	if err != nil {
		t.Fatalf("detectXMLDeclEncoding error = %v", err)
	}
	if label != "" {
		t.Fatalf("detectXMLDeclEncoding label = %q, want empty", label)
	}
}

func TestParseEntityRefErrors(t *testing.T) {
	if _, _, _, _, err := parseEntityRef([]byte("&"), 0, nil); !errors.Is(err, errInvalidEntity) {
		t.Fatalf("parseEntityRef short error = %v, want %v", err, errInvalidEntity)
	}
	if _, _, _, _, err := parseEntityRef([]byte("&x"), 0, nil); !errors.Is(err, errInvalidEntity) {
		t.Fatalf("parseEntityRef no-semi error = %v, want %v", err, errInvalidEntity)
	}
	if _, _, _, _, err := parseEntityRef([]byte("&;"), 0, nil); !errors.Is(err, errInvalidEntity) {
		t.Fatalf("parseEntityRef empty error = %v, want %v", err, errInvalidEntity)
	}

	resolver := &entityResolver{custom: map[string]string{"ok": "v", "bad": "\x00"}}
	consumed, replacement, _, isNumeric, err := parseEntityRef([]byte("&ok;"), 0, resolver)
	if err != nil {
		t.Fatalf("parseEntityRef custom error = %v", err)
	}
	if consumed == 0 || isNumeric || replacement != "v" {
		t.Fatalf("parseEntityRef custom = %d/%v/%q, want consumed and v", consumed, isNumeric, replacement)
	}
	if _, _, _, _, err := parseEntityRef([]byte("&bad;"), 0, resolver); !errors.Is(err, errInvalidChar) {
		t.Fatalf("parseEntityRef bad error = %v, want %v", err, errInvalidChar)
	}

	consumed, _, r, isNumeric, err := parseEntityRef([]byte("&#x41;"), 0, nil)
	if err != nil {
		t.Fatalf("parseEntityRef numeric error = %v", err)
	}
	if !isNumeric || r != 'A' || consumed != len("&#x41;") {
		t.Fatalf("parseEntityRef numeric = %d/%v/%q, want A", consumed, isNumeric, r)
	}
}

func TestParseNumericEntityErrors(t *testing.T) {
	tests := [][]byte{
		[]byte("#"),
		[]byte("#x"),
		[]byte("#xG"),
		[]byte("#-1"),
	}
	for _, tt := range tests {
		if _, err := parseNumericEntity(tt); !errors.Is(err, errInvalidCharRef) {
			t.Fatalf("parseNumericEntity(%q) = %v, want %v", tt, err, errInvalidCharRef)
		}
	}
}

func TestSyntaxErrorNil(t *testing.T) {
	var err *SyntaxError
	if got := err.Error(); got != "<nil>" {
		t.Fatalf("SyntaxError.Error = %q, want <nil>", got)
	}
}

func TestValueCloneEmpty(t *testing.T) {
	var value Value
	if value.Clone() != nil {
		t.Fatalf("Value.Clone = %v, want nil", value.Clone())
	}
}

func TestWrapCharsetReaderBufio(t *testing.T) {
	bufReader := bufio.NewReader(strings.NewReader("<root/>"))
	reader, err := wrapCharsetReader(bufReader, nil, 4)
	if err != nil {
		t.Fatalf("wrapCharsetReader error = %v", err)
	}
	if reader != bufReader {
		t.Fatalf("wrapCharsetReader reader = %T, want *bufio.Reader", reader)
	}
}

func TestScanPIInto(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<?pi?><root/>"), EmitPI(true))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken PI error = %v", err)
	}
	if tok.Kind != KindPI {
		t.Fatalf("PI kind = %v, want %v", tok.Kind, KindPI)
	}
	if got := string(dec.SpanBytes(tok.Text)); got != "pi" {
		t.Fatalf("PI text = %q, want pi", got)
	}

	dec = NewDecoder(strings.NewReader("<?pi test?><root/>"), EmitPI(true))
	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken PI data error = %v", err)
	}
	if tok.Kind != KindPI {
		t.Fatalf("PI data kind = %v, want %v", tok.Kind, KindPI)
	}

	dec = NewDecoder(strings.NewReader("<?xml?>"), EmitPI(true))
	if _, err := dec.ReadToken(); !errors.Is(err, errInvalidPI) {
		t.Fatalf("ReadToken xml PI error = %v, want %v", err, errInvalidPI)
	}
}

func TestScanDirectiveInto(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<!DOCTYPE root><root/>"), EmitDirectives(true))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken directive error = %v", err)
	}
	if tok.Kind != KindDirective {
		t.Fatalf("directive kind = %v, want %v", tok.Kind, KindDirective)
	}
}

func TestScanNameUnexpectedEOF(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: nil}, eof: true}
	if _, err := dec.scanName(false); !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("scanName error = %v, want %v", err, errUnexpectedEOF)
	}
}

type errReader struct {
	err error
}

func (r errReader) Read(p []byte) (int, error) {
	return 0, r.err
}

func newAttrValueDecoder(data []byte, opts ...Options) *Decoder {
	dec := NewDecoder(strings.NewReader(""), opts...)
	dec.buf.data = data
	dec.pos = 0
	dec.eof = true
	return dec
}

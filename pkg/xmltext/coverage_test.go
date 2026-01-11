package xmltext

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDecoderUtilities(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>\n<child attr=\"v\"/></root>"), ResolveEntities(true))
	if value, ok := GetOption(dec.Options(), ResolveEntities); !ok || !value {
		t.Fatalf("Options ResolveEntities = %v, want true", value)
	}

	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	if tok.Line() != 1 || tok.Column() != 1 {
		t.Fatalf("root line/column = %d/%d, want 1/1", tok.Line(), tok.Column())
	}
	if got := string(dec.UnreadBuffer()); !strings.HasPrefix(got, "\n<child") {
		t.Fatalf("UnreadBuffer = %q, want prefix \\n<child", got)
	}
	if dec.InputOffset() != int64(len("<root>")) {
		t.Fatalf("InputOffset = %d, want %d", dec.InputOffset(), len("<root>"))
	}
	if tok.Clone().Kind() != tok.Kind() {
		t.Fatalf("Clone kind = %v, want %v", tok.Clone().Kind(), tok.Kind())
	}

	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken child error = %v", err)
	}
	if tok.Kind() == KindCharData {
		tok, err = dec.ReadToken()
		if err != nil {
			t.Fatalf("ReadToken child start error = %v", err)
		}
	}
	if tok.Kind() != KindStartElement {
		t.Fatalf("child kind = %v, want %v", tok.Kind(), KindStartElement)
	}
	if tok.Line() != 2 || tok.Column() != 1 {
		t.Fatalf("child line/column = %d/%d, want 2/1", tok.Line(), tok.Column())
	}
	if got := dec.StackIndex(0); string(dec.SpanBytes(got.Name.Local)) != "root" {
		t.Fatalf("StackIndex(0) name = %q, want root", dec.SpanBytes(got.Name.Local))
	}
	if got := dec.StackIndex(1); string(dec.SpanBytes(got.Name.Local)) != "child" {
		t.Fatalf("StackIndex(1) name = %q, want child", dec.SpanBytes(got.Name.Local))
	}
	if got := dec.StackIndex(2); got.Name.Full.buf != nil {
		t.Fatalf("StackIndex(2) = %v, want zero value", got)
	}
	if got := dec.StackPointer(); got != "/root[1]/child[1]" {
		t.Fatalf("StackPointer = %q, want /root[1]/child[1]", got)
	}

	dst := []AttrSpan{{}}
	attrs := tok.AttrsInto(dst)
	if len(attrs) != 2 {
		t.Fatalf("AttrsInto len = %d, want 2", len(attrs))
	}

	for {
		_, err := dec.ReadToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadToken error = %v", err)
		}
	}
	if dec.UnreadBuffer() != nil {
		t.Fatalf("UnreadBuffer at EOF = %v, want nil", dec.UnreadBuffer())
	}
}

func TestDebugPoisonSpans(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root><child/></root>"), DebugPoisonSpans(true))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	if dec.SpanBytes(tok.Name().Local) == nil {
		t.Fatalf("SpanBytes returned nil for current token")
	}
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken child error = %v", err)
	}
	if tok.raw.bytes() != nil {
		t.Fatalf("SpanBytes returned data for stale token")
	}
}

func TestValueHelpers(t *testing.T) {
	value := Value("<a><b/></a>")
	if !value.IsValid() {
		t.Fatalf("Value.IsValid = false, want true")
	}
	invalid := Value("<a>")
	if invalid.IsValid() {
		t.Fatalf("Value.IsValid = true, want false")
	}
	if got := value.Clone(); string(got) != string(value) {
		t.Fatalf("Clone = %q, want %q", got, value)
	}
}

func TestSpanHelpers(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root attr="a&amp;b">x&amp;y</root>`))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken error = %v", err)
	}
	attr := tok.Attrs()[0]
	if got := string(CopyAttrValue(nil, attr)); got != "a&amp;b" {
		t.Fatalf("CopyAttrValue = %q, want a&amp;b", got)
	}
	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if got := string(CopySpan(nil, tok.TextSpan())); got != "x&amp;y" {
		t.Fatalf("CopySpan = %q, want x&amp;y", got)
	}
	unescaped, err := UnescapeInto(nil, tok.TextSpan())
	if err != nil {
		t.Fatalf("UnescapeInto error = %v", err)
	}
	if string(unescaped) != "x&y" {
		t.Fatalf("UnescapeInto = %q, want x&y", unescaped)
	}
}

func TestSyntaxErrorFormatting(t *testing.T) {
	syntax := &SyntaxError{Line: 2, Column: 3, Err: errInvalidToken}
	if got := syntax.Error(); !strings.Contains(got, "line 2") {
		t.Fatalf("Error = %q, want line 2", got)
	}
	syntax = &SyntaxError{Offset: 10, Err: errInvalidToken}
	if got := syntax.Error(); !strings.Contains(got, "offset 10") {
		t.Fatalf("Error = %q, want offset 10", got)
	}
}

func TestEntityParsing(t *testing.T) {
	if _, err := parseNumericEntity([]byte("#9")); err != nil {
		t.Fatalf("parseNumericEntity decimal error = %v", err)
	}
	if _, err := parseNumericEntity([]byte("#xA")); err != nil {
		t.Fatalf("parseNumericEntity hex error = %v", err)
	}
	if _, err := parseNumericEntity([]byte("#x110000")); err == nil {
		t.Fatalf("expected error for out of range value")
	}
}

func TestInternerSetMax(t *testing.T) {
	interner := newNameInterner(0)
	interner.setMax(1)
	_ = interner.intern([]byte("a"))
	_ = interner.intern([]byte("b"))
	if interner.stats.Count != 1 {
		t.Fatalf("intern count = %d, want 1", interner.stats.Count)
	}
}

func TestKindString(t *testing.T) {
	if KindStartElement.String() != "StartElement" {
		t.Fatalf("KindStartElement = %q, want StartElement", KindStartElement.String())
	}
	if Kind(99).String() != "Unknown" {
		t.Fatalf("Kind(99) = %q, want Unknown", Kind(99).String())
	}
}

func TestPathXPath(t *testing.T) {
	buf := spanBuffer{data: []byte("p:root")}
	name := newQNameSpan(&buf, 0, len(buf.data))
	path := Path{{Name: name, Index: 1}}
	if got := path.XPath(); got != "/p:root[1]" {
		t.Fatalf("XPath = %q, want /p:root[1]", got)
	}
}

func TestPeekSkips(t *testing.T) {
	input := `<!--c--><?pi data?><!DOCTYPE root><root/>`
	dec := NewDecoder(strings.NewReader(input))
	if got := dec.PeekKind(); got != KindStartElement {
		t.Fatalf("PeekKind = %v, want %v", got, KindStartElement)
	}
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken start error = %v", err)
	}
	if got := dec.PeekKind(); got != KindEndElement {
		t.Fatalf("PeekKind after start = %v, want %v", got, KindEndElement)
	}
}

func TestDirectiveToken(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<!DOCTYPE root [<!ELEMENT root EMPTY>]><root/>`), EmitDirectives(true))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken directive error = %v", err)
	}
	if tok.Kind() != KindDirective {
		t.Fatalf("directive kind = %v, want %v", tok.Kind(), KindDirective)
	}
	if got := string(dec.SpanBytes(tok.TextSpan())); !strings.HasPrefix(got, "DOCTYPE root") {
		t.Fatalf("directive text = %q, want DOCTYPE root...", got)
	}
}

func TestCharsetReader(t *testing.T) {
	reader, err := wrapCharsetReader(strings.NewReader("\ufeff<root/>"), nil)
	if err != nil {
		t.Fatalf("wrapCharsetReader BOM error = %v", err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll error = %v", err)
	}
	if string(out) != "<root/>" {
		t.Fatalf("ReadAll = %q, want <root/>", out)
	}

	_, err = wrapCharsetReader(bytes.NewReader([]byte{0xFE, 0xFF, 0x00, 0x3C}), nil)
	if !errors.Is(err, errUnsupportedEncoding) {
		t.Fatalf("wrapCharsetReader error = %v, want %v", err, errUnsupportedEncoding)
	}

	called := false
	reader, err = wrapCharsetReader(bytes.NewReader([]byte{0xFE, 0xFF, 0x00, 0x3C}), func(label string, r io.Reader) (io.Reader, error) {
		called = true
		if label == "" {
			t.Fatalf("label is empty")
		}
		return strings.NewReader("<root/>"), nil
	})
	if err != nil {
		t.Fatalf("wrapCharsetReader custom error = %v", err)
	}
	if !called {
		t.Fatalf("charset reader not called")
	}
	out, err = io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll custom error = %v", err)
	}
	if string(out) != "<root/>" {
		t.Fatalf("ReadAll custom = %q, want <root/>", out)
	}
}

func TestDetectEncoding(t *testing.T) {
	reader := bufio.NewReader(bytes.NewReader([]byte{0x00, 0x3C, 0x00, 0x3F}))
	label, err := detectEncoding(reader)
	if err != nil {
		t.Fatalf("detectEncoding error = %v", err)
	}
	if label != "utf-16be" {
		t.Fatalf("label = %q, want utf-16be", label)
	}
}

func TestDecoderSkipValueBranches(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root/>"))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	if tok.Kind() != KindStartElement {
		t.Fatalf("root kind = %v, want %v", tok.Kind(), KindStartElement)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue self-closing error = %v", err)
	}
	if _, err := dec.ReadToken(); err != io.EOF {
		t.Fatalf("ReadToken EOF = %v, want io.EOF", err)
	}
	if err := dec.SkipValue(); err != io.EOF {
		t.Fatalf("SkipValue EOF = %v, want io.EOF", err)
	}

	dec = NewDecoder(strings.NewReader("<root><child/></root>"))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root start error = %v", err)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue element error = %v", err)
	}
	if _, err := dec.ReadToken(); err != io.EOF {
		t.Fatalf("ReadToken EOF after skip = %v, want io.EOF", err)
	}

	dec = NewDecoder(strings.NewReader("<root>text</root>"))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root start error = %v", err)
	}
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue after text error = %v", err)
	}
	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken end error = %v", err)
	}
	if tok.Kind() != KindEndElement {
		t.Fatalf("end kind = %v, want %v", tok.Kind(), KindEndElement)
	}

	dec = NewDecoder(strings.NewReader("<root><!--a--><!--b--></root>"), EmitComments(true))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root start error = %v", err)
	}
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken comment error = %v", err)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue comment error = %v", err)
	}
	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken end error = %v", err)
	}
	if tok.Kind() != KindEndElement {
		t.Fatalf("end kind = %v, want %v", tok.Kind(), KindEndElement)
	}
}

func TestReadValueVariants(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>x&amp;y</root>"))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	value, err := dec.ReadValue()
	if err != nil {
		t.Fatalf("ReadValue raw error = %v", err)
	}
	if string(value) != "x&amp;y" {
		t.Fatalf("ReadValue raw = %q, want x&amp;y", value)
	}

	dec = NewDecoder(strings.NewReader("<root>x&amp;y</root>"), ResolveEntities(true))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root resolve error = %v", err)
	}
	value, err = dec.ReadValue()
	if err != nil {
		t.Fatalf("ReadValue resolve error = %v", err)
	}
	if string(value) != "x&y" {
		t.Fatalf("ReadValue resolve = %q, want x&y", value)
	}

	dec = NewDecoder(strings.NewReader("<root><![CDATA[x]]></root>"), ResolveEntities(true))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root CDATA error = %v", err)
	}
	value, err = dec.ReadValue()
	if err != nil {
		t.Fatalf("ReadValue CDATA error = %v", err)
	}
	if string(value) != "<![CDATA[x]]>" {
		t.Fatalf("ReadValue CDATA = %q, want <![CDATA[x]]>", value)
	}

	dec = NewDecoder(strings.NewReader("<!--c--><root/>"), EmitComments(true))
	value, err = dec.ReadValue()
	if err != nil {
		t.Fatalf("ReadValue comment error = %v", err)
	}
	if string(value) != "<!--c-->" {
		t.Fatalf("ReadValue comment = %q, want <!--c-->", value)
	}
}

func TestReadValueLateExpansion(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root><a>ok</a><b>1 &amp; 2</b></root>"), ResolveEntities(true))
	value, err := dec.ReadValue()
	if err != nil {
		t.Fatalf("ReadValue error = %v", err)
	}
	if string(value) != "<root><a>ok</a><b>1 & 2</b></root>" {
		t.Fatalf("ReadValue = %q, want expanded subtree", value)
	}
}

func TestReadValueNoExpansion(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root><a>ok</a></root>"), ResolveEntities(true))
	value, err := dec.ReadValue()
	if err != nil {
		t.Fatalf("ReadValue error = %v", err)
	}
	if string(value) != "<root><a>ok</a></root>" {
		t.Fatalf("ReadValue = %q, want raw subtree", value)
	}
}

func TestCharDataInvalidSequence(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>]]></root>"))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	_, err := dec.ReadToken()
	if !errors.Is(err, errInvalidToken) {
		t.Fatalf("char data error = %v, want %v", err, errInvalidToken)
	}
}

func TestWhitespaceEntityOutsideRoot(t *testing.T) {
	dec := NewDecoder(strings.NewReader("&#x20;<root/>"))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken whitespace error = %v", err)
	}
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	if tok.Kind() != KindStartElement {
		t.Fatalf("root kind = %v, want %v", tok.Kind(), KindStartElement)
	}
}

func TestCharDataEOF(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>text"))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if tok.Kind() != KindCharData {
		t.Fatalf("text kind = %v, want %v", tok.Kind(), KindCharData)
	}
	_, err = dec.ReadToken()
	if !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("ReadToken EOF error = %v, want %v", err, errUnexpectedEOF)
	}
}

func TestCharDataMaxTokenSize(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>abcdefgh</root>"), MaxTokenSize(6))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	_, err := dec.ReadToken()
	if !errors.Is(err, errTokenTooLarge) {
		t.Fatalf("ReadToken error = %v, want %v", err, errTokenTooLarge)
	}
}

type chunkReader struct {
	data []byte
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	p[0] = r.data[0]
	r.data = r.data[1:]
	return 1, nil
}

func TestUnicodeNameWithChunkReader(t *testing.T) {
	reader := &chunkReader{data: []byte("<\u00e9/>")}
	dec := NewDecoder(reader)
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken error = %v", err)
	}
	if got := string(dec.SpanBytes(tok.Name().Local)); got != "\u00e9" {
		t.Fatalf("name = %q, want \u00e9", got)
	}
}

func TestUnterminatedPI(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<?pi"))
	_, err := dec.ReadToken()
	if !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("ReadToken error = %v, want %v", err, errUnexpectedEOF)
	}
}

func TestUnterminatedComment(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<!--oops"))
	_, err := dec.ReadToken()
	if !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("ReadToken error = %v, want %v", err, errUnexpectedEOF)
	}
}

func TestPIWithoutData(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<?pi?><root/>"), EmitPI(true))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken PI error = %v", err)
	}
	if tok.Kind() != KindPI {
		t.Fatalf("PI kind = %v, want %v", tok.Kind(), KindPI)
	}
	if got := string(dec.SpanBytes(tok.TextSpan())); got != "pi" {
		t.Fatalf("PI text = %q, want pi", got)
	}
}

func TestInvalidQNames(t *testing.T) {
	tests := []string{
		"<:a/>",
		"<a:/>",
		"<a:b:c/>",
	}
	for _, input := range tests {
		dec := NewDecoder(strings.NewReader(input))
		_, err := dec.ReadToken()
		if !errors.Is(err, errInvalidName) {
			t.Fatalf("ReadToken(%q) error = %v, want %v", input, err, errInvalidName)
		}
	}
}

func TestPeekSkipDirectiveQuotes(t *testing.T) {
	input := `<!DOCTYPE root [<!ENTITY x "y">]><root/>`
	dec := NewDecoder(strings.NewReader(input))
	if got := dec.PeekKind(); got != KindStartElement {
		t.Fatalf("PeekKind = %v, want %v", got, KindStartElement)
	}
}

func TestInvalidUTF8Name(t *testing.T) {
	reader := &chunkReader{data: []byte("<\xc3/>")}
	dec := NewDecoder(reader)
	_, err := dec.ReadToken()
	if !errors.Is(err, errInvalidChar) {
		t.Fatalf("ReadToken error = %v, want %v", err, errInvalidChar)
	}
}

func TestEntityMapCopy(t *testing.T) {
	values := map[string]string{"foo": "bar"}
	opts := WithEntityMap(values)
	values["foo"] = "baz"
	dec := NewDecoder(strings.NewReader("<root>&foo;</root>"), ResolveEntities(true), opts)
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if got := string(dec.SpanBytes(tok.TextSpan())); got != "bar" {
		t.Fatalf("entity value = %q, want bar", got)
	}
}

func TestXMLCharValidation(t *testing.T) {
	if !isValidXMLChar('\n') {
		t.Fatalf("isValidXMLChar(\\n) = false, want true")
	}
	if isValidXMLChar(0x01) {
		t.Fatalf("isValidXMLChar(0x01) = true, want false")
	}
	if err := validateXMLChars([]byte("ok")); err != nil {
		t.Fatalf("validateXMLChars error = %v", err)
	}
	if err := validateXMLText([]byte("a&amp;b"), &entityResolver{}); err != nil {
		t.Fatalf("validateXMLText error = %v", err)
	}
}

func TestEncodingXMLParity(t *testing.T) {
	cases := []string{
		`<root/>`,
		`<root attr="v">text</root>`,
		`<root attr="x&amp;y">a&lt;b<![CDATA[c]]>d</root>`,
		`<?pi data?><root><!--c--><child/></root>`,
		`<!DOCTYPE root><root/>`,
	}

	for _, input := range cases {
		tokens, err := readXMLTextTokens(input)
		if err != nil {
			t.Fatalf("readXMLTextTokens(%q) error = %v", input, err)
		}
		encTokens, err := readEncodingXMLTokens(input)
		if err != nil {
			t.Fatalf("readEncodingXMLTokens(%q) error = %v", input, err)
		}
		if !tokensEqual(tokens, encTokens) {
			t.Fatalf("tokens mismatch for %q:\nxmltext=%v\nencoding=%v", input, tokens, encTokens)
		}
	}

	invalid := `<root><child></root>`
	if _, err := readXMLTextTokens(invalid); err == nil {
		t.Fatalf("expected xmltext error for mismatched tags")
	}
	if _, err := readEncodingXMLTokens(invalid); err == nil {
		t.Fatalf("expected encoding/xml error for mismatched tags")
	}
}

type simpleToken struct {
	kind  Kind
	name  string
	text  string
	attrs []simpleAttr
}

type simpleAttr struct {
	name  string
	value string
}

func readXMLTextTokens(input string) ([]simpleToken, error) {
	dec := NewDecoder(
		strings.NewReader(input),
		ResolveEntities(true),
		EmitComments(true),
		EmitPI(true),
		EmitDirectives(true),
		CoalesceCharData(true),
	)
	var tokens []simpleToken
	for {
		tok, err := dec.ReadToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, simplifyXMLTextToken(dec, tok))
	}
	return tokens, nil
}

func simplifyXMLTextToken(dec *Decoder, tok Token) simpleToken {
	kind := tok.Kind()
	if kind == KindCDATA {
		kind = KindCharData
	}
	out := simpleToken{kind: kind}
	switch kind {
	case KindStartElement, KindEndElement:
		out.name = string(dec.SpanBytes(tok.Name().Local))
	case KindCharData, KindComment, KindDirective:
		out.text = string(dec.SpanBytes(tok.TextSpan()))
	case KindPI:
		target, inst := splitPIText(tok.TextSpan().bytes())
		out.name = target
		out.text = string(inst)
	}
	if tok.Kind() == KindStartElement {
		for _, attr := range tok.Attrs() {
			out.attrs = append(out.attrs, simpleAttr{
				name:  string(dec.SpanBytes(attr.Name.Local)),
				value: string(dec.SpanBytes(attr.ValueSpan)),
			})
		}
	}
	return out
}

func readEncodingXMLTokens(input string) ([]simpleToken, error) {
	dec := xml.NewDecoder(strings.NewReader(input))
	var tokens []simpleToken
	var textBuf []byte
	flushText := func() {
		if len(textBuf) == 0 {
			return
		}
		tokens = append(tokens, simpleToken{kind: KindCharData, text: string(textBuf)})
		textBuf = textBuf[:0]
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := tok.(type) {
		case xml.StartElement:
			flushText()
			out := simpleToken{kind: KindStartElement, name: value.Name.Local}
			for _, attr := range value.Attr {
				out.attrs = append(out.attrs, simpleAttr{
					name:  attr.Name.Local,
					value: attr.Value,
				})
			}
			tokens = append(tokens, out)
		case xml.EndElement:
			flushText()
			tokens = append(tokens, simpleToken{kind: KindEndElement, name: value.Name.Local})
		case xml.CharData:
			textBuf = append(textBuf, value...)
		case xml.Comment:
			flushText()
			tokens = append(tokens, simpleToken{kind: KindComment, text: string(value)})
		case xml.ProcInst:
			flushText()
			tokens = append(tokens, simpleToken{
				kind: KindPI,
				name: value.Target,
				text: string(value.Inst),
			})
		case xml.Directive:
			flushText()
			tokens = append(tokens, simpleToken{kind: KindDirective, text: string(value)})
		}
	}
	flushText()
	return tokens, nil
}

func splitPIText(data []byte) (string, []byte) {
	for i := 0; i < len(data); i++ {
		if isWhitespace(data[i]) {
			target := string(data[:i])
			inst := bytes.TrimLeft(data[i:], " \t\r\n")
			return target, inst
		}
	}
	return string(data), nil
}

func tokensEqual(a, b []simpleToken) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].kind != b[i].kind || a[i].name != b[i].name || a[i].text != b[i].text {
			return false
		}
		if len(a[i].attrs) != len(b[i].attrs) {
			return false
		}
		for j := range a[i].attrs {
			if a[i].attrs[j] != b[i].attrs[j] {
				return false
			}
		}
	}
	return true
}

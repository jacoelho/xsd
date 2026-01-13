package xmltext

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

const testInputBody = "<!DOCTYPE html PUBLIC \"-//W3C//DTD XHTML 1.0 Transitional//EN\"\n" +
	"  \"http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd\">\n" +
	"<body xmlns:foo=\"ns1\" xmlns=\"ns2\" xmlns:tag=\"ns3\" " +
	"\r\n\t" + "  >\n" +
	"  <hello lang=\"en\">World &lt;&gt;&apos;&quot; &#x767d;&#40300;\u7fd4</hello>\n" +
	"  <query>&\u4f55; &is-it;</query>\n" +
	"  <goodbye />\n" +
	"  <outer foo:attr=\"value\" xmlns:tag=\"ns4\">\n" +
	"    <inner/>\n" +
	"  </outer>\n" +
	"  <tag:name>\n" +
	"    <![CDATA[Some text here.]]>\n" +
	"  </tag:name>\n" +
	"</body><!-- missing final newline -->"

const testInput = "\n<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" + testInputBody

const testInputTrimmed = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" + testInputBody

var testEntity = map[string]string{"\u4f55": "What", "is-it": "is it?"}

const testInputAltEncoding = "<?xml version=\"1.0\" encoding=\"x-testing-uppercase\"?>\n" +
	"<TAG>VALUE</TAG>"

const testInputAltEncodingWhitespace = "\n<?xml version=\"1.0\" encoding=\"x-testing-uppercase\"?>\n" +
	"<TAG>VALUE</TAG>"

const nestedDirectivesInput = `
<!DOCTYPE [<!ENTITY rdf "http://www.w3.org/1999/02/22-rdf-syntax-ns#">]>
<!DOCTYPE [<!ENTITY xlt ">">]>
<!DOCTYPE [<!ENTITY xlt "<">]>
<!DOCTYPE [<!ENTITY xlt '>'>]>
<!DOCTYPE [<!ENTITY xlt '<'>]>
<!DOCTYPE [<!ENTITY xlt '">'>]>
<!DOCTYPE [<!ENTITY xlt "'<">]>
`

const directivesWithCommentsInput = `
<!DOCTYPE [<!-- a comment --><!ENTITY rdf "http://www.w3.org/1999/02/22-rdf-syntax-ns#">]>
<!DOCTYPE [<!ENTITY go "Golang"><!-- a comment-->]>
<!DOCTYPE <!-> <!> <!----> <!-->--> <!--->--> [<!ENTITY go "Golang"><!-- a comment-->]>
`

const nonStrictInput = "\n<tag>non&entity</tag>\n" +
	"<tag>&unknown;entity</tag>\n" +
	"<tag>&#123</tag>\n" +
	"<tag>&#zzz;</tag>\n" +
	"<tag>&\u306a\u307e\u30483;</tag>\n" +
	"<tag>&lt-gt;</tag>\n" +
	"<tag>&;</tag>\n" +
	"<tag>&0a;</tag>\n"

var xmlInput = []string{
	"<",
	"<t",
	"<t ",
	"<t/",
	"<!",
	"<!-",
	"<!--",
	"<!--c-",
	"<!--c--",
	"<!d",
	"<t></",
	"<t></t",
	"<?",
	"<?p",
	"<t a",
	"<t a=",
	"<t a='",
	"<t a=''",
	"<t/><![",
	"<t/><![C",
	"<t/><![CDATA[d",
	"<t/><![CDATA[d]",
	"<t/><![CDATA[d]]",
	"<>",
	"<t/a",
	"<0 />",
	"<?0 >",
	"</0>",
	"<t 0=''>",
	"<t a='&'>",
	"<t a='<'>",
	"<t>&nbspc;</t>",
	"<t a>",
	"<t a=>",
	"<t a=v>",
	"<t></e>",
	"<t></>",
	"<t></t!",
	"<t>cdata]]></t>",
}

var characterTests = []struct {
	input string
}{
	{"\x12<doc/>"},
	{"<?xml version=\"1.0\"?>\x0b<doc/>"},
	{"\xef\xbf\xbe<doc/>"},
	{"<?xml version=\"1.0\"?><doc>\r\n<hiya/>\x07<toots/></doc>"},
	{"<?xml version=\"1.0\"?><doc \x12='value'>what's up</doc>"},
	{"<doc>&abc\x01;</doc>"},
	{"<doc>&\x01;</doc>"},
	{"<doc>&\xef\xbf\xbe;</doc>"},
	{"<doc>&hello;</doc>"},
}

const issue68387Input = "<item b=']]>'/>"

const trailingInput = "<FOO></FOO>  "

const entityInsideCDATAInput = "<test><![CDATA[ &val=foo ]]></test>"

const linePosInput = "<root>\n" +
	"<?pi\n" +
	" ?>  <elt\n" +
	"att\n" +
	"=\n" +
	"\"val\">\n" +
	"<![CDATA[\n" +
	"]]><!--\n\n" +
	"--></elt>\n" +
	"</root>"

type encodingXMLStrictMode int

type encodingXMLTokenMode int

const (
	encodingXMLStrictDefault encodingXMLStrictMode = iota
	encodingXMLStrictEnabled
	encodingXMLStrictDisabled
)

const (
	encodingXMLTokenCooked encodingXMLTokenMode = iota
	encodingXMLTokenRaw
)

type encodingXMLTokenOptions struct {
	entityMap     map[string]string
	charsetReader func(label string, r io.Reader) (io.Reader, error)
	strictMode    encodingXMLStrictMode
	tokenMode     encodingXMLTokenMode
}

func readXMLTextTokensWithOptions(input string, opts ...Options) ([]simpleToken, error) {
	base := []Options{
		ResolveEntities(true),
		EmitComments(true),
		EmitPI(true),
		EmitDirectives(true),
		CoalesceCharData(true),
	}
	base = append(base, opts...)
	dec := NewDecoder(strings.NewReader(input), base...)
	var tokens []simpleToken
	var tok Token
	var buf TokenBuffer
	for {
		err := dec.ReadTokenInto(&tok, &buf)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, simplifyXMLTextToken(tok))
	}
	return tokens, nil
}

func readXMLTextTokensRawWithOptions(input string, opts ...Options) ([]simpleToken, error) {
	base := []Options{
		ResolveEntities(true),
		EmitComments(true),
		EmitPI(true),
		EmitDirectives(true),
		CoalesceCharData(true),
	}
	base = append(base, opts...)
	dec := NewDecoder(strings.NewReader(input), base...)
	var tokens []simpleToken
	var tok Token
	var buf TokenBuffer
	for {
		err := dec.ReadTokenInto(&tok, &buf)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, simplifyXMLTextTokenRaw(tok))
	}
	return tokens, nil
}

func readEncodingXMLTokensWithOptions(input string, opts encodingXMLTokenOptions) ([]simpleToken, error) {
	dec := xml.NewDecoder(strings.NewReader(input))
	if opts.entityMap != nil {
		dec.Entity = opts.entityMap
	}
	if opts.charsetReader != nil {
		dec.CharsetReader = opts.charsetReader
	}
	switch opts.strictMode {
	case encodingXMLStrictEnabled:
		dec.Strict = true
	case encodingXMLStrictDisabled:
		dec.Strict = false
	}
	readToken := dec.Token
	if opts.tokenMode == encodingXMLTokenRaw {
		readToken = dec.RawToken
	}
	nameValue := func(name xml.Name) string {
		if opts.tokenMode != encodingXMLTokenRaw {
			return name.Local
		}
		if name.Space == "" {
			return name.Local
		}
		if name.Space == "xmlns" && name.Local == "xmlns" {
			return "xmlns"
		}
		return name.Space + ":" + name.Local
	}
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
		tok, err := readToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := tok.(type) {
		case xml.StartElement:
			flushText()
			out := simpleToken{kind: KindStartElement, name: nameValue(value.Name)}
			for _, attr := range value.Attr {
				out.attrs = append(out.attrs, simpleAttr{
					name:  nameValue(attr.Name),
					value: attr.Value,
				})
			}
			tokens = append(tokens, out)
		case xml.EndElement:
			flushText()
			tokens = append(tokens, simpleToken{kind: KindEndElement, name: nameValue(value.Name)})
		case xml.CharData:
			textBuf = append(textBuf, value...)
		case xml.Comment:
			flushText()
			tokens = append(tokens, simpleToken{kind: KindComment, text: string(value)})
		case xml.ProcInst:
			flushText()
			tokens = append(tokens, simpleToken{kind: KindPI, name: value.Target, text: string(value.Inst)})
		case xml.Directive:
			flushText()
			tokens = append(tokens, simpleToken{kind: KindDirective, text: string(value)})
		}
	}
	flushText()
	return tokens, nil
}

type downCaser struct {
	r io.ByteReader
}

func splitDirectiveInputs(input string) []string {
	var out []string
	for line := range strings.SplitSeq(input, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func wrapWithRoot(directive string) string {
	return directive + "<root/>"
}

func normalizeDirectiveTokens(tokens []simpleToken) []simpleToken {
	if len(tokens) == 0 {
		return tokens
	}
	out := make([]simpleToken, len(tokens))
	copy(out, tokens)
	for i := range out {
		if out[i].kind == KindDirective {
			out[i].text = stripDirectiveComments(out[i].text)
		}
	}
	return out
}

func stripDirectiveComments(text string) string {
	for {
		start := strings.Index(text, "<!--")
		if start < 0 {
			return text
		}
		rest := text[start+len("<!--"):]
		end := strings.Index(rest, "-->")
		if end < 0 {
			return text
		}
		text = text[:start] + " " + rest[end+len("-->"):]
	}
}

func (d *downCaser) Read(p []byte) (int, error) {
	for i := range p {
		b, err := d.ReadByte()
		if err != nil {
			if i == 0 {
				return 0, err
			}
			return i, nil
		}
		p[i] = b
	}
	return len(p), nil
}

func (d *downCaser) ReadByte() (byte, error) {
	c, err := d.r.ReadByte()
	if err != nil {
		return 0, err
	}
	if c >= 'A' && c <= 'Z' {
		c += 'a' - 'A'
	}
	return c, nil
}

func TestEncodingXMLParityCases(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		opts    []Options
		encOpts encodingXMLTokenOptions
	}{
		{
			name:  "testInput",
			input: testInputTrimmed,
			opts:  []Options{WithEntityMap(testEntity)},
			encOpts: encodingXMLTokenOptions{
				entityMap: testEntity,
			},
		},
		{
			name:  "issue68387",
			input: issue68387Input,
		},
		{
			name:  "trailingWhitespace",
			input: trailingInput,
		},
		{
			name:  "entityInsideCDATA",
			input: entityInsideCDATAInput,
		},
	}

	nestedInputs := splitDirectiveInputs(nestedDirectivesInput)
	for i, input := range nestedInputs {
		cases = append(cases, struct {
			name    string
			input   string
			opts    []Options
			encOpts encodingXMLTokenOptions
		}{
			name:  "nestedDirective:" + strconv.Itoa(i),
			input: wrapWithRoot(input),
		})
	}

	commentInputs := splitDirectiveInputs(directivesWithCommentsInput)
	for i, input := range commentInputs {
		if strings.Contains(input, "<!->") {
			continue
		}
		cases = append(cases, struct {
			name    string
			input   string
			opts    []Options
			encOpts encodingXMLTokenOptions
		}{
			name:  "directiveWithComment:" + strconv.Itoa(i),
			input: wrapWithRoot(input),
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := readXMLTextTokensWithOptions(tc.input, tc.opts...)
			if err != nil {
				t.Fatalf("readXMLTextTokens error = %v", err)
			}
			encTokens, err := readEncodingXMLTokensWithOptions(tc.input, tc.encOpts)
			if err != nil {
				t.Fatalf("readEncodingXMLTokens error = %v", err)
			}
			normalized := normalizeDirectiveTokens(tokens)
			encNormalized := normalizeDirectiveTokens(encTokens)
			if !tokensEqual(normalized, encNormalized) {
				t.Fatalf("tokens mismatch for %q:\nxmltext=%v\nencoding=%v", tc.input, normalized, encNormalized)
			}
		})
	}
}

func TestEncodingXMLRawTokenParityCases(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		opts    []Options
		encOpts encodingXMLTokenOptions
	}{
		{
			name:  "testInputRaw",
			input: testInputTrimmed,
			opts:  []Options{WithEntityMap(testEntity)},
			encOpts: encodingXMLTokenOptions{
				entityMap: testEntity,
				tokenMode: encodingXMLTokenRaw,
			},
		},
		{
			name:  "issue68387Raw",
			input: issue68387Input,
			encOpts: encodingXMLTokenOptions{
				tokenMode: encodingXMLTokenRaw,
			},
		},
		{
			name:  "trailingWhitespaceRaw",
			input: trailingInput,
			encOpts: encodingXMLTokenOptions{
				tokenMode: encodingXMLTokenRaw,
			},
		},
		{
			name:  "entityInsideCDATARaw",
			input: entityInsideCDATAInput,
			encOpts: encodingXMLTokenOptions{
				tokenMode: encodingXMLTokenRaw,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := readXMLTextTokensRawWithOptions(tc.input, tc.opts...)
			if err != nil {
				t.Fatalf("readXMLTextTokensRaw error = %v", err)
			}
			encTokens, err := readEncodingXMLTokensWithOptions(tc.input, tc.encOpts)
			if err != nil {
				t.Fatalf("readEncodingXMLTokens error = %v", err)
			}
			normalized := normalizeDirectiveTokens(tokens)
			encNormalized := normalizeDirectiveTokens(encTokens)
			if !tokensEqual(normalized, encNormalized) {
				t.Fatalf("tokens mismatch for %q:\nxmltext=%v\nencoding=%v", tc.input, normalized, encNormalized)
			}
		})
	}
}

func TestEncodingXMLAltEncoding(t *testing.T) {
	called := false
	reader := func(label string, input io.Reader) (io.Reader, error) {
		if label != "x-testing-uppercase" {
			return nil, errors.New("unexpected charset label")
		}
		byteReader, ok := input.(io.ByteReader)
		if !ok {
			return nil, errors.New("charset reader needs io.ByteReader")
		}
		called = true
		return &downCaser{r: byteReader}, nil
	}

	options := []Options{WithCharsetReader(reader)}
	tokens, err := readXMLTextTokensWithOptions(testInputAltEncoding, options...)
	if err != nil {
		t.Fatalf("readXMLTextTokens error = %v", err)
	}
	encTokens, err := readEncodingXMLTokensWithOptions(testInputAltEncoding, encodingXMLTokenOptions{charsetReader: reader})
	if err != nil {
		t.Fatalf("readEncodingXMLTokens error = %v", err)
	}
	if !called {
		t.Fatalf("expected charset reader to be called")
	}
	normalized := normalizeDirectiveTokens(tokens)
	encNormalized := normalizeDirectiveTokens(encTokens)
	if !tokensEqual(normalized, encNormalized) {
		t.Fatalf("tokens mismatch:\nxmltext=%v\nencoding=%v", normalized, encNormalized)
	}
}

func TestEncodingXMLAltEncodingNoConverter(t *testing.T) {
	dec := NewDecoder(strings.NewReader(testInputAltEncoding))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected charset error")
	}
	if !errors.Is(err, errUnsupportedEncoding) {
		t.Fatalf("error = %v, want %v", err, errUnsupportedEncoding)
	}
}

func TestEncodingXMLDeclAfterWhitespaceRejected(t *testing.T) {
	tests := []string{
		testInput,
		testInputAltEncodingWhitespace,
	}
	for _, input := range tests {
		dec := NewDecoder(strings.NewReader(input))
		var err error
		reader := newTokenReader(dec)
		for err == nil {
			_, err = reader.Next()
		}
		var syntax *SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("input %q error = %v, want SyntaxError", input, err)
		}
	}
}

func TestEncodingXMLDuplicateDirectivesRejected(t *testing.T) {
	tests := []string{
		nestedDirectivesInput,
		directivesWithCommentsInput,
	}
	for _, input := range tests {
		dec := NewDecoder(strings.NewReader(input))
		var err error
		reader := newTokenReader(dec)
		for err == nil {
			_, err = reader.Next()
		}
		var syntax *SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("input %q error = %v, want SyntaxError", input, err)
		}
	}
}

func TestEncodingXMLDirectiveCommentsRejected(t *testing.T) {
	for _, input := range splitDirectiveInputs(directivesWithCommentsInput) {
		if !strings.Contains(input, "<!->") {
			continue
		}
		dec := NewDecoder(strings.NewReader(wrapWithRoot(input)))
		var err error
		reader := newTokenReader(dec)
		for err == nil {
			_, err = reader.Next()
		}
		var syntax *SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("input %q error = %v, want SyntaxError", input, err)
		}
	}
}

func TestEncodingXMLSyntaxErrors(t *testing.T) {
	for _, input := range xmlInput {
		dec := NewDecoder(strings.NewReader(input))
		var err error
		reader := newTokenReader(dec)
		for err == nil {
			_, err = reader.Next()
		}
		var syntax *SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("input %q error = %v, want SyntaxError", input, err)
		}
	}
}

func TestEncodingXMLNonStrictInputsRejected(t *testing.T) {
	tests := []string{
		nonStrictInput,
		"<tag attr=azAZ09:-_\t>",
		"<p nowrap>",
		"<p nowrap >",
		"<input checked/>",
		"<input checked />",
	}
	for _, input := range tests {
		dec := NewDecoder(strings.NewReader(input))
		var err error
		reader := newTokenReader(dec)
		for err == nil {
			_, err = reader.Next()
		}
		var syntax *SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("input %q error = %v, want SyntaxError", input, err)
		}
	}
}

func TestEncodingXMLInputOffset(t *testing.T) {
	dec := NewDecoder(
		strings.NewReader(testInputTrimmed),
		ResolveEntities(true),
		EmitComments(true),
		EmitPI(true),
		EmitDirectives(true),
		CoalesceCharData(true),
		WithEntityMap(testEntity),
	)
	inputBytes := []byte(testInputTrimmed)
	var lastEnd int64
	var tok Token
	var buf TokenBuffer
	for {
		start := dec.InputOffset()
		err := dec.ReadTokenInto(&tok, &buf)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("ReadToken error = %v", err)
		}
		end := dec.InputOffset()
		switch {
		case start < lastEnd:
			t.Fatalf("token %v: start %d before last end %d", tok.Kind, start, lastEnd)
		case start >= end:
			if start == end && end == lastEnd {
				break
			}
			t.Fatalf("token %v: start %d >= end %d", tok.Kind, start, end)
		case end > int64(len(inputBytes)):
			t.Fatalf("token %v: end %d beyond input", tok.Kind, end)
		}
		lastEnd = end
	}
}

func TestEncodingXMLLineColumns(t *testing.T) {
	dec := NewDecoder(
		strings.NewReader(linePosInput),
		ResolveEntities(true),
		EmitComments(true),
		EmitPI(true),
		EmitDirectives(true),
	)
	inputBytes := []byte(linePosInput)
	var tok Token
	var buf TokenBuffer
	for {
		start := dec.InputOffset()
		err := dec.ReadTokenInto(&tok, &buf)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("ReadToken error = %v", err)
		}
		line, col := lineColumnAtOffset(inputBytes, int(start))
		if tok.Line != line || tok.Column != col {
			t.Fatalf("token %v line/col = %d/%d, want %d/%d", tok.Kind, tok.Line, tok.Column, line, col)
		}
	}
}

func lineColumnAtOffset(data []byte, offset int) (int, int) {
	line, column := 1, 1
	for i := 0; i < offset && i < len(data); {
		switch data[i] {
		case '\n':
			line++
			column = 1
			i++
		case '\r':
			line++
			column = 1
			i++
			if i < offset && i < len(data) && data[i] == '\n' {
				i++
			}
		default:
			column++
			i++
		}
	}
	return line, column
}

func TestEncodingXMLDisallowedCharacters(t *testing.T) {
	for _, test := range characterTests {
		dec := NewDecoder(strings.NewReader(test.input))
		var err error
		reader := newTokenReader(dec)
		for err == nil {
			_, err = reader.Next()
		}
		var syntax *SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("input %q error = %v, want SyntaxError", test.input, err)
		}
	}
}

func TestEncodingXMLCharacterRanges(t *testing.T) {
	invalid := []rune{
		utf8.MaxRune + 1,
		0xD800,
		0xDFFF,
		-1,
	}
	for _, r := range invalid {
		if isValidXMLChar(r) {
			t.Fatalf("rune %U considered valid", r)
		}
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
	name  string
	text  string
	attrs []simpleAttr
	kind  Kind
}

type simpleAttr struct {
	name  string
	value string
}

func readXMLTextTokens(input string) ([]simpleToken, error) {
	return readXMLTextTokensWithOptions(input)
}

func readEncodingXMLTokens(input string) ([]simpleToken, error) {
	return readEncodingXMLTokensWithOptions(input, encodingXMLTokenOptions{})
}

func simplifyXMLTextToken(tok Token) simpleToken {
	kind := tok.Kind
	if kind == KindCDATA {
		kind = KindCharData
	}
	out := simpleToken{kind: kind}
	switch kind {
	case KindStartElement, KindEndElement:
		out.name = localName(tok.Name)
	case KindCharData, KindComment, KindDirective:
		out.text = string(tok.Text)
	case KindPI:
		target, inst := splitPIText(tok.Text)
		out.name = target
		out.text = string(inst)
	}
	if tok.Kind == KindStartElement {
		for _, attr := range tok.Attrs {
			out.attrs = append(out.attrs, simpleAttr{
				name:  localName(attr.Name),
				value: string(attr.Value),
			})
		}
	}
	return out
}

func simplifyXMLTextTokenRaw(tok Token) simpleToken {
	kind := tok.Kind
	if kind == KindCDATA {
		kind = KindCharData
	}
	out := simpleToken{kind: kind}
	switch kind {
	case KindStartElement, KindEndElement:
		out.name = string(tok.Name)
	case KindCharData, KindComment, KindDirective:
		out.text = string(tok.Text)
	case KindPI:
		target, inst := splitPIText(tok.Text)
		out.name = target
		out.text = string(inst)
	}
	if tok.Kind == KindStartElement {
		for _, attr := range tok.Attrs {
			out.attrs = append(out.attrs, simpleAttr{
				name:  string(attr.Name),
				value: string(attr.Value),
			})
		}
	}
	return out
}

func localName(name []byte) string {
	for i, b := range name {
		if b == ':' {
			return string(name[i+1:])
		}
	}
	return string(name)
}

func splitPIText(data []byte) (string, []byte) {
	for i := range data {
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

package format

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jacoelho/xsd/xsderrors"
)

type formatDataErrorReader struct {
	data string
	err  error
	done bool
}

func (r *formatDataErrorReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), r.err
}

func TestFormatXMLIndentsElements(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root><item id="1"><name>A &amp; B</name></item><empty/></root>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}

	const want = `<root>
  <item id="1">
    <name>A &amp; B</name>
  </item>
  <empty></empty>
</root>`
	if out.String() != want {
		t.Fatalf("XML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLDataWithEOFRequiresPureEOF(t *testing.T) {
	const input = `<root/>`
	var pure strings.Builder
	if err := XML(&pure, &formatDataErrorReader{data: input, err: io.EOF}); err != nil {
		t.Fatalf("XML(pure EOF) error = %v", err)
	}

	sentinel := errors.New("read failed")
	var joined strings.Builder
	err := XML(&joined, &formatDataErrorReader{data: input, err: errors.Join(io.EOF, sentinel)})
	if !errors.Is(err, sentinel) {
		t.Fatalf("XML(joined EOF) error = %v, want reader cause", err)
	}
	if joined.Len() != 0 {
		t.Fatalf("XML(joined EOF) wrote %q before rejecting input", joined.String())
	}
}

func TestFormatXMLPreservesWhitespaceOnlyText(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root><v> </v></root>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}

	const want = `<root>
  <v> </v>
</root>`
	if out.String() != want {
		t.Fatalf("XML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLDoesNotIndentMixedContent(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<p><b>bold</b>tail</p>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<p><b>bold</b>tail</p>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLPreservesInlineWhitespaceBetweenElements(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<p><b>bold</b> <i>it</i></p>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<p><b>bold</b> <i>it</i></p>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLReindentsLineBreakWhitespaceBetweenElements(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader("<root>\n<a></a>\n<b></b>\n</root>"))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}

	const want = `<root>
  <a></a>
  <b></b>
</root>`
	if out.String() != want {
		t.Fatalf("XML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLPreservesXMLSpace(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root xml:space="preserve"> <a> x </a> </root>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<root xml:space="preserve"> <a> x </a> </root>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLEscapesAttributeWhitespace(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root a="x&#10;y&#13;z&#9;w"/>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<root a="x&#10;y&#13;z&#9;w"></root>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLEscapesAttributePredefinedEntities(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root a="&amp;&lt;&quot;"/>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<root a="&amp;&lt;&quot;"></root>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLPreservesNamespaceDeclarations(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<?xml version="1.0"?>
<x:books xmlns:x="urn:books"><book id="bk001"/></x:books>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}

	const want = `<x:books xmlns:x="urn:books">
  <book id="bk001"></book>
</x:books>`
	if out.String() != want {
		t.Fatalf("XML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLStripsUTF8BOM(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader("\xef\xbb\xbf<root/>"))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<root></root>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLHandlesBareCRText(t *testing.T) {
	done := make(chan struct{})
	var out strings.Builder
	var err error
	go func() {
		err = XML(&out, strings.NewReader("<root>a\rb</root>"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("XML() timed out")
	}
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<root>a&#xA;b</root>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLNormalizesCDATALineEndings(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader("<root><![CDATA[a\rb]]></root>"))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != "<root><![CDATA[a\nb]]></root>" {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLPreservesProcessingInstructions(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<?xml version="1.0"?><?xml-stylesheet type="text/xsl" href="style.xsl"?><root><?pi data?><v>1</v></root><?tail?>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}

	const want = `<?xml-stylesheet type="text/xsl" href="style.xsl"?>
<root>
  <?pi data?>
  <v>1</v>
</root>
<?tail?>`
	if out.String() != want {
		t.Fatalf("XML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLRejectsMalformedProcessingInstructions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "xml declaration without content", input: `<?xml?><root/>`, want: "invalid XML declaration"},
		{name: "xml target after start", input: `<root><?xml version="1.0"?></root>`, want: "xml processing instruction target is reserved"},
		{name: "question before target end", input: `<?pi? data?><root/>`, want: "processing instruction target must be followed by whitespace or ?>"},
		{name: "eof in target", input: `<?pi`, want: "unexpected EOF in processing instruction"},
		{name: "eof in content", input: `<?pi data`, want: "unexpected EOF"},
		{name: "invalid utf8 content", input: "<?pi \xff?><root/>", want: "invalid UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			err := XML(&out, strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("XML() succeeded")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("XML() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestFormatXMLRejectsDuplicateAttributes(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root a="1" a="2"/>`))
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "duplicate attribute") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLRejectsExpandedDuplicateAttributes(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root xmlns:a="urn:x" xmlns:b="urn:x" a:id="1" b:id="2"/>`))
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "duplicate attribute {urn:x}id") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLRejectsDuplicateNamespaceDeclarations(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root xmlns:a="urn:x" xmlns:a="urn:y"/>`))
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "duplicate attribute") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLRejectsLargeDuplicateAttributes(t *testing.T) {
	var input strings.Builder
	input.WriteString("<root")
	for i := range 40 {
		input.WriteString(` a`)
		input.WriteString(strconv.Itoa(i))
		input.WriteString(`="`)
		input.WriteString(strconv.Itoa(i))
		input.WriteByte('"')
	}
	input.WriteString(` a39="dup"/>`)
	var out strings.Builder
	err := XML(&out, strings.NewReader(input.String()))
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "duplicate attribute a39") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLRejectsUnboundAttributePrefix(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root p:id="1"/>`))
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "unbound namespace prefix p") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLRejectsCDATAOutsideRoot(t *testing.T) {
	for _, input := range []string{
		`<![CDATA[ ]]><root/>`,
		`<root/><![CDATA[
]]>`,
	} {
		t.Run(input, func(t *testing.T) {
			var out strings.Builder
			err := XML(&out, strings.NewReader(input))
			if err == nil {
				t.Fatal("XML() succeeded")
			}
			if !strings.Contains(err.Error(), "CDATA section outside root element") {
				t.Fatalf("XML() error = %v", err)
			}
		})
	}
}

func TestFormatXMLRejectsMalformedComments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "double hyphen", input: `<root><!-- bad -- comment --></root>`, want: "invalid XML comment"},
		{name: "eof after dash", input: `<root><!-- bad -`, want: "unexpected EOF in comment"},
		{name: "eof after double dash", input: `<root><!-- bad --`, want: "unexpected EOF in comment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			err := XML(&out, strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("XML() succeeded")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("XML() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestFormatXMLRejectsTextOutsideRoot(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root/>text`))
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "text outside root element") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLRejectsEmptyAndUnclosedDocuments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "XML document is empty"},
		{name: "unclosed", input: "<root>", want: "unexpected EOF before end element </root>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			err := XML(&out, strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("XML() succeeded")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("XML() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestFormatXMLWithOptionsLimitsNodes(t *testing.T) {
	var out strings.Builder
	err := XMLWithOptions(&out, strings.NewReader(`<root><a/><b/></root>`), Options{MaxNodes: 2})
	if err == nil {
		t.Fatal("XMLWithOptions() succeeded")
	}
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("XMLWithOptions() error type = %T, want *xsderrors.Error", err)
	}
	if !strings.Contains(err.Error(), "XML node limit exceeded") {
		t.Fatalf("XMLWithOptions() error = %v", err)
	}
}

func TestFormatXMLWithOptionsAllowsTokenAtLimit(t *testing.T) {
	var out strings.Builder
	err := XMLWithOptions(&out, strings.NewReader(`<?pi abc?><r/>`), Options{MaxTokenBytes: 5})
	if err != nil {
		t.Fatalf("XMLWithOptions() error = %v", err)
	}
	if out.String() != "<?pi abc?>\n<r></r>" {
		t.Fatalf("XMLWithOptions() = %q", out.String())
	}
}

func TestFormatXMLWithOptionsAllowsInputBytesAtLimit(t *testing.T) {
	var out strings.Builder
	input := `<r/>`
	err := XMLWithOptions(&out, strings.NewReader(input), Options{MaxInputBytes: int64(len(input))})
	if err != nil {
		t.Fatalf("XMLWithOptions() error = %v", err)
	}
	if out.String() != "<r></r>" {
		t.Fatalf("XMLWithOptions() = %q", out.String())
	}
}

func TestFormatXMLWithOptionsRejectsOutputBytesAfterPartialWrite(t *testing.T) {
	var out strings.Builder
	err := XMLWithOptions(&out, strings.NewReader(`<root><item/></root>`), Options{MaxOutputBytes: 8})
	if err == nil {
		t.Fatal("XMLWithOptions() succeeded")
	}
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("XMLWithOptions() error type = %T, want *xsderrors.Error", err)
	}
	if !errors.Is(err, errFormatOutputLimit) {
		t.Fatalf("XMLWithOptions() error = %v, want %v", err, errFormatOutputLimit)
	}
	if out.Len() > 8 {
		t.Fatalf("output len = %d, want <= 8", out.Len())
	}
}

func TestFormatXMLWithOptionsRejectsInputBytesAfterSniff(t *testing.T) {
	var out strings.Builder
	input := `<r/>`
	err := XMLWithOptions(&out, strings.NewReader(input+"X"), Options{MaxInputBytes: int64(len(input))})
	if err == nil {
		t.Fatal("XMLWithOptions() succeeded")
	}
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("XMLWithOptions() error type = %T, want *xsderrors.Error", err)
	}
	if xerr.Code != codeFormatLimit {
		t.Fatalf("XMLWithOptions() code = %q, want %q", xerr.Code, codeFormatLimit)
	}
	if !errors.Is(err, errFormatInputLimit) {
		t.Fatalf("XMLWithOptions() error = %v, want %v", err, errFormatInputLimit)
	}
}

func TestFormatXMLWithOptionsRejectsNegativeLimits(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "depth", opts: Options{MaxDepth: -1}},
		{name: "nodes", opts: Options{MaxNodes: -1}},
		{name: "input", opts: Options{MaxInputBytes: -1}},
		{name: "output", opts: Options{MaxOutputBytes: -1}},
		{name: "token", opts: Options{MaxTokenBytes: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			err := XMLWithOptions(&out, strings.NewReader(`<root/>`), tt.opts)
			if err == nil {
				t.Fatal("XMLWithOptions() succeeded")
			}
			var xerr *xsderrors.Error
			if !errors.As(err, &xerr) {
				t.Fatalf("XMLWithOptions() error type = %T, want *xsderrors.Error", err)
			}
		})
	}
}

func TestFormatXMLWithOptionsRejectsNilEndpoints(t *testing.T) {
	var out strings.Builder
	if err := XMLWithOptions(nil, strings.NewReader(`<root/>`), Options{}); err == nil {
		t.Fatal("XMLWithOptions() accepted nil writer")
	}
	if err := XMLWithOptions(&out, nil, Options{}); err == nil {
		t.Fatal("XMLWithOptions() accepted nil reader")
	}
}

func TestFormatXMLRejectsExcessiveDepth(t *testing.T) {
	var input strings.Builder
	for range maxFormatDepth + 1 {
		input.WriteString("<a>")
	}
	for range maxFormatDepth + 1 {
		input.WriteString("</a>")
	}

	var out strings.Builder
	err := XML(&out, strings.NewReader(input.String()))
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "XML nesting exceeds") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLReportsLine(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader("<root>\n  <a></root>"))
	if err == nil {
		t.Fatal("XML() succeeded")
	}

	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("XML() error type = %T, want *xsderrors.Error", err)
	}
	if xerr.Line != 2 {
		t.Fatalf("Line = %d, want 2", xerr.Line)
	}
}

func TestFormatXMLPreservesText(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root>  keep  </root>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<root>  keep  </root>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLPreservesComments(t *testing.T) {
	var out strings.Builder
	err := XML(&out, strings.NewReader(`<root><!-- note --><v>1</v></root>`))
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}

	const want = `<root>
  <!-- note -->
  <v>1</v>
</root>`
	if out.String() != want {
		t.Fatalf("XML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLKeepsCommentOnlyContentInline(t *testing.T) {
	for _, input := range []string{
		`<v><!--c--></v>`,
		`<v><!--c--> </v>`,
		`<v> <!--c--> </v>`,
		`<v><?pi?> </v>`,
	} {
		t.Run(input, func(t *testing.T) {
			var out strings.Builder
			err := XML(&out, strings.NewReader(input))
			if err != nil {
				t.Fatalf("XML() error = %v", err)
			}
			if out.String() != input {
				t.Fatalf("XML() = %q, want %q", out.String(), input)
			}
		})
	}
}

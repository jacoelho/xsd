package xsd

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFormatXMLIndentsElements(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<root><item id="1"><name>A &amp; B</name></item><empty/></root>`))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}

	const want = `<root>
  <item id="1">
    <name>A &amp; B</name>
  </item>
  <empty></empty>
</root>`
	if out.String() != want {
		t.Fatalf("FormatXML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLPreservesWhitespaceOnlyText(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<root><v> </v></root>`))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}

	const want = `<root>
  <v> </v>
</root>`
	if out.String() != want {
		t.Fatalf("FormatXML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLDoesNotIndentMixedContent(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<p><b>bold</b>tail</p>`))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}
	if out.String() != `<p><b>bold</b>tail</p>` {
		t.Fatalf("FormatXML() = %q", out.String())
	}
}

func TestFormatXMLPreservesInlineWhitespaceBetweenElements(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<p><b>bold</b> <i>it</i></p>`))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}
	if out.String() != `<p><b>bold</b> <i>it</i></p>` {
		t.Fatalf("FormatXML() = %q", out.String())
	}
}

func TestFormatXMLReindentsLineBreakWhitespaceBetweenElements(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader("<root>\n<a></a>\n<b></b>\n</root>"))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}

	const want = `<root>
  <a></a>
  <b></b>
</root>`
	if out.String() != want {
		t.Fatalf("FormatXML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLPreservesXMLSpace(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<root xml:space="preserve"> <a> x </a> </root>`))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}
	if out.String() != `<root xml:space="preserve"> <a> x </a> </root>` {
		t.Fatalf("FormatXML() = %q", out.String())
	}
}

func TestFormatXMLEscapesAttributeWhitespace(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<root a="x&#10;y&#13;z&#9;w"/>`))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}
	if out.String() != `<root a="x&#10;y&#13;z&#9;w"></root>` {
		t.Fatalf("FormatXML() = %q", out.String())
	}
}

func TestFormatXMLPreservesNamespaceDeclarations(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<?xml version="1.0"?>
<x:books xmlns:x="urn:books"><book id="bk001"/></x:books>`))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}

	const want = `<x:books xmlns:x="urn:books">
  <book id="bk001"></book>
</x:books>`
	if out.String() != want {
		t.Fatalf("FormatXML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLStripsUTF8BOM(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader("\xef\xbb\xbf<root/>"))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}
	if out.String() != `<root></root>` {
		t.Fatalf("FormatXML() = %q", out.String())
	}
}

func TestFormatXMLHandlesBareCRText(t *testing.T) {
	done := make(chan struct{})
	var out strings.Builder
	var err error
	go func() {
		err = FormatXML(&out, strings.NewReader("<root>a\rb</root>"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("FormatXML() timed out")
	}
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}
	if out.String() != `<root>a&#xA;b</root>` {
		t.Fatalf("FormatXML() = %q", out.String())
	}
}

func TestFormatXMLPreservesProcessingInstructions(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<?xml version="1.0"?><?xml-stylesheet type="text/xsl" href="style.xsl"?><root><?pi data?><v>1</v></root><?tail?>`))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}

	const want = `<?xml-stylesheet type="text/xsl" href="style.xsl"?>
<root>
  <?pi data?>
  <v>1</v>
</root>
<?tail?>`
	if out.String() != want {
		t.Fatalf("FormatXML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLRejectsDuplicateAttributes(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<root a="1" a="2"/>`))
	if err == nil {
		t.Fatal("FormatXML() succeeded")
	}
	if !strings.Contains(err.Error(), "duplicate attribute") {
		t.Fatalf("FormatXML() error = %v", err)
	}
}

func TestFormatXMLRejectsExpandedDuplicateAttributes(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<root xmlns:a="urn:x" xmlns:b="urn:x" a:id="1" b:id="2"/>`))
	if err == nil {
		t.Fatal("FormatXML() succeeded")
	}
	if !strings.Contains(err.Error(), "duplicate attribute {urn:x}id") {
		t.Fatalf("FormatXML() error = %v", err)
	}
}

func TestFormatXMLRejectsUnboundAttributePrefix(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<root p:id="1"/>`))
	if err == nil {
		t.Fatal("FormatXML() succeeded")
	}
	if !strings.Contains(err.Error(), "unbound namespace prefix p") {
		t.Fatalf("FormatXML() error = %v", err)
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
			err := FormatXML(&out, strings.NewReader(input))
			if err == nil {
				t.Fatal("FormatXML() succeeded")
			}
			if !strings.Contains(err.Error(), "CDATA section outside root element") {
				t.Fatalf("FormatXML() error = %v", err)
			}
		})
	}
}

func TestFormatXMLWithOptionsLimitsNodes(t *testing.T) {
	var out strings.Builder
	err := FormatXMLWithOptions(&out, strings.NewReader(`<root><a/><b/></root>`), FormatOptions{MaxNodes: 2})
	if err == nil {
		t.Fatal("FormatXMLWithOptions() succeeded")
	}
	var xerr *XMLFormatError
	if !errors.As(err, &xerr) {
		t.Fatalf("FormatXMLWithOptions() error type = %T, want *XMLFormatError", err)
	}
	if !strings.Contains(err.Error(), "XML node limit exceeded") {
		t.Fatalf("FormatXMLWithOptions() error = %v", err)
	}
}

func TestFormatXMLWithOptionsAllowsTokenAtLimit(t *testing.T) {
	var out strings.Builder
	err := FormatXMLWithOptions(&out, strings.NewReader(`<?pi abc?><r/>`), FormatOptions{MaxTokenBytes: 3})
	if err != nil {
		t.Fatalf("FormatXMLWithOptions() error = %v", err)
	}
	if out.String() != "<?pi abc?>\n<r></r>" {
		t.Fatalf("FormatXMLWithOptions() = %q", out.String())
	}
}

func TestFormatXMLWithOptionsRejectsNegativeLimits(t *testing.T) {
	var out strings.Builder
	err := FormatXMLWithOptions(&out, strings.NewReader(`<root/>`), FormatOptions{MaxNodes: -1})
	if err == nil {
		t.Fatal("FormatXMLWithOptions() succeeded")
	}
	var xerr *XMLFormatError
	if !errors.As(err, &xerr) {
		t.Fatalf("FormatXMLWithOptions() error type = %T, want *XMLFormatError", err)
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
	err := FormatXML(&out, strings.NewReader(input.String()))
	if err == nil {
		t.Fatal("FormatXML() succeeded")
	}
	if !strings.Contains(err.Error(), "XML nesting exceeds") {
		t.Fatalf("FormatXML() error = %v", err)
	}
}

func TestFormatXMLReportsLine(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader("<root>\n  <a></root>"))
	if err == nil {
		t.Fatal("FormatXML() succeeded")
	}

	var xerr *XMLFormatError
	if !errors.As(err, &xerr) {
		t.Fatalf("FormatXML() error type = %T, want *XMLFormatError", err)
	}
	if xerr.Line != 2 {
		t.Fatalf("Line = %d, want 2", xerr.Line)
	}
}

func TestFormatXMLPreservesText(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<root>  keep  </root>`))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}
	if out.String() != `<root>  keep  </root>` {
		t.Fatalf("FormatXML() = %q", out.String())
	}
}

func TestFormatXMLPreservesComments(t *testing.T) {
	var out strings.Builder
	err := FormatXML(&out, strings.NewReader(`<root><!-- note --><v>1</v></root>`))
	if err != nil {
		t.Fatalf("FormatXML() error = %v", err)
	}

	const want = `<root>
  <!-- note -->
  <v>1</v>
</root>`
	if out.String() != want {
		t.Fatalf("FormatXML() =\n%s\nwant\n%s", out.String(), want)
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
			err := FormatXML(&out, strings.NewReader(input))
			if err != nil {
				t.Fatalf("FormatXML() error = %v", err)
			}
			if out.String() != input {
				t.Fatalf("FormatXML() = %q, want %q", out.String(), input)
			}
		})
	}
}

package format

import (
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

type shortNilWriter struct{}

func (shortNilWriter) Write(p []byte) (int, error) {
	return len(p) - 1, nil
}

type formatWriterFunc func([]byte) (int, error)

func (f formatWriterFunc) Write(p []byte) (int, error) {
	return f(p)
}

type formatDualWriter struct {
	write       func([]byte) (int, error)
	writeString func(string) (int, error)
	writes      int
	strings     int
}

func (w *formatDualWriter) Write(p []byte) (int, error) {
	w.writes++
	return w.write(p)
}

func (w *formatDualWriter) WriteString(s string) (int, error) {
	w.strings++
	return w.writeString(s)
}

func TestMaxBytesWriterWriteAndWriteStringShareSemantics(t *testing.T) {
	t.Parallel()

	limitErr := errors.New("limit")
	writeErr := errors.New("write")
	tests := []struct {
		name      string
		max       int64
		input     string
		delegateN int
		delegate  error
		wantN     int
		wantErr   error
		wantCause error
		rejectErr error
		wantBytes int64
	}{
		{name: "exact", max: 3, input: "abc", delegateN: 3, wantN: 3, wantBytes: 3},
		{name: "crossed limit", max: 2, input: "€x", delegateN: 2, wantN: 2, wantErr: limitErr, wantBytes: 2},
		{name: "crossed limit with writer error", max: 2, input: "abc", delegateN: 2, delegate: writeErr, wantN: 2, wantErr: writeErr, rejectErr: limitErr, wantBytes: 2},
		{name: "negative count", max: 3, input: "abc", delegateN: -1, wantErr: io.ErrShortWrite},
		{name: "negative count with cause", max: 3, input: "abc", delegateN: -1, delegate: writeErr, wantErr: io.ErrShortWrite, wantCause: writeErr},
		{name: "oversized count", max: 3, input: "abc", delegateN: 4, wantErr: io.ErrShortWrite},
		{name: "oversized count with cause", max: 3, input: "abc", delegateN: 4, delegate: writeErr, wantErr: io.ErrShortWrite, wantCause: writeErr},
		{name: "short nil", max: 3, input: "abc", delegateN: 2, wantN: 2, wantErr: io.ErrShortWrite, wantBytes: 2},
		{name: "partial with cause", max: 3, input: "abc", delegateN: 2, delegate: writeErr, wantN: 2, wantErr: writeErr, wantBytes: 2},
		{name: "complete with cause", max: 3, input: "abc", delegateN: 3, delegate: writeErr, wantN: 3, wantErr: writeErr, wantBytes: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, method := range []string{"Write", "WriteString"} {
				t.Run(method, func(t *testing.T) {
					t.Parallel()
					var delegated string
					delegate := func(s string) (int, error) {
						delegated = s
						return tt.delegateN, tt.delegate
					}
					underlying := &formatDualWriter{
						write:       func(p []byte) (int, error) { return delegate(string(p)) },
						writeString: delegate,
					}
					var bounded *maxBytesWriter
					var n int
					var err error
					if method == "Write" {
						bounded = &maxBytesWriter{w: underlying, max: tt.max, err: limitErr}
						n, err = bounded.Write([]byte(tt.input))
						if underlying.writes != 1 || underlying.strings != 0 {
							t.Fatalf("delegate calls = Write %d, WriteString %d", underlying.writes, underlying.strings)
						}
					} else {
						w := &maxBytesStringWriter{
							w: underlying, max: tt.max, err: limitErr,
						}
						bounded = &w.maxBytesWriter
						n, err = w.WriteString(tt.input)
						if underlying.writes != 0 || underlying.strings != 1 {
							t.Fatalf("delegate calls = Write %d, WriteString %d", underlying.writes, underlying.strings)
						}
					}
					if n != tt.wantN || !errors.Is(err, tt.wantErr) || (tt.wantCause != nil && !errors.Is(err, tt.wantCause)) {
						t.Fatalf("result = %d, %v; want %d, %v with cause %v", n, err, tt.wantN, tt.wantErr, tt.wantCause)
					}
					if tt.rejectErr != nil && errors.Is(err, tt.rejectErr) {
						t.Fatalf("result error = %v, must not contain %v", err, tt.rejectErr)
					}
					wantDelegated := tt.input
					if int64(len(wantDelegated)) > tt.max {
						wantDelegated = wantDelegated[:tt.max]
					}
					if delegated != wantDelegated {
						t.Fatalf("delegated %q, want byte prefix %q", delegated, wantDelegated)
					}
					if bounded.n != tt.wantBytes {
						t.Fatalf("recorded bytes = %d, want %d", bounded.n, tt.wantBytes)
					}
				})
			}
		})
	}
}

func TestMaxBytesWriterExhaustionAndEmptyWrites(t *testing.T) {
	t.Parallel()

	limitErr := errors.New("limit")
	var out strings.Builder
	w := &maxBytesStringWriter{
		w: &out, max: 1, err: limitErr,
	}
	if n, err := w.WriteString("a"); n != 1 || err != nil {
		t.Fatalf("first WriteString() = %d, %v", n, err)
	}
	if n, err := w.WriteString("b"); n != 0 || !errors.Is(err, limitErr) {
		t.Fatalf("exhausted WriteString() = %d, %v", n, err)
	}
	if n, err := w.Write(nil); n != 0 || err != nil {
		t.Fatalf("empty Write() = %d, %v", n, err)
	}
	if n, err := w.WriteString(""); n != 0 || err != nil {
		t.Fatalf("empty WriteString() = %d, %v", n, err)
	}
	if out.String() != "a" || w.n != 1 {
		t.Fatalf("writer = %q, %d bytes; want a, 1", out.String(), w.n)
	}
}

func TestNewMaxBytesWriterSelectsStringWriterCapability(t *testing.T) {
	t.Parallel()

	var direct strings.Builder
	withStringWriter := newMaxBytesWriter(&direct, 3, errors.New("limit"))
	if _, ok := withStringWriter.(io.StringWriter); !ok {
		t.Fatal("StringWriter delegate lost WriteString capability")
	}

	var out strings.Builder
	writer := formatWriterFunc(out.Write)
	writerOnly := newMaxBytesWriter(writer, 3, errors.New("limit"))
	if _, ok := writerOnly.(io.StringWriter); ok {
		t.Fatal("writer-only delegate gained WriteString capability")
	}
	if n, err := io.WriteString(writerOnly, "abc"); n != 3 || err != nil {
		t.Fatalf("io.WriteString() = %d, %v", n, err)
	}
	if out.String() != "abc" {
		t.Fatalf("fallback output = %q", out.String())
	}
}

func TestMaxBytesStringWriterDoesNotAddRetainedState(t *testing.T) {
	t.Parallel()

	if got, want := unsafe.Sizeof(maxBytesStringWriter{}), unsafe.Sizeof(maxBytesWriter{}); got > want {
		t.Fatalf("StringWriter wrapper size = %d, base writer = %d", got, want)
	}
}

func TestMaxBytesWriterAccountsActualBytesBeforeNextLimit(t *testing.T) {
	t.Parallel()

	limitErr := errors.New("limit")
	calls := 0
	writer := formatWriterFunc(func(p []byte) (int, error) {
		calls++
		if calls == 1 {
			return 1, errors.New("first write")
		}
		return len(p), nil
	})
	w := newMaxBytesWriter(writer, 2, limitErr)
	if n, err := w.Write([]byte("ab")); n != 1 || err == nil {
		t.Fatalf("first Write() = %d, %v", n, err)
	}
	if n, err := io.WriteString(w, "cd"); n != 1 || !errors.Is(err, limitErr) {
		t.Fatalf("second io.WriteString() = %d, %v", n, err)
	}
	bounded, ok := w.(*maxBytesWriter)
	if !ok {
		t.Fatalf("writer-only wrapper type = %T", w)
	}
	if bounded.n != 2 {
		t.Fatalf("recorded bytes = %d, want 2", bounded.n)
	}
}

func TestFormatXMLIndentsElements(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root><item id="1"><name>A &amp; B</name></item><empty/></root>`)
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

func TestFormatXMLPreservesWhitespaceOnlyText(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root><v> </v></root>`)
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

func TestFormatXMLRejectsXML11WithoutInternalCause(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	err := XML(&out, `<?xml version="1.1"?><root/>`)
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Code() != xsderrors.CodeUnsupportedXML11 {
		t.Fatalf("XML() error = %v, want %q", err, xsderrors.CodeUnsupportedXML11)
	}
	if diagnostic.Cause() != nil {
		t.Fatalf("XML() XML 1.1 cause = %T, want nil", diagnostic.Cause())
	}
}

func TestFormatXMLDoesNotIndentMixedContent(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<p><b>bold</b>tail</p>`)
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<p><b>bold</b>tail</p>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLPreservesInlineWhitespaceBetweenElements(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<p><b>bold</b> <i>it</i></p>`)
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<p><b>bold</b> <i>it</i></p>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLReindentsLineBreakWhitespaceBetweenElements(t *testing.T) {
	var out strings.Builder
	err := XML(&out, "<root>\n<a></a>\n<b></b>\n</root>")
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

func TestFormatXMLPreservesSpaceWhitespaceBetweenElements(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root> <a></a> </root>`)
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}

	const want = `<root> <a></a> </root>`
	if out.String() != want {
		t.Fatalf("XML() =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestFormatXMLPreservesXMLSpace(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root xml:space="preserve"> <a> x </a> </root>`)
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<root xml:space="preserve"> <a> x </a> </root>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLUsesLexicalXMLSpaceBeforeNamespaceAdmission(t *testing.T) {
	var out strings.Builder
	err := XML(&out, "<root xml:space=\"preserve\">\n  <a/>\n</root>")
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	const want = "<root xml:space=\"preserve\">&#xA;  <a></a>&#xA;</root>"
	if out.String() != want {
		t.Fatalf("XML() = %q, want %q", out.String(), want)
	}
}

func TestFormatXMLEscapesAttributeWhitespace(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root a="x&#10;y&#13;z&#9;w"/>`)
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<root a="x&#10;y&#13;z&#9;w"></root>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLEscapesAttributePredefinedEntities(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root a="&amp;&lt;&quot;"/>`)
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<root a="&amp;&lt;&quot;"></root>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLPreservesNamespaceDeclarations(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<?xml version="1.0"?>
<x:books xmlns:x="urn:books"><book id="bk001"/></x:books>`)
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
	err := XML(&out, "\xef\xbb\xbf<root/>")
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
		err = XML(&out, "<root>a\rb</root>")
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

func TestFormatXMLNormalizesCRLFTextWithoutDuplicatingCR(t *testing.T) {
	var out strings.Builder
	if err := XML(&out, "<root>a\r\nb</root>"); err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	const want = "<root>a&#xA;b</root>"
	if out.String() != want {
		t.Fatalf("XML() = %q, want %q", out.String(), want)
	}
	var decoded struct {
		Text string `xml:",chardata"`
	}
	if err := xml.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("formatted XML decode error = %v", err)
	}
	if decoded.Text != "a\nb" {
		t.Fatalf("decoded text = %q, want %q", decoded.Text, "a\nb")
	}
}

func TestFormatXMLNormalizesCDATALineEndings(t *testing.T) {
	var out strings.Builder
	err := XML(&out, "<root><![CDATA[a\rb]]></root>")
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != "<root><![CDATA[a\nb]]></root>" {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLPreservesLargeCDATASourceSpans(t *testing.T) {
	content := strings.Repeat("x", 64*1024+17)
	input := "<root><![CDATA[" + content + "]]></root>"
	var out strings.Builder
	if err := XML(&out, input); err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	want := "<root><![CDATA[" + content + "]]></root>"
	if out.String() != want {
		t.Fatalf("XML() length = %d, want %d", out.Len(), len(want))
	}
}

func TestFormatXMLDecodesEntityAtInputBufferBoundary(t *testing.T) {
	value := strings.Repeat("x", 64*1024-32) + "&amp;y"
	input := `<root value="` + value + `"/>`
	var out strings.Builder
	if err := XML(&out, input); err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	want := `<root value="` + strings.Repeat("x", 64*1024-32) + `&amp;y"></root>`
	if out.String() != want {
		t.Fatalf("XML() output mismatch at entity boundary")
	}
}

func TestFormatXMLNormalizesCommentAndProcessingInstructionLineEndings(t *testing.T) {
	var out strings.Builder
	err := XML(&out, "<root><!--a\rb--><?p a\r\nb?></root>")
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != "<root><!--a\nb--><?p a\nb?></root>" {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLPreservesProcessingInstructions(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<?xml version="1.0"?><?xml-stylesheet type="text/xsl" href="style.xsl"?><root><?pi data?><v>1</v></root><?tail?>`)
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
			err := XML(&out, tt.input)
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
	err := XML(&out, `<root a="1" a="2"/>`)
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "duplicate attribute") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLRejectsExpandedDuplicateAttributes(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root xmlns:a="urn:x" xmlns:b="urn:x" a:id="1" b:id="2"/>`)
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "duplicate attribute {urn:x}id") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLRejectsDuplicateNamespaceDeclarations(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root xmlns:a="urn:x" xmlns:a="urn:y"/>`)
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
	err := XML(&out, input.String())
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "duplicate attribute a39") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLRejectsUnboundAttributePrefix(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root p:id="1"/>`)
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
			err := XML(&out, input)
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
			err := XML(&out, tt.input)
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
	err := XML(&out, `<root/>text`)
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
			err := XML(&out, tt.input)
			if err == nil {
				t.Fatal("XML() succeeded")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("XML() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestFormatXMLRejectsTruncatedTrailingMarkup(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{`</`, `</root `, `<next `} {
		t.Run(suffix, func(t *testing.T) {
			t.Parallel()
			var out strings.Builder
			err := XML(&out, `<root/>`+suffix)
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != xsderrors.CodeFormatXML {
				t.Fatalf("XML() error = %v, want %q", err, xsderrors.CodeFormatXML)
			}
			if out.Len() != 0 {
				t.Fatalf("XML() wrote %q before rejecting input", out.String())
			}
		})
	}
}

func TestFormatXMLWithOptionsLimitsNodes(t *testing.T) {
	var out strings.Builder
	err := XMLWithOptions(&out, `<root><a/><b/></root>`, Options{MaxNodes: 2})
	if err == nil {
		t.Fatal("XMLWithOptions() succeeded")
	}
	if diagnostic, ok := errors.AsType[*xsderrors.Error](err); !ok || diagnostic == nil {
		t.Fatalf("XMLWithOptions() error type = %T, want *xsderrors.Error", err)
	}
	if !strings.Contains(err.Error(), "XML node limit exceeded") {
		t.Fatalf("XMLWithOptions() error = %v", err)
	}
}

func TestFormatXMLWithOptionsAllowsTokenAtLimit(t *testing.T) {
	var out strings.Builder
	err := XMLWithOptions(&out, `<?pi abc?><r/>`, Options{MaxTokenBytes: 5})
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
	err := XMLWithOptions(&out, input, Options{MaxInputBytes: int64(len(input))})
	if err != nil {
		t.Fatalf("XMLWithOptions() error = %v", err)
	}
	if out.String() != "<r></r>" {
		t.Fatalf("XMLWithOptions() = %q", out.String())
	}
}

func TestFormatXMLWithOptionsRejectsOutputBytesAfterPartialWrite(t *testing.T) {
	var out strings.Builder
	err := XMLWithOptions(&out, `<root><item/></root>`, Options{MaxOutputBytes: 8})
	if err == nil {
		t.Fatal("XMLWithOptions() succeeded")
	}
	if diagnostic, ok := errors.AsType[*xsderrors.Error](err); !ok || diagnostic == nil {
		t.Fatalf("XMLWithOptions() error type = %T, want *xsderrors.Error", err)
	}
	if !errors.Is(err, errFormatOutputLimit) {
		t.Fatalf("XMLWithOptions() error = %v, want %v", err, errFormatOutputLimit)
	}
	if out.Len() > 8 {
		t.Fatalf("output len = %d, want <= 8", out.Len())
	}
}

func TestFormatXMLRejectsShortWriteWithoutWriterError(t *testing.T) {
	err := XML(shortNilWriter{}, `<root/>`)
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("XML() error = %v, want %v", err, io.ErrShortWrite)
	}
}

func TestFormatXMLRejectsInvalidWriterCounts(t *testing.T) {
	writeErr := errors.New("write failed")
	tests := []struct {
		name      string
		write     formatWriterFunc
		wantCause bool
	}{
		{
			name:  "negative",
			write: func([]byte) (int, error) { return -1, nil },
		},
		{
			name:  "oversized",
			write: func(p []byte) (int, error) { return len(p) + 1, nil },
		},
		{
			name:      "negative with error",
			write:     func([]byte) (int, error) { return -1, writeErr },
			wantCause: true,
		},
		{
			name:      "oversized with error",
			write:     func(p []byte) (int, error) { return len(p) + 1, writeErr },
			wantCause: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := XML(test.write, `<root/>`)
			if !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("XML() error = %v, want %v", err, io.ErrShortWrite)
			}
			if errors.Is(err, writeErr) != test.wantCause {
				t.Fatalf("XML() error = %v, write cause present = %t, want %t", err, errors.Is(err, writeErr), test.wantCause)
			}
		})
	}
}

func TestFormatXMLWithOptionsRejectsInputBytesAfterSniff(t *testing.T) {
	var out strings.Builder
	input := `<r/>`
	err := XMLWithOptions(&out, input+"X", Options{MaxInputBytes: int64(len(input))})
	if err == nil {
		t.Fatal("XMLWithOptions() succeeded")
	}
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("XMLWithOptions() error type = %T, want *xsderrors.Error", err)
	}
	if xerr.Code() != xsderrors.CodeFormatLimit {
		t.Fatalf("XMLWithOptions() code = %q, want %q", xerr.Code(), xsderrors.CodeFormatLimit)
	}
	if !xmlstream.IsInputLimit(err) {
		t.Fatalf("XMLWithOptions() error = %v, want input limit", err)
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
			err := XMLWithOptions(&out, `<root/>`, tt.opts)
			if err == nil {
				t.Fatal("XMLWithOptions() succeeded")
			}
			if diagnostic, ok := errors.AsType[*xsderrors.Error](err); !ok || diagnostic == nil {
				t.Fatalf("XMLWithOptions() error type = %T, want *xsderrors.Error", err)
			}
		})
	}
}

func TestNormalizeFormatOptionsUsesFiniteDefaults(t *testing.T) {
	t.Parallel()
	got, err := normalizeFormatOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := formatOptions{
		maxDepth:       maxFormatDepth,
		maxNodes:       defaultMaxFormatNodes,
		maxInputBytes:  defaultMaxFormatInputBytes,
		maxOutputBytes: defaultMaxFormatOutputBytes,
		maxTokenBytes:  defaultMaxFormatTokenBytes,
	}
	if got != want {
		t.Fatalf("normalizeFormatOptions() = %+v, want %+v", got, want)
	}
}

func TestFormatXMLWithOptionsRejectsNilWriterAndEmptyInput(t *testing.T) {
	var out strings.Builder
	if err := XMLWithOptions(nil, `<root/>`, Options{}); err == nil {
		t.Fatal("XMLWithOptions() accepted nil writer")
	}
	if err := XMLWithOptions(&out, "", Options{}); err == nil {
		t.Fatal("XMLWithOptions() accepted empty input")
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
	err := XML(&out, input.String())
	if err == nil {
		t.Fatal("XML() succeeded")
	}
	if !strings.Contains(err.Error(), "XML nesting exceeds") {
		t.Fatalf("XML() error = %v", err)
	}
}

func TestFormatXMLReportsLine(t *testing.T) {
	var out strings.Builder
	err := XML(&out, "<root>\n  <a></root>")
	if err == nil {
		t.Fatal("XML() succeeded")
	}

	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("XML() error type = %T, want *xsderrors.Error", err)
	}
	if xerr.Line() != 2 {
		t.Fatalf("Line = %d, want 2", xerr.Line())
	}
}

func TestFormatXMLUsesBufferedCharacterFailurePosition(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	err := XML(&out, "<root>\nabcdefgh\x01</root>")
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("XML() error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Line() != 2 || xerr.Column() != 9 {
		t.Fatalf("XML() location = %d:%d, want 2:9", xerr.Line(), xerr.Column())
	}
}

func TestFormatXMLPreservesText(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root>  keep  </root>`)
	if err != nil {
		t.Fatalf("XML() error = %v", err)
	}
	if out.String() != `<root>  keep  </root>` {
		t.Fatalf("XML() = %q", out.String())
	}
}

func TestFormatXMLPreservesComments(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<root><!-- note --><v>1</v></root>`)
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
			err := XML(&out, input)
			if err != nil {
				t.Fatalf("XML() error = %v", err)
			}
			if out.String() != input {
				t.Fatalf("XML() = %q, want %q", out.String(), input)
			}
		})
	}
}

func TestFormatXMLRejectsReferencesOutsideRoot(t *testing.T) {
	for _, input := range []string{`&#32;<root/>`, `<root/>&#x20;`, " \t&#9;\n<root/>"} {
		var out strings.Builder
		err := XML(&out, input)
		diagnostic, ok := errors.AsType[*xsderrors.Error](err)
		if !ok || diagnostic.Code() != xsderrors.CodeFormatXML || out.Len() != 0 {
			t.Fatalf("XML(%q) = %v, output %q; want format.xml and no output", input, err, out.String())
		}
	}
}

func TestFormatXMLPreservesInnerReferences(t *testing.T) {
	var out strings.Builder
	err := XML(&out, " \r\n<root>&#32;x&amp;y<![CDATA[&#32;]]></root>\t")
	const want = `<root> x&amp;y<![CDATA[&#32;]]></root>`
	if err != nil || out.String() != want {
		t.Fatalf("XML() = %v, output %q; want %q", err, out.String(), want)
	}
}

func TestFormatXMLRejectsEmptyEntityReferenceWithoutOutput(t *testing.T) {
	var out strings.Builder
	err := XML(&out, `<r>&;</r>`)
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Code() != xsderrors.CodeFormatXML {
		t.Fatalf("XML() error = %v, want format.xml", err)
	}
	if err == nil {
		t.Fatal("XML() accepted empty entity reference")
	}
	if out.Len() != 0 {
		t.Fatalf("XML() wrote %q before rejecting malformed entity", out.String())
	}
}

package xmlstream

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestReaderCharDataEntities(t *testing.T) {
	input := `<root>a&amp;b&lt;c&gt;d&apos;e&quot;f</root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("char data error = %v", err)
	}
	if ev.Kind != EventCharData {
		t.Fatalf("char data kind = %v, want %v", ev.Kind, EventCharData)
	}
	if got := string(ev.Text); got != `a&b<c>d'e"f` {
		t.Fatalf("char data = %q, want a&b<c>d'e\"f", got)
	}
}

func TestReaderNumericCharRefs(t *testing.T) {
	input := `<root>&#65;&#x42;</root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("char data error = %v", err)
	}
	if got := string(ev.Text); got != "AB" {
		t.Fatalf("char data = %q, want AB", got)
	}
}

func TestReaderCharDataBufferGrowth(t *testing.T) {
	payload := strings.Repeat("a", 300) + "&amp;"
	input := "<root>" + payload + "</root>"
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("char data error = %v", err)
	}
	want := strings.Repeat("a", 300) + "&"
	if got := string(ev.Text); got != want {
		t.Fatalf("char data len = %d, want %d", len(got), len(want))
	}
}

func TestReaderAttrEntities(t *testing.T) {
	input := `<root attr="a&amp;b"/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if got, ok := ev.Attr("", "attr"); !ok || string(got) != "a&b" {
		t.Fatalf("attr = %q, ok=%v, want a&b, true", string(got), ok)
	}
}

func TestReaderMultipleEscapedAttributes(t *testing.T) {
	input := `<root a="&amp;" b="&lt;" c="&gt;"/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if v, ok := ev.Attr("", "a"); !ok || string(v) != "&" {
		t.Fatalf("a = %q, ok=%v, want &, true", string(v), ok)
	}
	if v, ok := ev.Attr("", "b"); !ok || string(v) != "<" {
		t.Fatalf("b = %q, ok=%v, want <, true", string(v), ok)
	}
	if v, ok := ev.Attr("", "c"); !ok || string(v) != ">" {
		t.Fatalf("c = %q, ok=%v, want >, true", string(v), ok)
	}
}

func TestReaderCharDataInvalidEntity(t *testing.T) {
	input := `<root>&bad;</root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	_, err = r.Next()
	if err == nil {
		t.Fatalf("invalid entity error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("invalid entity error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestCharDataTruncatedNumericRef(t *testing.T) {
	input := `<root>&#</root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	_, err = r.Next()
	if err == nil {
		t.Fatalf("truncated ref error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("truncated ref error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestReaderInvalidEntitySyntaxError(t *testing.T) {
	input := `<root attr="a&bad;"/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	_, err = r.Next()
	if err == nil {
		t.Fatalf("invalid entity error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("invalid entity error type = %T, want *xmltext.SyntaxError", err)
	}
	if syntax.Line == 0 || syntax.Column == 0 {
		t.Fatalf("syntax location = %d:%d, want non-zero", syntax.Line, syntax.Column)
	}
}

func TestEmptyAttrValueWithUnescapeFlag(t *testing.T) {
	input := `<root attr=""/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	val, ok := ev.Attr("", "attr")
	if !ok {
		t.Fatalf("attr not found")
	}
	if len(val) != 0 {
		t.Fatalf("attr value len = %d, want 0", len(val))
	}
}

func TestAttrValueBytesEmptyAfterUnescape(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	value, err := r.attrValueBytes(nil, true)
	if err != nil {
		t.Fatalf("attrValueBytes error = %v", err)
	}
	if value != nil {
		t.Fatalf("attrValueBytes value = %v, want nil", value)
	}
}

func TestAttrValueBytesResetOnError(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	prefix := []byte("prefix")
	r.valueBuf = append(r.valueBuf, prefix...)
	if _, err = r.attrValueBytes([]byte("&bad;"), true); err == nil {
		t.Fatalf("attrValueBytes error = nil, want error")
	}
	if len(r.valueBuf) != len(prefix) {
		t.Fatalf("valueBuf len = %d, want %d", len(r.valueBuf), len(prefix))
	}
	if !bytes.Equal(r.valueBuf, prefix) {
		t.Fatalf("valueBuf = %q, want %q", r.valueBuf, prefix)
	}
}

func TestTextBytesEmptyAfterUnescape(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	value, err := r.textBytes(nil, true)
	if err != nil {
		t.Fatalf("textBytes error = %v", err)
	}
	if value != nil {
		t.Fatalf("textBytes value = %v, want nil", value)
	}
}

func TestTextBytesResetOnError(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	prefix := []byte("prefix")
	r.valueBuf = append(r.valueBuf, prefix...)
	if _, err = r.textBytes([]byte("&bad;"), true); err == nil {
		t.Fatalf("textBytes error = nil, want error")
	}
	if len(r.valueBuf) != len(prefix) {
		t.Fatalf("valueBuf len = %d, want %d", len(r.valueBuf), len(prefix))
	}
	if !bytes.Equal(r.valueBuf, prefix) {
		t.Fatalf("valueBuf = %q, want %q", r.valueBuf, prefix)
	}
}

func TestNamespaceValueStringEmptyAfterUnescape(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	value, err := r.namespaceValueString(nil, true)
	if err != nil {
		t.Fatalf("namespaceValueString error = %v", err)
	}
	if value != "" {
		t.Fatalf("namespaceValueString = %q, want empty", value)
	}
}

func TestNamespaceValueStringGrowth(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	r.nsBuf = make([]byte, 0, 1)
	input := []byte("a&amp;" + strings.Repeat("b", 64))
	value, err := r.namespaceValueString(input, true)
	if err != nil {
		t.Fatalf("namespaceValueString error = %v", err)
	}
	want := "a&" + strings.Repeat("b", 64)
	if value != want {
		t.Fatalf("namespaceValueString = %q, want %q", value, want)
	}
}

func TestUnescapeIntoBufferZeroCap(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	out, err := unescapeIntoBuffer(r.dec, nil, 0, []byte("a&amp;b"))
	if err != nil {
		t.Fatalf("unescapeIntoBuffer error = %v", err)
	}
	if got := string(out); got != "a&b" {
		t.Fatalf("unescapeIntoBuffer = %q, want a&b", got)
	}
}

func TestUnescapeIntoBufferMinCap(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	buf := make([]byte, 0, 1)
	input := []byte("a&amp;" + strings.Repeat("b", 64))
	out, err := unescapeIntoBuffer(r.dec, buf, 0, input)
	if err != nil {
		t.Fatalf("unescapeIntoBuffer error = %v", err)
	}
	want := "a&" + strings.Repeat("b", 64)
	if got := string(out); got != want {
		t.Fatalf("unescapeIntoBuffer = %q, want %q", got, want)
	}
}

func TestNamespaceValueStringEmpty(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	value, err := r.namespaceValueString(nil, false)
	if err != nil {
		t.Fatalf("namespaceValueString error = %v", err)
	}
	if value != "" {
		t.Fatalf("namespaceValueString = %q, want empty", value)
	}
}

func TestNamespaceValueStringResetOnError(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	prefix := []byte("prefix")
	r.nsBuf = append(r.nsBuf, prefix...)
	if _, err = r.namespaceValueString([]byte("&bad;"), true); err == nil {
		t.Fatalf("namespaceValueString error = nil, want error")
	}
	if len(r.nsBuf) != len(prefix) {
		t.Fatalf("nsBuf len = %d, want %d", len(r.nsBuf), len(prefix))
	}
	if !bytes.Equal(r.nsBuf, prefix) {
		t.Fatalf("nsBuf = %q, want %q", r.nsBuf, prefix)
	}
}

package xmlstream

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestStringReaderNil(t *testing.T) {
	if _, err := NewStringReader(nil); !errors.Is(err, errNilReader) {
		t.Fatalf("NewStringReader nil error = %v, want %v", err, errNilReader)
	}

	var r *StringReader
	if _, err := r.Next(); !errors.Is(err, errNilReader) {
		t.Fatalf("Next nil error = %v, want %v", err, errNilReader)
	}
	if err := r.Reset(strings.NewReader("<root/>")); !errors.Is(err, errNilReader) {
		t.Fatalf("Reset nil error = %v, want %v", err, errNilReader)
	}
	if err := r.SkipSubtree(); !errors.Is(err, errNilReader) {
		t.Fatalf("SkipSubtree nil error = %v, want %v", err, errNilReader)
	}
	if line, col := r.CurrentPos(); line != 0 || col != 0 {
		t.Fatalf("CurrentPos nil = %d:%d, want 0:0", line, col)
	}
	if ns, ok := r.LookupNamespace(""); ok || ns != "" {
		t.Fatalf("LookupNamespace nil = %q, ok=%v, want empty, false", ns, ok)
	}
	if ns, ok := r.LookupNamespaceAt("", 0); ok || ns != "" {
		t.Fatalf("LookupNamespaceAt nil = %q, ok=%v, want empty, false", ns, ok)
	}
}

func TestStringReaderResetNilSource(t *testing.T) {
	r, err := NewStringReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	if err := r.Reset(nil); !errors.Is(err, errNilReader) {
		t.Fatalf("Reset nil src error = %v, want %v", err, errNilReader)
	}
}

func TestStringReaderResetIDs(t *testing.T) {
	r, err := NewStringReader(strings.NewReader("<root><a/></root>"))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.ID != 0 {
		t.Fatalf("root ID = %d, want 0", ev.ID)
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("a start error = %v", err)
	}
	if ev.ID != 1 {
		t.Fatalf("a ID = %d, want 1", ev.ID)
	}

	if err = r.Reset(strings.NewReader("<root/>")); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("root start after reset error = %v", err)
	}
	if ev.ID != 0 {
		t.Fatalf("root ID after reset = %d, want 0", ev.ID)
	}
}

func TestStringReaderResetOptions(t *testing.T) {
	r, err := NewStringReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	if err = r.Reset(strings.NewReader("<root/>"), TrackLineColumn(false)); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Line != 0 || ev.Column != 0 {
		t.Fatalf("line/column = %d:%d, want 0:0", ev.Line, ev.Column)
	}
}

func TestStringReaderResetNilDecoder(t *testing.T) {
	r, err := NewStringReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	r.dec = nil
	if err = r.Reset(strings.NewReader("<root/>")); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "root" {
		t.Fatalf("event = %v %s, want root start", ev.Kind, ev.Name.String())
	}
}

func TestStringReaderResetClearsLastWasStart(t *testing.T) {
	r, err := NewStringReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if err = r.Reset(strings.NewReader("<root/>")); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	if err = r.SkipSubtree(); !errors.Is(err, errNoStartElement) {
		t.Fatalf("SkipSubtree error = %v, want %v", err, errNoStartElement)
	}
}

func TestStringReaderInputOffset(t *testing.T) {
	r, err := NewStringReader(strings.NewReader("<root><child/></root>"))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.Kind != EventStartElement {
		t.Fatalf("event = %v, want start", ev.Kind)
	}
	if offset := r.InputOffset(); offset == 0 {
		t.Fatalf("InputOffset = 0, want > 0")
	}
}

func TestStringReaderNamespaceLookup(t *testing.T) {
	xmlData := `<root xmlns="urn:root" xmlns:r="urn:root2">
<child xmlns:p="urn:child" p:attr="v"/></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}

	event, err := nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if event.Name.Namespace != "urn:root" {
		t.Fatalf("root namespace = %q, want urn:root", event.Name.Namespace)
	}
	if event.ScopeDepth != 0 {
		t.Fatalf("root scope depth = %d, want 0", event.ScopeDepth)
	}
	if ns, ok := dec.LookupNamespaceAt("", event.ScopeDepth); !ok || ns != "urn:root" {
		t.Fatalf("default namespace = %q (ok=%v), want urn:root", ns, ok)
	}

	event, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start child error = %v", err)
	}
	if event.ScopeDepth != 1 {
		t.Fatalf("child scope depth = %d, want 1", event.ScopeDepth)
	}
	if ns, ok := dec.LookupNamespaceAt("p", event.ScopeDepth); !ok || ns != "urn:child" {
		t.Fatalf("prefix p = %q (ok=%v), want urn:child", ns, ok)
	}
	if ns, ok := dec.LookupNamespaceAt("", event.ScopeDepth); !ok || ns != "urn:root" {
		t.Fatalf("default namespace = %q (ok=%v), want urn:root", ns, ok)
	}

	foundXMLNS := false
	for _, attr := range event.Attrs {
		if attr.NamespaceURI() == XMLNSNamespace && attr.LocalName() == "p" {
			foundXMLNS = true
			break
		}
	}
	if !foundXMLNS {
		t.Fatalf("expected xmlns:p attribute in child attributes")
	}
}

func TestStringReaderNamespaceShadowing(t *testing.T) {
	input := `<root xmlns:p="urn:one"><child xmlns:p="urn:two"><p:inner/></child><p:after/></root>`
	r, err := NewStringReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next() // child
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	if ev.Name.Namespace != "" {
		t.Fatalf("child namespace = %q, want empty", ev.Name.Namespace)
	}
	ev, err = r.Next() // inner
	if err != nil {
		t.Fatalf("inner start error = %v", err)
	}
	if ev.Name.Namespace != "urn:two" {
		t.Fatalf("inner namespace = %q, want urn:two", ev.Name.Namespace)
	}
	if _, err = r.Next(); err != nil { // inner end
		t.Fatalf("inner end error = %v", err)
	}
	if _, err = r.Next(); err != nil { // child end
		t.Fatalf("child end error = %v", err)
	}
	ev, err = r.Next() // after
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if ev.Name.Namespace != "urn:one" {
		t.Fatalf("after namespace = %q, want urn:one", ev.Name.Namespace)
	}
}

func TestStringReaderNamespaceDepthShadowing(t *testing.T) {
	const levels = 20
	var b strings.Builder
	b.WriteString(`<p:e0 xmlns:p="urn:0">`)
	for i := 1; i < levels; i++ {
		_, _ = b.WriteString(`<p:e`)
		_, _ = b.WriteString(strconv.Itoa(i))
		_, _ = b.WriteString(` xmlns:p="urn:`)
		_, _ = b.WriteString(strconv.Itoa(i))
		_, _ = b.WriteString(`">`)
	}
	for i := levels - 1; i >= 1; i-- {
		_, _ = b.WriteString(`</p:e`)
		_, _ = b.WriteString(strconv.Itoa(i))
		_, _ = b.WriteString(`>`)
	}
	b.WriteString(`<p:after/>`)
	b.WriteString(`</p:e0>`)
	r, err := NewStringReader(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	expect := make([]string, 0, levels+1)
	for i := 0; i < levels; i++ {
		expect = append(expect, "urn:"+strconv.Itoa(i))
	}
	expect = append(expect, "urn:0")
	var seen int
	for {
		ev, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next error = %v", err)
		}
		if ev.Kind != EventStartElement {
			continue
		}
		if ev.Name.Namespace != expect[seen] {
			t.Fatalf("namespace %d = %q, want %q", seen, ev.Name.Namespace, expect[seen])
		}
		seen++
		if seen == len(expect) {
			break
		}
	}
	if seen != len(expect) {
		t.Fatalf("seen %d namespaces, want %d", seen, len(expect))
	}
}

func TestStringReaderVeryDeepNesting(t *testing.T) {
	const depth = 1200
	var b strings.Builder
	b.WriteString("<root>")
	for i := 0; i < depth; i++ {
		b.WriteString("<a>")
	}
	for i := 0; i < depth; i++ {
		b.WriteString("</a>")
	}
	b.WriteString("</root>")
	r, err := NewStringReader(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	var starts int
	for {
		ev, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next error = %v", err)
		}
		if ev.Kind == EventStartElement {
			starts++
		}
	}
	if starts != depth+1 {
		t.Fatalf("start elements = %d, want %d", starts, depth+1)
	}
}

func TestStringReaderLookupNamespace(t *testing.T) {
	input := `<root xmlns="urn:root"><child xmlns="urn:child"/></root>`
	r, err := NewStringReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next() // child
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	if ns, ok := r.LookupNamespace(""); !ok || ns != "urn:child" {
		t.Fatalf("LookupNamespace = %q, ok=%v, want urn:child, true", ns, ok)
	}
	if ns, ok := r.LookupNamespaceAt("", ev.ScopeDepth-1); !ok || ns != "urn:root" {
		t.Fatalf("LookupNamespaceAt parent = %q, ok=%v, want urn:root, true", ns, ok)
	}
}

func TestStringReaderNamespacePrefixedError(t *testing.T) {
	input := `<root xmlns:a="&bad;"><a:child/></root>`
	r, err := NewStringReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	_, err = r.Next()
	if err == nil {
		t.Fatalf("prefixed namespace error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("prefixed namespace error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestStringReaderNamespaceDefaultError(t *testing.T) {
	input := `<root xmlns="&bad;"><child/></root>`
	r, err := NewStringReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	_, err = r.Next()
	if err == nil {
		t.Fatalf("default namespace error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("default namespace error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestStringReaderDefaultNamespaceUndeclare(t *testing.T) {
	xmlData := `<root xmlns="urn:root"><child xmlns=""><grand/></child></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}

	event, err := nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if ns, ok := dec.LookupNamespaceAt("", event.ScopeDepth); !ok || ns != "urn:root" {
		t.Fatalf("root default namespace = %q (ok=%v), want urn:root", ns, ok)
	}

	event, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start child error = %v", err)
	}
	if ns, ok := dec.LookupNamespaceAt("", event.ScopeDepth); !ok || ns != "" {
		t.Fatalf("child default namespace = %q (ok=%v), want empty", ns, ok)
	}

	event, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start grand error = %v", err)
	}
	if ns, ok := dec.LookupNamespaceAt("", event.ScopeDepth); !ok || ns != "" {
		t.Fatalf("grand default namespace = %q (ok=%v), want empty", ns, ok)
	}
}

func TestStringReaderDefaultNamespaceUndeclareRedeclare(t *testing.T) {
	xmlData := `<root xmlns="a"><child xmlns=""><grand xmlns="b"/></child></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	event, err := nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if event.Name.Namespace != "a" {
		t.Fatalf("root namespace = %q, want a", event.Name.Namespace)
	}
	event, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start child error = %v", err)
	}
	if event.Name.Namespace != "" {
		t.Fatalf("child namespace = %q, want empty", event.Name.Namespace)
	}
	event, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start grand error = %v", err)
	}
	if event.Name.Namespace != "b" {
		t.Fatalf("grand namespace = %q, want b", event.Name.Namespace)
	}
}

func TestStringReaderSelfClosingNamespace(t *testing.T) {
	xmlData := `<p:root xmlns:p="urn:p"/>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	ev, err := dec.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Namespace != "urn:p" || ev.Name.Local != "root" {
		t.Fatalf("start = %v %s, want {urn:p}root", ev.Kind, ev.Name.String())
	}
	ev, err = dec.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Kind != EventEndElement || ev.Name.Local != "root" {
		t.Fatalf("end = %v %s, want root end", ev.Kind, ev.Name.String())
	}
}

func TestStringReaderSkipSubtree(t *testing.T) {
	xmlData := `<root xmlns:a="urn:a"><skip xmlns:b="urn:b"><inner/></skip><after/></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}

	event, err := nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if event.Name.Local != "root" {
		t.Fatalf("first element = %q, want root", event.Name.Local)
	}

	event, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start skip error = %v", err)
	}
	if event.Name.Local != "skip" {
		t.Fatalf("second element = %q, want skip", event.Name.Local)
	}
	if skipErr := dec.SkipSubtree(); skipErr != nil {
		t.Fatalf("SkipSubtree error = %v", skipErr)
	}

	event, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start after error = %v", err)
	}
	if event.Name.Local != "after" {
		t.Fatalf("after element = %q, want after", event.Name.Local)
	}
	if ns, ok := dec.LookupNamespaceAt("b", event.ScopeDepth); ok || ns != "" {
		t.Fatalf("prefix b = %q (ok=%v), want empty, false", ns, ok)
	}
	if ns, ok := dec.LookupNamespaceAt("a", event.ScopeDepth); !ok || ns != "urn:a" {
		t.Fatalf("prefix a = %q (ok=%v), want urn:a", ns, ok)
	}
}

func TestStringReaderSkipSubtreeEmptyElement(t *testing.T) {
	xmlData := `<root><skip/><after/></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	_, err = nextStringStartEvent(dec) // root
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	event, err := nextStringStartEvent(dec) // skip
	if err != nil {
		t.Fatalf("next start skip error = %v", err)
	}
	if event.Name.Local != "skip" {
		t.Fatalf("skip element = %q, want skip", event.Name.Local)
	}
	if skipErr := dec.SkipSubtree(); skipErr != nil {
		t.Fatalf("SkipSubtree error = %v", skipErr)
	}
	event, err = nextStringStartEvent(dec) // after
	if err != nil {
		t.Fatalf("next start after error = %v", err)
	}
	if event.Name.Local != "after" {
		t.Fatalf("after element = %q, want after", event.Name.Local)
	}
}

func TestStringReaderSkipSubtreeMalformed(t *testing.T) {
	xmlData := `<root><skip>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	if _, err = nextStringStartEvent(dec); err != nil { // root
		t.Fatalf("next start root error = %v", err)
	}
	if _, err = nextStringStartEvent(dec); err != nil { // skip
		t.Fatalf("next start skip error = %v", err)
	}
	if err = dec.SkipSubtree(); err == nil {
		t.Fatalf("SkipSubtree error = nil, want error")
	}
}

func TestStringReaderPopElementNameEmpty(t *testing.T) {
	r := &StringReader{}
	if _, err := r.popElementName(); err == nil {
		t.Fatalf("popElementName error = nil, want error")
	}
}

func TestStringReaderSkipSubtreeNoStart(t *testing.T) {
	xmlData := `<root><a/><b/></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	if skipErr := dec.SkipSubtree(); !errors.Is(skipErr, errNoStartElement) {
		t.Fatalf("SkipSubtree error = %v, want %v", skipErr, errNoStartElement)
	}

	_, err = nextStringStartEvent(dec) // root
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	_, err = nextStringStartEvent(dec) // a
	if err != nil {
		t.Fatalf("next start a error = %v", err)
	}
	if skipErr := dec.SkipSubtree(); skipErr != nil {
		t.Fatalf("SkipSubtree a error = %v", skipErr)
	}
	if skipErr := dec.SkipSubtree(); !errors.Is(skipErr, errNoStartElement) {
		t.Fatalf("second SkipSubtree error = %v, want %v", skipErr, errNoStartElement)
	}
}

func TestStringReaderAttrValueCopy(t *testing.T) {
	xmlData := `<doc><root attr="foo"/><next attr="bar"/></doc>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}

	_, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start doc error = %v", err)
	}

	event, err := nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if len(event.Attrs) != 1 {
		t.Fatalf("root attr count = %d, want 1", len(event.Attrs))
	}
	value := event.Attrs[0].Value()

	event, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start next error = %v", err)
	}
	if len(event.Attrs) != 1 {
		t.Fatalf("next attr count = %d, want 1", len(event.Attrs))
	}
	if value != "foo" {
		t.Fatalf("root attr value = %q, want foo", value)
	}
}

func TestStringReaderAttrValueUnescape(t *testing.T) {
	xmlData := `<root attr="a&amp;b"/>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	event, err := nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if len(event.Attrs) != 1 {
		t.Fatalf("attr count = %d, want 1", len(event.Attrs))
	}
	if value := event.Attrs[0].Value(); value != "a&b" {
		t.Fatalf("attr value = %q, want a&b", value)
	}
}

func TestStringReaderEmptyAttrValue(t *testing.T) {
	xmlData := `<root attr=""/>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	event, err := nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if len(event.Attrs) != 1 {
		t.Fatalf("attr count = %d, want 1", len(event.Attrs))
	}
	if value := event.Attrs[0].Value(); value != "" {
		t.Fatalf("attr value = %q, want empty", value)
	}
}

func TestStringReaderAttrNamespacesSameLocal(t *testing.T) {
	xmlData := `<root xmlns:a="urn:a" xmlns:b="urn:b" a:attr="1" b:attr="2"/>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	event, err := nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if len(event.Attrs) != 4 {
		t.Fatalf("attr count = %d, want 4", len(event.Attrs))
	}
	var aValue string
	var bValue string
	for _, attr := range event.Attrs {
		if attr.LocalName() != "attr" {
			continue
		}
		switch attr.NamespaceURI() {
		case "urn:a":
			aValue = attr.Value()
		case "urn:b":
			bValue = attr.Value()
		}
	}
	if aValue != "1" || bValue != "2" {
		t.Fatalf("attr values = %q/%q, want 1/2", aValue, bValue)
	}
}

func TestStringReaderAttrValueError(t *testing.T) {
	xmlData := `<root attr="&bad;"></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	_, err = dec.Next()
	if err == nil {
		t.Fatalf("attr value error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("attr value error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestStringReaderAttrValueLarge(t *testing.T) {
	value := strings.Repeat("x", 1024)
	xmlData := `<root attr="` + value + `"/>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	event, err := nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if len(event.Attrs) != 1 {
		t.Fatalf("attr count = %d, want 1", len(event.Attrs))
	}
	if got := event.Attrs[0].Value(); got != value {
		t.Fatalf("attr value len = %d, want %d", len(got), len(value))
	}
}

func TestStringReaderNamespaceValueLarge(t *testing.T) {
	value := "urn:" + strings.Repeat("long:", 300)
	xmlData := `<root xmlns="` + value + `"><child/></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	ev, err := nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if ns, ok := dec.LookupNamespaceAt("", ev.ScopeDepth); !ok || ns != value {
		t.Fatalf("default namespace len = %d, want %d", len(ns), len(value))
	}
}

func TestStringReaderEmptyNamespacePrefixDecl(t *testing.T) {
	xmlData := `<root xmlns:="urn:test"/>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	_, err = dec.Next()
	if err == nil {
		t.Fatalf("empty prefix error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("empty prefix error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestStringReaderTextUnescapeGrowth(t *testing.T) {
	escaped := strings.Repeat("&amp;", 300)
	xmlData := `<root>` + escaped + `</root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	_, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	var sb strings.Builder
	for {
		ev, err := dec.Next()
		if err != nil {
			t.Fatalf("Next error = %v", err)
		}
		if ev.Kind == EventCharData {
			_, _ = sb.Write(ev.Text)
			continue
		}
		if ev.Kind == EventEndElement {
			break
		}
	}
	want := strings.Repeat("&", 300)
	if got := sb.String(); got != want {
		t.Fatalf("text len = %d, want %d", len(got), len(want))
	}
}

func TestStringReaderMixedContentCDATA(t *testing.T) {
	xmlData := `<root>text<![CDATA[cdata]]>more</root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	_, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	var sb strings.Builder
	for {
		ev, err := dec.Next()
		if err != nil {
			t.Fatalf("Next error = %v", err)
		}
		if ev.Kind == EventCharData {
			_, _ = sb.Write(ev.Text)
			continue
		}
		if ev.Kind == EventEndElement {
			break
		}
	}
	if got := sb.String(); got != "textcdatamore" {
		t.Fatalf("text = %q, want %q", got, "textcdatamore")
	}
}

func TestStringReaderSkipsCommentsAndPI(t *testing.T) {
	input := `<!--comment--><?pi test?><root/>`
	dec, err := NewStringReader(strings.NewReader(input), EmitComments(true), EmitPI(true))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	ev, err := dec.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "root" {
		t.Fatalf("event = %v %s, want root start", ev.Kind, ev.Name.String())
	}
	for {
		if _, err = dec.Next(); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("Next error = %v", err)
		}
	}
}

func TestStringReaderSkipsDirectives(t *testing.T) {
	input := `<!DOCTYPE root><root/>`
	dec, err := NewStringReader(strings.NewReader(input), EmitDirectives(true))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	ev, err := dec.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "root" {
		t.Fatalf("event = %v %s, want root start", ev.Kind, ev.Name.String())
	}
}

func TestStringReaderCommentOnly(t *testing.T) {
	input := `<!--comment-->`
	dec, err := NewStringReader(strings.NewReader(input), EmitComments(true))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	_, err = dec.Next()
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("Next error = %v, want syntax error", err)
	}
}

func TestStringReaderCDATA(t *testing.T) {
	xmlData := `<root><![CDATA[text & <content>]]></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	_, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	ev, err := dec.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Kind != EventCharData {
		t.Fatalf("kind = %v, want %v", ev.Kind, EventCharData)
	}
	if got := string(ev.Text); got != "text & <content>" {
		t.Fatalf("text = %q, want %q", got, "text & <content>")
	}
}

func TestStringReaderEmptyElement(t *testing.T) {
	xmlData := `<root></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	_, err = nextStringStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	ev, err := dec.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Kind != EventEndElement {
		t.Fatalf("kind = %v, want %v", ev.Kind, EventEndElement)
	}
}

func TestStringReaderTrackLineColumnDisabled(t *testing.T) {
	dec, err := NewStringReader(strings.NewReader("<root/>"), TrackLineColumn(false))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	ev, err := dec.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Line != 0 || ev.Column != 0 {
		t.Fatalf("line/column = %d:%d, want 0:0", ev.Line, ev.Column)
	}
}

func TestStringReaderEOF(t *testing.T) {
	dec, err := NewStringReader(strings.NewReader(`<root/>`))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	for {
		_, err := dec.Next()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("Next error = %v", err)
		}
	}
}

func TestStringReaderUnboundPrefixElement(t *testing.T) {
	xmlData := `<root><p:child/></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	for {
		_, err := dec.Next()
		if err == nil {
			continue
		}
		var syntax *xmltext.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("Next error type = %T, want *xmltext.SyntaxError", err)
		}
		if !errors.Is(err, ErrUnboundPrefix) {
			t.Fatalf("Next error = %v, want unbound prefix", err)
		}
		return
	}
}

func TestStringReaderUnboundPrefixAttr(t *testing.T) {
	xmlData := `<root><child p:attr="v"/></root>`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	for {
		_, err := dec.Next()
		if err == nil {
			continue
		}
		var syntax *xmltext.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("Next error type = %T, want *xmltext.SyntaxError", err)
		}
		if !errors.Is(err, ErrUnboundPrefix) {
			t.Fatalf("Next error = %v, want unbound prefix", err)
		}
		return
	}
}

func TestStringReaderMalformedXML(t *testing.T) {
	xmlData := `<root><child`
	dec, err := NewStringReader(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStringReader error = %v", err)
	}
	for {
		_, err = dec.Next()
		if err != nil {
			break
		}
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("malformed XML error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestConcurrentStringReaderCreation(t *testing.T) {
	const goroutines = 8
	input := `<root><child/></root>`
	errs := make(chan error, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := NewStringReader(strings.NewReader(input))
			if err != nil {
				errs <- err
				return
			}
			for {
				if _, err = r.Next(); errors.Is(err, io.EOF) {
					break
				} else if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent reader error = %v", err)
	}
}

func nextStringStartEvent(dec *StringReader) (StringEvent, error) {
	for {
		event, err := dec.Next()
		if err != nil {
			return StringEvent{}, err
		}
		if event.Kind == EventStartElement {
			return event, nil
		}
	}
}

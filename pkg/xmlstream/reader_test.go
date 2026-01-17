package xmlstream

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestReaderNamespaceAndAttrs(t *testing.T) {
	input := `<root xmlns="urn:default" xmlns:x="urn:x" x:attr="v" plain="p"><x:child/></root>`

	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next root start error = %v", err)
	}
	if ev.Kind != EventStartElement {
		t.Fatalf("root kind = %v, want %v", ev.Kind, EventStartElement)
	}
	if ev.Name.Namespace != "urn:default" || ev.Name.Local != "root" {
		t.Fatalf("root name = %s, want {urn:default}root", ev.Name.String())
	}
	if ev.ScopeDepth != 0 {
		t.Fatalf("root scope depth = %d, want 0", ev.ScopeDepth)
	}
	if ns, ok := r.LookupNamespaceAt("x", ev.ScopeDepth); !ok || ns != "urn:x" {
		t.Fatalf("LookupNamespaceAt(x) = %q, ok=%v", ns, ok)
	}

	var seenPlain bool
	var seenX bool
	for _, attr := range ev.Attrs {
		switch {
		case attr.Name.Namespace == "" && attr.Name.Local == "plain":
			seenPlain = true
			if string(attr.Value) != "p" {
				t.Fatalf("plain attr = %q, want p", string(attr.Value))
			}
		case attr.Name.Namespace == "urn:x" && attr.Name.Local == "attr":
			seenX = true
			if string(attr.Value) != "v" {
				t.Fatalf("x:attr value = %q, want v", string(attr.Value))
			}
		case attr.Name.Namespace == XMLNSNamespace:
			t.Fatalf("xmlns attribute should not be exposed")
		}
	}
	if !seenPlain || !seenX {
		t.Fatalf("attrs seen plain=%v x=%v", seenPlain, seenX)
	}

	ev, err = r.Next()
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	if ev.Kind != EventStartElement {
		t.Fatalf("child kind = %v, want %v", ev.Kind, EventStartElement)
	}
	if ev.Name.Namespace != "urn:x" || ev.Name.Local != "child" {
		t.Fatalf("child name = %s, want {urn:x}child", ev.Name.String())
	}
}

func TestMixedContentElements(t *testing.T) {
	input := `<p>Hello <b>world</b> today!</p>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("p start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "p" {
		t.Fatalf("start = %v %s, want p start", ev.Kind, ev.Name.String())
	}
	var texts []string
	for {
		ev, err := r.Next()
		if err != nil {
			t.Fatalf("Next error = %v", err)
		}
		switch ev.Kind {
		case EventCharData:
			texts = append(texts, string(ev.Text))
		case EventEndElement:
			if ev.Name.Local == "p" {
				if len(texts) != 3 || texts[0] != "Hello " || texts[1] != "world" || texts[2] != " today!" {
					t.Fatalf("texts = %#v, want [\"Hello \" \"world\" \" today!\"]", texts)
				}
				return
			}
		}
	}
}

func TestNextRaw(t *testing.T) {
	input := `<root xmlns:x="urn:x" x:attr="v"><x:child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("NextRaw root error = %v", err)
	}
	if ev.Kind != EventStartElement {
		t.Fatalf("root kind = %v, want %v", ev.Kind, EventStartElement)
	}
	if string(ev.Name.Full) != "root" || string(ev.Name.Local) != "root" || len(ev.Name.Prefix) != 0 {
		t.Fatalf("root raw name = %q/%q/%q", ev.Name.Full, ev.Name.Prefix, ev.Name.Local)
	}
	if len(ev.Attrs) != 1 {
		t.Fatalf("attrs len = %d, want 1", len(ev.Attrs))
	}
	if string(ev.Attrs[0].Name.Full) != "x:attr" || string(ev.Attrs[0].Name.Prefix) != "x" || string(ev.Attrs[0].Name.Local) != "attr" {
		t.Fatalf("attr raw name = %q/%q/%q", ev.Attrs[0].Name.Full, ev.Attrs[0].Name.Prefix, ev.Attrs[0].Name.Local)
	}
	if string(ev.Attrs[0].Value) != "v" {
		t.Fatalf("attr value = %q, want v", string(ev.Attrs[0].Value))
	}

	ev, err = r.NextRaw()
	if err != nil {
		t.Fatalf("NextRaw child start error = %v", err)
	}
	if ev.Kind != EventStartElement || string(ev.Name.Full) != "x:child" {
		t.Fatalf("child start = %v %q", ev.Kind, ev.Name.Full)
	}

	ev, err = r.NextRaw()
	if err != nil {
		t.Fatalf("NextRaw child end error = %v", err)
	}
	if ev.Kind != EventEndElement || string(ev.Name.Full) != "x:child" {
		t.Fatalf("child end = %v %q", ev.Kind, ev.Name.Full)
	}
}

func TestNextRawMixedWithNext(t *testing.T) {
	input := `<root><child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	if _, err = r.NextRaw(); err != nil {
		t.Fatalf("NextRaw root error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next child start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "child" {
		t.Fatalf("child start = %v %s", ev.Kind, ev.Name.String())
	}
	if _, err = r.Next(); err != nil { // child end
		t.Fatalf("child end error = %v", err)
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("root end error = %v", err)
	}
	if ev.Kind != EventEndElement || ev.Name.Local != "root" {
		t.Fatalf("root end = %v %s", ev.Kind, ev.Name.String())
	}
}

func TestReaderNilErrors(t *testing.T) {
	var err error
	if _, err = NewReader(nil); err == nil {
		t.Fatalf("NewReader nil error = nil, want error")
	}

	var r *Reader
	if _, err = r.Next(); !errors.Is(err, errNilReader) {
		t.Fatalf("Next nil error = %v, want %v", err, errNilReader)
	}
	if err = r.Reset(nil); !errors.Is(err, errNilReader) {
		t.Fatalf("Reset nil error = %v, want %v", err, errNilReader)
	}
	if err = r.SkipSubtree(); !errors.Is(err, errNilReader) {
		t.Fatalf("SkipSubtree nil error = %v, want %v", err, errNilReader)
	}
	if _, err = r.ReadSubtreeBytes(); !errors.Is(err, errNoStartElement) {
		t.Fatalf("ReadSubtreeBytes nil error = %v, want %v", err, errNoStartElement)
	}
	if _, err = r.ReadSubtreeInto(nil); !errors.Is(err, errNoStartElement) {
		t.Fatalf("ReadSubtreeInto nil error = %v, want %v", err, errNoStartElement)
	}
	if line, col := r.CurrentPos(); line != 0 || col != 0 {
		t.Fatalf("CurrentPos nil = %d:%d, want 0:0", line, col)
	}
	if got := r.InputOffset(); got != 0 {
		t.Fatalf("InputOffset nil = %d, want 0", got)
	}
}

func TestReaderResetNilSource(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if err = r.Reset(nil); !errors.Is(err, errNilReader) {
		t.Fatalf("Reset nil src error = %v, want %v", err, errNilReader)
	}
}

func TestReaderResetIDs(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root><a/><b/></root>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
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
	if _, err = r.Next(); err != nil { // a end
		t.Fatalf("a end error = %v", err)
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("b start error = %v", err)
	}
	if ev.ID != 2 {
		t.Fatalf("b ID = %d, want 2", ev.ID)
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

func TestCurrentPosAndInputOffset(t *testing.T) {
	input := "<root>\n<child/></root>"
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	line, col := r.CurrentPos()
	if line != ev.Line || col != ev.Column {
		t.Fatalf("CurrentPos = %d:%d, want %d:%d", line, col, ev.Line, ev.Column)
	}
	offset := r.InputOffset()
	if offset == 0 {
		t.Fatalf("InputOffset = 0, want > 0")
	}
	if _, err = r.Next(); err != nil { // child start
		t.Fatalf("child start error = %v", err)
	}
	if r.InputOffset() <= offset {
		t.Fatalf("InputOffset did not advance")
	}
}

func TestRawNameFromBytesEmpty(t *testing.T) {
	name := rawNameFromBytes(nil)
	if len(name.Full) != 0 || len(name.Prefix) != 0 || len(name.Local) != 0 {
		t.Fatalf("RawName = %#v, want empty", name)
	}
}

func TestRawNameFromBytesPrefixed(t *testing.T) {
	name := rawNameFromBytes([]byte("p:local"))
	if string(name.Full) != "p:local" {
		t.Fatalf("Full = %q, want p:local", name.Full)
	}
	if string(name.Prefix) != "p" {
		t.Fatalf("Prefix = %q, want p", name.Prefix)
	}
	if string(name.Local) != "local" {
		t.Fatalf("Local = %q, want local", name.Local)
	}
}

func TestRawNameFromBytesColonOnly(t *testing.T) {
	name := rawNameFromBytes([]byte(":"))
	if string(name.Full) != ":" {
		t.Fatalf("Full = %q, want :", name.Full)
	}
	if len(name.Prefix) != 0 {
		t.Fatalf("Prefix = %q, want empty", name.Prefix)
	}
	if len(name.Local) != 0 {
		t.Fatalf("Local = %q, want empty", name.Local)
	}
}

func TestXMLDeclarationSkipped(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?><root/>`
	r, err := NewReader(strings.NewReader(input), EmitPI(true))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "root" {
		t.Fatalf("first event = %v %s, want root start", ev.Kind, ev.Name.String())
	}
}

func TestScopeDepthBeforeRoot(t *testing.T) {
	input := `<!--c--><root/>`
	r, err := NewReader(strings.NewReader(input), EmitComments(true))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("comment error = %v", err)
	}
	if ev.Kind != EventComment {
		t.Fatalf("comment kind = %v, want %v", ev.Kind, EventComment)
	}
	if ev.ScopeDepth != 0 {
		t.Fatalf("comment scope depth = %d, want 0", ev.ScopeDepth)
	}
}

func TestReaderCDATAEntityLiteral(t *testing.T) {
	input := `<root><![CDATA[&amp;]]></root>`
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
	if got := string(ev.Text); got != "&amp;" {
		t.Fatalf("char data = %q, want &amp;", got)
	}
}

func TestReaderCDATAEmpty(t *testing.T) {
	input := `<root><![CDATA[]]></root>`
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
	if len(ev.Text) != 0 {
		t.Fatalf("char data len = %d, want 0", len(ev.Text))
	}
}

func TestReaderEmptyAttrValue(t *testing.T) {
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
		t.Fatalf("attr missing, want empty value")
	}
	if len(val) != 0 {
		t.Fatalf("attr len = %d, want 0", len(val))
	}
}

func TestEmptyElementEquivalence(t *testing.T) {
	//nolint:govet // small test helper.
	type token struct {
		kind EventKind
		name string
	}
	collect := func(input string) ([]token, error) {
		r, err := NewReader(strings.NewReader(input))
		if err != nil {
			return nil, err
		}
		var out []token
		for {
			ev, err := r.Next()
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			if err != nil {
				return nil, err
			}
			if ev.Kind == EventStartElement || ev.Kind == EventEndElement {
				out = append(out, token{kind: ev.Kind, name: ev.Name.Local})
			}
		}
	}
	selfClosing, err := collect(`<root><elem/></root>`)
	if err != nil {
		t.Fatalf("self-closing collect error = %v", err)
	}
	explicit, err := collect(`<root><elem></elem></root>`)
	if err != nil {
		t.Fatalf("explicit collect error = %v", err)
	}
	if len(selfClosing) != len(explicit) {
		t.Fatalf("token lengths = %d/%d, want equal", len(selfClosing), len(explicit))
	}
	for i := range selfClosing {
		if selfClosing[i] != explicit[i] {
			t.Fatalf("token %d = %#v, want %#v", i, selfClosing[i], explicit[i])
		}
	}
}

func TestReaderResetOptions(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if err = r.Reset(strings.NewReader("<root/>"), TrackLineColumn(false)); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start after reset error = %v", err)
	}
	if ev.Line != 0 || ev.Column != 0 {
		t.Fatalf("line/column = %d:%d, want 0:0", ev.Line, ev.Column)
	}
}

func TestReaderResetMaxDepth(t *testing.T) {
	r, err := NewReader(strings.NewReader("<a><b/></a>"), MaxDepth(1))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err == nil {
		t.Fatalf("depth error = nil, want error")
	}
	if err = r.Reset(strings.NewReader("<a><b/></a>"), MaxDepth(2)); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start after reset error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("child start after reset error = %v", err)
	}
}

func TestReaderResetNilDecoder(t *testing.T) {
	var err error
	r := &Reader{}
	if err = r.Reset(strings.NewReader("<root/>")); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "root" {
		t.Fatalf("root event = %v %s, want root start", ev.Kind, ev.Name.String())
	}
}

func TestReaderResetNilNames(t *testing.T) {
	var err error
	r := &Reader{names: nil}
	if err = r.Reset(strings.NewReader("<root/>")); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	if r.names == nil {
		t.Fatalf("names = nil, want initialized")
	}
}

func TestReaderNextRecreatesNames(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	r.names = nil
	if _, err = r.Next(); err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if r.names == nil {
		t.Fatalf("names = nil, want initialized")
	}
}

func TestQNameCacheMaxEntriesOption(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"), MaxQNameInternEntries(7))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if r.names.maxEntries != 7 {
		t.Fatalf("maxEntries = %d, want 7", r.names.maxEntries)
	}
	if err = r.Reset(strings.NewReader("<root/>"), MaxQNameInternEntries(3)); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	if r.names.maxEntries != 3 {
		t.Fatalf("maxEntries after reset = %d, want 3", r.names.maxEntries)
	}
}

func TestQNameCacheLimitDefaultOption(t *testing.T) {
	if got := qnameCacheLimit(nil); got != qnameCacheMaxEntries {
		t.Fatalf("qnameCacheLimit(nil) = %d, want %d", got, qnameCacheMaxEntries)
	}
}

func TestQNameCacheLimitNegativeOption(t *testing.T) {
	if got := qnameCacheLimit([]xmltext.Options{xmltext.MaxQNameInternEntries(-5)}); got != 0 {
		t.Fatalf("qnameCacheLimit(-5) = %d, want 0", got)
	}
	r, err := NewReader(strings.NewReader("<root/>"), MaxQNameInternEntries(-5))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if r.names.maxEntries != 0 {
		t.Fatalf("maxEntries = %d, want 0", r.names.maxEntries)
	}
}

func TestResetNamespaceIsolation(t *testing.T) {
	r, err := NewReader(strings.NewReader(`<a xmlns="urn:one"><b/></a>`))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("first root error = %v", err)
	}
	if ev.Name.Namespace != "urn:one" {
		t.Fatalf("first namespace = %q, want urn:one", ev.Name.Namespace)
	}
	if err = r.Reset(strings.NewReader(`<a xmlns="urn:two"><b/></a>`)); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("second root error = %v", err)
	}
	if ev.Name.Namespace != "urn:two" {
		t.Fatalf("second namespace = %q, want urn:two", ev.Name.Namespace)
	}
	if ns, ok := r.LookupNamespaceAt("", 0); !ok || ns != "urn:two" {
		t.Fatalf("LookupNamespaceAt = %q, ok=%v, want urn:two, true", ns, ok)
	}
}

func TestReaderMultipleReset(t *testing.T) {
	r, err := NewReader(strings.NewReader("<a/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("a start error = %v", err)
	}
	if err = r.Reset(strings.NewReader("<b/>")); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	if err = r.Reset(strings.NewReader("<c/>")); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("c start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "c" {
		t.Fatalf("event = %v %s, want c start", ev.Kind, ev.Name.String())
	}
}

func TestReaderResetClearsPendingPop(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root><child/></root>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // child
		t.Fatalf("child start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // child end
		t.Fatalf("child end error = %v", err)
	}
	if !r.pendingPop {
		t.Fatalf("pendingPop = false, want true")
	}
	if err = r.Reset(strings.NewReader("<root/>")); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	if r.pendingPop {
		t.Fatalf("pendingPop = true, want false")
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start after reset error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "root" {
		t.Fatalf("root event = %v %s, want root start", ev.Kind, ev.Name.String())
	}
}

func TestSkipSubtreeError(t *testing.T) {
	input := `<root><item>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // item
		t.Fatalf("item start error = %v", err)
	}
	if err = r.SkipSubtree(); err == nil {
		t.Fatalf("SkipSubtree error = nil, want error")
	} else {
		var syntax *xmltext.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("SkipSubtree error type = %T, want *xmltext.SyntaxError", err)
		}
	}
}

func TestSkipSubtreeEmptyStack(t *testing.T) {
	input := `<root/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if err = r.SkipSubtree(); !errors.Is(err, errNoStartElement) {
		t.Fatalf("SkipSubtree error = %v, want %v", err, errNoStartElement)
	}
}

func TestEndElementHasNoID(t *testing.T) {
	input := `<root><child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // child
		t.Fatalf("child start error = %v", err)
	}
	ev, err := r.Next() // child end
	if err != nil {
		t.Fatalf("child end error = %v", err)
	}
	if ev.Kind != EventEndElement {
		t.Fatalf("kind = %v, want %v", ev.Kind, EventEndElement)
	}
	if ev.ID != 0 {
		t.Fatalf("end element ID = %d, want 0", ev.ID)
	}
}

func TestSkipSubtreePendingPop(t *testing.T) {
	input := `<root><skip><inner/></skip><after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // skip
		t.Fatalf("skip start error = %v", err)
	}
	r.pendingPop = true
	r.ns.push(nsScope{})
	if err = r.SkipSubtree(); err != nil {
		t.Fatalf("SkipSubtree error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "after" {
		t.Fatalf("after event = %v %s, want after start", ev.Kind, ev.Name.String())
	}
}

func TestNamespaceDeclsAfterSkipSubtree(t *testing.T) {
	input := `<root xmlns:a="urn:a"><skip xmlns:b="urn:b"><inner/></skip><after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next() // root
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	rootDecls := declsToMap(r.NamespaceDeclsAt(ev.ScopeDepth))
	if rootDecls["a"] != "urn:a" {
		t.Fatalf("root prefix a = %q, want urn:a", rootDecls["a"])
	}
	if _, err = r.Next(); err != nil { // skip
		t.Fatalf("skip start error = %v", err)
	}
	if err = r.SkipSubtree(); err != nil {
		t.Fatalf("SkipSubtree error = %v", err)
	}
	ev, err = r.Next() // after
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if decls := r.NamespaceDeclsAt(ev.ScopeDepth); decls != nil {
		t.Fatalf("after decls = %v, want nil", decls)
	}
	if ns, ok := r.LookupNamespaceAt("b", ev.ScopeDepth); ok || ns != "" {
		t.Fatalf("prefix b = %q (ok=%v), want empty, false", ns, ok)
	}
	if ns, ok := r.LookupNamespaceAt("a", ev.ScopeDepth); !ok || ns != "urn:a" {
		t.Fatalf("prefix a = %q (ok=%v), want urn:a, true", ns, ok)
	}
}

func TestMultipleSkipSubtree(t *testing.T) {
	input := `<root><a><x/></a><b><y/></b><c/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("a start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "a" {
		t.Fatalf("a start = %v %s, want a start", ev.Kind, ev.Name.String())
	}
	if err = r.SkipSubtree(); err != nil {
		t.Fatalf("SkipSubtree a error = %v", err)
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("b start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "b" {
		t.Fatalf("b start = %v %s, want b start", ev.Kind, ev.Name.String())
	}
	if err = r.SkipSubtree(); err != nil {
		t.Fatalf("SkipSubtree b error = %v", err)
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("c start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "c" {
		t.Fatalf("c start = %v %s, want c start", ev.Kind, ev.Name.String())
	}
}

func TestSkipSubtreeTwiceConsecutive(t *testing.T) {
	input := `<root><a/><b/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // a
		t.Fatalf("a start error = %v", err)
	}
	if err = r.SkipSubtree(); err != nil {
		t.Fatalf("SkipSubtree error = %v", err)
	}
	if err = r.SkipSubtree(); !errors.Is(err, errNoStartElement) {
		t.Fatalf("second SkipSubtree error = %v, want %v", err, errNoStartElement)
	}
}

func TestPopElementNameEmpty(t *testing.T) {
	var err error
	r := &Reader{}
	if _, err = r.popElementName(); err == nil {
		t.Fatalf("popElementName error = nil, want error")
	}
}

func TestEndEventRawEmptyStack(t *testing.T) {
	r := &Reader{}
	tok := xmltext.Token{Name: []byte("root")}
	if _, _, err := r.endEvent(nextRaw, &tok, 1, 1); err == nil {
		t.Fatalf("endEvent error = nil, want error")
	}
}

func TestDeeplyNestedElements(t *testing.T) {
	const depth = 200
	var b strings.Builder
	for i := 0; i < depth; i++ {
		fmt.Fprintf(&b, "<e%d>", i)
	}
	for i := depth - 1; i >= 0; i-- {
		fmt.Fprintf(&b, "</e%d>", i)
	}
	r, err := NewReader(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	var startCount int
	for {
		ev, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next error at depth %d: %v", startCount, err)
		}
		if ev.Kind == EventStartElement {
			startCount++
		}
	}
	if startCount != depth {
		t.Fatalf("start count = %d, want %d", startCount, depth)
	}
}

func TestConcurrentReaderCreation(t *testing.T) {
	const goroutines = 8
	input := `<root><child/></root>`
	errs := make(chan error, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := NewReader(strings.NewReader(input))
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

package xmltext

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDecoderTokensBasic(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root attr="v">text</root>`))

	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken start error = %v", err)
	}
	if tok.Kind() != KindStartElement {
		t.Fatalf("start kind = %v, want %v", tok.Kind(), KindStartElement)
	}
	if got := string(dec.SpanBytes(tok.Name().Local)); got != "root" {
		t.Fatalf("start name = %q, want root", got)
	}
	if tok.AttrCount() != 1 {
		t.Fatalf("attr count = %d, want 1", tok.AttrCount())
	}
	attr := tok.Attrs()[0]
	if got := string(dec.SpanBytes(attr.Name.Local)); got != "attr" {
		t.Fatalf("attr name = %q, want attr", got)
	}
	if got := string(dec.SpanBytes(attr.ValueSpan)); got != "v" {
		t.Fatalf("attr value = %q, want v", got)
	}

	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if tok.Kind() != KindCharData {
		t.Fatalf("text kind = %v, want %v", tok.Kind(), KindCharData)
	}
	if got := string(dec.SpanBytes(tok.TextSpan())); got != "text" {
		t.Fatalf("text = %q, want text", got)
	}

	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken end error = %v", err)
	}
	if tok.Kind() != KindEndElement {
		t.Fatalf("end kind = %v, want %v", tok.Kind(), KindEndElement)
	}
	if got := string(dec.SpanBytes(tok.Name().Local)); got != "root" {
		t.Fatalf("end name = %q, want root", got)
	}

	if _, err := dec.ReadToken(); err != io.EOF {
		t.Fatalf("ReadToken EOF = %v, want io.EOF", err)
	}
}

func TestDecoderResolveEntities(t *testing.T) {
	input := `<root attr="a&amp;b">x&amp;y</root>`

	dec := NewDecoder(strings.NewReader(input))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken start error = %v", err)
	}
	if !tok.AttrNeedsUnescape(0) {
		t.Fatalf("AttrNeedsUnescape = false, want true")
	}
	if got := string(dec.SpanBytes(tok.Attrs()[0].ValueSpan)); got != "a&amp;b" {
		t.Fatalf("raw attr value = %q, want a&amp;b", got)
	}
	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if !tok.TextNeedsUnescape() {
		t.Fatalf("TextNeedsUnescape = false, want true")
	}

	dec = NewDecoder(strings.NewReader(input), ResolveEntities(true))
	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken start resolve error = %v", err)
	}
	if tok.AttrNeedsUnescape(0) {
		t.Fatalf("AttrNeedsUnescape = true, want false")
	}
	if got := string(dec.SpanBytes(tok.Attrs()[0].ValueSpan)); got != "a&b" {
		t.Fatalf("unescaped attr value = %q, want a&b", got)
	}
	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken text resolve error = %v", err)
	}
	if tok.TextNeedsUnescape() {
		t.Fatalf("TextNeedsUnescape = true, want false")
	}
	if got := string(dec.SpanBytes(tok.TextSpan())); got != "x&y" {
		t.Fatalf("unescaped text = %q, want x&y", got)
	}
}

func TestDecoderAttrValueBuffer(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root a="1&amp;2" b="3&amp;4"/>`), ResolveEntities(true))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken error = %v", err)
	}
	if tok.AttrCount() != 2 {
		t.Fatalf("attr count = %d, want 2", tok.AttrCount())
	}
	attrs := tok.Attrs()
	if got := string(dec.SpanBytes(attrs[0].ValueSpan)); got != "1&2" {
		t.Fatalf("attr a = %q, want 1&2", got)
	}
	if got := string(dec.SpanBytes(attrs[1].ValueSpan)); got != "3&4" {
		t.Fatalf("attr b = %q, want 3&4", got)
	}
}

func TestDecoderStackPath(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><child/><child>t</child></root>`))

	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("start root error = %v", err)
	}
	if tok.Kind() != KindStartElement {
		t.Fatalf("root kind = %v, want %v", tok.Kind(), KindStartElement)
	}
	if dec.StackDepth() != 1 {
		t.Fatalf("root depth = %d, want 1", dec.StackDepth())
	}
	if got := dec.StackPath(nil).String(); got != "/root[1]" {
		t.Fatalf("root path = %q, want /root[1]", got)
	}

	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("child1 start error = %v", err)
	}
	if dec.StackDepth() != 2 {
		t.Fatalf("child1 depth = %d, want 2", dec.StackDepth())
	}
	if got := dec.StackPath(nil).String(); got != "/root[1]/child[1]" {
		t.Fatalf("child1 path = %q, want /root[1]/child[1]", got)
	}

	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("child1 end error = %v", err)
	}
	if tok.Kind() != KindEndElement {
		t.Fatalf("child1 end kind = %v, want %v", tok.Kind(), KindEndElement)
	}
	if dec.StackDepth() != 1 {
		t.Fatalf("after child1 depth = %d, want 1", dec.StackDepth())
	}

	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("child2 start error = %v", err)
	}
	if got := dec.StackPath(nil).String(); got != "/root[1]/child[2]" {
		t.Fatalf("child2 path = %q, want /root[1]/child[2]", got)
	}

	_, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("child2 text error = %v", err)
	}
	_, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("child2 end error = %v", err)
	}
	if dec.StackDepth() != 1 {
		t.Fatalf("after child2 depth = %d, want 1", dec.StackDepth())
	}
}

func TestDecoderPeekKind(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root/>`))
	if got := dec.PeekKind(); got != KindStartElement {
		t.Fatalf("PeekKind = %v, want %v", got, KindStartElement)
	}
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken start error = %v", err)
	}
	if got := dec.PeekKind(); got != KindEndElement {
		t.Fatalf("PeekKind = %v, want %v", got, KindEndElement)
	}
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken end error = %v", err)
	}
	if got := dec.PeekKind(); got != KindNone {
		t.Fatalf("PeekKind = %v, want %v", got, KindNone)
	}
}

func TestDecoderSkipValue(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><skip><inner/></skip><after/></root>`))

	tok, err := dec.ReadToken()
	if err != nil || tok.Kind() != KindStartElement {
		t.Fatalf("root start error = %v", err)
	}
	tok, err = dec.ReadToken()
	if err != nil || tok.Kind() != KindStartElement {
		t.Fatalf("skip start error = %v", err)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue error = %v", err)
	}
	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if got := string(dec.SpanBytes(tok.Name().Local)); got != "after" {
		t.Fatalf("after name = %q, want after", got)
	}
}

func TestDecoderCommentsPIAndCDATA(t *testing.T) {
	input := `<?pi ok?><root><!--c--><![CDATA[x]]></root>`

	dec := NewDecoder(strings.NewReader(input), EmitComments(true), EmitPI(true))
	kinds := []Kind{}
	for {
		tok, err := dec.ReadToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadToken error = %v", err)
		}
		kinds = append(kinds, tok.Kind())
	}
	want := []Kind{KindPI, KindStartElement, KindComment, KindCDATA, KindEndElement}
	if len(kinds) != len(want) {
		t.Fatalf("token count = %d, want %d", len(kinds), len(want))
	}
	for i, got := range kinds {
		if got != want[i] {
			t.Fatalf("kind[%d] = %v, want %v", i, got, want[i])
		}
	}

	dec = NewDecoder(strings.NewReader(input))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken start error = %v", err)
	}
	if tok.Kind() != KindStartElement {
		t.Fatalf("start kind = %v, want %v", tok.Kind(), KindStartElement)
	}
	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken cdata error = %v", err)
	}
	if tok.Kind() != KindCDATA {
		t.Fatalf("cdata kind = %v, want %v", tok.Kind(), KindCDATA)
	}
}

func TestDecoderCoalesceCharData(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><![CDATA[foo]]>bar</root>`), CoalesceCharData(true))
	_, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("start root error = %v", err)
	}
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("text error = %v", err)
	}
	if tok.Kind() != KindCharData {
		t.Fatalf("text kind = %v, want %v", tok.Kind(), KindCharData)
	}
	if got := string(dec.SpanBytes(tok.TextSpan())); got != "foobar" {
		t.Fatalf("text = %q, want foobar", got)
	}
}

func TestDecoderReadValue(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><a>1</a><b/></root>`))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	value, err := dec.ReadValue()
	if err != nil {
		t.Fatalf("ReadValue error = %v", err)
	}
	if got := string(value); got != `<a>1</a>` {
		t.Fatalf("ReadValue = %q, want <a>1</a>", got)
	}
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("after value error = %v", err)
	}
	if got := string(dec.SpanBytes(tok.Name().Local)); got != "b" {
		t.Fatalf("after value name = %q, want b", got)
	}
}

func TestDecoderReadValueResolveEntities(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><a attr="x&amp;y">1 &lt; 2</a><b/></root>`), ResolveEntities(true))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	value, err := dec.ReadValue()
	if err != nil {
		t.Fatalf("ReadValue error = %v", err)
	}
	if got := string(value); got != `<a attr="x&y">1 < 2</a>` {
		t.Fatalf("ReadValue = %q, want <a attr=\"x&y\">1 < 2</a>", got)
	}
}

func TestDecoderDuplicateAttrs(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root a="1" a="2"/>`))
	_, err := dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errDuplicateAttr) {
		t.Fatalf("error = %v, want %v", err, errDuplicateAttr)
	}
}

func TestDecoderContentOutsideRoot(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`text<root/>`))
	_, err := dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errContentOutsideRoot) {
		t.Fatalf("error = %v, want %v", err, errContentOutsideRoot)
	}

	dec = NewDecoder(strings.NewReader(" \n<root/>"))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("whitespace token error = %v", err)
	}
	if tok.Kind() != KindCharData {
		t.Fatalf("whitespace kind = %v, want %v", tok.Kind(), KindCharData)
	}
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
}

func TestDecoderMultipleRoots(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<a/><b/>`))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("root end error = %v", err)
	}
	_, err := dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errMultipleRoots) {
		t.Fatalf("error = %v, want %v", err, errMultipleRoots)
	}
}

func TestDecoderMissingRoot(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<!--c-->`))
	_, err := dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errMissingRoot) {
		t.Fatalf("error = %v, want %v", err, errMissingRoot)
	}
}

func TestDecoderXMLDeclPlacement(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<!--c--><?xml version="1.0"?><root/>`))
	_, err := dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errMisplacedXMLDecl) {
		t.Fatalf("error = %v, want %v", err, errMisplacedXMLDecl)
	}

	dec = NewDecoder(strings.NewReader(`<?xml version="1.0"?><?xml version="1.0"?><root/>`))
	_, err = dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errDuplicateXMLDecl) {
		t.Fatalf("error = %v, want %v", err, errDuplicateXMLDecl)
	}
}

func TestDecoderInvalidCommentAndPI(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<!--a--b--><root/>`))
	_, err := dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errInvalidComment) {
		t.Fatalf("error = %v, want %v", err, errInvalidComment)
	}

	dec = NewDecoder(strings.NewReader(`<?target=1?><root/>`))
	_, err = dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errInvalidPI) {
		t.Fatalf("error = %v, want %v", err, errInvalidPI)
	}
}

func TestDecoderInvalidChar(t *testing.T) {
	input := []byte{'<', 'a', '>', 0x01, '<', '/', 'a', '>'}
	dec := NewDecoder(strings.NewReader(string(input)))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("start error = %v", err)
	}
	_, err := dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errInvalidChar) {
		t.Fatalf("error = %v, want %v", err, errInvalidChar)
	}
}

func TestDecoderInvalidName(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<1a/>`))
	_, err := dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errInvalidName) {
		t.Fatalf("error = %v, want %v", err, errInvalidName)
	}
}

func TestDecoderSyntaxError(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<a></b>`))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("start error = %v", err)
	}
	_, err := dec.ReadToken()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var syntax *SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("error type = %T, want *SyntaxError", err)
	}
}

func TestDecoderLimits(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<a><b/></a>`), MaxDepth(1))
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if _, err := dec.ReadToken(); err == nil {
		t.Fatalf("expected depth error, got nil")
	}

	dec = NewDecoder(strings.NewReader(`<a b="1" c="2"/>`), MaxAttrs(1))
	if _, err := dec.ReadToken(); err == nil {
		t.Fatalf("expected attr limit error, got nil")
	}
}

func TestDecoderInternStats(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><root/></root>`))
	for {
		_, err := dec.ReadToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadToken error = %v", err)
		}
	}
	stats := dec.InternStats()
	if stats.Count == 0 {
		t.Fatalf("intern count = 0, want > 0")
	}
	if stats.Hits == 0 {
		t.Fatalf("intern hits = 0, want > 0")
	}
}

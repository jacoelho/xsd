package xmltext

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestDecoderTokensBasic(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root attr="v">text</root>`))
	reader := newTokenReader(dec)

	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken start error = %v", err)
	}
	if tok.Kind != KindStartElement {
		t.Fatalf("start kind = %v, want %v", tok.Kind, KindStartElement)
	}
	if got := string(tok.Name); got != "root" {
		t.Fatalf("start name = %q, want root", got)
	}
	if len(tok.Attrs) != 1 {
		t.Fatalf("attr count = %d, want 1", len(tok.Attrs))
	}
	attr := tok.Attrs[0]
	if got := string(attr.Name); got != "attr" {
		t.Fatalf("attr name = %q, want attr", got)
	}
	if got := string(attr.Value); got != "v" {
		t.Fatalf("attr value = %q, want v", got)
	}

	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if tok.Kind != KindCharData {
		t.Fatalf("text kind = %v, want %v", tok.Kind, KindCharData)
	}
	if got := string(tok.Text); got != "text" {
		t.Fatalf("text = %q, want text", got)
	}

	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken end error = %v", err)
	}
	if tok.Kind != KindEndElement {
		t.Fatalf("end kind = %v, want %v", tok.Kind, KindEndElement)
	}
	if got := string(tok.Name); got != "root" {
		t.Fatalf("end name = %q, want root", got)
	}

	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("ReadToken EOF = %v, want io.EOF", err)
	}
}

func TestDecoderResolveEntities(t *testing.T) {
	input := `<root attr="a&amp;b">x&amp;y</root>`

	dec := NewDecoder(strings.NewReader(input))
	reader := newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken start error = %v", err)
	}
	if len(tok.Attrs) == 0 || !tok.Attrs[0].ValueNeeds {
		t.Fatalf("AttrNeedsUnescape = false, want true")
	}
	if got := string(tok.Attrs[0].Value); got != "a&amp;b" {
		t.Fatalf("raw attr value = %q, want a&amp;b", got)
	}
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if !tok.TextNeeds {
		t.Fatalf("TextNeedsUnescape = false, want true")
	}

	dec = NewDecoder(strings.NewReader(input), ResolveEntities(true))
	reader = newTokenReader(dec)
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken start resolve error = %v", err)
	}
	if len(tok.Attrs) > 0 && tok.Attrs[0].ValueNeeds {
		t.Fatalf("AttrNeedsUnescape = true, want false")
	}
	if got := string(tok.Attrs[0].Value); got != "a&b" {
		t.Fatalf("unescaped attr value = %q, want a&b", got)
	}
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken text resolve error = %v", err)
	}
	if tok.TextNeeds {
		t.Fatalf("TextNeedsUnescape = true, want false")
	}
	if got := string(tok.Text); got != "x&y" {
		t.Fatalf("unescaped text = %q, want x&y", got)
	}
}

func TestDecoderAttrValueBuffer(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root a="1&amp;2" b="3&amp;4"/>`), ResolveEntities(true))
	reader := newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken error = %v", err)
	}
	if len(tok.Attrs) != 2 {
		t.Fatalf("attr count = %d, want 2", len(tok.Attrs))
	}
	attrs := tok.Attrs
	if got := string(attrs[0].Value); got != "1&2" {
		t.Fatalf("attr a = %q, want 1&2", got)
	}
	if got := string(attrs[1].Value); got != "3&4" {
		t.Fatalf("attr b = %q, want 3&4", got)
	}
}

func TestDecoderStackPath(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><child/><child>t</child></root>`))
	reader := newTokenReader(dec)

	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("start root error = %v", err)
	}
	if tok.Kind != KindStartElement {
		t.Fatalf("root kind = %v, want %v", tok.Kind, KindStartElement)
	}
	if len(dec.stack) != 1 {
		t.Fatalf("root depth = %d, want 1", len(dec.stack))
	}
	if got := dec.StackPointer(); got != "/root[1]" {
		t.Fatalf("root path = %q, want /root[1]", got)
	}

	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("child1 start error = %v", err)
	}
	if len(dec.stack) != 2 {
		t.Fatalf("child1 depth = %d, want 2", len(dec.stack))
	}
	if got := dec.StackPointer(); got != "/root[1]/child[1]" {
		t.Fatalf("child1 path = %q, want /root[1]/child[1]", got)
	}

	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("child1 end error = %v", err)
	}
	if tok.Kind != KindEndElement {
		t.Fatalf("child1 end kind = %v, want %v", tok.Kind, KindEndElement)
	}
	if len(dec.stack) != 1 {
		t.Fatalf("after child1 depth = %d, want 1", len(dec.stack))
	}

	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("child2 start error = %v", err)
	}
	if got := dec.StackPointer(); got != "/root[1]/child[2]" {
		t.Fatalf("child2 path = %q, want /root[1]/child[2]", got)
	}

	_, err = reader.Next()
	if err != nil {
		t.Fatalf("child2 text error = %v", err)
	}
	_, err = reader.Next()
	if err != nil {
		t.Fatalf("child2 end error = %v", err)
	}
	if len(dec.stack) != 1 {
		t.Fatalf("after child2 depth = %d, want 1", len(dec.stack))
	}
}

func TestDecoderPeekKind(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root/>`))
	reader := newTokenReader(dec)
	if got := dec.PeekKind(); got != KindStartElement {
		t.Fatalf("PeekKind = %v, want %v", got, KindStartElement)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken start error = %v", err)
	}
	if got := dec.PeekKind(); got != KindEndElement {
		t.Fatalf("PeekKind = %v, want %v", got, KindEndElement)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken end error = %v", err)
	}
	if got := dec.PeekKind(); got != KindNone {
		t.Fatalf("PeekKind = %v, want %v", got, KindNone)
	}
}

func TestDecoderSkipValue(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><skip><inner/></skip><after/></root>`))
	reader := newTokenReader(dec)

	tok, err := reader.Next()
	if err != nil || tok.Kind != KindStartElement {
		t.Fatalf("root start error = %v", err)
	}
	tok, err = reader.Next()
	if err != nil || tok.Kind != KindStartElement {
		t.Fatalf("skip start error = %v", err)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue error = %v", err)
	}
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if got := string(tok.Name); got != "after" {
		t.Fatalf("after name = %q, want after", got)
	}
}

func TestDecoderCommentsPIAndCDATA(t *testing.T) {
	input := `<?pi ok?><root><!--c--><![CDATA[x]]></root>`

	dec := NewDecoder(strings.NewReader(input), EmitComments(true), EmitPI(true))
	reader := newTokenReader(dec)
	kinds := []Kind{}
	for {
		tok, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("ReadToken error = %v", err)
		}
		kinds = append(kinds, tok.Kind)
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
	reader = newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken start error = %v", err)
	}
	if tok.Kind != KindStartElement {
		t.Fatalf("start kind = %v, want %v", tok.Kind, KindStartElement)
	}
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken cdata error = %v", err)
	}
	if tok.Kind != KindCDATA {
		t.Fatalf("cdata kind = %v, want %v", tok.Kind, KindCDATA)
	}
}

func TestDecoderCoalesceCharData(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><![CDATA[foo]]>bar</root>`), CoalesceCharData(true))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if err != nil {
		t.Fatalf("start root error = %v", err)
	}
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("text error = %v", err)
	}
	if tok.Kind != KindCharData {
		t.Fatalf("text kind = %v, want %v", tok.Kind, KindCharData)
	}
	if got := string(tok.Text); got != "foobar" {
		t.Fatalf("text = %q, want foobar", got)
	}
}

func TestDecoderReadValue(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><a>1</a><b/></root>`))
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	scratch := make([]byte, 64)
	n, err := dec.ReadValueInto(scratch)
	if err != nil {
		t.Fatalf("ReadValueInto error = %v", err)
	}
	if got := string(scratch[:n]); got != `<a>1</a>` {
		t.Fatalf("ReadValueInto = %q, want <a>1</a>", got)
	}
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("after value error = %v", err)
	}
	if got := string(tok.Name); got != "b" {
		t.Fatalf("after value name = %q, want b", got)
	}
}

func TestDecoderReadValueResolveEntities(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><a attr="x&amp;y">1 &lt; 2</a><b/></root>`), ResolveEntities(true))
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	scratch := make([]byte, 64)
	n, err := dec.ReadValueInto(scratch)
	if err != nil {
		t.Fatalf("ReadValueInto error = %v", err)
	}
	if got := string(scratch[:n]); got != `<a attr="x&y">1 < 2</a>` {
		t.Fatalf("ReadValueInto = %q, want <a attr=\"x&y\">1 < 2</a>", got)
	}
}

func TestDecoderCoalesceCharDataResolveEntitiesScenarios(t *testing.T) {
	longText := strings.Repeat("x", 64)
	tests := []struct {
		name      string
		input     string
		wantText  string
		wantRaw   string
		nextLocal string
		opts      []Options
	}{
		{
			name:     "cdata-then-entity",
			input:    `<root><a><![CDATA[foo]]>&amp;bar</a></root>`,
			wantText: `foo&bar`,
			wantRaw:  `<![CDATA[foo]]>&amp;bar`,
		},
		{
			name:     "entity-then-cdata",
			input:    `<root><a>foo&amp;bar<![CDATA[baz]]></a></root>`,
			wantText: `foo&barbaz`,
			wantRaw:  `foo&amp;bar<![CDATA[baz]]>`,
		},
		{
			name:     "multi-segment",
			input:    `<root><a><![CDATA[foo]]>bar&amp;baz<![CDATA[qux]]></a></root>`,
			wantText: `foobar&bazqux`,
			wantRaw:  `<![CDATA[foo]]>bar&amp;baz<![CDATA[qux]]>`,
		},
		{
			name:     "buffer-boundary",
			input:    `<root><a><![CDATA[` + longText + `]]>&amp;` + longText + `</a></root>`,
			wantText: longText + `&` + longText,
			wantRaw:  `<![CDATA[` + longText + `]]>&amp;` + longText,
			opts:     []Options{BufferSize(32)},
		},
		{
			name:      "sibling-after",
			input:     `<root><a><![CDATA[foo]]>&amp;bar</a><b/></root>`,
			wantText:  `foo&bar`,
			wantRaw:   `<![CDATA[foo]]>&amp;bar`,
			nextLocal: "b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []Options{ResolveEntities(true), CoalesceCharData(true)}
			opts = append(opts, tt.opts...)
			dec := NewDecoder(strings.NewReader(tt.input), opts...)
			var tok rawToken
			if err := dec.readTokenIntoRaw(&tok); err != nil {
				t.Fatalf("root start error = %v", err)
			}
			if tok.kind != KindStartElement {
				t.Fatalf("root start kind = %v, want %v", tok.kind, KindStartElement)
			}
			if got := string(dec.spanBytes(tok.name.Local)); got != "root" {
				t.Fatalf("root start name = %q, want root", got)
			}
			if err := dec.readTokenIntoRaw(&tok); err != nil {
				t.Fatalf("a start error = %v", err)
			}
			if tok.kind != KindStartElement {
				t.Fatalf("a start kind = %v, want %v", tok.kind, KindStartElement)
			}
			if got := string(dec.spanBytes(tok.name.Local)); got != "a" {
				t.Fatalf("a start name = %q, want a", got)
			}
			if err := dec.readTokenIntoRaw(&tok); err != nil {
				t.Fatalf("text error = %v", err)
			}
			if tok.kind != KindCharData {
				t.Fatalf("text kind = %v, want %v", tok.kind, KindCharData)
			}
			if tok.raw.buf != &dec.buf {
				t.Fatalf("raw buffer mismatch")
			}
			if got := string(dec.spanBytes(tok.text)); got != tt.wantText {
				t.Fatalf("text = %q, want %s", got, tt.wantText)
			}
			if got := string(dec.spanBytes(tok.raw)); got != tt.wantRaw {
				t.Fatalf("raw = %q, want %s", got, tt.wantRaw)
			}
			if tt.nextLocal == "" {
				return
			}
			if err := dec.readTokenIntoRaw(&tok); err != nil {
				t.Fatalf("a end error = %v", err)
			}
			if tok.kind != KindEndElement {
				t.Fatalf("a end kind = %v, want %v", tok.kind, KindEndElement)
			}
			if err := dec.readTokenIntoRaw(&tok); err != nil {
				t.Fatalf("after text error = %v", err)
			}
			if got := string(dec.spanBytes(tok.name.Local)); got != tt.nextLocal {
				t.Fatalf("after text name = %q, want %s", got, tt.nextLocal)
			}
		})
	}
}

func TestDecoderDuplicateAttrs(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root a="1" a="2"/>`))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errDuplicateAttr) {
		t.Fatalf("error = %v, want %v", err, errDuplicateAttr)
	}
}

func TestDecoderContentOutsideRoot(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`text<root/>`))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errContentOutsideRoot) {
		t.Fatalf("error = %v, want %v", err, errContentOutsideRoot)
	}

	dec = NewDecoder(strings.NewReader(" \n<root/>"))
	reader = newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("whitespace token error = %v", err)
	}
	if tok.Kind != KindCharData {
		t.Fatalf("whitespace kind = %v, want %v", tok.Kind, KindCharData)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
}

func TestDecoderMultipleRoots(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<a/><b/>`))
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatalf("root end error = %v", err)
	}
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errMultipleRoots) {
		t.Fatalf("error = %v, want %v", err, errMultipleRoots)
	}
}

func TestDecoderDuplicateAttrsLarge(t *testing.T) {
	var b strings.Builder
	b.WriteString("<root")
	for i := 0; i < attrSeenSmallMax+1; i++ {
		b.WriteString(" a")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("=\"")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\"")
	}
	b.WriteString(" a0=\"dup\"/>")

	dec := NewDecoder(strings.NewReader(b.String()))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected duplicate attribute error")
	}
	if !errors.Is(err, errDuplicateAttr) {
		t.Fatalf("error = %v, want %v", err, errDuplicateAttr)
	}
}

func TestDecoderPINonASCIIName(t *testing.T) {
	input := "<?\u00e9\u03c0 data?><root/>"
	dec := NewDecoder(strings.NewReader(input), EmitPI(true), BufferSize(1))
	reader := newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken PI error = %v", err)
	}
	if tok.Kind != KindPI {
		t.Fatalf("PI kind = %v, want %v", tok.Kind, KindPI)
	}
	if got := string(tok.Text); !strings.HasPrefix(got, "\u00e9\u03c0") {
		t.Fatalf("PI text = %q, want prefix \\u00e9\\u03c0", got)
	}
}

func TestDecoderSkipValueScenarios(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><a/><b><c/></b><d>text</d></root>`))
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken a error = %v", err)
	}
	if tok.Kind != KindStartElement {
		t.Fatalf("a kind = %v, want %v", tok.Kind, KindStartElement)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue a error = %v", err)
	}
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken b error = %v", err)
	}
	if got := string(tok.Name); got != "b" {
		t.Fatalf("b name = %q, want b", got)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue b error = %v", err)
	}
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken d error = %v", err)
	}
	if got := string(tok.Name); got != "d" {
		t.Fatalf("d name = %q, want d", got)
	}
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if tok.Kind != KindCharData {
		t.Fatalf("text kind = %v, want %v", tok.Kind, KindCharData)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue text error = %v", err)
	}
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken d end error = %v", err)
	}
	if tok.Kind != KindEndElement {
		t.Fatalf("d end kind = %v, want %v", tok.Kind, KindEndElement)
	}
}

func TestDecoderMissingRoot(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<!--c-->`))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errMissingRoot) {
		t.Fatalf("error = %v, want %v", err, errMissingRoot)
	}
}

func TestDecoderXMLDeclPlacement(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<!--c--><?xml version="1.0"?><root/>`))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errMisplacedXMLDecl) {
		t.Fatalf("error = %v, want %v", err, errMisplacedXMLDecl)
	}

	dec = NewDecoder(strings.NewReader(`<?xml version="1.0"?><?xml version="1.0"?><root/>`))
	reader = newTokenReader(dec)
	_, err = reader.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errDuplicateXMLDecl) {
		t.Fatalf("error = %v, want %v", err, errDuplicateXMLDecl)
	}
}

func TestDecoderInvalidCommentAndPI(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<!--a--b--><root/>`))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errInvalidComment) {
		t.Fatalf("error = %v, want %v", err, errInvalidComment)
	}

	dec = NewDecoder(strings.NewReader(`<?target=1?><root/>`))
	reader = newTokenReader(dec)
	_, err = reader.Next()
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
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("start error = %v", err)
	}
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errInvalidChar) {
		t.Fatalf("error = %v, want %v", err, errInvalidChar)
	}
}

func TestDecoderInvalidName(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<1a/>`))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errInvalidName) {
		t.Fatalf("error = %v, want %v", err, errInvalidName)
	}
}

func TestDecoderQNameInvalidColons(t *testing.T) {
	tests := []string{
		`<a:te:st xmlns:a="abcd"/>`,
		`<a:test xmlns:a="abcd">1</a:te:st>`,
	}
	for _, input := range tests {
		err := readAllTokens(input)
		if err == nil {
			t.Fatalf("input %q error = nil, want %v", input, errInvalidName)
		}
		if !errors.Is(err, errInvalidName) {
			t.Fatalf("input %q error = %v, want %v", input, err, errInvalidName)
		}
	}
}

func TestDecoderQNameMismatchedEndTags(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "unclosedChild",
			input:   `<x:book xmlns:x="abcd" xmlns:y="abcd"><unclosetag>one</x:book>`,
			wantErr: true,
		},
		{
			name:  "matchingQName",
			input: `<x:book xmlns:x="abcd" xmlns:y="abcd">one</x:book>`,
		},
		{
			name:    "mismatchedPrefix",
			input:   `<x:book xmlns:x="abcd" xmlns:y="abcd">one</y:book>`,
			wantErr: true,
		},
		{
			name:    "mismatchedPrefixOrder",
			input:   `<x:book xmlns:y="abcd" xmlns:x="abcd">one</y:book>`,
			wantErr: true,
		},
		{
			name:    "unboundPrefix",
			input:   `<x:book xmlns:x="abcd">one</y:book>`,
			wantErr: true,
		},
		{
			name:    "noNamespaceDecl",
			input:   `<x:book>one</y:book>`,
			wantErr: true,
		},
		{
			name:    "mismatchedNoPrefix",
			input:   `<xbook>one</ybook>`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := readAllTokens(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("input %q error = nil, want %v", tc.input, errMismatchedEndTag)
				}
				if !errors.Is(err, errMismatchedEndTag) {
					t.Fatalf("input %q error = %v, want %v", tc.input, err, errMismatchedEndTag)
				}
				return
			}
			if err != nil {
				t.Fatalf("input %q error = %v, want nil", tc.input, err)
			}
		})
	}
}

func TestDecoderSyntaxError(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<a></b>`))
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("start error = %v", err)
	}
	_, err := reader.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var syntax *SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("error type = %T, want *SyntaxError", err)
	}
}

func TestDecoderSyntaxErrorLineNumber(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<P>Foo<P>\n\n<P>Bar</>\n"))
	var err error
	reader := newTokenReader(dec)
	for err == nil {
		_, err = reader.Next()
	}
	var syntax *SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("error type = %T, want *SyntaxError", err)
	}
	if syntax.Line != 3 {
		t.Fatalf("line = %d, want 3", syntax.Line)
	}
}

func TestDecoderIncompleteRootSyntaxErrors(t *testing.T) {
	tests := []string{
		"<root>",
		"<root><foo>",
		"<root><foo></foo>",
	}
	for _, input := range tests {
		dec := NewDecoder(strings.NewReader(input))
		var err error
		reader := newTokenReader(dec)
		for err == nil {
			_, err = reader.Next()
		}
		if errors.Is(err, io.EOF) {
			t.Fatalf("input %q error = io.EOF, want SyntaxError", input)
		}
		var syntax *SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("input %q error type = %T, want *SyntaxError", input, err)
		}
	}
}

func readAllTokens(input string) error {
	dec := NewDecoder(strings.NewReader(input))
	reader := newTokenReader(dec)
	for {
		_, err := reader.Next()
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
}

func TestDecoderLimits(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<a><b/></a>`), MaxDepth(1))
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if _, err := reader.Next(); err == nil {
		t.Fatalf("expected depth error, got nil")
	}

	dec = NewDecoder(strings.NewReader(`<a b="1" c="2"/>`), MaxAttrs(1))
	reader = newTokenReader(dec)
	if _, err := reader.Next(); err == nil {
		t.Fatalf("expected attr limit error, got nil")
	}
}

func TestDecoderInternStats(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root><root/></root>`))
	reader := newTokenReader(dec)
	for {
		_, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("ReadToken error = %v", err)
		}
	}
	stats := dec.interner.stats
	if stats.Count == 0 {
		t.Fatalf("intern count = 0, want > 0")
	}
	if stats.Hits == 0 {
		t.Fatalf("intern hits = 0, want > 0")
	}
}

func TestDecoderNilAccessors(t *testing.T) {
	var dec *Decoder
	if got := dec.PeekKind(); got != KindNone {
		t.Fatalf("PeekKind = %v, want %v", got, KindNone)
	}
	if got := dec.InputOffset(); got != 0 {
		t.Fatalf("InputOffset = %d, want 0", got)
	}
	if got := dec.StackPointer(); got != "" {
		t.Fatalf("StackPointer = %q, want empty", got)
	}
	if err := dec.SkipValue(); !errors.Is(err, errNilReader) {
		t.Fatalf("SkipValue error = %v, want %v", err, errNilReader)
	}
	if _, ok := dec.Options().ResolveEntities(); ok {
		t.Fatalf("Options ResolveEntities = true, want false")
	}
}

func TestPeekKindBranches(t *testing.T) {
	dec := &Decoder{pendingTokenValid: true, pendingToken: rawToken{kind: KindPI}}
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
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	if got := dec.PeekKind(); got != KindCharData {
		t.Fatalf("PeekKind CDATA = %v, want %v", got, KindCharData)
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

func TestHasAttrExpansion(t *testing.T) {
	tok := rawToken{attrRawNeeds: []bool{false, false}}
	if hasAttrExpansion(tok) {
		t.Fatalf("hasAttrExpansion = true, want false")
	}
	tok.attrRawNeeds[1] = true
	if !hasAttrExpansion(tok) {
		t.Fatalf("hasAttrExpansion = false, want true")
	}
}

func TestSpanStringEmpty(t *testing.T) {
	dec := &Decoder{}
	if got := dec.spanString(span{}); got != "" {
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
	reader := newTokenReader(dec)
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("ReadToken after SkipValue = %v, want io.EOF", err)
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

func TestDetectXMLDeclEncodingUTF8Variants(t *testing.T) {
	tests := []string{
		`<?xml encoding="UtF-8" version="1.0"?><root/>`,
		`<?xml encoding="UTF-8" version="1.0"?><root/>`,
		`<?xml encoding="utf-8" version="1.0"?><root/>`,
	}
	for _, input := range tests {
		reader := bufio.NewReader(strings.NewReader(input))
		label, err := detectXMLDeclEncoding(reader)
		if err != nil {
			t.Fatalf("detectXMLDeclEncoding error = %v", err)
		}
		if label != "" {
			t.Fatalf("detectXMLDeclEncoding label = %q, want empty", label)
		}
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

func TestWrapCharsetReaderFromBufioNoEncoding(t *testing.T) {
	bufReader := bufio.NewReader(strings.NewReader("<root/>"))
	reader, err := wrapCharsetReaderFromBufio(bufReader, nil)
	if err != nil {
		t.Fatalf("wrapCharsetReaderFromBufio error = %v", err)
	}
	if reader != bufReader {
		t.Fatalf("wrapCharsetReaderFromBufio reader = %T, want *bufio.Reader", reader)
	}
}

func TestNormalizeLimit(t *testing.T) {
	if got := normalizeLimit(-1); got != 0 {
		t.Fatalf("normalizeLimit(-1) = %d, want 0", got)
	}
	if got := normalizeLimit(3); got != 3 {
		t.Fatalf("normalizeLimit(3) = %d, want 3", got)
	}
}

func TestDecoderUtilities(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>\n<child attr=\"v\"/></root>"), ResolveEntities(true))
	if value, ok := dec.Options().ResolveEntities(); !ok || !value {
		t.Fatalf("Options ResolveEntities = %v, want true", value)
	}

	reader := newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	if tok.Line != 1 || tok.Column != 1 {
		t.Fatalf("root line/column = %d/%d, want 1/1", tok.Line, tok.Column)
	}
	if got := string(dec.unreadBuffer()); !strings.HasPrefix(got, "\n<child") {
		t.Fatalf("UnreadBuffer = %q, want prefix \\n<child", got)
	}
	if dec.InputOffset() != int64(len("<root>")) {
		t.Fatalf("InputOffset = %d, want %d", dec.InputOffset(), len("<root>"))
	}

	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken child error = %v", err)
	}
	if tok.Kind == KindCharData {
		tok, err = reader.Next()
		if err != nil {
			t.Fatalf("ReadToken child start error = %v", err)
		}
	}
	if tok.Kind != KindStartElement {
		t.Fatalf("child kind = %v, want %v", tok.Kind, KindStartElement)
	}
	if tok.Line != 2 || tok.Column != 1 {
		t.Fatalf("child line/column = %d/%d, want 2/1", tok.Line, tok.Column)
	}
	if len(dec.stack) < 2 {
		t.Fatalf("stack depth = %d, want >= 2", len(dec.stack))
	}
	if got := string(dec.spanBytes(dec.stack[0].name.Local)); got != "root" {
		t.Fatalf("stack[0] name = %q, want root", got)
	}
	if got := string(dec.spanBytes(dec.stack[1].name.Local)); got != "child" {
		t.Fatalf("stack[1] name = %q, want child", got)
	}
	if got := dec.StackPointer(); got != "/root[1]/child[1]" {
		t.Fatalf("StackPointer = %q, want /root[1]/child[1]", got)
	}

	for {
		_, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("ReadToken error = %v", err)
		}
	}
	if dec.unreadBuffer() != nil {
		t.Fatalf("UnreadBuffer at EOF = %v, want nil", dec.unreadBuffer())
	}
}

func TestDebugPoisonSpans(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root><child/></root>"), DebugPoisonSpans(true))
	var tok rawToken
	if err := dec.readTokenIntoRaw(&tok); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	rawSpan := tok.raw
	if rawSpan.bytes() == nil {
		t.Fatalf("span bytes returned nil for current token")
	}
	if err := dec.readTokenIntoRaw(&tok); err != nil {
		t.Fatalf("ReadToken child error = %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic for stale token")
		}
	}()
	_ = rawSpan.bytes()
}

func TestSpanStringStableAndUnstable(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>text</root>"))
	var tok rawToken
	if err := dec.readTokenIntoRaw(&tok); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	if tok.name.Local.buf == nil || !tok.name.Local.buf.stable {
		t.Fatalf("expected stable name buffer")
	}
	if got := dec.spanString(tok.name.Local); got != "root" {
		t.Fatalf("SpanString root = %q, want root", got)
	}

	if err := dec.readTokenIntoRaw(&tok); err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if tok.kind != KindCharData {
		t.Fatalf("text kind = %v, want %v", tok.kind, KindCharData)
	}
	if tok.text.buf == nil || tok.text.buf.stable {
		t.Fatalf("expected unstable text buffer")
	}
	if got := dec.spanString(tok.text); got != "text" {
		t.Fatalf("SpanString text = %q, want text", got)
	}
}

func TestReadTokenIntoWithBuffers(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root a="1" b="2"></root>`))
	var tok Token
	buf := TokenBuffer{
		Attrs:     make([]Attr, 0, 2),
		AttrName:  make([]byte, 0, 8),
		AttrValue: make([]byte, 0, 8),
	}
	if err := dec.ReadTokenInto(&tok, &buf); err != nil {
		t.Fatalf("ReadTokenInto error = %v", err)
	}
	if tok.Kind != KindStartElement {
		t.Fatalf("kind = %v, want %v", tok.Kind, KindStartElement)
	}
	if got := len(tok.Attrs); got != 2 {
		t.Fatalf("attrs len = %d, want 2", got)
	}
	if got := string(tok.Attrs[0].Name); got != "a" {
		t.Fatalf("attr[0] name = %q, want a", got)
	}
	if got := string(tok.Attrs[0].Value); got != "1" {
		t.Fatalf("attr[0] value = %q, want 1", got)
	}
}

func TestDecoderSkipValueBranches(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root/>"))
	reader := newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	if tok.Kind != KindStartElement {
		t.Fatalf("root kind = %v, want %v", tok.Kind, KindStartElement)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue self-closing error = %v", err)
	}
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("ReadToken EOF = %v, want io.EOF", err)
	}
	if err := dec.SkipValue(); !errors.Is(err, io.EOF) {
		t.Fatalf("SkipValue EOF = %v, want io.EOF", err)
	}

	dec = NewDecoder(strings.NewReader("<root><child/></root>"))
	reader = newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken root start error = %v", err)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue element error = %v", err)
	}
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("ReadToken EOF after skip = %v, want io.EOF", err)
	}

	dec = NewDecoder(strings.NewReader("<root>text</root>"))
	reader = newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken root start error = %v", err)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue after text error = %v", err)
	}
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken end error = %v", err)
	}
	if tok.Kind != KindEndElement {
		t.Fatalf("end kind = %v, want %v", tok.Kind, KindEndElement)
	}

	dec = NewDecoder(strings.NewReader("<root><!--a--><!--b--></root>"), EmitComments(true))
	reader = newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken root start error = %v", err)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken comment error = %v", err)
	}
	if err := dec.SkipValue(); err != nil {
		t.Fatalf("SkipValue comment error = %v", err)
	}
	tok, err = reader.Next()
	if err != nil {
		t.Fatalf("ReadToken end error = %v", err)
	}
	if tok.Kind != KindEndElement {
		t.Fatalf("end kind = %v, want %v", tok.Kind, KindEndElement)
	}
}

func TestCharDataInvalidSequence(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>]]></root>"))
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	_, err := reader.Next()
	if !errors.Is(err, errInvalidToken) {
		t.Fatalf("char data error = %v, want %v", err, errInvalidToken)
	}
}

func TestWhitespaceEntityOutsideRoot(t *testing.T) {
	dec := NewDecoder(strings.NewReader("&#x20;<root/>"))
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken whitespace error = %v", err)
	}
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	if tok.Kind != KindStartElement {
		t.Fatalf("root kind = %v, want %v", tok.Kind, KindStartElement)
	}
}

func TestCharDataEOF(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>text"))
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if tok.Kind != KindCharData {
		t.Fatalf("text kind = %v, want %v", tok.Kind, KindCharData)
	}
	_, err = reader.Next()
	if !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("ReadToken EOF error = %v, want %v", err, errUnexpectedEOF)
	}
}

func TestCharDataMaxTokenSize(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>abcdefgh</root>"), MaxTokenSize(6))
	reader := newTokenReader(dec)
	if _, err := reader.Next(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	_, err := reader.Next()
	if !errors.Is(err, errTokenTooLarge) {
		t.Fatalf("ReadToken error = %v, want %v", err, errTokenTooLarge)
	}
}

func TestUnicodeNameWithChunkReader(t *testing.T) {
	reader := &chunkReader{data: []byte("<\u00e9/>")}
	dec := NewDecoder(reader)
	tokReader := newTokenReader(dec)
	tok, err := tokReader.Next()
	if err != nil {
		t.Fatalf("ReadToken error = %v", err)
	}
	if got := string(tok.Name); got != "\u00e9" {
		t.Fatalf("name = %q, want \u00e9", got)
	}
}

func TestUnterminatedPI(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<?pi"))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("ReadToken error = %v, want %v", err, errUnexpectedEOF)
	}
}

func TestUnterminatedComment(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<!--oops"))
	reader := newTokenReader(dec)
	_, err := reader.Next()
	if !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("ReadToken error = %v, want %v", err, errUnexpectedEOF)
	}
}

func TestPIWithoutData(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<?pi?><root/>"), EmitPI(true))
	reader := newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken PI error = %v", err)
	}
	if tok.Kind != KindPI {
		t.Fatalf("PI kind = %v, want %v", tok.Kind, KindPI)
	}
	if got := string(tok.Text); got != "pi" {
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
		reader := newTokenReader(dec)
		_, err := reader.Next()
		if !errors.Is(err, errInvalidName) {
			t.Fatalf("ReadToken(%q) error = %v, want %v", input, err, errInvalidName)
		}
	}
}

func TestInvalidUTF8Name(t *testing.T) {
	reader := &chunkReader{data: []byte("<\xc3/>")}
	dec := NewDecoder(reader)
	tokReader := newTokenReader(dec)
	_, err := tokReader.Next()
	if !errors.Is(err, errInvalidChar) {
		t.Fatalf("ReadToken error = %v, want %v", err, errInvalidChar)
	}
}

func TestDirectiveToken(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<!DOCTYPE root [<!ELEMENT root EMPTY>]><root/>`), EmitDirectives(true))
	reader := newTokenReader(dec)
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("ReadToken directive error = %v", err)
	}
	if tok.Kind != KindDirective {
		t.Fatalf("directive kind = %v, want %v", tok.Kind, KindDirective)
	}
	if got := string(tok.Text); !strings.HasPrefix(got, "DOCTYPE root") {
		t.Fatalf("directive text = %q, want DOCTYPE root...", got)
	}
}

func TestCharsetReader(t *testing.T) {
	reader, err := wrapCharsetReaderFromBufio(bufio.NewReader(strings.NewReader("\ufeff<root/>")), nil)
	if err != nil {
		t.Fatalf("wrapCharsetReaderFromBufio BOM error = %v", err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll error = %v", err)
	}
	if string(out) != "<root/>" {
		t.Fatalf("ReadAll = %q, want <root/>", out)
	}

	_, err = wrapCharsetReaderFromBufio(bufio.NewReader(bytes.NewReader([]byte{0xFE, 0xFF, 0x00, 0x3C})), nil)
	if !errors.Is(err, errUnsupportedEncoding) {
		t.Fatalf("wrapCharsetReaderFromBufio error = %v, want %v", err, errUnsupportedEncoding)
	}

	called := false
	reader, err = wrapCharsetReaderFromBufio(bufio.NewReader(bytes.NewReader([]byte{0xFE, 0xFF, 0x00, 0x3C})), func(label string, r io.Reader) (io.Reader, error) {
		called = true
		if label == "" {
			t.Fatalf("label is empty")
		}
		return strings.NewReader("<root/>"), nil
	})
	if err != nil {
		t.Fatalf("wrapCharsetReaderFromBufio custom error = %v", err)
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

func TestDetectEncodingBufferFull(t *testing.T) {
	reader := bufio.NewReaderSize(bytes.NewReader([]byte{0xFF, 0xFE, 0x00, 0x00}), 2)
	label, err := detectEncoding(reader)
	if err != nil {
		t.Fatalf("detectEncoding error = %v", err)
	}
	if label != "utf-16" {
		t.Fatalf("label = %q, want utf-16", label)
	}
}

func TestDetectXMLDeclEncodingTruncated(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(`<?xml version="1.0" encoding="ISO-8859-1"`))
	label, err := detectXMLDeclEncoding(reader)
	if err != nil {
		t.Fatalf("detectXMLDeclEncoding error = %v", err)
	}
	if label != "" {
		t.Fatalf("detectXMLDeclEncoding label = %q, want empty", label)
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

type errReader struct {
	err error
}

func (r errReader) Read(p []byte) (int, error) {
	return 0, r.err
}

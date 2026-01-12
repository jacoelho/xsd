package xmltext

import (
	"errors"
	"strings"
	"testing"
)

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

func TestAppendTokenValueInvalidOrder(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("abcdef")}}
	cursor := 5
	tok := Token{
		Kind: KindCharData,
		Raw:  makeSpan(&dec.buf, 2, 4),
		Text: makeSpan(&dec.buf, 2, 4),
	}
	if _, err := dec.appendTokenValue(nil, tok, &cursor); !errors.Is(err, errInvalidToken) {
		t.Fatalf("appendTokenValue error = %v, want %v", err, errInvalidToken)
	}
}

func TestAppendTokenValueAttrMismatch(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("abcdef")}}
	cursor := 0
	tok := Token{
		Kind:    KindStartElement,
		Raw:     makeSpan(&dec.buf, 0, 4),
		AttrRaw: []Span{{}},
	}
	if _, err := dec.appendTokenValue(nil, tok, &cursor); !errors.Is(err, errInvalidToken) {
		t.Fatalf("appendTokenValue error = %v, want %v", err, errInvalidToken)
	}
}

func TestAppendTokenValueAttrSpanOutOfRange(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("<a b=\"c\">")}}
	cursor := 0
	tok := Token{
		Kind:         KindStartElement,
		Raw:          makeSpan(&dec.buf, 0, len(dec.buf.data)),
		AttrRaw:      []Span{{Start: -1, End: 2, buf: &dec.buf}},
		AttrRawNeeds: []bool{true},
		Attrs:        []AttrSpan{{ValueSpan: makeSpan(&dec.buf, 0, 1)}},
	}
	if _, err := dec.appendTokenValue(nil, tok, &cursor); !errors.Is(err, errInvalidToken) {
		t.Fatalf("appendTokenValue error = %v, want %v", err, errInvalidToken)
	}
}

func TestAppendTokenValueNilRaw(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("abc")}}
	cursor := 1
	tok := Token{Kind: KindCharData}
	out, err := dec.appendTokenValue(nil, tok, &cursor)
	if err != nil {
		t.Fatalf("appendTokenValue error = %v", err)
	}
	if out != nil {
		t.Fatalf("appendTokenValue out = %v, want nil", out)
	}
	if cursor != 1 {
		t.Fatalf("cursor = %d, want 1", cursor)
	}
}

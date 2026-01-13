package xmltext

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadValueVariants(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root>x&amp;y</root>"))
	var tok Token
	var buf TokenBuffer
	scratch := make([]byte, 64)
	if err := dec.ReadTokenInto(&tok, &buf); err != nil {
		t.Fatalf("ReadTokenInto root error = %v", err)
	}
	n, err := dec.ReadValueInto(scratch)
	if err != nil {
		t.Fatalf("ReadValueInto raw error = %v", err)
	}
	if string(scratch[:n]) != "x&amp;y" {
		t.Fatalf("ReadValueInto raw = %q, want x&amp;y", scratch[:n])
	}

	dec = NewDecoder(strings.NewReader("<root>x&amp;y</root>"), ResolveEntities(true))
	if err := dec.ReadTokenInto(&tok, &buf); err != nil {
		t.Fatalf("ReadTokenInto root resolve error = %v", err)
	}
	n, err = dec.ReadValueInto(scratch)
	if err != nil {
		t.Fatalf("ReadValueInto resolve error = %v", err)
	}
	if string(scratch[:n]) != "x&y" {
		t.Fatalf("ReadValueInto resolve = %q, want x&y", scratch[:n])
	}

	dec = NewDecoder(strings.NewReader("<root><![CDATA[x]]></root>"), ResolveEntities(true))
	if err := dec.ReadTokenInto(&tok, &buf); err != nil {
		t.Fatalf("ReadTokenInto root CDATA error = %v", err)
	}
	n, err = dec.ReadValueInto(scratch)
	if err != nil {
		t.Fatalf("ReadValueInto CDATA error = %v", err)
	}
	if string(scratch[:n]) != "<![CDATA[x]]>" {
		t.Fatalf("ReadValueInto CDATA = %q, want <![CDATA[x]]>", scratch[:n])
	}

	dec = NewDecoder(strings.NewReader("<!--c--><root/>"), EmitComments(true))
	n, err = dec.ReadValueInto(scratch)
	if err != nil {
		t.Fatalf("ReadValueInto comment error = %v", err)
	}
	if string(scratch[:n]) != "<!--c-->" {
		t.Fatalf("ReadValueInto comment = %q, want <!--c-->", scratch[:n])
	}
}

func TestReadValueLateExpansion(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root><a>ok</a><b>1 &amp; 2</b></root>"), ResolveEntities(true))
	scratch := make([]byte, 128)
	n, err := dec.ReadValueInto(scratch)
	if err != nil {
		t.Fatalf("ReadValueInto error = %v", err)
	}
	if string(scratch[:n]) != "<root><a>ok</a><b>1 & 2</b></root>" {
		t.Fatalf("ReadValueInto = %q, want expanded subtree", scratch[:n])
	}
}

func TestReadValueNoExpansion(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root><a>ok</a></root>"), ResolveEntities(true))
	scratch := make([]byte, 64)
	n, err := dec.ReadValueInto(scratch)
	if err != nil {
		t.Fatalf("ReadValueInto error = %v", err)
	}
	if string(scratch[:n]) != "<root><a>ok</a></root>" {
		t.Fatalf("ReadValueInto = %q, want raw subtree", scratch[:n])
	}
}

func TestReadValueIntoShortBuffer(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root><a>123</a><b/></root>"))
	var tok Token
	var buf TokenBuffer
	if err := dec.ReadTokenInto(&tok, &buf); err != nil {
		t.Fatalf("ReadTokenInto root error = %v", err)
	}
	scratch := make([]byte, 4)
	n, err := dec.ReadValueInto(scratch)
	if !errors.Is(err, io.ErrShortBuffer) {
		t.Fatalf("ReadValueInto error = %v, want %v", err, io.ErrShortBuffer)
	}
	if n != len(scratch) {
		t.Fatalf("ReadValueInto n = %d, want %d", n, len(scratch))
	}
	if err := dec.ReadTokenInto(&tok, &buf); err != nil {
		t.Fatalf("ReadTokenInto after short buffer error = %v", err)
	}
	if tok.Kind != KindStartElement || string(tok.Name) != "b" {
		t.Fatalf("after short buffer token = %v %q, want start element b", tok.Kind, tok.Name)
	}
}

func TestAppendTokenValueInvalidOrder(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("abcdef")}}
	cursor := 5
	tok := rawToken{
		kind: KindCharData,
		raw:  makeSpan(&dec.buf, 2, 4),
		text: makeSpan(&dec.buf, 2, 4),
	}
	writer := bufferWriter{dst: make([]byte, 0)}
	if err := dec.appendTokenValue(&writer, tok, &cursor); !errors.Is(err, errInvalidToken) {
		t.Fatalf("appendTokenValue error = %v, want %v", err, errInvalidToken)
	}
}

func TestAppendTokenValueAttrMismatch(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("abcdef")}}
	cursor := 0
	tok := rawToken{
		kind:    KindStartElement,
		raw:     makeSpan(&dec.buf, 0, 4),
		attrRaw: []span{{}},
	}
	writer := bufferWriter{dst: make([]byte, 0)}
	if err := dec.appendTokenValue(&writer, tok, &cursor); !errors.Is(err, errInvalidToken) {
		t.Fatalf("appendTokenValue error = %v, want %v", err, errInvalidToken)
	}
}

func TestAppendTokenValueAttrSpanOutOfRange(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("<a b=\"c\">")}}
	cursor := 0
	tok := rawToken{
		kind:         KindStartElement,
		raw:          makeSpan(&dec.buf, 0, len(dec.buf.data)),
		attrRaw:      []span{{Start: -1, End: 2, buf: &dec.buf}},
		attrRawNeeds: []bool{true},
		attrs:        []attrSpan{{ValueSpan: makeSpan(&dec.buf, 0, 1)}},
	}
	writer := bufferWriter{dst: make([]byte, 0)}
	if err := dec.appendTokenValue(&writer, tok, &cursor); !errors.Is(err, errInvalidToken) {
		t.Fatalf("appendTokenValue error = %v, want %v", err, errInvalidToken)
	}
}

func TestAppendTokenValueNilRaw(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("abc")}}
	cursor := 1
	tok := rawToken{kind: KindCharData}
	writer := bufferWriter{dst: make([]byte, 0)}
	err := dec.appendTokenValue(&writer, tok, &cursor)
	if err != nil {
		t.Fatalf("appendTokenValue error = %v", err)
	}
	if writer.n != 0 {
		t.Fatalf("appendTokenValue wrote = %d, want 0", writer.n)
	}
	if cursor != 1 {
		t.Fatalf("cursor = %d, want 1", cursor)
	}
}

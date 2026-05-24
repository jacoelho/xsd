package xsd

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type eofWithDataReader struct {
	data []byte
	done bool
}

func (r *eofWithDataReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, r.data), io.EOF
}

func TestXMLStreamParserConsumesBytesReturnedWithEOF(t *testing.T) {
	names := newByteStringCache()
	values := newByteStringCache()
	p := new(xmlStreamParser)
	p.reset(&eofWithDataReader{data: []byte(`<root/>`)}, &names, &values)

	tok, err := p.next()
	if err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	if tok.kind != streamTokenStart || tok.start.Name.Local != "root" {
		t.Fatalf("first token = %+v, want root start", tok)
	}

	tok, err = p.next()
	if err != nil {
		t.Fatalf("next root end error = %v", err)
	}
	if tok.kind != streamTokenEnd || tok.end.Name.Local != "root" {
		t.Fatalf("second token = %+v, want root end", tok)
	}

	_, err = p.next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("final error = %v, want EOF", err)
	}
}

func TestXMLStreamParserSkipsCommentsByDefault(t *testing.T) {
	names := newByteStringCache()
	values := newByteStringCache()
	p := new(xmlStreamParser)
	p.reset(strings.NewReader(`<root><!--`+strings.Repeat("x", 1<<20)+`--><v>1</v></root>`), &names, &values)

	tok, err := p.next()
	if err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	if tok.kind != streamTokenStart || tok.start.Name.Local != "root" {
		t.Fatalf("first token = %+v, want root start", tok)
	}

	tok, err = p.next()
	if err != nil {
		t.Fatalf("next child start error = %v", err)
	}
	if tok.kind == streamTokenComment {
		t.Fatal("default parser emitted comment")
	}
	if tok.kind != streamTokenStart || tok.start.Name.Local != "v" {
		t.Fatalf("second token = %+v, want v start", tok)
	}
	if cap(p.directive) != 0 {
		t.Fatalf("comment data retained with cap %d", cap(p.directive))
	}
}

func TestXMLStreamParserEmitsCommentsWhenEnabled(t *testing.T) {
	names := newByteStringCache()
	values := newByteStringCache()
	p := new(xmlStreamParser)
	p.reset(strings.NewReader(`<root><!-- note --></root>`), &names, &values)
	p.emitComments = true

	if _, err := p.next(); err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	tok, err := p.next()
	if err != nil {
		t.Fatalf("next comment error = %v", err)
	}
	if tok.kind != streamTokenComment || string(tok.directive) != " note " {
		t.Fatalf("comment token = %+v", tok)
	}
}

func TestXMLStreamParserHandlesBareCRText(t *testing.T) {
	done := make(chan struct{})
	var tok streamToken
	var err error
	go func() {
		names := newByteStringCache()
		values := newByteStringCache()
		p := new(xmlStreamParser)
		p.reset(strings.NewReader("<root>a\rb</root>"), &names, &values)
		if _, err = p.next(); err != nil {
			close(done)
			return
		}
		tok, err = p.next()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("parser timed out")
	}
	if err != nil {
		t.Fatalf("next char data error = %v", err)
	}
	if tok.kind != streamTokenCharData || string(tok.data) != "a\nb" {
		t.Fatalf("char data token = %+v", tok)
	}
}

func TestXMLStreamParserNormalizesCDATALineEndings(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "bare_cr", in: "<root><![CDATA[a\rb]]></root>"},
		{name: "crlf", in: "<root><![CDATA[a\r\nb]]></root>"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			names := newByteStringCache()
			values := newByteStringCache()
			p := new(xmlStreamParser)
			p.reset(strings.NewReader(test.in), &names, &values)
			if _, err := p.next(); err != nil {
				t.Fatalf("next root start error = %v", err)
			}
			tok, err := p.next()
			if err != nil {
				t.Fatalf("next CDATA error = %v", err)
			}
			if tok.kind != streamTokenCharData || !tok.cdata || string(tok.data) != "a\nb" {
				t.Fatalf("CDATA token = %+v", tok)
			}
		})
	}
}

func TestXMLStreamParserRejectsInvalidSkippedComment(t *testing.T) {
	names := newByteStringCache()
	values := newByteStringCache()
	p := new(xmlStreamParser)
	p.reset(strings.NewReader(`<root><!-- invalid -- comment --></root>`), &names, &values)

	if _, err := p.next(); err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	_, err := p.next()
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("invalid comment error = %v", err)
	}
}

func TestXMLStreamParserChunksLargeCDATA(t *testing.T) {
	names := newByteStringCache()
	values := newByteStringCache()
	data := strings.Repeat("x", 70*1024)
	p := new(xmlStreamParser)
	p.reset(strings.NewReader(`<root><![CDATA[`+data+`]]></root>`), &names, &values)

	if _, err := p.next(); err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	total := 0
	chunks := 0
	for {
		tok, err := p.next()
		if err != nil {
			t.Fatalf("next CDATA chunk error = %v", err)
		}
		if tok.kind == streamTokenEnd {
			break
		}
		if tok.kind != streamTokenCharData || !tok.cdata {
			t.Fatalf("token = %+v, want CDATA char data", tok)
		}
		if len(tok.data) > len(p.br.buf) {
			t.Fatalf("CDATA chunk len = %d, want <= %d", len(tok.data), len(p.br.buf))
		}
		total += len(tok.data)
		chunks++
	}
	if total != len(data) {
		t.Fatalf("CDATA total = %d, want %d", total, len(data))
	}
	if chunks < 2 {
		t.Fatalf("CDATA chunks = %d, want multiple chunks", chunks)
	}
}

func TestLargeCDATAValidatesWithoutAccumulatingParserBuffer(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`)
	data := strings.Repeat("x", 70*1024)
	if err := engine.Validate(strings.NewReader(`<root><![CDATA[` + data + `]]></root>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

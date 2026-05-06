package xsd

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestXMLStreamParserSkipsCommentsByDefault(t *testing.T) {
	names := newByteStringCache(512, 256)
	values := newByteStringCache(512, 256)
	p := newXMLStreamParser(strings.NewReader(`<root><!--`+strings.Repeat("x", 1<<20)+`--><v>1</v></root>`), &names, &values)

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
	names := newByteStringCache(512, 256)
	values := newByteStringCache(512, 256)
	p := newXMLStreamParser(strings.NewReader(`<root><!-- note --></root>`), &names, &values)
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
		names := newByteStringCache(512, 256)
		values := newByteStringCache(512, 256)
		p := newXMLStreamParser(strings.NewReader("<root>a\rb</root>"), &names, &values)
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

func TestXMLStreamParserRejectsInvalidSkippedComment(t *testing.T) {
	names := newByteStringCache(512, 256)
	values := newByteStringCache(512, 256)
	p := newXMLStreamParser(strings.NewReader(`<root><!-- invalid -- comment --></root>`), &names, &values)

	if _, err := p.next(); err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	_, err := p.next()
	if err == nil || err == io.EOF {
		t.Fatalf("invalid comment error = %v", err)
	}
}

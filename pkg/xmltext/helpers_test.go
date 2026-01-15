package xmltext

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestUnescapeInto(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root attr="a&amp;b">x&amp;y</root>`))
	var tok Token
	if err := dec.ReadTokenInto(&tok); err != nil {
		t.Fatalf("ReadTokenInto error = %v", err)
	}
	attr := tok.Attrs[0]
	if got := string(attr.Value); got != "a&amp;b" {
		t.Fatalf("attr value = %q, want a&amp;b", got)
	}
	scratch := make([]byte, len(attr.Value))
	n, err := dec.UnescapeInto(scratch, attr.Value)
	if err != nil {
		t.Fatalf("UnescapeInto attr error = %v", err)
	}
	if string(scratch[:n]) != "a&b" {
		t.Fatalf("UnescapeInto attr = %q, want a&b", scratch[:n])
	}

	if err := dec.ReadTokenInto(&tok); err != nil {
		t.Fatalf("ReadTokenInto text error = %v", err)
	}
	if got := string(tok.Text); got != "x&amp;y" {
		t.Fatalf("text = %q, want x&amp;y", got)
	}
	scratch = make([]byte, len(tok.Text))
	n, err = dec.UnescapeInto(scratch, tok.Text)
	if err != nil {
		t.Fatalf("UnescapeInto text error = %v", err)
	}
	if string(scratch[:n]) != "x&y" {
		t.Fatalf("UnescapeInto text = %q, want x&y", scratch[:n])
	}
}

func TestUnescapeIntoShortBuffer(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<root/>"))
	buf := make([]byte, 2)
	n, err := dec.UnescapeInto(buf, []byte("a&amp;b"))
	if !errors.Is(err, io.ErrShortBuffer) {
		t.Fatalf("UnescapeInto error = %v, want %v", err, io.ErrShortBuffer)
	}
	if n != len(buf) {
		t.Fatalf("UnescapeInto n = %d, want %d", n, len(buf))
	}
	if string(buf[:n]) != "a&" {
		t.Fatalf("UnescapeInto = %q, want a&", buf[:n])
	}
}

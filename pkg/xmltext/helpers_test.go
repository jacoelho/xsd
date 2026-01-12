package xmltext

import (
	"strings"
	"testing"
)

func TestSpanHelpers(t *testing.T) {
	dec := NewDecoder(strings.NewReader(`<root attr="a&amp;b">x&amp;y</root>`))
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken error = %v", err)
	}
	attr := tok.Attrs[0]
	if got := string(CopyAttrValue(nil, attr)); got != "a&amp;b" {
		t.Fatalf("CopyAttrValue = %q, want a&amp;b", got)
	}
	tok, err = dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if got := string(CopySpan(nil, tok.Text)); got != "x&amp;y" {
		t.Fatalf("CopySpan = %q, want x&amp;y", got)
	}
	unescaped, err := UnescapeInto(nil, tok.Text)
	if err != nil {
		t.Fatalf("UnescapeInto error = %v", err)
	}
	if string(unescaped) != "x&y" {
		t.Fatalf("UnescapeInto = %q, want x&y", unescaped)
	}
}

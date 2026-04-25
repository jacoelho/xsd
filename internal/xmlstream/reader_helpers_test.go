package xmlstream

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/xmltext"
)

func TestPopElementStack(t *testing.T) {
	root := QName{Local: "root"}
	child := QName{Namespace: "urn:test", Local: "child"}
	stack := []elementStackEntry{
		{qname: root, nameID: 1},
		{qname: child, nameID: 2},
	}
	got, rest, err := popElementStack(stack, 2)
	if err != nil {
		t.Fatalf("popElementStack error = %v", err)
	}
	if got.qname != child || got.nameID != 2 {
		t.Fatalf("popped = %#v, want child with NameID 2", got)
	}
	if len(rest) != 1 || rest[0].qname != root || rest[0].nameID != 1 {
		t.Fatalf("rest = %#v, want root with NameID 1", rest)
	}
}

func TestDecodeAttrValueBytesEmptyBuffer(t *testing.T) {
	dec := xmltext.NewDecoder(strings.NewReader("<root/>"))
	var buf []byte
	buf, out, err := decodeAttrValueBytes(dec, buf, []byte("a&amp;b"))
	if err != nil {
		t.Fatalf("decodeAttrValueBytes error = %v", err)
	}
	if string(out) != "a&b" {
		t.Fatalf("decodeAttrValueBytes = %q, want a&b", out)
	}
	if len(buf) == 0 {
		t.Fatalf("buffer len = 0, want > 0")
	}
}

func TestAppendNamespaceValueEmpty(t *testing.T) {
	buf := []byte("x")
	buf, value := appendNamespaceValue(buf, []byte{})
	if value != "" {
		t.Fatalf("appendNamespaceValue = %q, want empty", value)
	}
	if string(buf) != "x" {
		t.Fatalf("buffer = %q, want x", buf)
	}
}

func namespaceDeclsAt(r *Reader, depth int) []NamespaceDecl {
	if r == nil {
		return nil
	}
	return slices.Collect(r.NamespaceDeclsSeq(depth))
}

func namespaceDeclsCurrent(r *Reader) []NamespaceDecl {
	if r == nil {
		return nil
	}
	return slices.Collect(r.CurrentNamespaceDeclsSeq())
}

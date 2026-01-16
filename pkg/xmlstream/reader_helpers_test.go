package xmlstream

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestPopQName(t *testing.T) {
	root := QName{Local: "root"}
	child := QName{Namespace: "urn:test", Local: "child"}
	stack := []QName{root, child}
	got, rest, err := popQName(stack, 2)
	if err != nil {
		t.Fatalf("popQName error = %v", err)
	}
	if got != child {
		t.Fatalf("popQName = %v, want %v", got, child)
	}
	if len(rest) != 1 || rest[0] != root {
		t.Fatalf("rest = %v, want [%v]", rest, root)
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

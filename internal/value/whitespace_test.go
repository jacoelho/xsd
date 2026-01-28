package value

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestNormalizeWhitespaceReplace(t *testing.T) {
	in := []byte("a\tb\nc\rd")
	got := NormalizeWhitespace(runtime.WS_Replace, in, nil)
	want := []byte("a b c d")
	if !bytes.Equal(got, want) {
		t.Fatalf("NormalizeWhitespace(replace) = %q, want %q", string(got), string(want))
	}
}

func TestNormalizeWhitespaceCollapse(t *testing.T) {
	in := []byte("  a\t b \n c  ")
	got := NormalizeWhitespace(runtime.WS_Collapse, in, nil)
	want := []byte("a b c")
	if !bytes.Equal(got, want) {
		t.Fatalf("NormalizeWhitespace(collapse) = %q, want %q", string(got), string(want))
	}
}

func TestTrimXMLWhitespace(t *testing.T) {
	in := []byte("\t abc \n")
	got := TrimXMLWhitespace(in)
	want := []byte("abc")
	if !bytes.Equal(got, want) {
		t.Fatalf("TrimXMLWhitespace() = %q, want %q", string(got), string(want))
	}
}

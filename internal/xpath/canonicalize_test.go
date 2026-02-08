package xpath

import (
	"testing"

	"github.com/jacoelho/xsd/internal/qname"
)

func TestCanonicalizeNodeTestUnprefixed(t *testing.T) {
	test := NodeTest{Local: "item"}
	canon := CanonicalizeNodeTest(test)
	if !canon.NamespaceSpecified {
		t.Fatalf("expected NamespaceSpecified to be true")
	}
	if canon.Namespace != qname.NamespaceEmpty {
		t.Fatalf("expected empty namespace, got %q", canon.Namespace)
	}
}

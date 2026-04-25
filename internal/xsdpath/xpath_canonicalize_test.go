package xsdpath

import (
	"testing"

	"github.com/jacoelho/xsd/internal/xsdlex"
)

func TestCanonicalizeNodeTestUnprefixed(t *testing.T) {
	test := NodeTest{Local: "item"}
	canon := CanonicalizeNodeTest(test)
	if !canon.NamespaceSpecified {
		t.Fatalf("expected NamespaceSpecified to be true")
	}
	if canon.Namespace != xsdlex.NamespaceEmpty {
		t.Fatalf("expected empty namespace, got %q", canon.Namespace)
	}
}

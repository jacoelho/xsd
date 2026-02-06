package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

func TestUnprefixedNodeTestMatchesNoNamespace(t *testing.T) {
	expr, err := xpath.Parse("item", nil, xpath.AttributesDisallowed)
	if err != nil {
		t.Fatalf("parse xpath: %v", err)
	}
	if len(expr.Paths) == 0 || len(expr.Paths[0].Steps) == 0 {
		t.Fatalf("expected parsed xpath steps")
	}
	test := expr.Paths[0].Steps[0].Test
	if nodeTestMatchesQName(test, types.QName{Namespace: "urn:test", Local: "item"}) {
		t.Fatalf("unprefixed node test should not match namespaced element")
	}
	if !nodeTestMatchesQName(test, types.QName{Namespace: types.NamespaceEmpty, Local: "item"}) {
		t.Fatalf("unprefixed node test should match no-namespace element")
	}
}

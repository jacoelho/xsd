package runtimebuild

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestEffectiveAttributeQNameNilSchema(t *testing.T) {
	attr := &types.AttributeDecl{
		Name: types.QName{Local: "attr"},
		Form: types.FormQualified,
	}
	qname := effectiveAttributeQName(nil, attr)
	if qname.Namespace != types.NamespaceEmpty || qname.Local != "attr" {
		t.Fatalf("qname = %v, want empty namespace and local attr", qname)
	}

	attr.SourceNamespace = types.NamespaceURI("urn:src")
	qname = effectiveAttributeQName(nil, attr)
	if qname.Namespace != types.NamespaceURI("urn:src") || qname.Local != "attr" {
		t.Fatalf("qname = %v, want source namespace", qname)
	}
}

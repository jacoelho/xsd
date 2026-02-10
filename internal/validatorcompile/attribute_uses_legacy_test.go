package validatorcompile

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/typeops"
)

func TestEffectiveAttributeQNameNilSchema(t *testing.T) {
	attr := &model.AttributeDecl{
		Name: model.QName{Local: "attr"},
		Form: model.FormQualified,
	}
	qname := typeops.EffectiveAttributeQName(nil, attr)
	if qname.Namespace != model.NamespaceEmpty || qname.Local != "attr" {
		t.Fatalf("qname = %v, want empty namespace and local attr", qname)
	}

	attr.SourceNamespace = model.NamespaceURI("urn:src")
	qname = typeops.EffectiveAttributeQName(nil, attr)
	if qname.Namespace != model.NamespaceURI("urn:src") || qname.Local != "attr" {
		t.Fatalf("qname = %v, want source namespace", qname)
	}
}

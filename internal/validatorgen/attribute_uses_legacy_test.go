package validatorgen

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

func TestEffectiveAttributeQNameNilSchema(t *testing.T) {
	attr := &model.AttributeDecl{
		Name: model.QName{Local: "attr"},
		Form: model.FormQualified,
	}
	qname := typeresolve.EffectiveAttributeQName(nil, attr)
	if qname.Namespace != model.NamespaceEmpty || qname.Local != "attr" {
		t.Fatalf("qname = %v, want empty namespace and local attr", qname)
	}

	attr.SourceNamespace = model.NamespaceURI("urn:src")
	qname = typeresolve.EffectiveAttributeQName(nil, attr)
	if qname.Namespace != model.NamespaceURI("urn:src") || qname.Local != "attr" {
		t.Fatalf("qname = %v, want source namespace", qname)
	}
}

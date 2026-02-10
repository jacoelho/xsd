package semanticresolve

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestResolveComplexTypeBase(t *testing.T) {
	schema := parser.NewSchema()
	baseQName := model.QName{Namespace: "urn:test", Local: "Base"}
	base := model.NewComplexType(baseQName, model.NamespaceURI("urn:test"))
	base.SetContent(&model.ElementContent{})
	schema.TypeDefs[baseQName] = base

	derivedQName := model.QName{Namespace: "urn:test", Local: "Derived"}
	derived := model.NewComplexType(derivedQName, model.NamespaceURI("urn:test"))
	derived.SetContent(&model.ComplexContent{Base: baseQName})

	resolver := NewResolver(schema)
	if err := resolver.resolveComplexTypeBase(derivedQName, derived); err != nil {
		t.Fatalf("resolveComplexTypeBase: %v", err)
	}
	if derived.ResolvedBase != base {
		t.Fatalf("resolved base = %v, want %v", derived.ResolvedBase, base)
	}
}

func TestResolveComplexTypeAttributes(t *testing.T) {
	schema := parser.NewSchema()
	agQName := model.QName{Namespace: "urn:test", Local: "ag"}
	schema.AttributeGroups[agQName] = &model.AttributeGroup{
		Name:            agQName,
		SourceNamespace: model.NamespaceURI("urn:test"),
	}

	ctQName := model.QName{Namespace: "urn:test", Local: "ct"}
	ct := model.NewComplexType(ctQName, model.NamespaceURI("urn:test"))
	ct.AttrGroups = []model.QName{agQName}

	resolver := NewResolver(schema)
	if err := resolver.resolveComplexTypeAttributes(ctQName, ct); err != nil {
		t.Fatalf("resolveComplexTypeAttributes: %v", err)
	}
}

package resolver

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestResolveComplexTypeBase(t *testing.T) {
	schema := parser.NewSchema()
	baseQName := types.QName{Namespace: "urn:test", Local: "Base"}
	base := types.NewComplexType(baseQName, types.NamespaceURI("urn:test"))
	base.SetContent(&types.ElementContent{})
	schema.TypeDefs[baseQName] = base

	derivedQName := types.QName{Namespace: "urn:test", Local: "Derived"}
	derived := types.NewComplexType(derivedQName, types.NamespaceURI("urn:test"))
	derived.SetContent(&types.ComplexContent{Base: baseQName})

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
	agQName := types.QName{Namespace: "urn:test", Local: "ag"}
	schema.AttributeGroups[agQName] = &types.AttributeGroup{
		Name:            agQName,
		SourceNamespace: types.NamespaceURI("urn:test"),
	}

	ctQName := types.QName{Namespace: "urn:test", Local: "ct"}
	ct := types.NewComplexType(ctQName, types.NamespaceURI("urn:test"))
	ct.AttrGroups = []types.QName{agQName}

	resolver := NewResolver(schema)
	if err := resolver.resolveComplexTypeAttributes(ctQName, ct); err != nil {
		t.Fatalf("resolveComplexTypeAttributes: %v", err)
	}
}

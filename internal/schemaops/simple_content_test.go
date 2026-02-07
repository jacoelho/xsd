package schemaops

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestResolveSimpleContentTextTypeResolvesExtensionAndRestriction(t *testing.T) {
	baseQName := types.QName{Namespace: "urn:test", Local: "Base"}
	base := types.NewComplexType(baseQName, "urn:test")
	base.SetContent(&types.SimpleContent{
		Extension: &types.Extension{Base: types.GetBuiltin(types.TypeNameString).Name()},
	})

	derived := types.NewComplexType(types.QName{Namespace: "urn:test", Local: "Derived"}, "urn:test")
	derivedRestriction := &types.Restriction{}
	derived.SetContent(&types.SimpleContent{
		Restriction: derivedRestriction,
		Base:        baseQName,
	})

	cache := make(map[*types.ComplexType]types.Type)
	got, err := ResolveSimpleContentTextType(derived, SimpleContentTextTypeOptions{
		ResolveQName: func(name types.QName) types.Type {
			if name == baseQName {
				return base
			}
			return types.GetBuiltinNS(name.Namespace, name.Local)
		},
		Cache: cache,
	})
	if err != nil {
		t.Fatalf("ResolveSimpleContentTextType error = %v", err)
	}

	restricted, ok := got.(*types.SimpleType)
	if !ok || restricted == nil {
		t.Fatalf("resolved type = %T, want *types.SimpleType", got)
	}
	if restricted.ResolvedBase != types.GetBuiltin(types.TypeNameString) {
		t.Fatalf("resolved base = %v, want xs:string", restricted.ResolvedBase)
	}
	if restricted.Restriction != derivedRestriction {
		t.Fatalf("restriction pointer changed during resolution")
	}
	if len(cache) != 2 {
		t.Fatalf("cache length = %d, want 2", len(cache))
	}
}

func TestResolveSimpleContentTextTypeReportsCycle(t *testing.T) {
	ct := types.NewComplexType(types.QName{Namespace: "urn:test", Local: "Self"}, "urn:test")
	ct.ResolvedBase = ct
	ct.SetContent(&types.SimpleContent{
		Extension: &types.Extension{Base: types.QName{Namespace: "urn:test", Local: "Self"}},
	})

	_, err := ResolveSimpleContentTextType(ct, SimpleContentTextTypeOptions{})
	if err == nil || !strings.Contains(err.Error(), "simpleContent cycle detected") {
		t.Fatalf("error = %v, want cycle detection", err)
	}
}

func TestResolveSimpleContentTextTypeReportsMissingBase(t *testing.T) {
	ct := types.NewComplexType(types.QName{Namespace: "urn:test", Local: "Missing"}, "urn:test")
	ct.SetContent(&types.SimpleContent{
		Extension: &types.Extension{Base: types.QName{Namespace: "urn:test", Local: "Unknown"}},
	})

	_, err := ResolveSimpleContentTextType(ct, SimpleContentTextTypeOptions{
		ResolveQName: func(_ types.QName) types.Type { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "simpleContent base missing") {
		t.Fatalf("error = %v, want missing base", err)
	}
}

func TestResolveSimpleContentTextTypeReturnsNilForNonSimpleContent(t *testing.T) {
	ct := types.NewComplexType(types.QName{Namespace: "urn:test", Local: "ElementContent"}, "urn:test")
	ct.SetContent(&types.ElementContent{
		Particle: &types.ElementDecl{
			Name:      types.QName{Namespace: "urn:test", Local: "item"},
			MinOccurs: types.OccursFromInt(1),
			MaxOccurs: types.OccursFromInt(1),
		},
	})

	got, err := ResolveSimpleContentTextType(ct, SimpleContentTextTypeOptions{})
	if err != nil {
		t.Fatalf("ResolveSimpleContentTextType error = %v", err)
	}
	if got != nil {
		t.Fatalf("resolved type = %T, want nil", got)
	}
}

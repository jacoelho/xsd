package grouprefs

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
)

func TestResolveSimpleContentTextTypeResolvesExtensionAndRestriction(t *testing.T) {
	baseQName := model.QName{Namespace: "urn:test", Local: "Base"}
	base := model.NewComplexType(baseQName, "urn:test")
	base.SetContent(&model.SimpleContent{
		Extension: &model.Extension{Base: builtins.Get(model.TypeNameString).Name()},
	})

	derived := model.NewComplexType(model.QName{Namespace: "urn:test", Local: "Derived"}, "urn:test")
	derivedRestriction := &model.Restriction{}
	derived.SetContent(&model.SimpleContent{
		Restriction: derivedRestriction,
		Base:        baseQName,
	})

	cache := make(map[*model.ComplexType]model.Type)
	got, err := ResolveSimpleContentTextType(derived, SimpleContentTextTypeOptions{
		ResolveQName: func(name model.QName) model.Type {
			if name == baseQName {
				return base
			}
			return builtins.GetNS(name.Namespace, name.Local)
		},
		Cache: cache,
	})
	if err != nil {
		t.Fatalf("ResolveSimpleContentTextType error = %v", err)
	}

	restricted, ok := got.(*model.SimpleType)
	if !ok || restricted == nil {
		t.Fatalf("resolved type = %T, want *model.SimpleType", got)
	}
	if restricted.ResolvedBase != builtins.Get(model.TypeNameString) {
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
	ct := model.NewComplexType(model.QName{Namespace: "urn:test", Local: "Self"}, "urn:test")
	ct.ResolvedBase = ct
	ct.SetContent(&model.SimpleContent{
		Extension: &model.Extension{Base: model.QName{Namespace: "urn:test", Local: "Self"}},
	})

	_, err := ResolveSimpleContentTextType(ct, SimpleContentTextTypeOptions{})
	if err == nil || !strings.Contains(err.Error(), "simpleContent cycle detected") {
		t.Fatalf("error = %v, want cycle detection", err)
	}
}

func TestResolveSimpleContentTextTypeReportsMissingBase(t *testing.T) {
	ct := model.NewComplexType(model.QName{Namespace: "urn:test", Local: "Missing"}, "urn:test")
	ct.SetContent(&model.SimpleContent{
		Extension: &model.Extension{Base: model.QName{Namespace: "urn:test", Local: "Unknown"}},
	})

	_, err := ResolveSimpleContentTextType(ct, SimpleContentTextTypeOptions{
		ResolveQName: func(_ model.QName) model.Type { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "simpleContent base missing") {
		t.Fatalf("error = %v, want missing base", err)
	}
}

func TestResolveSimpleContentTextTypeReturnsNilForNonSimpleContent(t *testing.T) {
	ct := model.NewComplexType(model.QName{Namespace: "urn:test", Local: "ElementContent"}, "urn:test")
	ct.SetContent(&model.ElementContent{
		Particle: &model.ElementDecl{
			Name:      model.QName{Namespace: "urn:test", Local: "item"},
			MinOccurs: occurs.OccursFromInt(1),
			MaxOccurs: occurs.OccursFromInt(1),
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

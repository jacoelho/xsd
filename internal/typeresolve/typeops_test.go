package typeresolve

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestCollectRestrictionFacetsPatternSyntaxError(t *testing.T) {
	t.Parallel()

	restriction := &model.Restriction{
		Base:   model.QName{Namespace: model.XSDNamespace, Local: "string"},
		Facets: []any{&model.Pattern{Value: "["}},
	}

	_, err := CollectRestrictionFacets(nil, restriction, builtins.Get(model.TypeNameString), nil)
	if err == nil {
		t.Fatalf("expected pattern syntax error")
	}
}

func TestCollectRestrictionFacetsDeferredFacetErrors(t *testing.T) {
	t.Parallel()

	baseType := builtins.Get(model.TypeNameInt)
	if baseType == nil {
		t.Fatalf("builtin int type not found")
	}

	tests := []struct {
		name      string
		facetName string
		value     string
	}{
		{name: "unknown facet", facetName: "unknownFacet", value: "1"},
		{name: "invalid lexical", facetName: "minInclusive", value: "abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			restriction := &model.Restriction{
				Base: model.QName{Namespace: model.XSDNamespace, Local: "int"},
				Facets: []any{
					&model.DeferredFacet{FacetName: tc.facetName, FacetValue: tc.value},
				},
			}
			_, err := CollectRestrictionFacets(nil, restriction, baseType, nil)
			if err == nil {
				t.Fatalf("expected deferred facet conversion error")
			}
		})
	}
}

func TestCollectSimpleTypeFacetsPropagatesRestrictionErrors(t *testing.T) {
	t.Parallel()

	st := &model.SimpleType{
		QName: model.QName{Local: "bad"},
		Restriction: &model.Restriction{
			Base:   model.QName{Namespace: model.XSDNamespace, Local: "string"},
			Facets: []any{&model.Pattern{Value: "["}},
		},
	}

	_, err := CollectSimpleTypeFacets(nil, st, nil)
	if err == nil {
		t.Fatalf("expected facet collection error")
	}
}

func TestResolveUnionMemberTypesHandlesCycles(t *testing.T) {
	t.Parallel()

	schema := parser.NewSchema()
	qname := model.QName{Namespace: "urn:test", Local: "A"}
	st := &model.SimpleType{
		QName:       qname,
		Restriction: &model.Restriction{Base: qname},
	}
	schema.TypeDefs[qname] = st

	members := ResolveUnionMemberTypes(schema, st)
	if len(members) != 0 {
		t.Fatalf("expected no members from cyclic restriction, got %d", len(members))
	}
}

func TestIsIDOnlyDerivedTypeHandlesCycles(t *testing.T) {
	t.Parallel()

	schema := parser.NewSchema()
	aQName := model.QName{Namespace: "urn:test", Local: "A"}
	bQName := model.QName{Namespace: "urn:test", Local: "B"}

	a := &model.SimpleType{
		QName:       aQName,
		Restriction: &model.Restriction{Base: bQName},
	}
	b := &model.SimpleType{
		QName:       bQName,
		Restriction: &model.Restriction{Base: aQName},
	}
	schema.TypeDefs[aQName] = a
	schema.TypeDefs[bQName] = b

	if IsIDOnlyDerivedType(schema, a) {
		t.Fatalf("expected cyclic non-ID derivation to be false")
	}

	idDerived := &model.SimpleType{
		Restriction: &model.Restriction{
			Base: model.QName{Namespace: model.XSDNamespace, Local: string(model.TypeNameID)},
		},
	}
	if !IsIDOnlyDerivedType(nil, idDerived) {
		t.Fatalf("expected direct xs:ID derivation to be true")
	}
}

func TestDefaultDeferredFacetConverter(t *testing.T) {
	t.Parallel()

	baseType := builtins.Get(model.TypeNameInt)
	if baseType == nil {
		t.Fatalf("builtin int type not found")
	}

	if facet, err := DefaultDeferredFacetConverter(nil, baseType); err != nil || facet != nil {
		t.Fatalf("nil deferred facet should be ignored, got facet=%v err=%v", facet, err)
	}

	facet, err := DefaultDeferredFacetConverter(&model.DeferredFacet{
		FacetName:  "minInclusive",
		FacetValue: "1",
	}, baseType)
	if err != nil {
		t.Fatalf("expected valid deferred facet conversion: %v", err)
	}
	if facet == nil {
		t.Fatalf("expected non-nil converted facet")
	}

	_, err = DefaultDeferredFacetConverter(&model.DeferredFacet{
		FacetName:  "unknownFacet",
		FacetValue: "1",
	}, baseType)
	if err == nil || !strings.Contains(err.Error(), "unknown deferred facet type") {
		t.Fatalf("expected unknown deferred facet error, got %v", err)
	}
}

func TestResolveUnionMemberTypesResolvesInlineAndNamedMembers(t *testing.T) {
	t.Parallel()

	schema := parser.NewSchema()
	namedMemberQName := model.QName{Namespace: "urn:test", Local: "NamedMember"}
	namedMember := &model.SimpleType{
		QName: namedMemberQName,
		Restriction: &model.Restriction{
			Base: model.QName{Namespace: model.XSDNamespace, Local: "int"},
		},
	}
	schema.TypeDefs[namedMemberQName] = namedMember

	inlineMember := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "InlineMember"},
		Restriction: &model.Restriction{
			Base: model.QName{Namespace: model.XSDNamespace, Local: "string"},
		},
	}
	union := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "U"},
		Union: &model.UnionType{
			MemberTypes: []model.QName{namedMemberQName},
			InlineTypes: []*model.SimpleType{inlineMember},
		},
	}

	members := ResolveUnionMemberTypes(schema, union)
	if len(members) != 2 {
		t.Fatalf("ResolveUnionMemberTypes() len = %d, want 2", len(members))
	}
	if members[0] != inlineMember {
		t.Fatalf("ResolveUnionMemberTypes()[0] = %v, want inline member", members[0].Name())
	}
	if members[1] != namedMember {
		t.Fatalf("ResolveUnionMemberTypes()[1] = %v, want named member", members[1].Name())
	}
}

func TestResolveListItemTypeResolvesNamedRestrictionBase(t *testing.T) {
	t.Parallel()

	schema := parser.NewSchema()
	baseQName := model.QName{Namespace: "urn:test", Local: "BaseList"}
	baseList := &model.SimpleType{
		QName: baseQName,
		List: &model.ListType{
			ItemType: model.QName{Namespace: model.XSDNamespace, Local: "int"},
		},
	}
	schema.TypeDefs[baseQName] = baseList

	derived := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "DerivedList"},
		Restriction: &model.Restriction{
			Base: baseQName,
		},
	}

	itemType := ResolveListItemType(schema, derived)
	if itemType == nil {
		t.Fatalf("ResolveListItemType() returned nil")
	}
	if itemType.Name().Local != "int" {
		t.Fatalf("ResolveListItemType() local name = %q, want %q", itemType.Name().Local, "int")
	}
}

func TestHelpersNilInputs(t *testing.T) {
	t.Parallel()

	if got, err := ResolveTypeQName(nil, model.QName{}, TypeReferenceMustExist); err != nil || got != nil {
		t.Fatalf("ResolveTypeQName nil input = (%v, %v), want (nil, nil)", got, err)
	}
	if got := ResolveSimpleTypeReferenceAllowMissing(nil, model.QName{}); got != nil {
		t.Fatalf("ResolveSimpleTypeReferenceAllowMissing nil input = %v, want nil", got)
	}
	if got := ResolveSimpleContentBaseTypeFromContent(nil, nil); got != nil {
		t.Fatalf("ResolveSimpleContentBaseTypeFromContent nil input = %v, want nil", got)
	}
	if got := ResolveListItemType(nil, nil); got != nil {
		t.Fatalf("ResolveListItemType nil input = %v, want nil", got)
	}
	facets, err := CollectSimpleTypeFacets(nil, nil, nil)
	if err != nil {
		t.Fatalf("CollectSimpleTypeFacets nil input error = %v", err)
	}
	if facets != nil {
		t.Fatalf("CollectSimpleTypeFacets nil input = %v, want nil", facets)
	}
}

func TestResolveSimpleTypeReferenceMissingTypeReturnsError(t *testing.T) {
	t.Parallel()

	_, err := ResolveTypeQName(nil, model.QName{Namespace: "urn:test", Local: "Missing"}, TypeReferenceMustExist)
	if err == nil || !strings.Contains(err.Error(), "type {urn:test}Missing not found") {
		t.Fatalf("expected missing type error, got %v", err)
	}
}

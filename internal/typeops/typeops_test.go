package typeops

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestCollectRestrictionFacetsPatternSyntaxError(t *testing.T) {
	t.Parallel()

	restriction := &types.Restriction{
		Base:   types.QName{Namespace: types.XSDNamespace, Local: "string"},
		Facets: []any{&types.Pattern{Value: "["}},
	}

	_, err := CollectRestrictionFacets(nil, restriction, types.GetBuiltin(types.TypeNameString), nil)
	if err == nil {
		t.Fatalf("expected pattern syntax error")
	}
}

func TestCollectRestrictionFacetsDeferredFacetErrors(t *testing.T) {
	t.Parallel()

	baseType := types.GetBuiltin(types.TypeNameInt)
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
			restriction := &types.Restriction{
				Base: types.QName{Namespace: types.XSDNamespace, Local: "int"},
				Facets: []any{
					&types.DeferredFacet{FacetName: tc.facetName, FacetValue: tc.value},
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

	st := &types.SimpleType{
		QName: types.QName{Local: "bad"},
		Restriction: &types.Restriction{
			Base:   types.QName{Namespace: types.XSDNamespace, Local: "string"},
			Facets: []any{&types.Pattern{Value: "["}},
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
	qname := types.QName{Namespace: "urn:test", Local: "A"}
	st := &types.SimpleType{
		QName:       qname,
		Restriction: &types.Restriction{Base: qname},
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
	aQName := types.QName{Namespace: "urn:test", Local: "A"}
	bQName := types.QName{Namespace: "urn:test", Local: "B"}

	a := &types.SimpleType{
		QName:       aQName,
		Restriction: &types.Restriction{Base: bQName},
	}
	b := &types.SimpleType{
		QName:       bQName,
		Restriction: &types.Restriction{Base: aQName},
	}
	schema.TypeDefs[aQName] = a
	schema.TypeDefs[bQName] = b

	if IsIDOnlyDerivedType(schema, a) {
		t.Fatalf("expected cyclic non-ID derivation to be false")
	}

	idDerived := &types.SimpleType{
		Restriction: &types.Restriction{
			Base: types.QName{Namespace: types.XSDNamespace, Local: string(types.TypeNameID)},
		},
	}
	if !IsIDOnlyDerivedType(nil, idDerived) {
		t.Fatalf("expected direct xs:ID derivation to be true")
	}
}

func TestDefaultDeferredFacetConverter(t *testing.T) {
	t.Parallel()

	baseType := types.GetBuiltin(types.TypeNameInt)
	if baseType == nil {
		t.Fatalf("builtin int type not found")
	}

	if facet, err := DefaultDeferredFacetConverter(nil, baseType); err != nil || facet != nil {
		t.Fatalf("nil deferred facet should be ignored, got facet=%v err=%v", facet, err)
	}

	facet, err := DefaultDeferredFacetConverter(&types.DeferredFacet{
		FacetName:  "minInclusive",
		FacetValue: "1",
	}, baseType)
	if err != nil {
		t.Fatalf("expected valid deferred facet conversion: %v", err)
	}
	if facet == nil {
		t.Fatalf("expected non-nil converted facet")
	}

	_, err = DefaultDeferredFacetConverter(&types.DeferredFacet{
		FacetName:  "unknownFacet",
		FacetValue: "1",
	}, baseType)
	if err == nil || !strings.Contains(err.Error(), "unknown deferred facet type") {
		t.Fatalf("expected unknown deferred facet error, got %v", err)
	}
}

func TestHelpersNilInputs(t *testing.T) {
	t.Parallel()

	if got, err := ResolveTypeQName(nil, types.QName{}, TypeReferenceMustExist); err != nil || got != nil {
		t.Fatalf("ResolveTypeQName nil input = (%v, %v), want (nil, nil)", got, err)
	}
	if got := ResolveSimpleTypeReferenceAllowMissing(nil, types.QName{}); got != nil {
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

	_, err := ResolveTypeQName(nil, types.QName{Namespace: "urn:test", Local: "Missing"}, TypeReferenceMustExist)
	if err == nil || !strings.Contains(err.Error(), "type {urn:test}Missing not found") {
		t.Fatalf("expected missing type error, got %v", err)
	}
}

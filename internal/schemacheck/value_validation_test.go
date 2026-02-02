package schemacheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestSchemacheckListAcceptsEmptyValue(t *testing.T) {
	list := &types.ListType{ItemType: types.QName{Namespace: types.XSDNamespace, Local: "token"}}
	st, err := types.NewListSimpleType(types.QName{Namespace: "urn:test", Local: "List"}, "urn:test", list, nil)
	if err != nil {
		t.Fatalf("NewListSimpleType: %v", err)
	}
	if err := validateValueAgainstTypeWithFacets(nil, "", st, nil, make(map[types.Type]bool)); err != nil {
		t.Fatalf("unexpected empty list error: %v", err)
	}
}

func TestSchemacheckListMinLengthRejectsEmpty(t *testing.T) {
	list := &types.ListType{ItemType: types.QName{Namespace: types.XSDNamespace, Local: "token"}}
	st, err := types.NewListSimpleType(types.QName{Namespace: "urn:test", Local: "ListMin"}, "urn:test", list, nil)
	if err != nil {
		t.Fatalf("NewListSimpleType: %v", err)
	}
	st.Restriction = &types.Restriction{Facets: []any{&types.MinLength{Value: 1}}}
	if err := validateValueAgainstTypeWithFacets(nil, "", st, nil, make(map[types.Type]bool)); err == nil {
		t.Fatalf("expected empty list to fail minLength")
	}
}

package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestSchemacheckListAcceptsEmptyValue(t *testing.T) {
	list := &model.ListType{ItemType: model.QName{Namespace: model.XSDNamespace, Local: "token"}}
	st, err := model.NewListSimpleType(model.QName{Namespace: "urn:test", Local: "List"}, "urn:test", list, nil)
	if err != nil {
		t.Fatalf("NewListSimpleType: %v", err)
	}
	if err := validateValueAgainstTypeWithFacets(nil, "", st, nil); err != nil {
		t.Fatalf("unexpected empty list error: %v", err)
	}
}

func TestSchemacheckListMinLengthRejectsEmpty(t *testing.T) {
	list := &model.ListType{ItemType: model.QName{Namespace: model.XSDNamespace, Local: "token"}}
	st, err := model.NewListSimpleType(model.QName{Namespace: "urn:test", Local: "ListMin"}, "urn:test", list, nil)
	if err != nil {
		t.Fatalf("NewListSimpleType: %v", err)
	}
	st.Restriction = &model.Restriction{Facets: []any{&model.MinLength{Value: 1}}}
	if err := validateValueAgainstTypeWithFacets(nil, "", st, nil); err == nil {
		t.Fatalf("expected empty list to fail minLength")
	}
}

package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestListWhiteSpaceFacetOverrideRejected(t *testing.T) {
	cases := []struct {
		name       string
		whiteSpace model.WhiteSpace
	}{
		{"preserve", model.WhiteSpacePreserve},
		{"replace", model.WhiteSpaceReplace},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			schema := parser.NewSchema()
			list := &model.ListType{ItemType: model.QName{Namespace: model.XSDNamespace, Local: "string"}}
			st, err := model.NewListSimpleType(model.QName{Namespace: "urn:test", Local: "list"}, "urn:test", list, nil)
			if err != nil {
				t.Fatalf("NewListSimpleType error = %v", err)
			}
			st.SetWhiteSpaceExplicit(tt.whiteSpace)
			schema.TypeDefs[st.QName] = st

			if errs := ValidateStructure(schema); len(errs) == 0 {
				t.Fatalf("expected list whiteSpace error for %s", tt.name)
			}
		})
	}
}

func TestListWhiteSpaceCollapseAllowed(t *testing.T) {
	schema := parser.NewSchema()
	list := &model.ListType{ItemType: model.QName{Namespace: model.XSDNamespace, Local: "string"}}
	st, err := model.NewListSimpleType(model.QName{Namespace: "urn:test", Local: "list"}, "urn:test", list, nil)
	if err != nil {
		t.Fatalf("NewListSimpleType error = %v", err)
	}
	st.SetWhiteSpaceExplicit(model.WhiteSpaceCollapse)
	schema.TypeDefs[st.QName] = st

	if errs := ValidateStructure(schema); len(errs) != 0 {
		t.Fatalf("expected schema to be valid, got %v", errs)
	}
}

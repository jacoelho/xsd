package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestListWhiteSpaceFacetOverrideRejected(t *testing.T) {
	cases := []struct {
		name       string
		whiteSpace types.WhiteSpace
	}{
		{"preserve", types.WhiteSpacePreserve},
		{"replace", types.WhiteSpaceReplace},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			schema := parser.NewSchema()
			list := &types.ListType{ItemType: types.QName{Namespace: types.XSDNamespace, Local: "string"}}
			st, err := types.NewListSimpleType(types.QName{Namespace: "urn:test", Local: "list"}, "urn:test", list, nil)
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
	list := &types.ListType{ItemType: types.QName{Namespace: types.XSDNamespace, Local: "string"}}
	st, err := types.NewListSimpleType(types.QName{Namespace: "urn:test", Local: "list"}, "urn:test", list, nil)
	if err != nil {
		t.Fatalf("NewListSimpleType error = %v", err)
	}
	st.SetWhiteSpaceExplicit(types.WhiteSpaceCollapse)
	schema.TypeDefs[st.QName] = st

	if errs := ValidateStructure(schema); len(errs) != 0 {
		t.Fatalf("expected schema to be valid, got %v", errs)
	}
}

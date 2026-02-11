package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestMergeAttributesFromGroupsForValidationTraversesNestedCycle(t *testing.T) {
	schema := parser.NewSchema()
	g1 := model.QName{Namespace: "urn:test", Local: "G1"}
	g2 := model.QName{Namespace: "urn:test", Local: "G2"}
	g3 := model.QName{Namespace: "urn:test", Local: "G3"}
	schema.AttributeGroups[g1] = &model.AttributeGroup{
		Name:       g1,
		AttrGroups: []model.QName{g2},
		Attributes: []*model.AttributeDecl{{Name: model.QName{Local: "a"}, Use: model.Optional}},
	}
	schema.AttributeGroups[g2] = &model.AttributeGroup{
		Name:       g2,
		AttrGroups: []model.QName{g3},
		Attributes: []*model.AttributeDecl{{Name: model.QName{Local: "b"}, Use: model.Required}},
	}
	schema.AttributeGroups[g3] = &model.AttributeGroup{
		Name:       g3,
		AttrGroups: []model.QName{g1},
		Attributes: []*model.AttributeDecl{{Name: model.QName{Local: "c"}, Use: model.Optional}},
	}

	attrMap := make(map[model.QName]*model.AttributeDecl)
	mergeAttributesFromGroupsForValidation(schema, []model.QName{g1}, attrMap)

	for _, local := range []string{"a", "b", "c"} {
		key := model.QName{Local: local}
		if _, ok := attrMap[key]; !ok {
			t.Fatalf("attribute %s not collected", local)
		}
	}
}

func TestCollectAnyAttributeFromGroupsSharedNestedDedup(t *testing.T) {
	schema := parser.NewSchema()
	g1 := model.QName{Namespace: "urn:test", Local: "G1"}
	g2 := model.QName{Namespace: "urn:test", Local: "G2"}
	shared := model.QName{Namespace: "urn:test", Local: "Shared"}

	schema.AttributeGroups[g1] = &model.AttributeGroup{Name: g1, AttrGroups: []model.QName{shared}}
	schema.AttributeGroups[g2] = &model.AttributeGroup{Name: g2, AttrGroups: []model.QName{shared}}
	schema.AttributeGroups[shared] = &model.AttributeGroup{
		Name: shared,
		AnyAttribute: &model.AnyAttribute{
			Namespace:       model.NSCAny,
			ProcessContents: model.Lax,
		},
	}

	wildcards := collectAnyAttributeFromGroups(schema, []model.QName{g1, g2})
	if len(wildcards) != 1 {
		t.Fatalf("wildcards = %d, want 1", len(wildcards))
	}
}

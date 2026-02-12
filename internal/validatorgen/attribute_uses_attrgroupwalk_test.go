package validatorgen

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
)

func TestCollectAttributeUsesIgnoresProhibitedFromAttributeGroups(t *testing.T) {
	schema := parser.NewSchema()
	typeQName := model.QName{Namespace: "urn:test", Local: "T"}
	groupQName := model.QName{Namespace: "urn:test", Local: "AG"}
	attrLocal := model.QName{Local: "a"}

	ct := model.NewComplexType(typeQName, "urn:test")
	ct.SetContent(&model.EmptyContent{})
	ct.SetAttributes([]*model.AttributeDecl{
		{Name: attrLocal, Use: model.Optional},
	})
	ct.AttrGroups = []model.QName{groupQName}

	schema.TypeDefs[typeQName] = ct
	schema.AttributeGroups[groupQName] = &model.AttributeGroup{
		Name: groupQName,
		Attributes: []*model.AttributeDecl{
			{Name: attrLocal, Use: model.Prohibited},
			{Name: model.QName{Local: "b"}, Use: model.Optional},
		},
	}

	attrs, _, err := CollectAttributeUses(schema, ct)
	if err != nil {
		t.Fatalf("CollectAttributeUses() error = %v", err)
	}

	byLocal := make(map[string]*model.AttributeDecl, len(attrs))
	for _, attr := range attrs {
		byLocal[attr.Name.Local] = attr
	}
	if got := byLocal["a"]; got == nil {
		t.Fatalf("attribute a missing")
	} else if got.Use != model.Optional {
		t.Fatalf("attribute a use = %v, want %v", got.Use, model.Optional)
	}
	if byLocal["b"] == nil {
		t.Fatalf("attribute b missing")
	}
}

func TestCollectAttributeUsesMissingNestedAttributeGroup(t *testing.T) {
	schema := parser.NewSchema()
	typeQName := model.QName{Namespace: "urn:test", Local: "T"}
	rootGroup := model.QName{Namespace: "urn:test", Local: "AG"}
	missingGroup := model.QName{Namespace: "urn:test", Local: "Missing"}

	ct := model.NewComplexType(typeQName, "urn:test")
	ct.SetContent(&model.EmptyContent{})
	ct.AttrGroups = []model.QName{rootGroup}
	schema.TypeDefs[typeQName] = ct
	schema.AttributeGroups[rootGroup] = &model.AttributeGroup{
		Name:       rootGroup,
		AttrGroups: []model.QName{missingGroup},
	}

	_, _, err := CollectAttributeUses(schema, ct)
	if err == nil {
		t.Fatalf("expected missing attributeGroup error")
	}
	var missingErr attrgroupwalk.AttrGroupMissingError
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected AttrGroupMissingError, got %T (%v)", err, err)
	}
	if missingErr.QName != missingGroup {
		t.Fatalf("missing QName = %s, want %s", missingErr.QName, missingGroup)
	}
}

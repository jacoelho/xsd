package semantics_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

func TestWalkAttributeGroupsDeterministicDedup(t *testing.T) {
	schema := parser.NewSchema()
	a := model.QName{Local: "a"}
	b := model.QName{Local: "b"}
	schema.AttributeGroups[a] = &model.AttributeGroup{}
	schema.AttributeGroups[b] = &model.AttributeGroup{}

	var seen []model.QName
	if err := semantics.WalkAttributeGroups(schema, []model.QName{a, b}, semantics.MissingIgnore, func(q model.QName, _ *model.AttributeGroup) error {
		seen = append(seen, q)
		return nil
	}); err != nil {
		t.Fatalf("WalkAttributeGroups() error = %v", err)
	}
	if len(seen) != 2 || seen[0] != a || seen[1] != b {
		t.Fatalf("seen = %v, want [%s %s]", seen, a, b)
	}
}

func TestWalkAttributeGroupsMissingPolicy(t *testing.T) {
	schema := parser.NewSchema()
	ref := model.QName{Local: "missing"}

	if err := semantics.WalkAttributeGroups(schema, []model.QName{ref}, semantics.MissingIgnore, nil); err != nil {
		t.Fatalf("WalkAttributeGroups missing ignore error = %v", err)
	}

	err := semantics.WalkAttributeGroups(schema, []model.QName{ref}, semantics.MissingError, nil)
	if err == nil {
		t.Fatal("expected missing error")
	}
	var missing semantics.AttributeGroupMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("expected AttributeGroupMissingError, got %T", err)
	}
	if missing.QName != ref {
		t.Fatalf("missing ref = %s, want %s", missing.QName, ref)
	}
}

func TestAttributeGroupContextDetectsCycles(t *testing.T) {
	schema := parser.NewSchema()
	a := model.QName{Local: "a"}
	b := model.QName{Local: "b"}
	schema.AttributeGroups[a] = &model.AttributeGroup{AttrGroups: []model.QName{b}}
	schema.AttributeGroups[b] = &model.AttributeGroup{AttrGroups: []model.QName{a}}

	ctx := semantics.NewAttributeGroupContext(schema, semantics.AttributeGroupWalkOptions{
		Missing: semantics.MissingError,
		Cycles:  semantics.CyclePolicyError,
	})
	err := ctx.Walk([]model.QName{a}, nil)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	var cycle semantics.AttributeGroupCycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("expected AttributeGroupCycleError, got %T", err)
	}
}

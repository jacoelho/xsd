package analysis

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestWalkAttributeGroupsDeterministicDedup(t *testing.T) {
	schema := parser.NewSchema()
	a := model.QName{Local: "a"}
	b := model.QName{Local: "b"}
	schema.AttributeGroups[a] = &model.AttributeGroup{}
	schema.AttributeGroups[b] = &model.AttributeGroup{}

	var seen []model.QName
	if err := WalkAttributeGroups(schema, []model.QName{a, b}, MissingIgnore, func(q model.QName, _ *model.AttributeGroup) error {
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

	if err := WalkAttributeGroups(schema, []model.QName{ref}, MissingIgnore, nil); err != nil {
		t.Fatalf("WalkAttributeGroups missing ignore error = %v", err)
	}

	err := WalkAttributeGroups(schema, []model.QName{ref}, MissingError, nil)
	if err == nil {
		t.Fatal("expected missing error")
	}
	var missing AttributeGroupMissingError
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

	ctx := NewAttributeGroupContext(schema, AttributeGroupWalkOptions{
		Missing: MissingError,
		Cycles:  CycleError,
	})
	err := ctx.Walk([]model.QName{a}, nil)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	var cycle AttributeGroupCycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("expected AttributeGroupCycleError, got %T", err)
	}
}

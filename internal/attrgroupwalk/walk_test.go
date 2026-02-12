package attrgroupwalk

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
)

func TestWalkDepthFirstAndCycleDedup(t *testing.T) {
	schema := parser.NewSchema()
	a := model.QName{Namespace: "urn:test", Local: "A"}
	b := model.QName{Namespace: "urn:test", Local: "B"}
	c := model.QName{Namespace: "urn:test", Local: "C"}
	schema.AttributeGroups[a] = &model.AttributeGroup{Name: a, AttrGroups: []model.QName{b}}
	schema.AttributeGroups[b] = &model.AttributeGroup{Name: b, AttrGroups: []model.QName{c}}
	schema.AttributeGroups[c] = &model.AttributeGroup{Name: c, AttrGroups: []model.QName{a}}

	var got []string
	if err := Walk(schema, []model.QName{a, b}, MissingIgnore, func(q model.QName, _ *model.AttributeGroup) error {
		got = append(got, q.Local)
		return nil
	}); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	if strings.Join(got, ",") != "A,B,C" {
		t.Fatalf("visited order = %v, want [A B C]", got)
	}
}

func TestWalkMissingPolicy(t *testing.T) {
	schema := parser.NewSchema()
	ref := model.QName{Namespace: "urn:test", Local: "Missing"}

	if err := Walk(schema, []model.QName{ref}, MissingIgnore, nil); err != nil {
		t.Fatalf("Walk() ignore missing error = %v", err)
	}

	err := Walk(schema, []model.QName{ref}, MissingError, nil)
	if err == nil {
		t.Fatalf("expected missing attributeGroup error")
	}
	var missing AttrGroupMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("expected AttrGroupMissingError, got %T", err)
	}
	if missing.QName != ref {
		t.Fatalf("missing QName = %s, want %s", missing.QName, ref)
	}
}

func TestWalkCyclePolicyError(t *testing.T) {
	schema := parser.NewSchema()
	a := model.QName{Namespace: "urn:test", Local: "A"}
	b := model.QName{Namespace: "urn:test", Local: "B"}
	schema.AttributeGroups[a] = &model.AttributeGroup{Name: a, AttrGroups: []model.QName{b}}
	schema.AttributeGroups[b] = &model.AttributeGroup{Name: b, AttrGroups: []model.QName{a}}

	err := WalkWithOptions(schema, []model.QName{a}, Options{
		Missing: MissingError,
		Cycles:  CycleError,
	}, nil)
	if err == nil {
		t.Fatalf("expected cycle error")
	}
	var cycle AttrGroupCycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("expected AttrGroupCycleError, got %T", err)
	}
	if cycle.QName != a {
		t.Fatalf("cycle QName = %s, want %s", cycle.QName, a)
	}
}

func TestWalkSharedSubgraphDedupAcrossRoots(t *testing.T) {
	schema := parser.NewSchema()
	a := model.QName{Namespace: "urn:test", Local: "A"}
	b := model.QName{Namespace: "urn:test", Local: "B"}
	c := model.QName{Namespace: "urn:test", Local: "C"}
	schema.AttributeGroups[a] = &model.AttributeGroup{Name: a, AttrGroups: []model.QName{c}}
	schema.AttributeGroups[b] = &model.AttributeGroup{Name: b, AttrGroups: []model.QName{c}}
	schema.AttributeGroups[c] = &model.AttributeGroup{Name: c}

	var got []string
	if err := Walk(schema, []model.QName{a, b}, MissingError, func(q model.QName, _ *model.AttributeGroup) error {
		got = append(got, q.Local)
		return nil
	}); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	if strings.Join(got, ",") != "A,C,B" {
		t.Fatalf("visited order = %v, want [A C B]", got)
	}
}

func TestContextWalkReusesMemoizedClosureAcrossPasses(t *testing.T) {
	schema := parser.NewSchema()
	a := model.QName{Namespace: "urn:test", Local: "A"}
	c := model.QName{Namespace: "urn:test", Local: "C"}
	schema.AttributeGroups[a] = &model.AttributeGroup{Name: a, AttrGroups: []model.QName{c}}
	schema.AttributeGroups[c] = &model.AttributeGroup{Name: c}

	ctx := NewContext(schema, Options{Missing: MissingError, Cycles: CycleError})
	var first []string
	if err := ctx.Walk([]model.QName{a}, func(q model.QName, _ *model.AttributeGroup) error {
		first = append(first, q.Local)
		return nil
	}); err != nil {
		t.Fatalf("first walk error = %v", err)
	}
	var second []string
	if err := ctx.Walk([]model.QName{a}, func(q model.QName, _ *model.AttributeGroup) error {
		second = append(second, q.Local)
		return nil
	}); err != nil {
		t.Fatalf("second walk error = %v", err)
	}

	if strings.Join(first, ",") != "A,C" {
		t.Fatalf("first walk order = %v, want [A C]", first)
	}
	if strings.Join(second, ",") != "A,C" {
		t.Fatalf("second walk order = %v, want [A C]", second)
	}
}

package compiler

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestPlanIncludeNamespaceMismatch(t *testing.T) {
	t.Parallel()

	target := parser.NewSchema()
	source := parser.NewSchema()
	source.TargetNamespace = "urn:other"

	_, err := PlanInclude("urn:test", []int{0}, target, parser.IncludeInfo{IncludeIndex: 0}, "inc.xsd", source)
	if err == nil || !strings.Contains(err.Error(), "different target namespace") {
		t.Fatalf("PlanInclude() error = %v, want namespace mismatch", err)
	}
}

func TestPlanIncludeAllowsChameleonRemap(t *testing.T) {
	t.Parallel()

	target := parser.NewSchema()
	target.GlobalDecls = make([]parser.GlobalDecl, 2)
	source := parser.NewSchema()

	plan, err := PlanInclude("urn:test", []int{1, 0}, target, parser.IncludeInfo{
		DeclIndex:    1,
		IncludeIndex: 1,
	}, "inc.xsd", source)
	if err != nil {
		t.Fatalf("PlanInclude() error = %v", err)
	}
	if plan.kind != Include || plan.remap != RemapNamespace || plan.insert != 2 {
		t.Fatalf("plan = %+v, want include/remap/insert=2", plan)
	}
}

func TestPlanImportMismatch(t *testing.T) {
	t.Parallel()

	source := parser.NewSchema()
	source.TargetNamespace = "urn:other"

	_, err := PlanImport("imp.xsd", "urn:test", source, 0)
	if err == nil || !strings.Contains(err.Error(), "namespace mismatch") {
		t.Fatalf("PlanImport() error = %v, want namespace mismatch", err)
	}
}

package normalize

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/loadmerge"
	expparser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/prep"
	"github.com/jacoelho/xsd/internal/runtimeassemble"
)

// Prepare clones, resolves, and validates a parsed schema.
func Prepare(sch *expparser.Schema) (*Artifacts, error) {
	if sch == nil {
		return nil, fmt.Errorf("prepare schema: schema is nil")
	}
	cloned, err := loadmerge.CloneSchemaDeep(sch)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: clone schema: %w", err)
	}
	return PrepareOwned(cloned)
}

// PrepareOwned resolves and validates a parsed schema in place.
func PrepareOwned(sch *expparser.Schema) (*Artifacts, error) {
	if sch == nil {
		return nil, fmt.Errorf("prepare schema: schema is nil")
	}
	if err := prep.ResolveAndValidateOwned(sch); err != nil {
		return nil, fmt.Errorf("prepare schema: %w", err)
	}
	reg, err := analysis.AssignIDs(sch)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: assign IDs: %w", err)
	}
	cycleErr := analysis.DetectCycles(sch)
	if cycleErr != nil {
		return nil, fmt.Errorf("prepare schema: detect cycles: %w", cycleErr)
	}
	upaErr := prep.ValidateUPA(sch, reg)
	if upaErr != nil {
		return nil, fmt.Errorf("prepare schema: validate UPA: %w", upaErr)
	}
	refs, err := analysis.ResolveReferences(sch, reg)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: resolve references: %w", err)
	}
	complexTypes, err := runtimeassemble.BuildComplexTypePlan(sch, reg)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: complex type plan: %w", err)
	}
	return &Artifacts{
		schema:       sch,
		registry:     reg,
		refs:         refs,
		complexTypes: complexTypes,
	}, nil
}

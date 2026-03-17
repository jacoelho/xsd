package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/validatorgen"
)

// Prepare clones, normalizes, and validates a parsed schema.
func Prepare(parsed *parser.Schema) (*Prepared, error) {
	if parsed == nil {
		return nil, fmt.Errorf("prepare schema: schema is nil")
	}
	return PrepareOwned(parser.CloneSchema(parsed))
}

// PrepareOwned normalizes and validates a parsed schema in place.
func PrepareOwned(parsed *parser.Schema) (*Prepared, error) {
	if parsed == nil {
		return nil, fmt.Errorf("prepare schema: schema is nil")
	}
	if err := resolveAndValidateOwned(parsed); err != nil {
		return nil, fmt.Errorf("prepare schema: %w", err)
	}
	registry, err := analysis.AssignIDs(parsed)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: assign IDs: %w", err)
	}
	err = analysis.DetectCycles(parsed)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: detect cycles: %w", err)
	}
	err = validateUPA(parsed, registry)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: validate UPA: %w", err)
	}
	refs, err := analysis.ResolveReferences(parsed, registry)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: resolve references: %w", err)
	}
	complexTypes, err := validatorgen.BuildComplexTypePlan(parsed, registry)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: complex type plan: %w", err)
	}
	return &Prepared{
		schema:       parsed,
		registry:     registry,
		refs:         refs,
		complexTypes: complexTypes,
	}, nil
}

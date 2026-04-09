package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/parser"
)

type prepareResult struct {
	schema       *parser.Schema
	registry     *analysis.Registry
	refs         *analysis.ResolvedReferences
	complexTypes *complexplan.ComplexTypes
}

// Prepare clones, normalizes, and validates a parsed schema.
func Prepare(parsed *parser.Schema) (*Prepared, error) {
	result, err := prepareParsedSchema(parsed, true)
	if err != nil {
		return nil, err
	}
	return preparedFromResult(result), nil
}

// PrepareOwned normalizes and validates a parsed schema in place.
func PrepareOwned(parsed *parser.Schema) (*Prepared, error) {
	result, err := prepareParsedSchema(parsed, false)
	if err != nil {
		return nil, err
	}
	return preparedFromResult(result), nil
}

func prepareParsedSchema(parsed *parser.Schema, clone bool) (*prepareResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("prepare schema: schema is nil")
	}
	if clone {
		parsed = parser.CloneSchema(parsed)
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
	refs, err := analysis.ResolveReferences(parsed, registry)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: resolve references: %w", err)
	}
	complexTypes, err := buildPreparedComplexTypes(parsed, registry, refs)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: %w", err)
	}
	return &prepareResult{
		schema:       parsed,
		registry:     registry,
		refs:         refs,
		complexTypes: complexTypes,
	}, nil
}

func preparedFromResult(result *prepareResult) *Prepared {
	if result == nil {
		return nil
	}
	return &Prepared{
		schema:       result.schema,
		registry:     result.registry,
		refs:         result.refs,
		complexTypes: result.complexTypes,
	}
}

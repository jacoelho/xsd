package compiler

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

type prepareResult struct {
	schema    *parser.Schema
	registry  *analysis.Registry
	refs      *analysis.ResolvedReferences
	semantics *semantics.Context
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

// ResolveAndValidateOwned resolves semantic references and validates the schema in place.
func ResolveAndValidateOwned(sch *parser.Schema) error {
	validationErrs, err := semantics.ResolveAndValidateSchema(sch)
	if err != nil {
		return err
	}
	if len(validationErrs) > 0 {
		return formatValidationErrors(validationErrs)
	}
	return nil
}

func prepareParsedSchema(parsed *parser.Schema, clone bool) (*prepareResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("prepare schema: schema is nil")
	}
	if clone {
		parsed = parser.CloneSchema(parsed)
	}
	if err := ResolveAndValidateOwned(parsed); err != nil {
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
	semanticCtx, err := semantics.Build(parsed, registry, refs)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: semantics: %w", err)
	}
	err = semanticCtx.Particles().ValidateUPA()
	if err != nil {
		return nil, fmt.Errorf("prepare schema: validate UPA: %w", err)
	}
	return &prepareResult{
		schema:    parsed,
		registry:  registry,
		refs:      refs,
		semantics: semanticCtx,
	}, nil
}

func preparedFromResult(result *prepareResult) *Prepared {
	if result == nil {
		return nil
	}
	return &Prepared{
		schema:    result.schema,
		registry:  result.registry,
		refs:      result.refs,
		semantics: result.semantics,
	}
}

func complexTypeLabel(ct *model.ComplexType) string {
	if ct == nil {
		return "complexType"
	}
	if ct.QName.IsZero() {
		return "anonymous complexType"
	}
	return fmt.Sprintf("complexType %s", ct.QName)
}

func formatValidationErrors(validationErrs []error) error {
	if len(validationErrs) == 0 {
		return nil
	}

	errs := validationErrs
	if len(validationErrs) > 1 {
		errs = slices.Clone(validationErrs)
		slices.SortStableFunc(errs, func(a, b error) int {
			return strings.Compare(a.Error(), b.Error())
		})
	}

	var msg strings.Builder
	msg.WriteString("schema validation failed:")
	for _, err := range errs {
		msg.WriteString("\n  - ")
		msg.WriteString(err.Error())
	}
	return errors.New(msg.String())
}

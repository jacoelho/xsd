package compiler

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

func resolveAndValidateOwned(sch *parser.Schema) error {
	validationErrs, err := semantics.ResolveAndValidateSchema(sch)
	if err != nil {
		return err
	}
	if len(validationErrs) > 0 {
		return formatValidationErrors(validationErrs)
	}
	return nil
}

func buildPreparedComplexTypes(
	schema *parser.Schema,
	registry *analysis.Registry,
	refs *analysis.ResolvedReferences,
) (*complexplan.ComplexTypes, error) {
	semanticCtx, err := semantics.Build(schema, registry, refs)
	if err != nil {
		return nil, fmt.Errorf("semantics: %w", err)
	}
	if err := semanticCtx.Particles().ValidateUPA(); err != nil {
		return nil, fmt.Errorf("validate UPA: %w", err)
	}
	complexTypes, err := semanticCtx.ComplexTypes()
	if err != nil {
		return nil, fmt.Errorf("complex types: %w", err)
	}
	return complexTypes, nil
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

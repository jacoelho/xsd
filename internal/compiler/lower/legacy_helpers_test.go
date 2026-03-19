package lower

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semanticcheck"
	"github.com/jacoelho/xsd/internal/semanticresolve"
)

func resolveSchema(schemaXML string) (*parser.Schema, error) {
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		return nil, err
	}
	if err := resolveAndValidateOwned(sch); err != nil {
		return nil, err
	}
	return sch, nil
}

func parseAndAssign(schemaXML string) (*parser.Schema, *analysis.Registry, error) {
	sch, err := resolveSchema(schemaXML)
	if err != nil {
		return nil, nil, err
	}
	reg, err := analysis.AssignIDs(sch)
	if err != nil {
		return nil, nil, err
	}
	if _, err := analysis.ResolveReferences(sch, reg); err != nil {
		return nil, nil, err
	}
	return sch, reg, nil
}

func mustResolveSchema(tb testing.TB, schemaXML string) *parser.Schema {
	tb.Helper()
	sch, err := resolveSchema(schemaXML)
	if err != nil {
		tb.Fatalf("parse schema: %v", err)
	}
	return sch
}

func resolveAndValidateOwned(sch *parser.Schema) error {
	if sch == nil {
		return fmt.Errorf("schema is nil")
	}
	if err := semanticresolve.ResolveGroupReferences(sch); err != nil {
		return fmt.Errorf("resolve group references: %w", err)
	}
	structureErrs := semanticcheck.ValidateStructure(sch)
	if len(structureErrs) > 0 {
		return formatValidationErrors(structureErrs)
	}
	if err := semanticresolve.NewResolver(sch).Resolve(); err != nil {
		return fmt.Errorf("resolve type references: %w", err)
	}
	refErrs := semanticresolve.ValidateReferences(sch)
	if len(refErrs) > 0 {
		return formatValidationErrors(refErrs)
	}
	deferredRangeErrs := semanticcheck.ValidateDeferredRangeFacetValues(sch)
	if len(deferredRangeErrs) > 0 {
		return formatValidationErrors(deferredRangeErrs)
	}
	if parser.HasPlaceholders(sch) {
		return fmt.Errorf("schema has unresolved placeholders")
	}
	return nil
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

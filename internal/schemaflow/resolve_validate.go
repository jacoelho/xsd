package schemaflow

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semanticcheck"
	"github.com/jacoelho/xsd/internal/semanticresolve"
)

// ResolveAndValidate clones the parsed schema, then runs semantic resolution and validation.
// The returned schema is resolved and validated; the input schema is never mutated.
func ResolveAndValidate(sch *parser.Schema) (*parser.Schema, error) {
	if sch == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	cloned, err := loadmerge.CloneSchemaDeep(sch)
	if err != nil {
		return nil, fmt.Errorf("clone schema: %w", err)
	}
	if err := ResolveAndValidateOwned(cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

// ResolveAndValidateOwned resolves and validates schema references in place.
// The caller must own the provided schema and expect mutation.
func ResolveAndValidateOwned(sch *parser.Schema) error {
	if sch == nil {
		return fmt.Errorf("schema is nil")
	}
	if err := semanticresolve.ResolveGroupReferences(sch); err != nil {
		return fmt.Errorf("resolve group references: %w", err)
	}
	structureErrors := semanticcheck.ValidateStructure(sch)
	if len(structureErrors) > 0 {
		return formatValidationErrors(structureErrors)
	}
	if err := semanticresolve.NewResolver(sch).Resolve(); err != nil {
		return fmt.Errorf("resolve type references: %w", err)
	}
	refErrors := semanticresolve.ValidateReferences(sch)
	if len(refErrors) > 0 {
		return formatValidationErrors(refErrors)
	}
	deferredRangeErrors := semanticcheck.ValidateDeferredRangeFacetValues(sch)
	if len(deferredRangeErrors) > 0 {
		return formatValidationErrors(deferredRangeErrors)
	}
	if parser.HasPlaceholders(sch) {
		return fmt.Errorf("schema has unresolved placeholders")
	}
	return nil
}

func formatValidationErrors(validationErrors []error) error {
	if len(validationErrors) == 0 {
		return nil
	}
	errs := validationErrors
	if len(validationErrors) > 1 {
		errs = make([]error, len(validationErrors))
		copy(errs, validationErrors)
		slices.SortStableFunc(errs, func(a, b error) int {
			return strings.Compare(a.Error(), b.Error())
		})
	}
	var errMsg strings.Builder
	errMsg.WriteString("schema validation failed:")
	for _, err := range errs {
		errMsg.WriteString("\n  - ")
		errMsg.WriteString(err.Error())
	}
	return errors.New(errMsg.String())
}

package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

// ResolveAndValidateSchema runs schema semantic preparation checks in compile order.
// It returns validation errors separately from hard preparation failures.
func ResolveAndValidateSchema(sch *parser.Schema) ([]error, error) {
	if sch == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if err := ResolveGroupReferences(sch); err != nil {
		return nil, fmt.Errorf("resolve group references: %w", err)
	}
	if structureErrs := ValidateStructure(sch); len(structureErrs) > 0 {
		return structureErrs, nil
	}
	if err := NewResolver(sch).Resolve(); err != nil {
		return nil, fmt.Errorf("resolve type references: %w", err)
	}
	if refErrs := ValidateReferences(sch); len(refErrs) > 0 {
		return refErrs, nil
	}
	if deferredRangeErrs := ValidateDeferredRangeFacetValues(sch); len(deferredRangeErrs) > 0 {
		return deferredRangeErrs, nil
	}
	if parser.HasPlaceholders(sch) {
		return nil, fmt.Errorf("schema has unresolved placeholders")
	}
	return nil, nil
}

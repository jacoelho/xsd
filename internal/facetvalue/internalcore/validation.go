package internalcore

import "fmt"

type ApplyFacetOps struct {
	ValidateFacet func(facet any, value any, baseType any) error
}

// ApplyFacets applies all facets using callback-based validation.
func ApplyFacets(value any, facets []any, baseType any, ops ApplyFacetOps) error {
	for _, facet := range facets {
		if err := ops.ValidateFacet(facet, value, baseType); err != nil {
			return err
		}
	}
	return nil
}

type ValidateFacetOps struct {
	FacetName                       func(facet any) string
	ShouldSkipLengthFacet           func(baseType any, facet any) bool
	IsQNameOrNotationType           func(baseType any) bool
	IsListTypeForFacetValidation    func(baseType any) bool
	ValidateQNameEnumerationLexical func(facet any, value string, baseType any, context map[string]string) (bool, error)
	ValidateLexicalFacet            func(facet any, value string, baseType any) (bool, error)
	TypedValueForFacet              func(value string, baseType any) any
	ValidateFacet                   func(facet any, value any, baseType any) error
}

// ValidateValueAgainstFacets validates value against facets using callback adapters.
func ValidateValueAgainstFacets(value string, baseType any, facets []any, context map[string]string, ops ValidateFacetOps) error {
	if len(facets) == 0 {
		return nil
	}

	var typed any
	for _, facet := range facets {
		if ops.ShouldSkipLengthFacet(baseType, facet) {
			continue
		}

		if ops.IsQNameOrNotationType(baseType) && !ops.IsListTypeForFacetValidation(baseType) {
			if handled, err := ops.ValidateQNameEnumerationLexical(facet, value, baseType, context); handled {
				if err != nil {
					return err
				}
				continue
			}
		}

		if handled, err := ops.ValidateLexicalFacet(facet, value, baseType); handled {
			if err != nil {
				return fmt.Errorf("facet '%s' violation: %w", ops.FacetName(facet), err)
			}
			continue
		}

		if typed == nil {
			typed = ops.TypedValueForFacet(value, baseType)
		}
		if err := ops.ValidateFacet(facet, typed, baseType); err != nil {
			return fmt.Errorf("facet '%s' violation: %w", ops.FacetName(facet), err)
		}
	}

	return nil
}

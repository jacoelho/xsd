package semanticcheck

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

// validateFacetInheritance validates that derived facets are valid restrictions of base type facets
func validateFacetInheritance(derivedFacets []types.Facet, baseType types.Type) error {
	return validateFacetInheritanceWithVisited(derivedFacets, baseType, make(map[types.Type]bool))
}

// validateFacetInheritanceWithVisited validates facet inheritance with cycle detection
func validateFacetInheritanceWithVisited(derivedFacets []types.Facet, baseType types.Type, visited map[types.Type]bool) error {
	if baseType == nil {
		return nil // no base type, nothing to inherit
	}

	if visited[baseType] {
		return nil // already visited, skip to avoid infinite recursion
	}
	visited[baseType] = true

	// get base type facets if it's a user-defined simple type with restrictions
	var baseFacets []types.Facet
	switch bt := baseType.(type) {
	case *types.SimpleType:
		if bt.Restriction == nil {
			return nil
		}
		// convert []interface{} to []types.Facet
		baseFacets = make([]types.Facet, 0, len(bt.Restriction.Facets))
		for _, f := range bt.Restriction.Facets {
			switch facet := f.(type) {
			case types.Facet:
				baseFacets = append(baseFacets, facet)
			case *types.DeferredFacet:
				resolvedFacet, err := convertDeferredFacet(facet, bt.BaseType())
				if err != nil {
					return err
				}
				if resolvedFacet != nil {
					baseFacets = append(baseFacets, resolvedFacet)
				}
			}
		}
		// note: We don't recursively validate base type's base type here
		// that validation happens when the base type itself is validated
	case *types.BuiltinType:
		baseFacets = implicitRangeFacetsForBuiltin(bt)
	default:
		// built-in types don't have explicit facets in Restriction.Facets
		// they have implicit facets defined by the type itself
		return nil
	}

	if len(baseFacets) == 0 {
		return nil // no base facets to inherit
	}

	baseFacetMap := make(map[string]types.Facet)
	for _, facet := range baseFacets {
		baseFacetMap[facet.Name()] = facet
	}

	if err := validateRangeFacetInheritance(derivedFacets, baseFacets, baseType); err != nil {
		return err
	}

	for _, derivedFacet := range derivedFacets {
		facetName := derivedFacet.Name()
		if baseFacet, exists := baseFacetMap[facetName]; exists {
			// facet exists in base - validate that derived is stricter
			if err := validateFacetRestriction(facetName, baseFacet, derivedFacet, baseType); err != nil {
				return err
			}
		}
		// if facet doesn't exist in base, it's a new facet (allowed if applicable)
	}

	return nil
}

func validateRangeFacetInheritance(derivedFacets, baseFacets []types.Facet, baseType types.Type) error {
	base := extractRangeFacetInfo(baseFacets)
	derived := extractRangeFacetInfo(derivedFacets)

	if base.hasMin && derived.hasMin {
		cmp, err := compareFacetValues(derived.minValue, base.minValue, baseType)
		if errors.Is(err, errDurationNotComparable) || errors.Is(err, errFloatNotComparable) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("min facet: cannot compare values: %w", err)
		}
		if cmp < 0 {
			return fmt.Errorf("min facet: derived value (%s) must be >= base value (%s) to be a valid restriction", derived.minValue, base.minValue)
		}
		if cmp == 0 && !base.minInclusive && derived.minInclusive {
			return fmt.Errorf("min facet: derived inclusive value (%s) cannot relax base exclusive bound", derived.minValue)
		}
	}

	if base.hasMax && derived.hasMax {
		cmp, err := compareFacetValues(derived.maxValue, base.maxValue, baseType)
		if errors.Is(err, errDurationNotComparable) || errors.Is(err, errFloatNotComparable) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("max facet: cannot compare values: %w", err)
		}
		if cmp > 0 {
			return fmt.Errorf("max facet: derived value (%s) must be <= base value (%s) to be a valid restriction", derived.maxValue, base.maxValue)
		}
		if cmp == 0 && !base.maxInclusive && derived.maxInclusive {
			return fmt.Errorf("max facet: derived inclusive value (%s) cannot relax base exclusive bound", derived.maxValue)
		}
	}

	// ensure derived min does not exceed base max (inherited constraint).
	if base.hasMax && derived.hasMin {
		cmp, err := compareFacetValues(derived.minValue, base.maxValue, baseType)
		if errors.Is(err, errDurationNotComparable) || errors.Is(err, errFloatNotComparable) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("min/max facet: cannot compare values: %w", err)
		}
		if cmp > 0 {
			return fmt.Errorf("min/max facet: derived min (%s) must be <= base max (%s)", derived.minValue, base.maxValue)
		}
		if cmp == 0 && (!base.maxInclusive || !derived.minInclusive) {
			return fmt.Errorf("min/max facet: derived min (%s) cannot relax base max bound", derived.minValue)
		}
	}

	// ensure derived max does not fall below base min (inherited constraint).
	if base.hasMin && derived.hasMax {
		cmp, err := compareFacetValues(derived.maxValue, base.minValue, baseType)
		if errors.Is(err, errDurationNotComparable) || errors.Is(err, errFloatNotComparable) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("min/max facet: cannot compare values: %w", err)
		}
		if cmp < 0 {
			return fmt.Errorf("min/max facet: derived max (%s) must be >= base min (%s)", derived.maxValue, base.minValue)
		}
		if cmp == 0 && (!base.minInclusive || !derived.maxInclusive) {
			return fmt.Errorf("min/max facet: derived max (%s) cannot relax base min bound", derived.maxValue)
		}
	}

	return nil
}

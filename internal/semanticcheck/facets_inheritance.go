package semanticcheck

import (
	"errors"
	"fmt"

	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/model"
)

// validateFacetInheritance validates that derived facets are valid restrictions of base type facets
func validateFacetInheritance(derivedFacets []model.Facet, baseType model.Type) error {
	visited := make(map[model.Type]bool)
	var walk = func(current model.Type) error {
		if current == nil {
			return nil
		}
		if visited[current] {
			return nil
		}
		visited[current] = true

		// get base type facets if it's a user-defined simple type with restrictions
		var baseFacets []model.Facet
		switch bt := current.(type) {
		case *model.SimpleType:
			if bt.Restriction == nil {
				return nil
			}

			baseFacets = make([]model.Facet, 0, len(bt.Restriction.Facets))
			for _, f := range bt.Restriction.Facets {
				switch facet := f.(type) {
				case model.Facet:
					baseFacets = append(baseFacets, facet)
				case *model.DeferredFacet:
					resolvedFacet, err := convertDeferredFacet(facet, bt.BaseType())
					if err != nil {
						return err
					}
					if resolvedFacet != nil {
						baseFacets = append(baseFacets, resolvedFacet)
					}
				}
			}

		case *model.BuiltinType:
			baseFacets = implicitRangeFacetsForBuiltin(bt)
		default:

			return nil
		}

		if len(baseFacets) == 0 {
			return nil
		}

		baseFacetMap := make(map[string]model.Facet)
		for _, facet := range baseFacets {
			baseFacetMap[facet.Name()] = facet
		}

		if err := validateRangeFacetInheritance(derivedFacets, baseFacets, current); err != nil {
			return err
		}

		for _, derivedFacet := range derivedFacets {
			facetName := derivedFacet.Name()
			if baseFacet, exists := baseFacetMap[facetName]; exists {

				if err := validateFacetRestriction(facetName, baseFacet, derivedFacet, current); err != nil {
					return err
				}
			}

		}

		return nil
	}

	if err := walk(baseType); err != nil {
		return err
	}

	return nil
}

func validateRangeFacetInheritance(derivedFacets, baseFacets []model.Facet, baseType model.Type) error {
	base := extractRangeFacetInfo(baseFacets)
	derived := extractRangeFacetInfo(derivedFacets)

	if base.hasMin && derived.hasMin {
		cmp, err := facetengine.CompareFacetValues(derived.minValue, base.minValue, baseType)
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
		cmp, err := facetengine.CompareFacetValues(derived.maxValue, base.maxValue, baseType)
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
		cmp, err := facetengine.CompareFacetValues(derived.minValue, base.maxValue, baseType)
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
		cmp, err := facetengine.CompareFacetValues(derived.maxValue, base.minValue, baseType)
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

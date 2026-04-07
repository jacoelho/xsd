package semantics

import (
	"errors"
	"fmt"

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

	return walk(baseType)
}

func validateRangeFacetInheritance(derivedFacets, baseFacets []model.Facet, baseType model.Type) error {
	base := extractRangeFacetInfo(baseFacets)
	derived := extractRangeFacetInfo(derivedFacets)

	checks := []rangeFacetCheck{
		{
			active:             base.hasMin && derived.hasMin,
			errPrefix:          "min facet",
			left:               derived.minValue,
			right:              base.minValue,
			invalidOrdering:    func(cmp int) bool { return cmp < 0 },
			invalidComparison:  "derived value (%s) must be >= base value (%s) to be a valid restriction",
			invalidEquality:    cmpEqualsZero,
			invalidEqualBounds: func() bool { return !base.minInclusive && derived.minInclusive },
			invalidEqualMsg:    "derived inclusive value (%s) cannot relax base exclusive bound",
		},
		{
			active:             base.hasMax && derived.hasMax,
			errPrefix:          "max facet",
			left:               derived.maxValue,
			right:              base.maxValue,
			invalidOrdering:    func(cmp int) bool { return cmp > 0 },
			invalidComparison:  "derived value (%s) must be <= base value (%s) to be a valid restriction",
			invalidEquality:    cmpEqualsZero,
			invalidEqualBounds: func() bool { return !base.maxInclusive && derived.maxInclusive },
			invalidEqualMsg:    "derived inclusive value (%s) cannot relax base exclusive bound",
		},
		{
			active:             base.hasMax && derived.hasMin,
			errPrefix:          "min/max facet",
			left:               derived.minValue,
			right:              base.maxValue,
			invalidOrdering:    func(cmp int) bool { return cmp > 0 },
			invalidComparison:  "derived min (%s) must be <= base max (%s)",
			invalidEquality:    cmpEqualsZero,
			invalidEqualBounds: func() bool { return !base.maxInclusive || !derived.minInclusive },
			invalidEqualMsg:    "derived min (%s) cannot relax base max bound",
		},
		{
			active:             base.hasMin && derived.hasMax,
			errPrefix:          "min/max facet",
			left:               derived.maxValue,
			right:              base.minValue,
			invalidOrdering:    func(cmp int) bool { return cmp < 0 },
			invalidComparison:  "derived max (%s) must be >= base min (%s)",
			invalidEquality:    cmpEqualsZero,
			invalidEqualBounds: func() bool { return !base.minInclusive || !derived.maxInclusive },
			invalidEqualMsg:    "derived max (%s) cannot relax base min bound",
		},
	}

	for _, check := range checks {
		ok, err := validateRangeFacetCheck(check, baseType)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}

	return nil
}

type rangeFacetCheck struct {
	active             bool
	errPrefix          string
	left               string
	right              string
	invalidOrdering    func(int) bool
	invalidComparison  string
	invalidEquality    func(int) bool
	invalidEqualBounds func() bool
	invalidEqualMsg    string
}

func validateRangeFacetCheck(check rangeFacetCheck, baseType model.Type) (bool, error) {
	if !check.active {
		return true, nil
	}
	cmp, comparable, err := compareFacetBounds(check.left, check.right, baseType)
	if !comparable {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%s: cannot compare values: %w", check.errPrefix, err)
	}
	if check.invalidOrdering(cmp) {
		return false, fmt.Errorf("%s: "+check.invalidComparison, check.errPrefix, check.left, check.right)
	}
	if check.invalidEquality(cmp) && check.invalidEqualBounds() {
		return false, fmt.Errorf("%s: "+check.invalidEqualMsg, check.errPrefix, check.left)
	}
	return true, nil
}

func cmpEqualsZero(cmp int) bool {
	return cmp == 0
}

func compareFacetBounds(left, right string, baseType model.Type) (int, bool, error) {
	cmp, err := CompareFacetValues(left, right, baseType)
	if errors.Is(err, errDurationNotComparable) || errors.Is(err, errFloatNotComparable) {
		return 0, false, nil
	}
	if err != nil {
		return 0, true, err
	}
	return cmp, true, nil
}

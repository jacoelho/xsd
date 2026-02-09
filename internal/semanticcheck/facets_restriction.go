package semanticcheck

import (
	"errors"
	"fmt"

	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/types"
)

// validateFacetRestriction validates that a derived facet is a valid restriction of the base facet
func validateFacetRestriction(facetName string, baseFacet, derivedFacet types.Facet, baseType types.Type) error {
	switch facetName {
	case "maxInclusive", "maxExclusive":
		// for max facets, derived value must be <= base value (stricter = smaller)
		baseLexical, baseOk := baseFacet.(types.LexicalFacet)
		derivedLexical, derivedOk := derivedFacet.(types.LexicalFacet)
		if !baseOk || !derivedOk {
			return nil
		}
		baseValStr := baseLexical.GetLexical()
		derivedValStr := derivedLexical.GetLexical()
		if baseValStr == "" || derivedValStr == "" {
			return nil
		}
		cmp, err := facetengine.CompareFacetValues(derivedValStr, baseValStr, baseType)
		if errors.Is(err, errDurationNotComparable) || errors.Is(err, errFloatNotComparable) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("facet %s: cannot compare values: %w", facetName, err)
		}
		if cmp > 0 {
			return fmt.Errorf("facet %s: derived value (%s) must be <= base value (%s) to be a valid restriction", facetName, derivedValStr, baseValStr)
		}

	case "minInclusive", "minExclusive":
		// for min facets, derived value must be >= base value (stricter = larger)
		baseLexical, baseOk := baseFacet.(types.LexicalFacet)
		derivedLexical, derivedOk := derivedFacet.(types.LexicalFacet)
		if !baseOk || !derivedOk {
			return nil
		}
		baseValStr := baseLexical.GetLexical()
		derivedValStr := derivedLexical.GetLexical()
		if baseValStr == "" || derivedValStr == "" {
			return nil
		}
		cmp, err := facetengine.CompareFacetValues(derivedValStr, baseValStr, baseType)
		if errors.Is(err, errDurationNotComparable) || errors.Is(err, errFloatNotComparable) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("facet %s: cannot compare values: %w", facetName, err)
		}
		if cmp < 0 {
			return fmt.Errorf("facet %s: derived value (%s) must be >= base value (%s) to be a valid restriction", facetName, derivedValStr, baseValStr)
		}

	case "maxLength":
		// for maxLength, derived value must be <= base value
		baseIntValue, baseOk := baseFacet.(types.IntValueFacet)
		derivedIntValue, derivedOk := derivedFacet.(types.IntValueFacet)
		if baseOk && derivedOk {
			baseValInt := baseIntValue.GetIntValue()
			derivedValInt := derivedIntValue.GetIntValue()
			if derivedValInt > baseValInt {
				return fmt.Errorf("facet %s: derived value (%d) must be <= base value (%d) to be a valid restriction", facetName, derivedValInt, baseValInt)
			}
		}

	case "minLength":
		// for minLength, derived value must be >= base value
		baseIntValue, baseOk := baseFacet.(types.IntValueFacet)
		derivedIntValue, derivedOk := derivedFacet.(types.IntValueFacet)
		if baseOk && derivedOk {
			baseValInt := baseIntValue.GetIntValue()
			derivedValInt := derivedIntValue.GetIntValue()
			if derivedValInt < baseValInt {
				return fmt.Errorf("facet %s: derived value (%d) must be >= base value (%d) to be a valid restriction", facetName, derivedValInt, baseValInt)
			}
		}

	case "length":
		// for length, derived value must equal base value (can't change length in restriction)
		baseIntValue, baseOk := baseFacet.(types.IntValueFacet)
		derivedIntValue, derivedOk := derivedFacet.(types.IntValueFacet)
		if baseOk && derivedOk {
			baseValInt := baseIntValue.GetIntValue()
			derivedValInt := derivedIntValue.GetIntValue()
			if derivedValInt != baseValInt {
				return fmt.Errorf("facet %s: derived value (%d) must equal base value (%d) in a restriction", facetName, derivedValInt, baseValInt)
			}
		}

	case "totalDigits", "fractionDigits":
		// for digit facets, derived value must be <= base value
		baseIntValue, baseOk := baseFacet.(types.IntValueFacet)
		derivedIntValue, derivedOk := derivedFacet.(types.IntValueFacet)
		if baseOk && derivedOk {
			baseValInt := baseIntValue.GetIntValue()
			derivedValInt := derivedIntValue.GetIntValue()
			if derivedValInt > baseValInt {
				return fmt.Errorf("facet %s: derived value (%d) must be <= base value (%d) to be a valid restriction", facetName, derivedValInt, baseValInt)
			}
		}

	case "pattern":
		// pattern facets: derived pattern must be a subset of base pattern
		// this is complex to validate, so for now we'll allow it
		// (pattern validation is done separately)

	case "enumeration":
		// enumeration: derived values must be a subset of base values
		baseEnum, baseOk := baseFacet.(*types.Enumeration)
		derivedEnum, derivedOk := derivedFacet.(*types.Enumeration)
		if baseOk && derivedOk {
			baseValues := make(map[string]bool)
			for _, val := range baseEnum.Values() {
				baseValues[val] = true
			}
			for _, derivedValStr := range derivedEnum.Values() {
				if !baseValues[derivedValStr] {
					return fmt.Errorf("facet %s: derived enumeration value (%s) must be in base enumeration", facetName, derivedValStr)
				}
			}
		}

	case "whiteSpace":
		// whiteSpace: can only be made stricter (preserve -> replace -> collapse)
		// note: whiteSpace is stored on SimpleType, not as a Facet, so this case is
		// handled in validateWhiteSpaceRestriction separately.
	}

	return nil
}

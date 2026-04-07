package semantics

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

// validateFacetRestriction validates that a derived facet is a valid restriction of the base facet
func validateFacetRestriction(facetName string, baseFacet, derivedFacet model.Facet, baseType model.Type) error {
	switch facetName {
	case "maxInclusive", "maxExclusive", "minInclusive", "minExclusive":
		return validateRangeFacetRestriction(facetName, baseFacet, derivedFacet, baseType)
	case "maxLength":
		return validateIntFacetRestriction(facetName, baseFacet, derivedFacet, func(derived, base int) bool { return derived <= base }, "<=")
	case "minLength":
		return validateIntFacetRestriction(facetName, baseFacet, derivedFacet, func(derived, base int) bool { return derived >= base }, ">=")
	case "length":
		return validateIntFacetRestriction(facetName, baseFacet, derivedFacet, func(derived, base int) bool { return derived == base }, "equal")
	case "totalDigits", "fractionDigits":
		return validateIntFacetRestriction(facetName, baseFacet, derivedFacet, func(derived, base int) bool { return derived <= base }, "<=")
	case "pattern":
		return nil
	case "enumeration":
		return validateEnumerationFacetRestriction(facetName, baseFacet, derivedFacet)
	case "whiteSpace":
		return nil
	}

	return nil
}

func validateRangeFacetRestriction(facetName string, baseFacet, derivedFacet model.Facet, baseType model.Type) error {
	baseValStr, derivedValStr, ok := lexicalFacetValues(baseFacet, derivedFacet)
	if !ok || baseValStr == "" || derivedValStr == "" {
		return nil
	}
	cmp, err := CompareFacetValues(derivedValStr, baseValStr, baseType)
	if errors.Is(err, errDurationNotComparable) || errors.Is(err, errFloatNotComparable) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("facet %s: cannot compare values: %w", facetName, err)
	}
	matches, ok := RestrictionRangeSatisfied(facetName, cmp)
	if !ok || matches {
		return nil
	}
	rule, _ := RestrictionRange(facetName)
	return fmt.Errorf("facet %s: derived value (%s) must be %s base value (%s) to be a valid restriction", facetName, derivedValStr, rule.Comparator, baseValStr)
}

func lexicalFacetValues(baseFacet, derivedFacet model.Facet) (string, string, bool) {
	baseLexical, baseOK := baseFacet.(model.LexicalFacet)
	derivedLexical, derivedOK := derivedFacet.(model.LexicalFacet)
	if !baseOK || !derivedOK {
		return "", "", false
	}
	return baseLexical.GetLexical(), derivedLexical.GetLexical(), true
}

func validateIntFacetRestriction(facetName string, baseFacet, derivedFacet model.Facet, allow func(derived, base int) bool, comparator string) error {
	baseValInt, derivedValInt, ok := intFacetValues(baseFacet, derivedFacet)
	if !ok || allow(derivedValInt, baseValInt) {
		return nil
	}
	if comparator == "equal" {
		return fmt.Errorf("facet %s: derived value (%d) must equal base value (%d) in a restriction", facetName, derivedValInt, baseValInt)
	}
	return fmt.Errorf("facet %s: derived value (%d) must be %s base value (%d) to be a valid restriction", facetName, derivedValInt, comparator, baseValInt)
}

func intFacetValues(baseFacet, derivedFacet model.Facet) (int, int, bool) {
	baseIntValue, baseOK := baseFacet.(model.IntValueFacet)
	derivedIntValue, derivedOK := derivedFacet.(model.IntValueFacet)
	if !baseOK || !derivedOK {
		return 0, 0, false
	}
	return baseIntValue.GetIntValue(), derivedIntValue.GetIntValue(), true
}

func validateEnumerationFacetRestriction(facetName string, baseFacet, derivedFacet model.Facet) error {
	baseEnum, baseOK := baseFacet.(*model.Enumeration)
	derivedEnum, derivedOK := derivedFacet.(*model.Enumeration)
	if !baseOK || !derivedOK {
		return nil
	}
	baseValues := make(map[string]bool, len(baseEnum.Values()))
	for _, val := range baseEnum.Values() {
		baseValues[val] = true
	}
	for _, derivedValStr := range derivedEnum.Values() {
		if !baseValues[derivedValStr] {
			return fmt.Errorf("facet %s: derived enumeration value (%s) must be in base enumeration", facetName, derivedValStr)
		}
	}
	return nil
}

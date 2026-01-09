package validation

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jacoelho/xsd/internal/types"
)

type rangeFacetInfo struct {
	minValue     *string
	minInclusive bool
	maxValue     *string
	maxInclusive bool
}

var builtinRangeFacetInfo = map[string]rangeFacetInfo{
	"positiveInteger":    {minValue: stringPtr("1"), minInclusive: true},
	"nonNegativeInteger": {minValue: stringPtr("0"), minInclusive: true},
	"negativeInteger":    {maxValue: stringPtr("-1"), maxInclusive: true},
	"nonPositiveInteger": {maxValue: stringPtr("0"), maxInclusive: true},
	"byte":               {minValue: stringPtr("-128"), minInclusive: true, maxValue: stringPtr("127"), maxInclusive: true},
	"short":              {minValue: stringPtr("-32768"), minInclusive: true, maxValue: stringPtr("32767"), maxInclusive: true},
	"int":                {minValue: stringPtr("-2147483648"), minInclusive: true, maxValue: stringPtr("2147483647"), maxInclusive: true},
	"long":               {minValue: stringPtr("-9223372036854775808"), minInclusive: true, maxValue: stringPtr("9223372036854775807"), maxInclusive: true},
	"unsignedByte":       {minValue: stringPtr("0"), minInclusive: true, maxValue: stringPtr("255"), maxInclusive: true},
	"unsignedShort":      {minValue: stringPtr("0"), minInclusive: true, maxValue: stringPtr("65535"), maxInclusive: true},
	"unsignedInt":        {minValue: stringPtr("0"), minInclusive: true, maxValue: stringPtr("4294967295"), maxInclusive: true},
	"unsignedLong":       {minValue: stringPtr("0"), minInclusive: true, maxValue: stringPtr("18446744073709551615"), maxInclusive: true},
}

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
	if baseST, ok := baseType.(*types.SimpleType); ok && baseST.Restriction != nil {
		// convert []interface{} to []types.Facet
		baseFacets = make([]types.Facet, 0, len(baseST.Restriction.Facets))
		for _, f := range baseST.Restriction.Facets {
			if facet, ok := f.(types.Facet); ok {
				baseFacets = append(baseFacets, facet)
			} else if df, ok := f.(*types.DeferredFacet); ok {
				resolvedFacet, err := convertDeferredFacet(df, baseST.BaseType())
				if err != nil {
					continue
				}
				if resolvedFacet != nil {
					baseFacets = append(baseFacets, resolvedFacet)
				}
			}
		}
		// note: We don't recursively validate base type's base type here
		// that validation happens when the base type itself is validated
	} else if bt, ok := baseType.(*types.BuiltinType); ok {
		baseFacets = implicitRangeFacetsForBuiltin(bt)
	} else {
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

func implicitRangeFacetsForBuiltin(bt *types.BuiltinType) []types.Facet {
	info, ok := builtinRangeFacetInfo[bt.Name().Local]
	if !ok {
		return nil
	}
	var result []types.Facet
	if info.minValue != nil {
		if info.minInclusive {
			if facet, err := types.NewMinInclusive(*info.minValue, bt); err == nil {
				result = append(result, facet)
			}
		} else if facet, err := types.NewMinExclusive(*info.minValue, bt); err == nil {
			result = append(result, facet)
		}
	}
	if info.maxValue != nil {
		if info.maxInclusive {
			if facet, err := types.NewMaxInclusive(*info.maxValue, bt); err == nil {
				result = append(result, facet)
			}
		} else if facet, err := types.NewMaxExclusive(*info.maxValue, bt); err == nil {
			result = append(result, facet)
		}
	}
	return result
}

func extractRangeFacetInfo(facetsList []types.Facet) rangeFacetInfo {
	var info rangeFacetInfo
	for _, facet := range facetsList {
		lex, ok := facet.(types.LexicalFacet)
		if !ok {
			continue
		}
		val := lex.GetLexical()
		if val == "" {
			continue
		}
		switch facet.Name() {
		case "minInclusive":
			info.minValue = &val
			info.minInclusive = true
		case "minExclusive":
			info.minValue = &val
			info.minInclusive = false
		case "maxInclusive":
			info.maxValue = &val
			info.maxInclusive = true
		case "maxExclusive":
			info.maxValue = &val
			info.maxInclusive = false
		}
	}
	return info
}

func validateRangeFacetInheritance(derivedFacets, baseFacets []types.Facet, baseType types.Type) error {
	base := extractRangeFacetInfo(baseFacets)
	derived := extractRangeFacetInfo(derivedFacets)

	if base.minValue != nil && derived.minValue != nil {
		cmp, err := compareFacetValues(*derived.minValue, *base.minValue, baseType)
		if errors.Is(err, errDurationNotComparable) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("min facet: cannot compare values: %w", err)
		}
		if cmp < 0 {
			return fmt.Errorf("min facet: derived value (%s) must be >= base value (%s) to be a valid restriction", *derived.minValue, *base.minValue)
		}
		if cmp == 0 && !base.minInclusive && derived.minInclusive {
			return fmt.Errorf("min facet: derived inclusive value (%s) cannot relax base exclusive bound", *derived.minValue)
		}
	}

	if base.maxValue != nil && derived.maxValue != nil {
		cmp, err := compareFacetValues(*derived.maxValue, *base.maxValue, baseType)
		if errors.Is(err, errDurationNotComparable) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("max facet: cannot compare values: %w", err)
		}
		if cmp > 0 {
			return fmt.Errorf("max facet: derived value (%s) must be <= base value (%s) to be a valid restriction", *derived.maxValue, *base.maxValue)
		}
		if cmp == 0 && !base.maxInclusive && derived.maxInclusive {
			return fmt.Errorf("max facet: derived inclusive value (%s) cannot relax base exclusive bound", *derived.maxValue)
		}
	}

	// ensure derived min does not exceed base max (inherited constraint).
	if base.maxValue != nil && derived.minValue != nil {
		cmp, err := compareFacetValues(*derived.minValue, *base.maxValue, baseType)
		if errors.Is(err, errDurationNotComparable) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("min/max facet: cannot compare values: %w", err)
		}
		if cmp > 0 {
			return fmt.Errorf("min/max facet: derived min (%s) must be <= base max (%s)", *derived.minValue, *base.maxValue)
		}
		if cmp == 0 && (!base.maxInclusive || !derived.minInclusive) {
			return fmt.Errorf("min/max facet: derived min (%s) cannot relax base max bound", *derived.minValue)
		}
	}

	// ensure derived max does not fall below base min (inherited constraint).
	if base.minValue != nil && derived.maxValue != nil {
		cmp, err := compareFacetValues(*derived.maxValue, *base.minValue, baseType)
		if errors.Is(err, errDurationNotComparable) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("min/max facet: cannot compare values: %w", err)
		}
		if cmp < 0 {
			return fmt.Errorf("min/max facet: derived max (%s) must be >= base min (%s)", *derived.maxValue, *base.minValue)
		}
		if cmp == 0 && (!base.minInclusive || !derived.maxInclusive) {
			return fmt.Errorf("min/max facet: derived max (%s) cannot relax base min bound", *derived.maxValue)
		}
	}

	return nil
}

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
		cmp, err := compareFacetValues(derivedValStr, baseValStr, baseType)
		if errors.Is(err, errDurationNotComparable) {
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
		cmp, err := compareFacetValues(derivedValStr, baseValStr, baseType)
		if errors.Is(err, errDurationNotComparable) {
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
			for _, val := range baseEnum.Values {
				baseValues[val] = true
			}
			for _, derivedValStr := range derivedEnum.Values {
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

// compareFacetValues compares two facet values according to the base type
// Returns: -1 if val1 < val2, 0 if equal, 1 if val1 > val2
func compareFacetValues(val1, val2 string, baseType types.Type) (int, error) {
	var primitiveType types.Type
	if st, ok := baseType.(*types.SimpleType); ok {
		primitiveType = st.PrimitiveType()
		if primitiveType == nil {
			// fall back to base type
			primitiveType = baseType
		}
	} else {
		primitiveType = baseType
	}

	// check if it's a numeric type by checking the primitive type name
	if st, ok := primitiveType.(*types.SimpleType); ok {
		typeName := st.QName.Local
		if typeName == "duration" {
			return compareDurationValues(val1, val2)
		}
		if isNumericTypeName(typeName) {
			return compareNumericFacetValues(val1, val2)
		}
		if facets := st.FundamentalFacets(); facets != nil && facets.Numeric {
			return compareNumericFacetValues(val1, val2)
		}
		// for date/time, use timezone-aware comparison.
		if facets := st.FundamentalFacets(); facets != nil && facets.Ordered == types.OrderedTotal {
			if isDateTimeTypeName(typeName) {
				return compareDateTimeValues(val1, val2, typeName)
			}
			return strings.Compare(val1, val2), nil
		}
	}
	if bt, ok := primitiveType.(*types.BuiltinType); ok {
		typeName := bt.Name().Local
		if isDateTimeTypeName(typeName) {
			return compareDateTimeValues(val1, val2, typeName)
		}
		if bt.FundamentalFacets() != nil && bt.FundamentalFacets().Numeric {
			return compareNumericFacetValues(val1, val2)
		}
		if bt.FundamentalFacets() != nil && bt.FundamentalFacets().Ordered == types.OrderedTotal {
			return strings.Compare(val1, val2), nil
		}
	}

	// default: try numeric comparison first (many types are numeric)
	// if that fails, use string comparison
	if cmp, err := compareNumericFacetValues(val1, val2); err == nil {
		return cmp, nil
	}

	if cmp, err := compareDurationValues(val1, val2); err == nil {
		return cmp, nil
	}

	// default: string comparison
	return strings.Compare(val1, val2), nil
}

// compareNumericFacetValues compares two numeric facet values
func compareNumericFacetValues(val1, val2 string) (int, error) {
	d1, _, err1 := big.ParseFloat(val1, 10, 256, big.ToNearestEven)
	d2, _, err2 := big.ParseFloat(val2, 10, 256, big.ToNearestEven)
	if err1 != nil || err2 != nil {
		return 0, fmt.Errorf("invalid numeric values: %s, %s", val1, val2)
	}
	return d1.Cmp(d2), nil
}

func stringPtr(val string) *string {
	return &val
}

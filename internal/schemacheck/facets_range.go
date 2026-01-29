package schemacheck

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/types"
)

// isNumericTypeName checks if a type name represents a numeric type
func isNumericTypeName(typeName string) bool {
	numericTypes := []string{
		"decimal", "float", "double", "integer", "long", "int", "short", "byte",
		"nonNegativeInteger", "positiveInteger", "unsignedLong", "unsignedInt",
		"unsignedShort", "unsignedByte", "nonPositiveInteger", "negativeInteger",
	}
	return slices.Contains(numericTypes, typeName)
}

func isDurationType(baseType types.Type, baseQName types.QName) bool {
	if baseQName.Namespace == types.XSDNamespace && baseQName.Local == "duration" {
		return true
	}
	if baseType == nil {
		return false
	}
	primitive := baseType.PrimitiveType()
	if primitive == nil {
		return false
	}
	return primitive.Name().Namespace == types.XSDNamespace && primitive.Name().Local == "duration"
}

type rangeFacetInfo struct {
	minValue     string
	maxValue     string
	minInclusive bool
	maxInclusive bool
	hasMin       bool
	hasMax       bool
}

func builtinRangeFacetInfoFor(typeName string) (rangeFacetInfo, bool) {
	switch typeName {
	case "positiveInteger":
		return rangeFacetInfo{minValue: "1", minInclusive: true, hasMin: true}, true
	case "nonNegativeInteger":
		return rangeFacetInfo{minValue: "0", minInclusive: true, hasMin: true}, true
	case "negativeInteger":
		return rangeFacetInfo{maxValue: "-1", maxInclusive: true, hasMax: true}, true
	case "nonPositiveInteger":
		return rangeFacetInfo{maxValue: "0", maxInclusive: true, hasMax: true}, true
	case "byte":
		return rangeFacetInfo{minValue: "-128", minInclusive: true, maxValue: "127", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "short":
		return rangeFacetInfo{minValue: "-32768", minInclusive: true, maxValue: "32767", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "int":
		return rangeFacetInfo{minValue: "-2147483648", minInclusive: true, maxValue: "2147483647", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "long":
		return rangeFacetInfo{minValue: "-9223372036854775808", minInclusive: true, maxValue: "9223372036854775807", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "unsignedByte":
		return rangeFacetInfo{minValue: "0", minInclusive: true, maxValue: "255", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "unsignedShort":
		return rangeFacetInfo{minValue: "0", minInclusive: true, maxValue: "65535", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "unsignedInt":
		return rangeFacetInfo{minValue: "0", minInclusive: true, maxValue: "4294967295", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "unsignedLong":
		return rangeFacetInfo{minValue: "0", minInclusive: true, maxValue: "18446744073709551615", maxInclusive: true, hasMin: true, hasMax: true}, true
	default:
		return rangeFacetInfo{}, false
	}
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

func implicitRangeFacetsForBuiltin(bt *types.BuiltinType) []types.Facet {
	info, ok := builtinRangeFacetInfoFor(bt.Name().Local)
	if !ok {
		return nil
	}
	var result []types.Facet
	if info.hasMin {
		if info.minInclusive {
			if facet, err := types.NewMinInclusive(info.minValue, bt); err == nil {
				result = append(result, facet)
			}
		} else if facet, err := types.NewMinExclusive(info.minValue, bt); err == nil {
			result = append(result, facet)
		}
	}
	if info.hasMax {
		if info.maxInclusive {
			if facet, err := types.NewMaxInclusive(info.maxValue, bt); err == nil {
				result = append(result, facet)
			}
		} else if facet, err := types.NewMaxExclusive(info.maxValue, bt); err == nil {
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
			info.minValue = val
			info.minInclusive = true
			info.hasMin = true
		case "minExclusive":
			info.minValue = val
			info.minInclusive = false
			info.hasMin = true
		case "maxInclusive":
			info.maxValue = val
			info.maxInclusive = true
			info.hasMax = true
		case "maxExclusive":
			info.maxValue = val
			info.maxInclusive = false
			info.hasMax = true
		}
	}
	return info
}

func validateRangeFacetInheritance(derivedFacets, baseFacets []types.Facet, baseType types.Type) error {
	base := extractRangeFacetInfo(baseFacets)
	derived := extractRangeFacetInfo(derivedFacets)

	if base.hasMin && derived.hasMin {
		cmp, err := compareFacetValues(derived.minValue, base.minValue, baseType)
		if errors.Is(err, errDurationNotComparable) {
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
		if errors.Is(err, errDurationNotComparable) {
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
		if errors.Is(err, errDurationNotComparable) {
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
		if errors.Is(err, errDurationNotComparable) {
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
		switch typeName {
		case "duration":
			return compareDurationValues(val1, val2)
		case "float":
			return compareFloatFacetValues(val1, val2)
		case "double":
			return compareDoubleFacetValues(val1, val2)
		}
		if isNumericTypeName(typeName) {
			return compareNumericFacetValues(val1, val2)
		}
		if facets := st.FundamentalFacets(); facets != nil {
			if facets.Numeric {
				return compareNumericFacetValues(val1, val2)
			}
			// for date/time, use timezone-aware comparison.
			if isDateTimeTypeName(typeName) {
				return compareDateTimeValues(val1, val2, typeName)
			}
			if facets.Ordered == types.OrderedTotal {
				return strings.Compare(val1, val2), nil
			}
		}
	}
	if bt, ok := primitiveType.(*types.BuiltinType); ok {
		typeName := bt.Name().Local
		switch typeName {
		case "duration":
			return compareDurationValues(val1, val2)
		case "float":
			return compareFloatFacetValues(val1, val2)
		case "double":
			return compareDoubleFacetValues(val1, val2)
		}
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
	d1, err := types.ParseDecimal(val1)
	if err != nil {
		return 0, err
	}
	d2, err := types.ParseDecimal(val2)
	if err != nil {
		return 0, err
	}
	return d1.Cmp(d2), nil
}

func compareFloatFacetValues(val1, val2 string) (int, error) {
	f1, err := types.ParseFloat(val1)
	if err != nil {
		return 0, err
	}
	f2, err := types.ParseFloat(val2)
	if err != nil {
		return 0, err
	}
	return compareFloatValues(float64(f1), float64(f2)), nil
}

func compareDoubleFacetValues(val1, val2 string) (int, error) {
	f1, err := types.ParseDouble(val1)
	if err != nil {
		return 0, err
	}
	f2, err := types.ParseDouble(val2)
	if err != nil {
		return 0, err
	}
	return compareFloatValues(f1, f2), nil
}

func compareFloatValues(v1, v2 float64) int {
	if math.IsNaN(v1) || math.IsNaN(v2) {
		return 0
	}
	if v1 < v2 {
		return -1
	}
	if v1 > v2 {
		return 1
	}
	return 0
}

// validateRangeFacets validates consistency of range facets
func validateRangeFacets(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseType types.Type, baseQName types.QName) error {
	if isDurationType(baseType, baseQName) {
		return validateDurationRangeFacets(minExclusive, maxExclusive, minInclusive, maxInclusive)
	}
	// per XSD spec: maxInclusive and maxExclusive cannot both be present
	if maxInclusive != nil && maxExclusive != nil {
		return fmt.Errorf("maxInclusive and maxExclusive cannot both be specified")
	}

	// per XSD spec: minInclusive and minExclusive cannot both be present
	if minInclusive != nil && minExclusive != nil {
		return fmt.Errorf("minInclusive and minExclusive cannot both be specified")
	}

	baseTypeForCompare := baseType
	if baseTypeForCompare == nil {
		if bt := types.GetBuiltinNS(baseQName.Namespace, baseQName.Local); bt != nil {
			baseTypeForCompare = bt
		}
	}

	compare := func(v1, v2 string) (int, bool, error) {
		if baseTypeForCompare == nil {
			return 0, false, nil
		}
		if facets := baseTypeForCompare.FundamentalFacets(); facets != nil && facets.Ordered == types.OrderedNone {
			return 0, false, nil
		}
		cmp, err := compareFacetValues(v1, v2, baseTypeForCompare)
		if errors.Is(err, errDateTimeNotComparable) || errors.Is(err, errDurationNotComparable) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		return cmp, true, nil
	}

	if minExclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minExclusive/maxInclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
		}
	}
	if minExclusive != nil && maxExclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxExclusive); err != nil {
			return fmt.Errorf("minExclusive/maxExclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxExclusive (%s)", *minExclusive, *maxExclusive)
		}
	}
	if minInclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minInclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minInclusive/maxInclusive: %w", err)
		} else if ok && cmp > 0 {
			return fmt.Errorf("minInclusive (%s) must be <= maxInclusive (%s)", *minInclusive, *maxInclusive)
		}
	}
	if minInclusive != nil && maxExclusive != nil {
		if cmp, ok, err := compare(*minInclusive, *maxExclusive); err != nil {
			return fmt.Errorf("minInclusive/maxExclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minInclusive (%s) must be < maxExclusive (%s)", *minInclusive, *maxExclusive)
		}
	}
	if minExclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minExclusive/maxInclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
		}
	}

	return nil
}

func validateDurationRangeFacets(minExclusive, maxExclusive, minInclusive, maxInclusive *string) error {
	compare := func(v1, v2 string) (int, bool, error) {
		cmp, err := compareDurationValues(v1, v2)
		if errors.Is(err, errDurationNotComparable) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		return cmp, true, nil
	}

	if maxInclusive != nil && maxExclusive != nil {
		return fmt.Errorf("maxInclusive and maxExclusive cannot both be specified")
	}
	if minInclusive != nil && minExclusive != nil {
		return fmt.Errorf("minInclusive and minExclusive cannot both be specified")
	}

	if minExclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minExclusive/maxInclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
		}
	}
	if minExclusive != nil && maxExclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxExclusive); err != nil {
			return fmt.Errorf("minExclusive/maxExclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxExclusive (%s)", *minExclusive, *maxExclusive)
		}
	}
	if minInclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minInclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minInclusive/maxInclusive: %w", err)
		} else if ok && cmp > 0 {
			return fmt.Errorf("minInclusive (%s) must be <= maxInclusive (%s)", *minInclusive, *maxInclusive)
		}
	}
	if minInclusive != nil && maxExclusive != nil {
		if cmp, ok, err := compare(*minInclusive, *maxExclusive); err != nil {
			return fmt.Errorf("minInclusive/maxExclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minInclusive (%s) must be < maxExclusive (%s)", *minInclusive, *maxExclusive)
		}
	}
	if minExclusive != nil && maxInclusive != nil {
		if cmp, ok, err := compare(*minExclusive, *maxInclusive); err != nil {
			return fmt.Errorf("minExclusive/maxInclusive: %w", err)
		} else if ok && cmp >= 0 {
			return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
		}
	}

	return nil
}

// validateRangeFacetValues validates that range facet values are within the base type's value space
// Per XSD spec, facet values must be valid for the base type
func validateRangeFacetValues(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseType types.Type, bt *types.BuiltinType) error {
	// try to get a validator and whitespace handling for the base type
	var validator types.TypeValidator
	var whiteSpace types.WhiteSpace

	if bt != nil {
		validator = func(value string) error {
			return bt.Validate(value)
		}
		whiteSpace = bt.WhiteSpace()
	} else if baseType != nil {
		// for user-defined types, try to get the underlying built-in type validator
		switch t := baseType.(type) {
		case *types.BuiltinType:
			validator = func(value string) error {
				return t.Validate(value)
			}
			whiteSpace = t.WhiteSpace()
		case *types.SimpleType:
			// for SimpleType, check if it has a built-in base
			if t.IsBuiltin() || t.QName.Namespace == types.XSDNamespace {
				if builtinType := types.GetBuiltinNS(t.QName.Namespace, t.QName.Local); builtinType != nil {
					validator = func(value string) error {
						return builtinType.Validate(value)
					}
					whiteSpace = builtinType.WhiteSpace()
				}
			} else {
				builtinType := findBuiltinAncestor(baseType)
				if builtinType != nil {
					validator = func(value string) error {
						return builtinType.Validate(value)
					}
					whiteSpace = builtinType.WhiteSpace()
				}
			}
		}
	}

	if validator == nil {
		return nil // can't validate without a validator
	}

	// helper to normalize whitespace before validation
	normalizeValue := func(val string) string {
		switch whiteSpace {
		case types.WhiteSpaceCollapse:
			// collapse: replace sequences of whitespace with single space, trim leading/trailing
			val = types.TrimXMLWhitespace(val)
			// replace multiple whitespace with single space
			return joinFields(val)
		case types.WhiteSpaceReplace:
			// replace: replace all whitespace chars with spaces
			return strings.Map(func(r rune) rune {
				if r == '\t' || r == '\n' || r == '\r' {
					return ' '
				}
				return r
			}, val)
		default:
			return val
		}
	}

	if minExclusive != nil {
		normalized := normalizeValue(*minExclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("minExclusive value %q is not valid for base type: %w", *minExclusive, err)
		}
	}
	if maxExclusive != nil {
		normalized := normalizeValue(*maxExclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("maxExclusive value %q is not valid for base type: %w", *maxExclusive, err)
		}
	}
	if minInclusive != nil {
		normalized := normalizeValue(*minInclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("minInclusive value %q is not valid for base type: %w", *minInclusive, err)
		}
	}
	if maxInclusive != nil {
		normalized := normalizeValue(*maxInclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("maxInclusive value %q is not valid for base type: %w", *maxInclusive, err)
		}
	}

	return nil
}

func joinFields(value string) string {
	var b strings.Builder
	first := true
	for field := range types.FieldsXMLWhitespaceSeq(value) {
		if !first {
			b.WriteByte(' ')
		}
		first = false
		b.WriteString(field)
	}
	return b.String()
}

// findBuiltinAncestor walks up the type hierarchy to find the nearest built-in type
func findBuiltinAncestor(t types.Type) *types.BuiltinType {
	visited := make(map[types.Type]bool)
	current := t
	for current != nil && !visited[current] {
		visited[current] = true

		switch ct := current.(type) {
		case *types.BuiltinType:
			return ct
		case *types.SimpleType:
			if ct.IsBuiltin() || ct.QName.Namespace == types.XSDNamespace {
				if bt := types.GetBuiltinNS(ct.QName.Namespace, ct.QName.Local); bt != nil {
					return bt
				}
			}
		}

		current = current.BaseType()
	}
	return nil
}

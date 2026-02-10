package model

import "fmt"

// ValidateFacetApplicability checks if a facet is applicable to the base type.
// It returns an error that mirrors schema validation messages for inapplicable facets.
func ValidateFacetApplicability(facetName string, baseType Type, baseQName QName) error {
	baseTypeName := baseTypeNameForApplicability(baseType, baseQName)

	if baseType != nil {
		if baseST, ok := baseType.(*SimpleType); ok && baseST.Variety() == UnionVariety {
			switch facetName {
			case "pattern", "enumeration":
			default:
				return fmt.Errorf("facet %s is not applicable to union type %s", facetName, baseTypeName)
			}
		}
	}

	if isRangeFacetName(facetName) {
		if isListType(baseType, baseTypeName) {
			return fmt.Errorf("facet %s is not applicable to list type %s", facetName, baseTypeName)
		}
		facets := fundamentalFacetsFor(baseType, baseQName)
		if facets == nil || (facets.Ordered != OrderedTotal && facets.Ordered != OrderedPartial) {
			return fmt.Errorf("facet %s is only applicable to ordered types, but base type %s is not ordered", facetName, baseTypeName)
		}
	}

	if isDigitFacetName(facetName) {
		facets := fundamentalFacetsFor(baseType, baseQName)
		if facets == nil || !facets.Numeric {
			return fmt.Errorf("facet %s is only applicable to numeric types, but base type %s is not numeric", facetName, baseTypeName)
		}
	}

	if isLengthFacetName(facetName) {
		if isListType(baseType, baseTypeName) {
			return nil
		}
		primitiveName := primitiveTypeName(baseType, baseQName)
		switch {
		case primitiveName == "boolean":
			return fmt.Errorf("facet %s is not applicable to boolean type", facetName)
		case primitiveName == "duration":
			return fmt.Errorf("facet %s is not applicable to duration type", facetName)
		case IsNumericTypeName(primitiveName):
			return fmt.Errorf("facet %s is not applicable to numeric type %s", facetName, baseTypeName)
		case isDateTimeTypeName(primitiveName):
			return fmt.Errorf("facet %s is not applicable to date/time type %s", facetName, baseTypeName)
		}
	}

	return nil
}

func baseTypeNameForApplicability(baseType Type, baseQName QName) string {
	if baseType != nil {
		return baseType.Name().Local
	}
	return baseQName.Local
}

func fundamentalFacetsFor(baseType Type, baseQName QName) *FundamentalFacets {
	if baseType != nil {
		if baseType.IsBuiltin() {
			return baseType.FundamentalFacets()
		}
		if primitive := baseType.PrimitiveType(); primitive != nil {
			return primitive.FundamentalFacets()
		}
	}
	if baseQName.Namespace == XSDNamespace && baseQName.Local != "" {
		if bt := GetBuiltin(TypeName(baseQName.Local)); bt != nil {
			return bt.FundamentalFacets()
		}
	}
	return nil
}

func primitiveTypeName(baseType Type, baseQName QName) string {
	if baseType != nil {
		if primitive := baseType.PrimitiveType(); primitive != nil {
			return primitive.Name().Local
		}
		return baseType.Name().Local
	}
	return baseQName.Local
}

func isListType(baseType Type, baseTypeName string) bool {
	if baseTypeName != "" && isBuiltinListTypeName(baseTypeName) {
		return true
	}
	if baseType == nil {
		return false
	}
	if baseST, ok := baseType.(*SimpleType); ok {
		return baseST.Variety() == ListVariety
	}
	return false
}

func isRangeFacetName(name string) bool {
	switch name {
	case "minExclusive", "maxExclusive", "minInclusive", "maxInclusive":
		return true
	default:
		return false
	}
}

func isDigitFacetName(name string) bool {
	switch name {
	case "totalDigits", "fractionDigits":
		return true
	default:
		return false
	}
}

func isLengthFacetName(name string) bool {
	switch name {
	case "length", "minLength", "maxLength":
		return true
	default:
		return false
	}
}

func isDateTimeTypeName(typeName string) bool {
	switch typeName {
	case "dateTime", "date", "time", "gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth":
		return true
	default:
		return false
	}
}

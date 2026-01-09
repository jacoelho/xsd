package validation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/types"
)

func validateFacetConstraints(facetList []types.Facet, baseType types.Type, baseQName types.QName) error {
	var minExclusive, maxExclusive, minInclusive, maxInclusive *string
	var length, minLength, maxLength *int
	var totalDigits, fractionDigits *int
	var hasEnumeration bool

	baseTypeName := baseQName.Local
	isBuiltin := baseQName.Namespace == types.XSDNamespace
	var bt *types.BuiltinType
	if isBuiltin {
		bt = types.GetBuiltin(types.TypeName(baseTypeName))
	}

	// valid XSD 1.0 facets
	validFacets := map[string]bool{
		"length":         true,
		"minLength":      true,
		"maxLength":      true,
		"pattern":        true,
		"enumeration":    true,
		"whiteSpace":     true,
		"maxInclusive":   true,
		"maxExclusive":   true,
		"minInclusive":   true,
		"minExclusive":   true,
		"totalDigits":    true,
		"fractionDigits": true,
	}

	for _, facet := range facetList {
		name := facet.Name()

		// validate that the facet is a known XSD facet
		if !validFacets[name] {
			return fmt.Errorf("unknown or invalid facet '%s' (not a valid XSD 1.0 facet)", name)
		}

		switch name {
		case "minExclusive", "maxExclusive", "minInclusive", "maxInclusive":
			// all range facets are generic and implement LexicalFacet
			if lf, ok := facet.(types.LexicalFacet); ok {
				val := lf.GetLexical()
				if val != "" {
					switch name {
					case "minExclusive":
						minExclusive = &val
					case "maxExclusive":
						maxExclusive = &val
					case "minInclusive":
						minInclusive = &val
					case "maxInclusive":
						maxInclusive = &val
					}
				}
			}

		case "length":
			if ivf, ok := facet.(types.IntValueFacet); ok {
				val := ivf.GetIntValue()
				length = &val
			}

		case "minLength":
			if ivf, ok := facet.(types.IntValueFacet); ok {
				val := ivf.GetIntValue()
				minLength = &val
			}

		case "maxLength":
			if ivf, ok := facet.(types.IntValueFacet); ok {
				val := ivf.GetIntValue()
				maxLength = &val
			}

		case "enumeration":
			hasEnumeration = true

		case "totalDigits":
			if ivf, ok := facet.(types.IntValueFacet); ok {
				val := ivf.GetIntValue()
				totalDigits = &val
			}

		case "fractionDigits":
			if ivf, ok := facet.(types.IntValueFacet); ok {
				val := ivf.GetIntValue()
				fractionDigits = &val
			}

		case "pattern":
			if patternFacet, ok := facet.(interface{ ValidateSyntax() error }); ok {
				if err := patternFacet.ValidateSyntax(); err != nil {
					return fmt.Errorf("pattern facet: %w", err)
				}
			}
		}

		if err := validateFacetApplicability(name, baseTypeName, bt, isBuiltin, baseType); err != nil {
			return err
		}
	}

	if length != nil && (minLength != nil || maxLength != nil) {
		if !isListTypeForFacets(baseType, baseQName) {
			return fmt.Errorf("length facet cannot be used together with minLength or maxLength")
		}
		if maxLength != nil {
			return fmt.Errorf("length facet cannot be used together with maxLength for list types")
		}
	}

	if minLength != nil && maxLength != nil {
		if *minLength > *maxLength {
			return fmt.Errorf("minLength (%d) must be <= maxLength (%d)", *minLength, *maxLength)
		}
	}

	// built-in list types require at least one item.
	if isBuiltinListTypeName(baseTypeName) {
		if length != nil && *length < 1 {
			return fmt.Errorf("length (%d) must be >= 1 for list type %s", *length, baseTypeName)
		}
		if minLength != nil && *minLength < 1 {
			return fmt.Errorf("minLength (%d) must be >= 1 for list type %s", *minLength, baseTypeName)
		}
		if maxLength != nil && *maxLength < 1 {
			return fmt.Errorf("maxLength (%d) must be >= 1 for list type %s", *maxLength, baseTypeName)
		}
	}

	if err := validateRangeFacets(minExclusive, maxExclusive, minInclusive, maxInclusive, baseTypeName, bt); err != nil {
		return err
	}

	// validate that range facet values are within the base type's value space
	if err := validateRangeFacetValues(minExclusive, maxExclusive, minInclusive, maxInclusive, baseType, bt); err != nil {
		return err
	}

	// per XSD spec: fractionDigits must be <= totalDigits
	if totalDigits != nil && fractionDigits != nil {
		if *fractionDigits > *totalDigits {
			return fmt.Errorf("fractionDigits (%d) must be <= totalDigits (%d)", *fractionDigits, *totalDigits)
		}
	}

	// per XSD spec: fractionDigits must be 0 for integer-derived types
	// integer types are derived from decimal with fractionDigits=0 fixed
	if fractionDigits != nil && *fractionDigits != 0 {
		if isBuiltin && isIntegerTypeName(baseTypeName) {
			return fmt.Errorf("fractionDigits must be 0 for integer type %s, got %d", baseTypeName, *fractionDigits)
		}
		// also check user-defined types derived from integer types
		if baseType != nil {
			effectiveTypeName := getEffectiveIntegerTypeName(baseType)
			if effectiveTypeName != "" {
				return fmt.Errorf("fractionDigits must be 0 for type derived from %s, got %d", effectiveTypeName, *fractionDigits)
			}
		}
	}

	// validate enumeration values if base type is known
	if hasEnumeration && baseType != nil {
		if err := validateEnumerationValues(facetList, baseType); err != nil {
			return err
		}
	}

	return nil
}

func isListTypeForFacets(baseType types.Type, baseQName types.QName) bool {
	if st, ok := baseType.(*types.SimpleType); ok {
		if st.Variety() == types.ListVariety {
			return true
		}
	}
	if baseQName.Namespace == types.XSDNamespace && isBuiltinListTypeName(baseQName.Local) {
		return true
	}
	return false
}

// validateFacetApplicability checks if a facet is applicable to the base type
func validateFacetApplicability(facetName, baseTypeName string, bt *types.BuiltinType, isBuiltin bool, baseType types.Type) error {
	if baseType != nil {
		if baseST, ok := baseType.(*types.SimpleType); ok {
			if baseST.Variety() == types.UnionVariety {
				switch facetName {
				case "pattern", "enumeration":
					// allowed on union types
				default:
					return fmt.Errorf("facet %s is not applicable to union type %s", facetName, baseTypeName)
				}
			}
		}
	}

	// range facets (min/max) are applicable to ordered types (OrderedTotal or OrderedPartial)
	// per XSD 1.0 spec: range facets apply to types with ordered != none
	// range facets are NOT applicable to list types (lists don't have ordered value space)
	rangeFacets := []string{"minExclusive", "maxExclusive", "minInclusive", "maxInclusive"}
	for _, rf := range rangeFacets {
		if facetName == rf {
			// check if the base type is a list type - range facets are NOT applicable to list types
			if baseType != nil {
				if baseST, ok := baseType.(*types.SimpleType); ok {
					if baseST.Variety() == types.ListVariety {
						return fmt.Errorf("facet %s is not applicable to list type %s", facetName, baseTypeName)
					}
				}
			}

			var facets *types.FundamentalFacets

			if isBuiltin && bt != nil {
				// built-in type: use its fundamental facets directly
				facets = bt.FundamentalFacets()
			} else if baseType != nil {
				// user-defined type: get fundamental facets from primitive type
				primitive := baseType.PrimitiveType()
				if primitive != nil {
					facets = primitive.FundamentalFacets()
				}
			}

			if facets == nil || (facets.Ordered != types.OrderedTotal && facets.Ordered != types.OrderedPartial) {
				return fmt.Errorf("facet %s is only applicable to ordered types, but base type %s is not ordered", facetName, baseTypeName)
			}
		}
	}

	// totalDigits and fractionDigits are only applicable to numeric types
	digitFacets := []string{"totalDigits", "fractionDigits"}
	for _, df := range digitFacets {
		if facetName == df {
			if isBuiltin && bt != nil {
				if !isNumericTypeName(baseTypeName) {
					return fmt.Errorf("facet %s is only applicable to numeric types, but base type %s is not numeric", facetName, baseTypeName)
				}
			}
		}
	}

	// length facets (length, minLength, maxLength) don't apply to certain types
	// according to XSD spec: length doesn't apply to boolean, numeric types, or date/time types
	// length applies to: string types, list types, and binary types (hexBinary, base64Binary)
	// important: For list types, length facets ARE applicable (they count list items, not string length)
	lengthFacets := []string{"length", "minLength", "maxLength"}
	for _, lf := range lengthFacets {
		if facetName == lf {
			// first check if the base type is a list type - length facets ARE applicable to list types
			if baseType != nil {
				if baseST, ok := baseType.(*types.SimpleType); ok {
					if baseST.Variety() == types.ListVariety {
						// list types support length facets (they count list items)
						continue
					}
				}
			}

			if isBuiltin && bt != nil {
				// check if type doesn't support length facets
				if baseTypeName == "boolean" {
					return fmt.Errorf("facet %s is not applicable to boolean type", facetName)
				}
				// check if it's a numeric type (doesn't support length)
				if isNumericTypeName(baseTypeName) {
					return fmt.Errorf("facet %s is not applicable to numeric type %s", facetName, baseTypeName)
				}
				// check if it's a date/time type (doesn't support length)
				if isDateTimeTypeName(baseTypeName) {
					return fmt.Errorf("facet %s is not applicable to date/time type %s", facetName, baseTypeName)
				}
			} else if baseType != nil {
				// for user-defined types, check the primitive type
				primitive := baseType.PrimitiveType()
				if primitive != nil {
					primitiveName := primitive.Name().Local
					if primitiveName == "boolean" {
						return fmt.Errorf("facet %s is not applicable to boolean type", facetName)
					}
					if isNumericTypeName(primitiveName) {
						return fmt.Errorf("facet %s is not applicable to numeric type %s", facetName, baseTypeName)
					}
					if isDateTimeTypeName(primitiveName) {
						return fmt.Errorf("facet %s is not applicable to date/time type %s", facetName, baseTypeName)
					}
				}
			}
		}
	}

	return nil
}

// isNumericTypeName checks if a type name represents a numeric type
func isNumericTypeName(typeName string) bool {
	numericTypes := []string{
		"decimal", "float", "double", "integer", "long", "int", "short", "byte",
		"nonNegativeInteger", "positiveInteger", "unsignedLong", "unsignedInt",
		"unsignedShort", "unsignedByte", "nonPositiveInteger", "negativeInteger",
	}
	return slices.Contains(numericTypes, typeName)
}

// isIntegerTypeName checks if a type name represents an integer-derived type
// Integer types are derived from decimal with fractionDigits=0 fixed
func isIntegerTypeName(typeName string) bool {
	integerTypes := []string{
		"integer", "long", "int", "short", "byte",
		"nonNegativeInteger", "positiveInteger", "unsignedLong", "unsignedInt",
		"unsignedShort", "unsignedByte", "nonPositiveInteger", "negativeInteger",
	}
	return slices.Contains(integerTypes, typeName)
}

// getEffectiveIntegerTypeName returns the name of the integer type if the given type
// is derived from an integer type (including user-defined types). Returns empty string
// if not derived from integer.
func getEffectiveIntegerTypeName(t types.Type) string {
	// walk up the type hierarchy to find if it's derived from an integer type
	visited := make(map[types.Type]bool)
	current := t
	for current != nil && !visited[current] {
		visited[current] = true

		var typeName string
		switch ct := current.(type) {
		case *types.BuiltinType:
			typeName = ct.Name().Local
		case *types.SimpleType:
			if ct.IsBuiltin() || ct.QName.Namespace == types.XSDNamespace {
				typeName = ct.QName.Local
			}
		}

		if typeName != "" && isIntegerTypeName(typeName) {
			return typeName
		}

		current = current.BaseType()
	}
	return ""
}

// isDateTimeTypeName checks if a type name represents a date/time type
func isDateTimeTypeName(typeName string) bool {
	dateTimeTypes := []string{
		"dateTime", "date", "time", "gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth",
	}
	return slices.Contains(dateTimeTypes, typeName)
}

func isBuiltinListTypeName(name string) bool {
	return name == "NMTOKENS" || name == "IDREFS" || name == "ENTITIES"
}

func validateListItemValue(itemType types.Type, value string) error {
	if bt, ok := itemType.(*types.BuiltinType); ok {
		return bt.Validate(value)
	}
	if st, ok := itemType.(*types.SimpleType); ok {
		return st.Validate(value)
	}
	return fmt.Errorf("cannot validate list item against non-simple type %T", itemType)
}

// validateEnumerationValues validates that enumeration values are valid for the base type
func validateEnumerationValues(facetList []types.Facet, baseType types.Type) error {
	var enumValues []string
	for _, f := range facetList {
		if f.Name() != "enumeration" {
			continue
		}
		if enum, ok := f.(*types.Enumeration); ok {
			enumValues = enum.Values
			break
		}
	}

	if len(enumValues) == 0 {
		return nil // no enumeration to validate
	}

	// for list types, enumeration values are space-separated lists of item values
	if st, ok := baseType.(*types.SimpleType); ok {
		if st.Variety() == types.ListVariety {
			itemType := st.ItemType
			if itemType == nil && st.List != nil && st.List.InlineItemType != nil {
				itemType = st.List.InlineItemType
			}
			if itemType == nil {
				return nil
			}
			for _, enumVal := range enumValues {
				found := false
				for item := range strings.FieldsSeq(enumVal) {
					found = true
					if err := validateListItemValue(itemType, item); err != nil {
						return fmt.Errorf("enumeration value %q contains invalid list item %q: %w", enumVal, item, err)
					}
				}
				if !found {
					return fmt.Errorf("enumeration value %q must contain at least one list item", enumVal)
				}
			}
			return nil
		}
	}

	// note: empty string is a valid enumeration value per XSD spec for string-based types

	// try to get the built-in type validator
	var bt *types.BuiltinType
	var typeName string

	switch t := baseType.(type) {
	case *types.BuiltinType:
		bt = t
		typeName = t.Name().Local
		if isBuiltinListTypeName(typeName) {
			itemType := t.BaseType()
			if itemType == nil {
				return nil
			}
			for _, enumVal := range enumValues {
				found := false
				for item := range strings.FieldsSeq(enumVal) {
					found = true
					if err := validateListItemValue(itemType, item); err != nil {
						return fmt.Errorf("enumeration value %q contains invalid list item %q: %w", enumVal, item, err)
					}
				}
				if !found {
					return fmt.Errorf("enumeration value %q must contain at least one list item", enumVal)
				}
			}
			return nil
		}
	case *types.SimpleType:
		if t.IsBuiltin() {
			bt = types.GetBuiltinNS(t.QName.Namespace, t.QName.Local)
			typeName = t.QName.Local
		} else {
			// for user-defined types, try to get the primitive type for validation
			primitive := t.PrimitiveType()
			if primitive != nil {
				if pbt, ok := primitive.(*types.BuiltinType); ok {
					bt = pbt
					typeName = pbt.Name().Local
				}
			}
		}
	}

	// if we found a built-in type, validate enumeration values against it
	if bt != nil {
		for i, val := range enumValues {
			if err := bt.Validate(val); err != nil {
				return fmt.Errorf("enumeration value %d (%q) is not valid for base type %s: %w", i+1, val, typeName, err)
			}
		}
	}

	return nil
}

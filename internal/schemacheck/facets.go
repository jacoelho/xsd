package schemacheck

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

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

var errDateTimeNotComparable = errors.New("date/time values are not comparable")
var errDurationNotComparable = errors.New("duration values are not comparable")

// validateRangeFacets validates consistency of range facets
func validateRangeFacets(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseTypeName string, bt *types.BuiltinType) error {
	if baseTypeName == "duration" {
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

	// minExclusive > maxInclusive is invalid
	if minExclusive != nil && maxInclusive != nil {
		if err := compareRangeValues(baseTypeName, bt); err == nil {
			// values are comparable, check if minExclusive >= maxInclusive
			if compareNumericOrString(*minExclusive, *maxInclusive, baseTypeName, bt) >= 0 {
				return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
			}
		}
	}

	// minExclusive >= maxExclusive is invalid
	if minExclusive != nil && maxExclusive != nil {
		err := compareRangeValues(baseTypeName, bt)
		if err == nil {
			if compareNumericOrString(*minExclusive, *maxExclusive, baseTypeName, bt) >= 0 {
				return fmt.Errorf("minExclusive (%s) must be < maxExclusive (%s)", *minExclusive, *maxExclusive)
			}
		} else {
			// if compareRangeValues failed (e.g., bt is nil), try comparison anyway for known types
			if isDateTimeTypeName(baseTypeName) || isNumericTypeName(baseTypeName) {
				if compareNumericOrString(*minExclusive, *maxExclusive, baseTypeName, bt) >= 0 {
					return fmt.Errorf("minExclusive (%s) must be < maxExclusive (%s)", *minExclusive, *maxExclusive)
				}
			}
		}
	}

	// minInclusive > maxInclusive is invalid
	if minInclusive != nil && maxInclusive != nil {
		err := compareRangeValues(baseTypeName, bt)
		if err == nil {
			if compareNumericOrString(*minInclusive, *maxInclusive, baseTypeName, bt) > 0 {
				return fmt.Errorf("minInclusive (%s) must be <= maxInclusive (%s)", *minInclusive, *maxInclusive)
			}
		} else {
			// if compareRangeValues failed (e.g., bt is nil), try comparison anyway for known types
			if isDateTimeTypeName(baseTypeName) || isNumericTypeName(baseTypeName) {
				if compareNumericOrString(*minInclusive, *maxInclusive, baseTypeName, bt) > 0 {
					return fmt.Errorf("minInclusive (%s) must be <= maxInclusive (%s)", *minInclusive, *maxInclusive)
				}
			}
		}
	}

	// minInclusive >= maxExclusive is invalid
	if minInclusive != nil && maxExclusive != nil {
		if err := compareRangeValues(baseTypeName, bt); err == nil {
			if compareNumericOrString(*minInclusive, *maxExclusive, baseTypeName, bt) >= 0 {
				return fmt.Errorf("minInclusive (%s) must be < maxExclusive (%s)", *minInclusive, *maxExclusive)
			}
		}
	}

	// minExclusive >= maxInclusive is invalid (already checked above, but also check with inclusive)
	if minExclusive != nil && maxInclusive != nil {
		if err := compareRangeValues(baseTypeName, bt); err == nil {
			if compareNumericOrString(*minExclusive, *maxInclusive, baseTypeName, bt) >= 0 {
				return fmt.Errorf("minExclusive (%s) must be < maxInclusive (%s)", *minExclusive, *maxInclusive)
			}
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
			val = strings.TrimSpace(val)
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
	for field := range strings.FieldsSeq(value) {
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

// compareRangeValues returns an error if range values cannot be compared for the base type.
func compareRangeValues(baseTypeName string, bt *types.BuiltinType) error {
	if baseTypeName == "duration" {
		return nil
	}
	if bt == nil || !bt.Ordered() {
		return fmt.Errorf("cannot compare values for non-ordered type")
	}
	return nil
}

// compareNumericOrString compares two values, returning -1, 0, or 1
func compareNumericOrString(v1, v2, baseTypeName string, bt *types.BuiltinType) int {
	// if bt is nil, try to compare anyway if it's a known date/time or numeric type
	if bt == nil {
		if baseTypeName == "duration" {
			if cmp, err := compareDurationValues(v1, v2); err == nil {
				return cmp
			}
			return 0
		}
		// for date/time types, we can still compare using string comparison
		if isDateTimeTypeName(baseTypeName) {
			if cmp, err := compareDateTimeValues(v1, v2, baseTypeName); err == nil {
				return cmp
			}
			return 0
		}
		// for numeric types, try parsing
		if isNumericTypeName(baseTypeName) {
			val1, err1 := strconv.ParseFloat(v1, 64)
			val2, err2 := strconv.ParseFloat(v2, 64)
			if err1 == nil && err2 == nil {
				if val1 < val2 {
					return -1
				}
				if val1 > val2 {
					return 1
				}
				return 0
			}
		}
		return 0 // can't compare without type info
	}

	if !bt.Ordered() {
		return 0 // can't compare
	}

	// try numeric comparison first
	if isNumericTypeName(baseTypeName) {
		val1, err1 := strconv.ParseFloat(v1, 64)
		val2, err2 := strconv.ParseFloat(v2, 64)
		if err1 == nil && err2 == nil {
			if val1 < val2 {
				return -1
			}
			if val1 > val2 {
				return 1
			}
			return 0
		}
	}

	if baseTypeName == "duration" {
		if cmp, err := compareDurationValues(v1, v2); err == nil {
			return cmp
		}
	}

	// for date/time types, try to parse and compare as dates
	if isDateTimeTypeName(baseTypeName) {
		if result, err := compareDateTimeValues(v1, v2, baseTypeName); err == nil && result != 0 {
			return result
		}
	}

	// fall back to string comparison
	if v1 < v2 {
		return -1
	}
	if v1 > v2 {
		return 1
	}
	return 0
}

// compareDateTimeValues compares two date/time values, returning -1, 0, or 1
func compareDateTimeValues(v1, v2, baseTypeName string) (int, error) {
	switch baseTypeName {
	case "date":
		t1, tz1, err := parseXSDDate(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDDate(v2)
		if err != nil {
			return 0, err
		}
		if tz1 != tz2 {
			return 0, errDateTimeNotComparable
		}
		return compareTimes(t1, t2), nil
	case "dateTime":
		t1, tz1, err := parseXSDDateTime(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDDateTime(v2)
		if err != nil {
			return 0, err
		}
		if tz1 != tz2 {
			return 0, errDateTimeNotComparable
		}
		return compareTimes(t1, t2), nil
	case "time":
		t1, tz1, err := parseXSDTime(v1)
		if err != nil {
			return 0, err
		}
		t2, tz2, err := parseXSDTime(v2)
		if err != nil {
			return 0, err
		}
		if tz1 != tz2 {
			return 0, errDateTimeNotComparable
		}
		return compareTimes(t1, t2), nil
	}

	// fallback: lexicographic comparison for other date/time types.
	if v1 < v2 {
		return -1, nil
	}
	if v1 > v2 {
		return 1, nil
	}
	return 0, nil
}

func compareTimes(t1, t2 time.Time) int {
	if t1.Before(t2) {
		return -1
	}
	if t1.After(t2) {
		return 1
	}
	return 0
}

func splitTimezone(value string) (string, bool, int, error) {
	if before, ok := strings.CutSuffix(value, "Z"); ok {
		return before, true, 0, nil
	}
	if len(value) >= 6 {
		sep := value[len(value)-6]
		if (sep == '+' || sep == '-') && value[len(value)-3] == ':' {
			base := value[:len(value)-6]
			hours, err := strconv.Atoi(value[len(value)-5 : len(value)-3])
			if err != nil {
				return "", false, 0, fmt.Errorf("invalid timezone offset in %q", value)
			}
			mins, err := strconv.Atoi(value[len(value)-2:])
			if err != nil {
				return "", false, 0, fmt.Errorf("invalid timezone offset in %q", value)
			}
			offset := hours*3600 + mins*60
			if sep == '-' {
				offset = -offset
			}
			return base, true, offset, nil
		}
	}
	return value, false, 0, nil
}

func parseXSDDate(value string) (time.Time, bool, error) {
	base, hasTZ, offset, err := splitTimezone(value)
	if err != nil {
		return time.Time{}, false, err
	}
	t, err := time.Parse("2006-01-02", base)
	if err != nil {
		return time.Time{}, false, err
	}
	if hasTZ {
		loc := time.FixedZone("", offset)
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).UTC()
	}
	return t, hasTZ, nil
}

func parseXSDDateTime(value string) (time.Time, bool, error) {
	base, hasTZ, offset, err := splitTimezone(value)
	if err != nil {
		return time.Time{}, false, err
	}
	layouts := []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	var parsed time.Time
	var parseErr error
	for _, layout := range layouts {
		parsed, parseErr = time.Parse(layout, base)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return time.Time{}, false, parseErr
	}
	if hasTZ {
		loc := time.FixedZone("", offset)
		parsed = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), loc).UTC()
	}
	return parsed, hasTZ, nil
}

func parseXSDTime(value string) (time.Time, bool, error) {
	base, hasTZ, offset, err := splitTimezone(value)
	if err != nil {
		return time.Time{}, false, err
	}
	layouts := []string{
		"15:04:05.999999999",
		"15:04:05",
	}
	var parsed time.Time
	var parseErr error
	for _, layout := range layouts {
		parsed, parseErr = time.Parse(layout, base)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return time.Time{}, false, parseErr
	}
	loc := time.UTC
	if hasTZ {
		loc = time.FixedZone("", offset)
	}
	parsed = time.Date(2000, 1, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), loc).UTC()
	return parsed, hasTZ, nil
}

func compareDurationValues(v1, v2 string) (int, error) {
	months1, seconds1, err := parseDurationParts(v1)
	if err != nil {
		return 0, err
	}
	months2, seconds2, err := parseDurationParts(v2)
	if err != nil {
		return 0, err
	}

	if months1 == months2 && seconds1 == seconds2 {
		return 0, nil
	}
	if months1 <= months2 && seconds1 <= seconds2 {
		return -1, nil
	}
	if months1 >= months2 && seconds1 >= seconds2 {
		return 1, nil
	}
	return 0, errDurationNotComparable
}

func parseDurationParts(value string) (int, float64, error) {
	if value == "" {
		return 0, 0, fmt.Errorf("empty duration")
	}

	negative := value[0] == '-'
	if negative {
		value = value[1:]
	}
	if len(value) == 0 || value[0] != 'P' {
		return 0, 0, fmt.Errorf("duration must start with P")
	}
	value = value[1:]

	var years, months, days, hours, minutes int
	var seconds float64

	datePart := value
	timePart := ""
	if before, after, ok := strings.Cut(value, "T"); ok {
		datePart = before
		timePart = after
		if extra := strings.IndexByte(timePart, 'T'); extra != -1 {
			timePart = timePart[:extra]
		}
	}

	datePattern := regexp.MustCompile(`([0-9]+)Y|([0-9]+)M|([0-9]+)D`)
	matches := datePattern.FindAllStringSubmatch(datePart, -1)
	for _, match := range matches {
		if match[1] != "" {
			years, _ = strconv.Atoi(match[1])
		}
		if match[2] != "" {
			months, _ = strconv.Atoi(match[2])
		}
		if match[3] != "" {
			days, _ = strconv.Atoi(match[3])
		}
	}

	if timePart != "" {
		timePattern := regexp.MustCompile(`([0-9]+)H|([0-9]+)M|([0-9]+(?:\.[0-9]+)?)S`)
		matches = timePattern.FindAllStringSubmatch(timePart, -1)
		for _, match := range matches {
			if match[1] != "" {
				hours, _ = strconv.Atoi(match[1])
			}
			if match[2] != "" {
				minutes, _ = strconv.Atoi(match[2])
			}
			if match[3] != "" {
				seconds, _ = strconv.ParseFloat(match[3], 64)
			}
		}
	}

	totalMonths := years*12 + months
	totalSeconds := float64(days*24*60*60+hours*60*60+minutes*60) + seconds

	if negative {
		totalMonths = -totalMonths
		totalSeconds = -totalSeconds
	}

	return totalMonths, totalSeconds, nil
}

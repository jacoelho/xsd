package validator

import "github.com/jacoelho/xsd/internal/types"

func fixedValueMatches(actualValue, fixedValue string, typ types.Type) bool {
	normalizedValue := types.NormalizeWhiteSpace(actualValue, typ)
	normalizedFixed := types.NormalizeWhiteSpace(fixedValue, typ)
	return normalizedValue == normalizedFixed
}

// compareTypedValues compares two TypedValues for equality in the value space.
// Per XSD spec section 4.2.1, equality is determined by the value space, not lexical representation.
// For example, "1.0" and "1.00" are equal decimals, and "true" and "1" are equal booleans.
func compareTypedValues(left, right types.TypedValue) bool {
	return types.ValuesEqual(left, right)
}

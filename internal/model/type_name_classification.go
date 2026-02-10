package model

// IsIntegerTypeName reports whether typeName is an XSD built-in integer-derived type.
func IsIntegerTypeName(typeName string) bool {
	switch typeName {
	case "integer", "long", "int", "short", "byte",
		"nonNegativeInteger", "positiveInteger", "unsignedLong", "unsignedInt",
		"unsignedShort", "unsignedByte", "nonPositiveInteger", "negativeInteger":
		return true
	default:
		return false
	}
}

// IsNumericTypeName reports whether typeName is an XSD built-in numeric type.
func IsNumericTypeName(typeName string) bool {
	switch typeName {
	case "decimal", "float", "double":
		return true
	default:
		return IsIntegerTypeName(typeName)
	}
}

// IsDateTimeTypeName reports whether typeName is an XSD built-in date/time type.
func IsDateTimeTypeName(typeName string) bool {
	switch typeName {
	case "dateTime", "date", "time", "gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth":
		return true
	default:
		return false
	}
}

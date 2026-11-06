package value

// Aliases expose unexported identifiers to the external value_test package.

const (
	BuiltinNCName       = builtinNCNameLexical
	OrderedFacetEqual   = orderedFacetEqual
	OrderedFacetLess    = orderedFacetLess
	PrimitiveDate       = primitiveDate
	PrimitiveGDay       = primitiveGDay
	PrimitiveGMonth     = primitiveGMonth
	PrimitiveGMonthDay  = primitiveGMonthDay
	PrimitiveGYear      = primitiveGYear
	PrimitiveGYearMonth = primitiveGYearMonth
	PrimitiveTime       = primitiveTime
)

type (
	OrderedFacetRelation = orderedFacetRelation
)

var (
	CompareTemporalValues = compareTemporalValues
	CompareTimePartial    = compareTimePartial
)

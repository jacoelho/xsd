package types

// FundamentalFacets represents the fundamental facets of a simple type
type FundamentalFacets struct {
	Ordered     Ordered
	Bounded     bool
	Cardinality Cardinality
	Numeric     bool
}

// ComputeFundamentalFacets computes fundamental facets for a primitive type
func ComputeFundamentalFacets(typeName TypeName) *FundamentalFacets {
	switch typeName {
	// Numeric types (ordered=total, numeric=true)
	case TypeNameDecimal, TypeNameFloat, TypeNameDouble:
		return &FundamentalFacets{
			Ordered:     OrderedTotal,
			Bounded:     false,
			Cardinality: CardinalityUncountablyInfinite,
			Numeric:     true,
		}
	// Date/time types (ordered=total, numeric=false)
	case TypeNameDateTime, TypeNameTime, TypeNameDate, TypeNameGYearMonth, TypeNameGYear, TypeNameGMonthDay, TypeNameGDay, TypeNameGMonth:
		return &FundamentalFacets{
			Ordered:     OrderedTotal,
			Bounded:     false,
			Cardinality: CardinalityUncountablyInfinite,
			Numeric:     false,
		}
	// Duration (ordered=partial)
	case TypeNameDuration:
		return &FundamentalFacets{
			Ordered:     OrderedPartial,
			Bounded:     false,
			Cardinality: CardinalityUncountablyInfinite,
			Numeric:     false,
		}
	// String types (ordered=none, countably infinite)
	case TypeNameString, TypeNameHexBinary, TypeNameBase64Binary, TypeNameAnyURI, TypeNameQName, TypeNameNOTATION:
		return &FundamentalFacets{
			Ordered:     OrderedNone,
			Bounded:     false,
			Cardinality: CardinalityCountablyInfinite,
			Numeric:     false,
		}
	// Boolean (ordered=none, finite)
	case TypeNameBoolean:
		return &FundamentalFacets{
			Ordered:     OrderedNone,
			Bounded:     false,
			Cardinality: CardinalityFinite,
			Numeric:     false,
		}
	default:
		return nil
	}
}

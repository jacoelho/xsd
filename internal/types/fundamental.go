package types

// FundamentalFacets represents the fundamental facets of a simple type
type FundamentalFacets struct {
	Ordered     Ordered
	Cardinality Cardinality
	Bounded     bool
	Numeric     bool
}

// ComputeFundamentalFacets computes fundamental facets for a primitive type
func ComputeFundamentalFacets(typeName TypeName) *FundamentalFacets {
	switch typeName {
	// numeric types (ordered=total, numeric=true)
	case TypeNameDecimal, TypeNameFloat, TypeNameDouble:
		return &FundamentalFacets{
			Ordered:     OrderedTotal,
			Bounded:     false,
			Cardinality: CardinalityUncountablyInfinite,
			Numeric:     true,
		}
	// date/time types (ordered=total, numeric=false)
	case TypeNameDateTime, TypeNameTime, TypeNameDate, TypeNameGYearMonth, TypeNameGYear, TypeNameGMonthDay, TypeNameGDay, TypeNameGMonth:
		return &FundamentalFacets{
			Ordered:     OrderedTotal,
			Bounded:     false,
			Cardinality: CardinalityUncountablyInfinite,
			Numeric:     false,
		}
	// duration (ordered=partial)
	case TypeNameDuration:
		return &FundamentalFacets{
			Ordered:     OrderedPartial,
			Bounded:     false,
			Cardinality: CardinalityUncountablyInfinite,
			Numeric:     false,
		}
	// string types (ordered=none, countably infinite)
	case TypeNameString, TypeNameHexBinary, TypeNameBase64Binary, TypeNameAnyURI, TypeNameQName, TypeNameNOTATION:
		return &FundamentalFacets{
			Ordered:     OrderedNone,
			Bounded:     false,
			Cardinality: CardinalityCountablyInfinite,
			Numeric:     false,
		}
	// boolean (ordered=none, finite)
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

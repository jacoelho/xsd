package types

// Type represents an XSD type (simple or complex)
type Type interface {
	Name() QName
	IsBuiltin() bool
	BaseType() Type
	PrimitiveType() Type
	FundamentalFacets() *FundamentalFacets
	WhiteSpace() WhiteSpace
}

// LengthMeasurable types know how to measure their length for facet validation.
type LengthMeasurable interface {
	Type

	// MeasureLength returns length in type-appropriate units (octets, items, or characters).
	MeasureLength(value string) int
}

// Ordered represents the ordered facet value
type Ordered int

const (
	// OrderedNone indicates no ordering.
	OrderedNone Ordered = iota
	// OrderedPartial indicates partial ordering.
	OrderedPartial
	// OrderedTotal indicates total ordering.
	OrderedTotal
)

// String returns the string form of the ordered facet.
func (o Ordered) String() string {
	switch o {
	case OrderedTotal:
		return "total"
	case OrderedPartial:
		return "partial"
	case OrderedNone:
		return "none"
	default:
		return "unknown"
	}
}

// Cardinality represents the cardinality facet value.
type Cardinality int

const (
	// CardinalityFinite indicates a finite value space.
	CardinalityFinite Cardinality = iota
	// CardinalityCountablyInfinite indicates a countably infinite value space.
	CardinalityCountablyInfinite
	// CardinalityUncountablyInfinite indicates an uncountably infinite value space.
	CardinalityUncountablyInfinite
)

// SimpleTypeVariety represents the variety of a simple type.
type SimpleTypeVariety int

const (
	// AtomicVariety indicates an atomic simple type (derived by restriction).
	AtomicVariety SimpleTypeVariety = iota
	// ListVariety indicates a list simple type (derived by list).
	ListVariety
	// UnionVariety indicates a union simple type (derived by union).
	UnionVariety
)

// ListType represents a list type
type ListType struct {
	// From itemType attribute (QName to resolve)
	ItemType QName
	// From inline <simpleType> child (already parsed)
	InlineItemType *SimpleType
}

// UnionType represents a union type
type UnionType struct {
	// From memberTypes attribute (QNames to resolve)
	MemberTypes []QName
	// From inline <simpleType> children (already parsed)
	InlineTypes []*SimpleType
}

// IsFacetApplicable determines if a facet is applicable to a type based on its fundamental facets
func IsFacetApplicable(facetName string, facets *FundamentalFacets) bool {
	if facets == nil {
		return false
	}

	switch facetName {
	case "minInclusive", "maxInclusive", "minExclusive", "maxExclusive":
		// range facets require ordered types.
		return facets.Ordered == OrderedTotal || facets.Ordered == OrderedPartial

	case "length", "minLength", "maxLength", "pattern", "enumeration", "whiteSpace":
		// length and lexical facets apply to all types.
		return true

	case "fractionDigits", "totalDigits":
		// digit facets apply to numeric types.
		return facets.Numeric
	}

	// default: assume applicable
	return true
}

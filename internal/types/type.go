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

// HasBaseType exposes a base type relationship.
type HasBaseType interface {
	BaseType() Type
}

// FacetCarrier exposes fundamental facets and whitespace handling.
type FacetCarrier interface {
	FundamentalFacets() *FundamentalFacets
	WhiteSpace() WhiteSpace
}

// ValueSpace exposes lexical validation and parsing behavior.
type ValueSpace interface {
	// Validate checks if a lexical value is valid for this type.
	Validate(lexical string) error

	// ParseValue converts a lexical value to a TypedValue.
	ParseValue(lexical string) (TypedValue, error)
}

// SimpleTypeDefinition extends Type with validation capabilities.
type SimpleTypeDefinition interface {
	Type
	ValueSpace

	// Variety returns the simple type variety (atomic, list, union).
	Variety() SimpleTypeVariety
}

// ComplexTypeDefinition extends Type with content model information.
type ComplexTypeDefinition interface {
	Type

	// Content returns the content model (empty, simple, element-only, mixed).
	Content() Content

	// Attributes returns the attribute declarations.
	Attributes() []*AttributeDecl

	// AnyAttribute returns the wildcard attribute if present.
	AnyAttribute() *AnyAttribute

	// Mixed returns true if this type allows mixed content.
	Mixed() bool
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
	ItemType       QName       // From itemType attribute (QName to resolve)
	InlineItemType *SimpleType // From inline <simpleType> child (already parsed)
}

// UnionType represents a union type
type UnionType struct {
	MemberTypes []QName       // From memberTypes attribute (QNames to resolve)
	InlineTypes []*SimpleType // From inline <simpleType> children (already parsed)
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
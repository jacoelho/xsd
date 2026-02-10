package model

// Type represents an XSD type (simple or complex)
type Type interface {
	Name() QName
	IsBuiltin() bool
	BaseType() Type
	PrimitiveType() Type
	FundamentalFacets() *FundamentalFacets
	WhiteSpace() WhiteSpace
}

func as[T any](value any) (T, bool) {
	v, ok := value.(T)
	return v, ok
}

// AsSimpleType performs a type assertion to *SimpleType.
func AsSimpleType(t Type) (*SimpleType, bool) {
	return as[*SimpleType](t)
}

// AsComplexType performs a type assertion to *ComplexType.
func AsComplexType(t Type) (*ComplexType, bool) {
	return as[*ComplexType](t)
}

// AsBuiltinType performs a type assertion to *BuiltinType.
func AsBuiltinType(t Type) (*BuiltinType, bool) {
	return as[*BuiltinType](t)
}

// AsDerivedType performs a type assertion to DerivedType.
func AsDerivedType(t Type) (DerivedType, bool) {
	return as[DerivedType](t)
}

// LengthMeasurable types know how to measure their length for facet schemacheck.
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

func (v SimpleTypeVariety) String() string {
	switch v {
	case AtomicVariety:
		return "atomic"
	case ListVariety:
		return "list"
	case UnionVariety:
		return "union"
	default:
		return "unknown"
	}
}

// ListType represents a list type
type ListType struct {
	InlineItemType *SimpleType
	ItemType       QName
}

// UnionType represents a union type
type UnionType struct {
	// From memberTypes attribute (QNames to resolve)
	MemberTypes []QName
	// From inline <simpleType> children (already parsed)
	InlineTypes []*SimpleType
}

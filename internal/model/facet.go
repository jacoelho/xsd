package model

import (
	"errors"
	"fmt"
)

var (
	_ Facet = (*Pattern)(nil)
	_ Facet = (*PatternSet)(nil)
	_ Facet = (*Enumeration)(nil)
	_ Facet = (*Length)(nil)
	_ Facet = (*MinLength)(nil)
	_ Facet = (*MaxLength)(nil)
	_ Facet = (*TotalDigits)(nil)
	_ Facet = (*FractionDigits)(nil)
	_ Facet = (*RangeFacet)(nil)
)

// StringTypedValue is a simple TypedValue wrapper for string values
// Used when parsing to native type fails but we still need to validate facets
type StringTypedValue struct {
	Typ   Type
	Value string
}

// DeferredFacet stores raw facet data when the base type is not available during parsing.
// These facets are validated during schema validation when the base type is resolved.
type DeferredFacet struct {
	FacetName  string
	FacetValue string
}

// Type returns the XSD type used for facet checks.
func (s *StringTypedValue) Type() Type { return s.Typ }

// Lexical returns the raw lexical value used when parsing fails.
func (s *StringTypedValue) Lexical() string { return s.Value }

// Native returns the lexical value as its native representation.
func (s *StringTypedValue) Native() any { return s.Value }

// String returns the lexical value for error messages.
func (s *StringTypedValue) String() string { return s.Value }

// typedValueForFacet creates a TypedValue for facet schemacheck.
func typedValueForFacet(value string, typ Type) TypedValue {
	switch t := typ.(type) {
	case *SimpleType:
		if parsed, err := t.parseValueInternal(value, false); err == nil {
			return parsed
		}
	case *BuiltinType:
		if parsed, err := t.ParseValue(value); err == nil {
			return parsed
		}
	}
	return &StringTypedValue{Value: value, Typ: typ}
}

// Facet is the unified interface for all constraining facets
type Facet interface {
	Name() string
	Validate(value TypedValue, baseType Type) error
}

// LexicalFacet is a facet that has a lexical string value.
// Examples include pattern and enumeration facets.
type LexicalFacet interface {
	Facet
	GetLexical() string
}

// LexicalValidator validates a lexical value without a TypedValue.
type LexicalValidator interface {
	Facet
	ValidateLexical(lexical string, baseType Type) error
}

// IntValueFacet is a facet that has an integer value.
// Examples include length, minLength, maxLength, totalDigits, and fractionDigits facets.
type IntValueFacet interface {
	Facet
	GetIntValue() int
}

// ErrCannotDeterminePrimitiveType is returned when the primitive type cannot be
// determined during parsing; schema validation handles this later.
var ErrCannotDeterminePrimitiveType = errors.New("cannot determine primitive type")

// NewMinInclusive creates a minInclusive facet based on the base type.
func NewMinInclusive(lexical string, baseType Type) (Facet, error) {
	return newRangeBoundFacet("minInclusive", lexical, baseType, func(cmp int) bool { return cmp >= 0 }, ">=")
}

// NewMaxInclusive creates a maxInclusive facet based on the base type.
func NewMaxInclusive(lexical string, baseType Type) (Facet, error) {
	return newRangeBoundFacet("maxInclusive", lexical, baseType, func(cmp int) bool { return cmp <= 0 }, "<=")
}

// NewMinExclusive creates a minExclusive facet based on the base type.
func NewMinExclusive(lexical string, baseType Type) (Facet, error) {
	return newRangeBoundFacet("minExclusive", lexical, baseType, func(cmp int) bool { return cmp > 0 }, ">")
}

// NewMaxExclusive creates a maxExclusive facet based on the base type.
func NewMaxExclusive(lexical string, baseType Type) (Facet, error) {
	return newRangeBoundFacet("maxExclusive", lexical, baseType, func(cmp int) bool { return cmp < 0 }, "<")
}

func newRangeBoundFacet(
	name, lexical string,
	baseType Type,
	cmpFunc func(int) bool,
	errOp string,
) (Facet, error) {
	compVal, err := newRangeFacet(name, lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &RangeFacet{
		name:    name,
		lexical: lexical,
		value:   compVal,
		cmpFunc: cmpFunc,
		errOp:   errOp,
	}, nil
}

// newRangeFacet parses the lexical value and creates the appropriate ComparableValue.
func newRangeFacet(facetName, lexical string, baseType Type) (ComparableValue, error) {
	if err := validateRangeFacetApplicability(facetName, baseType); err != nil {
		return nil, err
	}
	return parseRangeFacetValue(facetName, lexical, baseType)
}

func validateRangeFacetApplicability(facetName string, baseType Type) error {
	facets := rangeFacetFundamentalFacets(baseType)
	if facets == nil {
		return nil
	}
	if facets.Ordered == OrderedTotal || facets.Ordered == OrderedPartial {
		return nil
	}
	typeName := "unknown"
	if builtinType, ok := AsBuiltinType(baseType); ok {
		typeName = builtinType.Name().Local
	} else if simpleType, ok := AsSimpleType(baseType); ok {
		typeName = simpleType.QName.Local
	}
	return fmt.Errorf("%s: only applicable to ordered types, but base type %s is not ordered", facetName, typeName)
}

func rangeFacetFundamentalFacets(baseType Type) *FundamentalFacets {
	if baseType == nil {
		return nil
	}
	facets := baseType.FundamentalFacets()
	if facets != nil {
		return facets
	}
	primitive := baseType.PrimitiveType()
	if primitive == nil || primitive == baseType {
		return nil
	}
	return primitive.FundamentalFacets()
}

func parseRangeFacetValue(facetName, lexical string, baseType Type) (ComparableValue, error) {
	if baseType == nil {
		return nil, fmt.Errorf("%s: %w", facetName, ErrCannotDeterminePrimitiveType)
	}
	comparableValue, err := parseLexicalToComparable(lexical, baseType)
	if err == nil {
		return comparableValue, nil
	}
	var unsupported unsupportedComparablePrimitiveError
	if errors.As(err, &unsupported) {
		return nil, fmt.Errorf("%s: no parser available for primitive type %s", facetName, unsupported.primitive)
	}
	return nil, fmt.Errorf("%s: %w", facetName, err)
}

package model

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/durationlex"
	typefacetcore "github.com/jacoelho/xsd/internal/typefacet/internalcore"
	"github.com/jacoelho/xsd/internal/value/temporal"
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

// IsLengthFacet reports whether the facet is a length-related facet
// (length, minLength, or maxLength).
func IsLengthFacet(facet Facet) bool {
	switch facet.(type) {
	case *Length, *MinLength, *MaxLength:
		return true
	default:
		return false
	}
}

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

// TypedValueForFacet creates a TypedValue for facet schemacheck.
func TypedValueForFacet(value string, typ Type) TypedValue {
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

// ApplyFacets applies all facets to a TypedValue
func ApplyFacets(value TypedValue, facets []Facet, baseType Type) error {
	facetsAny := make([]any, len(facets))
	for i, facet := range facets {
		facetsAny[i] = facet
	}
	return typefacetcore.ApplyFacets(value, facetsAny, baseType, typefacetcore.ApplyFacetOps{
		ValidateFacet: func(facet any, value any, baseType any) error {
			f, ok := facet.(Facet)
			if !ok {
				return fmt.Errorf("invalid facet %T", facet)
			}
			tv, ok := value.(TypedValue)
			if !ok {
				return fmt.Errorf("invalid typed value %T", value)
			}
			bt, ok := baseType.(Type)
			if !ok {
				return fmt.Errorf("invalid base type %T", baseType)
			}
			return f.Validate(tv, bt)
		},
	})
}

// ErrCannotDeterminePrimitiveType is returned when the primitive type cannot be
// determined during parsing; schema validation handles this later.
var ErrCannotDeterminePrimitiveType = errors.New("cannot determine primitive type")

// NewMinInclusive creates a minInclusive facet based on the base type
// It automatically determines the correct ComparableValue type and parses the value
func NewMinInclusive(lexical string, baseType Type) (Facet, error) {
	compVal, err := newRangeFacet("minInclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &RangeFacet{
		name:    "minInclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}, nil
}

// NewMaxInclusive creates a maxInclusive facet based on the base type
func NewMaxInclusive(lexical string, baseType Type) (Facet, error) {
	compVal, err := newRangeFacet("maxInclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &RangeFacet{
		name:    "maxInclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp <= 0 },
		errOp:   "<=",
	}, nil
}

// NewMinExclusive creates a minExclusive facet based on the base type
func NewMinExclusive(lexical string, baseType Type) (Facet, error) {
	compVal, err := newRangeFacet("minExclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &RangeFacet{
		name:    "minExclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp > 0 },
		errOp:   ">",
	}, nil
}

// NewMaxExclusive creates a maxExclusive facet based on the base type
func NewMaxExclusive(lexical string, baseType Type) (Facet, error) {
	compVal, err := newRangeFacet("maxExclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &RangeFacet{
		name:    "maxExclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp < 0 },
		errOp:   "<",
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
	typeName := baseType.Name().Local
	if value, handled, err := parseRangeFacetValueForTypeName(facetName, lexical, baseType, typeName); handled {
		return value, err
	}
	return parseRangeFacetValueForPrimitive(facetName, lexical, baseType)
}

func parseRangeFacetValueForTypeName(facetName, lexical string, baseType Type, typeName string) (ComparableValue, bool, error) {
	switch typeName {
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		value, err := parseRangeInteger(facetName, lexical, baseType)
		return value, true, err
	default:
		return nil, false, nil
	}
}

func parseRangeFacetValueForPrimitive(facetName, lexical string, baseType Type) (ComparableValue, error) {
	primitiveType := baseType.PrimitiveType()
	if primitiveType == nil {
		return nil, fmt.Errorf("%s: %w", facetName, ErrCannotDeterminePrimitiveType)
	}

	primitiveName := primitiveType.Name().Local
	isIntegerDerived := isIntegerDerivedType(baseType)

	switch primitiveName {
	case "decimal":
		if isIntegerDerived {
			return parseRangeInteger(facetName, lexical, baseType)
		}
		return parseRangeDecimal(facetName, lexical, baseType)
	case "integer":
		return parseRangeInteger(facetName, lexical, baseType)
	case "dateTime", "date", "time", "gYear", "gYearMonth", "gMonth", "gMonthDay", "gDay":
		return parseRangeTemporal(facetName, lexical, baseType, primitiveName)
	case "float":
		return parseRangeFloat(facetName, lexical, baseType)
	case "double":
		return parseRangeDouble(facetName, lexical, baseType)
	case "duration":
		return parseRangeDuration(facetName, lexical, baseType)
	default:
		return nil, fmt.Errorf("%s: no parser available for primitive type %s", facetName, primitiveName)
	}
}

func parseRangeInteger(facetName, lexical string, baseType Type) (ComparableValue, error) {
	intVal, err := ParseInteger(lexical)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return ComparableInt{Value: intVal, Typ: baseType}, nil
}

func parseRangeDecimal(facetName, lexical string, baseType Type) (ComparableValue, error) {
	rat, err := ParseDecimal(lexical)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return ComparableDec{Value: rat, Typ: baseType}, nil
}

func parseRangeTemporal(facetName, lexical string, baseType Type, primitiveName string) (ComparableValue, error) {
	timeVal, err := temporal.ParsePrimitive(primitiveName, []byte(lexical))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return ComparableTime{
		Value:        timeVal.Time,
		Typ:          baseType,
		TimezoneKind: TimezoneKind(lexical),
		Kind:         timeVal.Kind,
		LeapSecond:   timeVal.LeapSecond,
	}, nil
}

func parseRangeFloat(facetName, lexical string, baseType Type) (ComparableValue, error) {
	floatVal, err := ParseFloat(lexical)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return ComparableFloat32{Value: floatVal, Typ: baseType}, nil
}

func parseRangeDouble(facetName, lexical string, baseType Type) (ComparableValue, error) {
	doubleVal, err := ParseDouble(lexical)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return ComparableFloat64{Value: doubleVal, Typ: baseType}, nil
}

func parseRangeDuration(facetName, lexical string, baseType Type) (ComparableValue, error) {
	xsdDur, err := durationlex.Parse(lexical)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return ComparableXSDDuration{Value: xsdDur, Typ: baseType}, nil
}

package facets

import (
	"github.com/jacoelho/xsd/internal/types"
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
	Value string
	Typ   types.Type
}

// DeferredFacet stores raw facet data when the base type is not available during parsing.
// These facets are validated during schema validation when the base type is resolved.
type DeferredFacet struct {
	FacetName  string
	FacetValue string
}

// Type returns the XSD type used for facet checks.
func (s *StringTypedValue) Type() types.Type { return s.Typ }

// Lexical returns the raw lexical value used when parsing fails.
func (s *StringTypedValue) Lexical() string { return s.Value }

// Native returns the lexical value as its native representation.
func (s *StringTypedValue) Native() any { return s.Value }

// String returns the lexical value for error messages.
func (s *StringTypedValue) String() string { return s.Value }

// TypedValueForFacet creates a TypedValue for facet validation.
func TypedValueForFacet(value string, typ types.Type) types.TypedValue {
	if st, ok := typ.(types.SimpleTypeDefinition); ok {
		if parsed, err := st.ParseValue(value); err == nil {
			return parsed
		}
	}
	return &StringTypedValue{Value: value, Typ: typ}
}

// Facet is the unified interface for all constraining facets
type Facet interface {
	Name() string
	Validate(value types.TypedValue, baseType types.Type) error
}

// LexicalFacet is a facet that has a lexical string value.
// Examples include pattern and enumeration facets.
type LexicalFacet interface {
	Facet
	GetLexical() string
}

// IntValueFacet is a facet that has an integer value.
// Examples include length, minLength, maxLength, totalDigits, and fractionDigits facets.
type IntValueFacet interface {
	Facet
	GetIntValue() int
}

// ApplyFacets applies all facets to a TypedValue
func ApplyFacets(value types.TypedValue, facets []Facet, baseType types.Type) error {
	for _, f := range facets {
		if err := f.Validate(value, baseType); err != nil {
			return err
		}
	}
	return nil
}

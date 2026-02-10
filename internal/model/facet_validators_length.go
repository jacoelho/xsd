package model

import (
	"fmt"
	"unicode/utf8"
)

// Length represents a length facet
type Length struct {
	Value int
}

// Name returns the facet name
func (l *Length) Name() string {
	return "length"
}

// GetIntValue returns the integer value (implements IntValueFacet)
func (l *Length) GetIntValue() int {
	return l.Value
}

// Validate checks if the value has the exact length (unified Facet interface)
func (l *Length) Validate(value TypedValue, baseType Type) error {
	return l.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value has the exact length.
func (l *Length) ValidateLexical(lexical string, baseType Type) error {
	// per XSD 1.0 errata, length facets are ignored for QName and NOTATION types
	if isQNameOrNotationType(baseType) {
		return nil
	}
	length := getLength(lexical, baseType)
	if length != l.Value {
		return fmt.Errorf("length must be %d, got %d", l.Value, length)
	}
	return nil
}

// MinLength represents a minLength facet
type MinLength struct {
	Value int
}

// Name returns the facet name
func (m *MinLength) Name() string {
	return "minLength"
}

// GetIntValue returns the integer value (implements IntValueFacet)
func (m *MinLength) GetIntValue() int {
	return m.Value
}

// Validate checks if the value meets minimum length (unified Facet interface)
func (m *MinLength) Validate(value TypedValue, baseType Type) error {
	return m.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value meets minimum length.
func (m *MinLength) ValidateLexical(lexical string, baseType Type) error {
	// per XSD 1.0 errata, length facets are ignored for QName and NOTATION types
	if isQNameOrNotationType(baseType) {
		return nil
	}
	length := getLength(lexical, baseType)
	if length < m.Value {
		return fmt.Errorf("length must be at least %d, got %d", m.Value, length)
	}
	return nil
}

// MaxLength represents a maxLength facet
type MaxLength struct {
	Value int
}

// Name returns the facet name
func (m *MaxLength) Name() string {
	return "maxLength"
}

// GetIntValue returns the integer value (implements IntValueFacet)
func (m *MaxLength) GetIntValue() int {
	return m.Value
}

// Validate checks if the value meets maximum length (unified Facet interface)
func (m *MaxLength) Validate(value TypedValue, baseType Type) error {
	return m.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value meets maximum length.
func (m *MaxLength) ValidateLexical(lexical string, baseType Type) error {
	// per XSD 1.0 errata, length facets are ignored for QName and NOTATION types
	if isQNameOrNotationType(baseType) {
		return nil
	}
	length := getLength(lexical, baseType)
	if length > m.Value {
		return fmt.Errorf("length must be at most %d, got %d", m.Value, length)
	}
	return nil
}

// getLength calculates the length of a value according to XSD 1.0 specification.
// The unit of length varies by type:
//   - hexBinary/base64Binary: octets (bytes) - XSD 1.0 Part 2, sections 3.2.1.1-3.2.1.3
//   - list types: number of list items - XSD 1.0 Part 2, section 3.2.1
//   - string types: characters (Unicode code points) - XSD 1.0 Part 2, sections 3.2.1.1-3.2.1.3
func getLength(value string, baseType Type) int {
	if baseType == nil {
		// no type information - use character count as default
		return utf8.RuneCountInString(value)
	}

	// use LengthMeasurable interface if available
	if lm, ok := baseType.(LengthMeasurable); ok {
		return lm.MeasureLength(value)
	}

	// fallback: character count for types that don't implement LengthMeasurable
	return utf8.RuneCountInString(value)
}

// Enumeration represents an enumeration facet.

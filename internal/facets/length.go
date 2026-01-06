package facets

import (
	"fmt"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/types"
)

// isQNameOrNotationType checks if a type is QName, NOTATION, or restricts either.
// Per XSD 1.0 errata, length facets should be ignored for QName and NOTATION types
// because their value space length depends on namespace context, not lexical form.
func isQNameOrNotationType(t types.Type) bool {
	if t == nil {
		return false
	}

	qnameOrNotation := func(local string) bool {
		return local == string(types.TypeNameQName) || local == string(types.TypeNameNOTATION)
	}

	// List types should still honor length facets on list size.
	if st, ok := t.(*types.SimpleType); ok && st.Variety() == types.ListVariety {
		return false
	}

	if st, ok := t.(*types.SimpleType); ok {
		visited := make(map[*types.SimpleType]bool)
		current := st
		for current != nil && !visited[current] {
			visited[current] = true
			if current.Restriction != nil && !current.Restriction.Base.IsZero() {
				if qnameOrNotation(current.Restriction.Base.Local) {
					ns := current.Restriction.Base.Namespace
					if ns == types.XSDNamespace || ns.IsEmpty() {
						return true
					}
				}
			}
			next, ok := current.ResolvedBase.(*types.SimpleType)
			if !ok {
				break
			}
			current = next
		}
	}

	if qnameOrNotation(t.Name().Local) {
		return true
	}
	if primitive := t.PrimitiveType(); primitive != nil && qnameOrNotation(primitive.Name().Local) {
		return true
	}

	// Walk base types for restriction chains that didn't resolve primitive type.
	visited := make(map[types.Type]bool)
	current := t
	for current != nil && !visited[current] {
		visited[current] = true
		if qnameOrNotation(current.Name().Local) {
			return true
		}
		current = current.BaseType()
	}

	return false
}

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
func (l *Length) Validate(value types.TypedValue, baseType types.Type) error {
	// Per XSD 1.0 errata, length facets are ignored for QName and NOTATION types
	if isQNameOrNotationType(baseType) {
		return nil
	}
	lexical := value.Lexical()
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
func (m *MinLength) Validate(value types.TypedValue, baseType types.Type) error {
	// Per XSD 1.0 errata, length facets are ignored for QName and NOTATION types
	if isQNameOrNotationType(baseType) {
		return nil
	}
	lexical := value.Lexical()
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
func (m *MaxLength) Validate(value types.TypedValue, baseType types.Type) error {
	// Per XSD 1.0 errata, length facets are ignored for QName and NOTATION types
	if isQNameOrNotationType(baseType) {
		return nil
	}
	lexical := value.Lexical()
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
func getLength(value string, baseType types.Type) int {
	if baseType == nil {
		// No type information - use character count as default
		return utf8.RuneCountInString(value)
	}

	// Use LengthMeasurable interface if available
	if lm, ok := baseType.(types.LengthMeasurable); ok {
		return lm.MeasureLength(value)
	}

	// Fallback: character count for types that don't implement LengthMeasurable
	return utf8.RuneCountInString(value)
}

package model

import (
	"errors"
	"fmt"
)

type RangeFacet struct {
	// Facet name (minInclusive, maxInclusive, etc.)
	name string
	// Keep lexical for schema/error messages
	lexical string
	// Comparable value
	value ComparableValue
	// Comparison function: returns true if validation passes
	cmpFunc func(cmp int) bool
	// Error operator string (">=", "<=", ">", "<")
	errOp string
}

// Name returns the facet name
func (r *RangeFacet) Name() string {
	return r.name
}

// GetLexical returns the lexical value (implements LexicalFacet)
func (r *RangeFacet) GetLexical() string {
	return r.lexical
}

// Validate validates a TypedValue using ComparableValue comparison
func (r *RangeFacet) Validate(value TypedValue, baseType Type) error {
	compVal, err := extractComparableValue(value, baseType)
	if err != nil {
		return fmt.Errorf("%s: %w", r.name, err)
	}

	// compare using ComparableValue interface
	cmp, err := compVal.Compare(r.value)
	if err != nil {
		if errors.Is(err, errIndeterminateDurationComparison) || errors.Is(err, errIndeterminateTimeComparison) {
			return fmt.Errorf("value %s must be %s %s", value.String(), r.errOp, r.lexical)
		}
		return fmt.Errorf("%s: cannot compare values: %w", r.name, err)
	}

	if !r.cmpFunc(cmp) {
		return fmt.Errorf("value %s must be %s %s", value.String(), r.errOp, r.lexical)
	}

	return nil
}

// isQNameOrNotationType checks if a type is QName, NOTATION, or restricts either.
// Per XSD 1.0 errata, length facets should be ignored for QName and NOTATION types
// because their value space length depends on namespace context, not lexical form.

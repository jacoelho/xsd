package facets

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/types"
)

// TotalDigits represents a totalDigits facet
type TotalDigits struct {
	Value int
}

// Name returns the facet name
func (t *TotalDigits) Name() string {
	return "totalDigits"
}

// GetIntValue returns the integer value (implements IntValueFacet)
func (t *TotalDigits) GetIntValue() int {
	return t.Value
}

// Validate checks if the total number of digits doesn't exceed the limit (unified Facet interface)
func (t *TotalDigits) Validate(value types.TypedValue, baseType types.Type) error {
	lexical := strings.TrimSpace(value.Lexical())
	digitCount := countDigits(lexical)
	if digitCount > t.Value {
		return fmt.Errorf("total number of digits (%d) exceeds limit (%d)", digitCount, t.Value)
	}
	return nil
}

// FractionDigits represents a fractionDigits facet
type FractionDigits struct {
	Value int
}

// Name returns the facet name
func (f *FractionDigits) Name() string {
	return "fractionDigits"
}

// GetIntValue returns the integer value (implements IntValueFacet)
func (f *FractionDigits) GetIntValue() int {
	return f.Value
}

// Validate checks if the number of fractional digits doesn't exceed the limit (unified Facet interface)
func (f *FractionDigits) Validate(value types.TypedValue, baseType types.Type) error {
	lexical := strings.TrimSpace(value.Lexical())
	fractionDigits := countFractionDigits(lexical)
	if fractionDigits > f.Value {
		return fmt.Errorf("number of fraction digits (%d) exceeds limit (%d)", fractionDigits, f.Value)
	}
	return nil
}

// countDigits counts the total number of digits in a string
func countDigits(value string) int {
	count := 0
	for _, r := range value {
		if r >= '0' && r <= '9' {
			count++
		}
	}
	return count
}

// countFractionDigits counts digits after the decimal point
func countFractionDigits(value string) int {
	_, after, ok := strings.Cut(value, ".")
	if !ok {
		return 0 // no decimal point, so no fraction digits
	}

	fractionPart := after

	// remove exponent if present (e.g., "1.23E4" -> "1.23")
	if eIdx := strings.IndexAny(fractionPart, "Ee"); eIdx >= 0 {
		fractionPart = fractionPart[:eIdx]
	}

	return countDigits(fractionPart)
}
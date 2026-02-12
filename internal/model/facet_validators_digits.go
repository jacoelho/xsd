package model

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/num"
)

// TotalDigits defines an exported type.
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
func (t *TotalDigits) Validate(value TypedValue, baseType Type) error {
	return t.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value respects totalDigits.
func (t *TotalDigits) ValidateLexical(lexical string, _ Type) error {
	lexical = TrimXMLWhitespace(lexical)
	digitCount, _, err := normalizedDecimalDigits(lexical)
	if err != nil {
		return err
	}
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
func (f *FractionDigits) Validate(value TypedValue, baseType Type) error {
	return f.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value respects fractionDigits.
func (f *FractionDigits) ValidateLexical(lexical string, _ Type) error {
	lexical = TrimXMLWhitespace(lexical)
	_, fractionDigits, err := normalizedDecimalDigits(lexical)
	if err != nil {
		return err
	}
	if fractionDigits > f.Value {
		return fmt.Errorf("number of fraction digits (%d) exceeds limit (%d)", fractionDigits, f.Value)
	}
	return nil
}

func normalizedDecimalDigits(value string) (int, int, error) {
	if value == "" {
		return 0, 0, fmt.Errorf("invalid decimal: empty string")
	}
	// strip exponent if present (e.g., "1.23E4" -> "1.23")
	if eIdx := strings.IndexAny(value, "Ee"); eIdx >= 0 {
		value = value[:eIdx]
	}
	if value == "" {
		return 0, 0, fmt.Errorf("invalid decimal: empty string")
	}
	if value[0] == '+' || value[0] == '-' {
		value = value[1:]
	}
	dec, perr := num.ParseDec([]byte(value))
	if perr != nil {
		return 0, 0, fmt.Errorf("invalid decimal: %s", value)
	}
	return len(dec.Coef), int(dec.Scale), nil
}

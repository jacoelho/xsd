package value

import (
	"bytes"
	"fmt"

	"github.com/jacoelho/xsd/internal/num"
)

// ParseBoolean parses a boolean lexical value into a bool.
func ParseBoolean(lexical []byte) (bool, error) {
	trimmed := TrimXMLWhitespace(lexical)
	switch {
	case bytes.Equal(trimmed, []byte("true")) || bytes.Equal(trimmed, []byte("1")):
		return true, nil
	case bytes.Equal(trimmed, []byte("false")) || bytes.Equal(trimmed, []byte("0")):
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean: %s", string(trimmed))
	}
}

// ParseFloat parses a float lexical value into float32.
func ParseFloat(lexical []byte) (float32, error) {
	trimmed := TrimXMLWhitespace(lexical)
	f, _, perr := num.ParseFloat32(trimmed)
	if perr == nil {
		return f, nil
	}
	if perr.Kind == num.ParseEmpty {
		return 0, fmt.Errorf("invalid float: empty string")
	}
	return 0, fmt.Errorf("invalid float: %s", string(trimmed))
}

// ParseDouble parses a double lexical value into float64.
func ParseDouble(lexical []byte) (float64, error) {
	trimmed := TrimXMLWhitespace(lexical)
	f, _, perr := num.ParseFloat64(trimmed)
	if perr == nil {
		return f, nil
	}
	if perr.Kind == num.ParseEmpty {
		return 0, fmt.Errorf("invalid double: empty string")
	}
	return 0, fmt.Errorf("invalid double: %s", string(trimmed))
}

// ValidateFloatLexical checks whether the lexical form is valid for float.
func ValidateFloatLexical(lexical []byte) error {
	return validateFloatLexical(lexical, "float")
}

// ValidateDoubleLexical checks whether the lexical form is valid for double.
func ValidateDoubleLexical(lexical []byte) error {
	return validateFloatLexical(lexical, "double")
}

func validateFloatLexical(lexical []byte, label string) error {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return fmt.Errorf("invalid %s: empty string", label)
	}
	if perr := num.ValidateFloatLexical(trimmed); perr != nil {
		if perr.Kind == num.ParseEmpty {
			return fmt.Errorf("invalid %s: empty string", label)
		}
		return fmt.Errorf("invalid %s: %s", label, string(trimmed))
	}
	return nil
}

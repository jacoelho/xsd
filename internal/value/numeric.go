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

// ParseDecimal parses a decimal lexical value into num.Dec.
func ParseDecimal(lexical []byte) (num.Dec, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return num.Dec{}, fmt.Errorf("invalid decimal: empty string")
	}
	dec, perr := num.ParseDec(trimmed)
	if perr != nil {
		return num.Dec{}, fmt.Errorf("invalid decimal: %s", string(trimmed))
	}
	return dec, nil
}

// ParseInteger parses an integer lexical value into num.Int.
func ParseInteger(lexical []byte) (num.Int, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return num.Int{}, fmt.Errorf("invalid integer: empty string")
	}
	intVal, perr := num.ParseInt(trimmed)
	if perr != nil {
		return num.Int{}, fmt.Errorf("invalid integer: %s", string(trimmed))
	}
	return intVal, nil
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
	f, _, perr := num.ParseFloat(trimmed, 64)
	if perr == nil {
		return f, nil
	}
	if perr.Kind == num.ParseEmpty {
		return 0, fmt.Errorf("invalid double: empty string")
	}
	return 0, fmt.Errorf("invalid double: %s", string(trimmed))
}

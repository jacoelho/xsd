package value

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
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
	if len(trimmed) == 0 {
		return 0, fmt.Errorf("invalid float: empty string")
	}
	s := unsafeString(trimmed)
	switch s {
	case "+INF":
		return 0, fmt.Errorf("invalid float: +INF")
	case "INF":
		return float32(math.Inf(1)), nil
	case "-INF":
		return float32(math.Inf(-1)), nil
	case "NaN":
		return float32(math.NaN()), nil
	default:
		if !isFloatLexical(trimmed) {
			return 0, fmt.Errorf("invalid float: %s", string(trimmed))
		}
		f, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid float: %s", string(trimmed))
		}
		return float32(f), nil
	}
}

// ParseDouble parses a double lexical value into float64.
func ParseDouble(lexical []byte) (float64, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return 0, fmt.Errorf("invalid double: empty string")
	}
	s := unsafeString(trimmed)
	switch s {
	case "+INF":
		return 0, fmt.Errorf("invalid double: +INF")
	case "INF":
		return math.Inf(1), nil
	case "-INF":
		return math.Inf(-1), nil
	case "NaN":
		return math.NaN(), nil
	default:
		if !isFloatLexical(trimmed) {
			return 0, fmt.Errorf("invalid double: %s", string(trimmed))
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid double: %s", string(trimmed))
		}
		return f, nil
	}
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
	if bytes.Equal(trimmed, []byte("+INF")) {
		return fmt.Errorf("invalid %s: +INF", label)
	}
	if bytes.Equal(trimmed, []byte("INF")) || bytes.Equal(trimmed, []byte("-INF")) || bytes.Equal(trimmed, []byte("NaN")) {
		return nil
	}
	if !isFloatLexical(trimmed) {
		return fmt.Errorf("invalid %s: %s", label, string(trimmed))
	}
	return nil
}

func isFloatLexical(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	i := 0
	if value[i] == '+' || value[i] == '-' {
		i++
		if i == len(value) {
			return false
		}
	}
	startDigits := 0
	for i < len(value) && isDigit(value[i]) {
		i++
		startDigits++
	}
	if i < len(value) && value[i] == '.' {
		i++
		fracDigits := 0
		for i < len(value) && isDigit(value[i]) {
			i++
			fracDigits++
		}
		if startDigits == 0 && fracDigits == 0 {
			return false
		}
	} else if startDigits == 0 {
		return false
	}
	if i < len(value) && (value[i] == 'e' || value[i] == 'E') {
		i++
		if i == len(value) {
			return false
		}
		if value[i] == '+' || value[i] == '-' {
			i++
			if i == len(value) {
				return false
			}
		}
		expDigits := 0
		for i < len(value) && isDigit(value[i]) {
			i++
			expDigits++
		}
		if expDigits == 0 {
			return false
		}
	}
	return i == len(value)
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

package num

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"unsafe"
)

// FloatClass identifies the ordering class of a float value.
type FloatClass uint8

const (
	FloatFinite FloatClass = iota
	FloatPosInf
	FloatNegInf
	FloatNaN
)

// ParseFloat32 parses an XSD float lexical value.
func ParseFloat32(b []byte) (float32, FloatClass, *ParseError) {
	f, class, err := ParseFloat(b, 32)
	return float32(f), class, err
}

// ValidateFloatLexical checks whether a float/double lexical form is valid.
func ValidateFloatLexical(b []byte) *ParseError {
	if len(b) == 0 {
		return &ParseError{Kind: ParseEmpty}
	}
	switch {
	case bytes.Equal(b, []byte("INF")),
		bytes.Equal(b, []byte("-INF")),
		bytes.Equal(b, []byte("NaN")):
		return nil
	case bytes.Equal(b, []byte("+INF")):
		return &ParseError{Kind: ParseBadChar}
	}
	if !isFloatLexical(b) {
		return &ParseError{Kind: ParseBadChar}
	}
	return nil
}

// ParseFloat parses an XSD float/double lexical value for the requested bit size.
func ParseFloat(b []byte, bits int) (float64, FloatClass, *ParseError) {
	if len(b) == 0 {
		return 0, FloatFinite, &ParseError{Kind: ParseEmpty}
	}
	switch {
	case bytes.Equal(b, []byte("INF")):
		return math.Inf(1), FloatPosInf, nil
	case bytes.Equal(b, []byte("-INF")):
		return math.Inf(-1), FloatNegInf, nil
	case bytes.Equal(b, []byte("NaN")):
		return math.NaN(), FloatNaN, nil
	case bytes.Equal(b, []byte("+INF")):
		return 0, FloatFinite, &ParseError{Kind: ParseBadChar}
	}
	if !isFloatLexical(b) {
		return 0, FloatFinite, &ParseError{Kind: ParseBadChar}
	}
	lexical := ""
	if len(b) != 0 {
		lexical = unsafe.String(unsafe.SliceData(b), len(b))
	}
	f, err := strconv.ParseFloat(lexical, bits)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			class := FloatFinite
			if math.IsInf(f, 1) {
				class = FloatPosInf
			} else if math.IsInf(f, -1) {
				class = FloatNegInf
			}
			return f, class, nil
		}
		return 0, FloatFinite, &ParseError{Kind: ParseBadChar}
	}
	return f, FloatFinite, nil
}

// CompareFloat compares two parsed float/double values.
// The boolean result is false when either side is NaN (unordered).
func CompareFloat(a float64, ac FloatClass, b float64, bc FloatClass) (int, bool) {
	if ac == FloatNaN || bc == FloatNaN {
		return 0, false
	}
	if ac == FloatPosInf {
		if bc == FloatPosInf {
			return 0, true
		}
		return 1, true
	}
	if ac == FloatNegInf {
		if bc == FloatNegInf {
			return 0, true
		}
		return -1, true
	}
	if bc == FloatPosInf {
		return -1, true
	}
	if bc == FloatNegInf {
		return 1, true
	}
	switch {
	case a < b:
		return -1, true
	case a > b:
		return 1, true
	default:
		return 0, true
	}
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

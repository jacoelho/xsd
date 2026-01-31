package num

import (
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
	if len(b) == 0 {
		return 0, FloatFinite, &ParseError{Kind: ParseEmpty}
	}
	switch {
	case bytesEqual(b, []byte("INF")):
		return float32(math.Inf(1)), FloatPosInf, nil
	case bytesEqual(b, []byte("-INF")):
		return float32(math.Inf(-1)), FloatNegInf, nil
	case bytesEqual(b, []byte("NaN")):
		return float32(math.NaN()), FloatNaN, nil
	case bytesEqual(b, []byte("+INF")):
		return 0, FloatFinite, &ParseError{Kind: ParseBadChar}
	}
	if !isFloatLexical(b) {
		return 0, FloatFinite, &ParseError{Kind: ParseBadChar}
	}
	f, err := strconv.ParseFloat(unsafeString(b), 32)
	if err != nil {
		return 0, FloatFinite, &ParseError{Kind: ParseBadChar}
	}
	return float32(f), FloatFinite, nil
}

// ValidateFloatLexical checks whether a float/double lexical form is valid.
func ValidateFloatLexical(b []byte) *ParseError {
	if len(b) == 0 {
		return &ParseError{Kind: ParseEmpty}
	}
	switch {
	case bytesEqual(b, []byte("INF")),
		bytesEqual(b, []byte("-INF")),
		bytesEqual(b, []byte("NaN")):
		return nil
	case bytesEqual(b, []byte("+INF")):
		return &ParseError{Kind: ParseBadChar}
	}
	if !isFloatLexical(b) {
		return &ParseError{Kind: ParseBadChar}
	}
	return nil
}

// ParseFloat64 parses an XSD double lexical value.
func ParseFloat64(b []byte) (float64, FloatClass, *ParseError) {
	if len(b) == 0 {
		return 0, FloatFinite, &ParseError{Kind: ParseEmpty}
	}
	switch {
	case bytesEqual(b, []byte("INF")):
		return math.Inf(1), FloatPosInf, nil
	case bytesEqual(b, []byte("-INF")):
		return math.Inf(-1), FloatNegInf, nil
	case bytesEqual(b, []byte("NaN")):
		return math.NaN(), FloatNaN, nil
	case bytesEqual(b, []byte("+INF")):
		return 0, FloatFinite, &ParseError{Kind: ParseBadChar}
	}
	if !isFloatLexical(b) {
		return 0, FloatFinite, &ParseError{Kind: ParseBadChar}
	}
	f, err := strconv.ParseFloat(unsafeString(b), 64)
	if err != nil {
		return 0, FloatFinite, &ParseError{Kind: ParseBadChar}
	}
	return f, FloatFinite, nil
}

// CompareFloat32 compares two float32 values using XSD ordering rules.
func CompareFloat32(a float32, ac FloatClass, b float32, bc FloatClass) (int, bool) {
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

// CompareFloat64 compares two float64 values using XSD ordering rules.
func CompareFloat64(a float64, ac FloatClass, b float64, bc FloatClass) (int, bool) {
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

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func unsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
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

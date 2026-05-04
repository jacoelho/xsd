package xsd

import (
	"fmt"
	"math"
	"strconv"
)

func parseFloatCanonical(s string, bits int) (string, error) {
	v, err := parseXSDFloat(s, bits)
	if err != nil {
		return "", err
	}
	if math.IsInf(v, 1) {
		return "INF", nil
	}
	if math.IsInf(v, -1) {
		return "-INF", nil
	}
	if math.IsNaN(v) {
		return "NaN", nil
	}
	return strconv.FormatFloat(v, 'g', -1, bits), nil
}

func parseXSDFloat(s string, bits int) (float64, error) {
	switch s {
	case "INF":
		return math.Inf(1), nil
	case "-INF":
		return math.Inf(-1), nil
	case "NaN":
		return math.NaN(), nil
	}
	if !isXSDFloatLexical(s) {
		return 0, fmt.Errorf("invalid float")
	}
	v, err := strconv.ParseFloat(s, bits)
	if err != nil {
		if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
			return v, nil
		}
		return 0, fmt.Errorf("invalid float")
	}
	return v, nil
}

func isXSDFloatLexical(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[i] == '+' || s[i] == '-' {
		i++
		if i == len(s) {
			return false
		}
	}
	digits := 0
	for i < len(s) && isASCIIDigit(s[i]) {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isASCIIDigit(s[i]) {
			i++
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		expDigits := 0
		for i < len(s) && isASCIIDigit(s[i]) {
			i++
			expDigits++
		}
		if expDigits == 0 {
			return false
		}
	}
	return i == len(s)
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func applyFloatBounds(kind primitiveKind, f facetSet, norm string) error {
	bits := 64
	if kind == primFloat {
		bits = 32
	}
	value, err := parseXSDFloat(norm, bits)
	if err != nil {
		return err
	}
	cmpLit := func(l *compiledLiteral) (float64, bool, error) {
		if l == nil {
			return 0, false, nil
		}
		v, err := parseXSDFloat(l.Canonical, bits)
		return v, true, err
	}
	if lit, ok, err := cmpLit(f.MinInclusive); err != nil {
		return err
	} else if ok && !(value >= lit) {
		return fmt.Errorf("minInclusive facet failed")
	}
	if lit, ok, err := cmpLit(f.MaxInclusive); err != nil {
		return err
	} else if ok && !(value <= lit) {
		return fmt.Errorf("maxInclusive facet failed")
	}
	if lit, ok, err := cmpLit(f.MinExclusive); err != nil {
		return err
	} else if ok && !(value > lit) {
		return fmt.Errorf("minExclusive facet failed")
	}
	if lit, ok, err := cmpLit(f.MaxExclusive); err != nil {
		return err
	} else if ok && !(value < lit) {
		return fmt.Errorf("maxExclusive facet failed")
	}
	return nil
}

func validateFloatFacetBounds(kind primitiveKind, f facetSet) error {
	bits := 64
	if kind == primFloat {
		bits = 32
	}
	lower, lowerExclusive, hasLower, err := floatLowerBound(bits, f)
	if err != nil {
		return err
	}
	upper, upperExclusive, hasUpper, err := floatUpperBound(bits, f)
	if err != nil {
		return err
	}
	if !hasLower || !hasUpper {
		return nil
	}
	if !(lower <= upper) || lower == upper && (lowerExclusive || upperExclusive) {
		return fmt.Errorf("float lower bound cannot exceed upper bound")
	}
	return nil
}

func facetBoundCanonical[T any](inclusive, exclusive *compiledLiteral, parse func(string) (T, error), preferExclusive func(T, T) bool) (T, bool, bool, error) {
	return facetBound(inclusive, exclusive, func(l *compiledLiteral) string { return l.Canonical }, parse, preferExclusive)
}

func facetBoundLexical[T any](inclusive, exclusive *compiledLiteral, parse func(string) (T, error), preferExclusive func(T, T) bool) (T, bool, bool, error) {
	return facetBound(inclusive, exclusive, func(l *compiledLiteral) string { return l.Lexical }, parse, preferExclusive)
}

func facetBound[T any](inclusive, exclusive *compiledLiteral, text func(*compiledLiteral) string, parse func(string) (T, error), preferExclusive func(T, T) bool) (T, bool, bool, error) {
	var zero T
	if inclusive != nil {
		out, err := parse(text(inclusive))
		if err != nil {
			return zero, false, false, err
		}
		if exclusive != nil {
			other, err := parse(text(exclusive))
			if err != nil {
				return zero, false, false, err
			}
			if preferExclusive(other, out) {
				return other, true, true, nil
			}
		}
		return out, false, true, nil
	}
	if exclusive != nil {
		out, err := parse(text(exclusive))
		if err != nil {
			return zero, false, false, err
		}
		return out, true, true, nil
	}
	return zero, false, false, nil
}

func floatLowerBound(bits int, f facetSet) (float64, bool, bool, error) {
	parse := func(s string) (float64, error) { return parseXSDFloat(s, bits) }
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parse, func(other, out float64) bool { return other >= out })
}

func floatUpperBound(bits int, f facetSet) (float64, bool, bool, error) {
	parse := func(s string) (float64, error) { return parseXSDFloat(s, bits) }
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parse, func(other, out float64) bool { return other <= out })
}

func cmpFloat64(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

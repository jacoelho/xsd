package xsd

import (
	"fmt"
	"math"
	"strconv"
)

func formatXSDFloatCanonical(v float64, bits int) string {
	if math.IsInf(v, 1) {
		return "INF"
	}
	if math.IsInf(v, -1) {
		return "-INF"
	}
	if math.IsNaN(v) {
		return "NaN"
	}
	return strconv.FormatFloat(v, 'g', -1, bits)
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

func applyFloatBounds(kind primitiveKind, f facetSet, norm string, actual actualValue) error {
	bits := 64
	if kind == primFloat {
		bits = 32
	}
	value := actual.Float
	if !actual.Valid || actual.Kind != kind {
		var err error
		value, err = parseXSDFloat(norm, bits)
		if err != nil {
			return err
		}
	}
	cmpLit := func(l *compiledLiteral) (float64, bool, error) {
		if l == nil {
			return 0, false, nil
		}
		if l.Actual.Valid && l.Actual.Kind == kind {
			return l.Actual.Float, true, nil
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
	lower, err := floatLowerBound(bits, f)
	if err != nil {
		return err
	}
	upper, err := floatUpperBound(bits, f)
	if err != nil {
		return err
	}
	if !lower.present() || !upper.present() {
		return nil
	}
	if !(lower.value <= upper.value) || lower.value == upper.value && (lower.exclusive() || upper.exclusive()) {
		return fmt.Errorf("float lower bound cannot exceed upper bound")
	}
	return nil
}

type facetBoundKind uint8

const (
	facetBoundAbsent facetBoundKind = iota
	facetBoundInclusive
	facetBoundExclusive
)

type orderedFacetBound[T any] struct {
	value T
	kind  facetBoundKind
}

func (b orderedFacetBound[T]) present() bool {
	return b.kind != facetBoundAbsent
}

func (b orderedFacetBound[T]) exclusive() bool {
	return b.kind == facetBoundExclusive
}

func facetBoundCanonical[T any](inclusive, exclusive *compiledLiteral, parse func(string) (T, error), preferExclusive func(T, T) bool) (orderedFacetBound[T], error) {
	return facetBound(inclusive, exclusive, func(l *compiledLiteral) string { return l.Canonical }, parse, preferExclusive)
}

func facetBoundLexical[T any](inclusive, exclusive *compiledLiteral, parse func(string) (T, error), preferExclusive func(T, T) bool) (orderedFacetBound[T], error) {
	return facetBound(inclusive, exclusive, func(l *compiledLiteral) string { return l.Lexical }, parse, preferExclusive)
}

func facetBound[T any](inclusive, exclusive *compiledLiteral, text func(*compiledLiteral) string, parse func(string) (T, error), preferExclusive func(T, T) bool) (orderedFacetBound[T], error) {
	if inclusive != nil {
		out, err := parse(text(inclusive))
		if err != nil {
			return orderedFacetBound[T]{}, err
		}
		if exclusive != nil {
			other, err := parse(text(exclusive))
			if err != nil {
				return orderedFacetBound[T]{}, err
			}
			if preferExclusive(other, out) {
				return orderedFacetBound[T]{value: other, kind: facetBoundExclusive}, nil
			}
		}
		return orderedFacetBound[T]{value: out, kind: facetBoundInclusive}, nil
	}
	if exclusive != nil {
		out, err := parse(text(exclusive))
		if err != nil {
			return orderedFacetBound[T]{}, err
		}
		return orderedFacetBound[T]{value: out, kind: facetBoundExclusive}, nil
	}
	return orderedFacetBound[T]{}, nil
}

func floatLowerBound(bits int, f facetSet) (orderedFacetBound[float64], error) {
	parse := func(s string) (float64, error) { return parseXSDFloat(s, bits) }
	return facetBoundCanonical(f.MinInclusive, f.MinExclusive, parse, func(other, out float64) bool { return other >= out })
}

func floatUpperBound(bits int, f facetSet) (orderedFacetBound[float64], error) {
	parse := func(s string) (float64, error) { return parseXSDFloat(s, bits) }
	return facetBoundCanonical(f.MaxInclusive, f.MaxExclusive, parse, func(other, out float64) bool { return other <= out })
}

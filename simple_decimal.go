package xsd

import (
	"cmp"
	"fmt"
	"strings"
)

type decimalValue struct {
	Canonical        string
	IntegerCanonical string
	text             string
	start            int
	intEnd           int
	intTrimStart     int
	fracStart        int
	fracTrimEnd      int
	IntegerLexical   bool
	negative         bool
	TotalDigits      uint32
	FractionDigits   uint32
}

func parseDecimalValue(s string) (decimalValue, error) {
	return parseDecimalMode(s, decimalValueOnly)
}

func validateBuiltinIntNoCanonical(s string) error {
	if s == "" {
		return fmt.Errorf("invalid decimal")
	}
	start := 0
	negative := false
	if s[0] == '+' || s[0] == '-' {
		negative = s[0] == '-'
		start = 1
	}
	if start == len(s) {
		return fmt.Errorf("invalid decimal")
	}
	digits := 0
	dot := false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == '.':
			if dot {
				return fmt.Errorf("invalid decimal")
			}
			dot = true
		default:
			return fmt.Errorf("invalid decimal")
		}
	}
	if digits == 0 {
		return fmt.Errorf("invalid decimal")
	}
	if dot {
		return fmt.Errorf("invalid integer")
	}

	digitStart := start
	for digitStart < len(s) && s[digitStart] == '0' {
		digitStart++
	}
	if digitStart == len(s) {
		return nil
	}
	limit := "2147483647"
	if negative {
		limit = "2147483648"
	}
	digitsText := s[digitStart:]
	if len(digitsText) > len(limit) || len(digitsText) == len(limit) && digitsText > limit {
		if negative {
			return fmt.Errorf("minInclusive facet failed")
		}
		return fmt.Errorf("maxInclusive facet failed")
	}
	return nil
}

func validateBuiltinIntNoCanonicalBytes(s []byte) error {
	if len(s) == 0 {
		return fmt.Errorf("invalid decimal")
	}
	start := 0
	negative := false
	if s[0] == '+' || s[0] == '-' {
		negative = s[0] == '-'
		start = 1
	}
	if start == len(s) {
		return fmt.Errorf("invalid decimal")
	}
	digits := 0
	dot := false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == '.':
			if dot {
				return fmt.Errorf("invalid decimal")
			}
			dot = true
		default:
			return fmt.Errorf("invalid decimal")
		}
	}
	if digits == 0 {
		return fmt.Errorf("invalid decimal")
	}
	if dot {
		return fmt.Errorf("invalid integer")
	}

	digitStart := start
	for digitStart < len(s) && s[digitStart] == '0' {
		digitStart++
	}
	if digitStart == len(s) {
		return nil
	}
	limit := "2147483647"
	if negative {
		limit = "2147483648"
	}
	digitsText := s[digitStart:]
	if len(digitsText) > len(limit) || len(digitsText) == len(limit) && stringBytesGreaterThan(digitsText, limit) {
		if negative {
			return fmt.Errorf("minInclusive facet failed")
		}
		return fmt.Errorf("maxInclusive facet failed")
	}
	return nil
}

func stringBytesGreaterThan(s []byte, limit string) bool {
	for i := range s {
		if s[i] != limit[i] {
			return s[i] > limit[i]
		}
	}
	return false
}

func validateDecimalNoOutputBytesFast(f facetSet, s []byte) (bool, error) {
	if f.TotalDigits != nil ||
		f.FractionDigits != nil ||
		f.MinExclusive != nil ||
		f.MaxExclusive != nil ||
		len(f.Enumeration) != 0 ||
		len(f.Patterns) != 0 {
		return false, nil
	}
	minBound, hasMin, ok := decimalInclusiveNonNegativeBound(f.MinInclusive)
	if !ok {
		return false, nil
	}
	maxBound, hasMax, ok := decimalInclusiveNonNegativeBound(f.MaxInclusive)
	if !ok {
		return false, nil
	}
	return true, validateDecimalBytesNonNegativeBounds(s, minBound, hasMin, maxBound, hasMax)
}

type decimalBytesBound struct {
	int  string
	frac string
}

func decimalInclusiveNonNegativeBound(l *compiledLiteral) (decimalBytesBound, bool, bool) {
	if l == nil {
		return decimalBytesBound{}, false, true
	}
	dec := literalDecimal(l)
	if dec.isNegative() {
		return decimalBytesBound{}, false, false
	}
	bound := decimalBytesBound{frac: dec.text[dec.fracStart:dec.fracTrimEnd]}
	if dec.intDigits() == 0 {
		bound.int = "0"
		return bound, true, true
	}
	bound.int = dec.text[dec.intTrimStart:dec.intEnd]
	return bound, true, true
}

func validateDecimalBytesNonNegativeBounds(s []byte, minBound decimalBytesBound, hasMin bool, maxBound decimalBytesBound, hasMax bool) error {
	if len(s) == 0 {
		return fmt.Errorf("invalid decimal")
	}
	start := 0
	negative := false
	if s[0] == '+' || s[0] == '-' {
		negative = s[0] == '-'
		start = 1
	}
	if start == len(s) {
		return fmt.Errorf("invalid decimal")
	}
	dot := -1
	digits := 0
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.':
			if dot >= 0 {
				return fmt.Errorf("invalid decimal")
			}
			dot = i
		case c >= '0' && c <= '9':
			digits++
		default:
			return fmt.Errorf("invalid decimal")
		}
	}
	if digits == 0 {
		return fmt.Errorf("invalid decimal")
	}

	intEnd := len(s)
	fracStart := len(s)
	if dot >= 0 {
		intEnd = dot
		fracStart = dot + 1
	}
	intTrimStart := start
	for intTrimStart < intEnd && s[intTrimStart] == '0' {
		intTrimStart++
	}
	fracTrimEnd := len(s)
	for fracTrimEnd > fracStart && s[fracTrimEnd-1] == '0' {
		fracTrimEnd--
	}
	nonZero := intTrimStart < intEnd || fracTrimEnd > fracStart
	if negative && nonZero {
		if hasMin {
			return fmt.Errorf("minInclusive facet failed")
		}
		return nil
	}
	if hasMin && comparePositiveDecimalBytesToBound(s, intTrimStart, intEnd, fracStart, fracTrimEnd, minBound) < 0 {
		return fmt.Errorf("minInclusive facet failed")
	}
	if hasMax && comparePositiveDecimalBytesToBound(s, intTrimStart, intEnd, fracStart, fracTrimEnd, maxBound) > 0 {
		return fmt.Errorf("maxInclusive facet failed")
	}
	return nil
}

func comparePositiveDecimalBytesToBound(s []byte, intTrimStart, intEnd, fracStart, fracTrimEnd int, bound decimalBytesBound) int {
	intDigits := intEnd - intTrimStart
	if intDigits == 0 {
		intDigits = 1
	}
	if intDigits < len(bound.int) {
		return -1
	}
	if intDigits > len(bound.int) {
		return 1
	}
	for i := range intDigits {
		digit := byte('0')
		if intEnd > intTrimStart {
			digit = s[intTrimStart+i]
		}
		if digit < bound.int[i] {
			return -1
		}
		if digit > bound.int[i] {
			return 1
		}
	}
	fracDigits := fracTrimEnd - fracStart
	common := min(fracDigits, len(bound.frac))
	for i := range common {
		if s[fracStart+i] < bound.frac[i] {
			return -1
		}
		if s[fracStart+i] > bound.frac[i] {
			return 1
		}
	}
	if fracDigits < len(bound.frac) {
		return -1
	}
	if fracDigits > len(bound.frac) {
		return 1
	}
	return 0
}

type decimalParseMode uint8

const (
	decimalValueOnly decimalParseMode = iota
	decimalWithCanonical
)

func parseDecimalMode(s string, mode decimalParseMode) (decimalValue, error) {
	if s == "" {
		return decimalValue{}, fmt.Errorf("invalid decimal")
	}
	start := 0
	negative := false
	if s[0] == '+' || s[0] == '-' {
		negative = s[0] == '-'
		start = 1
	}
	if start == len(s) {
		return decimalValue{}, fmt.Errorf("invalid decimal")
	}
	dot := -1
	digits := 0
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.':
			if dot >= 0 {
				return decimalValue{}, fmt.Errorf("invalid decimal")
			}
			dot = i
		case c >= '0' && c <= '9':
			digits++
		default:
			return decimalValue{}, fmt.Errorf("invalid decimal")
		}
	}
	if digits == 0 {
		return decimalValue{}, fmt.Errorf("invalid decimal")
	}

	intEnd := len(s)
	fracStart := len(s)
	if dot >= 0 {
		intEnd = dot
		fracStart = dot + 1
	}
	intTrimStart := start
	for intTrimStart < intEnd && s[intTrimStart] == '0' {
		intTrimStart++
	}
	fracTrimEnd := len(s)
	for fracTrimEnd > fracStart && s[fracTrimEnd-1] == '0' {
		fracTrimEnd--
	}

	intDigits := intEnd - intTrimStart
	fracDigits := fracTrimEnd - fracStart
	totalDigits := intDigits + fracDigits
	if intDigits == 0 {
		firstFracDigit := fracStart
		for firstFracDigit < fracTrimEnd && s[firstFracDigit] == '0' {
			firstFracDigit++
		}
		totalDigits = fracTrimEnd - firstFracDigit
	}
	if totalDigits == 0 {
		totalDigits = 1
	}

	totalDigits32, err := checkedUint32(totalDigits, "decimal totalDigits exceeds uint32 limit")
	if err != nil {
		return decimalValue{}, err
	}
	fracDigits32, err := checkedUint32(fracDigits, "decimal fractionDigits exceeds uint32 limit")
	if err != nil {
		return decimalValue{}, err
	}
	out := decimalValue{
		text:           s,
		start:          start,
		intEnd:         intEnd,
		intTrimStart:   intTrimStart,
		fracStart:      fracStart,
		fracTrimEnd:    fracTrimEnd,
		IntegerLexical: dot < 0,
		negative:       negative,
		TotalDigits:    totalDigits32,
		FractionDigits: fracDigits32,
	}
	if mode == decimalWithCanonical {
		out.Canonical = out.canonical()
		out.IntegerCanonical = out.integerCanonical()
	}
	return out, nil
}

func (d decimalValue) canonical() string {
	if d.Canonical != "" {
		return d.Canonical
	}
	if d.text == "" {
		return "0.0"
	}
	intDigits := d.intDigits()
	fracDigits := d.fracDigits()
	if intDigits == 0 && fracDigits == 0 {
		return "0.0"
	}
	if intDigits > 0 && fracDigits > 0 {
		if d.negative {
			if d.intTrimStart == d.start {
				return d.text[:d.fracTrimEnd]
			}
			var b strings.Builder
			b.Grow(1 + intDigits + 1 + fracDigits)
			b.WriteByte('-')
			b.WriteString(d.text[d.intTrimStart:d.intEnd])
			b.WriteByte('.')
			b.WriteString(d.text[d.fracStart:d.fracTrimEnd])
			return b.String()
		}
		return d.text[d.intTrimStart:d.fracTrimEnd]
	}

	intPart := "0"
	if intDigits > 0 {
		intPart = d.text[d.intTrimStart:d.intEnd]
	}
	fracPart := "0"
	if fracDigits > 0 {
		fracPart = d.text[d.fracStart:d.fracTrimEnd]
	}
	var b strings.Builder
	if d.negative && (intDigits != 0 || fracDigits != 0) {
		b.Grow(1 + len(intPart) + 1 + len(fracPart))
		b.WriteByte('-')
	} else {
		b.Grow(len(intPart) + 1 + len(fracPart))
	}
	b.WriteString(intPart)
	b.WriteByte('.')
	b.WriteString(fracPart)
	return b.String()
}

func (d decimalValue) integerCanonical() string {
	if d.IntegerCanonical != "" {
		return d.IntegerCanonical
	}
	if d.text == "" {
		return "0"
	}
	intDigits := d.intDigits()
	if intDigits == 0 {
		return "0"
	}
	if d.negative {
		if d.intTrimStart == d.start {
			return d.text[:d.intEnd]
		}
		return "-" + d.text[d.intTrimStart:d.intEnd]
	}
	return d.text[d.intTrimStart:d.intEnd]
}

func (d decimalValue) intDigits() int {
	if d.text == "" {
		return 0
	}
	return d.intEnd - d.intTrimStart
}

func (d decimalValue) fracDigits() int {
	if d.text == "" {
		return 0
	}
	return d.fracTrimEnd - d.fracStart
}

func (d decimalValue) isZero() bool {
	return d.intDigits() == 0 && d.fracDigits() == 0
}

func (d decimalValue) isNegative() bool {
	return d.negative && !d.isZero()
}

func compareDecimalValues(a, b decimalValue) int {
	aNeg := a.isNegative()
	bNeg := b.isNegative()
	if aNeg != bNeg {
		if aNeg {
			return -1
		}
		return 1
	}
	n := comparePositiveDecimalValues(a, b)
	if aNeg {
		return -n
	}
	return n
}

func comparePositiveDecimalValues(a, b decimalValue) int {
	if n := cmp.Compare(a.intDigits(), b.intDigits()); n != 0 {
		return n
	}
	for i := range a.intDigits() {
		if n := cmp.Compare(a.text[a.intTrimStart+i], b.text[b.intTrimStart+i]); n != 0 {
			return n
		}
	}
	n := max(a.fracDigits(), b.fracDigits())
	for i := range n {
		ad := byte('0')
		if i < a.fracDigits() {
			ad = a.text[a.fracStart+i]
		}
		bd := byte('0')
		if i < b.fracDigits() {
			bd = b.text[b.fracStart+i]
		}
		if ad < bd {
			return -1
		}
		if ad > bd {
			return 1
		}
	}
	return 0
}

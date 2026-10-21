package value

import (
	"errors"
	"strings"
)

const decimalZeroCanonical = "0.0"

// DecimalValue is the parsed xs:decimal value-space representation used by
// compile, freeze, and validation.
type DecimalValue struct {
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

// ParseDecimalCanonical parses an xs:decimal lexical value and precomputes both
// decimal and integer canonical strings.
func ParseDecimalCanonical(s string) (DecimalValue, error) {
	out, err := ParseDecimalValue(s)
	if err != nil {
		return DecimalValue{}, err
	}
	out.Canonical = out.CanonicalText()
	out.IntegerCanonical = out.IntegerCanonicalText()
	return out, nil
}

// ParseDecimalValue parses an xs:decimal lexical value without precomputing
// canonical strings.
func ParseDecimalValue(s string) (DecimalValue, error) {
	scan, err := scanDecimalText(s)
	if err != nil {
		return DecimalValue{}, err
	}
	intTrimStart := skipLeadingZeros(s, scan.start, scan.intEnd)
	fracTrimEnd := trimTrailingZeros(s, scan.fracStart, len(s))

	intDigits := scan.intEnd - intTrimStart
	fracDigits := fracTrimEnd - scan.fracStart
	totalDigits := intDigits + fracDigits
	if totalDigits == 0 {
		totalDigits = 1
	}

	totalDigits32, err := checkedUint32(totalDigits, msgTotalDigitsLimit)
	if err != nil {
		return DecimalValue{}, err
	}
	fracDigits32, err := checkedUint32(fracDigits, msgFractionDigitsLimit)
	if err != nil {
		return DecimalValue{}, err
	}
	out := DecimalValue{
		text:           s,
		start:          scan.start,
		intEnd:         scan.intEnd,
		intTrimStart:   intTrimStart,
		fracStart:      scan.fracStart,
		fracTrimEnd:    fracTrimEnd,
		IntegerLexical: !scan.dot,
		negative:       scan.negative && (intDigits != 0 || fracDigits != 0),
		TotalDigits:    totalDigits32,
		FractionDigits: fracDigits32,
	}
	return out, nil
}

// CanonicalText returns the xs:decimal canonical lexical value.
func (d DecimalValue) CanonicalText() string {
	if d.Canonical != "" {
		return d.Canonical
	}
	if d.text == "" {
		return decimalZeroCanonical
	}
	intDigits := d.intDigits()
	fracDigits := d.fracDigits()
	if intDigits == 0 && fracDigits == 0 {
		return decimalZeroCanonical
	}
	if intDigits > 0 && fracDigits > 0 {
		return d.canonicalTextWithBothParts(intDigits, fracDigits)
	}
	return d.canonicalTextWithZeroPart(intDigits, fracDigits)
}

func (d DecimalValue) canonicalTextWithBothParts(intDigits, fracDigits int) string {
	if !d.negative {
		return d.text[d.intTrimStart:d.fracTrimEnd]
	}
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

func (d DecimalValue) canonicalTextWithZeroPart(intDigits, fracDigits int) string {
	intPart := "0"
	if intDigits > 0 {
		intPart = d.text[d.intTrimStart:d.intEnd]
	}
	fracPart := "0"
	if fracDigits > 0 {
		fracPart = d.text[d.fracStart:d.fracTrimEnd]
	}
	var b strings.Builder
	if d.negative {
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

// IntegerCanonicalText returns the xs:integer canonical lexical value for an
// already-validated integer-family decimal value.
func (d DecimalValue) IntegerCanonicalText() string {
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

func (d DecimalValue) intDigits() int {
	return d.intEnd - d.intTrimStart
}

func (d DecimalValue) fracDigits() int {
	return d.fracTrimEnd - d.fracStart
}

// IsNegative reports whether the decimal value is less than zero.
func (d DecimalValue) IsNegative() bool {
	return d.negative
}

// CompareDecimalValues compares two xs:decimal value-space values.
func CompareDecimalValues(a, b DecimalValue) int {
	aNeg := a.IsNegative()
	bNeg := b.IsNegative()
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

func comparePositiveDecimalValues(a, b DecimalValue) int {
	if n := compareIntegerMagnitudeText(a.text[a.intTrimStart:a.intEnd], b.text[b.intTrimStart:b.intEnd]); n != 0 {
		return n
	}
	return compareDecimalFractionValues(a, b)
}

func compareDecimalFractionValues(a, b DecimalValue) int {
	return compareFraction(a.text[a.fracStart:a.fracTrimEnd], b.text[b.fracStart:b.fracTrimEnd])
}

func checkedUint32(n int, msg string) (uint32, error) {
	if n < 0 || uint64(n) > uint64(^uint32(0)) {
		return 0, errors.New(msg)
	}
	return uint32(n), nil
}

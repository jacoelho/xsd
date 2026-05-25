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

package xsd

import (
	"cmp"
	"fmt"
	"strings"
)

type decimalValue struct {
	Canonical        string
	IntegerCanonical string
	IntegerLexical   bool
	TotalDigits      uint32
	FractionDigits   uint32
}

func parseDecimal(s string) (decimalValue, error) {
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

	return decimalValue{
		Canonical:        canonicalDecimal(s, negative, intEnd, intTrimStart, fracStart, fracTrimEnd),
		IntegerCanonical: canonicalIntegerDecimal(s, negative, start, intEnd, intTrimStart),
		IntegerLexical:   dot < 0,
		TotalDigits:      uint32(totalDigits),
		FractionDigits:   uint32(fracDigits),
	}, nil
}

func canonicalDecimal(s string, negative bool, intEnd, intTrimStart, fracStart, fracTrimEnd int) string {
	intDigits := intEnd - intTrimStart
	fracDigits := fracTrimEnd - fracStart
	if intDigits == 0 && fracDigits == 0 {
		return "0.0"
	}
	intPart := "0"
	if intDigits > 0 {
		intPart = s[intTrimStart:intEnd]
	}
	fracPart := "0"
	if fracDigits > 0 {
		fracPart = s[fracStart:fracTrimEnd]
	}
	var b strings.Builder
	if negative && (intDigits != 0 || fracDigits != 0) {
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

func canonicalIntegerDecimal(s string, negative bool, start, intEnd, intTrimStart int) string {
	intDigits := intEnd - intTrimStart
	if intDigits == 0 {
		return "0"
	}
	if negative {
		if intTrimStart == start {
			return s[:intEnd]
		}
		return "-" + s[intTrimStart:intEnd]
	}
	return s[intTrimStart:intEnd]
}

func compareCanonicalDecimal(a, b string) int {
	if a == b {
		return 0
	}
	aNeg := strings.HasPrefix(a, "-")
	bNeg := strings.HasPrefix(b, "-")
	if aNeg != bNeg {
		if aNeg {
			return -1
		}
		return 1
	}
	if aNeg {
		return -comparePositiveCanonicalDecimal(strings.TrimPrefix(a, "-"), strings.TrimPrefix(b, "-"))
	}
	return comparePositiveCanonicalDecimal(a, b)
}

func comparePositiveCanonicalDecimal(a, b string) int {
	aInt, aFrac, _ := strings.Cut(a, ".")
	bInt, bFrac, _ := strings.Cut(b, ".")
	if n := cmp.Compare(len(aInt), len(bInt)); n != 0 {
		return n
	}
	if aInt != bInt {
		return cmp.Compare(aInt, bInt)
	}
	n := max(len(aFrac), len(bFrac))
	for i := range n {
		ad := byte('0')
		if i < len(aFrac) {
			ad = aFrac[i]
		}
		bd := byte('0')
		if i < len(bFrac) {
			bd = bFrac[i]
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

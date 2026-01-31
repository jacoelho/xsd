package num

// Dec represents an arbitrary-precision decimal.
type Dec struct {
	Coef  []byte
	Scale uint32
	Sign  int8
}

// ParseDec parses a decimal lexical value into a Dec.
func ParseDec(b []byte) (Dec, *ParseError) {
	dec, _, err := parseDecInto(b, nil)
	return dec, err
}

// ParseDecInto parses a decimal lexical value into a Dec using dst as scratch.
func ParseDecInto(b, dst []byte) (Dec, []byte, *ParseError) {
	return parseDecInto(b, dst)
}

func parseDecInto(b, dst []byte) (Dec, []byte, *ParseError) {
	if len(b) == 0 {
		return Dec{}, dst, &ParseError{Kind: ParseEmpty}
	}
	sign := int8(1)
	i := 0
	switch b[0] {
	case '+':
		i++
	case '-':
		sign = -1
		i++
	}
	if i >= len(b) {
		return Dec{}, dst, &ParseError{Kind: ParseNoDigits}
	}
	sawDot := false
	sawDigit := false
	dotIndex := -1
	for idx := i; idx < len(b); idx++ {
		c := b[idx]
		if isDigit(c) {
			sawDigit = true
			continue
		}
		if c == '.' {
			if sawDot {
				return Dec{}, dst, &ParseError{Kind: ParseMultipleDots}
			}
			sawDot = true
			dotIndex = idx
			continue
		}
		return Dec{}, dst, &ParseError{Kind: ParseBadChar}
	}
	if !sawDigit {
		return Dec{}, dst, &ParseError{Kind: ParseNoDigits}
	}

	var intPart, fracPart []byte
	if dotIndex == -1 {
		intPart = b[i:]
	} else {
		intPart = b[i:dotIndex]
		fracPart = b[dotIndex+1:]
	}
	scale := uint32(len(fracPart))

	var coef []byte
	if dotIndex == -1 {
		coef = intPart
	} else {
		need := len(intPart) + len(fracPart)
		if cap(dst) < need {
			dst = make([]byte, need)
		} else {
			dst = dst[:need]
		}
		copy(dst, intPart)
		copy(dst[len(intPart):], fracPart)
		coef = dst
	}

	coef = trimLeadingZeros(coef)
	for scale > 0 && len(coef) > 0 && coef[len(coef)-1] == '0' {
		coef = coef[:len(coef)-1]
		scale--
	}
	if len(coef) == 0 || allZeros(coef) {
		return Dec{Sign: 0, Coef: zeroDigits, Scale: 0}, dst, nil
	}
	return Dec{Sign: sign, Coef: coef, Scale: scale}, dst, nil
}

// Compare compares two Dec values.
func (a Dec) Compare(b Dec) int {
	if a.Sign == 0 && b.Sign == 0 {
		return 0
	}
	if a.Sign != b.Sign {
		if a.Sign < b.Sign {
			return -1
		}
		return 1
	}
	cmp := compareDecAbs(a, b)
	if a.Sign < 0 {
		return -cmp
	}
	return cmp
}

// RenderCanonical appends the canonical lexical form to dst.
func (a Dec) RenderCanonical(dst []byte) []byte {
	if a.Sign == 0 {
		return append(dst, '0', '.', '0')
	}
	if a.Sign < 0 {
		dst = append(dst, '-')
	}
	if a.Scale == 0 {
		dst = append(dst, a.Coef...)
		dst = append(dst, '.', '0')
		return dst
	}

	intLen := len(a.Coef) - int(a.Scale)
	switch {
	case intLen > 0:
		dst = append(dst, a.Coef[:intLen]...)
		dst = append(dst, '.')
		dst = append(dst, a.Coef[intLen:]...)
	case intLen == 0:
		dst = append(dst, '0', '.')
		dst = append(dst, a.Coef...)
	default:
		dst = append(dst, '0', '.')
		for i := 0; i < -intLen; i++ {
			dst = append(dst, '0')
		}
		dst = append(dst, a.Coef...)
	}
	return dst
}

func compareDecAbs(a, b Dec) int {
	intLenA := len(a.Coef) - int(a.Scale)
	intLenB := len(b.Coef) - int(b.Scale)
	if intLenA != intLenB {
		if intLenA < intLenB {
			return -1
		}
		return 1
	}

	scaleMax := max(b.Scale, a.Scale)
	padA := int(scaleMax - a.Scale)
	padB := int(scaleMax - b.Scale)
	lenA := len(a.Coef) + padA
	lenB := len(b.Coef) + padB
	if lenA != lenB {
		if lenA < lenB {
			return -1
		}
		return 1
	}

	for i := range lenA {
		da := byte('0')
		db := byte('0')
		if i < len(a.Coef) {
			da = a.Coef[i]
		}
		if i < len(b.Coef) {
			db = b.Coef[i]
		}
		if da == db {
			continue
		}
		if da < db {
			return -1
		}
		return 1
	}
	return 0
}

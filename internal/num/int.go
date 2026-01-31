package num

var zeroDigits = []byte{'0'}

// Int represents an arbitrary-precision integer.
type Int struct {
	Sign   int8
	Digits []byte
}

// ParseInt parses an integer lexical value into an Int.
func ParseInt(b []byte) (Int, *ParseError) {
	if len(b) == 0 {
		return Int{}, &ParseError{Kind: ParseEmpty}
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
		return Int{}, &ParseError{Kind: ParseNoDigits}
	}
	for _, c := range b[i:] {
		if !isDigit(c) {
			return Int{}, &ParseError{Kind: ParseBadChar}
		}
	}
	digits := trimLeadingZeros(b[i:])
	if len(digits) == 0 || allZeros(digits) {
		return Int{Sign: 0, Digits: zeroDigits}, nil
	}
	return Int{Sign: sign, Digits: digits}, nil
}

// Compare compares two Int values.
func (a Int) Compare(b Int) int {
	if a.Sign == 0 && b.Sign == 0 {
		return 0
	}
	if a.Sign != b.Sign {
		if a.Sign < b.Sign {
			return -1
		}
		return 1
	}
	cmp := compareDigits(a.Digits, b.Digits)
	if a.Sign < 0 {
		return -cmp
	}
	return cmp
}

// CompareDec compares an Int to a Dec.
func (a Int) CompareDec(b Dec) int {
	return a.AsDec().Compare(b)
}

// AsDec converts an Int to a Dec.
func (a Int) AsDec() Dec {
	if a.Sign == 0 {
		return Dec{Sign: 0, Coef: zeroDigits, Scale: 0}
	}
	return Dec{Sign: a.Sign, Coef: a.Digits, Scale: 0}
}

// RenderCanonical appends the canonical lexical form to dst.
func (a Int) RenderCanonical(dst []byte) []byte {
	if a.Sign == 0 {
		return append(dst, '0')
	}
	if a.Sign < 0 {
		dst = append(dst, '-')
	}
	return append(dst, a.Digits...)
}

func compareDigits(a, b []byte) int {
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	for i := 0; i < len(a); i++ {
		if a[i] == b[i] {
			continue
		}
		if a[i] < b[i] {
			return -1
		}
		return 1
	}
	return 0
}

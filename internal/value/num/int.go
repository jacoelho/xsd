package num

var zeroDigits = []byte{'0'}

// Int represents an arbitrary-precision integer.
type Int struct {
	Digits []byte
	Sign   int8
}

// ParseInt parses an integer lexical value into an Int.
// The returned Int may share backing storage with b.
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

// Add returns the sum of two Int values.
func Add(a, b Int) Int {
	if a.Sign == 0 {
		return b
	}
	if b.Sign == 0 {
		return a
	}
	if a.Sign == b.Sign {
		return Int{Sign: a.Sign, Digits: addDigits(a.Digits, b.Digits)}
	}
	cmp := compareDigits(a.Digits, b.Digits)
	if cmp == 0 {
		return Int{Sign: 0, Digits: zeroDigits}
	}
	if cmp > 0 {
		return Int{Sign: a.Sign, Digits: subDigits(a.Digits, b.Digits)}
	}
	return Int{Sign: b.Sign, Digits: subDigits(b.Digits, a.Digits)}
}

// Mul returns the product of two Int values.
func Mul(a, b Int) Int {
	if a.Sign == 0 || b.Sign == 0 {
		return Int{Sign: 0, Digits: zeroDigits}
	}
	sign := int8(1)
	if a.Sign != b.Sign {
		sign = -1
	}
	return Int{Sign: sign, Digits: mulDigits(a.Digits, b.Digits)}
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
	for i := range a {
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

// FromUint64 returns an Int representation of a non-negative uint64.
func FromUint64(v uint64) Int {
	if v == 0 {
		return Int{Sign: 0, Digits: zeroDigits}
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte(v%10) + '0'
		v /= 10
	}
	digits := make([]byte, len(buf)-i)
	copy(digits, buf[i:])
	return Int{Sign: 1, Digits: digits}
}

// FromInt64 returns an Int representation of an int64.
func FromInt64(v int64) Int {
	if v == 0 {
		return Int{Sign: 0, Digits: zeroDigits}
	}
	sign := int8(1)
	u := uint64(v)
	if v < 0 {
		sign = -1
		u = uint64(-(v + 1))
		u++
	}
	intVal := FromUint64(u)
	intVal.Sign = sign
	return intVal
}

func addDigits(a, b []byte) []byte {
	maxLen := max(len(b), len(a))
	out := make([]byte, maxLen+1)
	i := len(a) - 1
	j := len(b) - 1
	k := len(out) - 1
	carry := 0
	for i >= 0 || j >= 0 || carry > 0 {
		sum := carry
		if i >= 0 {
			sum += int(a[i] - '0')
			i--
		}
		if j >= 0 {
			sum += int(b[j] - '0')
			j--
		}
		out[k] = byte(sum%10) + '0'
		carry = sum / 10
		k--
	}
	return normalizeDigits(out[k+1:])
}

func subDigits(a, b []byte) []byte {
	out := make([]byte, len(a))
	i := len(a) - 1
	j := len(b) - 1
	borrow := 0
	for i >= 0 {
		diff := int(a[i]-'0') - borrow
		if j >= 0 {
			diff -= int(b[j] - '0')
			j--
		}
		if diff < 0 {
			diff += 10
			borrow = 1
		} else {
			borrow = 0
		}
		out[i] = byte(diff) + '0'
		i--
	}
	return normalizeDigits(out)
}

func mulDigits(a, b []byte) []byte {
	if len(a) == 0 || len(b) == 0 {
		return zeroDigits
	}
	if len(a) == 1 && a[0] == '0' {
		return zeroDigits
	}
	if len(b) == 1 && b[0] == '0' {
		return zeroDigits
	}
	res := make([]int, len(a)+len(b))
	for i := len(a) - 1; i >= 0; i-- {
		ai := int(a[i] - '0')
		for j := len(b) - 1; j >= 0; j-- {
			res[i+j+1] += ai * int(b[j]-'0')
		}
	}
	for k := len(res) - 1; k > 0; k-- {
		carry := res[k] / 10
		res[k] %= 10
		res[k-1] += carry
	}
	out := make([]byte, len(res))
	for i := range res {
		out[i] = byte(res[i]) + '0'
	}
	return normalizeDigits(out)
}

func normalizeDigits(d []byte) []byte {
	d = trimLeadingZeros(d)
	if len(d) == 0 {
		return zeroDigits
	}
	return d
}

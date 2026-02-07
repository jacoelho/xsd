package num

// AddDecInt adds an integer delta to a decimal value without narrowing.
func AddDecInt(dec Dec, delta Int) Dec {
	if delta.Sign == 0 {
		return dec
	}
	scale := dec.Scale
	scaled := DecToScaledInt(dec, scale)
	if scale != 0 {
		delta = Mul(delta, Pow10Int(scale))
	}
	sum := Add(scaled, delta)
	return DecFromScaledInt(sum, scale)
}

// Pow10Int returns 10^scale as an Int.
func Pow10Int(scale uint32) Int {
	if scale == 0 {
		return FromInt64(1)
	}
	digits := make([]byte, int(scale)+1)
	digits[0] = '1'
	for i := 1; i < len(digits); i++ {
		digits[i] = '0'
	}
	return Int{Sign: 1, Digits: digits}
}

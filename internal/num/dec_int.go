package num

import "fmt"

// DecToScaledInt converts a Dec into an Int scaled by 10^scale.
// It truncates fractional digits when scale < dec.Scale.
func DecToScaledInt(dec Dec, scale uint32) Int {
	if dec.Sign == 0 || len(dec.Coef) == 0 || allZeros(dec.Coef) {
		return Int{Sign: 0, Digits: zeroDigits}
	}
	digits := dec.Coef
	switch {
	case scale > dec.Scale:
		zeros := int(scale - dec.Scale)
		out := make([]byte, len(digits)+zeros)
		copy(out, digits)
		for i := len(digits); i < len(out); i++ {
			out[i] = '0'
		}
		digits = out
	case scale < dec.Scale:
		diff := int(dec.Scale - scale)
		if diff >= len(digits) {
			return Int{Sign: 0, Digits: zeroDigits}
		}
		digits = digits[:len(digits)-diff]
	}
	digits = trimLeadingZeros(digits)
	if len(digits) == 0 || allZeros(digits) {
		return Int{Sign: 0, Digits: zeroDigits}
	}
	return Int{Sign: dec.Sign, Digits: digits}
}

// DecToScaledIntExact converts a Dec into an Int scaled by 10^scale.
// It returns an error if non-zero fractional digits would be lost.
func DecToScaledIntExact(dec Dec, scale uint32) (Int, error) {
	if dec.Sign == 0 || len(dec.Coef) == 0 || allZeros(dec.Coef) {
		return Int{Sign: 0, Digits: zeroDigits}, nil
	}
	digits := dec.Coef
	if scale >= dec.Scale {
		zeros := int(scale - dec.Scale)
		out := make([]byte, len(digits)+zeros)
		copy(out, digits)
		for i := len(digits); i < len(out); i++ {
			out[i] = '0'
		}
		digits = out
		digits = trimLeadingZeros(digits)
		if len(digits) == 0 || allZeros(digits) {
			return Int{Sign: 0, Digits: zeroDigits}, nil
		}
		return Int{Sign: dec.Sign, Digits: digits}, nil
	}

	diff := int(dec.Scale - scale)
	if diff > len(digits) {
		if allZeros(digits) {
			return Int{Sign: 0, Digits: zeroDigits}, nil
		}
		return Int{}, fmt.Errorf("seconds precision exceeds %d fractional digits", scale)
	}
	tail := digits[len(digits)-diff:]
	if !allZeros(tail) {
		return Int{}, fmt.Errorf("seconds precision exceeds %d fractional digits", scale)
	}
	digits = digits[:len(digits)-diff]
	digits = trimLeadingZeros(digits)
	if len(digits) == 0 || allZeros(digits) {
		return Int{Sign: 0, Digits: zeroDigits}, nil
	}
	return Int{Sign: dec.Sign, Digits: digits}, nil
}

// DecFromScaledInt converts an Int scaled by 10^scale into a Dec.
func DecFromScaledInt(val Int, scale uint32) Dec {
	if val.Sign == 0 || len(val.Digits) == 0 {
		return Dec{}
	}
	digits := trimLeadingZeros(val.Digits)
	if len(digits) == 0 || allZeros(digits) {
		return Dec{}
	}
	coef := append([]byte(nil), digits...)
	for scale > 0 && len(coef) > 0 && coef[len(coef)-1] == '0' {
		coef = coef[:len(coef)-1]
		scale--
	}
	if len(coef) == 0 || allZeros(coef) {
		return Dec{}
	}
	return Dec{Sign: val.Sign, Coef: coef, Scale: scale}
}

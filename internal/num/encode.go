package num

import "encoding/binary"

// EncodeDecKey appends the canonical decimal key encoding to dst.
func EncodeDecKey(dst []byte, v Dec) []byte {
	sign := byte(1)
	coef := v.Coef
	scale := v.Scale
	switch {
	case v.Sign < 0:
		sign = 2
	case v.Sign == 0:
		sign = 0
		coef = zeroDigits
		scale = 0
	}
	dst = append(dst, sign)
	dst = appendUvarint(dst, uint64(scale))
	dst = appendUvarint(dst, uint64(len(coef)))
	dst = append(dst, coef...)
	return dst
}

// EncodeIntKey appends the canonical integer key encoding to dst.
func EncodeIntKey(dst []byte, v Int) []byte {
	return EncodeDecKey(dst, v.AsDec())
}

func appendUvarint(dst []byte, v uint64) []byte {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], v)
	return append(dst, buf[:n]...)
}

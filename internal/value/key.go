package value

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

const (
	canonicalNaN32 = 0x7fc00000
	canonicalNaN64 = 0x7ff8000000000000
)

// CanonicalDecimalKey returns deterministic value-space bytes for a decimal lexical value.
func CanonicalDecimalKey(lexical, dst []byte) ([]byte, error) {
	trimmed := TrimXMLWhitespace(lexical)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("invalid decimal: empty string")
	}
	if !isValidDecimalLexical(trimmed) {
		return nil, fmt.Errorf("invalid decimal: %s", string(trimmed))
	}

	sign := byte(1)
	switch trimmed[0] {
	case '+':
		trimmed = trimmed[1:]
	case '-':
		sign = 2
		trimmed = trimmed[1:]
	}
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("invalid decimal: empty string")
	}

	intPart := trimmed
	fracPart := []byte{}
	if dot := indexByte(trimmed, '.'); dot >= 0 {
		intPart = trimmed[:dot]
		fracPart = trimmed[dot+1:]
	}

	coeff := make([]byte, 0, len(intPart)+len(fracPart))
	coeff = append(coeff, intPart...)
	coeff = append(coeff, fracPart...)
	coeff = trimLeftZeros(coeff)

	scale := len(fracPart)
	for scale > 0 && len(coeff) > 0 && coeff[len(coeff)-1] == '0' {
		coeff = coeff[:len(coeff)-1]
		scale--
	}
	if len(coeff) == 0 || allZeros(coeff) {
		sign = 0
		scale = 0
		coeff = nil
	}

	out := dst[:0]
	if cap(out) < 1+binary.MaxVarintLen64+len(coeff) {
		out = make([]byte, 0, 1+binary.MaxVarintLen64+len(coeff))
	}
	out = append(out, sign)
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(scale))
	out = append(out, buf[:n]...)
	out = append(out, coeff...)
	return out, nil
}

// CanonicalDecimalKeyFromCanonical returns deterministic value-space bytes for a canonical decimal lexical value.
// The input must be in canonical lexical form (see CanonicalDecimalBytes).
func CanonicalDecimalKeyFromCanonical(canonical, dst []byte) ([]byte, error) {
	if len(canonical) == 0 {
		return nil, fmt.Errorf("invalid decimal: empty string")
	}
	sign := byte(1)
	switch canonical[0] {
	case '+':
		canonical = canonical[1:]
	case '-':
		sign = 2
		canonical = canonical[1:]
	}
	if len(canonical) == 0 {
		return nil, fmt.Errorf("invalid decimal: empty string")
	}

	dot := indexByte(canonical, '.')
	if dot < 0 {
		return nil, fmt.Errorf("invalid decimal: missing decimal point")
	}
	intPart := canonical[:dot]
	fracPart := canonical[dot+1:]
	if len(intPart) == 0 || len(fracPart) == 0 {
		return nil, fmt.Errorf("invalid decimal: missing digits")
	}

	scale := len(fracPart)
	var coeff []byte
	if allZeros(fracPart) {
		scale = 0
		coeff = intPart
	} else {
		coeff = make([]byte, 0, len(intPart)+len(fracPart))
		coeff = append(coeff, intPart...)
		coeff = append(coeff, fracPart...)
	}
	coeff = trimLeftZeros(coeff)
	if len(coeff) == 0 || allZeros(coeff) {
		sign = 0
		scale = 0
		coeff = nil
	}

	out := dst[:0]
	if cap(out) < 1+binary.MaxVarintLen64+len(coeff) {
		out = make([]byte, 0, 1+binary.MaxVarintLen64+len(coeff))
	}
	out = append(out, sign)
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(scale))
	out = append(out, buf[:n]...)
	out = append(out, coeff...)
	return out, nil
}

// CanonicalIntegerKeyFromCanonical returns deterministic value-space bytes for a canonical integer lexical value.
// The input must be in canonical lexical form (no leading zeros except zero itself).
func CanonicalIntegerKeyFromCanonical(canonical, dst []byte) ([]byte, error) {
	if len(canonical) == 0 {
		return nil, fmt.Errorf("invalid integer: empty string")
	}
	sign := byte(1)
	switch canonical[0] {
	case '+':
		canonical = canonical[1:]
	case '-':
		sign = 2
		canonical = canonical[1:]
	}
	if len(canonical) == 0 {
		return nil, fmt.Errorf("invalid integer: empty string")
	}
	digits := trimLeftZeros(canonical)
	if len(digits) == 0 || allZeros(digits) {
		sign = 0
		digits = nil
	}
	out := dst[:0]
	if cap(out) < 1+binary.MaxVarintLen64+len(digits) {
		out = make([]byte, 0, 1+binary.MaxVarintLen64+len(digits))
	}
	out = append(out, sign)
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], 0)
	out = append(out, buf[:n]...)
	out = append(out, digits...)
	return out, nil
}

// CanonicalFloatKey returns deterministic value-space bytes for float/double lexical values.
func CanonicalFloatKey(lexical []byte, bits int, dst []byte) ([]byte, error) {
	switch bits {
	case 32:
		v, err := ParseFloat(lexical)
		if err != nil {
			return nil, err
		}
		if v == 0 {
			return appendFloat32(dst, 0), nil
		}
		if math.IsNaN(float64(v)) {
			return appendFloat32(dst, canonicalNaN32), nil
		}
		return appendFloat32(dst, math.Float32bits(v)), nil
	case 64:
		v, err := ParseDouble(lexical)
		if err != nil {
			return nil, err
		}
		if v == 0 {
			return appendFloat64(dst, 0), nil
		}
		if math.IsNaN(v) {
			return appendFloat64(dst, canonicalNaN64), nil
		}
		return appendFloat64(dst, math.Float64bits(v)), nil
	default:
		return nil, fmt.Errorf("unsupported float bits %d", bits)
	}
}

// CanonicalFloat32Key returns deterministic value-space bytes for a float32 value.
func CanonicalFloat32Key(value float32, dst []byte) []byte {
	if value == 0 {
		return appendFloat32(dst, 0)
	}
	if math.IsNaN(float64(value)) {
		return appendFloat32(dst, canonicalNaN32)
	}
	return appendFloat32(dst, math.Float32bits(value))
}

// CanonicalFloat64Key returns deterministic value-space bytes for a float64 value.
func CanonicalFloat64Key(value float64, dst []byte) []byte {
	if value == 0 {
		return appendFloat64(dst, 0)
	}
	if math.IsNaN(value) {
		return appendFloat64(dst, canonicalNaN64)
	}
	return appendFloat64(dst, math.Float64bits(value))
}

// CanonicalTemporalKey returns deterministic value-space bytes for temporal values.
// Values with timezone are normalized to UTC before formatting.
func CanonicalTemporalKey(value time.Time, kind string, hasTZ bool, dst []byte) []byte {
	if hasTZ {
		value = value.UTC()
	}
	out := dst[:0]
	canon := CanonicalDateTimeString(value, kind, hasTZ)
	out = append(out, canon...)
	return out
}

func appendFloat32(dst []byte, bits uint32) []byte {
	out := dst[:0]
	if cap(out) < 4 {
		out = make([]byte, 0, 4)
	}
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], bits)
	out = append(out, buf[:]...)
	return out
}

func appendFloat64(dst []byte, bits uint64) []byte {
	out := dst[:0]
	if cap(out) < 8 {
		out = make([]byte, 0, 8)
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], bits)
	out = append(out, buf[:]...)
	return out
}

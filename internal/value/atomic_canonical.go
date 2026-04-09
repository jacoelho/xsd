package value

import (
	"encoding/base64"
	"fmt"
	"unsafe"

	"github.com/jacoelho/xsd/internal/value/num"
)

// CanonicalizeString validates and returns the canonical lexical form for string-family values.
func CanonicalizeString(normalized []byte, validate func([]byte) error) ([]byte, error) {
	if validate != nil {
		if err := validate(normalized); err != nil {
			return nil, err
		}
	}
	return normalized, nil
}

// CanonicalizeBoolean parses and canonicalizes XSD boolean lexical values.
func CanonicalizeBoolean(normalized []byte) (bool, []byte, error) {
	v, err := ParseBoolean(normalized)
	if err != nil {
		return false, nil, err
	}
	if v {
		return true, []byte("true"), nil
	}
	return false, []byte("false"), nil
}

// CanonicalizeDecimal parses and canonicalizes XSD decimal lexical values.
func CanonicalizeDecimal(normalized []byte) (num.Dec, []byte, error) {
	dec, perr := num.ParseDec(normalized)
	if perr != nil {
		return num.Dec{}, nil, fmt.Errorf("invalid decimal")
	}
	return dec, dec.RenderCanonical(nil), nil
}

// CanonicalizeInteger parses and canonicalizes XSD integer lexical values.
func CanonicalizeInteger(normalized []byte, validate func(num.Int) error) (num.Int, []byte, error) {
	intVal, perr := num.ParseInt(normalized)
	if perr != nil {
		return num.Int{}, nil, fmt.Errorf("invalid integer")
	}
	if validate != nil {
		if err := validate(intVal); err != nil {
			return num.Int{}, nil, err
		}
	}
	return intVal, intVal.RenderCanonical(nil), nil
}

// CanonicalizeFloat32 parses and canonicalizes XSD float lexical values.
func CanonicalizeFloat32(normalized []byte) (float32, num.FloatClass, []byte, error) {
	v, class, perr := num.ParseFloat32(normalized)
	if perr != nil {
		return 0, num.FloatFinite, nil, fmt.Errorf("invalid float")
	}
	return v, class, []byte(CanonicalFloat(float64(v), 32)), nil
}

// CanonicalizeFloat64 parses and canonicalizes XSD double lexical values.
func CanonicalizeFloat64(normalized []byte) (float64, num.FloatClass, []byte, error) {
	v, class, perr := num.ParseFloat(normalized, 64)
	if perr != nil {
		return 0, num.FloatFinite, nil, fmt.Errorf("invalid double")
	}
	return v, class, []byte(CanonicalFloat(v, 64)), nil
}

// CanonicalizeDuration parses and canonicalizes XSD duration lexical values.
func CanonicalizeDuration(normalized []byte) (Duration, []byte, error) {
	dur, err := ParseDuration(unsafe.String(unsafe.SliceData(normalized), len(normalized)))
	if err != nil {
		return Duration{}, nil, err
	}
	return dur, []byte(CanonicalDurationString(dur)), nil
}

// CanonicalizeAnyURI validates and returns the canonical lexical form for xs:anyURI.
func CanonicalizeAnyURI(normalized []byte) ([]byte, error) {
	if err := ValidateAnyURI(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

// CanonicalizeHexBinary parses and canonicalizes xs:hexBinary.
func CanonicalizeHexBinary(normalized []byte) ([]byte, error) {
	decoded, err := ParseHexBinary(normalized)
	if err != nil {
		return nil, err
	}
	return UpperHex(nil, decoded), nil
}

// CanonicalizeBase64Binary parses and canonicalizes xs:base64Binary.
func CanonicalizeBase64Binary(normalized []byte) ([]byte, error) {
	decoded, err := ParseBase64Binary(normalized)
	if err != nil {
		return nil, err
	}
	canonical := make([]byte, base64.StdEncoding.EncodedLen(len(decoded)))
	base64.StdEncoding.Encode(canonical, decoded)
	return canonical, nil
}

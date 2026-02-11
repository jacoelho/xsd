package valuesemantics

import (
	"fmt"
	"unsafe"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/value"
)

// CanonicalizeBoolean parses and canonicalizes XSD boolean lexical values.
func CanonicalizeBoolean(normalized []byte) (bool, []byte, error) {
	v, err := value.ParseBoolean(normalized)
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
	return v, class, []byte(value.CanonicalFloat(float64(v), 32)), nil
}

// CanonicalizeFloat64 parses and canonicalizes XSD double lexical values.
func CanonicalizeFloat64(normalized []byte) (float64, num.FloatClass, []byte, error) {
	v, class, perr := num.ParseFloat(normalized, 64)
	if perr != nil {
		return 0, num.FloatFinite, nil, fmt.Errorf("invalid double")
	}
	return v, class, []byte(value.CanonicalFloat(v, 64)), nil
}

// CanonicalizeDuration parses and canonicalizes XSD duration lexical values.
func CanonicalizeDuration(normalized []byte) (durationlex.Duration, []byte, error) {
	dur, err := durationlex.Parse(unsafe.String(unsafe.SliceData(normalized), len(normalized)))
	if err != nil {
		return durationlex.Duration{}, nil, err
	}
	return dur, []byte(durationlex.CanonicalString(dur)), nil
}

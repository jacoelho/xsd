package validator

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) fixedValueMatches(
	validator runtime.ValidatorID,
	member runtime.ValidatorID,
	canonical []byte,
	metrics valueMetrics,
	resolver value.NSResolver,
	fixed runtime.ValueRef,
	fixedKey runtime.ValueKeyRef,
) (bool, error) {
	if fixedKey.Ref.Present {
		actualKind := metrics.keyKind
		actualKey := metrics.keyBytes
		if actualKind == runtime.VKInvalid {
			kind, key, err := s.keyForCanonicalValue(validator, canonical, resolver, member)
			if err != nil {
				return false, err
			}
			actualKind = kind
			actualKey = key
		}
		return actualKind == fixedKey.Kind && bytes.Equal(actualKey, valueBytes(s.rt.Values, fixedKey.Ref)), nil
	}
	return bytes.Equal(canonical, valueBytes(s.rt.Values, fixed)), nil
}

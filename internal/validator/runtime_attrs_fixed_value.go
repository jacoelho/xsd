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
	metrics ValueMetrics,
	resolver value.NSResolver,
	fixed runtime.ValueRef,
	fixedKey runtime.ValueKeyRef,
) (bool, error) {
	if fixedKey.Ref.Present {
		actualKind, actualKey, err := s.materializeObservedKey(validator, canonical, resolver, member, metrics)
		if err != nil {
			return false, err
		}
		return actualKind == fixedKey.Kind && bytes.Equal(actualKey, valueBytes(s.rt.Values, fixedKey.Ref)), nil
	}
	return bytes.Equal(canonical, valueBytes(s.rt.Values, fixed)), nil
}

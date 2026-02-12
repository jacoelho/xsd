package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) materializePolicyKey(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID, stored runtime.ValueKeyRef) (runtime.ValueKind, []byte, error) {
	kind := stored.Kind
	key := valueBytes(s.rt.Values, stored.Ref)
	if stored.Ref.Present {
		return kind, key, nil
	}
	return s.keyForCanonicalValue(validator, canonical, nil, member)
}

func (s *Session) materializeObservedKey(
	validator runtime.ValidatorID,
	canonical []byte,
	resolver value.NSResolver,
	member runtime.ValidatorID,
	metrics valueMetrics,
) (runtime.ValueKind, []byte, error) {
	if metrics.keyKind != runtime.VKInvalid {
		return metrics.keyKind, metrics.keyBytes, nil
	}
	return s.keyForCanonicalValue(validator, canonical, resolver, member)
}

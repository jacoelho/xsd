package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value/temporal"
	"github.com/jacoelho/xsd/internal/valuekey"
)

func (s *Session) canonicalizeTemporal(kind runtime.ValidatorKind, normalized []byte, needKey bool, metrics *valueMetrics) ([]byte, error) {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return nil, valueErrorf(valueErrInvalid, "unsupported temporal kind %d", kind)
	}
	tv, err := temporal.Parse(spec.Kind, normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	canon := []byte(temporal.Canonical(tv))
	if needKey {
		key := valuekey.TemporalKeyBytes(s.keyTmp[:0], spec.KeyTag, tv.Time, temporal.ValueTimezoneKind(tv.TimezoneKind), tv.LeapSecond)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKDateTime, key, false)
	}
	return canon, nil
}

func validateTemporalNoCanonical(kind runtime.ValidatorKind, normalized []byte) error {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return valueErrorf(valueErrInvalid, "unsupported temporal kind %d", kind)
	}
	if _, err := temporal.Parse(spec.Kind, normalized); err != nil {
		return valueErrorMsg(valueErrInvalid, err.Error())
	}
	return nil
}

func temporalSubkind(kind runtime.ValidatorKind) byte {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return 0
	}
	return spec.KeyTag
}

func parseTemporalForKind(kind runtime.ValidatorKind, lexical []byte) (temporal.Value, error) {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return temporal.Value{}, valueErrorf(valueErrInvalid, "unsupported temporal kind %d", kind)
	}
	return temporal.Parse(spec.Kind, lexical)
}

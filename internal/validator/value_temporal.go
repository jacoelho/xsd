package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value/temporal"
	"github.com/jacoelho/xsd/internal/valuekey"
)

type temporalSpec struct {
	kind   temporal.Kind
	keyTag byte
}

var temporalSpecs = [...]temporalSpec{
	runtime.VDateTime:   {kind: temporal.KindDateTime, keyTag: 0},
	runtime.VTime:       {kind: temporal.KindTime, keyTag: 2},
	runtime.VDate:       {kind: temporal.KindDate, keyTag: 1},
	runtime.VGYearMonth: {kind: temporal.KindGYearMonth, keyTag: 3},
	runtime.VGYear:      {kind: temporal.KindGYear, keyTag: 4},
	runtime.VGMonthDay:  {kind: temporal.KindGMonthDay, keyTag: 5},
	runtime.VGDay:       {kind: temporal.KindGDay, keyTag: 6},
	runtime.VGMonth:     {kind: temporal.KindGMonth, keyTag: 7},
}

func temporalSpecFor(kind runtime.ValidatorKind) (temporalSpec, bool) {
	if int(kind) >= len(temporalSpecs) {
		return temporalSpec{}, false
	}
	spec := temporalSpecs[kind]
	if spec.kind == temporal.KindInvalid {
		return temporalSpec{}, false
	}
	return spec, true
}

func (s *Session) canonicalizeTemporal(kind runtime.ValidatorKind, normalized []byte, needKey bool, metrics *valueMetrics) ([]byte, error) {
	spec, ok := temporalSpecFor(kind)
	if !ok {
		return nil, valueErrorf(valueErrInvalid, "unsupported temporal kind %d", kind)
	}
	tv, err := temporal.Parse(spec.kind, normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	canon := []byte(temporal.Canonical(tv))
	if needKey {
		key := valuekey.TemporalKeyBytes(s.keyTmp[:0], spec.keyTag, tv.Time, temporal.ValueTimezoneKind(tv.TimezoneKind), tv.LeapSecond)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKDateTime, key, false)
	}
	return canon, nil
}

func validateTemporalNoCanonical(kind runtime.ValidatorKind, normalized []byte) error {
	spec, ok := temporalSpecFor(kind)
	if !ok {
		return valueErrorf(valueErrInvalid, "unsupported temporal kind %d", kind)
	}
	if _, err := temporal.Parse(spec.kind, normalized); err != nil {
		return valueErrorMsg(valueErrInvalid, err.Error())
	}
	return nil
}

func temporalSubkind(kind runtime.ValidatorKind) byte {
	spec, ok := temporalSpecFor(kind)
	if !ok {
		return 0
	}
	return spec.keyTag
}

func parseTemporalForKind(kind runtime.ValidatorKind, lexical []byte) (temporal.Value, error) {
	spec, ok := temporalSpecFor(kind)
	if !ok {
		return temporal.Value{}, valueErrorf(valueErrInvalid, "unsupported temporal kind %d", kind)
	}
	return temporal.Parse(spec.kind, lexical)
}

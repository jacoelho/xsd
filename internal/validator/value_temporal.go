package validator

import (
	"time"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuekey"
)

type temporalSpec struct {
	parse  func([]byte) (time.Time, error)
	kind   string
	keyTag byte
}

var temporalSpecs = [...]temporalSpec{
	runtime.VDateTime:   {value.ParseDateTime, "dateTime", 0},
	runtime.VTime:       {value.ParseTime, "time", 2},
	runtime.VDate:       {value.ParseDate, "date", 1},
	runtime.VGYearMonth: {value.ParseGYearMonth, "gYearMonth", 3},
	runtime.VGYear:      {value.ParseGYear, "gYear", 4},
	runtime.VGMonthDay:  {value.ParseGMonthDay, "gMonthDay", 5},
	runtime.VGDay:       {value.ParseGDay, "gDay", 6},
	runtime.VGMonth:     {value.ParseGMonth, "gMonth", 7},
}

func temporalSpecFor(kind runtime.ValidatorKind) (temporalSpec, bool) {
	if int(kind) >= len(temporalSpecs) {
		return temporalSpec{}, false
	}
	spec := temporalSpecs[kind]
	if spec.parse == nil {
		return temporalSpec{}, false
	}
	return spec, true
}

func (s *Session) canonicalizeTemporal(kind runtime.ValidatorKind, normalized []byte, needKey bool, metrics *valueMetrics) ([]byte, error) {
	spec, ok := temporalSpecFor(kind)
	if !ok {
		return nil, valueErrorf(valueErrInvalid, "unsupported temporal kind %d", kind)
	}
	t, err := spec.parse(normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	hasTZ := value.HasTimezone(normalized)
	canon := []byte(value.CanonicalDateTimeString(t, spec.kind, hasTZ))
	if needKey {
		key := valuekey.TemporalKeyBytes(s.keyTmp[:0], spec.keyTag, t, hasTZ)
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
	if _, err := spec.parse(normalized); err != nil {
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

func parseTemporalForKind(kind runtime.ValidatorKind, lexical []byte) (time.Time, bool, error) {
	spec, ok := temporalSpecFor(kind)
	if !ok {
		return time.Time{}, false, valueErrorf(valueErrInvalid, "unsupported temporal kind %d", kind)
	}
	t, err := spec.parse(lexical)
	return t, value.HasTimezone(lexical), err
}

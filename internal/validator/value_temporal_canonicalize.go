package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) canonicalizeTemporal(kind runtime.ValidatorKind, normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return nil, xsderrors.Invalidf("unsupported temporal kind %d", kind)
	}
	tv, err := value.Parse(spec.Kind, normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	canonical := []byte(value.Canonical(tv))
	if needKey && s != nil {
		key := runtime.TemporalKeyBytes(s.keyTmp[:0], spec.KeyTag, tv.Time, tv.TimezoneKind, tv.LeapSecond)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKDateTime, key, false)
	}
	return canonical, nil
}

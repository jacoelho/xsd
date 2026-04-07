package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) canonicalizeAtomic(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	switch meta.Kind {
	case runtime.VString:
		return s.canonicalizeAtomicString(meta, normalized, needKey, metrics)
	case runtime.VBoolean:
		return s.canonicalizeAtomicBoolean(normalized, needKey, metrics)
	case runtime.VDecimal:
		return s.canonicalizeAtomicDecimal(normalized, needKey, metrics)
	case runtime.VInteger:
		return s.canonicalizeAtomicInteger(meta, normalized, needKey, metrics)
	case runtime.VFloat:
		return s.canonicalizeAtomicFloat(normalized, needKey, metrics)
	case runtime.VDouble:
		return s.canonicalizeAtomicDouble(normalized, needKey, metrics)
	case runtime.VDuration:
		return s.canonicalizeAtomicDuration(normalized, needKey, metrics)
	default:
		return nil, xsderrors.Invalidf("unsupported atomic kind %d", meta.Kind)
	}
}

func (s *Session) canonicalizeAtomicString(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	kind, ok := s.stringKind(meta)
	if !ok {
		return nil, xsderrors.Invalid("string validator out of range")
	}
	if err := runtime.ValidateStringKind(kind, normalized); err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKString, runtime.StringKeyBytes(s.keyTmp[:0], 0, normalized))
	}
	return normalized, nil
}

func (s *Session) canonicalizeAtomicBoolean(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	v, canonical, err := value.CanonicalizeBoolean(normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if needKey && s != nil {
		key := s.keyTmp[:0]
		key = append(key, 0)
		if v {
			key[0] = 1
		}
		s.setAtomicKey(metrics, runtime.VKBool, key)
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicDecimal(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	buf1 := []byte(nil)
	buf2 := []byte(nil)
	if s != nil {
		buf1 = s.Scratch.Buf1
		buf2 = s.Scratch.Buf2[:0]
	}

	dec, parsedBuf, parseErr := num.ParseDecInto(normalized, buf1)
	if parseErr != nil {
		return nil, xsderrors.Invalid("invalid decimal")
	}
	if s != nil {
		s.Scratch.Buf1 = parsedBuf
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetDecimal(dec)
	}

	canonical := dec.RenderCanonical(buf2)
	if s != nil {
		s.Scratch.Buf2 = canonical
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKDecimal, num.EncodeDecKey(s.keyTmp[:0], dec))
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicInteger(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	kind, ok := s.integerKind(meta)
	if !ok {
		return nil, xsderrors.Invalid("integer validator out of range")
	}

	intVal, parseErr := num.ParseInt(normalized)
	if parseErr != nil {
		return nil, xsderrors.Invalid("invalid integer")
	}
	if err := runtime.ValidateIntegerKind(kind, intVal); err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetInteger(intVal)
	}

	buf2 := []byte(nil)
	if s != nil {
		buf2 = s.Scratch.Buf2[:0]
	}
	canonical := intVal.RenderCanonical(buf2)
	if s != nil {
		s.Scratch.Buf2 = canonical
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKDecimal, num.EncodeDecKey(s.keyTmp[:0], intVal.AsDec()))
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicFloat(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	v, class, canonical, err := value.CanonicalizeFloat32(normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetFloat32(v, class)
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKFloat32, runtime.Float32Key(s.keyTmp[:0], v, class))
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicDouble(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	v, class, canonical, err := value.CanonicalizeFloat64(normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetFloat64(v, class)
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKFloat64, runtime.Float64Key(s.keyTmp[:0], v, class))
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicDuration(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	dur, canonical, err := value.CanonicalizeDuration(normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKDuration, runtime.DurationKeyBytes(s.keyTmp[:0], dur))
	}
	return canonical, nil
}

func (s *Session) setAtomicKey(metrics *ValueMetrics, kind runtime.ValueKind, key []byte) {
	if s == nil {
		return
	}
	s.keyTmp = key
	s.setKey(metrics, kind, key, false)
}

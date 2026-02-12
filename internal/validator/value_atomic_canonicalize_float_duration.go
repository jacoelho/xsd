package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/valuecodec"
	"github.com/jacoelho/xsd/internal/valuesemantics"
)

func (s *Session) canonicalizeAtomicFloat(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	v, class, canon, err := valuesemantics.CanonicalizeFloat32(normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if metrics != nil {
		metrics.float32Val = v
		metrics.float32Class = class
		metrics.float32Set = true
	}
	if needKey {
		key := valuecodec.Float32Key(s.keyTmp[:0], v, class)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKFloat32, key, false)
	}
	return canon, nil
}

func (s *Session) canonicalizeAtomicDouble(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	v, class, canon, err := valuesemantics.CanonicalizeFloat64(normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if metrics != nil {
		metrics.float64Val = v
		metrics.float64Class = class
		metrics.float64Set = true
	}
	if needKey {
		key := valuecodec.Float64Key(s.keyTmp[:0], v, class)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKFloat64, key, false)
	}
	return canon, nil
}

func (s *Session) canonicalizeAtomicDuration(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	dur, canon, err := valuesemantics.CanonicalizeDuration(normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if needKey {
		key := valuecodec.DurationKeyBytes(s.keyTmp[:0], dur)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKDuration, key, false)
	}
	return canon, nil
}

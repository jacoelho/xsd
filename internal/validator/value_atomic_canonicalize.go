package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
)

func (s *Session) canonicalizeAtomic(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *valruntime.State) ([]byte, error) {
	result, bufs, err := valruntime.Atomic(meta, normalized, needKey, s.canonicalKinds(), s.canonicalBuffers(), metrics.MeasureCache())
	s.restoreCanonicalBuffers(bufs)
	if err != nil {
		return nil, err
	}
	return s.finishCanonicalResult(metrics, result), nil
}

func (s *Session) canonicalizeTemporal(kind runtime.ValidatorKind, normalized []byte, needKey bool, metrics *valruntime.State) ([]byte, error) {
	result, bufs, err := valruntime.Temporal(kind, normalized, needKey, s.canonicalBuffers())
	s.restoreCanonicalBuffers(bufs)
	if err != nil {
		return nil, err
	}
	return s.finishCanonicalResult(metrics, result), nil
}

func (s *Session) canonicalizeAnyURI(normalized []byte, needKey bool, metrics *valruntime.State) ([]byte, error) {
	result, bufs, err := valruntime.AnyURI(normalized, needKey, s.canonicalBuffers())
	s.restoreCanonicalBuffers(bufs)
	if err != nil {
		return nil, err
	}
	return s.finishCanonicalResult(metrics, result), nil
}

func (s *Session) canonicalizeHexBinary(normalized []byte, needKey bool, metrics *valruntime.State) ([]byte, error) {
	result, bufs, err := valruntime.HexBinary(normalized, needKey, s.canonicalBuffers(), metrics.MeasureCache())
	s.restoreCanonicalBuffers(bufs)
	if err != nil {
		return nil, err
	}
	return s.finishCanonicalResult(metrics, result), nil
}

func (s *Session) canonicalizeBase64Binary(normalized []byte, needKey bool, metrics *valruntime.State) ([]byte, error) {
	result, bufs, err := valruntime.Base64Binary(normalized, needKey, s.canonicalBuffers(), metrics.MeasureCache())
	s.restoreCanonicalBuffers(bufs)
	if err != nil {
		return nil, err
	}
	return s.finishCanonicalResult(metrics, result), nil
}

func (s *Session) canonicalKinds() valruntime.KindLoader {
	return valruntime.KindLoader{
		StringKind:  s.stringKind,
		IntegerKind: s.integerKind,
	}
}

func (s *Session) canonicalBuffers() valruntime.CanonicalBuffers {
	if s == nil {
		return valruntime.CanonicalBuffers{}
	}
	return valruntime.CanonicalBuffers{
		Buf1:  s.Scratch.Buf1,
		Buf2:  s.Scratch.Buf2,
		Value: s.valueScratch,
		Key:   s.keyTmp,
	}
}

func (s *Session) restoreCanonicalBuffers(bufs valruntime.CanonicalBuffers) {
	if s == nil {
		return
	}
	s.Scratch.Buf1 = bufs.Buf1
	s.Scratch.Buf2 = bufs.Buf2
	s.valueScratch = bufs.Value
	s.keyTmp = bufs.Key
}

func (s *Session) finishCanonicalResult(metrics *valruntime.State, result valruntime.CanonicalResult) []byte {
	if result.HasKey() {
		s.setKey(metrics, result.KeyKind, result.Key, false)
	}
	return result.Canonical
}

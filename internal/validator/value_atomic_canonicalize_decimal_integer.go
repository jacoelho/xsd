package validator

import (
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) canonicalizeAtomicDecimal(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	dec, buf, perr := num.ParseDecInto(normalized, s.Scratch.Buf1)
	if perr != nil {
		return nil, valueErrorMsg(valueErrInvalid, "invalid decimal")
	}
	s.Scratch.Buf1 = buf
	if metrics != nil {
		metrics.decVal = dec
		metrics.decSet = true
		metrics.totalDigits = len(dec.Coef)
		metrics.fractionDigits = int(dec.Scale)
		metrics.digitsSet = true
	}
	canonRaw := dec.RenderCanonical(s.Scratch.Buf2[:0])
	s.Scratch.Buf2 = canonRaw
	canon := canonRaw
	if needKey {
		key := num.EncodeDecKey(s.keyTmp[:0], dec)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKDecimal, key, false)
	}
	return canon, nil
}

func (s *Session) canonicalizeAtomicInteger(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	kind, ok := s.integerKind(meta)
	if !ok {
		return nil, valueErrorf(valueErrInvalid, "integer validator out of range")
	}
	intVal, perr := num.ParseInt(normalized)
	if perr != nil {
		return nil, valueErrorMsg(valueErrInvalid, "invalid integer")
	}
	if err := runtime.ValidateIntegerKind(kind, intVal); err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if metrics != nil {
		metrics.intVal = intVal
		metrics.intSet = true
		metrics.totalDigits = len(intVal.Digits)
		metrics.fractionDigits = 0
		metrics.digitsSet = true
	}
	canonRaw := intVal.RenderCanonical(s.Scratch.Buf2[:0])
	s.Scratch.Buf2 = canonRaw
	canon := canonRaw
	if needKey {
		key := num.EncodeDecKey(s.keyTmp[:0], intVal.AsDec())
		s.keyTmp = key
		s.setKey(metrics, runtime.VKDecimal, key, false)
	}
	return canon, nil
}

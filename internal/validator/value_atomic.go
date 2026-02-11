package validator

import (
	"unsafe"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuecodec"
	"github.com/jacoelho/xsd/internal/valuesemantics"
)

func (s *Session) canonicalizeAtomic(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *valueMetrics) ([]byte, error) {
	switch meta.Kind {
	case runtime.VString:
		kind, ok := s.stringKind(meta)
		if !ok {
			return nil, valueErrorf(valueErrInvalid, "string validator out of range")
		}
		if err := runtime.ValidateStringKind(kind, normalized); err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		canon := normalized
		if needKey {
			key := valuecodec.StringKeyBytes(s.keyTmp[:0], 0, canon)
			s.keyTmp = key
			s.setKey(metrics, runtime.VKString, key, false)
		}
		return canon, nil
	case runtime.VBoolean:
		v, canon, err := valuesemantics.CanonicalizeBoolean(normalized)
		if err != nil {
			return nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		if needKey {
			key := byte(0)
			if v {
				key = 1
			}
			s.setKey(metrics, runtime.VKBool, []byte{key}, false)
		}
		return canon, nil
	case runtime.VDecimal:
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
	case runtime.VInteger:
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
	case runtime.VFloat:
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
	case runtime.VDouble:
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
	case runtime.VDuration:
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
	default:
		return nil, valueErrorf(valueErrInvalid, "unsupported atomic kind %d", meta.Kind)
	}
}

func (s *Session) validateAtomicNoCanonical(meta runtime.ValidatorMeta, normalized []byte) error {
	switch meta.Kind {
	case runtime.VString:
		kind, ok := s.stringKind(meta)
		if !ok {
			return valueErrorf(valueErrInvalid, "string validator out of range")
		}
		if err := runtime.ValidateStringKind(kind, normalized); err != nil {
			return valueErrorMsg(valueErrInvalid, err.Error())
		}
	case runtime.VBoolean:
		if _, err := value.ParseBoolean(normalized); err != nil {
			return valueErrorMsg(valueErrInvalid, err.Error())
		}
	case runtime.VDecimal:
		if _, perr := num.ParseDec(normalized); perr != nil {
			return valueErrorMsg(valueErrInvalid, "invalid decimal")
		}
	case runtime.VInteger:
		kind, ok := s.integerKind(meta)
		if !ok {
			return valueErrorf(valueErrInvalid, "integer validator out of range")
		}
		intVal, perr := num.ParseInt(normalized)
		if perr != nil {
			return valueErrorMsg(valueErrInvalid, "invalid integer")
		}
		if err := runtime.ValidateIntegerKind(kind, intVal); err != nil {
			return valueErrorMsg(valueErrInvalid, err.Error())
		}
	case runtime.VFloat:
		if perr := num.ValidateFloatLexical(normalized); perr != nil {
			return valueErrorMsg(valueErrInvalid, "invalid float")
		}
	case runtime.VDouble:
		if perr := num.ValidateFloatLexical(normalized); perr != nil {
			return valueErrorMsg(valueErrInvalid, "invalid double")
		}
	case runtime.VDuration:
		if _, err := durationlex.Parse(unsafe.String(unsafe.SliceData(normalized), len(normalized))); err != nil {
			return valueErrorMsg(valueErrInvalid, err.Error())
		}
	default:
		return valueErrorf(valueErrInvalid, "unsupported atomic kind %d", meta.Kind)
	}
	return nil
}

func (s *Session) stringKind(meta runtime.ValidatorMeta) (runtime.StringKind, bool) {
	if int(meta.Index) >= len(s.rt.Validators.String) {
		return runtime.StringAny, false
	}
	return s.rt.Validators.String[meta.Index].Kind, true
}

func (s *Session) integerKind(meta runtime.ValidatorMeta) (runtime.IntegerKind, bool) {
	if int(meta.Index) >= len(s.rt.Validators.Integer) {
		return runtime.IntegerAny, false
	}
	return s.rt.Validators.Integer[meta.Index].Kind, true
}

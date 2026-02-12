package validator

import (
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuecodec"
	"github.com/jacoelho/xsd/internal/valuesemantics"
)

func (s *Session) deriveKeyFromCanonical(kind runtime.ValidatorKind, canonical []byte) (runtime.ValueKind, []byte, error) {
	if vk, key, err := valuesemantics.KeyForValidatorKind(kind, canonical); err == nil {
		s.keyTmp = append(s.keyTmp[:0], key...)
		return vk, s.keyTmp, nil
	}

	switch kind {
	case runtime.VAnyURI:
		key := valuecodec.StringKeyBytes(s.keyTmp[:0], 1, canonical)
		s.keyTmp = key
		return runtime.VKString, key, nil
	case runtime.VQName, runtime.VNotation:
		tag := byte(0)
		if kind == runtime.VNotation {
			tag = 1
		}
		key := valuecodec.QNameKeyCanonical(s.keyTmp[:0], tag, canonical)
		if len(key) == 0 {
			return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "invalid QName key")
		}
		s.keyTmp = key
		return runtime.VKQName, key, nil
	case runtime.VHexBinary:
		decoded, err := value.ParseHexBinary(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := valuecodec.BinaryKeyBytes(s.keyTmp[:0], 0, decoded)
		s.keyTmp = key
		return runtime.VKBinary, key, nil
	case runtime.VBase64Binary:
		decoded, err := value.ParseBase64Binary(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := valuecodec.BinaryKeyBytes(s.keyTmp[:0], 1, decoded)
		s.keyTmp = key
		return runtime.VKBinary, key, nil
	default:
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", kind)
	}
}

func (s *Session) decForCanonical(canonical []byte, metrics *ValueMetrics) (num.Dec, error) {
	if metrics != nil && metrics.decSet {
		return metrics.decVal, nil
	}
	val, perr := num.ParseDec(canonical)
	if perr != nil {
		return num.Dec{}, valueErrorMsg(valueErrInvalid, "invalid decimal")
	}
	if metrics != nil {
		metrics.decVal = val
		metrics.decSet = true
	}
	return val, nil
}

func (s *Session) intForCanonical(canonical []byte, metrics *ValueMetrics) (num.Int, error) {
	if metrics != nil && metrics.intSet {
		return metrics.intVal, nil
	}
	val, perr := num.ParseInt(canonical)
	if perr != nil {
		return num.Int{}, valueErrorMsg(valueErrInvalid, "invalid integer")
	}
	if metrics != nil {
		metrics.intVal = val
		metrics.intSet = true
	}
	return val, nil
}

func (s *Session) float32ForCanonical(canonical []byte, metrics *ValueMetrics) (float32, num.FloatClass, error) {
	if metrics != nil && metrics.float32Set {
		return metrics.float32Val, metrics.float32Class, nil
	}
	val, class, perr := num.ParseFloat32(canonical)
	if perr != nil {
		return 0, num.FloatFinite, valueErrorMsg(valueErrInvalid, "invalid float")
	}
	if metrics != nil {
		metrics.float32Val = val
		metrics.float32Class = class
		metrics.float32Set = true
	}
	return val, class, nil
}

func (s *Session) float64ForCanonical(canonical []byte, metrics *ValueMetrics) (float64, num.FloatClass, error) {
	if metrics != nil && metrics.float64Set {
		return metrics.float64Val, metrics.float64Class, nil
	}
	val, class, perr := num.ParseFloat(canonical, 64)
	if perr != nil {
		return 0, num.FloatFinite, valueErrorMsg(valueErrInvalid, "invalid double")
	}
	if metrics != nil {
		metrics.float64Val = val
		metrics.float64Class = class
		metrics.float64Set = true
	}
	return val, class, nil
}

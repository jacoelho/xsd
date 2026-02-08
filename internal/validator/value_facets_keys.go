package validator

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value/temporal"
	"github.com/jacoelho/xsd/internal/valuekey"
)

func (s *Session) deriveKeyFromCanonical(kind runtime.ValidatorKind, canonical []byte) (runtime.ValueKind, []byte, error) {
	switch kind {
	case runtime.VString:
		key := valuekey.StringKeyBytes(s.keyTmp[:0], 0, canonical)
		s.keyTmp = key
		return runtime.VKString, key, nil
	case runtime.VBoolean:
		switch {
		case bytes.Equal(canonical, []byte("true")):
			return runtime.VKBool, []byte{1}, nil
		case bytes.Equal(canonical, []byte("false")):
			return runtime.VKBool, []byte{0}, nil
		default:
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid boolean")
		}
	case runtime.VDecimal:
		decVal, perr := num.ParseDec(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid decimal")
		}
		key := num.EncodeDecKey(s.keyTmp[:0], decVal)
		s.keyTmp = key
		return runtime.VKDecimal, key, nil
	case runtime.VInteger:
		intVal, perr := num.ParseInt(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid integer")
		}
		key := num.EncodeIntKey(s.keyTmp[:0], intVal)
		s.keyTmp = key
		return runtime.VKDecimal, key, nil
	case runtime.VFloat:
		v, class, perr := num.ParseFloat32(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid float")
		}
		key := valuekey.Float32Key(s.keyTmp[:0], v, class)
		s.keyTmp = key
		return runtime.VKFloat32, key, nil
	case runtime.VDouble:
		v, class, perr := num.ParseFloat64(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, "invalid double")
		}
		key := valuekey.Float64Key(s.keyTmp[:0], v, class)
		s.keyTmp = key
		return runtime.VKFloat64, key, nil
	case runtime.VAnyURI:
		key := valuekey.StringKeyBytes(s.keyTmp[:0], 1, canonical)
		s.keyTmp = key
		return runtime.VKString, key, nil
	case runtime.VQName, runtime.VNotation:
		tag := byte(0)
		if kind == runtime.VNotation {
			tag = 1
		}
		key := valuekey.QNameKeyCanonical(s.keyTmp[:0], tag, canonical)
		if len(key) == 0 {
			return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "invalid QName key")
		}
		s.keyTmp = key
		return runtime.VKQName, key, nil
	case runtime.VHexBinary:
		decoded, err := types.ParseHexBinary(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := valuekey.BinaryKeyBytes(s.keyTmp[:0], 0, decoded)
		s.keyTmp = key
		return runtime.VKBinary, key, nil
	case runtime.VBase64Binary:
		decoded, err := types.ParseBase64Binary(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := valuekey.BinaryKeyBytes(s.keyTmp[:0], 1, decoded)
		s.keyTmp = key
		return runtime.VKBinary, key, nil
	case runtime.VDuration:
		dur, err := types.ParseXSDDuration(string(canonical))
		if err != nil {
			return runtime.VKInvalid, nil, valueErrorMsg(valueErrInvalid, err.Error())
		}
		key := valuekey.DurationKeyBytes(s.keyTmp[:0], dur)
		s.keyTmp = key
		return runtime.VKDuration, key, nil
	case runtime.VDateTime, runtime.VDate, runtime.VTime, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		tv, err := parseTemporalForKind(kind, canonical)
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		key := valuekey.TemporalKeyBytes(s.keyTmp[:0], temporalSubkind(kind), tv.Time, temporal.ValueTimezoneKind(tv.TimezoneKind), tv.LeapSecond)
		s.keyTmp = key
		return runtime.VKDateTime, key, nil
	default:
		return runtime.VKInvalid, nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", kind)
	}
}

func (s *Session) decForCanonical(canonical []byte, metrics *valueMetrics) (num.Dec, error) {
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

func (s *Session) intForCanonical(canonical []byte, metrics *valueMetrics) (num.Int, error) {
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

func (s *Session) float32ForCanonical(canonical []byte, metrics *valueMetrics) (float32, num.FloatClass, error) {
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

func (s *Session) float64ForCanonical(canonical []byte, metrics *valueMetrics) (float64, num.FloatClass, error) {
	if metrics != nil && metrics.float64Set {
		return metrics.float64Val, metrics.float64Class, nil
	}
	val, class, perr := num.ParseFloat64(canonical)
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

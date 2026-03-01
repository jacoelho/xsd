package valuesemantics

import (
	"fmt"
	"unsafe"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
	"github.com/jacoelho/xsd/internal/valuecodec"
)

// KeyForValidatorKind derives deterministic value-key encoding from canonical lexical bytes.
func KeyForValidatorKind(kind runtime.ValidatorKind, canonical []byte) (runtime.ValueKind, []byte, error) {
	switch kind {
	case runtime.VString:
		return runtime.VKString, valuecodec.StringKeyBytes(nil, 0, canonical), nil
	case runtime.VBoolean:
		if string(canonical) == "true" {
			return runtime.VKBool, []byte{1}, nil
		}
		if string(canonical) == "false" {
			return runtime.VKBool, []byte{0}, nil
		}
		return runtime.VKInvalid, nil, fmt.Errorf("invalid boolean")
	case runtime.VDecimal:
		decVal, perr := num.ParseDec(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, fmt.Errorf("invalid decimal")
		}
		return runtime.VKDecimal, num.EncodeDecKey(nil, decVal), nil
	case runtime.VInteger:
		intVal, perr := num.ParseInt(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, fmt.Errorf("invalid integer")
		}
		return runtime.VKDecimal, num.EncodeDecKey(nil, intVal.AsDec()), nil
	case runtime.VFloat:
		v, class, perr := num.ParseFloat32(canonical)
		if perr != nil {
			return runtime.VKInvalid, nil, fmt.Errorf("invalid float")
		}
		return runtime.VKFloat32, valuecodec.Float32Key(nil, v, class), nil
	case runtime.VDouble:
		v, class, perr := num.ParseFloat(canonical, 64)
		if perr != nil {
			return runtime.VKInvalid, nil, fmt.Errorf("invalid double")
		}
		return runtime.VKFloat64, valuecodec.Float64Key(nil, v, class), nil
	case runtime.VDuration:
		dur, err := durationlex.Parse(unsafe.String(unsafe.SliceData(canonical), len(canonical)))
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		return runtime.VKDuration, valuecodec.DurationKeyBytes(nil, dur), nil
	case runtime.VDateTime, runtime.VDate, runtime.VTime, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		spec, ok := runtime.TemporalSpecForValidatorKind(kind)
		if !ok {
			return runtime.VKInvalid, nil, fmt.Errorf("unsupported temporal kind %d", kind)
		}
		tv, err := temporal.Parse(spec.Kind, canonical)
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		return runtime.VKDateTime, valuecodec.TemporalKeyBytes(nil, spec.KeyTag, tv.Time, tv.TimezoneKind, tv.LeapSecond), nil
	default:
		return runtime.VKInvalid, nil, fmt.Errorf("unsupported validator kind %d", kind)
	}
}

// KeyForPrimitiveName derives deterministic value-key encoding from normalized lexical text.
func KeyForPrimitiveName(primitive, normalized string, ctx map[string]string) (runtime.ValueKind, []byte, error) {
	switch primitive {
	case "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN":
		return runtime.VKString, valuecodec.StringKeyString(0, normalized), nil
	case "anyURI":
		return runtime.VKString, valuecodec.StringKeyString(1, normalized), nil
	case "decimal":
		decVal, perr := num.ParseDec([]byte(normalized))
		if perr != nil {
			return runtime.VKInvalid, nil, fmt.Errorf("invalid decimal")
		}
		return runtime.VKDecimal, num.EncodeDecKey(nil, decVal), nil
	case "boolean":
		v, err := value.ParseBoolean([]byte(normalized))
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		if v {
			return runtime.VKBool, []byte{1}, nil
		}
		return runtime.VKBool, []byte{0}, nil
	case "float":
		v, class, perr := num.ParseFloat32([]byte(normalized))
		if perr != nil {
			return runtime.VKInvalid, nil, fmt.Errorf("invalid float")
		}
		return runtime.VKFloat32, valuecodec.Float32Key(nil, v, class), nil
	case "double":
		v, class, perr := num.ParseFloat([]byte(normalized), 64)
		if perr != nil {
			return runtime.VKInvalid, nil, fmt.Errorf("invalid double")
		}
		return runtime.VKFloat64, valuecodec.Float64Key(nil, v, class), nil
	case "duration":
		dur, err := durationlex.Parse(normalized)
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		return runtime.VKDuration, valuecodec.DurationKeyBytes(nil, dur), nil
	case "hexBinary":
		b, err := value.ParseHexBinary([]byte(normalized))
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		return runtime.VKBinary, valuecodec.BinaryKeyBytes(nil, 0, b), nil
	case "base64Binary":
		b, err := value.ParseBase64Binary([]byte(normalized))
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		return runtime.VKBinary, valuecodec.BinaryKeyBytes(nil, 1, b), nil
	case "QName":
		qn, err := qname.ParseQNameValue(normalized, ctx)
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		return runtime.VKQName, valuecodec.QNameKeyStrings(0, qn.Namespace, qn.Local), nil
	case "NOTATION":
		qn, err := qname.ParseQNameValue(normalized, ctx)
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		return runtime.VKQName, valuecodec.QNameKeyStrings(1, qn.Namespace, qn.Local), nil
	default:
		if kind, ok := temporal.KindFromPrimitiveName(primitive); ok {
			tv, err := temporal.Parse(kind, []byte(normalized))
			if err != nil {
				return runtime.VKInvalid, nil, err
			}
			key, err := valuecodec.TemporalKeyFromValue(nil, tv)
			if err != nil {
				return runtime.VKInvalid, nil, err
			}
			return runtime.VKDateTime, key, nil
		}
		return runtime.VKInvalid, nil, fmt.Errorf("unsupported primitive type %s", primitive)
	}
}

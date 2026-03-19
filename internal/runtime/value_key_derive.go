package runtime

import (
	"fmt"
	"unsafe"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/value"
)

// KeyForValidatorKind derives deterministic value-key encoding from canonical lexical bytes.
func KeyForValidatorKind(kind ValidatorKind, canonical []byte) (ValueKind, []byte, error) {
	switch kind {
	case VString:
		return VKString, StringKeyBytes(nil, 0, canonical), nil
	case VBoolean:
		if string(canonical) == "true" {
			return VKBool, []byte{1}, nil
		}
		if string(canonical) == "false" {
			return VKBool, []byte{0}, nil
		}
		return VKInvalid, nil, fmt.Errorf("invalid boolean")
	case VDecimal:
		decVal, perr := num.ParseDec(canonical)
		if perr != nil {
			return VKInvalid, nil, fmt.Errorf("invalid decimal")
		}
		return VKDecimal, num.EncodeDecKey(nil, decVal), nil
	case VInteger:
		intVal, perr := num.ParseInt(canonical)
		if perr != nil {
			return VKInvalid, nil, fmt.Errorf("invalid integer")
		}
		return VKDecimal, num.EncodeDecKey(nil, intVal.AsDec()), nil
	case VFloat:
		v, class, perr := num.ParseFloat32(canonical)
		if perr != nil {
			return VKInvalid, nil, fmt.Errorf("invalid float")
		}
		return VKFloat32, Float32Key(nil, v, class), nil
	case VDouble:
		v, class, perr := num.ParseFloat(canonical, 64)
		if perr != nil {
			return VKInvalid, nil, fmt.Errorf("invalid double")
		}
		return VKFloat64, Float64Key(nil, v, class), nil
	case VDuration:
		dur, err := value.ParseDuration(unsafe.String(unsafe.SliceData(canonical), len(canonical)))
		if err != nil {
			return VKInvalid, nil, err
		}
		return VKDuration, DurationKeyBytes(nil, dur), nil
	case VDateTime, VDate, VTime, VGYearMonth, VGYear, VGMonthDay, VGDay, VGMonth:
		spec, ok := TemporalSpecForValidatorKind(kind)
		if !ok {
			return VKInvalid, nil, fmt.Errorf("unsupported temporal kind %d", kind)
		}
		tv, err := value.Parse(spec.Kind, canonical)
		if err != nil {
			return VKInvalid, nil, err
		}
		return VKDateTime, TemporalKeyBytes(nil, spec.KeyTag, tv.Time, tv.TimezoneKind, tv.LeapSecond), nil
	default:
		return VKInvalid, nil, fmt.Errorf("unsupported validator kind %d", kind)
	}
}

// KeyForPrimitiveName derives deterministic value-key encoding from normalized lexical text.
func KeyForPrimitiveName(primitive, normalized string, ctx map[string]string) (ValueKind, []byte, error) {
	switch primitive {
	case "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN":
		return VKString, StringKeyString(0, normalized), nil
	case "anyURI":
		return VKString, StringKeyString(1, normalized), nil
	case "decimal":
		decVal, perr := num.ParseDec([]byte(normalized))
		if perr != nil {
			return VKInvalid, nil, fmt.Errorf("invalid decimal")
		}
		return VKDecimal, num.EncodeDecKey(nil, decVal), nil
	case "boolean":
		v, err := value.ParseBoolean([]byte(normalized))
		if err != nil {
			return VKInvalid, nil, err
		}
		if v {
			return VKBool, []byte{1}, nil
		}
		return VKBool, []byte{0}, nil
	case "float":
		v, class, perr := num.ParseFloat32([]byte(normalized))
		if perr != nil {
			return VKInvalid, nil, fmt.Errorf("invalid float")
		}
		return VKFloat32, Float32Key(nil, v, class), nil
	case "double":
		v, class, perr := num.ParseFloat([]byte(normalized), 64)
		if perr != nil {
			return VKInvalid, nil, fmt.Errorf("invalid double")
		}
		return VKFloat64, Float64Key(nil, v, class), nil
	case "duration":
		dur, err := value.ParseDuration(normalized)
		if err != nil {
			return VKInvalid, nil, err
		}
		return VKDuration, DurationKeyBytes(nil, dur), nil
	case "hexBinary":
		b, err := value.ParseHexBinary([]byte(normalized))
		if err != nil {
			return VKInvalid, nil, err
		}
		return VKBinary, BinaryKeyBytes(nil, 0, b), nil
	case "base64Binary":
		b, err := value.ParseBase64Binary([]byte(normalized))
		if err != nil {
			return VKInvalid, nil, err
		}
		return VKBinary, BinaryKeyBytes(nil, 1, b), nil
	case "QName":
		qn, err := qname.ParseQNameValue(normalized, ctx)
		if err != nil {
			return VKInvalid, nil, err
		}
		return VKQName, QNameKeyStrings(0, qn.Namespace, qn.Local), nil
	case "NOTATION":
		qn, err := qname.ParseQNameValue(normalized, ctx)
		if err != nil {
			return VKInvalid, nil, err
		}
		return VKQName, QNameKeyStrings(1, qn.Namespace, qn.Local), nil
	default:
		if kind, ok := value.KindFromPrimitiveName(primitive); ok {
			tv, err := value.Parse(kind, []byte(normalized))
			if err != nil {
				return VKInvalid, nil, err
			}
			key, err := TemporalKeyFromValue(nil, tv)
			if err != nil {
				return VKInvalid, nil, err
			}
			return VKDateTime, key, nil
		}
		return VKInvalid, nil, fmt.Errorf("unsupported primitive type %s", primitive)
	}
}

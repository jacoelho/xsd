package validatorgen

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/num"
	qnamelex "github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
	"github.com/jacoelho/xsd/internal/valuecodec"
)

func (c *compiler) keyBytesAtomic(normalized string, typ model.Type, ctx map[string]string) (keyBytes, error) {
	primName, err := c.res.primitiveName(typ)
	if err != nil {
		return keyBytes{}, err
	}
	if kind, ok := temporal.KindFromPrimitiveName(primName); ok {
		return c.keyBytesTemporal(normalized, kind)
	}
	switch primName {
	case "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN":
		return keyBytes{kind: runtime.VKString, bytes: valuecodec.StringKeyString(0, normalized)}, nil
	case "anyURI":
		return keyBytes{kind: runtime.VKString, bytes: valuecodec.StringKeyString(1, normalized)}, nil
	case "decimal":
		if c.res.isIntegerDerived(typ) {
			intVal, err := parseInt(normalized)
			if err != nil {
				return keyBytes{}, err
			}
			if err := runtime.ValidateIntegerKind(c.integerKindForType(typ), intVal); err != nil {
				return keyBytes{}, err
			}
			return keyBytes{kind: runtime.VKDecimal, bytes: num.EncodeDecKey(nil, intVal.AsDec())}, nil
		}
		decVal, err := parseDec(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKDecimal, bytes: num.EncodeDecKey(nil, decVal)}, nil
	case "boolean":
		v, err := value.ParseBoolean([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		if v {
			return keyBytes{kind: runtime.VKBool, bytes: []byte{1}}, nil
		}
		return keyBytes{kind: runtime.VKBool, bytes: []byte{0}}, nil
	case "float":
		v, class, perr := num.ParseFloat32([]byte(normalized))
		if perr != nil {
			return keyBytes{}, fmt.Errorf("invalid float")
		}
		return keyBytes{kind: runtime.VKFloat32, bytes: valuecodec.Float32Key(nil, v, class)}, nil
	case "double":
		v, class, perr := num.ParseFloat([]byte(normalized), 64)
		if perr != nil {
			return keyBytes{}, fmt.Errorf("invalid double")
		}
		return keyBytes{kind: runtime.VKFloat64, bytes: valuecodec.Float64Key(nil, v, class)}, nil
	case "duration":
		dur, err := durationlex.Parse(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKDuration, bytes: valuecodec.DurationKeyBytes(nil, dur)}, nil
	case "hexBinary":
		b, err := value.ParseHexBinary([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKBinary, bytes: valuecodec.BinaryKeyBytes(nil, 0, b)}, nil
	case "base64Binary":
		b, err := value.ParseBase64Binary([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKBinary, bytes: valuecodec.BinaryKeyBytes(nil, 1, b)}, nil
	case "QName":
		qname, err := qnamelex.ParseQNameValue(normalized, ctx)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKQName, bytes: valuecodec.QNameKeyStrings(0, qname.Namespace, qname.Local)}, nil
	case "NOTATION":
		qname, err := qnamelex.ParseQNameValue(normalized, ctx)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKQName, bytes: valuecodec.QNameKeyStrings(1, qname.Namespace, qname.Local)}, nil
	default:
		return keyBytes{}, fmt.Errorf("unsupported primitive type %s", primName)
	}
}

func (c *compiler) keyBytesTemporal(normalized string, kind temporal.Kind) (keyBytes, error) {
	tv, err := temporal.Parse(kind, []byte(normalized))
	if err != nil {
		return keyBytes{}, err
	}
	key, err := valuecodec.TemporalKeyFromValue(nil, tv)
	if err != nil {
		return keyBytes{}, err
	}
	return keyBytes{kind: runtime.VKDateTime, bytes: key}, nil
}

func parseInt(normalized string) (num.Int, error) {
	val, perr := num.ParseInt([]byte(normalized))
	if perr != nil {
		return num.Int{}, fmt.Errorf("invalid integer")
	}
	return val, nil
}

func parseDec(normalized string) (num.Dec, error) {
	val, perr := num.ParseDec([]byte(normalized))
	if perr != nil {
		return num.Dec{}, fmt.Errorf("invalid decimal")
	}
	return val, nil
}

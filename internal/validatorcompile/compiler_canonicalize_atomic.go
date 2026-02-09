package validatorcompile

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

func (c *compiler) canonicalizeAtomic(normalized string, typ types.Type, ctx map[string]string) ([]byte, error) {
	if c.res.isQNameOrNotation(typ) {
		resolver := mapResolver(ctx)
		return value.CanonicalQName([]byte(normalized), resolver, nil)
	}

	primName, err := c.res.primitiveName(typ)
	if err != nil {
		return nil, err
	}

	switch primName {
	case "string":
		if err := runtime.ValidateStringKind(c.stringKindForType(typ), []byte(normalized)); err != nil {
			return nil, err
		}
		return []byte(normalized), nil
	case "anyURI":
		if err := value.ValidateAnyURI([]byte(normalized)); err != nil {
			return nil, err
		}
		return []byte(normalized), nil
	case "decimal":
		if c.res.isIntegerDerived(typ) {
			v, perr := num.ParseInt([]byte(normalized))
			if perr != nil {
				return nil, fmt.Errorf("invalid integer: %s", normalized)
			}
			if err := runtime.ValidateIntegerKind(c.integerKindForType(typ), v); err != nil {
				return nil, err
			}
			return v.RenderCanonical(nil), nil
		}
		v, perr := num.ParseDec([]byte(normalized))
		if perr != nil {
			return nil, fmt.Errorf("invalid decimal: %s", normalized)
		}
		return v.RenderCanonical(nil), nil
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte", "nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		v, perr := num.ParseInt([]byte(normalized))
		if perr != nil {
			return nil, fmt.Errorf("invalid integer: %s", normalized)
		}
		if err := runtime.ValidateIntegerKind(c.integerKindForType(typ), v); err != nil {
			return nil, err
		}
		return v.RenderCanonical(nil), nil
	case "boolean":
		v, err := value.ParseBoolean([]byte(normalized))
		if err != nil {
			return nil, err
		}
		if v {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	case "float":
		v, err := value.ParseFloat([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalFloat(float64(v), 32)), nil
	case "double":
		v, err := value.ParseDouble([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.CanonicalFloat(v, 64)), nil
	case "dateTime":
		tv, err := temporal.Parse(temporal.KindDateTime, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "date":
		tv, err := temporal.Parse(temporal.KindDate, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "time":
		tv, err := temporal.Parse(temporal.KindTime, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "gYearMonth":
		tv, err := temporal.Parse(temporal.KindGYearMonth, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "gYear":
		tv, err := temporal.Parse(temporal.KindGYear, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "gMonthDay":
		tv, err := temporal.Parse(temporal.KindGMonthDay, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "gDay":
		tv, err := temporal.Parse(temporal.KindGDay, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "gMonth":
		tv, err := temporal.Parse(temporal.KindGMonth, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
	case "duration":
		dur, err := durationlex.Parse(normalized)
		if err != nil {
			return nil, err
		}
		return []byte(types.ComparableXSDDuration{Value: dur}.String()), nil
	case "hexBinary":
		b, err := value.ParseHexBinary([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return upperHex(nil, b), nil
	case "base64Binary":
		b, err := value.ParseBase64Binary([]byte(normalized))
		if err != nil {
			return nil, err
		}
		out := make([]byte, base64.StdEncoding.EncodedLen(len(b)))
		base64.StdEncoding.Encode(out, b)
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported primitive type %s", primName)
	}
}

func upperHex(dst, src []byte) []byte {
	size := hex.EncodedLen(len(src))
	if cap(dst) < size {
		dst = make([]byte, size)
	} else {
		dst = dst[:size]
	}
	hex.Encode(dst, src)
	for i := range dst {
		if dst[i] >= 'a' && dst[i] <= 'f' {
			dst[i] -= 'a' - 'A'
		}
	}
	return dst
}

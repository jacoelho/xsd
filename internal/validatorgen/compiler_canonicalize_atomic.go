package validatorgen

import (
	"encoding/base64"
	"fmt"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

func (c *compiler) canonicalizeAtomic(normalized string, typ model.Type, ctx map[string]string) ([]byte, error) {
	if c.res.isQNameOrNotation(typ) {
		resolver := mapResolver(ctx)
		return value.CanonicalQName([]byte(normalized), resolver, nil)
	}

	primName, err := c.res.primitiveName(typ)
	if err != nil {
		return nil, err
	}
	if kind, ok := temporal.KindFromPrimitiveName(primName); ok {
		tv, err := temporal.Parse(kind, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(temporal.Canonical(tv)), nil
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
	case "duration":
		dur, err := durationlex.Parse(normalized)
		if err != nil {
			return nil, err
		}
		return []byte(model.ComparableXSDDuration{Value: dur}.String()), nil
	case "hexBinary":
		b, err := value.ParseHexBinary([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return value.UpperHex(nil, b), nil
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

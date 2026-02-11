package validatorgen

import (
	"encoding/base64"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
	"github.com/jacoelho/xsd/internal/valuesemantics"
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
			_, canon, err := valuesemantics.CanonicalizeInteger([]byte(normalized), func(v num.Int) error {
				return runtime.ValidateIntegerKind(c.integerKindForType(typ), v)
			})
			if err != nil {
				return nil, err
			}
			return canon, nil
		}
		_, canon, err := valuesemantics.CanonicalizeDecimal([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "boolean":
		_, canon, err := valuesemantics.CanonicalizeBoolean([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "float":
		_, _, canon, err := valuesemantics.CanonicalizeFloat32([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "double":
		_, _, canon, err := valuesemantics.CanonicalizeFloat64([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "duration":
		_, canon, err := valuesemantics.CanonicalizeDuration([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
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

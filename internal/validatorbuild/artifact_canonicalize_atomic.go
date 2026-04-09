package validatorbuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
)

func (c *artifactCompiler) canonicalizeAtomic(normalized string, typ model.Type, ctx map[string]string) ([]byte, error) {
	if c.res.isQNameOrNotation(typ) {
		resolver := mapResolver(ctx)
		return value.CanonicalQName([]byte(normalized), resolver, nil)
	}

	primName, err := c.res.primitiveName(typ)
	if err != nil {
		return nil, err
	}
	if kind, ok := value.KindFromPrimitiveName(primName); ok {
		tv, err := value.Parse(kind, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.Canonical(tv)), nil
	}

	switch primName {
	case "string":
		return value.CanonicalizeString([]byte(normalized), func(data []byte) error {
			return runtime.ValidateStringKind(c.stringKindForType(typ), data)
		})
	case "anyURI":
		return value.CanonicalizeAnyURI([]byte(normalized))
	case "decimal":
		if c.res.isIntegerDerived(typ) {
			_, canon, err := value.CanonicalizeInteger([]byte(normalized), func(v num.Int) error {
				return runtime.ValidateIntegerKind(c.integerKindForType(typ), v)
			})
			if err != nil {
				return nil, err
			}
			return canon, nil
		}
		_, canon, err := value.CanonicalizeDecimal([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "boolean":
		_, canon, err := value.CanonicalizeBoolean([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "float":
		_, _, canon, err := value.CanonicalizeFloat32([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "double":
		_, _, canon, err := value.CanonicalizeFloat64([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "duration":
		_, canon, err := value.CanonicalizeDuration([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "hexBinary":
		return value.CanonicalizeHexBinary([]byte(normalized))
	case "base64Binary":
		return value.CanonicalizeBase64Binary([]byte(normalized))
	default:
		return nil, fmt.Errorf("unsupported primitive type %s", primName)
	}
}

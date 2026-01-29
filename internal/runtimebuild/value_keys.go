package runtimebuild

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
)

type keyBytes struct {
	bytes []byte
	kind  runtime.ValueKind
}

func (c *compiler) valueKeysForNormalized(normalized string, typ types.Type, ctx map[string]string) ([]runtime.ValueKey, error) {
	keys, err := c.keyBytesForNormalized(normalized, typ, ctx)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.ValueKey, 0, len(keys))
	for _, key := range keys {
		out = append(out, c.makeValueKey(key.kind, key.bytes))
	}
	return out, nil
}

func (c *compiler) keyBytesForNormalized(normalized string, typ types.Type, ctx map[string]string) ([]keyBytes, error) {
	switch c.res.varietyForType(typ) {
	case types.ListVariety:
		item, ok := c.res.listItemTypeFromType(typ)
		if !ok || item == nil {
			return nil, fmt.Errorf("list type missing item type")
		}
		items := splitXMLWhitespace(normalized)
		var keyBytesBuf []byte
		for _, itemLex := range items {
			itemKey, err := c.keyBytesForNormalizedSingle(itemLex, item, ctx)
			if err != nil {
				return nil, err
			}
			keyBytesBuf = runtime.AppendListKey(keyBytesBuf, itemKey.kind, itemKey.bytes)
		}
		return []keyBytes{{kind: runtime.VKList, bytes: keyBytesBuf}}, nil
	case types.UnionVariety:
		members := c.res.unionMemberTypesFromType(typ)
		if len(members) == 0 {
			return nil, fmt.Errorf("union has no member types")
		}
		var out []keyBytes
		for _, member := range members {
			memberLex := c.normalizeLexical(normalized, member)
			memberFacets, err := c.facetsForType(member)
			if err != nil {
				return nil, err
			}
			partial := filterFacets(memberFacets, func(f types.Facet) bool {
				_, ok := f.(*types.Enumeration)
				return !ok
			})
			if validateErr := c.validatePartialFacets(memberLex, member, partial); validateErr != nil {
				continue
			}
			keys, err := c.keyBytesForNormalized(memberLex, member, ctx)
			if err != nil {
				continue
			}
			out = append(out, keys...)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("union value does not match any member type")
		}
		return out, nil
	default:
		key, err := c.keyBytesAtomic(normalized, typ, ctx)
		if err != nil {
			return nil, err
		}
		return []keyBytes{key}, nil
	}
}

func (c *compiler) keyBytesForNormalizedSingle(normalized string, typ types.Type, ctx map[string]string) (keyBytes, error) {
	keys, err := c.keyBytesForNormalized(normalized, typ, ctx)
	if err != nil {
		return keyBytes{}, err
	}
	if len(keys) == 0 {
		return keyBytes{}, fmt.Errorf("no value key produced")
	}
	return keys[0], nil
}

func (c *compiler) keyBytesAtomic(normalized string, typ types.Type, ctx map[string]string) (keyBytes, error) {
	primName, err := c.res.primitiveName(typ)
	if err != nil {
		return keyBytes{}, err
	}
	var vkind runtime.ValidatorKind
	if st, ok := types.AsSimpleType(typ); ok && st != nil {
		vkind, err = c.validatorKind(st)
		if err != nil {
			return keyBytes{}, err
		}
	} else {
		if primName == "decimal" && c.res.isIntegerDerived(typ) {
			vkind = runtime.VInteger
		} else {
			vkind, err = builtinValidatorKind(primName)
			if err != nil {
				return keyBytes{}, err
			}
		}
	}
	kind, ok := runtime.ValueKindForValidatorKind(vkind)
	if !ok {
		return keyBytes{}, fmt.Errorf("unsupported value kind")
	}

	if c.res.isQNameOrNotation(typ) {
		resolver := mapResolver(ctx)
		canon, err := value.CanonicalQName([]byte(normalized), resolver, nil)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: kind, bytes: canon}, nil
	}

	switch primName {
	case "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN", "anyURI":
		return keyBytes{kind: kind, bytes: []byte(normalized)}, nil
	case "decimal":
		if kind == runtime.VKInteger {
			v, err := value.ParseInteger([]byte(normalized))
			if err != nil {
				return keyBytes{}, err
			}
			canon := []byte(v.String())
			key, err := value.CanonicalIntegerKeyFromCanonical(canon, nil)
			if err != nil {
				return keyBytes{}, err
			}
			return keyBytes{kind: kind, bytes: key}, nil
		}
		key, err := value.CanonicalDecimalKey([]byte(normalized), nil)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: kind, bytes: key}, nil
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte", "nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		v, err := value.ParseInteger([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		canon := []byte(v.String())
		key, err := value.CanonicalIntegerKeyFromCanonical(canon, nil)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: kind, bytes: key}, nil
	case "boolean":
		v, err := value.ParseBoolean([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		out := []byte("false")
		if v {
			out = []byte("true")
		}
		return keyBytes{kind: kind, bytes: out}, nil
	case "float":
		v, err := value.ParseFloat([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: kind, bytes: value.CanonicalFloat32Key(v, nil)}, nil
	case "double":
		v, err := value.ParseDouble([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: kind, bytes: value.CanonicalFloat64Key(v, nil)}, nil
	case "dateTime":
		t, err := value.ParseDateTime([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: kind, bytes: value.CanonicalTemporalKey(t, "dateTime", hasTZ, nil)}, nil
	case "date":
		t, err := value.ParseDate([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: kind, bytes: value.CanonicalTemporalKey(t, "date", hasTZ, nil)}, nil
	case "time":
		t, err := value.ParseTime([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: kind, bytes: value.CanonicalTemporalKey(t, "time", hasTZ, nil)}, nil
	case "gYearMonth":
		t, err := value.ParseGYearMonth([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: kind, bytes: value.CanonicalTemporalKey(t, "gYearMonth", hasTZ, nil)}, nil
	case "gYear":
		t, err := value.ParseGYear([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: kind, bytes: value.CanonicalTemporalKey(t, "gYear", hasTZ, nil)}, nil
	case "gMonthDay":
		t, err := value.ParseGMonthDay([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: kind, bytes: value.CanonicalTemporalKey(t, "gMonthDay", hasTZ, nil)}, nil
	case "gDay":
		t, err := value.ParseGDay([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: kind, bytes: value.CanonicalTemporalKey(t, "gDay", hasTZ, nil)}, nil
	case "gMonth":
		t, err := value.ParseGMonth([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: kind, bytes: value.CanonicalTemporalKey(t, "gMonth", hasTZ, nil)}, nil
	case "duration":
		dur, err := types.ParseXSDDuration(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: kind, bytes: types.CanonicalDurationKey(dur, nil)}, nil
	case "hexBinary":
		b, err := types.ParseHexBinary(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: kind, bytes: []byte(strings.ToUpper(fmt.Sprintf("%x", b)))}, nil
	case "base64Binary":
		b, err := types.ParseBase64Binary(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: kind, bytes: []byte(encodeBase64(b))}, nil
	default:
		return keyBytes{}, fmt.Errorf("unsupported primitive type %s", primName)
	}
}

func (c *compiler) makeValueKey(kind runtime.ValueKind, key []byte) runtime.ValueKey {
	hash := runtime.HashKey(kind, key)
	ref := c.values.addWithHash(key, hash)
	return runtime.ValueKey{
		Kind: kind,
		Hash: hash,
		Ref:  ref,
	}
}

package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuekey"
)

type keyBytes struct {
	bytes []byte
	kind  runtime.ValueKind
}

func (c *compiler) valueKeysForNormalized(lexical, normalized string, typ types.Type, ctx map[string]string) ([]runtime.ValueKey, error) {
	keys, err := c.keyBytesForNormalized(lexical, normalized, typ, ctx)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.ValueKey, 0, len(keys))
	for _, key := range keys {
		out = append(out, c.makeValueKey(key.kind, key.bytes))
	}
	return out, nil
}

func (c *compiler) keyBytesForNormalized(lexical, normalized string, typ types.Type, ctx map[string]string) ([]keyBytes, error) {
	switch c.res.varietyForType(typ) {
	case types.ListVariety:
		item, ok := c.res.listItemTypeFromType(typ)
		if !ok || item == nil {
			return nil, fmt.Errorf("list type missing item type")
		}
		items := splitXMLWhitespace(normalized)
		var keyBytesBuf []byte
		keyBytesBuf = valuekey.AppendUvarint(keyBytesBuf, uint64(len(items)))
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
			memberLex := c.normalizeLexical(lexical, member)
			memberFacets, err := c.facetsForType(member)
			if err != nil {
				return nil, err
			}
			if validateErr := c.validateMemberFacets(memberLex, member, memberFacets, ctx, true); validateErr != nil {
				continue
			}
			keys, err := c.keyBytesForNormalized(lexical, memberLex, member, ctx)
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
	keys, err := c.keyBytesForNormalized(normalized, normalized, typ, ctx)
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
	switch primName {
	case "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN":
		return keyBytes{kind: runtime.VKString, bytes: valuekey.StringKeyString(0, normalized)}, nil
	case "anyURI":
		return keyBytes{kind: runtime.VKString, bytes: valuekey.StringKeyString(1, normalized)}, nil
	case "decimal":
		if c.res.isIntegerDerived(typ) {
			intVal, err := parseInt(normalized)
			if err != nil {
				return keyBytes{}, err
			}
			if err := runtime.ValidateIntegerKind(c.integerKindForType(typ), intVal); err != nil {
				return keyBytes{}, err
			}
			return keyBytes{kind: runtime.VKDecimal, bytes: num.EncodeIntKey(nil, intVal)}, nil
		}
		decVal, err := parseDec(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKDecimal, bytes: num.EncodeDecKey(nil, decVal)}, nil
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte", "nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		intVal, err := parseInt(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		if err := runtime.ValidateIntegerKind(c.integerKindForType(typ), intVal); err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKDecimal, bytes: num.EncodeIntKey(nil, intVal)}, nil
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
		return keyBytes{kind: runtime.VKFloat32, bytes: valuekey.Float32Key(nil, v, class)}, nil
	case "double":
		v, class, perr := num.ParseFloat64([]byte(normalized))
		if perr != nil {
			return keyBytes{}, fmt.Errorf("invalid double")
		}
		return keyBytes{kind: runtime.VKFloat64, bytes: valuekey.Float64Key(nil, v, class)}, nil
	case "dateTime":
		t, err := value.ParseDateTime([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: valuekey.TemporalKeyBytes(nil, 0, t, hasTZ)}, nil
	case "date":
		t, err := value.ParseDate([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: valuekey.TemporalKeyBytes(nil, 1, t, hasTZ)}, nil
	case "time":
		t, err := value.ParseTime([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: valuekey.TemporalKeyBytes(nil, 2, t, hasTZ)}, nil
	case "gYearMonth":
		t, err := value.ParseGYearMonth([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: valuekey.TemporalKeyBytes(nil, 3, t, hasTZ)}, nil
	case "gYear":
		t, err := value.ParseGYear([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: valuekey.TemporalKeyBytes(nil, 4, t, hasTZ)}, nil
	case "gMonthDay":
		t, err := value.ParseGMonthDay([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: valuekey.TemporalKeyBytes(nil, 5, t, hasTZ)}, nil
	case "gDay":
		t, err := value.ParseGDay([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: valuekey.TemporalKeyBytes(nil, 6, t, hasTZ)}, nil
	case "gMonth":
		t, err := value.ParseGMonth([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: valuekey.TemporalKeyBytes(nil, 7, t, hasTZ)}, nil
	case "duration":
		dur, err := types.ParseXSDDuration(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKDuration, bytes: valuekey.DurationKeyBytes(nil, dur)}, nil
	case "hexBinary":
		b, err := types.ParseHexBinary(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKBinary, bytes: valuekey.BinaryKeyBytes(nil, 0, b)}, nil
	case "base64Binary":
		b, err := types.ParseBase64Binary(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKBinary, bytes: valuekey.BinaryKeyBytes(nil, 1, b)}, nil
	case "QName":
		qname, err := types.ParseQNameValue(normalized, ctx)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKQName, bytes: valuekey.QNameKeyStrings(0, string(qname.Namespace), qname.Local)}, nil
	case "NOTATION":
		qname, err := types.ParseQNameValue(normalized, ctx)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKQName, bytes: valuekey.QNameKeyStrings(1, string(qname.Namespace), qname.Local)}, nil
	default:
		return keyBytes{}, fmt.Errorf("unsupported primitive type %s", primName)
	}
}

func (c *compiler) makeValueKey(kind runtime.ValueKind, key []byte) runtime.ValueKey {
	hash := runtime.HashKey(kind, key)
	bytes := append([]byte(nil), key...)
	return runtime.ValueKey{
		Kind:  kind,
		Hash:  hash,
		Bytes: bytes,
	}
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

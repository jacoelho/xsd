package runtimebuild

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jacoelho/xsd/internal/num"
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
		keyBytesBuf = appendUvarint(keyBytesBuf, uint64(len(items)))
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
	switch primName {
	case "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN":
		return keyBytes{kind: runtime.VKString, bytes: stringKeyBytes(0, normalized)}, nil
	case "anyURI":
		return keyBytes{kind: runtime.VKString, bytes: stringKeyBytes(1, normalized)}, nil
	case "decimal":
		if c.res.isIntegerDerived(typ) {
			intVal, err := parseInt(normalized)
			if err != nil {
				return keyBytes{}, err
			}
			if err := validateIntegerKind(c.integerKindForType(typ), intVal); err != nil {
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
		if err := validateIntegerKind(c.integerKindForType(typ), intVal); err != nil {
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
		return keyBytes{kind: runtime.VKFloat32, bytes: float32Key(v, class)}, nil
	case "double":
		v, class, perr := num.ParseFloat64([]byte(normalized))
		if perr != nil {
			return keyBytes{}, fmt.Errorf("invalid double")
		}
		return keyBytes{kind: runtime.VKFloat64, bytes: float64Key(v, class)}, nil
	case "dateTime":
		t, err := value.ParseDateTime([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: temporalKeyBytes(0, t, hasTZ)}, nil
	case "date":
		t, err := value.ParseDate([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: temporalKeyBytes(1, t, hasTZ)}, nil
	case "time":
		t, err := value.ParseTime([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: temporalKeyBytes(2, t, hasTZ)}, nil
	case "gYearMonth":
		t, err := value.ParseGYearMonth([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: temporalKeyBytes(3, t, hasTZ)}, nil
	case "gYear":
		t, err := value.ParseGYear([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: temporalKeyBytes(4, t, hasTZ)}, nil
	case "gMonthDay":
		t, err := value.ParseGMonthDay([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: temporalKeyBytes(5, t, hasTZ)}, nil
	case "gDay":
		t, err := value.ParseGDay([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: temporalKeyBytes(6, t, hasTZ)}, nil
	case "gMonth":
		t, err := value.ParseGMonth([]byte(normalized))
		if err != nil {
			return keyBytes{}, err
		}
		hasTZ := value.HasTimezone([]byte(normalized))
		return keyBytes{kind: runtime.VKDateTime, bytes: temporalKeyBytes(7, t, hasTZ)}, nil
	case "duration":
		dur, err := types.ParseXSDDuration(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKDuration, bytes: durationKeyBytes(dur)}, nil
	case "hexBinary":
		b, err := types.ParseHexBinary(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKBinary, bytes: binaryKeyBytes(0, b)}, nil
	case "base64Binary":
		b, err := types.ParseBase64Binary(normalized)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKBinary, bytes: binaryKeyBytes(1, b)}, nil
	case "QName":
		qname, err := types.ParseQNameValue(normalized, ctx)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKQName, bytes: qnameKeyBytes(0, qname)}, nil
	case "NOTATION":
		qname, err := types.ParseQNameValue(normalized, ctx)
		if err != nil {
			return keyBytes{}, err
		}
		return keyBytes{kind: runtime.VKQName, bytes: qnameKeyBytes(1, qname)}, nil
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

func validateIntegerKind(kind runtime.IntegerKind, value num.Int) error {
	switch kind {
	case runtime.IntegerAny:
		return nil
	case runtime.IntegerLong:
		return checkIntRange(value, minInt64, maxInt64)
	case runtime.IntegerInt:
		return checkIntRange(value, minInt32, maxInt32)
	case runtime.IntegerShort:
		return checkIntRange(value, minInt16, maxInt16)
	case runtime.IntegerByte:
		return checkIntRange(value, minInt8, maxInt8)
	case runtime.IntegerNonNegative:
		if value.Sign < 0 {
			return fmt.Errorf("invalid non-negative integer")
		}
		return nil
	case runtime.IntegerPositive:
		if value.Sign <= 0 {
			return fmt.Errorf("invalid positive integer")
		}
		return nil
	case runtime.IntegerNonPositive:
		if value.Sign > 0 {
			return fmt.Errorf("invalid non-positive integer")
		}
		return nil
	case runtime.IntegerNegative:
		if value.Sign >= 0 {
			return fmt.Errorf("invalid negative integer")
		}
		return nil
	case runtime.IntegerUnsignedLong:
		if value.Sign < 0 {
			return fmt.Errorf("invalid unsignedLong")
		}
		return checkIntRange(value, intZero, maxUint64)
	case runtime.IntegerUnsignedInt:
		if value.Sign < 0 {
			return fmt.Errorf("invalid unsignedInt")
		}
		return checkIntRange(value, intZero, maxUint32)
	case runtime.IntegerUnsignedShort:
		if value.Sign < 0 {
			return fmt.Errorf("invalid unsignedShort")
		}
		return checkIntRange(value, intZero, maxUint16)
	case runtime.IntegerUnsignedByte:
		if value.Sign < 0 {
			return fmt.Errorf("invalid unsignedByte")
		}
		return checkIntRange(value, intZero, maxUint8)
	default:
		return nil
	}
}

func checkIntRange(value, min, max num.Int) error {
	if value.Compare(min) < 0 || value.Compare(max) > 0 {
		return fmt.Errorf("integer out of range")
	}
	return nil
}

func stringKeyBytes(tag byte, normalized string) []byte {
	out := make([]byte, 1+len(normalized))
	out[0] = tag
	copy(out[1:], normalized)
	return out
}

func binaryKeyBytes(tag byte, data []byte) []byte {
	out := make([]byte, 1+len(data))
	out[0] = tag
	copy(out[1:], data)
	return out
}

func qnameKeyBytes(tag byte, name types.QName) []byte {
	ns := []byte(name.Namespace)
	local := []byte(name.Local)
	out := make([]byte, 0, 1+binary.MaxVarintLen64*2+len(ns)+len(local))
	out = append(out, tag)
	out = appendUvarint(out, uint64(len(ns)))
	out = append(out, ns...)
	out = appendUvarint(out, uint64(len(local)))
	out = append(out, local...)
	return out
}

const (
	canonicalNaN32 = 0x7fc00000
	canonicalNaN64 = 0x7ff8000000000000
)

func float32Key(value float32, class num.FloatClass) []byte {
	var bits uint32
	switch class {
	case num.FloatNaN:
		bits = canonicalNaN32
	default:
		if value == 0 {
			bits = 0
		} else {
			bits = math.Float32bits(value)
		}
	}
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, bits)
	return out
}

func float64Key(value float64, class num.FloatClass) []byte {
	var bits uint64
	switch class {
	case num.FloatNaN:
		bits = canonicalNaN64
	default:
		if value == 0 {
			bits = 0
		} else {
			bits = math.Float64bits(value)
		}
	}
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, bits)
	return out
}

func temporalKeyBytes(subkind byte, t time.Time, hasTZ bool) []byte {
	if hasTZ {
		utc := t.UTC()
		out := make([]byte, 14)
		out[0] = subkind
		out[1] = 1
		binary.BigEndian.PutUint64(out[2:], uint64(utc.Unix()))
		binary.BigEndian.PutUint32(out[10:], uint32(utc.Nanosecond()))
		return out
	}
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	out := make([]byte, 20)
	out[0] = subkind
	out[1] = 0
	binary.BigEndian.PutUint32(out[2:], uint32(int32(year)))
	binary.BigEndian.PutUint16(out[6:], uint16(month))
	binary.BigEndian.PutUint16(out[8:], uint16(day))
	binary.BigEndian.PutUint16(out[10:], uint16(hour))
	binary.BigEndian.PutUint16(out[12:], uint16(min))
	binary.BigEndian.PutUint16(out[14:], uint16(sec))
	binary.BigEndian.PutUint32(out[16:], uint32(t.Nanosecond()))
	return out
}

func durationKeyBytes(dur types.XSDDuration) []byte {
	monthsTotal := int64(dur.Years)*12 + int64(dur.Months)
	monthInt, _ := num.ParseInt([]byte(strconv.FormatInt(monthsTotal, 10)))

	secondsTotal := float64(dur.Days)*86400 + float64(dur.Hours)*3600 + float64(dur.Minutes)*60 + dur.Seconds
	if secondsTotal < 0 {
		secondsTotal = -secondsTotal
	}
	secStr := strconv.FormatFloat(secondsTotal, 'f', -1, 64)
	secDec, _ := num.ParseDec([]byte(secStr))

	sign := byte(1)
	if dur.Negative {
		sign = 2
	}
	if monthsTotal == 0 && secDec.Sign == 0 {
		sign = 0
	}
	out := make([]byte, 0, 1+len(monthInt.Digits)+len(secDec.Coef)+16)
	out = append(out, sign)
	out = num.EncodeIntKey(out, monthInt)
	out = num.EncodeDecKey(out, secDec)
	return out
}

func appendUvarint(dst []byte, v uint64) []byte {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], v)
	return append(dst, buf[:n]...)
}

var (
	intZero   = num.Int{Sign: 0, Digits: []byte{'0'}}
	minInt8   = num.Int{Sign: -1, Digits: []byte("128")}
	maxInt8   = num.Int{Sign: 1, Digits: []byte("127")}
	minInt16  = num.Int{Sign: -1, Digits: []byte("32768")}
	maxInt16  = num.Int{Sign: 1, Digits: []byte("32767")}
	minInt32  = num.Int{Sign: -1, Digits: []byte("2147483648")}
	maxInt32  = num.Int{Sign: 1, Digits: []byte("2147483647")}
	minInt64  = num.Int{Sign: -1, Digits: []byte("9223372036854775808")}
	maxInt64  = num.Int{Sign: 1, Digits: []byte("9223372036854775807")}
	maxUint8  = num.Int{Sign: 1, Digits: []byte("255")}
	maxUint16 = num.Int{Sign: 1, Digits: []byte("65535")}
	maxUint32 = num.Int{Sign: 1, Digits: []byte("4294967295")}
	maxUint64 = num.Int{Sign: 1, Digits: []byte("18446744073709551615")}
)

package schemair

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
	"github.com/jacoelho/xsd/internal/xsdlex"
)

type ValueKeyKind uint8

const (
	ValueKeyInvalid ValueKeyKind = iota
	ValueKeyBool
	ValueKeyDecimal
	ValueKeyFloat32
	ValueKeyFloat64
	ValueKeyString
	ValueKeyBinary
	ValueKeyQName
	ValueKeyDateTime
	ValueKeyDuration
	ValueKeyList
)

type ValueKey struct {
	Kind  ValueKeyKind
	Bytes []byte
}

type ValueSpecResolver func(TypeRef) (SimpleTypeSpec, bool)

func ValueKeysForLexical(lexical string, spec SimpleTypeSpec, ctx map[string]string, resolve ValueSpecResolver) ([]ValueKey, error) {
	normalized := NormalizeValueLexical(lexical, spec)
	return ValueKeysForNormalized(lexical, normalized, spec, ctx, resolve)
}

func ValueKeysForNormalized(lexical, normalized string, spec SimpleTypeSpec, ctx map[string]string, resolve ValueSpecResolver) ([]ValueKey, error) {
	switch spec.Variety {
	case TypeVarietyList:
		return valueKeysForList(normalized, spec, ctx, resolve)
	case TypeVarietyUnion:
		return valueKeysForUnion(lexical, spec, ctx, resolve)
	default:
		key, err := valueKeyAtomic(normalized, spec, ctx)
		if err != nil {
			return nil, err
		}
		return []ValueKey{key}, nil
	}
}

func NormalizeValueLexical(lexical string, spec SimpleTypeSpec) string {
	normalized := value.NormalizeWhitespace(valueWhitespaceMode(spec.Whitespace), []byte(lexical), nil)
	return string(normalized)
}

func valueKeysForList(normalized string, spec SimpleTypeSpec, ctx map[string]string, resolve ValueSpecResolver) ([]ValueKey, error) {
	if resolve == nil {
		return nil, fmt.Errorf("list type missing item type")
	}
	item, ok := resolve(spec.Item)
	if !ok {
		return nil, fmt.Errorf("list type missing item type")
	}
	count := 0
	for range value.FieldsXMLWhitespaceStringSeq(normalized) {
		count++
	}
	keyBytes := appendValueKeyUvarint(nil, uint64(count))
	for itemLex := range value.FieldsXMLWhitespaceStringSeq(normalized) {
		itemNorm := NormalizeValueLexical(itemLex, item)
		itemKeys, err := ValueKeysForNormalized(itemLex, itemNorm, item, ctx, resolve)
		if err != nil {
			return nil, err
		}
		if len(itemKeys) == 0 {
			return nil, fmt.Errorf("no value key produced")
		}
		itemKey := itemKeys[0]
		keyBytes = appendValueKeyListEntry(keyBytes, byte(itemKey.Kind), itemKey.Bytes)
	}
	return []ValueKey{{Kind: ValueKeyList, Bytes: keyBytes}}, nil
}

func valueKeysForUnion(lexical string, spec SimpleTypeSpec, ctx map[string]string, resolve ValueSpecResolver) ([]ValueKey, error) {
	if len(spec.Members) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	var out []ValueKey
	for _, ref := range spec.Members {
		if resolve == nil {
			continue
		}
		member, ok := resolve(ref)
		if !ok {
			continue
		}
		memberLex := NormalizeValueLexical(lexical, member)
		if err := validateSpecLexicalValueWithResolver(member, lexical, ctx, resolve, make(map[TypeRef]bool)); err != nil {
			continue
		}
		keys, err := ValueKeysForNormalized(lexical, memberLex, member, ctx, resolve)
		if err != nil {
			continue
		}
		out = append(out, keys...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("union value does not match any member type")
	}
	return out, nil
}

func valueKeyAtomic(normalized string, spec SimpleTypeSpec, ctx map[string]string) (ValueKey, error) {
	primitive := primitiveNameForValueKey(spec)
	if primitive == "decimal" && spec.IntegerDerived {
		if err := validateAtomicLexicalValue(specBuiltinNameForValueKey(spec), normalized, ctx); err != nil {
			return ValueKey{}, err
		}
		intVal, err := num.ParseInt([]byte(normalized))
		if err != nil {
			return ValueKey{}, fmt.Errorf("invalid integer")
		}
		return ValueKey{Kind: ValueKeyDecimal, Bytes: num.EncodeDecKey(nil, intVal.AsDec())}, nil
	}
	return valueKeyForPrimitiveName(primitive, normalized, ctx)
}

func valueKeyForPrimitiveName(primitive, normalized string, ctx map[string]string) (ValueKey, error) {
	switch primitive {
	case "anySimpleType", "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN":
		return ValueKey{Kind: ValueKeyString, Bytes: stringValueKey(0, normalized)}, nil
	case "anyURI":
		return ValueKey{Kind: ValueKeyString, Bytes: stringValueKey(1, normalized)}, nil
	case "decimal":
		decVal, err := num.ParseDec([]byte(normalized))
		if err != nil {
			return ValueKey{}, fmt.Errorf("invalid decimal")
		}
		return ValueKey{Kind: ValueKeyDecimal, Bytes: num.EncodeDecKey(nil, decVal)}, nil
	case "boolean":
		v, err := value.ParseBoolean([]byte(normalized))
		if err != nil {
			return ValueKey{}, err
		}
		if v {
			return ValueKey{Kind: ValueKeyBool, Bytes: []byte{1}}, nil
		}
		return ValueKey{Kind: ValueKeyBool, Bytes: []byte{0}}, nil
	case "float":
		v, class, err := num.ParseFloat32([]byte(normalized))
		if err != nil {
			return ValueKey{}, fmt.Errorf("invalid float")
		}
		return ValueKey{Kind: ValueKeyFloat32, Bytes: float32ValueKey(v, class)}, nil
	case "double":
		v, class, err := num.ParseFloat([]byte(normalized), 64)
		if err != nil {
			return ValueKey{}, fmt.Errorf("invalid double")
		}
		return ValueKey{Kind: ValueKeyFloat64, Bytes: float64ValueKey(v, class)}, nil
	case "duration":
		dur, err := value.ParseDuration(normalized)
		if err != nil {
			return ValueKey{}, err
		}
		return ValueKey{Kind: ValueKeyDuration, Bytes: durationValueKey(dur)}, nil
	case "hexBinary":
		b, err := value.ParseHexBinary([]byte(normalized))
		if err != nil {
			return ValueKey{}, err
		}
		return ValueKey{Kind: ValueKeyBinary, Bytes: binaryValueKey(0, b)}, nil
	case "base64Binary":
		b, err := value.ParseBase64Binary([]byte(normalized))
		if err != nil {
			return ValueKey{}, err
		}
		return ValueKey{Kind: ValueKeyBinary, Bytes: binaryValueKey(1, b)}, nil
	case "QName":
		qn, err := xsdlex.ParseQNameValue(normalized, ctx)
		if err != nil {
			return ValueKey{}, err
		}
		return ValueKey{Kind: ValueKeyQName, Bytes: qNameValueKey(0, qn.Namespace, qn.Local)}, nil
	case "NOTATION":
		qn, err := xsdlex.ParseQNameValue(normalized, ctx)
		if err != nil {
			return ValueKey{}, err
		}
		return ValueKey{Kind: ValueKeyQName, Bytes: qNameValueKey(1, qn.Namespace, qn.Local)}, nil
	default:
		if kind, ok := value.KindFromPrimitiveName(primitive); ok {
			tv, err := value.Parse(kind, []byte(normalized))
			if err != nil {
				return ValueKey{}, err
			}
			key, err := temporalValueKey(tv)
			if err != nil {
				return ValueKey{}, err
			}
			return ValueKey{Kind: ValueKeyDateTime, Bytes: key}, nil
		}
		return ValueKey{}, fmt.Errorf("unsupported primitive type %s", primitive)
	}
}

func primitiveNameForValueKey(spec SimpleTypeSpec) string {
	if spec.Primitive != "" {
		return spec.Primitive
	}
	if spec.Name.Local == "anyType" || spec.Name.Local == "anySimpleType" {
		return "anySimpleType"
	}
	if spec.BuiltinBase != "" {
		return spec.BuiltinBase
	}
	return spec.Name.Local
}

func specBuiltinNameForValueKey(spec SimpleTypeSpec) string {
	if spec.BuiltinBase != "" {
		return spec.BuiltinBase
	}
	if spec.Name.Local != "" {
		return spec.Name.Local
	}
	return primitiveNameForValueKey(spec)
}

func stringValueKey(tag byte, data string) []byte {
	out := make([]byte, 1+len(data))
	out[0] = tag
	copy(out[1:], data)
	return out
}

func binaryValueKey(tag byte, data []byte) []byte {
	out := make([]byte, 1, 1+len(data))
	out[0] = tag
	out = append(out, data...)
	return out
}

func qNameValueKey(tag byte, ns, local string) []byte {
	out := make([]byte, 0, 1+binary.MaxVarintLen64*2+len(ns)+len(local))
	out = append(out, tag)
	out = appendValueKeyUvarint(out, uint64(len(ns)))
	out = append(out, ns...)
	out = appendValueKeyUvarint(out, uint64(len(local)))
	out = append(out, local...)
	return out
}

const (
	canonicalNaN32 = 0x7fc00000
	canonicalNaN64 = 0x7ff8000000000000
)

func float32ValueKey(floatVal float32, class num.FloatClass) []byte {
	var bits uint32
	switch class {
	case num.FloatNaN:
		bits = canonicalNaN32
	default:
		if floatVal == 0 {
			bits = 0
		} else {
			bits = math.Float32bits(floatVal)
		}
	}
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, bits)
	return out
}

func float64ValueKey(floatVal float64, class num.FloatClass) []byte {
	var bits uint64
	switch class {
	case num.FloatNaN:
		bits = canonicalNaN64
	default:
		if floatVal == 0 {
			bits = 0
		} else {
			bits = math.Float64bits(floatVal)
		}
	}
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, bits)
	return out
}

func temporalValueKey(v value.Value) ([]byte, error) {
	subkind, ok := temporalSubkind(v.Kind)
	if !ok {
		return nil, fmt.Errorf("unsupported temporal kind %d", v.Kind)
	}
	t := v.Time
	tzFlag := byte(0)
	if v.TimezoneKind == value.TZKnown {
		tzFlag = 1
		t = t.UTC()
	}
	if subkind == 2 {
		seconds := t.Hour()*3600 + t.Minute()*60 + t.Second()
		out := make([]byte, 11)
		out[0] = subkind
		out[1] = tzFlag
		binary.BigEndian.PutUint32(out[2:], uint32(seconds))
		binary.BigEndian.PutUint32(out[6:], uint32(t.Nanosecond()))
		out[10] = leapSecondFlag(v.LeapSecond)
		return out, nil
	}
	year, month, day := t.Date()
	hour, minute, sec := t.Clock()
	keyLen := 20
	if subkind == 0 {
		keyLen = 21
	}
	out := make([]byte, keyLen)
	out[0] = subkind
	out[1] = tzFlag
	binary.BigEndian.PutUint32(out[2:], uint32(int32(year)))
	binary.BigEndian.PutUint16(out[6:], uint16(month))
	binary.BigEndian.PutUint16(out[8:], uint16(day))
	binary.BigEndian.PutUint16(out[10:], uint16(hour))
	binary.BigEndian.PutUint16(out[12:], uint16(minute))
	binary.BigEndian.PutUint16(out[14:], uint16(sec))
	binary.BigEndian.PutUint32(out[16:], uint32(t.Nanosecond()))
	if subkind == 0 {
		out[20] = leapSecondFlag(v.LeapSecond)
	}
	return out, nil
}

func temporalSubkind(kind value.Kind) (byte, bool) {
	switch kind {
	case value.KindDateTime:
		return 0, true
	case value.KindDate:
		return 1, true
	case value.KindTime:
		return 2, true
	case value.KindGYearMonth:
		return 3, true
	case value.KindGYear:
		return 4, true
	case value.KindGMonthDay:
		return 5, true
	case value.KindGDay:
		return 6, true
	case value.KindGMonth:
		return 7, true
	default:
		return 0, false
	}
}

func durationValueKey(dur value.Duration) []byte {
	months := durationMonthsTotal(dur)
	seconds := durationSecondsTotal(dur)
	sign := byte(1)
	if dur.Negative {
		sign = 2
	}
	if months.Sign == 0 && seconds.Sign == 0 {
		sign = 0
	}
	out := []byte{sign}
	out = num.EncodeDecKey(out, months.AsDec())
	out = num.EncodeDecKey(out, seconds)
	return out
}

func durationMonthsTotal(dur value.Duration) num.Int {
	years := num.FromInt64(int64(dur.Years))
	months := num.FromInt64(int64(dur.Months))
	if years.Sign == 0 {
		return months
	}
	return num.Add(num.Mul(years, num.FromInt64(12)), months)
}

func durationSecondsTotal(dur value.Duration) num.Dec {
	total := dur.Seconds
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Minutes)), num.FromInt64(60)))
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Hours)), num.FromInt64(3600)))
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Days)), num.FromInt64(86400)))
	return total
}

func appendValueKeyUvarint(dst []byte, v uint64) []byte {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], v)
	return append(dst, buf[:n]...)
}

func appendValueKeyListEntry(dst []byte, kind byte, key []byte) []byte {
	dst = append(dst, kind)
	dst = appendValueKeyUvarint(dst, uint64(len(key)))
	return append(dst, key...)
}

func leapSecondFlag(leapSecond bool) byte {
	if leapSecond {
		return 1
	}
	return 0
}

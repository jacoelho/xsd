package runtime

import "encoding/binary"

// ValueKind identifies the primitive value space used for comparisons.
type ValueKind uint8

const (
	VKInvalid ValueKind = iota
	VKString
	VKBool
	VKInteger
	VKDecimal
	VKFloat32
	VKFloat64
	VKDuration
	VKDateTime
	VKDate
	VKTime
	VKGYearMonth
	VKGYear
	VKGMonthDay
	VKGDay
	VKGMonth
	VKAnyURI
	VKQName
	VKHexBinary
	VKBase64Binary
	VKList
)

// ValueKey is the canonical, immutable representation of a value.
type ValueKey struct {
	Kind ValueKind
	Hash uint64
	Ref  ValueRef
}

// ValueKindForValidatorKind maps a validator kind to a value-space kind.
func ValueKindForValidatorKind(kind ValidatorKind) (ValueKind, bool) {
	switch kind {
	case VString:
		return VKString, true
	case VBoolean:
		return VKBool, true
	case VInteger:
		return VKInteger, true
	case VDecimal:
		return VKDecimal, true
	case VFloat:
		return VKFloat32, true
	case VDouble:
		return VKFloat64, true
	case VDuration:
		return VKDuration, true
	case VDateTime:
		return VKDateTime, true
	case VDate:
		return VKDate, true
	case VTime:
		return VKTime, true
	case VGYearMonth:
		return VKGYearMonth, true
	case VGYear:
		return VKGYear, true
	case VGMonthDay:
		return VKGMonthDay, true
	case VGDay:
		return VKGDay, true
	case VGMonth:
		return VKGMonth, true
	case VAnyURI:
		return VKAnyURI, true
	case VQName, VNotation:
		return VKQName, true
	case VHexBinary:
		return VKHexBinary, true
	case VBase64Binary:
		return VKBase64Binary, true
	case VList:
		return VKList, true
	default:
		return VKInvalid, false
	}
}

// AppendListKey appends a typed key entry to a list key buffer.
// The format is: kind (1 byte) + varint(len) + key bytes.
func AppendListKey(dst []byte, kind ValueKind, key []byte) []byte {
	out := dst
	out = append(out, byte(kind))
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(len(key)))
	out = append(out, buf[:n]...)
	out = append(out, key...)
	return out
}

package runtime

import "encoding/binary"

// ValueKind identifies the primitive value space used for comparisons.
type ValueKind uint8

const (
	// VKInvalid is an exported constant.
	VKInvalid ValueKind = iota
	// VKBool is an exported constant.
	VKBool
	// VKDecimal is an exported constant.
	VKDecimal
	// VKFloat32 is an exported constant.
	VKFloat32
	// VKFloat64 is an exported constant.
	VKFloat64
	// VKString is an exported constant.
	VKString
	// VKBinary is an exported constant.
	VKBinary
	// VKQName is an exported constant.
	VKQName
	// VKDateTime is an exported constant.
	VKDateTime
	// VKDuration is an exported constant.
	VKDuration
	// VKList is an exported constant.
	VKList
)

// ValueKey is the canonical, immutable representation of a value.
type ValueKey struct {
	Bytes []byte
	Hash  uint64
	Kind  ValueKind
}

// ValueKeyRef stores a semantic value key in the shared value blob.
type ValueKeyRef struct {
	Kind ValueKind
	Ref  ValueRef
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

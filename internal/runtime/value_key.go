package runtime

import "encoding/binary"

// ValueKind identifies the primitive value space used for comparisons.
type ValueKind uint8

const (
	VKInvalid ValueKind = iota
	VKBool
	VKDecimal
	VKFloat32
	VKFloat64
	VKString
	VKBinary
	VKQName
	VKDateTime
	VKDuration
	VKList
)

// ValueKey is the canonical, immutable representation of a value.
type ValueKey struct {
	Bytes []byte
	Hash  uint64
	Kind  ValueKind
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

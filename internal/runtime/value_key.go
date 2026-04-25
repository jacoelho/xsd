package runtime

import "github.com/jacoelho/xsd/internal/valuekey"

// ValueKind identifies the primitive value space used for comparisons.
type ValueKind = valuekey.Kind

const (
	VKInvalid  = valuekey.Invalid
	VKBool     = valuekey.Bool
	VKDecimal  = valuekey.Decimal
	VKFloat32  = valuekey.Float32
	VKFloat64  = valuekey.Float64
	VKString   = valuekey.String
	VKBinary   = valuekey.Binary
	VKQName    = valuekey.QName
	VKDateTime = valuekey.DateTime
	VKDuration = valuekey.Duration
	VKList     = valuekey.List
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
	return valuekey.AppendListEntry(dst, byte(kind), key)
}

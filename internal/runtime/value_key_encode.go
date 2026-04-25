package runtime

import (
	"time"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
	"github.com/jacoelho/xsd/internal/valuekey"
)

// StringKeyBytes writes a tagged string key into dst, reusing capacity; dst is overwritten from the start.
func StringKeyBytes(dst []byte, tag byte, data []byte) []byte {
	return valuekey.StringBytes(dst, tag, data)
}

// StringKeyString returns a tagged string key for a Go string.
func StringKeyString(tag byte, data string) []byte {
	return valuekey.StringString(tag, data)
}

// BinaryKeyBytes writes a tagged binary key into dst, reusing capacity; dst is overwritten from the start.
func BinaryKeyBytes(dst []byte, tag byte, data []byte) []byte {
	return valuekey.BinaryBytes(dst, tag, data)
}

// QNameKeyStrings returns a tagged QName key from namespace/local strings.
func QNameKeyStrings(tag byte, ns, local string) []byte {
	return valuekey.QNameStrings(tag, ns, local)
}

// QNameKeyCanonical writes a tagged QName key into dst, reusing capacity; dst is overwritten from the start.
func QNameKeyCanonical(dst []byte, tag byte, canonical []byte) []byte {
	return valuekey.QNameCanonical(dst, tag, canonical)
}

// Float32Key appends the canonical float32 key encoding to dst.
func Float32Key(dst []byte, floatVal float32, class num.FloatClass) []byte {
	return valuekey.Float32Bytes(dst, floatVal, class)
}

// Float64Key appends the canonical float64 key encoding to dst.
func Float64Key(dst []byte, floatVal float64, class num.FloatClass) []byte {
	return valuekey.Float64Bytes(dst, floatVal, class)
}

// TemporalKeyBytes appends a canonical temporal key encoding to dst.
func TemporalKeyBytes(dst []byte, subkind byte, t time.Time, tzKind value.TimezoneKind, leapSecond bool) []byte {
	return valuekey.TemporalBytes(dst, subkind, t, tzKind, leapSecond)
}

// TemporalSubkind maps a temporal kind to its compact key subkind.
func TemporalSubkind(kind value.Kind) (byte, bool) {
	return valuekey.TemporalSubkind(kind)
}

// TemporalKeyFromValue appends a canonical temporal key for v to dst.
func TemporalKeyFromValue(dst []byte, v value.Value) ([]byte, error) {
	return valuekey.TemporalFromValue(dst, v)
}

// DurationKeyBytes appends a canonical duration key encoding to dst.
func DurationKeyBytes(dst []byte, dur value.Duration) []byte {
	return valuekey.DurationBytes(dst, dur)
}

// AppendUvarint appends v as a varint-encoded uint64.
func AppendUvarint(dst []byte, v uint64) []byte {
	return valuekey.AppendUvarint(dst, v)
}

// AppendListEntry appends one typed list item: kind (1 byte), len (uvarint), key bytes.
func AppendListEntry(dst []byte, kind byte, key []byte) []byte {
	return valuekey.AppendListEntry(dst, kind, key)
}

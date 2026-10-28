package value

import (
	"strconv"
	"strings"
)

// PrimitiveIdentityKey builds the value-space identity key for one primitive
// value. The separator and primitive tag are part of the schema/runtime
// identity contract; callers must pass the primitive's canonical identity
// spelling, not its source lexical form.
func PrimitiveIdentityKey(kind PrimitiveKind, canonical string) string {
	var b strings.Builder
	b.Grow(2 + len(canonical))
	appendPrimitiveIdentity(&b, kind, canonical)
	return b.String()
}

func appendPrimitiveIdentity(b *strings.Builder, kind PrimitiveKind, canonical string) {
	b.WriteByte(byte(kind))
	b.WriteByte('\x1e')
	b.WriteString(canonical)
}

func appendPrimitiveIdentityItem(b *strings.Builder, kind PrimitiveKind, canonical string, digits *[20]byte) {
	b.Write(strconv.AppendInt(digits[:0], int64(len(canonical))+2, 10))
	b.WriteByte(':')
	appendPrimitiveIdentity(b, kind, canonical)
}

func appendListIdentityItem(b *strings.Builder, item string, digits *[20]byte) {
	b.Write(strconv.AppendInt(digits[:0], int64(len(item)), 10))
	b.WriteByte(':')
	b.WriteString(item)
}

// listIdentityTag marks a list key in place of a primitive kind. It lies
// outside the PrimitiveKind range, so no list key, including an empty list's,
// equals an atomic key (I-017).
const listIdentityTag = 0xff

// ListIdentityKey builds the value-space identity key for a list. Each item
// is length-framed so adjacent primitive keys cannot collide. Items are
// expected to be complete PrimitiveIdentityKey values.
func ListIdentityKey(items []string) string {
	var b strings.Builder
	// The exact size is not knowable without walking every item, but reserving
	// the list tag avoids a separate allocation for the common empty-list case.
	b.Grow(2)
	appendListIdentityTag(&b)
	var digits [20]byte
	for _, item := range items {
		appendListIdentityItem(&b, item, &digits)
	}
	return b.String()
}

// framedListIdentityKey tags an already framed item sequence as a list key.
func framedListIdentityKey(framed string) string {
	var b strings.Builder
	b.Grow(2 + len(framed))
	appendListIdentityTag(&b)
	b.WriteString(framed)
	return b.String()
}

func appendListIdentityTag(b *strings.Builder) {
	b.WriteByte(listIdentityTag)
	b.WriteByte('\x1e')
}

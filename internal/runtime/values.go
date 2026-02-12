package runtime

import "regexp"

// ValueRef defines an exported type.
type ValueRef struct {
	Off     uint32
	Len     uint32
	Hash    uint64
	Present bool
}

// ValueBlob defines an exported type.
type ValueBlob struct {
	Blob []byte
}

// Pattern defines an exported type.
type Pattern struct {
	Re     *regexp.Regexp
	Source []byte
}

// EnumTable defines an exported type.
type EnumTable struct {
	Off []uint32
	Len []uint32

	Keys []ValueKey

	HashOff []uint32
	HashLen []uint32
	Hashes  []uint64
	Slots   []uint32
}

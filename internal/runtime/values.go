package runtime

import "regexp"

// ValueRef references value ref data in packed tables.
type ValueRef struct {
	Off     uint32
	Len     uint32
	Hash    uint64
	Present bool
}

// ValueBlob stores canonical lexical bytes used by defaults/fixed/enums.
type ValueBlob struct {
	Blob []byte
}

// Pattern stores one compiled regex plus its original source.
type Pattern struct {
	Re     *regexp.Regexp
	Source []byte
}

// EnumTable stores packed enumeration keys and hash indexes.
type EnumTable struct {
	Off []uint32
	Len []uint32

	Keys []ValueKey

	HashOff []uint32
	HashLen []uint32
	Hashes  []uint64
	Slots   []uint32
}

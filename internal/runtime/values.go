package runtime

import "regexp"

type ValueRef struct {
	Off     uint32
	Len     uint32
	Hash    uint64
	Present bool
}

type ValueBlob struct {
	Blob []byte
}

type Pattern struct {
	Re *regexp.Regexp
}

type EnumTable struct {
	Off []uint32
	Len []uint32

	Values []ValueRef

	HashOff []uint32
	HashLen []uint32
	Hashes  []uint64
	Slots   []uint32
}

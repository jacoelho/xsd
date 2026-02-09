package validatorcompile

import "github.com/jacoelho/xsd/internal/runtime"

type valueBuilder struct {
	index map[string]runtime.ValueRef
	blob  []byte
}

func (b *valueBuilder) addWithHash(value []byte, hash uint64) runtime.ValueRef {
	if b.index == nil {
		b.index = make(map[string]runtime.ValueRef)
	}
	key := string(value)
	if ref, ok := b.index[key]; ok {
		return ref
	}
	ref := runtime.ValueRef{
		Off:     uint32(len(b.blob)),
		Len:     uint32(len(value)),
		Hash:    hash,
		Present: true,
	}
	b.blob = append(b.blob, value...)
	b.index[key] = ref
	return ref
}

func (b *valueBuilder) table() runtime.ValueBlob {
	return runtime.ValueBlob{Blob: b.blob}
}

type enumBuilder struct {
	off     []uint32
	len     []uint32
	keys    []runtime.ValueKey
	hashOff []uint32
	hashLen []uint32
	hashes  []uint64
	slots   []uint32
}

func (b *enumBuilder) add(keys []runtime.ValueKey) runtime.EnumID {
	if len(keys) == 0 {
		return 0
	}
	b.ensureInit()

	off := uint32(len(b.keys))
	b.keys = append(b.keys, keys...)
	b.off = append(b.off, off)
	b.len = append(b.len, uint32(len(keys)))

	tableSize := max(runtime.NextPow2(len(keys)*2), 1)
	hashOff := uint32(len(b.hashes))
	b.hashOff = append(b.hashOff, hashOff)
	b.hashLen = append(b.hashLen, uint32(tableSize))

	start := len(b.hashes)
	b.hashes = append(b.hashes, make([]uint64, tableSize)...)
	b.slots = append(b.slots, make([]uint32, tableSize)...)

	mask := uint64(tableSize - 1)
	for i, key := range keys {
		h := key.Hash
		slot := int(h & mask)
		for {
			idx := start + slot
			if b.slots[idx] == 0 {
				b.hashes[idx] = h
				b.slots[idx] = uint32(i) + 1
				break
			}
			slot = (slot + 1) & int(mask)
		}
	}
	return runtime.EnumID(len(b.off) - 1)
}

func (b *enumBuilder) table() runtime.EnumTable {
	return runtime.EnumTable{
		Off:     b.off,
		Len:     b.len,
		Keys:    b.keys,
		HashOff: b.hashOff,
		HashLen: b.hashLen,
		Hashes:  b.hashes,
		Slots:   b.slots,
	}
}

func (b *enumBuilder) ensureInit() {
	if len(b.off) > 0 {
		return
	}
	b.off = []uint32{0}
	b.len = []uint32{0}
	b.hashOff = []uint32{0}
	b.hashLen = []uint32{0}
}

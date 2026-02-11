package runtimeassemble

import (
	"encoding/binary"
	"hash"
	"hash/fnv"

	"github.com/jacoelho/xsd/internal/runtime"
)

type hashBuilder struct {
	h   hash.Hash64
	buf [8]byte
}

func newHashBuilder() *hashBuilder {
	return &hashBuilder{h: fnv.New64a()}
}

func (b *hashBuilder) write(p []byte) {
	// hash.Hash.Write never returns an error for standard library hashes.
	_, _ = b.h.Write(p)
}

func (b *hashBuilder) WriteU8(v uint8) {
	b.buf[0] = v
	b.write(b.buf[:1])
}

func (b *hashBuilder) WriteU32(v uint32) {
	binary.LittleEndian.PutUint32(b.buf[:4], v)
	b.write(b.buf[:4])
}

func (b *hashBuilder) WriteU64(v uint64) {
	binary.LittleEndian.PutUint64(b.buf[:8], v)
	b.write(b.buf[:8])
}

func (b *hashBuilder) WriteBool(v bool) {
	if v {
		b.WriteU8(1)
		return
	}
	b.WriteU8(0)
}

func (b *hashBuilder) WriteBytes(data []byte) {
	b.WriteU32(uint32(len(data)))
	if len(data) == 0 {
		return
	}
	b.write(data)
}

func (b *hashBuilder) sum64() uint64 {
	sum := b.h.Sum64()
	if sum == 0 {
		return 1
	}
	return sum
}

func computeBuildHash(rt *runtime.Schema) uint64 {
	if rt == nil {
		return 0
	}
	h := newHashBuilder()
	runtime.WriteFingerprint(h, rt)
	return h.sum64()
}

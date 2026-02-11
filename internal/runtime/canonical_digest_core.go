package runtime

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

type digestBuilder struct {
	h   hash.Hash
	buf [8]byte
}

func newDigestBuilder() *digestBuilder {
	return &digestBuilder{h: sha256.New()}
}

func (b *digestBuilder) write(p []byte) {
	// hash.Hash.Write never returns an error for standard library hashes.
	_, _ = b.h.Write(p)
}

func (b *digestBuilder) WriteU8(v uint8) {
	b.buf[0] = v
	b.write(b.buf[:1])
}

func (b *digestBuilder) WriteU32(v uint32) {
	binary.LittleEndian.PutUint32(b.buf[:4], v)
	b.write(b.buf[:4])
}

func (b *digestBuilder) WriteU64(v uint64) {
	binary.LittleEndian.PutUint64(b.buf[:8], v)
	b.write(b.buf[:8])
}

func (b *digestBuilder) WriteBool(v bool) {
	if v {
		b.WriteU8(1)
		return
	}
	b.WriteU8(0)
}

func (b *digestBuilder) WriteBytes(data []byte) {
	b.WriteU32(uint32(len(data)))
	if len(data) == 0 {
		return
	}
	b.write(data)
}

func (b *digestBuilder) sum() [32]byte {
	var out [32]byte
	sum := b.h.Sum(nil)
	copy(out[:], sum)
	return out
}

// CanonicalDigest returns a deterministic digest of canonical schema artifacts.
func (s *Schema) CanonicalDigest() [32]byte {
	if s == nil {
		return [32]byte{}
	}
	h := newDigestBuilder()
	WriteFingerprint(h, s)
	return h.sum()
}

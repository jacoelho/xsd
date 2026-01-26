package models

import (
	"encoding/binary"
	"math/bits"
	"strings"
)

type bitset struct {
	words []uint64
	size  int
}

func newBitset(size int) *bitset {
	if size <= 0 {
		return &bitset{size: size}
	}
	return &bitset{
		words: make([]uint64, (size+63)/64),
		size:  size,
	}
}

func (b *bitset) set(i int) {
	b.words[i/64] |= 1 << (i % 64)
}

func (b *bitset) or(other *bitset) {
	if other == nil {
		return
	}
	for i := range b.words {
		if i < len(other.words) {
			b.words[i] |= other.words[i]
		}
	}
}

func (b *bitset) clone() *bitset {
	clone := newBitset(b.size)
	copy(clone.words, b.words)
	return clone
}

func (b *bitset) empty() bool {
	for _, w := range b.words {
		if w != 0 {
			return false
		}
	}
	return true
}

func (b *bitset) forEach(f func(int)) {
	for i, w := range b.words {
		for w != 0 {
			bit := bits.TrailingZeros64(w)
			f(i*64 + bit)
			w &^= 1 << bit
		}
	}
}

func (b *bitset) key() string {
	if len(b.words) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(b.words) * 8)
	var buf [8]byte
	for _, w := range b.words {
		binary.LittleEndian.PutUint64(buf[:], w)
		_, _ = sb.Write(buf[:])
	}
	return sb.String()
}

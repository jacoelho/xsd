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

const (
	// bitsetWordBits is the number of bits in a uint64.
	bitsetWordBits = 64
	// bitsetWordBytes is the byte width of a uint64.
	bitsetWordBytes = bitsetWordBits / 8
)

func newBitset(size int) *bitset {
	if size <= 0 {
		return &bitset{size: size}
	}
	return &bitset{
		words: make([]uint64, (size+bitsetWordBits-1)/bitsetWordBits),
		size:  size,
	}
}

func (b *bitset) set(i int) {
	if b == nil || i < 0 || i >= b.size {
		return
	}
	b.words[i/bitsetWordBits] |= 1 << (i % bitsetWordBits)
}

func (b *bitset) or(other *bitset) {
	if b == nil || other == nil {
		return
	}
	for i := range b.words {
		if i < len(other.words) {
			b.words[i] |= other.words[i]
		}
	}
}

func (b *bitset) clone() *bitset {
	if b == nil {
		return nil
	}
	clone := newBitset(b.size)
	copy(clone.words, b.words)
	return clone
}

func (b *bitset) empty() bool {
	if b == nil {
		return true
	}
	for _, w := range b.words {
		if w != 0 {
			return false
		}
	}
	return true
}

func (b *bitset) forEach(f func(int)) {
	if b == nil {
		return
	}
	for i, w := range b.words {
		base := i * bitsetWordBits
		for w != 0 {
			bit := bits.TrailingZeros64(w)
			idx := base + bit
			if idx >= b.size {
				return
			}
			f(idx)
			w &^= 1 << bit
		}
	}
}

func (b *bitset) intersectionIndex(other *bitset) (int, bool) {
	if b == nil || other == nil {
		return 0, false
	}
	n := min(len(other.words), len(b.words))
	for i := range n {
		w := b.words[i] & other.words[i]
		if w != 0 {
			return i*bitsetWordBits + bits.TrailingZeros64(w), true
		}
	}
	return 0, false
}

func (b *bitset) key() string {
	if b == nil {
		return ""
	}
	if len(b.words) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(b.words) * bitsetWordBytes)
	var buf [8]byte
	for _, w := range b.words {
		binary.LittleEndian.PutUint64(buf[:], w)
		_, _ = sb.Write(buf[:])
	}
	return sb.String()
}

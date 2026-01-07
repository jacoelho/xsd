package contentmodel

import (
	"encoding/binary"
	"math/bits"
	"strconv"
	"strings"
)

// bitset is a compact set of non-negative integers.
type bitset struct {
	words []uint64
	n     int // capacity
}

func newBitset(n int) *bitset {
	return &bitset{
		words: make([]uint64, (n+63)/64),
		n:     n,
	}
}

func (b *bitset) set(i int)       { b.words[i/64] |= 1 << (i % 64) }
func (b *bitset) test(i int) bool { return b.words[i/64]&(1<<(i%64)) != 0 }

func (b *bitset) or(other *bitset) {
	for i := range b.words {
		if i < len(other.words) {
			b.words[i] |= other.words[i]
		}
	}
}

func (b *bitset) clone() *bitset {
	c := newBitset(b.n)
	copy(c.words, b.words)
	return c
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

func (b *bitset) clear() {
	for i := range b.words {
		b.words[i] = 0
	}
}

// key returns an opaque byte string used for fast state lookup.
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

// String returns a hex representation for use as map key.
func (b *bitset) String() string {
	var sb strings.Builder
	for i, w := range b.words {
		if i > 0 {
			sb.WriteByte(':')
		}
		hex := strconv.FormatUint(w, 16)
		// pad to 16 hex digits
		if len(hex) < 16 {
			hex = strings.Repeat("0", 16-len(hex)) + hex
		}
		sb.WriteString(strings.ToUpper(hex))
	}
	return sb.String()
}

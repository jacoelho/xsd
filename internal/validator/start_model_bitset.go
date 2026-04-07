package validator

import (
	"math/bits"

	"github.com/jacoelho/xsd/internal/runtime"
)

func bitsetSlice(blob runtime.BitsetBlob, ref runtime.BitsetRef) ([]uint64, bool) {
	if ref.Len == 0 {
		return nil, true
	}
	off := int(ref.Off)
	end := off + int(ref.Len)
	if off < 0 || end < 0 || end > len(blob.Words) {
		return nil, false
	}
	return blob.Words[off:end], true
}

func bitsetZero(words []uint64) {
	clear(words)
}

func bitsetOr(dst, src []uint64) {
	for i := range dst {
		if i < len(src) {
			dst[i] |= src[i]
		}
	}
}

func bitsetEmpty(words []uint64) bool {
	for _, w := range words {
		if w != 0 {
			return false
		}
	}
	return true
}

func bitsetIntersects(a, b []uint64) bool {
	for i := range min(len(a), len(b)) {
		if a[i]&b[i] != 0 {
			return true
		}
	}
	return false
}

func forEachBit(words []uint64, limit int, fn func(int)) {
	for wi, w := range words {
		for w != 0 {
			bit := bits.TrailingZeros64(w)
			pos := wi*64 + bit
			if pos >= limit {
				return
			}
			fn(pos)
			w &^= 1 << bit
		}
	}
}

func setBit(words []uint64, pos int) {
	if pos < 0 {
		return
	}
	word := pos / 64
	bit := uint(pos % 64)
	if word >= len(words) {
		return
	}
	words[word] |= 1 << bit
}

func allHas(words []uint64, idx int) bool {
	if idx < 0 {
		return false
	}
	word := idx / 64
	bit := uint(idx % 64)
	if word >= len(words) {
		return false
	}
	return words[word]&(1<<bit) != 0
}

func allSet(words []uint64, idx int) {
	if idx < 0 {
		return
	}
	word := idx / 64
	bit := uint(idx % 64)
	if word >= len(words) {
		return
	}
	words[word] |= 1 << bit
}

package contentmodel

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

const maxInt = int(^uint(0) >> 1)

func addCount(a, b int) (int, error) {
	if a > maxInt-b {
		return 0, fmt.Errorf("content model too large")
	}
	return a + b, nil
}

func mulCount(a, b int) (int, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	if a > maxInt/b {
		return 0, fmt.Errorf("content model too large")
	}
	return a * b, nil
}

func packBitset(blob *runtime.BitsetBlob, set *bitset) runtime.BitsetRef {
	if set == nil || len(set.words) == 0 {
		return runtime.BitsetRef{}
	}
	off := uint32(len(blob.Words))
	blob.Words = append(blob.Words, set.words...)
	return runtime.BitsetRef{Off: off, Len: uint32(len(set.words))}
}

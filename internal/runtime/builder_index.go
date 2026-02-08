package runtime

import "fmt"

func buildOpenAddressIndex[T ~uint32](count int, name string, hashForID func(id T) (uint64, error)) ([]uint64, []T, error) {
	size := NextPow2(count * 2)
	hashes := make([]uint64, size)
	ids := make([]T, size)
	mask := uint64(size - 1)

	for i := 1; i <= count; i++ {
		id := T(i)
		h, err := hashForID(id)
		if err != nil {
			return nil, nil, err
		}
		slot := int(h & mask)
		inserted := false
		for range size {
			if ids[slot] == 0 {
				ids[slot] = id
				hashes[slot] = h
				inserted = true
				break
			}
			slot = int((uint64(slot) + 1) & mask)
		}
		if !inserted {
			return nil, nil, fmt.Errorf("%s index table full", name)
		}
	}

	return hashes, ids, nil
}

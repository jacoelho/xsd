package models

import "testing"

func TestBitsetSetBoundsCheck(t *testing.T) {
	bs := newBitset(64)
	cases := []int{-1, 64, 65, 1000}
	for _, idx := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("set(%d) panicked: %v", idx, r)
				}
			}()
			bs.set(idx)
		}()
	}
}

func TestBitsetSetInRange(t *testing.T) {
	bs := newBitset(64)
	bs.set(0)
	if bs.words[0] == 0 {
		t.Fatalf("expected bit 0 to be set")
	}
}

func TestBitsetForEachBounds(t *testing.T) {
	size := 70
	bs := newBitset(size)
	bs.set(0)
	bs.set(63)
	bs.set(64)
	bs.set(69)

	// simulate a stray bit beyond size
	bs.words[1] |= 1 << 6 // bit 70

	var indices []int
	bs.forEach(func(i int) {
		indices = append(indices, i)
	})

	for _, idx := range indices {
		if idx >= size {
			t.Fatalf("forEach yielded invalid index %d (size=%d)", idx, size)
		}
	}
}

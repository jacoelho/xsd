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

func TestBitsetNilReceiver(t *testing.T) {
	var bs *bitset

	assertNotPanics(t, func() { bs.set(1) })
	assertNotPanics(t, func() { bs.or(nil) })
	assertNotPanics(t, func() { _ = bs.clone() })
	assertNotPanics(t, func() { _ = bs.empty() })
	assertNotPanics(t, func() { bs.forEach(func(int) {}) })
	assertNotPanics(t, func() { _, _ = bs.intersectionIndex(nil) })
	assertNotPanics(t, func() { _ = bs.key() })

	if !bs.empty() {
		t.Fatalf("nil bitset should be empty")
	}
	if bs.clone() != nil {
		t.Fatalf("nil bitset clone should be nil")
	}
	if bs.key() != "" {
		t.Fatalf("nil bitset key should be empty")
	}
	called := false
	bs.forEach(func(int) { called = true })
	if called {
		t.Fatalf("forEach should not call callback on nil bitset")
	}
}

func assertNotPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	fn()
}

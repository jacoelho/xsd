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

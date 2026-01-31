package validator

import "testing"

func TestArenaResetClearsBuffer(t *testing.T) {
	var a Arena
	buf := a.Alloc(4)
	for i := range buf {
		buf[i] = 0xff
	}
	a.Reset()

	buf = a.Alloc(4)
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("byte %d = %d, want 0", i, b)
		}
	}
}

func TestArenaResetOverflowCount(t *testing.T) {
	a := Arena{Max: 1}
	_ = a.Alloc(2)
	if a.OverflowCount == 0 {
		t.Fatalf("expected overflow count to increment")
	}
	a.Reset()
	if a.OverflowCount != 0 {
		t.Fatalf("overflow count = %d, want 0", a.OverflowCount)
	}
}

package xmltext

import "testing"

func TestInterningHelpers(t *testing.T) {
	interner := newNameInterner(2)
	if got := interner.internBytes(nil, -1); got.Full.buf != nil {
		t.Fatalf("internBytes empty = %v, want zero", got)
	}
	first := interner.intern([]byte("name"))
	second := interner.intern([]byte("name"))
	if first.Full.buf == nil || second.Full.buf == nil {
		t.Fatalf("interned spans missing buffers")
	}
	if interner.stats.Hits == 0 {
		t.Fatalf("intern hits = 0, want > 0")
	}

	for i := 0; i < nameInternerRecentSize+1; i++ {
		name := []byte{byte('a' + i)}
		_ = interner.intern(name)
	}

	limit := &nameInterner{maxEntries: -1}
	_ = limit.internBytesHash([]byte("x"), -1, hashBytes([]byte("x")))
	if limit.maxEntries != 0 {
		t.Fatalf("maxEntries = %d, want 0", limit.maxEntries)
	}
	limit.maxEntries = 1
	_ = limit.internBytesHash([]byte("a"), -1, hashBytes([]byte("a")))
	_ = limit.internBytesHash([]byte("b"), -1, hashBytes([]byte("b")))
	if limit.stats.Count != 1 {
		t.Fatalf("intern count = %d, want 1", limit.stats.Count)
	}
}

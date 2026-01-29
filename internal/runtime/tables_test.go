package runtime

import "testing"

func TestNamespaceTableEqualNamespaceBounds(t *testing.T) {
	table := NamespaceTable{
		Blob: []byte("abcdef"),
		Off:  []uint32{0, 2},
		Len:  []uint32{0, 3},
	}
	if table.equalNamespace(0, []byte("abc")) {
		t.Fatalf("expected id 0 to be invalid")
	}
	if table.equalNamespace(2, []byte("abc")) {
		t.Fatalf("expected out-of-range id to be invalid")
	}
	if !table.equalNamespace(1, []byte("cde")) {
		t.Fatalf("expected namespace to match")
	}

	table.Len[1] = 10
	if table.equalNamespace(1, []byte("cde")) {
		t.Fatalf("expected out-of-bounds blob to be rejected")
	}
}

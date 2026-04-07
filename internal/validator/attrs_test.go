package validator

import "testing"

func TestAttrNamesInternStableIDs(t *testing.T) {
	var names AttrNames

	first := names.Intern(1, []byte("urn:test"), []byte("id"))
	second := names.Intern(1, []byte("urn:test"), []byte("id"))
	other := names.Intern(2, []byte("urn:test"), []byte("code"))

	if first == 0 {
		t.Fatal("first interned ID = 0, want non-zero")
	}
	if second != first {
		t.Fatalf("second interned ID = %d, want %d", second, first)
	}
	if other == first {
		t.Fatalf("other interned ID = %d, want different from %d", other, first)
	}
}

func TestAttrNamesResetDropsOversizedStorage(t *testing.T) {
	names := AttrNames{
		Buckets: map[uint64][]AttrNameID{
			1: {1},
			2: {2},
			3: {3},
		},
		Names: make([]AttrName, 3),
	}

	names.Reset(2)

	if names.Buckets != nil {
		t.Fatalf("Buckets = %#v, want nil", names.Buckets)
	}
	if names.Names != nil {
		t.Fatalf("Names = %#v, want nil", names.Names)
	}
}

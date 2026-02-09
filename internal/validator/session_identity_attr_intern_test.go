package validator

import "testing"

func TestInternIdentityAttrNameStableIDs(t *testing.T) {
	s := &Session{}

	first := s.internIdentityAttrName([]byte("urn:test"), []byte("id"))
	second := s.internIdentityAttrName([]byte("urn:test"), []byte("id"))
	other := s.internIdentityAttrName([]byte("urn:test"), []byte("code"))

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

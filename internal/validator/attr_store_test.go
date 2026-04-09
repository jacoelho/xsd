package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestStoreRawStabilizesAndStoresValue(t *testing.T) {
	t.Parallel()

	var stabilized bool
	got := StoreRaw(
		nil,
		Start{Value: []byte("lexical")},
		true,
		func(attr *Start) {
			stabilized = true
			attr.NameCached = true
		},
		func(value []byte) []byte {
			return append([]byte("stored:"), value...)
		},
	)
	if !stabilized {
		t.Fatal("stabilizeName was not called")
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if !got[0].NameCached || string(got[0].Value) != "stored:lexical" || got[0].KeyKind != runtime.VKInvalid || got[0].KeyBytes != nil {
		t.Fatalf("got[0] = %+v", got[0])
	}
}

func TestStoreCanonicalAppendsKeyedValue(t *testing.T) {
	t.Parallel()

	got := StoreCanonical(
		nil,
		Start{Value: []byte("ignored")},
		true,
		nil,
		[]byte("canon"),
		runtime.VKString,
		[]byte("key"),
	)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if string(got[0].Value) != "canon" || got[0].KeyKind != runtime.VKString || string(got[0].KeyBytes) != "key" {
		t.Fatalf("got[0] = %+v", got[0])
	}
}

func TestStoreRawSkipsWhenDisabled(t *testing.T) {
	t.Parallel()

	got := StoreRaw(
		[]Start{{Value: []byte("keep")}},
		Start{Value: []byte("new")},
		false,
		nil,
		nil,
	)
	if len(got) != 1 || string(got[0].Value) != "keep" {
		t.Fatalf("got = %#v, want unchanged slice", got)
	}
}

func TestStoreRawIdentityOmitsValue(t *testing.T) {
	t.Parallel()

	got := StoreRawIdentity(
		nil,
		Start{Value: []byte("lexical")},
		true,
		nil,
	)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Value != nil || got[0].KeyKind != runtime.VKInvalid || got[0].KeyBytes != nil {
		t.Fatalf("got[0] = %+v", got[0])
	}
}

func TestStoreCanonicalIdentityOmitsValue(t *testing.T) {
	t.Parallel()

	got := StoreCanonicalIdentity(
		nil,
		Start{Value: []byte("ignored")},
		true,
		nil,
		runtime.VKString,
		[]byte("key"),
	)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Value != nil || got[0].KeyKind != runtime.VKString || string(got[0].KeyBytes) != "key" {
		t.Fatalf("got[0] = %+v", got[0])
	}
}

func TestStoreRawIdentityDropsNameBytesWhenSymbolPresent(t *testing.T) {
	t.Parallel()

	var stabilized bool
	got := StoreRawIdentity(
		nil,
		Start{
			Local:   []byte("name"),
			NSBytes: []byte("urn:test"),
			Sym:     7,
			NS:      3,
		},
		true,
		func(*Start) { stabilized = true },
	)
	if stabilized {
		t.Fatal("stabilizeName was called for symbol-backed attr")
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Local != nil || got[0].NSBytes != nil || !got[0].NameCached {
		t.Fatalf("got[0] = %+v", got[0])
	}
}

func TestStoreCanonicalIdentityStabilizesUnresolvedName(t *testing.T) {
	t.Parallel()

	var stabilized bool
	got := StoreCanonicalIdentity(
		nil,
		Start{
			Local:   []byte("name"),
			NSBytes: []byte("urn:test"),
		},
		true,
		func(attr *Start) {
			stabilized = true
			attr.Local = append([]byte(nil), attr.Local...)
			attr.NSBytes = append([]byte(nil), attr.NSBytes...)
			attr.NameCached = true
		},
		runtime.VKString,
		[]byte("key"),
	)
	if !stabilized {
		t.Fatal("stabilizeName was not called for unresolved attr")
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if !got[0].NameCached || string(got[0].Local) != "name" || string(got[0].NSBytes) != "urn:test" {
		t.Fatalf("got[0] = %+v", got[0])
	}
}

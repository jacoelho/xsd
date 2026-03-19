package attrs

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

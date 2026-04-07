package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestTrackCanonicalListVisitsCanonicalItems(t *testing.T) {
	count := 0
	err := trackCanonicalList(
		runtime.ValidatorMeta{
			Kind:       runtime.VList,
			Index:      0,
			WhiteSpace: runtime.WSCollapse,
		},
		runtime.ValidatorsBundle{
			List: []runtime.ListValidator{{Item: 9}},
		},
		[]byte("a b"),
		func(itemValidator runtime.ValidatorID, canonical []byte) error {
			if itemValidator != 9 {
				t.Fatalf("item validator = %d, want 9", itemValidator)
			}
			count++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("trackCanonicalList() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("trackCanonicalList() item count = %d, want 2", count)
	}
}

func TestDeriveCanonicalListKeyBuildsListKey(t *testing.T) {
	kind, key, err := deriveCanonicalListKey(
		runtime.ValidatorMeta{
			Kind:       runtime.VList,
			Index:      0,
			WhiteSpace: runtime.WSCollapse,
		},
		runtime.ValidatorsBundle{
			List: []runtime.ListValidator{{Item: 4}},
		},
		[]byte("a b"),
		make([]byte, 0, 32),
		func(itemValidator runtime.ValidatorID, canonical []byte) (runtime.ValueKind, []byte, error) {
			if itemValidator != 4 {
				t.Fatalf("item validator = %d, want 4", itemValidator)
			}
			return runtime.VKString, append([]byte("k:"), canonical...), nil
		},
	)
	if err != nil {
		t.Fatalf("deriveCanonicalListKey() error = %v", err)
	}
	if kind != runtime.VKList {
		t.Fatalf("deriveCanonicalListKey() kind = %v, want %v", kind, runtime.VKList)
	}
	want := runtime.AppendUvarint(nil, 2)
	want = runtime.AppendListEntry(want, byte(runtime.VKString), []byte("k:a"))
	want = runtime.AppendListEntry(want, byte(runtime.VKString), []byte("k:b"))
	if got := string(key); got != string(want) {
		t.Fatalf("deriveCanonicalListKey() = %v, want %v", key, want)
	}
}

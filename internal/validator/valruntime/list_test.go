package valruntime

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestCanonicalizeBuildsCanonicalValueAndKey(t *testing.T) {
	out, bufs, err := CanonicalizeList(
		ListInput{
			Meta: runtime.ValidatorMeta{
				Kind:       runtime.VList,
				Index:      0,
				WhiteSpace: runtime.WSCollapse,
			},
			Validators: runtime.ValidatorsBundle{
				List: []runtime.ListValidator{{Item: 7}},
			},
			Normalized:      []byte("a b"),
			ApplyWhitespace: true,
			NeedKey:         true,
			Buffers: ListBuffers{
				Value: make([]byte, 0, 8),
				Key:   make([]byte, 0, 16),
			},
		},
		func(itemValidator runtime.ValidatorID, item []byte, needKey bool) ([]byte, ListItemResult, error) {
			if itemValidator != 7 {
				t.Fatalf("item validator = %d, want 7", itemValidator)
			}
			canon := append([]byte(nil), item...)
			for i, b := range canon {
				if b >= 'a' && b <= 'z' {
					canon[i] = b - ('a' - 'A')
				}
			}
			return canon, ListItemResult{
				KeyKind:  runtime.VKString,
				KeyBytes: append([]byte("k:"), item...),
				KeySet:   needKey,
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("CanonicalizeList() error = %v", err)
	}
	if got := string(out.Canonical); got != "A B" {
		t.Fatalf("CanonicalizeList() canonical = %q, want %q", got, "A B")
	}
	if out.Count != 2 {
		t.Fatalf("CanonicalizeList() count = %d, want 2", out.Count)
	}
	if !out.KeySet || len(out.Key) == 0 {
		t.Fatal("CanonicalizeList() should derive a list key")
	}
	if len(bufs.Value) == 0 || len(bufs.Key) == 0 {
		t.Fatal("CanonicalizeList() should reuse caller-owned buffers")
	}
}

func TestCanonicalizeRequiresItemKeyWhenRequested(t *testing.T) {
	_, _, err := CanonicalizeList(
		ListInput{
			Meta: runtime.ValidatorMeta{
				Kind:       runtime.VList,
				Index:      0,
				WhiteSpace: runtime.WSCollapse,
			},
			Validators:      runtime.ValidatorsBundle{List: []runtime.ListValidator{{Item: 1}}},
			Normalized:      []byte("a"),
			ApplyWhitespace: true,
			NeedKey:         true,
		},
		func(runtime.ValidatorID, []byte, bool) ([]byte, ListItemResult, error) {
			return []byte("a"), ListItemResult{}, nil
		},
	)
	if err == nil || err.Error() != "list item key missing" {
		t.Fatalf("CanonicalizeList() error = %v, want list item key missing", err)
	}
}

func TestValidateNoCanonicalUsesCollapsedFloatFastPath(t *testing.T) {
	called := false
	err := ValidateListNoCanonical(
		ListNoCanonicalInput{
			Meta: runtime.ValidatorMeta{
				Kind:       runtime.VList,
				Index:      0,
				WhiteSpace: runtime.WSCollapse,
			},
			Validators: runtime.ValidatorsBundle{
				List: []runtime.ListValidator{{Item: 2}},
				Meta: []runtime.ValidatorMeta{
					{},
					{},
					{Kind: runtime.VDouble},
				},
			},
			Normalized:      []byte("1 2.5 -3"),
			ApplyWhitespace: true,
		},
		func(runtime.ValidatorID, []byte) error {
			called = true
			return errors.New("unexpected callback")
		},
	)
	if err != nil {
		t.Fatalf("ValidateListNoCanonical() error = %v", err)
	}
	if called {
		t.Fatal("ValidateListNoCanonical() should use the collapsed-float fast path")
	}
}

func TestValidateNoCanonicalVisitsItemsWithoutFastPath(t *testing.T) {
	count := 0
	err := ValidateListNoCanonical(
		ListNoCanonicalInput{
			Meta: runtime.ValidatorMeta{
				Kind:       runtime.VList,
				Index:      0,
				WhiteSpace: runtime.WSPreserve,
			},
			Validators: runtime.ValidatorsBundle{
				List: []runtime.ListValidator{{Item: 3}},
				Meta: []runtime.ValidatorMeta{
					{},
					{},
					{},
					{Kind: runtime.VString},
				},
			},
			Normalized:      []byte("a\tb"),
			ApplyWhitespace: false,
		},
		func(itemValidator runtime.ValidatorID, item []byte) error {
			if itemValidator != 3 {
				t.Fatalf("item validator = %d, want 3", itemValidator)
			}
			count++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ValidateListNoCanonical() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("ValidateListNoCanonical() item count = %d, want 2", count)
	}
}

func TestTrackCanonicalVisitsCanonicalItems(t *testing.T) {
	count := 0
	err := TrackCanonicalList(
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
		t.Fatalf("TrackCanonicalList() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("TrackCanonicalList() item count = %d, want 2", count)
	}
}

func TestDeriveCanonicalKeyBuildsListKey(t *testing.T) {
	kind, key, err := DeriveCanonicalListKey(
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
		t.Fatalf("DeriveCanonicalListKey() error = %v", err)
	}
	if kind != runtime.VKList {
		t.Fatalf("DeriveCanonicalListKey() kind = %v, want %v", kind, runtime.VKList)
	}
	want := runtime.AppendUvarint(nil, 2)
	want = runtime.AppendListEntry(want, byte(runtime.VKString), []byte("k:a"))
	want = runtime.AppendListEntry(want, byte(runtime.VKString), []byte("k:b"))
	if got := string(key); got != string(want) {
		t.Fatalf("DeriveCanonicalListKey() = %v, want %v", key, want)
	}
}

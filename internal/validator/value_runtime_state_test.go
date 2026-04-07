package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestResultAndCacheFields(t *testing.T) {
	var cache ValueCache

	cache.SetListLength(2)
	length, err := cache.Length(runtime.VString, []byte("ignored"))
	if err != nil {
		t.Fatalf("cache.Length() error = %v", err)
	}
	if length != 2 {
		t.Fatalf("cache.Length() = %d, want 2", length)
	}

	var result ValueState
	result.SetKey(runtime.VKString, []byte("k"))
	kind, key, ok := result.Key()
	if !ok {
		t.Fatal("result.Key() reported no key")
	}
	if kind != runtime.VKString {
		t.Fatalf("result.Key() kind = %v, want %v", kind, runtime.VKString)
	}
	if string(key) != "k" {
		t.Fatalf("result.Key() key = %q, want %q", key, "k")
	}
}

func TestNilResultMethods(t *testing.T) {
	var result *ValueState
	if result.HasKey() {
		t.Fatal("result.HasKey() = true, want false")
	}
	if _, _, ok := result.Key(); ok {
		t.Fatal("result.Key() ok = true, want false")
	}
	result.SetKey(runtime.VKString, []byte("k"))
}

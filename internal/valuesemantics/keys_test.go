package valuesemantics

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestKeyForValidatorKind(t *testing.T) {
	kind, key, err := KeyForValidatorKind(runtime.VBoolean, []byte("true"))
	if err != nil {
		t.Fatalf("KeyForValidatorKind() error = %v", err)
	}
	if kind != runtime.VKBool {
		t.Fatalf("kind = %v, want %v", kind, runtime.VKBool)
	}
	if len(key) != 1 || key[0] != 1 {
		t.Fatalf("key = %v, want [1]", key)
	}
}

func TestKeyForPrimitiveName(t *testing.T) {
	kind, key, err := KeyForPrimitiveName("string", "abc", nil)
	if err != nil {
		t.Fatalf("KeyForPrimitiveName() error = %v", err)
	}
	if kind != runtime.VKString {
		t.Fatalf("kind = %v, want %v", kind, runtime.VKString)
	}
	if len(key) == 0 {
		t.Fatalf("string key is empty")
	}
}

package runtime

import (
	"testing"
)

func TestKeyForValidatorKind(t *testing.T) {
	kind, key, err := KeyForValidatorKind(VBoolean, []byte("true"))
	if err != nil {
		t.Fatalf("KeyForValidatorKind() error = %v", err)
	}
	if kind != VKBool {
		t.Fatalf("kind = %v, want %v", kind, VKBool)
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
	if kind != VKString {
		t.Fatalf("kind = %v, want %v", kind, VKString)
	}
	if len(key) == 0 {
		t.Fatalf("string key is empty")
	}
}

func TestKeyForValidatorKindUnsupportedTemporalKind(t *testing.T) {
	_, _, err := KeyForValidatorKind(ValidatorKind(255), []byte("2001-01-01"))
	if err == nil {
		t.Fatal("KeyForValidatorKind() error = nil, want unsupported validator kind error")
	}
}

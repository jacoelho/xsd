package builtins

import (
	"testing"
)

func TestGetReturnsCanonicalBuiltinPointer(t *testing.T) {
	t.Parallel()

	got := Get(TypeNameString)
	if got == nil {
		t.Fatal("Get(string) returned nil")
	}
	if got != Get(TypeNameString) {
		t.Fatal("Get(string) did not return canonical builtin pointer")
	}
}

func TestGetNSMatchesGetPointer(t *testing.T) {
	t.Parallel()

	got := GetNS(XSDNamespace, string(TypeNameString))
	if got == nil {
		t.Fatal("GetNS(xsd,string) returned nil")
	}
	if got != Get(TypeNameString) {
		t.Fatal("GetNS(xsd,string) did not match Get(string) pointer")
	}
}

func TestNewSimpleTypeBuildsBuiltinWrapper(t *testing.T) {
	t.Parallel()

	got, err := NewSimpleType(TypeNameString)
	if err != nil {
		t.Fatalf("NewSimpleType(string) error = %v", err)
	}
	if got == nil {
		t.Fatal("NewSimpleType(string) returned nil")
	}
	if !got.IsBuiltin() {
		t.Fatal("NewSimpleType(string) returned non-builtin simple type")
	}
}

func TestNewSimpleTypeUnknownReturnsError(t *testing.T) {
	t.Parallel()

	if _, err := NewSimpleType(TypeName("unknown")); err == nil {
		t.Fatal("expected error for unknown builtin simple type")
	}
}

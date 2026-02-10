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

func TestBuiltinListTypeMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		item       TypeName
		shouldFind bool
	}{
		{name: "NMTOKENS", item: TypeNameNMTOKEN, shouldFind: true},
		{name: "IDREFS", item: TypeNameIDREF, shouldFind: true},
		{name: "ENTITIES", item: TypeNameENTITY, shouldFind: true},
		{name: "string", shouldFind: false},
	}

	for _, tc := range cases {
		item, ok := BuiltinListItemTypeName(tc.name)
		if ok != tc.shouldFind {
			t.Fatalf("BuiltinListItemTypeName(%q) found=%v, want %v", tc.name, ok, tc.shouldFind)
		}
		if item != tc.item {
			t.Fatalf("BuiltinListItemTypeName(%q) = %q, want %q", tc.name, item, tc.item)
		}
		if IsBuiltinListTypeName(tc.name) != tc.shouldFind {
			t.Fatalf("IsBuiltinListTypeName(%q) mismatch", tc.name)
		}
	}
}

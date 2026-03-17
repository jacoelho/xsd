package model

import "testing"

func TestGetBuiltinReturnsCanonicalPointer(t *testing.T) {
	t.Parallel()

	got := GetBuiltin(TypeNameString)
	if got == nil {
		t.Fatal("GetBuiltin(string) returned nil")
	}
	if got != GetBuiltin(TypeNameString) {
		t.Fatal("GetBuiltin(string) did not return canonical pointer")
	}
}

func TestGetBuiltinNSMatchesCanonicalPointer(t *testing.T) {
	t.Parallel()

	got := GetBuiltinNS(XSDNamespace, string(TypeNameString))
	if got == nil {
		t.Fatal("GetBuiltinNS(xsd,string) returned nil")
	}
	if got != GetBuiltin(TypeNameString) {
		t.Fatal("GetBuiltinNS(xsd,string) did not match GetBuiltin(string)")
	}
}

func TestNewBuiltinSimpleTypeBuildsBuiltinWrapper(t *testing.T) {
	t.Parallel()

	got, err := NewBuiltinSimpleType(TypeNameString)
	if err != nil {
		t.Fatalf("NewBuiltinSimpleType(string) error = %v", err)
	}
	if got == nil {
		t.Fatal("NewBuiltinSimpleType(string) returned nil")
	}
	if !got.IsBuiltin() {
		t.Fatal("NewBuiltinSimpleType(string) returned non-builtin simple type")
	}
}

func TestNewBuiltinSimpleTypeUnknownReturnsError(t *testing.T) {
	t.Parallel()

	if _, err := NewBuiltinSimpleType(TypeName("unknown")); err == nil {
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

func TestBuiltinTypesReturnsDeterministicOrder(t *testing.T) {
	t.Parallel()

	items := BuiltinTypes()
	if len(items) == 0 {
		t.Fatal("BuiltinTypes() returned empty builtins")
	}
	for i := 1; i < len(items); i++ {
		if items[i-1] == nil || items[i] == nil {
			t.Fatal("BuiltinTypes() returned nil entry")
		}
		if items[i-1].Name().Local > items[i].Name().Local {
			t.Fatalf("BuiltinTypes() is not sorted: %q > %q", items[i-1].Name().Local, items[i].Name().Local)
		}
	}
}

func TestMustBuiltinAndIsBuiltin(t *testing.T) {
	t.Parallel()

	got := MustBuiltin(TypeNameString)
	if got == nil {
		t.Fatal("MustBuiltin(string) returned nil")
	}
	if !IsBuiltin(QName{Namespace: XSDNamespace, Local: string(TypeNameString)}) {
		t.Fatal("IsBuiltin(string QName) = false, want true")
	}
	if IsBuiltin(QName{Namespace: "urn:test", Local: "string"}) {
		t.Fatal("IsBuiltin(non-xsd QName) = true, want false")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("MustBuiltin(unknown) did not panic")
		}
	}()
	_ = MustBuiltin(TypeName("unknown"))
}

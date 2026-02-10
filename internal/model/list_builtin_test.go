package model

import "testing"

func TestBuiltinListItemTypeName(t *testing.T) {
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
		gotItem, ok := builtinListItemTypeName(tc.name)
		if ok != tc.shouldFind {
			t.Fatalf("builtinListItemTypeName(%q) found=%v, want %v", tc.name, ok, tc.shouldFind)
		}
		if gotItem != tc.item {
			t.Fatalf("builtinListItemTypeName(%q) = %q, want %q", tc.name, gotItem, tc.item)
		}
		if isBuiltinListTypeName(tc.name) != tc.shouldFind {
			t.Fatalf("isBuiltinListTypeName(%q) mismatch", tc.name)
		}
	}
}

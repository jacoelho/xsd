package runtimeassemble

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestAnyAttributeUnionNamespaceList(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "xsdtests", "sunData", "combined", "008", "test.xsd")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	parsed := mustResolveSchema(t, string(data))
	rt, err := buildSchemaForTest(parsed, BuildConfig{})
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}

	nsID := rt.Namespaces.Lookup([]byte("urn:foo"))
	if nsID == 0 {
		t.Fatalf("namespace urn:foo not interned")
	}
	sym := rt.Symbols.Lookup(nsID, []byte("extension"))
	if sym == 0 {
		t.Fatalf("symbol for extension not found")
	}
	elemID := rt.GlobalElements[sym]
	if elemID == 0 {
		t.Fatalf("global element extension not found")
	}
	elem := rt.Elements[elemID]
	typ := rt.Types[elem.Type]
	if typ.Kind != runtime.TypeComplex {
		t.Fatalf("extension type kind = %d, want complex", typ.Kind)
	}
	ct := rt.ComplexTypes[typ.Complex.ID]
	if ct.AnyAttr == 0 {
		t.Fatalf("extension anyAttribute missing")
	}
	cases := []struct {
		name string
		ns   string
	}{
		{"urn:a", "urn:a"},
		{"urn:b", "urn:b"},
		{"urn:c", "urn:c"},
	}
	for _, tc := range cases {
		nsBytes := []byte(tc.ns)
		if !rt.WildcardAccepts(ct.AnyAttr, nsBytes, 0) {
			t.Fatalf("anyAttribute does not accept %s", tc.name)
		}
	}
}

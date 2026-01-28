package runtime

import (
	"reflect"
	"testing"

	xsdxml "github.com/jacoelho/xsd/internal/xml"
)

func TestNamespaceInterner(t *testing.T) {
	b := NewBuilder()
	emptyID := b.InternNamespace(nil)
	if emptyID != 1 {
		t.Fatalf("empty namespace id = %d, want 1", emptyID)
	}
	aID := b.InternNamespace([]byte("urn:a"))
	bID := b.InternNamespace([]byte("urn:b"))
	if aID == bID {
		t.Fatalf("expected distinct IDs for namespaces")
	}
	if again := b.InternNamespace([]byte("urn:a")); again != aID {
		t.Fatalf("namespace interning not stable: got %d want %d", again, aID)
	}

	schema := b.Build()
	if schema.PredefNS.Empty != emptyID {
		t.Fatalf("PredefNS.Empty = %d, want %d", schema.PredefNS.Empty, emptyID)
	}
	if schema.PredefNS.Xml == 0 || schema.PredefNS.Xsi == 0 {
		t.Fatalf("expected predefined XML/XSI namespaces")
	}
	if got := schema.Namespaces.Lookup([]byte("urn:a")); got != aID {
		t.Fatalf("Lookup(urn:a) = %d, want %d", got, aID)
	}
	if got := schema.Namespaces.Lookup([]byte("missing")); got != 0 {
		t.Fatalf("Lookup(missing) = %d, want 0", got)
	}
}

func TestPredefinedSymbols(t *testing.T) {
	b := NewBuilder()
	schema := b.Build()

	if schema.PredefNS.Xml == 0 || schema.PredefNS.Xsi == 0 {
		t.Fatalf("expected predefined XML/XSI namespaces")
	}

	if got := schema.Symbols.Lookup(schema.PredefNS.Xsi, []byte("type")); got != schema.Predef.XsiType {
		t.Fatalf("predef xsi:type = %d, lookup = %d", schema.Predef.XsiType, got)
	}
	if got := schema.Symbols.Lookup(schema.PredefNS.Xsi, []byte("nil")); got != schema.Predef.XsiNil {
		t.Fatalf("predef xsi:nil = %d, lookup = %d", schema.Predef.XsiNil, got)
	}
	if got := schema.Symbols.Lookup(schema.PredefNS.Xsi, []byte("schemaLocation")); got != schema.Predef.XsiSchemaLocation {
		t.Fatalf("predef xsi:schemaLocation = %d, lookup = %d", schema.Predef.XsiSchemaLocation, got)
	}
	if got := schema.Symbols.Lookup(schema.PredefNS.Xsi, []byte("noNamespaceSchemaLocation")); got != schema.Predef.XsiNoNamespaceSchemaLocation {
		t.Fatalf("predef xsi:noNamespaceSchemaLocation = %d, lookup = %d", schema.Predef.XsiNoNamespaceSchemaLocation, got)
	}
	if got := schema.Symbols.Lookup(schema.PredefNS.Xml, []byte("lang")); got != schema.Predef.XmlLang {
		t.Fatalf("predef xml:lang = %d, lookup = %d", schema.Predef.XmlLang, got)
	}
	if got := schema.Symbols.Lookup(schema.PredefNS.Xml, []byte("space")); got != schema.Predef.XmlSpace {
		t.Fatalf("predef xml:space = %d, lookup = %d", schema.Predef.XmlSpace, got)
	}

	if schema.Namespaces.Lookup([]byte(xsdxml.XMLNamespace)) != schema.PredefNS.Xml {
		t.Fatalf("xml namespace lookup mismatch")
	}
	if schema.Namespaces.Lookup([]byte(xsdxml.XSINamespace)) != schema.PredefNS.Xsi {
		t.Fatalf("xsi namespace lookup mismatch")
	}
}

func TestSymbolInterner(t *testing.T) {
	b := NewBuilder()
	aNS := b.InternNamespace([]byte("urn:a"))
	bNS := b.InternNamespace([]byte("urn:b"))

	rootA := b.InternSymbol(aNS, []byte("root"))
	if again := b.InternSymbol(aNS, []byte("root")); again != rootA {
		t.Fatalf("symbol interning not stable: got %d want %d", again, rootA)
	}
	rootB := b.InternSymbol(bNS, []byte("root"))
	if rootB == rootA {
		t.Fatalf("expected distinct IDs for different namespaces")
	}

	schema := b.Build()
	if got := schema.Symbols.Lookup(aNS, []byte("root")); got != rootA {
		t.Fatalf("Lookup(ns=a, root) = %d, want %d", got, rootA)
	}
	if got := schema.Symbols.Lookup(bNS, []byte("root")); got != rootB {
		t.Fatalf("Lookup(ns=b, root) = %d, want %d", got, rootB)
	}
	if got := schema.Symbols.Lookup(aNS, []byte("missing")); got != 0 {
		t.Fatalf("Lookup(ns=a, missing) = %d, want 0", got)
	}
}

func TestInternerDeterminism(t *testing.T) {
	b1 := NewBuilder()
	a1 := b1.InternNamespace([]byte("urn:a"))
	b1ns := b1.InternNamespace([]byte("urn:b"))
	sym1 := b1.InternSymbol(a1, []byte("root"))
	sym2 := b1.InternSymbol(b1ns, []byte("child"))
	s1 := b1.Build()

	b2 := NewBuilder()
	a2 := b2.InternNamespace([]byte("urn:a"))
	b2ns := b2.InternNamespace([]byte("urn:b"))
	sym1b := b2.InternSymbol(a2, []byte("root"))
	sym2b := b2.InternSymbol(b2ns, []byte("child"))
	s2 := b2.Build()

	if a1 != a2 || b1ns != b2ns || sym1 != sym1b || sym2 != sym2b {
		t.Fatalf("interned IDs differ across identical builds")
	}
	if !reflect.DeepEqual(s1.Namespaces, s2.Namespaces) {
		t.Fatalf("namespace tables differ across identical builds")
	}
	if !reflect.DeepEqual(s1.Symbols, s2.Symbols) {
		t.Fatalf("symbol tables differ across identical builds")
	}
}

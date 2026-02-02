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

	schema, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
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
	schema, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

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

	schema, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
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
	s1, err := b1.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	b2 := NewBuilder()
	a2 := b2.InternNamespace([]byte("urn:a"))
	b2ns := b2.InternNamespace([]byte("urn:b"))
	sym1b := b2.InternSymbol(a2, []byte("root"))
	sym2b := b2.InternSymbol(b2ns, []byte("child"))
	s2, err := b2.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

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

func TestBuildNamespaceIndexBounds(t *testing.T) {
	table := NamespaceTable{
		Off:  []uint32{0, 10},
		Len:  []uint32{0, 5},
		Blob: make([]byte, 10),
	}
	if _, err := buildNamespaceIndex(&table); err == nil {
		t.Fatalf("expected namespace index bounds error")
	}
}

func TestBuildSymbolsIndexBounds(t *testing.T) {
	table := SymbolsTable{
		NS:        []NamespaceID{0, 1},
		LocalOff:  []uint32{0, 8},
		LocalLen:  []uint32{0, 8},
		LocalBlob: make([]byte, 10),
	}
	if _, err := buildSymbolsIndex(&table); err == nil {
		t.Fatalf("expected symbols index bounds error")
	}
}

func TestBuilderBuildRejectsInvalidNamespaceTable(t *testing.T) {
	b := NewBuilder()
	b.namespaces.off = []uint32{0, 10}
	b.namespaces.len = []uint32{0, 5}
	b.namespaces.blob = make([]byte, 10)

	if _, err := b.Build(); err == nil {
		t.Fatalf("expected Build() error for invalid namespace table")
	}
}

func TestBuilderBuildRejectsInvalidSymbolTable(t *testing.T) {
	b := NewBuilder()
	b.symbols.ns = []NamespaceID{0, 1}
	b.symbols.localOff = []uint32{0, 8}
	b.symbols.localLen = []uint32{0, 8}
	b.symbols.localBlob = make([]byte, 10)

	if _, err := b.Build(); err == nil {
		t.Fatalf("expected Build() error for invalid symbol table")
	}
}

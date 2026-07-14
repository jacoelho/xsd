package runtime

import "testing"

func TestGlobalLookupHelpers(t *testing.T) {
	t.Parallel()

	_, qnames := runtimeGlobalsFixture(t)
	attributes := map[QName]AttributeID{qnames["attr"]: 0}
	types := map[QName]TypeID{qnames["simple"]: SimpleRef(0)}
	derivations := newTypeDerivationReadForTest(
		t,
		[]SimpleType{{Base: NoSimpleType}},
		[]ComplexType{{Derivation: DerivationKindNone}},
	)
	if typ, ok := GlobalTypeByName(types, derivations, qnames["simple"]); !ok || typ != SimpleRef(0) {
		t.Fatalf("GlobalTypeByName() = %v, %v; want simple 0, true", typ, ok)
	}
	if typ, ok := GlobalTypeByName(map[QName]TypeID{qnames["simple"]: ComplexRef(99)}, derivations, qnames["simple"]); ok || typ != (TypeID{}) {
		t.Fatalf("GlobalTypeByName(invalid) = %v, %v; want zero, false", typ, ok)
	}
	attributeDecls := []AttributeDeclRead{{}}
	if id, ok, valid := GlobalAttributeByName(attributes, attributeDecls, qnames["attr"]); !ok || !valid || id != 0 {
		t.Fatalf("GlobalAttributeByName() = %v, %v, %v; want 0, true, true", id, ok, valid)
	}
	if id, ok, valid := GlobalAttributeByName(attributes, attributeDecls, qnames["other"]); ok || !valid || id != 0 {
		t.Fatalf("GlobalAttributeByName(missing) = %v, %v, %v; want 0, false, true", id, ok, valid)
	}
	if id, ok, valid := GlobalAttributeByName(map[QName]AttributeID{qnames["attr"]: 99}, attributeDecls, qnames["attr"]); ok || valid || id != 0 {
		t.Fatalf("GlobalAttributeByName(invalid) = %v, %v, %v; want 0, false, false", id, ok, valid)
	}
	if got, ok := TypeNameByID([]SimpleType{{Name: qnames["simple"]}}, nil, SimpleRef(0)); !ok || got != qnames["simple"] {
		t.Fatalf("TypeNameByID(simple) = %v, %v; want simple name, true", got, ok)
	}
	if got, ok := TypeNameByID(nil, []ComplexType{{Name: qnames["complex"]}}, ComplexRef(0)); !ok || got != qnames["complex"] {
		t.Fatalf("TypeNameByID(complex) = %v, %v; want complex name, true", got, ok)
	}
}

func TestNotationReadMap(t *testing.T) {
	t.Parallel()

	names, qnames := runtimeGlobalsFixture(t)
	notations := map[QName]bool{qnames["notation"]: true, qnames["other"]: false}
	read := NewNotationReadMap(&names, notations)
	want := ExpandedName{Namespace: EmptyNamespaceURI, Local: "notation"}
	if len(read) != 1 || !read[want] {
		t.Fatalf("NewNotationReadMap() = %#v, want only %v", read, want)
	}
	if !EqualNotationReadMap(read, &names, notations) {
		t.Fatal("EqualNotationReadMap() = false, want true")
	}
	read[want] = false
	if EqualNotationReadMap(read, &names, notations) {
		t.Fatal("EqualNotationReadMap() accepted false projected notation")
	}
	read[want] = true
	read[ExpandedName{Namespace: EmptyNamespaceURI, Local: "other"}] = false
	if EqualNotationReadMap(read, &names, notations) {
		t.Fatal("EqualNotationReadMap() accepted extra projected notation")
	}
	if EqualNotationReadMap(nil, &names, notations) {
		t.Fatal("EqualNotationReadMap() accepted missing projected notation")
	}
	if EqualNotationReadMap(map[ExpandedName]bool{want: true}, nil, notations) {
		t.Fatal("EqualNotationReadMap() accepted nil name table")
	}
	if got := NewNotationReadMap(&names, map[QName]bool{qnames["other"]: false}); got != nil {
		t.Fatalf("NewNotationReadMap(false-only) = %#v, want nil", got)
	}
	if !EqualNotationReadMap(nil, &names, map[QName]bool{qnames["other"]: false}) {
		t.Fatal("EqualNotationReadMap(false-only) = false, want true")
	}
	if err := ValidateNotationReadMap(NewNotationReadMap(&names, notations), &names, notations); err != nil {
		t.Fatalf("ValidateNotationReadMap() error = %v", err)
	}
	if err := ValidateNotationReadMap(nil, &names, notations); err == nil || err.Error() != "notation read map does not match notations" {
		t.Fatalf("ValidateNotationReadMap(missing) error = %v, want notation invariant", err)
	}
}

func runtimeGlobalsFixture(t *testing.T) (NameTable, map[string]QName) {
	t.Helper()

	names, err := NewNameTable(16, []string{EmptyNamespaceURI}, []ExpandedName{
		{Namespace: EmptyNamespaceURI, Local: "attr"},
		{Namespace: EmptyNamespaceURI, Local: "elem"},
		{Namespace: EmptyNamespaceURI, Local: "simple"},
		{Namespace: EmptyNamespaceURI, Local: "complex"},
		{Namespace: EmptyNamespaceURI, Local: "identity"},
		{Namespace: EmptyNamespaceURI, Local: "notation"},
		{Namespace: EmptyNamespaceURI, Local: "other"},
	})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	qnames := make(map[string]QName)
	for _, local := range []string{"attr", "elem", "simple", "complex", "identity", "notation", "other"} {
		q, ok := names.LookupQName(EmptyNamespaceURI, local)
		if !ok {
			t.Fatalf("missing QName for %s", local)
		}
		qnames[local] = q
	}
	return names, qnames
}

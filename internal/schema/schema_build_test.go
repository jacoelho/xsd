package schema

import (
	"reflect"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
)

func TestSchemaBuildGlobalRegistrationIsAtomic(t *testing.T) {
	t.Parallel()

	c := compiler{rt: newSchemaBuild(NameTable{})}
	name := QName{Namespace: 1, Local: 2}
	decl := ElementDecl{Name: name}
	id, err := c.registerGlobalElement(name, decl)
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 || c.rt.GlobalElements[name] != id || len(c.rt.Elements) != 1 || c.rt.Elements[id].Scope != DeclarationScopeGlobal {
		t.Fatalf("registration = id %d globals %v elements %d", id, c.rt.GlobalElements, len(c.rt.Elements))
	}
}

func TestSchemaBuildPlaceholderCompletionKeepsStableID(t *testing.T) {
	t.Parallel()

	c := compiler{rt: newSchemaBuild(NameTable{})}
	firstName := QName{Local: 1}
	secondName := QName{Local: 2}
	first, err := c.addElement(ElementDecl{Name: firstName})
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.addElement(ElementDecl{Name: secondName})
	if err != nil {
		t.Fatal(err)
	}
	if first != 0 || second != 1 {
		t.Fatalf("placeholder IDs = %d, %d", first, second)
	}
	completed := ElementDecl{Name: firstName, Nillable: true}
	c.completeElement(first, completed)
	completed.Scope = DeclarationScopeNonGlobal
	if !reflect.DeepEqual(c.rt.Elements[first], completed) || c.rt.Elements[second].Name != secondName {
		t.Fatal("completion changed IDs or the wrong declaration")
	}
}

func TestElementCompilationFailureKeepsReservedPlaceholder(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"><xs:element name="bad" nillable="invalid"/></xs:schema>`)
	err = c.loadOwned([]source.Source{source.Bytes("bad.xsd", schema)})
	if err != nil {
		t.Fatal(err)
	}
	err = c.index()
	if err != nil {
		t.Fatal(err)
	}
	q, err := c.rt.internQName("urn:test", "bad")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.compileElementByQName(q); err == nil {
		t.Fatal("element compilation succeeded")
	}
	id, ok := c.rt.GlobalElements[q]
	if !ok {
		t.Fatal("failed element did not retain its reserved global ID")
	}
	want := ElementDecl{Name: q, Type: ComplexRef(c.rt.AnyType), Scope: DeclarationScopeGlobal}
	if got := c.rt.Elements[id]; !reflect.DeepEqual(got, want) {
		t.Fatalf("failed element = %#v, want reserved placeholder %#v", got, want)
	}
}

func TestSchemaBuildInstallsCorrelatedSubstitutionTables(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	headName, err := c.rt.internQName("urn:test", "head")
	if err != nil {
		t.Fatal(err)
	}
	memberName, err := c.rt.internQName("urn:test", "member")
	if err != nil {
		t.Fatal(err)
	}
	typ := ComplexRef(c.rt.AnyType)
	head, err := c.registerGlobalElement(headName, ElementDecl{Name: headName, Type: typ, SubstHead: NoElement})
	if err != nil {
		t.Fatal(err)
	}
	member, err := c.registerGlobalElement(memberName, ElementDecl{Name: memberName, Type: typ, SubstHead: head})
	if err != nil {
		t.Fatal(err)
	}
	table, err := BuildSubstitutionTable(&c.rt, &c.rt.Names, c.rt.Elements, c.rt.GlobalElements, limits.MaxSubstitutionClosureEntries, unboundedTypeDerivationWork)
	if err != nil {
		t.Fatal(err)
	}
	c.installFinalizedElements(slices.Clone(c.rt.Elements), table)
	if got, ok := c.rt.Substitutions.MemberByName(head, memberName); !ok || got != member {
		t.Fatalf("substitution lookup = %d/%v, want %d/true", got, ok, member)
	}
	var entries int
	c.rt.Substitutions.ForEachEntry(head, func(name QName, got ElementID) bool {
		entries++
		if name != memberName || got != member {
			t.Fatalf("substitution entry = %v/%d, want %v/%d", name, got, memberName, member)
		}
		return true
	})
	if entries != 1 {
		t.Fatalf("substitution entries = %d, want 1", entries)
	}
}

func TestSchemaBuildCompiledModelInstallationPreservesAlignment(t *testing.T) {
	t.Parallel()

	c := compiler{rt: newSchemaBuild(NameTable{})}
	if _, err := c.addModel(ContentModel{}); err != nil {
		t.Fatal(err)
	}
	compiled := []CompiledModel{{}}
	if err := c.installCompiledModels(compiled); err != nil {
		t.Fatal(err)
	}
	if len(c.rt.CompiledModels) != 1 {
		t.Fatalf("compiled models = %d", len(c.rt.CompiledModels))
	}
	if err := c.installCompiledModels(nil); err == nil {
		t.Fatal("misaligned compiled model installation succeeded")
	}
	if len(c.rt.CompiledModels) != 1 {
		t.Fatal("failed compiled model installation changed prior topology")
	}
}

func TestSchemaBuildBuiltinHandlesMatchRegisteredDeclarations(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidComplexTypeID(c.rt.AnyType, len(c.rt.ComplexTypes)) {
		t.Fatalf("xs:anyType ID = %d", c.rt.AnyType)
	}
	anyType := c.rt.ComplexTypes[c.rt.AnyType]
	if got, ok := c.rt.GlobalTypes[anyType.Name]; !ok || got != ComplexRef(c.rt.AnyType) {
		t.Fatalf("xs:anyType global = %v, %v", got, ok)
	}
	simpleNames := []string{
		"anySimpleType", "string", "boolean", "decimal", "integer", "int",
		"date", "dateTime", "time", "anyURI", "QName", "ID", "IDREF", "IDREFS",
		"NMTOKEN", "NMTOKENS", "ENTITY", "ENTITIES",
	}
	for _, local := range simpleNames {
		id, ok := value.BuiltinTypeID(local)
		if !ok {
			t.Fatalf("xs:%s has no value-program ID", local)
		}
		if !ValidSimpleTypeID(id, len(c.rt.SimpleTypes)) {
			t.Fatalf("xs:%s ID = %d", local, id)
		}
		declaration := c.rt.SimpleTypes[id]
		if got := c.rt.Names.Format(declaration.Name); got != "{"+vocab.XSDNamespaceURI+"}"+local {
			t.Fatalf("builtin ID %d name = %s, want xs:%s", id, got, local)
		}
		if got, ok := c.rt.GlobalTypes[declaration.Name]; !ok || got != SimpleRef(id) {
			t.Fatalf("xs:%s global = %v, %v", local, got, ok)
		}
	}
}

func TestSchemaBuildConstructionReadsDoNotAliasMutableTopology(t *testing.T) {
	t.Parallel()

	resolved := []ResolvedValueName{{Lexical: "p:value"}}
	constraint := &ValueConstraint{ResolvedNames: resolved, Lexical: "value"}
	fixedConstraint := &ValueConstraint{
		ResolvedNames: []ResolvedValueName{{Lexical: "p:fixed"}},
		Lexical:       "fixed",
	}
	build := schemaBuild{
		Attributes: []AttributeDecl{{Default: constraint, Fixed: fixedConstraint}},
		AttributeUseSets: []AttributeUseSet{{Uses: []AttributeUse{{
			Default: constraint,
			Fixed:   fixedConstraint,
		}}}},
	}
	rt := build

	attribute := rt.attributeUse(0)
	attribute.Default.ResolvedNames[0].Lexical = "changed"
	attribute.Fixed.ResolvedNames[0].Lexical = "changed"

	uses, _ := rt.attributeUsesAndWildcardForCompilation(0)
	uses[0] = AttributeUse{Name: QName{Local: 9}}
	if rt.Attributes[0].Default.ResolvedNames[0].Lexical != "p:value" ||
		rt.Attributes[0].Fixed.ResolvedNames[0].Lexical != "p:fixed" {
		t.Fatal("attribute-use projection aliases compiler-owned storage")
	}
	if rt.AttributeUseSets[0].Uses[0].Name != (QName{}) ||
		rt.AttributeUseSets[0].Uses[0].Default.ResolvedNames[0].Lexical != "p:value" ||
		rt.AttributeUseSets[0].Uses[0].Fixed.ResolvedNames[0].Lexical != "p:fixed" {
		t.Fatal("attribute-use-set projection aliases compiler-owned storage")
	}
}

func TestSchemaBuildOwnsNameInterning(t *testing.T) {
	t.Parallel()

	names, err := NewRuntimeNameTable(16)
	if err != nil {
		t.Fatal(err)
	}
	rt := newSchemaBuild(names)
	q, err := rt.internQName("urn:test", "value")
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := rt.lookupQName("urn:test", "value"); !ok || got != q {
		t.Fatalf("lookupQName() = %v/%v, want %v/true", got, ok, q)
	}
}

package compile

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
)

func TestSchemaBuildGlobalRegistrationIsAtomic(t *testing.T) {
	t.Parallel()

	c := compiler{rt: newCompilerSchemaBuild(runtime.NameTable{})}
	name := runtime.QName{Namespace: 1, Local: 2}
	decl := runtime.ElementDecl{Name: name}
	id, err := c.registerGlobalElement(name, decl)
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 || c.rt.build.GlobalElements[name] != id || len(c.rt.build.Elements) != 1 || c.rt.build.Elements[id].Scope != runtime.DeclarationScopeGlobal {
		t.Fatalf("registration = id %d globals %v elements %d", id, c.rt.build.GlobalElements, len(c.rt.build.Elements))
	}
}

func TestSchemaBuildPlaceholderCompletionKeepsStableID(t *testing.T) {
	t.Parallel()

	c := compiler{rt: newCompilerSchemaBuild(runtime.NameTable{})}
	firstName := runtime.QName{Local: 1}
	secondName := runtime.QName{Local: 2}
	first, err := c.addElement(runtime.ElementDecl{Name: firstName})
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.addElement(runtime.ElementDecl{Name: secondName})
	if err != nil {
		t.Fatal(err)
	}
	if first != 0 || second != 1 {
		t.Fatalf("placeholder IDs = %d, %d", first, second)
	}
	completed := runtime.ElementDecl{Name: firstName, Nillable: true}
	c.completeElement(first, completed)
	completed.Scope = runtime.DeclarationScopeNonGlobal
	if !reflect.DeepEqual(c.rt.build.Elements[first], completed) || c.rt.build.Elements[second].Name != secondName {
		t.Fatal("completion changed IDs or the wrong declaration")
	}
}

func TestElementCompilationFailureKeepsReservedPlaceholder(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(context.Background(), limits)
	if err != nil {
		t.Fatal(err)
	}
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"><xs:element name="bad" nillable="invalid"/></xs:schema>`)
	err = c.load([]source.Source{source.Bytes("bad.xsd", schema)})
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
	id, ok := c.rt.build.GlobalElements[q]
	if !ok {
		t.Fatal("failed element did not retain its reserved global ID")
	}
	want := runtime.ElementDecl{Name: q, Type: runtime.ComplexRef(c.rt.build.Builtin.AnyType), Scope: runtime.DeclarationScopeGlobal}
	if got := c.rt.build.Elements[id]; !reflect.DeepEqual(got, want) {
		t.Fatalf("failed element = %#v, want reserved placeholder %#v", got, want)
	}
}

func TestSchemaBuildInstallsCorrelatedSubstitutionTables(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(context.Background(), limits)
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
	typ := runtime.ComplexRef(c.rt.build.Builtin.AnyType)
	head, err := c.registerGlobalElement(headName, runtime.ElementDecl{Name: headName, Type: typ, SubstHead: runtime.NoElement})
	if err != nil {
		t.Fatal(err)
	}
	member, err := c.registerGlobalElement(memberName, runtime.ElementDecl{Name: memberName, Type: typ, SubstHead: head})
	if err != nil {
		t.Fatal(err)
	}
	table, err := runtime.BuildSubstitutionTable(&c.rt.build, &c.rt.build.Names, c.rt.build.Elements, c.rt.build.GlobalElements, limits.MaxSubstitutionClosureEntries)
	if err != nil {
		t.Fatal(err)
	}
	c.installFinalizedElements(slices.Clone(c.rt.build.Elements), table)
	if got, ok := c.rt.SubstitutionMemberByName(head, memberName); !ok || got != member {
		t.Fatalf("substitution lookup = %d/%v, want %d/true", got, ok, member)
	}
	var entries int
	c.rt.ForEachSubstitutionEntry(head, func(name runtime.QName, got runtime.ElementID) bool {
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

	c := compiler{rt: newCompilerSchemaBuild(runtime.NameTable{})}
	if _, err := c.addModel(runtime.ContentModel{}); err != nil {
		t.Fatal(err)
	}
	compiled := []runtime.CompiledModel{{}}
	if err := c.installCompiledModels(compiled); err != nil {
		t.Fatal(err)
	}
	if len(c.rt.build.CompiledModels) != 1 {
		t.Fatalf("compiled models = %d", len(c.rt.build.CompiledModels))
	}
	if err := c.installCompiledModels(nil); err == nil {
		t.Fatal("misaligned compiled model installation succeeded")
	}
	if len(c.rt.build.CompiledModels) != 1 {
		t.Fatal("failed compiled model installation changed prior topology")
	}
}

func TestSchemaBuildBuiltinHandlesMatchRegisteredDeclarations(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(context.Background(), limits)
	if err != nil {
		t.Fatal(err)
	}
	if !runtime.ValidComplexTypeID(c.rt.build.Builtin.AnyType, len(c.rt.build.ComplexTypes)) {
		t.Fatalf("xs:anyType ID = %d", c.rt.build.Builtin.AnyType)
	}
	anyType := c.rt.build.ComplexTypes[c.rt.build.Builtin.AnyType]
	if got, ok := c.rt.build.GlobalTypes[anyType.Name]; !ok || got != runtime.ComplexRef(c.rt.build.Builtin.AnyType) {
		t.Fatalf("xs:anyType global = %v, %v", got, ok)
	}
	simpleHandles := []struct {
		local string
		id    runtime.SimpleTypeID
	}{
		{local: "anySimpleType", id: c.rt.build.Builtin.AnySimpleType},
		{local: "string", id: c.rt.build.Builtin.String},
		{local: "boolean", id: c.rt.build.Builtin.Boolean},
		{local: "decimal", id: c.rt.build.Builtin.Decimal},
		{local: "integer", id: c.rt.build.Builtin.Integer},
		{local: "int", id: c.rt.build.Builtin.Int},
		{local: "date", id: c.rt.build.Builtin.Date},
		{local: "dateTime", id: c.rt.build.Builtin.DateTime},
		{local: "time", id: c.rt.build.Builtin.Time},
		{local: "anyURI", id: c.rt.build.Builtin.AnyURI},
		{local: "QName", id: c.rt.build.Builtin.QName},
		{local: "ID", id: c.rt.build.Builtin.ID},
		{local: "IDREF", id: c.rt.build.Builtin.IDREF},
		{local: "IDREFS", id: c.rt.build.Builtin.IDREFS},
		{local: "NMTOKEN", id: c.rt.build.Builtin.NMTOKEN},
		{local: "NMTOKENS", id: c.rt.build.Builtin.NMTOKENS},
		{local: "ENTITY", id: c.rt.build.Builtin.ENTITY},
		{local: "ENTITIES", id: c.rt.build.Builtin.ENTITIES},
	}
	for _, handle := range simpleHandles {
		if !runtime.ValidSimpleTypeID(handle.id, len(c.rt.build.SimpleTypes)) {
			t.Fatalf("xs:%s ID = %d", handle.local, handle.id)
		}
		declaration := c.rt.build.SimpleTypes[handle.id]
		if got := c.rt.build.Names.Format(declaration.Name); got != "{"+runtime.XSDNamespaceURI+"}"+handle.local {
			t.Fatalf("builtin ID %d name = %s, want xs:%s", handle.id, got, handle.local)
		}
		if got, ok := c.rt.build.GlobalTypes[declaration.Name]; !ok || got != runtime.SimpleRef(handle.id) {
			t.Fatalf("xs:%s global = %v, %v", handle.local, got, ok)
		}
	}
}

func TestSchemaBuildConstructionReadsDoNotAliasMutableTopology(t *testing.T) {
	t.Parallel()

	resolved := []runtime.ResolvedValueName{{Lexical: "p:value"}}
	constraint := &runtime.ValueConstraint{ResolvedNames: resolved, Lexical: "value"}
	fixedConstraint := &runtime.ValueConstraint{
		ResolvedNames: []runtime.ResolvedValueName{{Lexical: "p:fixed"}},
		Lexical:       "fixed",
	}
	build := runtime.SchemaBuild{
		SimpleTypes: []runtime.SimpleType{{Union: []runtime.SimpleTypeID{1}}},
		Elements: []runtime.ElementDecl{{
			Identity: []runtime.IdentityConstraintID{1},
			Default:  constraint,
			Fixed:    fixedConstraint,
		}},
		Attributes: []runtime.AttributeDecl{{Default: constraint, Fixed: fixedConstraint}},
		AttributeUseSets: []runtime.AttributeUseSet{{Uses: []runtime.AttributeUse{{
			Default: constraint,
			Fixed:   fixedConstraint,
		}}}},
	}
	rt := compilerSchemaBuild{build: build}

	typ, ok := rt.simpleValueType(0)
	if !ok {
		t.Fatal("simpleValueType() rejected valid type")
	}
	typ.UnionMembers[0] = 9

	element := rt.elementCopy(0)
	element.Identity[0] = 9
	element.Default.ResolvedNames[0].Lexical = "changed"
	element.Fixed.ResolvedNames[0].Lexical = "changed"

	attribute := rt.attributeUse(0)
	attribute.Default.ResolvedNames[0].Lexical = "changed"
	attribute.Fixed.ResolvedNames[0].Lexical = "changed"

	uses, _ := rt.attributeUsesAndWildcard(0)
	uses[0].Default.ResolvedNames[0].Lexical = "changed"
	uses[0].Fixed.ResolvedNames[0].Lexical = "changed"

	if rt.build.SimpleTypes[0].Union[0] != 1 {
		t.Fatal("simple-value projection aliases compiler-owned union members")
	}
	if rt.build.Elements[0].Identity[0] != 1 ||
		rt.build.Elements[0].Default.ResolvedNames[0].Lexical != "p:value" ||
		rt.build.Elements[0].Fixed.ResolvedNames[0].Lexical != "p:fixed" {
		t.Fatal("element construction copy aliases compiler-owned storage")
	}
	if rt.build.Attributes[0].Default.ResolvedNames[0].Lexical != "p:value" ||
		rt.build.Attributes[0].Fixed.ResolvedNames[0].Lexical != "p:fixed" {
		t.Fatal("attribute-use projection aliases compiler-owned storage")
	}
	if rt.build.AttributeUseSets[0].Uses[0].Default.ResolvedNames[0].Lexical != "p:value" ||
		rt.build.AttributeUseSets[0].Uses[0].Fixed.ResolvedNames[0].Lexical != "p:fixed" {
		t.Fatal("attribute-use-set projection aliases compiler-owned storage")
	}
}

func TestSchemaBuildOwnsNameInterning(t *testing.T) {
	t.Parallel()

	names, err := NewNameTable(16)
	if err != nil {
		t.Fatal(err)
	}
	rt := newCompilerSchemaBuild(names)
	q, err := rt.internQName("urn:test", "value")
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := rt.lookupQName("urn:test", "value"); !ok || got != q {
		t.Fatalf("lookupQName() = %v/%v, want %v/true", got, ok, q)
	}
}

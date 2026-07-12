package compile

import (
	"reflect"
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
	if id != 0 || c.rt.GlobalElements[name] != id || len(c.rt.Elements) != 1 {
		t.Fatalf("registration = id %d globals %v elements %d", id, c.rt.GlobalElements, len(c.rt.Elements))
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
	err = c.load([]source.Source{source.Bytes("bad.xsd", schema)})
	if err != nil {
		t.Fatal(err)
	}
	err = c.index()
	if err != nil {
		t.Fatal(err)
	}
	q, err := c.names.InternQName("urn:test", "bad")
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
	want := runtime.ElementDecl{Name: q, Type: runtime.ComplexRef(c.rt.Builtin.AnyType)}
	if got := c.rt.Elements[id]; !reflect.DeepEqual(got, want) {
		t.Fatalf("failed element = %#v, want reserved placeholder %#v", got, want)
	}
}

func TestSchemaBuildInstallsCorrelatedSubstitutionTables(t *testing.T) {
	t.Parallel()

	c := compiler{rt: newCompilerSchemaBuild(runtime.NameTable{})}
	headName := runtime.QName{Local: 1}
	memberName := runtime.QName{Local: 2}
	head, err := c.addElement(runtime.ElementDecl{Name: headName})
	if err != nil {
		t.Fatal(err)
	}
	member, err := c.addElement(runtime.ElementDecl{Name: memberName, SubstHead: head})
	if err != nil {
		t.Fatal(err)
	}
	c.installSubstitutions(map[runtime.ElementID][]runtime.ElementID{head: {member}})
	if got := c.rt.Substitutions[head]; len(got) != 1 || got[0] != member {
		t.Fatalf("substitutions = %v", got)
	}
	if got, ok := c.rt.SubstitutionMemberByName(head, memberName); !ok || got != member {
		t.Fatalf("substitution lookup = %d/%v, want %d/true", got, ok, member)
	}
	names := c.rt.SubstitutionNames(head)
	if got, ok := names.At(0); names.Len() != 1 || !ok || got != memberName {
		t.Fatalf("substitution names = len %d, first %v/%v", names.Len(), got, ok)
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
	if !runtime.ValidComplexTypeID(c.rt.Builtin.AnyType, len(c.rt.ComplexTypes)) {
		t.Fatalf("xs:anyType ID = %d", c.rt.Builtin.AnyType)
	}
	anyType := c.rt.ComplexTypes[c.rt.Builtin.AnyType]
	if got, ok := c.rt.GlobalTypes[anyType.Name]; !ok || got != runtime.ComplexRef(c.rt.Builtin.AnyType) {
		t.Fatalf("xs:anyType global = %v, %v", got, ok)
	}
	simpleHandles := []struct {
		local string
		id    runtime.SimpleTypeID
	}{
		{local: "anySimpleType", id: c.rt.Builtin.AnySimpleType},
		{local: "string", id: c.rt.Builtin.String},
		{local: "boolean", id: c.rt.Builtin.Boolean},
		{local: "decimal", id: c.rt.Builtin.Decimal},
		{local: "integer", id: c.rt.Builtin.Integer},
		{local: "int", id: c.rt.Builtin.Int},
		{local: "date", id: c.rt.Builtin.Date},
		{local: "dateTime", id: c.rt.Builtin.DateTime},
		{local: "time", id: c.rt.Builtin.Time},
		{local: "anyURI", id: c.rt.Builtin.AnyURI},
		{local: "QName", id: c.rt.Builtin.QName},
		{local: "ID", id: c.rt.Builtin.ID},
		{local: "IDREF", id: c.rt.Builtin.IDREF},
		{local: "IDREFS", id: c.rt.Builtin.IDREFS},
		{local: "NMTOKEN", id: c.rt.Builtin.NMTOKEN},
		{local: "NMTOKENS", id: c.rt.Builtin.NMTOKENS},
		{local: "ENTITY", id: c.rt.Builtin.ENTITY},
		{local: "ENTITIES", id: c.rt.Builtin.ENTITIES},
	}
	for _, handle := range simpleHandles {
		if !runtime.ValidSimpleTypeID(handle.id, len(c.rt.SimpleTypes)) {
			t.Fatalf("xs:%s ID = %d", handle.local, handle.id)
		}
		declaration := c.rt.SimpleTypes[handle.id]
		if got := c.rt.Names.Format(declaration.Name); got != "{"+runtime.XSDNamespaceURI+"}"+handle.local {
			t.Fatalf("builtin ID %d name = %s, want xs:%s", handle.id, got, handle.local)
		}
		if got, ok := c.rt.GlobalTypes[declaration.Name]; !ok || got != runtime.SimpleRef(handle.id) {
			t.Fatalf("xs:%s global = %v, %v", handle.local, got, ok)
		}
	}
}

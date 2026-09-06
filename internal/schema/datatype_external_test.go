package schema_test

import (
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/value"
)

func TestDecimalAndIntegerCanonicalValuesDiverge(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
	rt := engineRuntime(t, engine)
	decimal, err := rt.ValueProgram().Validate(builtinSimpleTypeID(t, rt, "decimal"), "5", value.Resolver{}, value.NeedCanonical, nil)
	if err != nil {
		t.Fatalf("ValueProgram.Validate(decimal) error = %v", err)
	}
	if decimal.CanonicalText() != "5.0" {
		t.Fatalf("decimal canonical = %q, want 5.0", decimal.CanonicalText())
	}
	integer, err := rt.ValueProgram().Validate(builtinSimpleTypeID(t, rt, "int"), "05", value.Resolver{}, value.NeedCanonical, nil)
	if err != nil {
		t.Fatalf("ValueProgram.Validate(int) error = %v", err)
	}
	if integer.CanonicalText() != "5" {
		t.Fatalf("int canonical = %q, want 5", integer.CanonicalText())
	}
}

func builtinSimpleTypeID(t *testing.T, rt *xsdSchema.Schema, local string) xsdSchema.SimpleTypeID {
	t.Helper()
	name, ok := rt.LookupQName("http://www.w3.org/2001/XMLSchema", local)
	if !ok {
		t.Fatalf("built-in type %q is not interned", local)
	}
	typ, ok := rt.GlobalType(name)
	if !ok {
		t.Fatalf("built-in type %q is not published", local)
	}
	id, ok := typ.Simple()
	if !ok {
		t.Fatalf("built-in type %q is not simple", local)
	}
	return id
}

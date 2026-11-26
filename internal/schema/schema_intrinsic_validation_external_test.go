package schema_test

import (
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

type intrinsicTypeMutation struct {
	name   string
	mutate func(st *xsdSchema.SimpleType)
}

// intrinsicDeclarationMutations changes one declaration field of an intrinsic
// type. Its value structure lives in the value builder, which a build cannot
// alter.
func intrinsicDeclarationMutations() []intrinsicTypeMutation {
	return []intrinsicTypeMutation{
		{name: "Final", mutate: func(st *xsdSchema.SimpleType) { st.Final = xsdSchema.DerivationRestriction }},
		{name: "Scope", mutate: func(st *xsdSchema.SimpleType) { st.Scope = xsdSchema.DeclarationScopeGlobal }},
	}
}

func TestPublishSchemaRejectsAlteredMissingSimpleTypeSentinel(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:m="urn:missing"><xs:import namespace="urn:missing"/><xs:element name="root" type="m:T"/></xs:schema>`
	sentinel := func(t *testing.T, build *xsdSchema.SchemaBuild) *xsdSchema.SimpleType {
		t.Helper()
		for i := range build.SimpleTypes {
			if build.SimpleTypes[i].Missing {
				return &build.SimpleTypes[i]
			}
		}
		t.Fatal("compiler did not create missing simple type sentinel")
		return nil
	}
	if err := validateSchemaBuild(mutableSchemaBuild(t, schema)); err != nil {
		t.Fatalf("unaltered sentinel error = %v", err)
	}
	for _, test := range intrinsicDeclarationMutations() {
		t.Run(test.name, func(t *testing.T) {
			build := mutableSchemaBuild(t, schema)
			test.mutate(sentinel(t, build))
			err := validateSchemaBuild(build)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if !strings.Contains(err.Error(), "missing simple type sentinel is invalid") {
				t.Fatalf("error = %v, want the sentinel audit", err)
			}
		})
	}
}

func TestPublishSchemaRejectsAlteredSchemaLocationType(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	schemaLocation := func(t *testing.T, build *xsdSchema.SchemaBuild) *xsdSchema.SimpleType {
		t.Helper()
		name, ok := build.Names.LookupQName(vocab.XSINamespaceURI, "$schemaLocation")
		if !ok {
			t.Fatal("missing intrinsic schemaLocation type name")
		}
		for i := range build.SimpleTypes {
			if build.SimpleTypes[i].Name == name {
				return &build.SimpleTypes[i]
			}
		}
		t.Fatal("missing intrinsic schemaLocation type")
		return nil
	}
	if err := validateSchemaBuild(mutableSchemaBuild(t, schema)); err != nil {
		t.Fatalf("unaltered schemaLocation type error = %v", err)
	}
	for _, test := range intrinsicDeclarationMutations() {
		t.Run(test.name, func(t *testing.T) {
			build := mutableSchemaBuild(t, schema)
			test.mutate(schemaLocation(t, build))
			err := validateSchemaBuild(build)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if !strings.Contains(err.Error(), "intrinsic schemaLocation type differs from its definition") {
				t.Fatalf("error = %v, want the schemaLocation audit", err)
			}
		})
	}
}

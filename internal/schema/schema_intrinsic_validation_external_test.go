package schema_test

import (
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

type intrinsicTypeMutation struct {
	name   string
	mutate func(st *xsdSchema.SimpleType)
}

// intrinsicValueSpecMutations changes one value-specification field, or one
// facet group, of an intrinsic definition that has no members or facets.
func intrinsicValueSpecMutations() []intrinsicTypeMutation {
	return []intrinsicTypeMutation{
		{name: "Union", mutate: func(st *xsdSchema.SimpleType) {
			st.ValueSpec.Union = append(st.ValueSpec.Union, value.BuiltinType(value.PrimitiveString))
		}},
		{name: "Base", mutate: func(st *xsdSchema.SimpleType) { st.ValueSpec.Base = value.BuiltinType(value.PrimitiveBoolean) }},
		{name: "ListItem", mutate: func(st *xsdSchema.SimpleType) { st.ValueSpec.ListItem = value.BuiltinType(value.PrimitiveBoolean) }},
		{name: "Variety", mutate: func(st *xsdSchema.SimpleType) {
			if st.ValueSpec.Variety == value.Union {
				st.ValueSpec.Variety = value.Atomic
			} else {
				st.ValueSpec.Variety = value.Union
			}
		}},
		{name: "Primitive", mutate: func(st *xsdSchema.SimpleType) { st.ValueSpec.Primitive = value.PrimitiveBoolean }},
		{name: "Whitespace", mutate: func(st *xsdSchema.SimpleType) {
			if st.ValueSpec.Whitespace == value.WhitespacePreserve {
				st.ValueSpec.Whitespace = value.WhitespaceCollapse
			} else {
				st.ValueSpec.Whitespace = value.WhitespacePreserve
			}
		}},
		{name: "WhitespacePresent", mutate: func(st *xsdSchema.SimpleType) {
			st.ValueSpec.WhitespacePresent = !st.ValueSpec.WhitespacePresent
		}},
		{name: "Builtin", mutate: func(st *xsdSchema.SimpleType) { st.ValueSpec.Builtin++ }},
		{name: "Patterns", mutate: func(st *xsdSchema.SimpleType) { st.ValueSpec.Facets.Patterns = [][]*value.Pattern{nil} }},
		{name: "Enumeration", mutate: func(st *xsdSchema.SimpleType) {
			st.ValueSpec.Facets.Enumeration = []value.LiteralSpec{{Lexical: "x"}}
		}},
		{name: "BoundFacet", mutate: func(st *xsdSchema.SimpleType) { st.ValueSpec.Facets.MaxInclusive.Lexical = "1" }},
		{name: "BoundFacetResolver", mutate: func(st *xsdSchema.SimpleType) {
			st.ValueSpec.Facets.MinExclusive.Resolver.Notation = func(string, string) bool { return true }
		}},
		{name: "CardinalityFacet", mutate: func(st *xsdSchema.SimpleType) { st.ValueSpec.Facets.MaxLength.Value = 1 }},
		{name: "FacetPresent", mutate: func(st *xsdSchema.SimpleType) { st.ValueSpec.Facets.Present = 1 }},
		{name: "FacetFixed", mutate: func(st *xsdSchema.SimpleType) { st.ValueSpec.Facets.Fixed = 1 }},
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
	mutations := append(intrinsicValueSpecMutations(),
		intrinsicTypeMutation{name: "Final", mutate: func(st *xsdSchema.SimpleType) { st.Final = xsdSchema.DerivationRestriction }},
		intrinsicTypeMutation{name: "Scope", mutate: func(st *xsdSchema.SimpleType) { st.Scope = xsdSchema.DeclarationScopeGlobal }},
	)
	for _, test := range mutations {
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
	for _, test := range intrinsicValueSpecMutations() {
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

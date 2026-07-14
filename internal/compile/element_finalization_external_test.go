package compile_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestRecursiveComplexElementConstraintsFinalizeAfterTypeCompletion(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		typeBody   string
	}{
		{
			name:       "named default",
			constraint: `default="text"`,
			typeBody:   `type="R"`,
		},
		{
			name:       "named fixed",
			constraint: `fixed="text"`,
			typeBody:   `type="R"`,
		},
		{
			name:       "deferred anonymous default",
			constraint: `default="text"`,
			typeBody: `<xs:complexType><xs:complexContent mixed="true">
          <xs:extension base="R"/>
        </xs:complexContent></xs:complexType>`,
		},
		{
			name:       "deferred anonymous fixed",
			constraint: `fixed="text"`,
			typeBody: `<xs:complexType><xs:complexContent mixed="true">
          <xs:extension base="R"/>
        </xs:complexContent></xs:complexType>`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="R" mixed="true">
    <xs:sequence minOccurs="0">
      <xs:element name="child" minOccurs="0" ` + test.constraint + ` ` + test.typeBody + `/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="R"/>
</xs:schema>`
			if test.typeBody[0] == '<' {
				schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="R" mixed="true">
    <xs:sequence minOccurs="0">
      <xs:element name="child" minOccurs="0" ` + test.constraint + `>` + test.typeBody + `</xs:element>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="R"/>
</xs:schema>`
			}
			if _, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
		})
	}
}

func TestElementConstraintFinalizationRejectsCompletedInvalidShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "element only",
			body: `<xs:complexType name="R"><xs:sequence minOccurs="0">
  <xs:element name="child" type="R" minOccurs="0" default="text"/>
</xs:sequence></xs:complexType>`,
		},
		{
			name: "mixed non-emptiable",
			body: `<xs:complexType name="R" mixed="true"><xs:sequence>
  <xs:element name="required"/><xs:element name="child" type="R" minOccurs="0" default="text"/>
</xs:sequence></xs:complexType>`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+test.body+`<xs:element name="root" type="R"/></xs:schema>`))})
			if err == nil {
				t.Fatal("Compile() succeeded")
			}
		})
	}
}

func TestElementConstraintFinalizationIsAtomic(t *testing.T) {
	limits, err := compile.NormalizeOptions(compile.Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := compile.NewCompilerForTest(limits)
	if err != nil {
		t.Fatal(err)
	}
	err = c.LoadForTest([]source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" type="xs:int" default="1"/>
  <xs:element name="b" type="xs:int" default="not-an-int"/>
</xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	err = c.IndexForTest()
	if err != nil {
		t.Fatal(err)
	}
	err = c.CompileGlobalsForTest()
	if err == nil {
		t.Fatal("CompileGlobalsForTest() succeeded")
	}
	build := c.RuntimeForTest()
	for _, local := range []string{"a", "b"} {
		q := mustQName(t, &build.Names, local)
		decl := build.Elements[build.GlobalElements[q]]
		if decl.Default != nil || decl.Fixed != nil {
			t.Fatalf("element %s committed a constraint after batch failure", local)
		}
	}
	_, err = compile.FreezeCompilerRuntimeForTest(c)
	var schemaErr *xsderrors.Error
	if !errors.As(err, &schemaErr) || schemaErr.Code != xsderrors.CodeInternalInvariant {
		t.Fatalf("FreezeCompilerRuntimeForTest() error = %v, want internal invariant", err)
	}
}

func TestSubstitutionEffectiveTypesAreResolvedHeadFirst(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">
  <xs:element name="z" type="xs:int"/>
  <xs:element name="b" substitutionGroup="t:z"/>
  <xs:element name="a" substitutionGroup="t:b"/>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="t:z"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	engine, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<t:root xmlns:t="urn:t"><t:a>7</t:a></t:root>`)
	mustNotValidateRuntime(t, engine, `<t:root xmlns:t="urn:t"><t:a>not-an-int</t:a></t:root>`, xsderrors.CodeValidationFacet)
}

func TestEqualTypeSubstitutionEdgeDoesNotImportUnrelatedAncestorBlocks(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">
  <xs:complexType name="Root" block="extension"/>
  <xs:complexType name="A"><xs:complexContent><xs:restriction base="t:Root"/></xs:complexContent></xs:complexType>
  <xs:complexType name="B"><xs:complexContent><xs:extension base="t:A"/></xs:complexContent></xs:complexType>
  <xs:element name="h" type="t:A"/>
  <xs:element name="m" type="t:A" substitutionGroup="t:h"/>
  <xs:element name="c" type="t:B" substitutionGroup="t:m"/>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="t:h"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	engine, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<t:root xmlns:t="urn:t"><t:c/></t:root>`)
}

func TestSubstitutionConstraintUsesUltimateEffectiveType(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t">
  <xs:element name="z" type="xs:int"/>
  <xs:element name="b" substitutionGroup="t:z"/>
  <xs:element name="a" substitutionGroup="t:b" default="not-an-int"/>
</xs:schema>`
	_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	var schemaErr *xsderrors.Error
	if !errors.As(err, &schemaErr) || schemaErr.Code != xsderrors.CodeSchemaFacet {
		t.Fatalf("Compile() error = %v, want %q", err, xsderrors.CodeSchemaFacet)
	}
}

func TestElementConsistencyRunsAfterSubstitutionTypeFinalization(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" substitutionGroup="t:head"/>
  <xs:complexType name="C"><xs:sequence>
    <xs:element ref="t:member"/><xs:element name="member" type="xs:string"/>
  </xs:sequence></xs:complexType>
  <xs:element name="root" type="t:C"/>
</xs:schema>`
	if _, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestSubstitutionClosureEntryLimit(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="h0" type="xs:string"/>
  <xs:element name="h1" substitutionGroup="h0"/>
  <xs:element name="h2" substitutionGroup="h1"/>
  <xs:element name="h3" substitutionGroup="h2"/>
</xs:schema>`
	if _, err := compile.Compile(context.Background(), compile.Options{MaxSubstitutionClosureEntries: 6}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile(exact limit) error = %v", err)
	}
	_, err := compile.Compile(context.Background(), compile.Options{MaxSubstitutionClosureEntries: 5}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	var schemaErr *xsderrors.Error
	if !errors.As(err, &schemaErr) || schemaErr.Code != xsderrors.CodeSchemaLimit {
		t.Fatalf("Compile(over limit) error = %v, want %q", err, xsderrors.CodeSchemaLimit)
	}
}

func TestSubstitutionCycleDiagnosticNamesAndLocatesCycleMember(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t">
  <xs:element name="a" substitutionGroup="t:y"/>
  <xs:element name="y" substitutionGroup="t:z"/>
  <xs:element name="z" substitutionGroup="t:y"/>
</xs:schema>`
	_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	var schemaErr *xsderrors.Error
	if !errors.As(err, &schemaErr) || schemaErr.Code != xsderrors.CodeSchemaReference {
		t.Fatalf("Compile() error = %v, want %q", err, xsderrors.CodeSchemaReference)
	}
	if !strings.Contains(schemaErr.Message, "y") && !strings.Contains(schemaErr.Message, "z") {
		t.Fatalf("Compile() message = %q, want actual cycle member y or z", schemaErr.Message)
	}
	if schemaErr.Path != "schema.xsd" || schemaErr.Line == 0 {
		t.Fatalf("Compile() location = %q:%d:%d, want schema.xsd with nonzero line", schemaErr.Path, schemaErr.Line, schemaErr.Column)
	}
}

func TestDeferredElementConsistencyDiagnosticRetainsModelLocation(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="C"><xs:sequence>
    <xs:element name="a" type="xs:string"/>
    <xs:element name="a" type="xs:int"/>
  </xs:sequence></xs:complexType>
  <xs:element name="root" type="C"/>
</xs:schema>`
	_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	var schemaErr *xsderrors.Error
	if !errors.As(err, &schemaErr) || schemaErr.Code != xsderrors.CodeSchemaContentModel {
		t.Fatalf("Compile() error = %v, want %q", err, xsderrors.CodeSchemaContentModel)
	}
	if schemaErr.Path != "schema.xsd" || schemaErr.Line == 0 {
		t.Fatalf("Compile() location = %q:%d:%d, want schema.xsd with nonzero line", schemaErr.Path, schemaErr.Line, schemaErr.Column)
	}
}

func TestGeneratedExtensionConsistencyDiagnosticRetainsDerivedLocation(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:sequence>
    <xs:element name="a" type="xs:string"/>
  </xs:sequence></xs:complexType>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="Base"><xs:sequence>
    <xs:element name="a" type="xs:int"/>
  </xs:sequence></xs:extension></xs:complexContent></xs:complexType>
  <xs:element name="root" type="Derived"/>
</xs:schema>`
	_, err := compile.Compile(context.Background(), compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	var schemaErr *xsderrors.Error
	if !errors.As(err, &schemaErr) || schemaErr.Code != xsderrors.CodeSchemaContentModel {
		t.Fatalf("Compile() error = %v, want %q", err, xsderrors.CodeSchemaContentModel)
	}
	if schemaErr.Path != "schema.xsd" || schemaErr.Line < 5 {
		t.Fatalf("Compile() location = %q:%d:%d, want derived extension model location", schemaErr.Path, schemaErr.Line, schemaErr.Column)
	}
}

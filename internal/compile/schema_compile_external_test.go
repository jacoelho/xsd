package compile_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSchemaCompileErrorsIncludeLocation(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		needle string
		code   xsderrors.Code
	}{
		{
			name: "pattern",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad">
    <xs:restriction base="xs:string">
      <xs:pattern value="[z-a]"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
			needle: `<xs:pattern`,
			code:   xsderrors.CodeSchemaFacet,
		},
		{
			name: "identity",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="child"/></xs:sequence></xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="."/>
      <xs:field xpath="/bad"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			needle: `<xs:field`,
			code:   xsderrors.CodeSchemaIdentity,
		},
		{
			name: "content",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Bad">
    <xs:element name="child"/>
  </xs:complexType>
</xs:schema>`,
			needle: `<xs:element name="child"`,
			code:   xsderrors.CodeSchemaContentModel,
		},
		{
			name: "duplicate schema id",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:simpleContent id="dup">
        <xs:extension id="dup" base="xs:string"/>
      </xs:simpleContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			needle: `<xs:extension id="dup"`,
			code:   xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			name: "invalid schema component name",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="0"/>
</xs:schema>`,
			needle: `<xs:attributeGroup name="0"`,
			code:   xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			name: "nested annotation",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation>
    <xs:annotation/>
  </xs:annotation>
</xs:schema>`,
			needle: `<xs:annotation/>`,
			code:   xsderrors.CodeSchemaContentModel,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(test.schema))})
			expectCode(t, err, test.code)
			expectSchemaCompileLine(t, err, lineOf(test.schema, test.needle))
		})
	}
}

func expectSchemaCompileLine(t *testing.T, err error, line int) {
	t.Helper()
	x, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error type = %T, want *xsderrors.Error", err)
	}
	if x.Category != xsderrors.CategorySchemaCompile {
		t.Fatalf("error category = %s, want %s", x.Category, xsderrors.CategorySchemaCompile)
	}
	if x.Line != line || x.Column == 0 {
		t.Fatalf("error location = %d:%d, want line %d and non-zero column", x.Line, x.Column, line)
	}
}

func lineOf(s, needle string) int {
	before, _, ok := strings.Cut(s, needle)
	if !ok {
		return 0
	}
	return strings.Count(before, "\n") + 1
}

func TestInvalidSchemaContentOrdering(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:attribute name="a"/>
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad">
    <xs:attribute name="a"/>
    <xs:complexType/>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:attribute name="a"/>
    <xs:annotation/>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad"><xs:complexType name="localName"/></xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad" block="substitution"/>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent mixed="true"><xs:extension base="base"/></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"/>
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="base"/><xs:annotation/></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"/>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:sequence/><xs:choice/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="base"><xs:all><xs:element name="b"/></xs:all></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestInvalidAnnotationStructureIsSchemaError(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation><xs:annotation/></xs:annotation>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:annotation/>
    <xs:annotation/>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation foo="bar"/>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation><xs:documentation xml:lang=" "/></xs:annotation>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="g">
    <xs:attribute name="a"/>
    <xs:annotation/>
  </xs:attributeGroup>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestComplexContentCannotDeriveFromItself(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="bad"><xs:sequence><xs:element name="child"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestSimpleTypeCannotRestrictAnySimpleType(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="bad"><xs:restriction base="xs:anySimpleType"/></xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestSimpleAndComplexTypesShareNames(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="dup"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="dup"/>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaDuplicate)
}

func TestImportedXMLNamespaceSchemaDefersToBuiltinAttributes(t *testing.T) {
	xmlSchema := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://www.w3.org/XML/1998/namespace">
  <xs:attribute name="lang" type="xs:string"/>
</xs:schema>`
	schema := `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:xml="http://www.w3.org/XML/1998/namespace">
  <xs:import namespace="http://www.w3.org/XML/1998/namespace" schemaLocation="xml.xsd"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute ref="xml:lang"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(schema)),
		source.Bytes("xml.xsd", []byte(xmlSchema))})

	if err != nil {
		t.Fatal(err)
	}
	mustValidateRuntime(t, engine, `<root xml:lang="en"/>`)
	mustNotValidateRuntime(t, engine, `<root xml:lang="@@"/>`, xsderrors.CodeValidationFacet)
}

func TestMissingElementTypeInvalidatesOnlyThatElement(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="good" type="xs:int"/>
  <xs:element name="bad" type="absent"/>
	</xs:schema>`)
	mustValidateRuntime(t, engine, `<good>1</good>`)
	mustNotValidateRuntime(t, engine, `<bad>1</bad>`, xsderrors.CodeInternalInvariant)
}

func TestElementDeclarationsMustBeConsistent(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="a" type="xs:int"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestExtendedElementDeclarationsMustBeConsistent(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence><xs:element name="item" type="xs:int"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="bad">
    <xs:complexContent>
      <xs:extension base="base">
        <xs:sequence><xs:element name="item" type="xs:date"/></xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestTypeFinalBlocksDerivation(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:complexType name="Base" final="extension"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="tns:Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Base" final="restriction"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="tns:Base"><xs:minLength value="1"/></xs:restriction></xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base" final="extension">
    <xs:simpleContent>
      <xs:extension base="xs:string"><xs:attribute name="a"/></xs:extension>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="Base"><xs:attribute name="b"/></xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestAnonymousSimpleTypeCannotHaveName(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="parent"><xs:restriction><xs:simpleType name="child"><xs:restriction base="xs:string"/></xs:simpleType></xs:restriction></xs:simpleType></xs:schema>`))})
	expectCode(t, err, xsderrors.CodeSchemaInvalidAttribute)
}

func TestSimpleDerivationAnnotationMustPrecedeContent(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="t">
    <xs:list>
      <xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>
      <xs:annotation/>
    </xs:list>
  </xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="t">
    <xs:union>
      <xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>
      <xs:annotation/>
    </xs:union>
  </xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestTopLevelSimpleTypeRequiresName(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestRestrictionElementPropertiesCannotBeLoosened(t *testing.T) {
	tests := []string{
		`<xs:complexType name="base"><xs:choice><xs:element name="e1" fixed="foo" type="xs:string"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:element name="e1" fixed="bar" type="xs:string"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:choice><xs:element name="e1" block="extension restriction"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:element name="e1" block="extension substitution"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for _, body := range tests {
		_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`))})
		expectCode(t, err, xsderrors.CodeSchemaContentModel)
	}
}

func TestRestrictionElementPreservesFixedValueIdentity(t *testing.T) {
	mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="intValue"><xs:restriction base="xs:integer"/></xs:simpleType>
  <xs:complexType name="base">
    <xs:sequence><xs:element name="a" type="xs:decimal" fixed="5.0"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence><xs:element name="a" type="intValue" fixed="5"/></xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`)
}

func TestRestrictionElementTypeCannotUseExtension(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:t="urn:test">
  <xs:complexType name="baseType"><xs:choice><xs:element name="f1"/><xs:element name="f2"/></xs:choice></xs:complexType>
  <xs:complexType name="extendedType">
    <xs:complexContent>
      <xs:extension base="t:baseType"><xs:choice><xs:element name="f3"/><xs:element name="f4"/></xs:choice></xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="base"><xs:choice><xs:element name="c1" type="t:baseType"/><xs:element name="c2"/></xs:choice></xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="t:base"><xs:choice><xs:element name="c1" type="t:extendedType"/><xs:element name="c2"/></xs:choice></xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRestrictionElementCanUseSubstitutionMember(t *testing.T) {
	mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="c" substitutionGroup="d" type="xs:anyType"/>
  <xs:element name="d" type="xs:anyType"/>
  <xs:complexType name="base"><xs:sequence><xs:element ref="d"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element ref="c"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)
}

func TestSubstitutionMemberInheritsHeadType(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:int"/>
  <xs:element name="member" substitutionGroup="head"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<member>1</member>`)
	mustNotValidateRuntime(t, engine, `<member>x</member>`, xsderrors.CodeValidationFacet)
}

func TestSubstitutionMemberWithMissingHeadUsesDefaultType(t *testing.T) {
	engine := mustCompileRuntime(t, `
		<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		  <xs:element name="member" substitutionGroup="missing"/>
		</xs:schema>`)
	mustValidateRuntime(t, engine, `<member>anything</member>`)
}

func TestContentModelSubstitutionRespectsElementBlock(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:int"/>
  <xs:element name="member" substitutionGroup="head"/>
  <xs:element name="blocked" type="xs:int" block="substitution"/>
  <xs:element name="blockedMember" substitutionGroup="blocked"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:element ref="head"/>
        <xs:element ref="blocked"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<root><member>1</member></root>`)
	mustNotValidateRuntime(t, engine, `<root><blockedMember>1</blockedMember></root>`, xsderrors.CodeValidationElement)
}

func TestFreezeRejectsSubstitutionClosureDrift(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" substitutionGroup="head" type="xs:string"/>
  <xs:element name="other" type="xs:string"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema, head, member, other runtime.ElementID)
	}{
		{
			name: "phantom substitution member",
			mutate: func(t *testing.T, rt *runtime.Schema, head, _, other runtime.ElementID) {
				t.Helper()
				rt.Substitutions[head] = append(rt.Substitutions[head], other)
				rt.SubstitutionLookup[head][rt.Elements[other].Name] = other
			},
		},
		{
			name: "missing declared member",
			mutate: func(t *testing.T, rt *runtime.Schema, head, _, _ runtime.ElementID) {
				t.Helper()
				rt.Substitutions[head] = nil
				rt.SubstitutionLookup[head] = nil
			},
		},
		{
			name: "stale lookup",
			mutate: func(t *testing.T, rt *runtime.Schema, head, member, _ runtime.ElementID) {
				t.Helper()
				delete(rt.SubstitutionLookup[head], rt.Elements[member].Name)
			},
		},
		{
			name: "cycle",
			mutate: func(t *testing.T, rt *runtime.Schema, head, member, _ runtime.ElementID) {
				t.Helper()
				rt.Elements[head].SubstHead = member
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			head := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "head")]
			member := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "member")]
			other := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "other")]
			tc.mutate(t, publishedRuntime(t, engine), head, member, other)
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsSubstitutionMapsWithoutHeads(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
	root := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	publishedRuntime(t, engine).Substitutions = map[runtime.ElementID][]runtime.ElementID{root: {root}}
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsInvalidWildcards(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="urn:b urn:a" processContents="lax" minOccurs="0"/>
      </xs:sequence>
      <xs:anyAttribute namespace="##other" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	wildcardByMode := func(t *testing.T, rt *runtime.Schema, mode runtime.WildcardMode) *runtime.Wildcard {
		t.Helper()
		for i := range rt.Wildcards {
			if rt.Wildcards[i].Mode == mode {
				return &rt.Wildcards[i]
			}
		}
		t.Fatalf("wildcard mode %d not found", mode)
		return nil
	}
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema)
	}{
		{
			name: "invalid process",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				wildcardByMode(t, rt, runtime.WildcardList).Process = runtime.ProcessContents(99)
			},
		},
		{
			name: "invalid mode",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				wildcardByMode(t, rt, runtime.WildcardList).Mode = runtime.WildcardMode(99)
			},
		},
		{
			name: "stale other field",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				ns, ok := rt.Names.LookupNamespace("urn:a")
				if !ok {
					t.Fatal("urn:a namespace not interned")
				}
				wildcardByMode(t, rt, runtime.WildcardList).OtherThan = ns
			},
		},
		{
			name: "unnormalized namespace list",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				w := wildcardByMode(t, rt, runtime.WildcardList)
				if len(w.Namespaces) < 2 {
					t.Fatalf("wildcard namespace list length = %d, want >= 2", len(w.Namespaces))
				}
				w.Namespaces[0], w.Namespaces[1] = w.Namespaces[1], w.Namespaces[0]
			},
		},
		{
			name: "invalid namespace id",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				wildcardByMode(t, rt, runtime.WildcardOther).OtherThan = runtime.NamespaceID(1 << 30)
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, publishedRuntime(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestAnonymousLocalTypeCanRestrictContainingType(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns="urn:test">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="foo"/>
      <xs:element name="bar" minOccurs="0">
        <xs:complexType>
          <xs:complexContent>
            <xs:restriction base="base">
              <xs:sequence><xs:element name="foo"/></xs:sequence>
            </xs:restriction>
          </xs:complexContent>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="base"/>
</xs:schema>`)
	mustValidateRuntime(t, engine, `<t:root xmlns:t="urn:test"><foo/><bar><foo/></bar></t:root>`)
}

func TestNamedComplexTypeCannotDeriveFromItself(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="self">
    <xs:complexContent><xs:extension base="self"/></xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestComplexContentExtensionCannotDropMixedBase(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base" mixed="true">
    <xs:sequence><xs:element name="a"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:extension base="base"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base" mixed="true">
    <xs:sequence><xs:element name="a" minOccurs="0"/></xs:sequence>
  </xs:complexType>
  <xs:element name="r">
    <xs:complexType>
      <xs:complexContent>
        <xs:extension base="base">
          <xs:sequence><xs:element name="b" minOccurs="0"/></xs:sequence>
        </xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestRecursiveComplexTypeThroughElementRefCompiles(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="node"/>
  <xs:element name="child" type="node"/>
  <xs:complexType name="node">
    <xs:choice maxOccurs="unbounded">
      <xs:element ref="child" minOccurs="0"/>
    </xs:choice>
  </xs:complexType>
</xs:schema>`))})

	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
}

func TestUnsupportedFeaturesAreExplicit(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:redefine schemaLocation="a.xsd"/></xs:schema>`))})
	expectCode(t, err, xsderrors.CodeUnsupportedRedefine)

	_, err = compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:anyAttribute notQName="##defined"/></xs:complexType></xs:element></xs:schema>`))})
	expectCode(t, err, xsderrors.CodeUnsupportedXSD11)
}

func TestCompileOptionsSchemaXMLLimits(t *testing.T) {
	deepSchema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:documentation>ok</xs:documentation></xs:annotation></xs:schema>`
	_, err := compile.Compile(compile.Options{MaxSchemaDepth: 2}, []source.Source{source.Bytes("schema.xsd", []byte(deepSchema))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaLimit)
	if _, boundaryErr := compile.Compile(compile.Options{MaxSchemaDepth: 3}, []source.Source{source.Bytes("schema.xsd", []byte(deepSchema))}); boundaryErr != nil {
		t.Fatalf("Compile() depth boundary error = %v", boundaryErr)
	}

	attrSchema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"><xs:element name="root"/></xs:schema>`
	_, err = compile.Compile(compile.Options{MaxSchemaAttributes: 1}, []source.Source{source.Bytes("schema.xsd", []byte(attrSchema))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaLimit)
	if _, boundaryErr := compile.Compile(compile.Options{MaxSchemaAttributes: 2}, []source.Source{source.Bytes("schema.xsd", []byte(attrSchema))}); boundaryErr != nil {
		t.Fatalf("Compile() attribute boundary error = %v", boundaryErr)
	}

	textSchema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:documentation>` + strings.Repeat("x", 129) + `</xs:documentation></xs:annotation></xs:schema>`
	_, err = compile.Compile(compile.Options{MaxSchemaTokenBytes: 128}, []source.Source{source.Bytes("schema.xsd", []byte(textSchema))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaLimit)
}

func TestCompileOptionsSchemaSourceByteLimit(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	if _, err := compile.Compile(compile.Options{MaxSchemaSourceBytes: int64(len(schema))}, []source.Source{source.Bytes("schema.xsd", []byte(schema))}); err != nil {
		t.Fatalf("Compile() source byte boundary error = %v", err)
	}

	_, err := compile.Compile(compile.Options{MaxSchemaSourceBytes: int64(len(schema) - 1)}, []source.Source{source.Bytes("schema.xsd", []byte(schema))})
	expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
}

func TestSchemaNamespaceContextsAreIsolated(t *testing.T) {
	limits, err := compile.NormalizeOptions(compile.Options{})
	if err != nil {
		t.Fatalf("NormalizeOptions() error = %v", err)
	}
	root, err := compile.ParseSchemaRootForTest([]byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:test"
           xmlns="urn:test"
           targetNamespace="urn:test">
  <xs:simpleType name="base"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:annotation xmlns:t="urn:other" xmlns="">
    <xs:documentation>namespace reset must stay local</xs:documentation>
  </xs:annotation>
  <xs:element name="prefixed" type="t:base"/>
  <xs:element name="defaulted" type="base"/>
  <xs:element name="local" xmlns:u="urn:test" type="u:base"/>
</xs:schema>`), limits)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	if got := root.NS["t"]; got != "urn:test" {
		t.Fatalf("root prefix t = %q, want urn:test", got)
	}
	if got := root.NS[""]; got != "urn:test" {
		t.Fatalf("root default namespace = %q, want urn:test", got)
	}
	annotation := root.Children[1]
	if got := annotation.NS["t"]; got != "urn:other" {
		t.Fatalf("annotation prefix t = %q, want urn:other", got)
	}
	if got := annotation.NS[""]; got != "" {
		t.Fatalf("annotation default namespace = %q, want empty", got)
	}
	prefixed := root.Children[2]
	if got := prefixed.NS["t"]; got != "urn:test" {
		t.Fatalf("sibling prefix t = %q, want urn:test", got)
	}
	defaulted := root.Children[3]
	if got := defaulted.NS[""]; got != "urn:test" {
		t.Fatalf("sibling default namespace = %q, want urn:test", got)
	}
	local := root.Children[4]
	if got := local.NS["u"]; got != "urn:test" {
		t.Fatalf("local prefix u = %q, want urn:test", got)
	}
}

func TestCompileOptionsRejectNegativeLimits(t *testing.T) {
	schemaSource := source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`))
	tests := []compile.Options{
		{MaxSchemaDepth: -1},
		{MaxSchemaAttributes: -1},
		{MaxSchemaTokenBytes: -1},
		{MaxSchemaSourceBytes: -1},
		{MaxSchemaNames: -1},
		{MaxContentModelStates: -1},
	}
	for _, opts := range tests {
		_, err := compile.Compile(opts, []source.Source{schemaSource})
		expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	}
}

func TestFreezeRejectsInconsistentValueConstraints(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string" default="abc"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(rt *runtime.Schema, decl *runtime.ElementDecl)
	}{
		{
			name: "canonical value mismatch",
			mutate: func(_ *runtime.Schema, decl *runtime.ElementDecl) {
				decl.Default.Value.Canonical = "other"
			},
		},
		{
			name: "invalid value type",
			mutate: func(_ *runtime.Schema, decl *runtime.ElementDecl) {
				decl.Default.Value.Type = runtime.SimpleTypeID(1 << 30)
			},
		},
		{
			name: "stale valid value type",
			mutate: func(rt *runtime.Schema, decl *runtime.ElementDecl) {
				decl.Default.Value.Type = rt.Builtin.Boolean
			},
		},
		{
			name: "stale identity key",
			mutate: func(_ *runtime.Schema, decl *runtime.ElementDecl) {
				decl.Default.Value.Identity = "stale"
			},
		},
		{
			name: "stale idref payload",
			mutate: func(_ *runtime.Schema, decl *runtime.ElementDecl) {
				decl.Default.Value.IDRefs = decl.Default.Value.Canonical
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			rootID := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
			tc.mutate(publishedRuntime(t, engine), &publishedRuntime(t, engine).Elements[rootID])
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsBothDefaultAndFixedValueConstraints(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="ga" type="xs:string" default="a"/>
  <xs:element name="value" type="xs:string" default="v"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="la" type="xs:string" default="b"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema)
	}{
		{
			name: "element declaration",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				id := rt.GlobalElements[mustQName(t, rt, "value")]
				rt.Elements[id].Fixed = runtime.CloneValueConstraint(rt.Elements[id].Default)
			},
		},
		{
			name: "attribute declaration",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				id := rt.GlobalAttributes[mustQName(t, rt, "ga")]
				rt.Attributes[id].Fixed = runtime.CloneValueConstraint(rt.Attributes[id].Default)
			},
		},
		{
			name: "attribute use",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				engine := rt
				set := rootAttributeUseSet(t, engine)
				set.Uses[0].Fixed = runtime.CloneValueConstraint(set.Uses[0].Default)
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, publishedRuntime(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsUnionValueConstraintStoredAsOwnerType(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:boolean"/>
  </xs:simpleType>
  <xs:element name="root" type="U" default="1"/>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	unionID := simpleTypeIDByName(t, publishedRuntime(t, engine), "U")
	publishedRuntime(t, engine).Elements[rootID].Default.Value.Type = unionID
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsValueConstraintThatNoLongerSatisfiesFacets(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:enumeration value="A"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Code" default="A"/>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	defaultValue := publishedRuntime(t, engine).Elements[rootID].Default
	defaultValue.Lexical = "B"
	defaultValue.Canonical = "B"
	defaultValue.Value.Canonical = "B"
	defaultValue.Value.Identity = runtime.SimpleIdentityKey(runtime.PrimitiveString, "B")
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeReplaysResolvedQNameValueConstraint(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:t="urn:test">
  <xs:element name="root" type="xs:QName" default="t:item"/>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() error = %v", err)
	}
}

func TestFreezeRejectsInvalidResolvedQNameReplay(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:t="urn:test">
  <xs:element name="root" type="xs:QName" default="t:item"/>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	defaultValue := publishedRuntime(t, engine).Elements[rootID].Default
	defaultValue.Lexical = "bad::item"
	defaultValue.Canonical = runtime.FormatExpandedName("urn:test", "item")
	defaultValue.Value.Canonical = defaultValue.Canonical
	defaultValue.Value.Identity = runtime.SimpleIdentityKey(runtime.PrimitiveQName, defaultValue.Canonical)
	defaultValue.ResolvedNames = []runtime.ResolvedValueName{{Lexical: defaultValue.Lexical, NS: "urn:test", Local: "item"}}
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsUnusedResolvedNameProof(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:t="urn:test">
  <xs:element name="root" type="xs:QName" default="t:item"/>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	defaultValue := publishedRuntime(t, engine).Elements[rootID].Default
	defaultValue.ResolvedNames = append(defaultValue.ResolvedNames, runtime.ResolvedValueName{Lexical: "t:other", NS: "urn:test", Local: "other"})
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsNondeterministicResolvedNameProof(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:p="urn:a">
  <xs:simpleType name="QNames">
    <xs:list itemType="xs:QName"/>
  </xs:simpleType>
  <xs:element name="root" type="QNames" default="p:x p:x"/>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	defaultValue := publishedRuntime(t, engine).Elements[rootID].Default
	if len(defaultValue.ResolvedNames) != 2 {
		t.Fatalf("resolved names = %d, want 2", len(defaultValue.ResolvedNames))
	}
	canonical := runtime.FormatExpandedName("urn:a", "x") + " " + runtime.FormatExpandedName("urn:b", "x")
	defaultValue.ResolvedNames[1].NS = "urn:b"
	defaultValue.Canonical = canonical
	defaultValue.Value.Canonical = canonical
	defaultValue.Value.Identity = runtime.SimpleIdentityKey(runtime.PrimitiveString, canonical)
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsMixedValueConstraintIdentityPayload(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" default="text">
    <xs:complexType mixed="true"/>
  </xs:element>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	publishedRuntime(t, engine).Elements[rootID].Default.Value.Identity = "stale"
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsMixedValueConstraintResolvedNameProof(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" default="text">
    <xs:complexType mixed="true"/>
  </xs:element>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	rootID := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	publishedRuntime(t, engine).Elements[rootID].Default.ResolvedNames = []runtime.ResolvedValueName{{Lexical: "p:x", NS: "urn:test", Local: "x"}}
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsCyclicUnionValueConstraintOwner(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:boolean"/>
  </xs:simpleType>
  <xs:element name="root" type="U" default="1"/>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	unionID := simpleTypeIDByName(t, publishedRuntime(t, engine), "U")
	publishedRuntime(t, engine).SimpleTypes[unionID].Union = []runtime.SimpleTypeID{unionID}
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsInconsistentNameTable(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	publishedRuntime(t, engine).Names = runtime.NameTable{}
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsGlobalNameMismatch(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" type="xs:string"/>
  <xs:element name="b" type="xs:string"/>
  <xs:attribute name="ga" type="xs:string"/>
  <xs:attribute name="gb" type="xs:string"/>
  <xs:simpleType name="t1">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="t2">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string"/>
            <xs:attribute name="id2" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k1">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:key name="k2">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id2"/>
    </xs:key>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema)
	}{
		{
			name: "global element points at other declaration",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.GlobalElements[mustQName(t, rt, "a")] = rt.GlobalElements[mustQName(t, rt, "b")]
			},
		},
		{
			name: "global attribute points at other declaration",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.GlobalAttributes[mustQName(t, rt, "ga")] = rt.GlobalAttributes[mustQName(t, rt, "gb")]
			},
		},
		{
			name: "global type points at other type",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.GlobalTypes[mustQName(t, rt, "t1")] = rt.GlobalTypes[mustQName(t, rt, "t2")]
			},
		},
		{
			name: "global identity points at other constraint",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.GlobalIdentities[mustQName(t, rt, "k1")] = rt.GlobalIdentities[mustQName(t, rt, "k2")]
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, publishedRuntime(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsIdentityFieldLookupDrift(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="name" type="xs:string"/>
            </xs:sequence>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k1">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
      <xs:field xpath="name"/>
    </xs:key>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(ic *runtime.IdentityConstraint)
	}{
		{
			name: "dropped attribute lookup",
			mutate: func(ic *runtime.IdentityConstraint) {
				ic.AttributeFields = nil
			},
		},
		{
			name: "element lookup field index drift",
			mutate: func(ic *runtime.IdentityConstraint) {
				ic.ElementFields[0].Field = 7
			},
		},
		{
			name: "extra wildcard lookup entry",
			mutate: func(ic *runtime.IdentityConstraint) {
				ic.AttributeWildcardFields = append(ic.AttributeWildcardFields, runtime.CompiledIdentityField{Field: 0})
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := publishedRuntime(t, engine).GlobalIdentities[mustQName(t, publishedRuntime(t, engine), "k1")]
			tc.mutate(&publishedRuntime(t, engine).Identities[id])
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsIdentityKindReferMismatch(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="kr1" refer="k">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:keyref>
    <xs:keyref name="kr2" refer="k">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema)
	}{
		{
			name: "key stores refer",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]].Refer = rt.GlobalIdentities[mustQName(t, rt, "kr1")]
			},
		},
		{
			name: "keyref missing refer",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "kr1")]].Refer = runtime.NoIdentityConstraint
			},
		},
		{
			name: "keyref references keyref",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "kr1")]].Refer = rt.GlobalIdentities[mustQName(t, rt, "kr2")]
			},
		},
		{
			name: "keyref field count drift",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				kr := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "kr1")]]
				kr.Fields = append(kr.Fields, runtime.IdentityField{})
				kr.ElementFields, kr.AttributeFields, kr.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(kr.Fields)
			},
		},
		{
			name: "missing selector",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]].Selector = nil
			},
		},
		{
			name: "missing fields",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Fields = nil
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "field without paths",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Fields[0].Paths = nil
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "selector self stores ignored path",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Selector[0] = runtime.IdentityPath{
					Self:       true,
					Descendant: true,
					Steps: []runtime.IdentityStep{{
						Name: mustQName(t, rt, "item"),
					}},
				}
			},
		},
		{
			name: "selector wildcard stores ignored name",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Selector[0].Steps[0].Wildcard = true
				ic.Selector[0].Steps[0].Name = runtime.QName{}
			},
		},
		{
			name: "field self stores ignored attribute",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Fields[0].Paths[0] = runtime.IdentityFieldPath{
					Self:      true,
					Attr:      true,
					Attribute: mustQName(t, rt, "id"),
				}
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "element field stores ignored attribute",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Fields[0].Paths[0] = runtime.IdentityFieldPath{
					Steps: []runtime.IdentityStep{{
						Name: mustQName(t, rt, "item"),
					}},
					Attribute: runtime.QName{},
				}
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(ic.Fields)
			},
		},
		{
			name: "attribute wildcard stores ignored name",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				ic := &rt.Identities[rt.GlobalIdentities[mustQName(t, rt, "k")]]
				ic.Fields[0].Paths[0].AttrWildcard = true
				ic.Fields[0].Paths[0].Attribute = mustQName(t, rt, "id")
				ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = runtime.BuildIdentityFieldLookup(ic.Fields)
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, publishedRuntime(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func rootAttributeUseSet(t *testing.T, engine *runtime.Schema) *runtime.AttributeUseSet {
	t.Helper()
	rootID := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	ctID, ok := publishedRuntime(t, engine).Elements[rootID].Type.Complex()
	if !ok {
		t.Fatal("root element type is not complex")
	}
	attrs := publishedRuntime(t, engine).ComplexTypes[ctID].Attrs
	if attrs == runtime.NoAttributeUseSet {
		t.Fatal("root complex type has no attribute use set")
	}
	return &publishedRuntime(t, engine).AttributeUseSets[attrs]
}

func TestFreezeRejectsAttributeUseSetIndexDrift(t *testing.T) {
	t.Run("stale index on empty uses", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute processContents="lax"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
		engine := mustCompileRuntime(t, schema)
		if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		set := rootAttributeUseSet(t, engine)
		if len(set.Uses) != 0 {
			t.Fatalf("expected empty attribute uses, got %d", len(set.Uses))
		}
		set.Index = map[runtime.QName]uint32{mustQName(t, publishedRuntime(t, engine), "root"): 5}
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("missing index entry", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:string"/>
      <xs:attribute name="b" type="xs:string"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
		engine := mustCompileRuntime(t, schema)
		if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		set := rootAttributeUseSet(t, engine)
		if len(set.Uses) != 2 {
			t.Fatalf("expected two attribute uses, got %d", len(set.Uses))
		}
		delete(set.Index, set.Uses[0].Name)
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
}

func TestFreezeRejectsAttributeUseSetDerivedSlotDrift(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="required" type="xs:string" use="required"/>
      <xs:attribute name="defaulted" type="xs:string" default="x"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(set *runtime.AttributeUseSet)
	}{
		{
			name: "missing required slot",
			mutate: func(set *runtime.AttributeUseSet) {
				set.Required = nil
			},
		},
		{
			name: "missing value constraint slot",
			mutate: func(set *runtime.AttributeUseSet) {
				set.ValueConstraints = nil
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(rootAttributeUseSet(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsPublishedProhibitedAttributeUse(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="plain" type="xs:string"/>
      <xs:attribute name="defaulted" type="xs:string" default="x"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	useByName := func(t *testing.T, rt *runtime.Schema, set *runtime.AttributeUseSet, local string) *runtime.AttributeUse {
		t.Helper()
		name := mustQName(t, rt, local)
		for i := range set.Uses {
			if set.Uses[i].Name == name {
				return &set.Uses[i]
			}
		}
		t.Fatalf("attribute use %q not found", local)
		return nil
	}
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema, set *runtime.AttributeUseSet)
	}{
		{
			name: "plain prohibited",
			mutate: func(t *testing.T, rt *runtime.Schema, set *runtime.AttributeUseSet) {
				t.Helper()
				useByName(t, rt, set, "plain").Prohibited = true
			},
		},
		{
			name: "prohibited with default",
			mutate: func(t *testing.T, rt *runtime.Schema, set *runtime.AttributeUseSet) {
				t.Helper()
				useByName(t, rt, set, "defaulted").Prohibited = true
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, publishedRuntime(t, engine), rootAttributeUseSet(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsIDAttributeSchemaInvariantDrift(t *testing.T) {
	t.Run("attribute declaration value constraint", func(t *testing.T) {
		engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" type="xs:string"/>
</xs:schema>`)
		if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		attr := publishedRuntime(t, engine).GlobalAttributes[mustQName(t, publishedRuntime(t, engine), "a")]
		publishedRuntime(t, engine).Attributes[attr].Type = publishedRuntime(t, engine).Builtin.ID
		publishedRuntime(t, engine).Attributes[attr].Default = runtimeValueConstraint(t, publishedRuntime(t, engine), publishedRuntime(t, engine).Builtin.ID, "abc")
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("element declaration value constraint", func(t *testing.T) {
		engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
		if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		root := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
		publishedRuntime(t, engine).Elements[root].Type = runtime.SimpleRef(publishedRuntime(t, engine).Builtin.ID)
		publishedRuntime(t, engine).Elements[root].Default = runtimeValueConstraint(t, publishedRuntime(t, engine), publishedRuntime(t, engine).Builtin.ID, "abc")
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("attribute use value constraint", func(t *testing.T) {
		engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:attribute name="a" type="xs:string"/></xs:complexType>
  </xs:element>
</xs:schema>`)
		if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		set := rootAttributeUseSet(t, engine)
		set.Uses[0].Type = publishedRuntime(t, engine).Builtin.ID
		set.Uses[0].Default = runtimeValueConstraint(t, publishedRuntime(t, engine), publishedRuntime(t, engine).Builtin.ID, "abc")
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("multiple ID attribute uses", func(t *testing.T) {
		engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:ID"/>
      <xs:attribute name="b" type="xs:string"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
		if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		set := rootAttributeUseSet(t, engine)
		set.Uses[1].Type = publishedRuntime(t, engine).Builtin.ID
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
}

func TestFreezeRejectsBareNotationElementValueConstraint(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="gif" public="image/gif"/>
  <xs:element name="root" type="xs:NOTATION"/>
</xs:schema>`)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	root := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	notationQName, ok := publishedRuntime(t, engine).Names.LookupQName(vocab.XSDNamespaceURI, "NOTATION")
	if !ok {
		t.Fatal("missing NOTATION builtin QName")
	}
	notationID, ok := publishedRuntime(t, engine).GlobalTypes[notationQName].Simple()
	if !ok {
		t.Fatal("NOTATION builtin is not a simple type")
	}
	publishedRuntime(t, engine).Elements[root].Default = runtimeValueConstraint(t, publishedRuntime(t, engine), notationID, "gif")
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsBrokenDFARowIndex(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:string" abstract="true"/>
  <xs:element name="sub" type="xs:string" substitutionGroup="head"/>
  <xs:element name="r">
    <xs:complexType>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="c1" type="xs:string"/>
        <xs:element name="c2" type="xs:string"/>
        <xs:element name="c3" type="xs:string"/>
        <xs:element name="c4" type="xs:string"/>
        <xs:element name="c5" type="xs:string"/>
        <xs:element name="c6" type="xs:string"/>
        <xs:element name="c7" type="xs:string"/>
        <xs:element ref="head"/>
        <xs:any namespace="urn:a" processContents="lax"/>
        <xs:any namespace="urn:b" processContents="lax"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	indexedRow := func(t *testing.T, engine *runtime.Schema) *runtime.CompiledModelRow {
		t.Helper()
		model := publishedRuntime(t, engine).CompiledModels[rootContentModel(t, engine)]
		for i := range model.Rows {
			if model.Rows[i].Index.IsEnabled() {
				return &model.Rows[i]
			}
		}
		t.Fatal("no indexed row in root content model")
		return nil
	}
	anyKey := func(t *testing.T, idx runtime.DFARowIndex) runtime.QName {
		t.Helper()
		for k := range idx.NameToEdge {
			return k
		}
		t.Fatal("name index is empty")
		return runtime.QName{}
	}
	mutations := []struct {
		name   string
		mutate func(t *testing.T, row *runtime.CompiledModelRow)
	}{
		{
			name: "name index position out of range",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				row.Index.NameToEdge[anyKey(t, row.Index)] = ^uint32(0)
			},
		},
		{
			name: "name index points at wildcard edge",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				row.Index.NameToEdge[anyKey(t, row.Index)] = row.Index.WildcardEdges[0]
			},
		},
		{
			name: "name index key does not match edge element",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				idx := row.Index
				a := anyKey(t, idx)
				own := idx.NameToEdge[a]
				for _, pos := range idx.NameToEdge {
					if pos != own {
						idx.NameToEdge[a] = pos
						return
					}
				}
				t.Fatal("name index has no second edge position")
			},
		},
		{
			name: "element edge missing from name index",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				delete(row.Index.NameToEdge, anyKey(t, row.Index))
			},
		},
		{
			name: "wildcard edge positions out of order",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				w := row.Index.WildcardEdges
				if len(w) < 2 {
					t.Fatalf("len(WildcardEdges) = %d, want >= 2", len(w))
				}
				w[0], w[1] = w[1], w[0]
			},
		},
		{
			name: "wildcard list contains element edge",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				row.Index.WildcardEdges[0] = row.Index.NameToEdge[anyKey(t, row.Index)]
			},
		},
		{
			name: "wildcard edge missing from wildcard list",
			mutate: func(t *testing.T, row *runtime.CompiledModelRow) {
				t.Helper()
				row.Index.WildcardEdges = row.Index.WildcardEdges[:len(row.Index.WildcardEdges)-1]
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, indexedRow(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsAmbiguousDFARow(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	model := &publishedRuntime(t, engine).CompiledModels[rootContentModel(t, engine)]
	for i := range model.Rows {
		row := &model.Rows[i]
		if row.Index.IsEnabled() || len(row.Edges) < 2 {
			continue
		}
		row.Edges[1].Particle = row.Edges[0].Particle
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		return
	}
	t.Fatal("no unindexed row with two edges")
}

func TestCompileRejectsCompiledModelDerivationDrift(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	modelID := rootContentModel(t, engine)
	model := &publishedRuntime(t, engine).CompiledModels[modelID]
	for i := range model.Rows {
		row := &model.Rows[i]
		if row.Index.IsEnabled() || len(row.Edges) < 2 {
			continue
		}
		row.Edges[0].To = row.Edges[1].To
		err := compile.ValidateCompiledModelDerivedForTest(publishedRuntime(t, engine), modelID, *model)
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		return
	}
	t.Fatal("no unindexed row with two edges")
}

func TestFreezeRejectsInconsistentSimpleVariety(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="atomicT"><xs:restriction base="xs:string"><xs:minLength value="1"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="listT"><xs:list itemType="xs:int"/></xs:simpleType>
  <xs:simpleType name="unionT"><xs:union memberTypes="xs:int xs:string"/></xs:simpleType>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	mutations := []struct {
		name     string
		typeName string
		mutate   func(rt *runtime.Schema, st *runtime.SimpleType)
	}{
		{
			name:     "atomic with union members",
			typeName: "atomicT",
			mutate: func(rt *runtime.Schema, st *runtime.SimpleType) {
				st.Union = []runtime.SimpleTypeID{rt.Builtin.String}
			},
		},
		{
			name:     "atomic with list item",
			typeName: "atomicT",
			mutate: func(rt *runtime.Schema, st *runtime.SimpleType) {
				st.ListItem = rt.Builtin.String
			},
		},
		{
			name:     "list without list item",
			typeName: "listT",
			mutate: func(rt *runtime.Schema, st *runtime.SimpleType) {
				st.ListItem = runtime.NoSimpleType
			},
		},
		{
			name:     "list with union members",
			typeName: "listT",
			mutate: func(rt *runtime.Schema, st *runtime.SimpleType) {
				st.Union = []runtime.SimpleTypeID{rt.Builtin.String}
			},
		},
		{
			name:     "union without members",
			typeName: "unionT",
			mutate: func(rt *runtime.Schema, st *runtime.SimpleType) {
				st.Union = nil
			},
		},
		{
			name:     "union with list item",
			typeName: "unionT",
			mutate: func(rt *runtime.Schema, st *runtime.SimpleType) {
				st.ListItem = rt.Builtin.String
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id, ok := publishedRuntime(t, engine).GlobalTypes[mustQName(t, publishedRuntime(t, engine), tc.typeName)].Simple()
			if !ok {
				t.Fatalf("%s is not a simple type", tc.typeName)
			}
			tc.mutate(publishedRuntime(t, engine), &publishedRuntime(t, engine).SimpleTypes[id])
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsZeroTypeID(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="CT"><xs:sequence/></xs:complexType>
  <xs:element name="root" type="CT"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema)
	}{
		{
			name: "element type",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rootID := rt.GlobalElements[mustQName(t, rt, "root")]
				rt.Elements[rootID].Type = runtime.TypeID{}
			},
		},
		{
			name: "complex type base",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				ctID, ok := rt.GlobalTypes[mustQName(t, rt, "CT")].Complex()
				if !ok {
					t.Fatal("CT is not a complex type")
				}
				rt.ComplexTypes[ctID].Base = runtime.TypeID{}
			},
		},
		{
			name: "global type",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.GlobalTypes[mustQName(t, rt, "CT")] = runtime.TypeID{}
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, publishedRuntime(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsMisclassifiedSimpleIdentity(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Ref"><xs:restriction base="xs:IDREF"/></xs:simpleType>
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:element name="root" type="Plain"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema)
	}{
		{
			name: "idref restriction loses identity",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				id, ok := rt.GlobalTypes[mustQName(t, rt, "Ref")].Simple()
				if !ok {
					t.Fatal("Ref is not a simple type")
				}
				rt.SimpleTypes[id].Identity = runtime.SimpleIdentityNone
			},
		},
		{
			name: "plain type gains identity",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				id, ok := rt.GlobalTypes[mustQName(t, rt, "Plain")].Simple()
				if !ok {
					t.Fatal("Plain is not a simple type")
				}
				rt.SimpleTypes[id].Identity = runtime.SimpleIdentityID
			},
		},
		{
			name: "builtin ID loses identity",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.SimpleTypes[rt.Builtin.ID].Identity = runtime.SimpleIdentityNone
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, publishedRuntime(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsInvalidSimpleTypeEnums(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(st *runtime.SimpleType)
	}{
		{
			name: "invalid primitive",
			mutate: func(st *runtime.SimpleType) {
				st.Primitive = runtime.PrimitiveKind(255)
			},
		},
		{
			name: "invalid whitespace",
			mutate: func(st *runtime.SimpleType) {
				st.Whitespace = runtime.WhitespaceMode(255)
			},
		},
		{
			name: "invalid builtin validation",
			mutate: func(st *runtime.SimpleType) {
				st.Builtin = runtime.BuiltinValidationKind(255)
			},
		},
		{
			name: "invalid final mask",
			mutate: func(st *runtime.SimpleType) {
				st.Final = runtime.DerivationExtension
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleTypeIDByName(t, publishedRuntime(t, engine), "Plain")
			tc.mutate(&publishedRuntime(t, engine).SimpleTypes[id])
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsSimpleTypeSemanticDrift(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	id := simpleTypeIDByName(t, publishedRuntime(t, engine), "Plain")
	publishedRuntime(t, engine).SimpleTypes[id].Primitive = runtime.PrimitiveBoolean
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsBuiltinHandleDrift(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema)
	}{
		{
			name: "simple handle points at wrong valid type",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.Builtin.String = rt.Builtin.Boolean
			},
		},
		{
			name: "simple shape drift",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.SimpleTypes[rt.Builtin.String].Whitespace = runtime.WhitespaceCollapse
			},
		},
		{
			name: "global type binding drift",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				q, ok := rt.Names.LookupQName(vocab.XSDNamespaceURI, "string")
				if !ok {
					t.Fatal("xs:string name not found")
				}
				rt.GlobalTypes[q] = runtime.SimpleRef(rt.Builtin.Boolean)
			},
		},
		{
			name: "missing builtin declaration table",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.Wildcards = nil
			},
		},
		{
			name: "builtin attribute handle drift",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				q, ok := rt.Names.LookupQName(vocab.XMLNamespaceURI, vocab.XMLAttrBase)
				if !ok {
					t.Fatal("xml:base name not found")
				}
				id, ok := rt.GlobalAttributes[q]
				if !ok {
					t.Fatal("xml:base attribute not found")
				}
				rt.Attributes[id].Type = rt.Builtin.String
			},
		},
		{
			name: "builtin attribute lexical validator drift",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				q, ok := rt.Names.LookupQName(vocab.XMLNamespaceURI, vocab.XMLAttrLang)
				if !ok {
					t.Fatal("xml:lang name not found")
				}
				id, ok := rt.GlobalAttributes[q]
				if !ok {
					t.Fatal("xml:lang attribute not found")
				}
				rt.SimpleTypes[rt.Attributes[id].Type].Builtin = runtime.BuiltinValidationNone
			},
		},
		{
			name: "anyType shape drift",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.ComplexTypes[rt.Builtin.AnyType].ContentKind = runtime.ContentElementOnly
			},
		},
		{
			name: "builtin list item drift",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.SimpleTypes[rt.Builtin.IDREFS].ListItem = rt.Builtin.String
			},
		},
		{
			name: "builtin facet drift",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				mutateBoundFacet(t, &rt.SimpleTypes[rt.Builtin.Int].Facets, runtime.FacetMaxInclusive, func(lit *runtime.CompiledLiteral) {
					lit.Canonical = "1"
				})
			},
		},
		{
			name: "anyType wildcard drift",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				attrs := rt.ComplexTypes[rt.Builtin.AnyType].Attrs
				rt.AttributeUseSets[attrs].Wildcard = runtime.NoWildcard
			},
		},
		{
			name: "non-handle builtin drift",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				q, ok := rt.Names.LookupQName(vocab.XSDNamespaceURI, "long")
				if !ok {
					t.Fatal("xs:long name not found")
				}
				id, ok := rt.GlobalTypes[q].Simple()
				if !ok {
					t.Fatal("xs:long is not simple")
				}
				rt.SimpleTypes[id].Whitespace = runtime.WhitespacePreserve
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, publishedRuntime(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsInvalidContentModelShape(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema)
	}{
		{
			name: "invalid kind",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				rt.Models[rootContentModel(t, rt)].Kind = runtime.ModelKind(255)
			},
		},
		{
			name: "invalid occurrence range",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				model := &rt.Models[rootContentModel(t, rt)]
				model.Occurs = runtime.Occurrence{Min: 2, Max: 1}
			},
		},
		{
			name: "unsorted choice limits",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				model := &rt.Models[rootContentModel(t, rt)]
				model.ChoiceLimits = []uint32{1, 0}
			},
		},
		{
			name: "unjustified choice limits",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				model := &rt.Models[rootContentModel(t, rt)]
				model.ChoiceLimits = []uint32{1}
			},
		},
		{
			name: "choice limit on non-sequence",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				model := &rt.Models[rootContentModel(t, rt)]
				model.Kind = runtime.ModelChoice
				model.ChoiceLimits = []uint32{1}
			},
		},
		{
			name: "any model inactive state",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				model := &rt.Models[rt.ComplexTypes[rt.Builtin.AnyType].Content]
				model.Occurs = runtime.Occurrence{Min: 1, Max: 1}
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, publishedRuntime(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsSimpleFacetLoosening(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		mutate func(*runtime.FacetSet)
	}{
		{
			name: "length",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:length value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.Length = 3
			},
		},
		{
			name: "minLength",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:minLength value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.MinLength = 1
			},
		},
		{
			name: "maxLength",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:maxLength value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.MaxLength = 3
			},
		},
		{
			name: "totalDigits",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:decimal"><xs:totalDigits value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.TotalDigits = 3
			},
		},
		{
			name: "fractionDigits",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:decimal"><xs:fractionDigits value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.FractionDigits = 3
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, tt.schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleTypeIDByName(t, publishedRuntime(t, engine), "Derived")
			tt.mutate(&publishedRuntime(t, engine).SimpleTypes[id].Facets)
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsFixedFacetMutation(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		mutate func(*runtime.Schema, *runtime.SimpleType)
	}{
		{
			name: "fixed maxLength",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:maxLength value="5" fixed="true"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"><xs:maxLength value="5"/></xs:restriction></xs:simpleType>
</xs:schema>`,
			mutate: func(_ *runtime.Schema, st *runtime.SimpleType) {
				st.Facets.MaxLength = 4
			},
		},
		{
			name: "fixed whiteSpace",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:whiteSpace value="replace" fixed="true"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"><xs:whiteSpace value="replace"/></xs:restriction></xs:simpleType>
</xs:schema>`,
			mutate: func(_ *runtime.Schema, st *runtime.SimpleType) {
				st.Whitespace = runtime.WhitespaceCollapse
			},
		},
		{
			name: "fixed ordered literal",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:decimal"><xs:minInclusive value="5" fixed="true"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Other"><xs:restriction base="xs:decimal"><xs:minInclusive value="6"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"><xs:minInclusive value="5.0"/></xs:restriction></xs:simpleType>
</xs:schema>`,
			mutate: func(rt *runtime.Schema, st *runtime.SimpleType) {
				id := simpleTypeIDByName(t, rt, "Other")
				lit, ok := runtime.BoundFacet(rt.SimpleTypes[id].Facets, runtime.FacetMinInclusive)
				if !ok {
					t.Fatal("Other minInclusive facet is missing")
				}
				runtime.SetBoundFacet(&st.Facets, runtime.FacetMinInclusive, lit, false)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, tt.schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleTypeIDByName(t, publishedRuntime(t, engine), "Derived")
			tt.mutate(publishedRuntime(t, engine), &publishedRuntime(t, engine).SimpleTypes[id])
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsInheritedFacetLoss(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		mutate func(*runtime.FacetSet)
	}{
		{
			name: "totalDigits",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:decimal"><xs:totalDigits value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				f.TotalDigits = 0
				f.Present &^= runtime.FacetTotalDigits
			},
		},
		{
			name: "date minInclusive",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:date"><xs:minInclusive value="2020-01-01"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"/></xs:simpleType>
</xs:schema>`,
			mutate: func(f *runtime.FacetSet) {
				mutateBoundFacet(t, f, runtime.FacetMinInclusive, func(lit *runtime.CompiledLiteral) {
					*lit = runtime.CompiledLiteral{}
				})
				f.Present &^= runtime.FacetMinInclusive
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, tt.schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleTypeIDByName(t, publishedRuntime(t, engine), "Derived")
			tt.mutate(&publishedRuntime(t, engine).SimpleTypes[id].Facets)
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsOrderedFacetLoosening(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base"><xs:restriction base="xs:date"><xs:minInclusive value="2020-01-01"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Earlier"><xs:restriction base="xs:date"><xs:minInclusive value="2019-01-01"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Base"><xs:minInclusive value="2021-01-01"/></xs:restriction></xs:simpleType>
</xs:schema>`)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	derivedID := simpleTypeIDByName(t, publishedRuntime(t, engine), "Derived")
	earlierID := simpleTypeIDByName(t, publishedRuntime(t, engine), "Earlier")
	lit, ok := runtime.BoundFacet(publishedRuntime(t, engine).SimpleTypes[earlierID].Facets, runtime.FacetMinInclusive)
	if !ok {
		t.Fatal("Earlier minInclusive facet is missing")
	}
	runtime.SetBoundFacet(&publishedRuntime(t, engine).SimpleTypes[derivedID].Facets, runtime.FacetMinInclusive, lit, false)
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsLengthFacetInconsistency(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*runtime.FacetSet)
	}{
		{
			name: "length differs from minLength",
			mutate: func(f *runtime.FacetSet) {
				f.MinLength = 1
			},
		},
		{
			name: "length differs from maxLength",
			mutate: func(f *runtime.FacetSet) {
				f.MaxLength = 3
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Sized">
    <xs:restriction base="xs:string">
      <xs:length value="2"/>
      <xs:minLength value="2"/>
      <xs:maxLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleTypeIDByName(t, publishedRuntime(t, engine), "Sized")
			tt.mutate(&publishedRuntime(t, engine).SimpleTypes[id].Facets)
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsSimpleTypeGraphInvalidity(t *testing.T) {
	t.Run("base cycle", func(t *testing.T) {
		engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="A"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:simpleType name="B"><xs:restriction base="A"/></xs:simpleType>
</xs:schema>`)
		if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		a := simpleTypeIDByName(t, publishedRuntime(t, engine), "A")
		b := simpleTypeIDByName(t, publishedRuntime(t, engine), "B")
		publishedRuntime(t, engine).SimpleTypes[a].Base = b
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("list item is list variety", func(t *testing.T) {
		engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Words"><xs:list itemType="xs:string"/></xs:simpleType>
  <xs:simpleType name="Bad"><xs:list itemType="xs:string"/></xs:simpleType>
</xs:schema>`)
		if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		words := simpleTypeIDByName(t, publishedRuntime(t, engine), "Words")
		bad := simpleTypeIDByName(t, publishedRuntime(t, engine), "Bad")
		publishedRuntime(t, engine).SimpleTypes[bad].ListItem = words
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
}

func TestFreezeRejectsLimitedContentModelSharedByNonRestriction(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:choice maxOccurs="unbounded">
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:sequence><xs:element name="a" type="xs:string" maxOccurs="unbounded"/></xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="Other">
    <xs:sequence><xs:element name="other" type="xs:string"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	limited := runtime.NoContentModel
	for id, model := range publishedRuntime(t, engine).Models {
		if len(model.ChoiceLimits) != 0 {
			limited = runtime.ContentModelID(id)
			break
		}
	}
	if limited == runtime.NoContentModel {
		t.Fatal("no limited content model found")
	}
	otherID, ok := publishedRuntime(t, engine).GlobalTypes[mustQName(t, publishedRuntime(t, engine), "Other")].Complex()
	if !ok {
		t.Fatal("Other is not complex")
	}
	publishedRuntime(t, engine).ComplexTypes[otherID].Content = limited
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsComplexExtensionDroppingOptionalBaseParticle(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence><xs:element name="a" type="xs:string" minOccurs="0"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:complexType name="OnlyB">
    <xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence>
  </xs:complexType>
</xs:schema>`)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	derived := complexTypeIDByName(t, publishedRuntime(t, engine), "Derived")
	onlyB := complexTypeIDByName(t, publishedRuntime(t, engine), "OnlyB")
	publishedRuntime(t, engine).ComplexTypes[derived].Content = publishedRuntime(t, engine).ComplexTypes[onlyB].Content
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsComplexExtensionWrapperOccurrenceDrift(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent>
  </xs:complexType>
</xs:schema>`)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	derived := complexTypeIDByName(t, publishedRuntime(t, engine), "Derived")
	publishedRuntime(t, engine).Models[publishedRuntime(t, engine).ComplexTypes[derived].Content].Occurs.Min = 0
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsInvalidComplexTypeShape(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element name="child" type="xs:string"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema, ct *runtime.ComplexType)
	}{
		{
			name: "missing content",
			mutate: func(t *testing.T, rt *runtime.Schema, ct *runtime.ComplexType) {
				t.Helper()
				ct.Content = runtime.NoContentModel
			},
		},
		{
			name: "missing attrs",
			mutate: func(t *testing.T, rt *runtime.Schema, ct *runtime.ComplexType) {
				t.Helper()
				ct.Attrs = runtime.NoAttributeUseSet
			},
		},
		{
			name: "invalid content kind",
			mutate: func(t *testing.T, rt *runtime.Schema, ct *runtime.ComplexType) {
				t.Helper()
				ct.ContentKind = runtime.ContentKind(255)
			},
		},
		{
			name: "invalid derivation",
			mutate: func(t *testing.T, rt *runtime.Schema, ct *runtime.ComplexType) {
				t.Helper()
				ct.Derivation = runtime.DerivationKind(255)
			},
		},
		{
			name: "invalid block mask",
			mutate: func(t *testing.T, rt *runtime.Schema, ct *runtime.ComplexType) {
				t.Helper()
				ct.Block = runtime.DerivationSubstitution
			},
		},
		{
			name: "invalid final mask",
			mutate: func(t *testing.T, rt *runtime.Schema, ct *runtime.ComplexType) {
				t.Helper()
				ct.Final = runtime.DerivationSubstitution
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			root := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
			ctID, ok := publishedRuntime(t, engine).Elements[root].Type.Complex()
			if !ok {
				t.Fatal("root type is not complex")
			}
			tc.mutate(t, publishedRuntime(t, engine), &publishedRuntime(t, engine).ComplexTypes[ctID])
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsAttributeWildcardBaseMismatch(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:anyAttribute namespace="##other" processContents="lax"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base"><xs:anyAttribute namespace="##local" processContents="lax"/></xs:extension></xs:complexContent>
  </xs:complexType>
</xs:schema>`)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	derived := complexTypeIDByName(t, publishedRuntime(t, engine), "Derived")
	set := &publishedRuntime(t, engine).AttributeUseSets[publishedRuntime(t, engine).ComplexTypes[derived].Attrs]
	if set.WildcardDeclared == runtime.NoWildcard {
		t.Fatal("derived attribute wildcard did not record declared wildcard")
	}
	set.WildcardBase = runtime.NoWildcard
	set.Wildcard = set.WildcardDeclared
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsInvalidDerivationSourceBeforeReplay(t *testing.T) {
	t.Run("invalid base wildcard", func(t *testing.T) {
		engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:anyAttribute namespace="##other" processContents="skip"/></xs:complexType>
  <xs:complexType name="Derived"><xs:complexContent><xs:restriction base="Base"/></xs:complexContent></xs:complexType>
</xs:schema>`)
		if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		base := complexTypeIDByName(t, publishedRuntime(t, engine), "Base")
		set := &publishedRuntime(t, engine).AttributeUseSets[publishedRuntime(t, engine).ComplexTypes[base].Attrs]
		bad := runtime.WildcardID(1 << 30)
		set.Wildcard = bad
		set.WildcardDeclared = bad
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
	t.Run("invalid model particle", func(t *testing.T) {
		engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:restriction base="Base"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:restriction></xs:complexContent>
  </xs:complexType>
</xs:schema>`)
		if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
			t.Fatalf("ValidateSchema() before mutation error = %v", err)
		}
		base := complexTypeIDByName(t, publishedRuntime(t, engine), "Base")
		model := &publishedRuntime(t, engine).Models[publishedRuntime(t, engine).ComplexTypes[base].Content]
		model.Particles[0] = runtime.ModelParticle(runtime.ContentModelID(1<<30), runtime.Occurrence{Min: 1, Max: 1})
		err := runtime.ValidateSchema(publishedRuntime(t, engine))
		expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	})
}

func TestFreezeRejectsInvalidElementMasks(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(decl *runtime.ElementDecl)
	}{
		{
			name: "invalid block",
			mutate: func(decl *runtime.ElementDecl) {
				decl.Block = runtime.DerivationList
			},
		},
		{
			name: "invalid final",
			mutate: func(decl *runtime.ElementDecl) {
				decl.Final = runtime.DerivationSubstitution
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			root := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
			tc.mutate(&publishedRuntime(t, engine).Elements[root])
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRuntimeConsumesCompilerRuntime(t *testing.T) {
	c, rt := frozenCompilerRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
      <xs:attribute name="code" type="Code" use="required" fixed="US"/>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath="child"/>
      <xs:field xpath="."/>
    </xs:key>
  </xs:element>
</xs:schema>`)
	engine := rt
	mustValidateRuntime(t, engine, `<r code="US"><child>x</child></r>`)

	if !reflect.ValueOf(*c.RuntimeForTest()).IsZero() {
		t.Fatal("freezeRuntime did not clear compile.Compiler runtime")
	}
	if err := runtime.ValidateSchema(rt); err != nil {
		t.Fatalf("published runtime after compile.Compiler consume = %v", err)
	}
	mustValidateRuntime(t, engine, `<r code="US"><child>x</child></r>`)
}

func TestFreezeRuntimeClearsCompilerMutationAliases(t *testing.T) {
	c := compiledCompilerRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
      <xs:attribute name="code" type="Code" use="required" fixed="US"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	frozen, err := compile.FreezeCompilerRuntimeForTest(c)
	if err != nil {
		t.Fatalf("compile.FreezeCompilerRuntimeForTest() error = %v", err)
	}
	rt := validationRuntimeForTest(t, frozen)
	engine := rt

	if !reflect.ValueOf(*c.RuntimeForTest()).IsZero() {
		t.Fatal("freezeRuntime did not clear compile.Compiler runtime")
	}
	if !c.NameInternerIsZeroForTest() {
		t.Fatal("freezeRuntime did not clear compile.Compiler name interner")
	}
	if err := runtime.ValidateSchema(rt); err != nil {
		t.Fatalf("published runtime after compile.Compiler consume = %v", err)
	}
	mustValidateRuntime(t, engine, `<r code="US"><child>x</child></r>`)
}

func TestFreezeRuntimeKeepsCompilerStateOnValidationFailure(t *testing.T) {
	c := compiledCompilerRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:element name="r" type="xs:string"/>
</xs:schema>`)
	rootName := mustQName(t, c.RuntimeForTest(), "r")
	c.RuntimeForTest().GlobalElements[rootName] = runtime.NoElement

	_, err := compile.FreezeCompilerRuntimeForTest(c)
	if err == nil {
		t.Fatal("compile.FreezeCompilerRuntimeForTest() error = nil, want validation error")
	}
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if reflect.ValueOf(*c.RuntimeForTest()).IsZero() {
		t.Fatal("FreezeCompilerRuntimeForTest cleared compile.Compiler runtime after validation failure")
	}
	if c.NameInternerIsZeroForTest() {
		t.Fatal("FreezeCompilerRuntimeForTest cleared compile.Compiler name interner after validation failure")
	}
}

func frozenCompilerRuntime(t *testing.T, schema string) (*compile.Compiler, *runtime.Schema) {
	t.Helper()
	c := compiledCompilerRuntime(t, schema)
	frozen, err := compile.FreezeCompilerRuntimeForTest(c)
	if err != nil {
		t.Fatalf("compile.FreezeCompilerRuntimeForTest() error = %v", err)
	}
	return c, validationRuntimeForTest(t, frozen)
}

func validationRuntimeForTest(tb testing.TB, rt *runtime.Schema) *runtime.Schema {
	tb.Helper()
	if rt != nil {
		if err := rt.EnsurePublished(); err != nil {
			tb.Fatalf("publish runtime: %v", err)
		}
		return rt
	}
	tb.Fatal("frozen runtime view is nil")
	return nil
}

func compiledCompilerRuntime(t *testing.T, schema string) *compile.Compiler {
	t.Helper()
	limits, err := compile.NormalizeOptions(compile.Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := compile.NewCompilerForTest(limits)
	if err != nil {
		t.Fatal(err)
	}
	err = c.LoadForTest([]source.Source{source.Bytes("schema.xsd", []byte(schema))})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	err = c.IndexForTest()
	if err != nil {
		t.Fatalf("index() error = %v", err)
	}
	err = c.CompileGlobalsForTest()
	if err != nil {
		t.Fatalf("compileGlobals() error = %v", err)
	}
	return c
}

func TestCompiledSimpleFastPathDerivedFromFacets(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="MyInt"><xs:restriction base="xs:int"/></xs:simpleType>
  <xs:simpleType name="MyShort"><xs:restriction base="xs:short"/></xs:simpleType>
  <xs:simpleType name="TightInt"><xs:restriction base="xs:int"><xs:maxInclusive value="10"/></xs:restriction></xs:simpleType>
</xs:schema>`)

	if got := publishedRuntime(t, engine).SimpleTypes[publishedRuntime(t, engine).Builtin.Int].Fast; got != runtime.SimpleFastInt {
		t.Fatalf("xs:int Fast = %v, want runtime.SimpleFastInt", got)
	}
	if got := simpleTypeByName(t, publishedRuntime(t, engine), "MyInt").Fast; got != runtime.SimpleFastInt {
		t.Fatalf("MyInt Fast = %v, want runtime.SimpleFastInt", got)
	}
	if got := simpleTypeByName(t, publishedRuntime(t, engine), "MyShort").Fast; got != runtime.SimpleFastNone {
		t.Fatalf("MyShort Fast = %v, want runtime.SimpleFastNone", got)
	}
	if got := simpleTypeByName(t, publishedRuntime(t, engine), "TightInt").Fast; got != runtime.SimpleFastNone {
		t.Fatalf("TightInt Fast = %v, want runtime.SimpleFastNone", got)
	}
}

func TestSimpleContentFacetRestrictionRecomputesFastPath(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:int"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root">
    <xs:complexType>
      <xs:simpleContent>
        <xs:restriction base="Base">
          <xs:maxInclusive value="10"/>
        </xs:restriction>
      </xs:simpleContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	rootID := publishedRuntime(t, engine).GlobalElements[mustQName(t, publishedRuntime(t, engine), "root")]
	ctID, ok := publishedRuntime(t, engine).Elements[rootID].Type.Complex()
	if !ok {
		t.Fatal("root type is not complex")
	}
	textType := publishedRuntime(t, engine).ComplexTypes[ctID].TextType
	if got := publishedRuntime(t, engine).SimpleTypes[textType].Fast; got != runtime.SimpleFastNone {
		t.Fatalf("simple content text type Fast = %v, want runtime.SimpleFastNone", got)
	}
	mustValidateRuntime(t, engine, `<root>10</root>`)
	mustNotValidateRuntime(t, engine, `<root>11</root>`, xsderrors.CodeValidationFacet)
}

func TestSimpleContentRestrictionAllowsEmptiableMixedBaseWithInlineType(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Mixed" mixed="true"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:simpleContent>
        <xs:restriction base="Mixed">
          <xs:simpleType>
            <xs:restriction base="xs:string"/>
          </xs:simpleType>
        </xs:restriction>
      </xs:simpleContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	mustValidateRuntime(t, engine, `<root>value</root>`)
}

func TestFreezeRejectsMisclassifiedSimpleFastPath(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="MyInt"><xs:restriction base="xs:int"/></xs:simpleType>
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`)

	id := simpleTypeIDByName(t, publishedRuntime(t, engine), "MyInt")
	publishedRuntime(t, engine).SimpleTypes[id].Fast = runtime.SimpleFastNone
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)

	engine = mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Plain"><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`)
	id = simpleTypeIDByName(t, publishedRuntime(t, engine), "Plain")
	publishedRuntime(t, engine).SimpleTypes[id].Fast = runtime.SimpleFastInt
	err = runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFreezeRejectsInvalidPatternFacetRepresentation(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Patterned">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Patterned"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(f *runtime.FacetSet)
	}{
		{
			name: "empty pattern group",
			mutate: func(f *runtime.FacetSet) {
				f.Patterns[0].Patterns = nil
			},
		},
		{
			name: "pattern without matcher",
			mutate: func(f *runtime.FacetSet) {
				f.Patterns[0].Patterns[0] = runtime.StringPattern{}
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			id := simpleTypeIDByName(t, publishedRuntime(t, engine), "Patterned")
			tc.mutate(&publishedRuntime(t, engine).SimpleTypes[id].Facets)
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsCompiledModelSourceMismatch(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:sequence><xs:element name="child" type="xs:string"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(model *runtime.CompiledModel)
	}{
		{
			name: "source id drift",
			mutate: func(model *runtime.CompiledModel) {
				model.Source = runtime.NoContentModel
			},
		},
		{
			name: "kind drift",
			mutate: func(model *runtime.CompiledModel) {
				model.Kind = runtime.CompiledModelEmpty
				model.Rows = nil
				model.Empty = true
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			modelID := rootContentModel(t, engine)
			tc.mutate(&publishedRuntime(t, engine).CompiledModels[modelID])
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsParticleWithStaleInactiveFields(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence><xs:element name="child" type="xs:string"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(t *testing.T, rt *runtime.Schema)
	}{
		{
			name: "model particle",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				for i := range rt.Models {
					for j := range rt.Models[i].Particles {
						p := &rt.Models[i].Particles[j]
						if p.Kind == runtime.ParticleElement {
							p.Wildcard = 0
							return
						}
					}
				}
				t.Fatal("no element particle found")
			},
		},
		{
			name: "compiled edge particle",
			mutate: func(t *testing.T, rt *runtime.Schema) {
				t.Helper()
				for i := range rt.CompiledModels {
					for j := range rt.CompiledModels[i].Rows {
						row := &rt.CompiledModels[i].Rows[j]
						for k := range row.Edges {
							if row.Edges[k].Particle.Kind == runtime.ParticleElement {
								row.Edges[k].Particle.Model = 0
								return
							}
						}
					}
				}
				t.Fatal("no compiled element edge found")
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, publishedRuntime(t, engine))
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func mustQName(t *testing.T, rt *runtime.Schema, local string) runtime.QName {
	t.Helper()
	q, ok := rt.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("LookupQName(%q) failed", local)
	}
	return q
}

func mutateBoundFacet(t *testing.T, f *runtime.FacetSet, flag runtime.FacetMask, mutate func(*runtime.CompiledLiteral)) {
	t.Helper()
	lit, ok := runtime.BoundFacet(*f, flag)
	if !ok {
		t.Fatalf("bound facet %d is missing", flag)
	}
	mutate(&lit)
	runtime.SetBoundFacet(f, flag, lit, false)
}

func complexTypeIDByName(t *testing.T, rt *runtime.Schema, local string) runtime.ComplexTypeID {
	t.Helper()
	typ, ok := rt.GlobalTypes[mustQName(t, rt, local)]
	if !ok {
		t.Fatalf("global type %q not found", local)
	}
	id, ok := typ.Complex()
	if !ok {
		t.Fatalf("global type %q is not complex", local)
	}
	return id
}

func simpleTypeIDByName(t *testing.T, rt *runtime.Schema, local string) runtime.SimpleTypeID {
	t.Helper()
	typ, ok := rt.GlobalTypes[mustQName(t, rt, local)]
	if !ok {
		t.Fatalf("global type %q not found", local)
	}
	id, ok := typ.Simple()
	if !ok {
		t.Fatalf("global type %q is not simple", local)
	}
	return id
}

func simpleTypeByName(t *testing.T, rt *runtime.Schema, local string) runtime.SimpleType {
	t.Helper()
	return rt.SimpleTypes[simpleTypeIDByName(t, rt, local)]
}

func runtimeValueConstraint(t *testing.T, rt *runtime.Schema, id runtime.SimpleTypeID, lexical string) *runtime.ValueConstraint {
	t.Helper()
	value, err := rt.ValidateSimpleValue(id, lexical, nil, runtime.SimpleNeedCanonical|runtime.SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("validateSimpleValueRuntimeBoundary(%q) error = %v", lexical, err)
	}
	return &runtime.ValueConstraint{
		Lexical:   lexical,
		Canonical: value.Canonical,
		Value:     value,
	}
}

func TestFreezeRejectsFacetPresenceMismatch(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Sized">
    <xs:restriction base="xs:string">
      <xs:maxLength value="4"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Sized"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(f *runtime.FacetSet)
	}{
		{
			name: "bit without value",
			mutate: func(f *runtime.FacetSet) {
				f.Present |= runtime.FacetLength
			},
		},
		{
			name: "value without bit",
			mutate: func(f *runtime.FacetSet) {
				f.Present &^= runtime.FacetMaxLength
			},
		},
		{
			name: "whiteSpace bit in presence mask",
			mutate: func(f *runtime.FacetSet) {
				f.Present |= runtime.FacetWhiteSpace
			},
		},
		{
			name: "fixed facet without presence",
			mutate: func(f *runtime.FacetSet) {
				f.Fixed |= runtime.FacetMinInclusive
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			typ := publishedRuntime(t, engine).GlobalTypes[mustQName(t, publishedRuntime(t, engine), "Sized")]
			id, ok := typ.Simple()
			if !ok {
				t.Fatal("Sized is not a simple type")
			}
			tc.mutate(&publishedRuntime(t, engine).SimpleTypes[id].Facets)
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestFreezeRejectsDecimalBoundWithoutActual(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Positive">
    <xs:restriction base="xs:int">
      <xs:minInclusive value="1"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Positive"/>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() before mutation error = %v", err)
	}
	typ := publishedRuntime(t, engine).GlobalTypes[mustQName(t, publishedRuntime(t, engine), "Positive")]
	id, ok := typ.Simple()
	if !ok {
		t.Fatal("Positive is not a simple type")
	}
	mutateBoundFacet(t, &publishedRuntime(t, engine).SimpleTypes[id].Facets, runtime.FacetMinInclusive, func(lit *runtime.CompiledLiteral) {
		lit.Actual = runtime.PrimitiveActualValue{}
	})
	err := runtime.ValidateSchema(publishedRuntime(t, engine))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestFixedWhitespaceFacetFreezes(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Collapsed">
    <xs:restriction base="xs:string">
      <xs:whiteSpace value="collapse" fixed="true"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="Collapsed"/>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
		t.Fatalf("ValidateSchema() error = %v", err)
	}
}

func TestMixedSimpleContentExtensionChain(t *testing.T) {
	const mixedBase = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A" mixed="true">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="B">
    <xs:complexContent mixed="true"><xs:extension base="A"/></xs:complexContent>
  </xs:complexType>
  <xs:complexType name="C">
    <xs:complexContent mixed="true"><xs:extension base="B"/></xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="C"/>
</xs:schema>`
	mustCompileRuntime(t, mixedBase)

	const nonMixedBase = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="B">
    <xs:complexContent mixed="true"><xs:extension base="A"/></xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="B"/>
</xs:schema>`
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(nonMixedBase))})
	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestFreezeRejectsInconsistentComplexContent(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="S">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="E">
    <xs:sequence><xs:element name="child"/></xs:sequence>
  </xs:complexType>
  <xs:element name="s" type="S"/>
  <xs:element name="e" type="E"/>
</xs:schema>`
	complexID := func(t *testing.T, engine *runtime.Schema, local string) runtime.ComplexTypeID {
		t.Helper()
		typ := publishedRuntime(t, engine).GlobalTypes[mustQName(t, publishedRuntime(t, engine), local)]
		id, ok := typ.Complex()
		if !ok {
			t.Fatalf("%s is not a complex type", local)
		}
		return id
	}
	mutations := []struct {
		name   string
		mutate func(t *testing.T, engine *runtime.Schema)
	}{
		{
			name: "text type without simple content",
			mutate: func(t *testing.T, engine *runtime.Schema) {
				t.Helper()
				publishedRuntime(t, engine).ComplexTypes[complexID(t, engine, "E")].TextType = publishedRuntime(t, engine).Builtin.String
			},
		},
		{
			name: "simple content with particles",
			mutate: func(t *testing.T, engine *runtime.Schema) {
				t.Helper()
				elementOnly := publishedRuntime(t, engine).ComplexTypes[complexID(t, engine, "E")]
				publishedRuntime(t, engine).ComplexTypes[complexID(t, engine, "S")].Content = elementOnly.Content
			},
		},
		{
			name: "simple content with invalid text type",
			mutate: func(t *testing.T, engine *runtime.Schema) {
				t.Helper()
				publishedRuntime(t, engine).ComplexTypes[complexID(t, engine, "S")].TextType = runtime.SimpleTypeID(1 << 30)
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompileRuntime(t, schema)
			if err := runtime.ValidateSchema(publishedRuntime(t, engine)); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			tc.mutate(t, engine)
			err := runtime.ValidateSchema(publishedRuntime(t, engine))
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestRuntimeElementAccessor(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`)
	rt := publishedRuntime(t, engine)
	if _, ok := rt.ElementDecl(runtime.NoElement); ok {
		t.Error("element(noElement) resolved, want miss")
	}
	if _, ok := rt.ElementDecl(runtime.ElementID(1 << 30)); ok {
		t.Error("element(out of range) resolved, want miss")
	}
	rootID := rt.GlobalElements[mustQName(t, rt, "root")]
	decl, ok := rt.ElementDecl(rootID)
	if !ok || decl.Name != mustQName(t, rt, "root") {
		t.Errorf("element(root) = (%v, %v), want root declaration", decl, ok)
	}
}

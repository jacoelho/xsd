package xsd

import (
	"bytes"
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

func TestSchemaCompileErrorsIncludeLocation(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		needle string
		code   ErrorCode
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
			code:   ErrSchemaFacet,
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
			code:   ErrSchemaIdentity,
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
			code:   ErrSchemaContentModel,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(sourceBytes("schema.xsd", []byte(test.schema)))
			expectCode(t, err, test.code)
			expectSchemaCompileLine(t, err, lineOf(test.schema, test.needle))
		})
	}
}

func expectSchemaCompileLine(t *testing.T, err error, line int) {
	t.Helper()
	x, ok := errors.AsType[*Error](err)
	if !ok {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if x.Category != SchemaCompileErrorCategory {
		t.Fatalf("error category = %s, want %s", x.Category, SchemaCompileErrorCategory)
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
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:attribute name="a"/>
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad">
    <xs:attribute name="a"/>
    <xs:complexType/>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:attribute name="a"/>
    <xs:annotation/>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="bad"><xs:complexType name="localName"/></xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad" block="substitution"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent mixed="true"><xs:extension base="base"/></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"/>
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="base"/><xs:annotation/></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"/>
  <xs:complexType name="bad"><xs:complexContent><xs:restriction base="base"><xs:sequence/><xs:choice/></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:sequence><xs:element name="a"/></xs:sequence></xs:complexType>
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="base"><xs:all><xs:element name="b"/></xs:all></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestInvalidAnnotationStructureIsSchemaError(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation><xs:annotation/></xs:annotation>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:annotation/>
    <xs:annotation/>
  </xs:element>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation foo="bar"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation><xs:documentation xml:lang=" "/></xs:annotation>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="g">
    <xs:attribute name="a"/>
    <xs:annotation/>
  </xs:attributeGroup>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestComplexContentCannotDeriveFromItself(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad"><xs:complexContent><xs:extension base="bad"><xs:sequence><xs:element name="child"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestSimpleTypeCannotRestrictAnySimpleType(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="bad"><xs:restriction base="xs:anySimpleType"/></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestSimpleAndComplexTypesShareNames(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="dup"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="dup"/>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaDuplicate)
}

func TestMissingElementTypeInvalidatesOnlyThatElement(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="good" type="xs:int"/>
  <xs:element name="bad" type="absent"/>
</xs:schema>`)
	mustValidate(t, engine, `<good>1</good>`)
	mustNotValidate(t, engine, `<bad>1</bad>`, ErrValidationFacet)
}

func TestElementDeclarationsMustBeConsistent(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="bad">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="a" type="xs:int"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestExtendedElementDeclarationsMustBeConsistent(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
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
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestTypeFinalBlocksDerivation(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:complexType name="Base" final="extension"><xs:sequence><xs:element name="a" type="xs:string"/></xs:sequence></xs:complexType>
  <xs:complexType name="Derived"><xs:complexContent><xs:extension base="tns:Base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Base" final="restriction"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="tns:Base"><xs:minLength value="1"/></xs:restriction></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
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
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestAnonymousSimpleTypeCannotHaveName(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="parent"><xs:restriction><xs:simpleType name="child"><xs:restriction base="xs:string"/></xs:simpleType></xs:restriction></xs:simpleType></xs:schema>`)))
	expectCode(t, err, ErrSchemaInvalidAttribute)
}

func TestSimpleDerivationAnnotationMustPrecedeContent(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="t">
    <xs:list>
      <xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>
      <xs:annotation/>
    </xs:list>
  </xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="t">
    <xs:union>
      <xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>
      <xs:annotation/>
    </xs:union>
  </xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestTopLevelSimpleTypeRequiresName(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestRestrictionElementPropertiesCannotBeLoosened(t *testing.T) {
	tests := []string{
		`<xs:complexType name="base"><xs:choice><xs:element name="e1" fixed="foo" type="xs:string"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:element name="e1" fixed="bar" type="xs:string"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
		`<xs:complexType name="base"><xs:choice><xs:element name="e1" block="extension restriction"/></xs:choice></xs:complexType>
		 <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:choice><xs:element name="e1" block="extension substitution"/></xs:choice></xs:restriction></xs:complexContent></xs:complexType>`,
	}
	for _, body := range tests {
		_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+body+`</xs:schema>`)))
		expectCode(t, err, ErrSchemaContentModel)
	}
}

func TestRestrictionElementTypeCannotUseExtension(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
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
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestRestrictionElementCanUseSubstitutionMember(t *testing.T) {
	mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="c" substitutionGroup="d" type="xs:anyType"/>
  <xs:element name="d" type="xs:anyType"/>
  <xs:complexType name="base"><xs:sequence><xs:element ref="d"/></xs:sequence></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element ref="c"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
</xs:schema>`)
}

func TestSubstitutionMemberInheritsHeadType(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:int"/>
  <xs:element name="member" substitutionGroup="head"/>
</xs:schema>`)
	mustValidate(t, engine, `<member>1</member>`)
	mustNotValidate(t, engine, `<member>x</member>`, ErrValidationFacet)
}

func TestSubstitutionMemberWithMissingHeadUsesDefaultType(t *testing.T) {
	engine := mustCompile(t, `
		<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		  <xs:element name="member" substitutionGroup="missing"/>
		</xs:schema>`)
	mustValidate(t, engine, `<member>anything</member>`)
}

func TestContentModelSubstitutionRespectsElementBlock(t *testing.T) {
	engine := mustCompile(t, `
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
	mustValidate(t, engine, `<root><member>1</member></root>`)
	mustNotValidate(t, engine, `<root><blockedMember>1</blockedMember></root>`, ErrValidationElement)
}

func TestAnonymousLocalTypeCanRestrictContainingType(t *testing.T) {
	engine := mustCompile(t, `
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
	mustValidate(t, engine, `<t:root xmlns:t="urn:test"><foo/><bar><foo/></bar></t:root>`)
}

func TestNamedComplexTypeCannotDeriveFromItself(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="self">
    <xs:complexContent><xs:extension base="self"/></xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	expectCode(t, err, ErrSchemaReference)
}

func TestComplexContentExtensionCannotDropMixedBase(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base" mixed="true">
    <xs:sequence><xs:element name="a"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:extension base="base"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}

	_, err = Compile(sourceBytes("schema.xsd", []byte(`
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
</xs:schema>`)))
	expectCode(t, err, ErrSchemaContentModel)
}

func TestRecursiveComplexTypeThroughElementRefCompiles(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="node"/>
  <xs:element name="child" type="node"/>
  <xs:complexType name="node">
    <xs:choice maxOccurs="unbounded">
      <xs:element ref="child" minOccurs="0"/>
    </xs:choice>
  </xs:complexType>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
}

func TestUnsupportedFeaturesAreExplicit(t *testing.T) {
	_, err := Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:redefine schemaLocation="a.xsd"/></xs:schema>`)))
	expectCode(t, err, ErrUnsupportedRedefine)

	_, err = Compile(sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"><xs:complexType><xs:anyAttribute notQName="##defined"/></xs:complexType></xs:element></xs:schema>`)))
	expectCode(t, err, ErrUnsupportedXSD11)
}

func TestCompileOptionsSchemaXMLLimits(t *testing.T) {
	deepSchema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:documentation>ok</xs:documentation></xs:annotation></xs:schema>`
	_, err := CompileWithOptions(CompileOptions{MaxSchemaDepth: 2}, sourceBytes("schema.xsd", []byte(deepSchema)))
	expectCategoryCode(t, err, SchemaParseErrorCategory, ErrSchemaLimit)
	if _, boundaryErr := CompileWithOptions(CompileOptions{MaxSchemaDepth: 3}, sourceBytes("schema.xsd", []byte(deepSchema))); boundaryErr != nil {
		t.Fatalf("CompileWithOptions() depth boundary error = %v", boundaryErr)
	}

	attrSchema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"><xs:element name="root"/></xs:schema>`
	_, err = CompileWithOptions(CompileOptions{MaxSchemaAttributes: 1}, sourceBytes("schema.xsd", []byte(attrSchema)))
	expectCategoryCode(t, err, SchemaParseErrorCategory, ErrSchemaLimit)
	if _, boundaryErr := CompileWithOptions(CompileOptions{MaxSchemaAttributes: 2}, sourceBytes("schema.xsd", []byte(attrSchema))); boundaryErr != nil {
		t.Fatalf("CompileWithOptions() attribute boundary error = %v", boundaryErr)
	}

	textSchema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:documentation>` + strings.Repeat("x", 129) + `</xs:documentation></xs:annotation></xs:schema>`
	_, err = CompileWithOptions(CompileOptions{MaxSchemaTokenBytes: 128}, sourceBytes("schema.xsd", []byte(textSchema)))
	expectCategoryCode(t, err, SchemaParseErrorCategory, ErrSchemaLimit)
}

func TestCompileOptionsSchemaSourceByteLimit(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	if _, err := CompileWithOptions(CompileOptions{MaxSchemaSourceBytes: int64(len(schema))}, sourceBytes("schema.xsd", []byte(schema))); err != nil {
		t.Fatalf("CompileWithOptions() source byte boundary error = %v", err)
	}

	_, err := CompileWithOptions(CompileOptions{MaxSchemaSourceBytes: int64(len(schema) - 1)}, sourceBytes("schema.xsd", []byte(schema)))
	expectCategoryCode(t, err, SchemaCompileErrorCategory, ErrSchemaLimit)
}

func TestSchemaNamespaceContextsAreIsolated(t *testing.T) {
	dec := xml.NewDecoder(bytes.NewReader([]byte(`
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
</xs:schema>`)))
	state := schemaParseState{
		dec: dec,
		nsStack: []map[string]string{{
			"xml": xmlNamespaceURI,
		}},
	}
	if err := state.parse(); err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	root := state.root
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
	source := sourceBytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`))
	tests := []CompileOptions{
		{MaxSchemaDepth: -1},
		{MaxSchemaAttributes: -1},
		{MaxSchemaTokenBytes: -1},
		{MaxSchemaSourceBytes: -1},
		{MaxSchemaNames: -1},
		{MaxContentModelStates: -1},
	}
	for _, opts := range tests {
		_, err := CompileWithOptions(opts, source)
		expectCategoryCode(t, err, SchemaCompileErrorCategory, ErrSchemaLimit)
	}
}

func TestFreezeRejectsInconsistentValueConstraints(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string" default="abc"/>
</xs:schema>`
	mutations := []struct {
		name   string
		mutate func(decl *elementDecl)
	}{
		{
			name: "canonical without constraint",
			mutate: func(decl *elementDecl) {
				decl.Default = valueConstraint{Canonical: "abc"}
			},
		},
		{
			name: "value without constraint",
			mutate: func(decl *elementDecl) {
				decl.Default = valueConstraint{Value: simpleValue{Canonical: "abc"}}
			},
		},
		{
			name: "canonical value mismatch",
			mutate: func(decl *elementDecl) {
				decl.Default.Value.Canonical = "other"
			},
		},
		{
			name: "invalid value type",
			mutate: func(decl *elementDecl) {
				decl.Default.Value.Type = simpleTypeID(1 << 30)
			},
		},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := mustCompile(t, schema)
			if err := validateRuntimeSchema(engine.rt); err != nil {
				t.Fatalf("validateRuntimeSchema() before mutation error = %v", err)
			}
			rootID := engine.rt.GlobalElements[mustQName(t, engine.rt, "", "root")]
			tc.mutate(&engine.rt.Elements[rootID])
			err := validateRuntimeSchema(engine.rt)
			expectCategoryCode(t, err, InternalErrorCategory, ErrInternalInvariant)
		})
	}
}

func mustQName(t *testing.T, rt *runtimeSchema, ns, local string) qName {
	t.Helper()
	q, err := rt.Names.InternQName(ns, local)
	if err != nil {
		t.Fatalf("InternQName(%q, %q) error = %v", ns, local, err)
	}
	return q
}

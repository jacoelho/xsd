package compile_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompileInheritedEnumerationRestrictionChain(t *testing.T) {
	t.Parallel()

	const (
		depth            = 100
		enumerationCount = 100
	)
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	schema.WriteString(`<xs:simpleType name="t0"><xs:restriction base="xs:decimal">`)
	for i := range enumerationCount {
		fmt.Fprintf(&schema, `<xs:enumeration value="%d"/>`, i)
	}
	schema.WriteString(`</xs:restriction></xs:simpleType>`)
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&schema, `<xs:simpleType name="t%d"><xs:restriction base="t%d"><xs:minInclusive value="0"/></xs:restriction></xs:simpleType>`, i, i-1)
	}
	fmt.Fprintf(&schema, `<xs:element name="root" type="t%d"/></xs:schema>`, depth)

	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(schema.String())),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	mustValidateRuntime(t, engine, `<root>0</root>`)
}

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

func TestMissingIncludedSchemaLocationDoesNotInvalidateSchema(t *testing.T) {
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		if location != "missing.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		return source.Source{}, xsderrors.ErrSchemaNotFound
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="missing.xsd"/>
</xs:schema>`)).WithResolver(resolver)})
	if err != nil {
		t.Fatalf("Compile() error = %v, want nil", err)
	}
}

func TestByteIdenticalSchemasResolveSourceRelativeIncludesIndependently(t *testing.T) {
	const main = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	resolver := source.Resolver(func(base, location string) (source.Source, error) {
		if location != "common.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		switch base {
		case "a/main.xsd":
			return source.Bytes("a/common.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a"/>
</xs:schema>`)), nil
		case "b/main.xsd":
			return source.Bytes("b/common.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="b"/>
</xs:schema>`)), nil
		default:
			return source.Source{}, errors.New("unexpected base " + base)
		}
	})
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("b/main.xsd", []byte(main)).WithResolver(resolver),
		source.Bytes("a/main.xsd", []byte(main)).WithResolver(resolver),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	mustValidateRuntime(t, engine, `<t:a xmlns:t="urn:test"/>`)
	mustValidateRuntime(t, engine, `<t:b xmlns:t="urn:test"/>`)
	mustValidateRuntime(t, engine, `<t:root xmlns:t="urn:test">ok</t:root>`)
}

func TestByteIdenticalSchemasDoNotSuppressDuplicateSourceRelativeIncludes(t *testing.T) {
	const main = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:include schemaLocation="common.xsd"/>
</xs:schema>`
	const common = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="dup"/>
</xs:schema>`
	resolver := source.Resolver(func(base, location string) (source.Source, error) {
		if location != "common.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		switch base {
		case "a/main.xsd":
			return source.Bytes("a/common.xsd", []byte(common)), nil
		case "b/main.xsd":
			return source.Bytes("b/common.xsd", []byte(common)), nil
		default:
			return source.Source{}, errors.New("unexpected base " + base)
		}
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("b/main.xsd", []byte(main)).WithResolver(resolver),
		source.Bytes("a/main.xsd", []byte(main)).WithResolver(resolver),
	})
	expectCode(t, err, xsderrors.CodeSchemaDuplicate)
	if !strings.Contains(err.Error(), "duplicate schema component") {
		t.Fatalf("error = %v, want duplicate schema component", err)
	}
}

func TestByteIdenticalSameTargetDuplicateKeepsImportGraphValidation(t *testing.T) {
	const main = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:main">
  <xs:import namespace="urn:dep" schemaLocation="dep.xsd"/>
</xs:schema>`
	resolver := source.Resolver(func(base, location string) (source.Source, error) {
		if location != "dep.xsd" {
			return source.Source{}, errors.New("unexpected location " + location)
		}
		switch base {
		case "a/main.xsd":
			return source.Bytes("a/dep.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:dep"/>`)), nil
		case "b/main.xsd":
			return source.Bytes("b/dep.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:wrong"/>`)), nil
		default:
			return source.Source{}, errors.New("unexpected base " + base)
		}
	})
	_, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("b/main.xsd", []byte(main)).WithResolver(resolver),
		source.Bytes("a/main.xsd", []byte(main)).WithResolver(resolver),
	})
	expectCode(t, err, xsderrors.CodeSchemaReference)
}

func TestByteIdenticalSameTargetSourcesCompileIdentityOnce(t *testing.T) {
	const schema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="code" type="xs:string" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemCode">
      <xs:selector xpath="item"/>
      <xs:field xpath="@code"/>
    </xs:key>
  </xs:element>
</xs:schema>`
	engine, err := compile.Compile(compile.Options{}, []source.Source{
		source.Bytes("b/schema.xsd", []byte(schema)),
		source.Bytes("a/schema.xsd", []byte(schema)),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	mustValidateRuntime(t, engine, `<t:root xmlns:t="urn:test"><item code="a"/><item code="b"/></t:root>`)
	mustNotValidateRuntime(t, engine, `<t:root xmlns:t="urn:test"><item code="a"/><item code="a"/></t:root>`, xsderrors.CodeValidationIdentity)
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

func TestAnonymousComplexDerivationWaitsForCompilingBase(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="Base"/>
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="child" minOccurs="0">
        <xs:complexType>
          <xs:complexContent>
            <xs:extension base="Base">
              <xs:sequence>
                <xs:element name="leaf" type="xs:string"/>
              </xs:sequence>
            </xs:extension>
          </xs:complexContent>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)

	mustValidateRuntime(t, engine, `<root><child><child><leaf>x</leaf></child><leaf>x</leaf></child></root>`)
}

func TestSubstitutionImplicitTypeInheritanceWaitsForCompleteHead(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="member" minOccurs="0"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:element name="member" substitutionGroup="head"/>
</xs:schema>`)

	mustValidateRuntime(t, engine, `<head><member/></head>`)
}

func TestSubstitutionInheritedTypeReplaysValueConstraint(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:int"/>
  <xs:element name="member" substitutionGroup="head" default="not-int"/>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaFacet)
}

func TestDuplicateSingleValueFacetRejectedPerRestrictionStep(t *testing.T) {
	_, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="bad">
    <xs:restriction base="xs:string">
      <xs:minLength value="1"/>
      <xs:minLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`))})

	expectCode(t, err, xsderrors.CodeSchemaFacet)
}

func TestRepeatedPatternAndEnumerationFacetsRemainLegal(t *testing.T) {
	mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="code">
    <xs:restriction base="xs:string">
      <xs:pattern value="A"/>
      <xs:pattern value="B"/>
      <xs:enumeration value="A"/>
      <xs:enumeration value="B"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="code"/>
</xs:schema>`)
}

func TestValidationComparesRawLexicalElementNames(t *testing.T) {
	engine := mustCompileRuntime(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test">
  <xs:element name="root">
    <xs:complexType/>
  </xs:element>
</xs:schema>`)

	err := validateWithRuntime(engine, `<p:root xmlns:p="urn:test" xmlns:q="urn:test"></q:root>`)
	expectCode(t, err, xsderrors.CodeValidationXML)
	if !strings.Contains(err.Error(), "end element </q:root> does not match start element <p:root>") {
		t.Fatalf("Validate() error = %v, want raw lexical mismatch", err)
	}

	err = validateWithRuntime(engine, `<p:root xmlns:p="urn:test"></q:root>`)
	expectCode(t, err, xsderrors.CodeValidationXML)
	if !strings.Contains(err.Error(), "unbound namespace prefix q") {
		t.Fatalf("Validate() error = %v, want namespace resolution error", err)
	}
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

func TestFreezeReplaysResolvedQNameValueConstraint(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:t="urn:test">
  <xs:element name="root" type="xs:QName" default="t:item"/>
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(build); err != nil {
		t.Fatalf("validateSchemaBuild() error = %v", err)
	}
}

func TestRuntimeKeyRefAmbiguousSiblingKeysWithSameDisplayPathDoesNotResolve(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="group" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string" use="required"/>
          </xs:complexType>
          <xs:key name="groupKey">
            <xs:selector xpath="."/>
            <xs:field xpath="@id"/>
          </xs:key>
        </xs:element>
      </xs:sequence>
      <xs:attribute name="rid" type="xs:string" use="required"/>
    </xs:complexType>
    <xs:keyref name="rootRef" refer="groupKey">
      <xs:selector xpath="."/>
      <xs:field xpath="@rid"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`
	engine := mustCompileRuntime(t, schema)
	mustNotValidateRuntime(t, engine, `<root rid="1"><group id="1"/><group id="1"/></root>`, xsderrors.CodeValidationIdentity)
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
	mustValidateRuntime(t, engine, `<r code="US"><child>x</child></r>`)
}

func TestFreezeRuntimeKeepsCompilerStateOnValidationFailure(t *testing.T) {
	c := compiledCompilerRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:element name="r" type="xs:string"/>
</xs:schema>`)
	build := c.RuntimeForTest()
	rootName := mustQName(t, &build.Names, "r")
	build.GlobalElements[rootName] = runtime.NoElement

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
	build := mutableSchemaBuild(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="MyInt"><xs:restriction base="xs:int"/></xs:simpleType>
  <xs:simpleType name="MyShort"><xs:restriction base="xs:short"/></xs:simpleType>
  <xs:simpleType name="TightInt"><xs:restriction base="xs:int"><xs:maxInclusive value="10"/></xs:restriction></xs:simpleType>
</xs:schema>`)

	if got := build.SimpleTypes[build.Builtin.Int].Fast; got != runtime.SimpleFastInt {
		t.Fatalf("xs:int Fast = %v, want runtime.SimpleFastInt", got)
	}
	if got := build.SimpleTypes[simpleBuildTypeIDByName(t, build, "MyInt")].Fast; got != runtime.SimpleFastInt {
		t.Fatalf("MyInt Fast = %v, want runtime.SimpleFastInt", got)
	}
	if got := build.SimpleTypes[simpleBuildTypeIDByName(t, build, "MyShort")].Fast; got != runtime.SimpleFastNone {
		t.Fatalf("MyShort Fast = %v, want runtime.SimpleFastNone", got)
	}
	if got := build.SimpleTypes[simpleBuildTypeIDByName(t, build, "TightInt")].Fast; got != runtime.SimpleFastNone {
		t.Fatalf("TightInt Fast = %v, want runtime.SimpleFastNone", got)
	}
}

func TestSimpleContentFacetRestrictionRecomputesFastPath(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
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
</xs:schema>`
	build := mutableSchemaBuild(t, schema)
	root := build.GlobalElements[mustQName(t, &build.Names, "root")]
	complexID, ok := build.Elements[root].Type.Complex()
	if !ok {
		t.Fatal("root type is not complex")
	}
	textType := build.ComplexTypes[complexID].TextType
	if got := build.SimpleTypes[textType].Fast; got != runtime.SimpleFastNone {
		t.Fatalf("simple content text type Fast = %v, want runtime.SimpleFastNone", got)
	}
	engine := mustCompileRuntime(t, schema)
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

type qnameLookup interface {
	LookupQName(namespace, local string) (runtime.QName, bool)
}

func mustQName(t *testing.T, rt qnameLookup, local string) runtime.QName {
	t.Helper()
	q, ok := rt.LookupQName("", local)
	if !ok {
		t.Fatalf("LookupQName(%q) failed", local)
	}
	return q
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
	build := mutableSchemaBuild(t, schema)
	if err := validateSchemaBuild(build); err != nil {
		t.Fatalf("validateSchemaBuild() error = %v", err)
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

func TestRuntimeElementAccessor(t *testing.T) {
	engine := mustCompileRuntime(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`)
	rt := publishedRuntime(t, engine)
	if _, ok := rt.Element(runtime.NoElement); ok {
		t.Error("element(noElement) resolved, want miss")
	}
	if _, ok := rt.Element(runtime.ElementID(1 << 30)); ok {
		t.Error("element(out of range) resolved, want miss")
	}
	rootName := mustQName(t, rt, "root")
	_, rootInfo, ok := rt.RootElement(runtime.RuntimeName{Known: true, Name: rootName})
	if !ok || rootInfo.Type.Kind != runtime.TypeSimple {
		t.Fatal("root element is missing")
	}
}

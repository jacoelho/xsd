package schemair

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/schemaast"
)

func TestResolveDocumentSetCoreDeclarations(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:maxLength value="10"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:complexType name="RootType">
    <xs:sequence>
      <xs:element name="code" type="tns:Code"/>
    </xs:sequence>
    <xs:attribute name="id" type="xs:ID" use="required"/>
  </xs:complexType>
  <xs:element name="root" type="tns:RootType"/>
</xs:schema>`)

	ir, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(ir.BuiltinTypes) == 0 {
		t.Fatal("builtin types empty")
	}
	if len(ir.Types) != 2 {
		t.Fatalf("types = %d, want 2", len(ir.Types))
	}
	if len(ir.Elements) != 2 {
		t.Fatalf("elements = %d, want global + local", len(ir.Elements))
	}
	if len(ir.Attributes) != 0 {
		t.Fatalf("attributes = %d, want no global attributes", len(ir.Attributes))
	}
	if len(ir.AttributeUses) != 1 || ir.AttributeUses[0].Use != AttributeRequired {
		t.Fatalf("attribute uses = %#v", ir.AttributeUses)
	}
	if len(ir.ComplexTypes) != 1 || ir.ComplexTypes[0].Content != ContentElement {
		t.Fatalf("complex plans = %#v", ir.ComplexTypes)
	}
	if len(ir.SimpleTypes) != 1 || ir.SimpleTypes[0].Facets[0].Kind != FacetMaxLength {
		t.Fatalf("simple specs = %#v", ir.SimpleTypes)
	}
}

func TestResolveDocumentSetInlineIDsAreDeterministic(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test" elementFormDefault="qualified">
  <xs:complexType name="RootType">
    <xs:sequence>
      <xs:element name="localSimple">
        <xs:simpleType>
          <xs:restriction base="xs:string">
            <xs:maxLength value="12"/>
          </xs:restriction>
        </xs:simpleType>
      </xs:element>
      <xs:element name="localComplex">
        <xs:complexType>
          <xs:attribute name="code">
            <xs:simpleType>
              <xs:restriction base="xs:string">
                <xs:enumeration value="A"/>
              </xs:restriction>
            </xs:simpleType>
          </xs:attribute>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
    <xs:attribute name="rootAttr">
      <xs:simpleType>
        <xs:list itemType="xs:int"/>
      </xs:simpleType>
    </xs:attribute>
  </xs:complexType>
  <xs:element name="root" type="tns:RootType"/>
</xs:schema>`)
	docs := &schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}

	first, err := Resolve(docs, ResolveConfig{})
	if err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}
	second, err := Resolve(docs, ResolveConfig{})
	if err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}
	if !reflect.DeepEqual(first.Types, second.Types) {
		t.Fatalf("types differ:\nfirst=%#v\nsecond=%#v", first.Types, second.Types)
	}
	if !reflect.DeepEqual(first.Elements, second.Elements) {
		t.Fatalf("elements differ:\nfirst=%#v\nsecond=%#v", first.Elements, second.Elements)
	}
	if !reflect.DeepEqual(first.Attributes, second.Attributes) {
		t.Fatalf("attributes differ:\nfirst=%#v\nsecond=%#v", first.Attributes, second.Attributes)
	}
	if !reflect.DeepEqual(first.AttributeUses, second.AttributeUses) {
		t.Fatalf("attribute uses differ:\nfirst=%#v\nsecond=%#v", first.AttributeUses, second.AttributeUses)
	}
}

func TestResolveDocumentSetRejectsTransitiveImportVisibility(t *testing.T) {
	a := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a" xmlns="urn:a" xmlns:b="urn:b" xmlns:c="urn:c">
  <xs:import namespace="urn:b" schemaLocation="b.xsd"/>
  <xs:element name="root" type="c:T"/>
</xs:schema>`)
	a.Location = "a.xsd"
	b := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b" xmlns="urn:b" xmlns:c="urn:c">
  <xs:import namespace="urn:c" schemaLocation="c.xsd"/>
</xs:schema>`)
	b.Location = "b.xsd"
	c := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:c" xmlns="urn:c">
  <xs:complexType name="T"/>
</xs:schema>`)
	c.Location = "c.xsd"

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*a, *b, *c}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), `namespace "urn:c" not imported`; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsElementDefaultAndFixed(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string" default="a" fixed="b"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot have both default and fixed values"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsElementDefaultOnElementOnlyComplexType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:sequence>
      <xs:element name="child" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="T" default="value"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot have default or fixed value because its type has element-only content"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsFixedOnMixedComplexType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T" mixed="true">
    <xs:sequence>
      <xs:element name="child" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="T" fixed="value"/>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsAttributeComplexType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="CT">
    <xs:simpleContent>
      <xs:extension base="xs:string"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:attribute name="a" type="CT"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "must reference a simple type"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsUnionComplexMemberType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="CT">
    <xs:simpleContent>
      <xs:extension base="xs:integer"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:simpleType name="U">
    <xs:union memberTypes="CT"/>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "must reference a simple type"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsEmptyRangeFacetValue(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T">
    <xs:restriction base="xs:decimal">
      <xs:maxInclusive value=""/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "invalid maxInclusive facet value"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsListOfUnionFixedValue(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:boolean xs:int xs:string"/>
  </xs:simpleType>
  <xs:element name="root" fixed="a b c d e f">
    <xs:simpleType>
      <xs:list itemType="U"/>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsUnionFixedOutsideMemberEnumeration(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" fixed="1">
    <xs:simpleType>
      <xs:union>
        <xs:simpleType>
          <xs:restriction base="xs:positiveInteger">
            <xs:minInclusive value="8"/>
            <xs:maxInclusive value="72"/>
          </xs:restriction>
        </xs:simpleType>
        <xs:simpleType>
          <xs:restriction base="xs:NMTOKEN">
            <xs:enumeration value="small"/>
            <xs:enumeration value="medium"/>
            <xs:enumeration value="large"/>
          </xs:restriction>
        </xs:simpleType>
      </xs:union>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "value not in enumeration"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsDefaultOutsidePattern(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="SixDigits">
    <xs:restriction base="xs:integer">
      <xs:pattern value="\d{6}"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:attribute name="a" type="SixDigits" default="0000000"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "pattern violation"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSimpleTypeDerivationCycle(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="foo"/>
  <xs:simpleType name="foo">
    <xs:restriction base="foo">
      <xs:pattern value="[0-9]{5}"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "type derivation cycle"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsChoiceWildcardOverlap(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice maxOccurs="10">
        <xs:any namespace="##other" processContents="lax"/>
        <xs:any namespace="A"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "content model is not deterministic"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSequenceWildcardOverlapAfterRepeat(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence maxOccurs="10">
        <xs:any namespace="##other" maxOccurs="2" processContents="lax"/>
        <xs:any namespace="A" processContents="lax"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "content model is not deterministic"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsExactRepeatBeforeSameElement(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:sequence>
      <xs:element name="b" minOccurs="2" maxOccurs="2"/>
      <xs:element name="b"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsVariableRepeatBeforeSameElement(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:sequence>
      <xs:element name="b" minOccurs="1" maxOccurs="2"/>
      <xs:element name="b"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "content model is not deterministic"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsNestedVariableRepeatBeforeSameFollowingElement(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:sequence>
      <xs:sequence>
        <xs:element name="a"/>
        <xs:sequence minOccurs="0">
          <xs:element name="b"/>
          <xs:element name="c" minOccurs="1" maxOccurs="2"/>
        </xs:sequence>
      </xs:sequence>
      <xs:element name="c"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "content model is not deterministic"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsExcludedWildcardBeforeElement(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:sequence>
      <xs:any namespace="##any" minOccurs="0" maxOccurs="0"/>
      <xs:element name="e"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsAllSubstitutionOverlap(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element type="xs:short" substitutionGroup="b" name="a"/>
  <xs:element type="xs:decimal" name="b"/>
  <xs:complexType name="T">
    <xs:all>
      <xs:element ref="a"/>
      <xs:element ref="b"/>
    </xs:all>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "content model is not deterministic"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsAllSubstitutionOverlapBlockedByHead(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" substitutionGroup="b" type="xs:anyType"/>
  <xs:element name="b" substitutionGroup="c" type="xs:anyType"/>
  <xs:element name="c" substitutionGroup="d" type="xs:anyType" block="substitution"/>
  <xs:element name="d" block="substitution"/>
  <xs:complexType name="T">
    <xs:all>
      <xs:element ref="b"/>
      <xs:element ref="c"/>
      <xs:element ref="d"/>
    </xs:all>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsInvalidElementDefaultLexicalValue(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:date" default="not-a-date"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "invalid default value"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsInvalidIdentitySelectorXPath(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:unique name="u">
      <xs:selector xpath="|"/>
      <xs:field xpath="@id"/>
    </xs:unique>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "selector xpath"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsAllModelExtensionWithAdditionalParticles(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:all>
      <xs:element name="a"/>
    </xs:all>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="Base">
        <xs:all>
          <xs:element name="b"/>
        </xs:all>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot extend all content model with additional particles"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsAllModelExtensionOfEmptyAllBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G">
    <xs:all>
      <xs:element name="a"/>
    </xs:all>
  </xs:group>
  <xs:complexType name="Base">
    <xs:all/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="Base">
        <xs:group ref="G"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsAllModelExtensionOfNonEmptyBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="a"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="Base">
        <xs:all>
          <xs:element name="b"/>
        </xs:all>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot extend non-empty content model with all content model"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsLengthWithMinLengthFacet(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:string">
        <xs:length value="5"/>
        <xs:minLength value="1"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "length facet cannot be used together with minLength or maxLength"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsLengthFacetOnUnionRestriction(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:NMTOKEN xs:integer"/>
  </xs:simpleType>
  <xs:simpleType name="T">
    <xs:restriction base="U">
      <xs:length value="5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "facet length is not applicable to union type"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsLessRestrictiveWhitespaceFacet(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="A">
    <xs:restriction base="xs:string">
      <xs:whiteSpace value="replace"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="B">
    <xs:restriction base="A">
      <xs:whiteSpace value="preserve"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "whiteSpace facet value 'preserve' cannot be less restrictive than base type's 'replace'"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsMoreRestrictiveWhitespaceFacet(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="A">
    <xs:restriction base="xs:string">
      <xs:whiteSpace value="preserve"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="B">
    <xs:restriction base="A">
      <xs:whiteSpace value="replace"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsDigitsFacetOnStringRestriction(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T">
    <xs:restriction base="xs:string">
      <xs:totalDigits value="5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "facet totalDigits is only applicable to decimal-derived types"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsFractionDigitsGreaterThanTotalDigits(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:decimal">
        <xs:fractionDigits value="6"/>
        <xs:totalDigits value="5"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "fractionDigits (6) must be <= totalDigits (5)"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsRangeFacetWeakerThanBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:time">
      <xs:maxInclusive value="12:00:00-10:00"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:maxInclusive value="12:00:00-14:00"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "derived value (12:00:00-14:00) must be <= base value (12:00:00-10:00)"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsRangeFacetRelaxingBuiltinBaseBound(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:positiveInteger">
        <xs:maxExclusive value="1"/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "derived max (1) cannot relax base min bound"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsWildcardProcessContentsRelaxation(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:any processContents="strict"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:any processContents="lax"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "processContents in restriction must be identical or stronger than base"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsExcludedElementRestrictingWildcardOutsideNamespace(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting">
  <xs:complexType name="B">
    <xs:sequence>
      <xs:any namespace="##targetNamespace" minOccurs="0"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="R">
    <xs:complexContent>
      <xs:restriction base="x:B">
        <xs:sequence>
          <xs:element name="e1" minOccurs="0" maxOccurs="0"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsGroupRefOccurrenceRelaxation(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A">
    <xs:choice>
      <xs:group ref="x" maxOccurs="unbounded"/>
      <xs:group ref="y" maxOccurs="unbounded"/>
    </xs:choice>
  </xs:complexType>
  <xs:group name="x">
    <xs:sequence>
      <xs:element name="x1"/>
      <xs:element name="x2"/>
    </xs:sequence>
  </xs:group>
  <xs:group name="y">
    <xs:choice>
      <xs:element name="y1"/>
      <xs:element name="y2"/>
    </xs:choice>
  </xs:group>
  <xs:element name="elem">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="A">
          <xs:choice>
            <xs:group ref="x" maxOccurs="unbounded"/>
          </xs:choice>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "maxOccurs cannot be unbounded when base maxOccurs is bounded (1)"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsChoiceRestrictionRemovedBranch(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:choice>
      <xs:element name="a"/>
      <xs:element name="b"/>
    </xs:choice>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:choice>
          <xs:element name="a" minOccurs="0" maxOccurs="0"/>
          <xs:element name="b"/>
        </xs:choice>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsChoiceRestrictionSubsetByName(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting" elementFormDefault="qualified">
  <xs:complexType name="base">
    <xs:choice>
      <xs:element name="e0"/>
      <xs:element name="e1"/>
      <xs:element name="e2"/>
    </xs:choice>
  </xs:complexType>
  <xs:complexType name="testing">
    <xs:complexContent>
      <xs:restriction base="x:base">
        <xs:choice>
          <xs:element name="e1"/>
          <xs:element name="e2"/>
        </xs:choice>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsChoiceRestrictionReorderedBranches(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting">
  <xs:complexType name="B">
    <xs:sequence>
      <xs:choice>
        <xs:element name="c1" minOccurs="1" maxOccurs="1"/>
        <xs:element name="c2" minOccurs="1" maxOccurs="1"/>
      </xs:choice>
      <xs:element name="foo" minOccurs="1" maxOccurs="1"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="R">
    <xs:complexContent>
      <xs:restriction base="x:B">
        <xs:sequence>
          <xs:choice>
            <xs:element name="c2" minOccurs="1" maxOccurs="1"/>
            <xs:element name="c1" minOccurs="1" maxOccurs="1"/>
          </xs:choice>
          <xs:element name="foo" minOccurs="1" maxOccurs="1"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "does not match any base particle in choice"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsPointlessChoiceRestriction(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://myuri" xmlns="http://myuri">
  <xs:element name="A" type="xs:string"/>
  <xs:element name="B" type="xs:string"/>
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element ref="A"/>
      <xs:element ref="B"/>
      <xs:choice>
        <xs:sequence>
          <xs:element name="AAA" minOccurs="0" maxOccurs="unbounded"/>
          <xs:element name="BBB" minOccurs="0" maxOccurs="unbounded"/>
        </xs:sequence>
        <xs:sequence>
          <xs:element name="AAAA" minOccurs="0" maxOccurs="unbounded"/>
          <xs:element name="BBBB" minOccurs="0" maxOccurs="unbounded"/>
        </xs:sequence>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:sequence>
          <xs:element ref="A"/>
          <xs:element ref="B"/>
          <xs:choice>
            <xs:sequence>
              <xs:element name="AAA" minOccurs="0" maxOccurs="unbounded"/>
              <xs:element name="BBB" minOccurs="0" maxOccurs="unbounded"/>
            </xs:sequence>
          </xs:choice>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "ComplexContent restriction"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsPointlessNestedSequenceRestriction(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element abstract="true" name="aba" type="xs:string"/>
  <xs:element abstract="true" name="abb" type="xs:int"/>
  <xs:element name="a" substitutionGroup="aba" type="xs:string"/>
  <xs:element name="b" substitutionGroup="abb" type="xs:int"/>
  <xs:element name="d" type="xs:anyURI"/>
  <xs:group name="abs">
    <xs:choice>
      <xs:element ref="aba"/>
      <xs:element ref="abb"/>
    </xs:choice>
  </xs:group>
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:group maxOccurs="unbounded" minOccurs="0" ref="abs"/>
      <xs:element minOccurs="0" ref="d"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:sequence>
          <xs:sequence minOccurs="1" maxOccurs="1">
            <xs:element ref="a"/>
            <xs:element ref="b"/>
          </xs:sequence>
          <xs:element ref="d"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "restriction particle"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsPointlessSequenceToChoiceRestriction(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:choice>
      <xs:sequence minOccurs="0">
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string" minOccurs="0"/>
      </xs:sequence>
    </xs:choice>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:choice>
          <xs:sequence>
            <xs:choice>
              <xs:element name="a" type="xs:string"/>
              <xs:element name="b" type="xs:string" minOccurs="0"/>
            </xs:choice>
          </xs:sequence>
        </xs:choice>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot restrict sequence to choice"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsChoiceRestrictionSubsetWithBranchOccurrence(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting" elementFormDefault="qualified">
  <xs:complexType name="base">
    <xs:choice>
      <xs:element name="foo" minOccurs="2" maxOccurs="6" type="xs:boolean"/>
      <xs:element name="foo1" minOccurs="0" maxOccurs="6" type="xs:boolean"/>
    </xs:choice>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="x:base">
          <xs:choice>
            <xs:element name="foo" minOccurs="2" maxOccurs="2" type="xs:boolean"/>
          </xs:choice>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetEmitsUnusedGroupLocalElements(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="foo">
    <xs:sequence>
      <xs:element name="a"/>
    </xs:sequence>
  </xs:group>
</xs:schema>`)

	ir, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(ir.Elements) != 1 {
		t.Fatalf("elements = %d, want 1", len(ir.Elements))
	}
	if got, want := ir.Elements[0].Name.Local, "a"; got != want {
		t.Fatalf("element name = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRuntimeNamesIncludeIdentityPathElements(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="A">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="B"/>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="KeyA">
      <xs:selector xpath="tns:A"/>
      <xs:field xpath="./tns:B"/>
    </xs:key>
  </xs:element>
</xs:schema>`)

	ir, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for _, name := range []Name{
		{Namespace: "urn:test", Local: "A"},
		{Namespace: "urn:test", Local: "B"},
	} {
		if !hasSymbolOp(ir.RuntimeNames, name) {
			t.Fatalf("runtime name plan missing symbol %#v", name)
		}
	}
}

func TestResolveDocumentSetRuntimeNamesIncludeImportedIdentityPathElements(t *testing.T) {
	mainDoc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:bar="bar"
           targetNamespace="foo"
           elementFormDefault="qualified">
  <xs:import namespace="bar" schemaLocation="bar.xsd"/>
  <xs:element name="root1">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="bar:root"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	importedDoc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="bar"
           targetNamespace="bar"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="A"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="KeyAAA">
      <xs:selector xpath="tns:A"/>
      <xs:field xpath="."/>
    </xs:key>
  </xs:element>
</xs:schema>`)

	ir, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*mainDoc, *importedDoc}}, ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !hasSymbolOp(ir.RuntimeNames, Name{Namespace: "bar", Local: "A"}) {
		t.Fatalf("runtime name plan missing imported local element symbol")
	}
}

func TestResolveDocumentSetRejectsInvalidEnumerationLexicalValue(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:language">
        <xs:enumeration value=""/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "enumeration value 1 (\"\") is not valid for base type language"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsDecimalEnumerationRestrictionByValue(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:decimal">
      <xs:enumeration value="1.0"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:enumeration value="1"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsBooleanEnumerationRestrictionByValue(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:boolean">
      <xs:enumeration value="true"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:enumeration value="1"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsIntegerEnumerationRestrictionByValue(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:int">
      <xs:enumeration value="01"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:enumeration value="1"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsListEnumerationRestrictionByValue(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="IntList">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="Base">
    <xs:restriction base="IntList">
      <xs:enumeration value=" 01  2 "/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:enumeration value="1 2"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsEnumerationRestrictionOutsideBaseValueSpace(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:decimal">
      <xs:enumeration value="1.0"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:enumeration value="2"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), `value not in enumeration`; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsQNameEnumerationRestrictionByExpandedName(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:a="urn:q" xmlns:b="urn:q">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="a:item"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base">
      <xs:enumeration value="b:item"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsNotationEnumerationRestrictionByExpandedName(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:a="urn:n" xmlns:b="urn:n" targetNamespace="urn:n">
  <xs:notation name="item" public="urn:item"/>
  <xs:simpleType name="Base">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="a:item"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="a:Base">
      <xs:enumeration value="b:item"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsEmptyListEnumerationLexicalValue(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:IDREFS">
        <xs:enumeration value=""/>
      </xs:restriction>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "enumeration value 1 (\"\") is not valid for base type IDREFS"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsUnionRestrictionEnumerationFromUnrestrictedMember(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="u1">
    <xs:union>
      <xs:simpleType>
        <xs:restriction base="xs:nonNegativeInteger"/>
      </xs:simpleType>
      <xs:simpleType>
        <xs:restriction base="xs:string">
          <xs:enumeration value="x"/>
          <xs:enumeration value="y"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:union>
  </xs:simpleType>
  <xs:simpleType name="u3">
    <xs:restriction base="u1">
      <xs:enumeration value="x"/>
      <xs:enumeration value="y"/>
      <xs:enumeration value="1"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsSubstitutionMemberWithUnionMemberType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns="http://xsdtesting">
  <xs:simpleType name="myType10">
    <xs:union memberTypes="xs:float xs:integer">
      <xs:simpleType>
        <xs:restriction base="xs:boolean"/>
      </xs:simpleType>
      <xs:simpleType>
        <xs:restriction base="xs:string">
          <xs:enumeration value="x"/>
          <xs:enumeration value="y"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:union>
  </xs:simpleType>
  <xs:element name="E1" type="myType10"/>
  <xs:element name="E2" substitutionGroup="E1" type="xs:integer"/>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsAttributeRestrictionToWiderUnionType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns="http://xsdtesting">
  <xs:simpleType name="myType10">
    <xs:union memberTypes="xs:float xs:integer">
      <xs:simpleType>
        <xs:restriction base="xs:boolean"/>
      </xs:simpleType>
    </xs:union>
  </xs:simpleType>
  <xs:complexType name="CT1">
    <xs:attribute name="att1" type="xs:integer"/>
  </xs:complexType>
  <xs:complexType name="CT2">
    <xs:complexContent>
      <xs:restriction base="CT1">
        <xs:attribute name="att1" type="myType10"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "attribute 'att1' type cannot be changed"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsProhibitedAttributeWithoutType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:attribute name="c" type="xs:string"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:attribute name="c" use="prohibited"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsAttributeDefaultAndFixed(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Root">
    <xs:attribute name="code" type="xs:string" default="a" fixed="b"/>
  </xs:complexType>
  <xs:element name="root" type="Root"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot have both default and fixed values"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsXSIAttributeDeclaration(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.w3.org/2001/XMLSchema-instance">
  <xs:attribute name="bad" type="xs:string"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "attributes in the xsi namespace are not allowed"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetKeepsAttributeGroupDeclNameLexical(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test" attributeFormDefault="qualified">
  <xs:attributeGroup name="G">
    <xs:attribute name="a" type="xs:string"/>
  </xs:attributeGroup>
  <xs:complexType name="T">
    <xs:attributeGroup ref="tns:G"/>
  </xs:complexType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`)

	ir, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	var declName, useName Name
	for _, attr := range ir.Attributes {
		if attr.Name.Local == "a" {
			declName = attr.Name
		}
	}
	for _, use := range ir.AttributeUses {
		if use.Name.Local == "a" {
			useName = use.Name
		}
	}
	if declName != (Name{Local: "a"}) {
		t.Fatalf("attribute declaration name = %#v, want lexical local name", declName)
	}
	if useName != (Name{Namespace: "urn:test", Local: "a"}) {
		t.Fatalf("attribute use name = %#v, want qualified effective name", useName)
	}
}

func TestResolveDocumentSetRejectsDuplicateAttributeUses(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test" attributeFormDefault="qualified">
  <xs:attributeGroup name="G">
    <xs:attribute name="a" type="xs:string"/>
  </xs:attributeGroup>
  <xs:complexType name="T">
    <xs:attributeGroup ref="tns:G"/>
    <xs:attribute name="a" type="xs:string"/>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "duplicate attribute 'a' in namespace 'urn:test'"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsRestrictionRelaxingRequiredAttribute(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:attribute name="A" type="xs:string" use="required"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:attribute name="A" type="xs:string" use="optional"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "required attribute 'A' cannot be relaxed"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsRestrictionChangingFixedAttribute(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:attribute name="A" type="xs:string" fixed="base"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:attribute name="A" type="xs:string" fixed="derived"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "attribute 'A' fixed value must match base type"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsRestrictionAttributeNotInBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:anyAttribute namespace="##other"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:attribute name="A" type="xs:string"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "attribute 'A' not present in base type"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsInvalidAnyAttributeRestriction(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:anyAttribute namespace="##other"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:anyAttribute namespace="##any"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "anyAttribute restriction: derived anyAttribute is not a valid subset of base anyAttribute"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsNonExpressibleAnyAttributeExtension(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="a" xmlns:a="a" xmlns:b="b">
  <xs:complexType name="Base">
    <xs:anyAttribute namespace="##other"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="a:Base">
        <xs:anyAttribute namespace="##local b c"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "anyAttribute extension: union of derived and base anyAttribute is not expressible"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsRestrictionNormalizedFixedAttribute(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="ints">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:complexType name="Base">
    <xs:attribute name="A" type="ints" fixed="1   2        3"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:attribute name="A" type="ints" fixed="1 2 3"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsAttributeRefFixedConflict(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="code" fixed="123"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute ref="code" fixed="abc"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "attribute reference 'code' fixed value 'abc' conflicts with declaration fixed value '123'"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsLocalIDAttributeFixed(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Root">
    <xs:attribute name="id" type="xs:ID" fixed="A1"/>
  </xs:complexType>
  <xs:element name="root" type="Root"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "ID types cannot have default or fixed values"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsAllChildMaxOccursGreaterThanOne(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g">
    <xs:all>
      <xs:element name="a" maxOccurs="2"/>
    </xs:all>
  </xs:group>
  <xs:element name="root">
    <xs:complexType>
      <xs:group ref="g"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "all particles must have maxOccurs <= 1"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsAllGroupRefInsideSequence(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G">
    <xs:all>
      <xs:element name="a"/>
    </xs:all>
  </xs:group>
  <xs:complexType name="T">
    <xs:sequence>
      <xs:group ref="G"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "xs:all cannot be nested inside sequence"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsAllDuplicateElementRef(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="e1" type="xs:string"/>
  <xs:complexType name="Root">
    <xs:all>
      <xs:element ref="e1"/>
      <xs:element ref="e1"/>
    </xs:all>
  </xs:complexType>
  <xs:element name="root" type="Root"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "xs:all: duplicate element declaration 'e1'"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsMaxZeroWithMinNonZeroParticle(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="g">
    <xs:sequence>
      <xs:element name="a"/>
    </xs:sequence>
  </xs:group>
  <xs:element name="root">
    <xs:complexType>
      <xs:group ref="g" minOccurs="1" maxOccurs="0"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "maxOccurs cannot be 0 when minOccurs > 0"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsDuplicateLocalElementDifferentTypes(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:complexType name="RootType">
    <xs:sequence>
      <xs:element name="A" type="xs:string"/>
      <xs:element name="A" type="xs:int"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="RootType"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "duplicate local element declaration 'A' with different types"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsRestrictionMinOccursBelowBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="head"/>
      <xs:choice minOccurs="1">
        <xs:element name="a"/>
        <xs:element name="b"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element name="head"/>
          <xs:choice minOccurs="0">
            <xs:element name="a"/>
            <xs:element name="b"/>
          </xs:choice>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "ComplexContent restriction: minOccurs (0) must be >= base minOccurs (1)"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsInlineRestrictionAgainstContainingType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="foo" xmlns="foo">
  <xs:complexType name="foo">
    <xs:sequence>
      <xs:element name="foo"/>
      <xs:element name="bar" minOccurs="0">
        <xs:complexType>
          <xs:complexContent>
            <xs:restriction base="foo">
              <xs:sequence>
                <xs:element name="foo"/>
              </xs:sequence>
            </xs:restriction>
          </xs:complexContent>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsDirectComplexTypeDerivationCycle(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:complexContent>
      <xs:restriction base="T"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "type derivation cycle"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsMutualComplexTypeDerivationCycle(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="foo" xmlns:foo="foo">
  <xs:complexType name="foo">
    <xs:complexContent>
      <xs:extension base="foo:bar"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="bar">
    <xs:complexContent>
      <xs:extension base="foo:foo"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "type derivation cycle"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsRestrictionOmittingOptionalSequenceParticle(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="foo" xmlns="foo">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="a"/>
      <xs:element name="b" minOccurs="0"/>
      <xs:element name="c"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="rst">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element name="a"/>
          <xs:element name="c"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsRestrictionOmittingEmptiableGroup(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence minOccurs="1" maxOccurs="1">
      <xs:sequence>
        <xs:element name="e1"/>
      </xs:sequence>
      <xs:sequence>
        <xs:element name="e2" minOccurs="0"/>
      </xs:sequence>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="base">
          <xs:sequence>
            <xs:sequence>
              <xs:element name="e1"/>
            </xs:sequence>
          </xs:sequence>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsRestrictionThroughGroupWrapper(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting" elementFormDefault="qualified">
  <xs:group name="P">
    <xs:sequence>
      <xs:element name="e1"/>
    </xs:sequence>
  </xs:group>
  <xs:complexType name="base">
    <xs:sequence minOccurs="1" maxOccurs="1">
      <xs:group ref="x:P"/>
      <xs:sequence>
        <xs:element name="e2" minOccurs="0"/>
      </xs:sequence>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="x:base">
          <xs:sequence>
            <xs:element name="e1" minOccurs="1"/>
          </xs:sequence>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsRestrictionThroughEmptiableGroupWrapper(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:any namespace="##any"/>
      <xs:sequence>
        <xs:element name="a" minOccurs="0"/>
      </xs:sequence>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element name="a" type="xs:string"/>
          <xs:element name="a" type="xs:string" minOccurs="0"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsAllRestrictionWithTooFewWildcardParticles(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:any namespace="##any" minOccurs="3" maxOccurs="3"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:all>
          <xs:element name="e1" type="xs:string"/>
          <xs:element name="e2" type="xs:string"/>
        </xs:all>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "ComplexContent restriction"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsAllRestrictionWithMatchingWildcardParticleCount(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:any namespace="##any" minOccurs="3" maxOccurs="3"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:all>
          <xs:element name="e1" type="xs:string"/>
          <xs:element name="e2" type="xs:string"/>
          <xs:element name="e3" type="xs:string"/>
        </xs:all>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsSequenceRestrictionWithMatchingWildcardParticleCount(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:any namespace="##any" minOccurs="2" maxOccurs="3"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element name="e" type="xs:string"/>
          <xs:element name="e" type="xs:string"/>
          <xs:any namespace="##targetNamespace"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsSequenceRestrictingChoiceBranch(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:choice>
      <xs:element name="myElement" type="xs:string"/>
      <xs:element name="myElement2" type="xs:string" minOccurs="0"/>
    </xs:choice>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element name="myElement2" type="xs:string" minOccurs="0" maxOccurs="1"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsChoiceGroupWrapperRestrictionRelaxingChildMax(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:choice>
      <xs:sequence minOccurs="1" maxOccurs="2">
        <xs:element name="b" minOccurs="2" maxOccurs="2"/>
      </xs:sequence>
    </xs:choice>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:choice>
          <xs:element name="b" minOccurs="3" maxOccurs="3"/>
        </xs:choice>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "maxOccurs"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsChoiceRestrictedByRepeatingSequenceBranch(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:choice maxOccurs="2">
        <xs:element name="a" type="xs:string" minOccurs="1"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:sequence maxOccurs="2">
            <xs:element name="a" type="xs:string" minOccurs="1"/>
          </xs:sequence>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsChoiceRestrictedBySequenceWithEnoughEffectiveOccurrences(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting">
  <xs:complexType name="B">
    <xs:choice minOccurs="3" maxOccurs="9">
      <xs:element name="e1"/>
      <xs:element name="e2"/>
    </xs:choice>
  </xs:complexType>
  <xs:complexType name="R">
    <xs:complexContent>
      <xs:restriction base="x:B">
        <xs:sequence minOccurs="2" maxOccurs="4">
          <xs:element name="e1"/>
          <xs:element name="e2"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsSequenceRestrictingAllGroupSubset(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:all>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="b" type="xs:string" minOccurs="0"/>
    </xs:all>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element name="a" type="xs:string"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsSequenceRestrictingAllGroupMissingRequired(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:all>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="b" type="xs:string"/>
    </xs:all>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element name="a" type="xs:string"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "restriction particle has no matching base particle"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsChoiceRestrictingRequiredAllGroup(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting" elementFormDefault="qualified">
  <xs:complexType name="base">
    <xs:all>
      <xs:element name="e1"/>
      <xs:element name="e2"/>
    </xs:all>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="x:base">
          <xs:choice>
            <xs:element name="e1"/>
            <xs:element name="e2"/>
          </xs:choice>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "minOccurs"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsAllRestrictionWithExtraElement(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting">
  <xs:complexType name="address">
    <xs:sequence>
      <xs:element name="street"/>
      <xs:element name="zip"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="e3"/>
  <xs:complexType name="B">
    <xs:all>
      <xs:element name="e1" minOccurs="1"/>
      <xs:element name="e2" type="x:address" minOccurs="0" maxOccurs="0"/>
      <xs:element ref="x:e3" minOccurs="1" maxOccurs="1"/>
    </xs:all>
  </xs:complexType>
  <xs:complexType name="R">
    <xs:complexContent>
      <xs:restriction base="x:B">
        <xs:all>
          <xs:element name="e1" minOccurs="1" maxOccurs="1"/>
          <xs:element name="e2" type="x:address" minOccurs="0" maxOccurs="0"/>
          <xs:element ref="x:e3" minOccurs="1" maxOccurs="1"/>
          <xs:element name="foo"/>
        </xs:all>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "maxOccurs"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsAllRestrictionReorderedElements(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting">
  <xs:complexType name="address">
    <xs:sequence>
      <xs:element name="street"/>
      <xs:element name="zip"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="e3"/>
  <xs:complexType name="B">
    <xs:all>
      <xs:element name="e1"/>
      <xs:element name="e2" type="x:address"/>
      <xs:element ref="x:e3"/>
    </xs:all>
  </xs:complexType>
  <xs:complexType name="R">
    <xs:complexContent>
      <xs:restriction base="x:B">
        <xs:all>
          <xs:element name="e2" type="x:address"/>
          <xs:element name="e1"/>
          <xs:element ref="x:e3"/>
        </xs:all>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "element name mismatch"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsElementRestrictedByWildcard(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting" elementFormDefault="qualified">
  <xs:complexType name="base">
    <xs:choice>
      <xs:element name="e1" minOccurs="2" maxOccurs="10"/>
    </xs:choice>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="x:base">
          <xs:choice>
            <xs:any minOccurs="3" maxOccurs="9"/>
          </xs:choice>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot restrict non-wildcard to wildcard"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsChoiceWrapperRestrictionBelowMinOccurs(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting">
  <xs:complexType name="B">
    <xs:sequence>
      <xs:choice minOccurs="2" maxOccurs="3">
        <xs:element name="c1" minOccurs="1" maxOccurs="1"/>
        <xs:element name="c2" minOccurs="1" maxOccurs="1"/>
      </xs:choice>
      <xs:choice>
        <xs:element name="d1" minOccurs="1" maxOccurs="1"/>
        <xs:element name="d2" minOccurs="1" maxOccurs="1"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="R">
    <xs:complexContent>
      <xs:restriction base="x:B">
        <xs:sequence>
          <xs:element name="c1" minOccurs="1" maxOccurs="1"/>
          <xs:element name="d1" minOccurs="1" maxOccurs="1"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "minOccurs"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsChoiceWrapperBranchMaxOccurs(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting">
  <xs:complexType name="B">
    <xs:sequence>
      <xs:choice>
        <xs:element name="c1" maxOccurs="2"/>
        <xs:element name="c2" maxOccurs="2"/>
      </xs:choice>
      <xs:choice>
        <xs:element name="d1" maxOccurs="2"/>
        <xs:element name="d2" maxOccurs="2"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="R">
    <xs:complexContent>
      <xs:restriction base="x:B">
        <xs:sequence>
          <xs:element name="c1" maxOccurs="2"/>
          <xs:element name="d1" maxOccurs="2"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsOptionalRepeatingChoiceRestrictingBranch(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:a="http://xsdtesting">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="annotation" minOccurs="0"/>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="element" minOccurs="1" maxOccurs="1"/>
        <xs:element name="any"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="a:Base">
        <xs:sequence>
          <xs:element name="annotation" minOccurs="0"/>
          <xs:element name="element" minOccurs="0" maxOccurs="unbounded"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsSingleChoiceOptionalBranchRelaxation(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:choice minOccurs="0">
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:choice>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element name="a" type="xs:string" minOccurs="0"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "minOccurs"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsRestrictionMakingElementNillable(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting" elementFormDefault="qualified">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="e1" minOccurs="2" maxOccurs="6" nillable="false"/>
      <xs:element name="e2" minOccurs="2" maxOccurs="6"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="x:base">
          <xs:sequence>
            <xs:element name="e1" minOccurs="2" maxOccurs="2" nillable="true"/>
            <xs:element name="e2" minOccurs="3" maxOccurs="5"/>
          </xs:sequence>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "nillable cannot be true"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsElementRestrictionToUnionMemberType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A" mixed="true">
    <xs:sequence>
      <xs:element name="A" type="deci-string" maxOccurs="unbounded"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="B">
    <xs:complexContent>
      <xs:restriction base="A">
        <xs:sequence>
          <xs:element name="A" type="xs:string" maxOccurs="10"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:simpleType name="deci-string">
    <xs:union memberTypes="xs:decimal xs:string"/>
  </xs:simpleType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsElementRestrictionByExtensionDerivedType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting" elementFormDefault="qualified">
  <xs:complexType name="foo">
    <xs:choice>
      <xs:element name="f1" maxOccurs="5"/>
      <xs:element name="f2"/>
    </xs:choice>
  </xs:complexType>
  <xs:complexType name="bar">
    <xs:complexContent>
      <xs:extension base="x:foo">
        <xs:choice>
          <xs:element name="f3" maxOccurs="6"/>
          <xs:element name="f4"/>
        </xs:choice>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="B">
    <xs:choice>
      <xs:element name="c1" type="x:foo"/>
      <xs:element name="c2"/>
    </xs:choice>
  </xs:complexType>
  <xs:complexType name="R">
    <xs:complexContent>
      <xs:restriction base="x:B">
        <xs:choice>
          <xs:element name="c1" type="x:bar"/>
          <xs:element name="c2"/>
        </xs:choice>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "restriction-derived"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsRestrictionNormalizedFixedElement(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="intList">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="e" type="intList" fixed="1 2 3"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element name="e" type="intList" fixed="1  2   3"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsNestedChoiceRestrictedByWildcard(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting" elementFormDefault="qualified">
  <xs:complexType name="base">
    <xs:choice>
      <xs:choice>
        <xs:element name="e1" minOccurs="0"/>
        <xs:element name="e2" minOccurs="0"/>
      </xs:choice>
    </xs:choice>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="x:base">
          <xs:choice>
            <xs:any/>
          </xs:choice>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot restrict non-wildcard to wildcard"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSequenceRestrictedByAllWithoutWildcard(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://xsdtesting" xmlns:x="http://xsdtesting" elementFormDefault="qualified">
  <xs:group name="Gb">
    <xs:sequence>
      <xs:element name="e1"/>
      <xs:element name="e2"/>
    </xs:sequence>
  </xs:group>
  <xs:group name="Gr">
    <xs:all>
      <xs:element name="e1"/>
      <xs:element name="e2"/>
    </xs:all>
  </xs:group>
  <xs:complexType name="base">
    <xs:group ref="x:Gb"/>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:restriction base="x:base">
          <xs:group ref="x:Gr"/>
        </xs:restriction>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot restrict sequence to xs:all"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsSequenceRestrictedBySingleElementAll(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="e1" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:group ref="grp"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:group name="grp">
    <xs:all>
      <xs:element name="e1" type="xs:string"/>
    </xs:all>
  </xs:group>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsChoiceOfSubstitutionMembersRestrictingHead(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head"/>
  <xs:element name="m1" substitutionGroup="head"/>
  <xs:element name="m2" substitutionGroup="head"/>
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element ref="head"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:choice>
            <xs:element ref="m1"/>
            <xs:element ref="m2"/>
          </xs:choice>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsSubstitutionMemberRestrictingHead(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" substitutionGroup="b" type="xs:int"/>
  <xs:element name="b" substitutionGroup="c" type="xs:int"/>
  <xs:element name="c" substitutionGroup="d" type="xs:anyType"/>
  <xs:element name="d" type="xs:anyType"/>
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element ref="d"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:sequence>
          <xs:element ref="c"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsSubstitutionMemberTypeNotDerived(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="ListType">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="UnionType">
    <xs:union memberTypes="ListType xs:date"/>
  </xs:simpleType>
  <xs:element name="head" type="ListType"/>
  <xs:element name="member" type="UnionType" substitutionGroup="head"/>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "type 'UnionType' is not derived from substitution group head type 'ListType'"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetSimpleContentRestrictionInlineSimpleType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:decimal"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:simpleType>
          <xs:restriction base="xs:integer"/>
        </xs:simpleType>
        <xs:maxInclusive value="16"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`)

	ir, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	var plan ComplexTypePlan
	for _, candidate := range ir.ComplexTypes {
		if ir.Types[candidate.TypeDecl-1].Name.Local == "Restricted" {
			plan = candidate
			break
		}
	}
	if plan.TypeDecl == 0 {
		t.Fatal("Restricted complex plan not found")
	}
	if plan.Content != ContentSimple {
		t.Fatalf("content = %v, want simple", plan.Content)
	}
	if plan.TextSpec.Base.Name.Local != "decimal" {
		t.Fatalf("text spec base = %v, want base simple content type", plan.TextSpec.Base)
	}
	if len(plan.TextSpec.Facets) == 0 || plan.TextSpec.Facets[len(plan.TextSpec.Facets)-1].Name != "maxInclusive" {
		t.Fatalf("text spec facets = %#v", plan.TextSpec.Facets)
	}
}

func TestResolveDocumentSetComplexContentExtensionFromSimpleContentIsEmpty(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns="urn:test">
  <xs:complexType name="Type1">
    <xs:simpleContent>
      <xs:extension base="xs:string">
        <xs:attribute name="Field1" type="xs:string"/>
      </xs:extension>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Type2">
    <xs:complexContent>
      <xs:extension base="Type1">
        <xs:attribute name="Field2" type="xs:string"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	ir, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	var plan ComplexTypePlan
	for _, candidate := range ir.ComplexTypes {
		if ir.Types[candidate.TypeDecl-1].Name.Local == "Type2" {
			plan = candidate
			break
		}
	}
	if plan.TypeDecl == 0 {
		t.Fatal("Type2 complex plan not found")
	}
	if plan.Content != ContentEmpty || !isZeroTypeRef(plan.TextType) || !isZeroSimpleTypeSpec(plan.TextSpec) {
		t.Fatalf("complex plan = %#v, want empty content without text", plan)
	}
}

func TestResolveDocumentSetRejectsComplexContentRestrictionFromSimpleContent(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:string">
        <xs:attribute name="attr1" type="xs:string"/>
      </xs:extension>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:restriction base="Base">
        <xs:sequence/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "complexContent restriction cannot derive from simpleContent type"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSimpleContentRestrictionSimpleBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="xs:string"/>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "simpleContent restriction cannot have simpleType base '{http://www.w3.org/2001/XMLSchema}string'"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSimpleContentRestrictionFacetsOnAnySimpleType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:anySimpleType"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:minLength value="1"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "simpleContent restriction cannot apply facets to base type anySimpleType"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSimpleContentExtensionAnyTypeBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Extended">
    <xs:simpleContent>
      <xs:extension base="xs:anyType"/>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "simpleContent extension cannot have base type anyType"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsUndeclaredNotationEnumeration(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:notation name="png" public="image/png"/>
  <xs:complexType name="Picture">
    <xs:attribute name="type">
      <xs:simpleType>
        <xs:restriction base="xs:NOTATION">
          <xs:enumeration value="jpeg"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:attribute>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "enumeration value \"jpeg\" does not reference a declared notation"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSimpleContentNotationRestrictionWithoutEnumeration(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:notation name="png" public="image/png"/>
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:NOTATION"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="Base"/>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "NOTATION restriction must have enumeration facet"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSimpleContentNotationRestrictionUndeclaredEnumeration(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:notation name="png" public="image/png"/>
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:NOTATION"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:enumeration value="jpeg"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "enumeration value \"jpeg\" does not reference a declared notation"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsSimpleContentNotationRestrictionDeclaredEnumeration(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:notation name="jpeg" public="image/jpeg"/>
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:NOTATION"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:enumeration value="jpeg"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetAllowsDerivedNotationRestrictionWithoutOwnEnumeration(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="ImageKind">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="jpeg"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:notation name="jpeg" public="image/jpeg"/>
  <xs:simpleType name="OneCharImageKind">
    <xs:restriction base="ImageKind">
      <xs:length value="1"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetSortsRuntimeNotations(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="jpeg" public="image/jpeg"/>
  <xs:notation name="mpeg" public="image/mpeg"/>
  <xs:notation name="g" public="image/gif"/>
</xs:schema>`)

	ir, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	got := make([]string, 0, len(ir.RuntimeNames.Notations))
	for _, name := range ir.RuntimeNames.Notations {
		got = append(got, name.Local)
	}
	want := []string{"g", "jpeg", "mpeg"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("runtime notations = %v, want %v", got, want)
	}
}

func TestResolveDocumentSetRejectsKeyrefFieldCountMismatch(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="uid" maxOccurs="unbounded"/>
        <xs:element ref="kid" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:keyref name="kruid" refer="kuid">
      <xs:selector xpath=".//uid"/>
      <xs:field xpath="@val"/>
      <xs:field xpath="@val2"/>
    </xs:keyref>
    <xs:key name="kuid">
      <xs:selector xpath=".//kid"/>
      <xs:field xpath="@val"/>
    </xs:key>
  </xs:element>
  <xs:element name="uid">
    <xs:complexType>
      <xs:attribute name="val" type="xs:string"/>
      <xs:attribute name="val2" type="xs:string"/>
    </xs:complexType>
  </xs:element>
  <xs:element name="kid">
    <xs:complexType>
      <xs:attribute name="val" type="xs:string"/>
      <xs:attribute name="val2" type="xs:string"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "keyref constraint \"kruid\" has 2 fields but referenced constraint \"kuid\" has 1 fields"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSimpleRestrictionBlockedByFinal(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Base" final="restriction">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:restriction base="Base"/>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "base type is final for restriction"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSimpleRestrictionAnyTypeBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad">
    <xs:restriction base="xs:anyType"/>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "simpleType restriction cannot have base type anyType"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSimpleListBlockedByFinal(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Base" final="list">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:list itemType="Base"/>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "base type is final for list"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsListItemListType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Items">
    <xs:list itemType="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="Nested">
    <xs:list itemType="Items"/>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "list itemType must be atomic or union, got list"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsRangeFacetOnUnorderedType(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Items">
    <xs:list itemType="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="Limited">
    <xs:restriction base="Items">
      <xs:maxInclusive value="100"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "only applicable to ordered types"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsInconsistentRangeFacets(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="SKU">
    <xs:restriction base="xs:int">
      <xs:minExclusive value="101"/>
      <xs:maxInclusive value="100"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "minExclusive (101) must be < maxInclusive (100)"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsConflictingRangeFacetBounds(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="SKU">
    <xs:restriction base="xs:int">
      <xs:minExclusive value="100"/>
      <xs:minInclusive value="100"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "minInclusive and minExclusive cannot both be specified"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsSimpleUnionBlockedByFinal(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Base" final="union">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="Other">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="Derived">
    <xs:union memberTypes="Base Other"/>
  </xs:simpleType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "base type is final for union"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsExtendingSimpleContentWithParticles(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:decimal">
        <xs:attribute name="code" type="xs:string"/>
      </xs:extension>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="Base">
        <xs:sequence>
          <xs:element name="value" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot extend simpleContent type Base with particles"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetRejectsMixedRestrictionOfElementOnlyBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:choice>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="b" type="xs:string"/>
    </xs:choice>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent mixed="true">
      <xs:restriction base="Base">
        <xs:sequence>
          <xs:element name="b" type="xs:string"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "cannot restrict element-only content type 'Base' to mixed content"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func TestResolveDocumentSetAllowsEmptyExtensionOfMixedBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base" mixed="true">
    <xs:sequence>
      <xs:element name="e" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="Base"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	if _, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveDocumentSetRejectsComplexContentExtensionSimpleBase(t *testing.T) {
	doc := parseDocumentForIRTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Base">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="Base">
        <xs:sequence>
          <xs:element name="value" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)

	_, err := Resolve(&schemaast.DocumentSet{Documents: []schemaast.SchemaDocument{*doc}}, ResolveConfig{})
	if err == nil {
		t.Fatal("Resolve() expected error")
	}
	if got, want := err.Error(), "complexContent extension cannot derive from simpleType 'Base'"; !strings.Contains(got, want) {
		t.Fatalf("Resolve() error = %q, want %q", got, want)
	}
}

func parseDocumentForIRTest(t testing.TB, src string) *schemaast.SchemaDocument {
	t.Helper()
	result, err := schemaast.ParseDocumentWithImportsOptions(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
	return result.Document
}

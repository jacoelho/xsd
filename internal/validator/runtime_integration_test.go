package validator

import (
	"errors"
	"strings"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestRuntimeAnyTypeAllowsAnyContent(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:foo="urn:foo" foo:attr="1">
  <foo:child/>
  <foo:child2>text</foo:child2>
</tns:root>`

	rt := mustBuildRuntimeSchema(t, schema)
	anyType := rt.Types[rt.Builtin.AnyType]
	if anyType.Kind != runtime.TypeComplex {
		t.Fatalf("anyType kind = %d, want complex", anyType.Kind)
	}
	if anyType.Complex.ID == 0 || int(anyType.Complex.ID) >= len(rt.ComplexTypes) {
		t.Fatalf("anyType complex ref missing")
	}
	ct := rt.ComplexTypes[anyType.Complex.ID]
	if ct.AnyAttr == 0 {
		t.Fatalf("anyType missing anyAttribute wildcard")
	}
	if int(ct.AnyAttr) >= len(rt.Wildcards) {
		t.Fatalf("anyType wildcard out of range")
	}
	if rt.Wildcards[ct.AnyAttr].NS.Kind != runtime.NSAny {
		t.Fatalf("anyType wildcard kind = %d, want NSAny", rt.Wildcards[ct.AnyAttr].NS.Kind)
	}

	sess := NewSession(rt)
	if err := sess.Validate(strings.NewReader(doc), nil); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeComplexContentExtensionIncludesBaseParticle(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:element name="b" type="xs:string"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test">
  <tns:a>one</tns:a>
  <tns:b>two</tns:b>
</tns:root>`

	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeAttributeWildcardInheritedOnExtension(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="child" type="xs:string"/>
    </xs:sequence>
    <xs:anyAttribute namespace="##any" processContents="skip"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:foo="urn:foo" foo:attr="1">
  <tns:child>ok</tns:child>
</tns:root>`

	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeMultipleIDAttributesInvalid(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="T">
    <xs:attribute name="a" type="xs:ID"/>
    <xs:attribute name="b" type="xs:ID"/>
  </xs:complexType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" a="x" b="y"/>`

	err := validateRuntimeDoc(t, schema, doc)
	if err == nil {
		t.Fatalf("expected multiple ID attribute error, got nil")
	}
	var violations xsderrors.ValidationList
	if !errors.As(err, &violations) {
		t.Fatalf("expected ValidationList error, got %T", err)
	}
	if !hasViolationCode([]xsderrors.Validation(violations), xsderrors.ErrMultipleIDAttr) {
		t.Fatalf("expected code %s, got %v", xsderrors.ErrMultipleIDAttr, violations)
	}
}

func TestRuntimeDefaultIDREFSInvalid(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="drefs" type="xs:IDREFS" default="abc"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	doc := `<root/>`
	err := validateRuntimeDoc(t, schema, doc)
	if err == nil {
		t.Fatalf("expected IDREFS default error, got nil")
	}
	var violations xsderrors.ValidationList
	if !errors.As(err, &violations) {
		t.Fatalf("expected ValidationList error, got %T", err)
	}
	if !hasViolationCode([]xsderrors.Validation(violations), xsderrors.ErrIDRefNotFound) {
		t.Fatalf("expected code %s, got %v", xsderrors.ErrIDRefNotFound, violations)
	}
}

func TestRuntimeAttributeGroupProhibitedIgnored(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base">
    <xs:attribute name="a"/>
  </xs:complexType>
  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="base">
        <xs:attributeGroup ref="attG"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
  <xs:attributeGroup name="attG">
    <xs:attribute name="a" use="prohibited"/>
  </xs:attributeGroup>
  <xs:element name="doc" type="derived"/>
</xs:schema>`

	doc := `<doc a="ok"/>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeEmptyChoiceRejectsEmpty(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="z">
    <xs:complexType>
      <xs:choice/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	doc := `<z/>`
	err := validateRuntimeDoc(t, schema, doc)
	if err == nil {
		t.Fatalf("expected empty choice to reject empty content")
	}
	var violations xsderrors.ValidationList
	if !errors.As(err, &violations) {
		t.Fatalf("expected ValidationList error, got %T", err)
	}
	if !hasViolationCode([]xsderrors.Validation(violations), xsderrors.ErrContentModelInvalid) {
		t.Fatalf("expected code %s, got %v", xsderrors.ErrContentModelInvalid, violations)
	}
}

func TestRuntimeXsiTypeDerivedFromBuiltin(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Derived">
    <xs:restriction base="xs:string">
      <xs:pattern value="[a-z]+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="tns:Derived">abc</tns:root>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeAnyAttributeSkipSkipsDeclaredAttributes(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:attribute name="a" type="xs:integer"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" tns:a="bra"/>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeSubstitutionGroupAnySimpleTypeEnumeration(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="item" type="xs:anySimpleType"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="item" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:element name="a" type="A" substitutionGroup="item"/>
  <xs:simpleType name="A">
    <xs:restriction base="xs:int">
      <xs:enumeration value="1"/>
      <xs:enumeration value="2"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`
	doc := `<root><a>1</a></root>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeTotalDigitsLeadingDotDecimal(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="t1" type="t1"/>
  <xs:complexType name="t1">
    <xs:attribute name="att9" use="optional">
      <xs:simpleType>
        <xs:restriction base="xs:decimal">
          <xs:totalDigits value="5"/>
          <xs:fractionDigits value="5"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:attribute>
  </xs:complexType>
</xs:schema>`

	doc := `<t1 att9=".12345"/>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

func TestRuntimeXsiTypeBlockedByBaseTypeBlock(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base" block="extension">
    <xs:sequence>
      <xs:element name="foo" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Base"/>
</xs:schema>`

	doc := `<tns:root xmlns:tns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="tns:Derived"><tns:foo>ok</tns:foo></tns:root>`
	if err := validateRuntimeDoc(t, schema, doc); err == nil {
		t.Fatalf("expected xsi:type block violation")
	}
}

func TestRuntimeIdentityFieldUnionSingleField(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tn="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice maxOccurs="unbounded">
        <xs:element name="key">
          <xs:complexType>
            <xs:attribute name="id" type="xs:decimal"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="ref" type="xs:decimal"/>
      </xs:choice>
    </xs:complexType>
    <xs:key name="k">
      <xs:selector xpath=".//tn:key"/>
      <xs:field xpath="@id|@id"/>
    </xs:key>
    <xs:keyref name="r" refer="tn:k">
      <xs:selector xpath=".//tn:ref"/>
      <xs:field xpath="."/>
    </xs:keyref>
  </xs:element>
</xs:schema>`

	doc := `<tn:root xmlns:tn="urn:test"><tn:key id="12"/><tn:ref> 12 </tn:ref></tn:root>`
	if err := validateRuntimeDoc(t, schema, doc); err != nil {
		t.Fatalf("validate runtime: %v", err)
	}
}

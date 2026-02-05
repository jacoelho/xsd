package validator

import (
	"strings"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
)

func TestIdentityUnprefixedSelectorRuntimeSemantics(t *testing.T) {
	namespacedSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:choice maxOccurs="unbounded">
          <xs:element name="item" type="xs:string" form="qualified"/>
          <xs:element name="item" type="xs:string" form="unqualified"/>
        </xs:choice>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u">
      <xs:selector xpath="item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	namespacedDoc := `<root xmlns="urn:test"><item>a</item><item>a</item></root>`
	if err := validateRuntimeDoc(t, namespacedSchema, namespacedDoc); err != nil {
		t.Fatalf("expected namespaced doc to pass, got %v", err)
	}

	noNamespaceSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u">
      <xs:selector xpath="item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	noNamespaceDoc := `<root><item>a</item><item>a</item></root>`
	err := validateRuntimeDoc(t, noNamespaceSchema, noNamespaceDoc)
	if err == nil {
		t.Fatalf("expected no-namespace doc to fail unique constraint")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrIdentityDuplicate) {
		t.Fatalf("expected ErrIdentityDuplicate, got %+v", list)
	}
}

func TestIdentitySelectorUnionNamespaceSemantics(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:choice maxOccurs="unbounded">
          <xs:element name="item" type="xs:string" form="qualified"/>
          <xs:element name="item" type="xs:string" form="unqualified"/>
        </xs:choice>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u">
      <xs:selector xpath="tns:item | item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	doc := `<root xmlns="urn:test"><item>a</item><item xmlns="">a</item></root>`
	err := validateRuntimeDoc(t, schema, doc)
	if err == nil {
		t.Fatalf("expected union selector to report duplicate")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrIdentityDuplicate) {
		t.Fatalf("expected ErrIdentityDuplicate, got %+v", list)
	}
}

func TestIdentityConstraintErrorIncludesName(t *testing.T) {
	t.Run("namespaced", func(t *testing.T) {
		schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u1">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

		doc := `<root xmlns="urn:test"><item>a</item><item>a</item></root>`
		err := validateRuntimeDoc(t, schema, doc)
		if err == nil {
			t.Fatalf("expected identity constraint error")
		}
		list := mustValidationList(t, err)
		if !strings.Contains(list[0].Message, "identity constraint {urn:test}u1") {
			t.Fatalf("expected constraint name in error, got %q", list[0].Message)
		}
	})

	t.Run("no-namespace", func(t *testing.T) {
		schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u1">
      <xs:selector xpath="item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

		doc := `<root><item>a</item><item>a</item></root>`
		err := validateRuntimeDoc(t, schema, doc)
		if err == nil {
			t.Fatalf("expected identity constraint error")
		}
		list := mustValidationList(t, err)
		if !strings.Contains(list[0].Message, "identity constraint u1") {
			t.Fatalf("expected constraint name in error, got %q", list[0].Message)
		}
	})
}

package validator

import (
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestWildcardAttributeIDREFTracking(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:wild"
           xmlns:tns="urn:wild"
           elementFormDefault="qualified">
  <xs:attribute name="idAttr" type="xs:ID"/>
  <xs:attribute name="refAttr" type="xs:IDREF"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:anyAttribute processContents="lax"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="ref">
          <xs:complexType>
            <xs:anyAttribute processContents="lax"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	t.Run("idref resolves via wildcard attribute", func(t *testing.T) {
		docXML := `<tns:root xmlns:tns="urn:wild">
  <tns:item tns:idAttr="a"/>
  <tns:ref tns:refAttr="a"/>
</tns:root>`
		violations, err := validateStreamDoc(t, schemaXML, docXML)
		if err != nil {
			t.Fatalf("ValidateStream() error = %v", err)
		}
		if len(violations) != 0 {
			t.Fatalf("expected no violations, got %v", violations)
		}
	})

	t.Run("duplicate ids via wildcard attribute", func(t *testing.T) {
		docXML := `<tns:root xmlns:tns="urn:wild">
  <tns:item tns:idAttr="dup"/>
  <tns:item tns:idAttr="dup"/>
</tns:root>`
		violations, err := validateStreamDoc(t, schemaXML, docXML)
		if err != nil {
			t.Fatalf("ValidateStream() error = %v", err)
		}
		if !hasViolationCode(violations, errors.ErrDuplicateID) {
			t.Fatalf("expected duplicate ID violation, got %v", violations)
		}
	})
}

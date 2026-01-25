package validator

import (
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestXPathUnprefixedNamesIgnoreDefaultNamespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="lax" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="u">
      <xs:selector xpath="item"/>
      <xs:field xpath="."/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test" xmlns:tns="urn:test">
  <item xmlns="">dup</item>
  <item xmlns="">dup</item>
  <tns:item>other</tns:item>
</root>`

	v := mustNewValidator(t, schemaXML)
	violations := validateStream(t, v, document)
	if !hasViolationCode(violations, errors.ErrIdentityDuplicate) {
		t.Fatalf("expected %s, got %v", errors.ErrIdentityDuplicate, violations)
	}
}

package validator

import "testing"

func TestXPathAttributeNamespaceWildcard(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           xmlns:p="urn:one"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="lax"/>
    </xs:complexType>
    <xs:unique name="u">
      <xs:selector xpath="."/>
      <xs:field xpath="@p:*"/>
    </xs:unique>
  </xs:element>
</xs:schema>`

	document := `<root xmlns="urn:test" xmlns:p="urn:one" xmlns:q="urn:two" p:a="1" q:b="2"/>`

	v := mustNewValidator(t, schemaXML)
	violations := validateStream(t, v, document)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

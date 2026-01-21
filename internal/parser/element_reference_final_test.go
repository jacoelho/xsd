package parser

import (
	"strings"
	"testing"
)

func TestElementReferenceRejectsFinal(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:example"
           xmlns:tns="urn:example"
           elementFormDefault="qualified">
  <xs:element name="child" type="xs:string"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="tns:child" final="restriction"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	if _, err := Parse(strings.NewReader(schemaXML)); err == nil {
		t.Fatalf("expected error for final on element reference")
	}
}

package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestSkipSubtreeSuppressesIDREFErrors(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
      </xs:sequence>
      <xs:attribute name="ref" type="xs:IDREF"/>
    </xs:complexType>
  </xs:element>
  <xs:element name="b">
    <xs:complexType>
      <xs:attribute name="id" type="xs:ID"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	v := mustNewValidator(t, schemaXML)
	doc := `<root xmlns="urn:test" ref="id1"><b id="id1"/></root>`

	violations, err := v.ValidateStream(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("ValidateStream error = %v", err)
	}

	for _, v := range violations {
		if v.Code == string(errors.ErrIDRefNotFound) {
			t.Fatalf("unexpected IDREF error: %v", v)
		}
	}
}

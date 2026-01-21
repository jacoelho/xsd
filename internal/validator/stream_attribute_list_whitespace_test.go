package validator

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestStreamAttributeListWhitespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
      <xs:attribute name="refs" type="xs:IDREFS"/>
      <xs:attribute name="ents" type="xs:ENTITIES"/>
      <xs:attribute name="tokens" type="xs:NMTOKENS"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docWithAttr := func(attrName, attrValue string) string {
		return fmt.Sprintf(`<root xmlns="urn:test" %s=%q><item id="a"/><item id="b"/><item id="c"/></root>`, attrName, attrValue)
	}

	attributes := []string{"refs", "ents", "tokens"}

	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "empty", value: "", valid: false},
		{name: "xml whitespace only", value: "&#x20;&#x9;&#xD;&#xA;", valid: false},
		{name: "space separated", value: "a b", valid: true},
		{name: "tab separated", value: "a&#x9;b", valid: true},
		{name: "lf separated", value: "a&#xA;b", valid: true},
		{name: "cr separated", value: "a&#xD;b", valid: true},
		{name: "crlf separated", value: "a&#xD;&#xA;b", valid: true},
		{name: "mixed separators", value: "&#x9;a&#xD;&#xA;b&#xA;c&#x9;", valid: true},
		{name: "non-xml nbsp", value: "a&#xA0;b", valid: false},
		{name: "non-xml nel", value: "a&#x85;b", valid: false},
		{name: "non-xml ls", value: "a&#x2028;b", valid: false},
		{name: "non-xml ps", value: "a&#x2029;b", valid: false},
		{name: "non-xml thin space", value: "a&#x2009;b", valid: false},
	}

	for _, attrName := range attributes {
		t.Run(attrName, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					docXML := docWithAttr(attrName, tt.value)
					violations, err := validateStreamDoc(t, schemaXML, docXML)
					if err != nil {
						t.Fatalf("ValidateStream() error = %v", err)
					}
					if tt.valid {
						if len(violations) != 0 {
							t.Fatalf("expected no violations, got %d", len(violations))
						}
						return
					}
					if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
						t.Fatalf("expected datatype violation, got %v", violations)
					}
				})
			}
		})
	}
}

package schema

import "testing"

func TestValidateUPA(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:upa"
           xmlns:tns="urn:upa"
           elementFormDefault="qualified">
  <xs:complexType name="T">
    <xs:choice>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="a" type="xs:string"/>
    </xs:choice>
  </xs:complexType>
</xs:schema>`

	schema := mustParseSchema(t, schemaXML)
	registry, err := AssignIDs(schema)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}
	if err := ValidateUPA(schema, registry); err == nil {
		t.Fatalf("expected UPA violation")
	}
}

func TestValidateUPAValidSequence(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:upaok"
           xmlns:tns="urn:upaok"
           elementFormDefault="qualified">
  <xs:complexType name="T">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="b" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	schema := mustParseSchema(t, schemaXML)
	registry, err := AssignIDs(schema)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}
	if err := ValidateUPA(schema, registry); err != nil {
		t.Fatalf("unexpected UPA error: %v", err)
	}
}

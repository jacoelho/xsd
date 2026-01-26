package validator

import "testing"

func TestUnionFixedValueNumericEquality(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="numUnion">
    <xs:union memberTypes="xs:integer xs:decimal"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:numUnion" fixed="1.0"/>
</xs:schema>`

	v := mustNewValidator(t, schemaXML)
	violations := validateStream(t, v, `<root xmlns="urn:test">1</root>`)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestUnionFixedValueNumericEqualityInverse(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="numUnion">
    <xs:union memberTypes="xs:integer xs:decimal"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:numUnion" fixed="1"/>
</xs:schema>`

	v := mustNewValidator(t, schemaXML)
	violations := validateStream(t, v, `<root xmlns="urn:test">1.0</root>`)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestUnionFixedValueNonMatchingRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="numUnion">
    <xs:union memberTypes="xs:integer xs:decimal"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:numUnion" fixed="1"/>
</xs:schema>`

	v := mustNewValidator(t, schemaXML)
	violations := validateStream(t, v, `<root xmlns="urn:test">2</root>`)
	if len(violations) == 0 {
		t.Fatalf("expected fixed-value violation")
	}
}

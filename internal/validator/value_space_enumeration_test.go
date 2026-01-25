package validator

import (
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestHexBinaryEnumerationValueSpace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="HexEnum">
    <xs:restriction base="xs:hexBinary">
      <xs:enumeration value="0a"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="HexEnum"/>
</xs:schema>`

	doc := `<root>0A</root>`

	v := mustNewValidator(t, schemaXML)
	violations := validateStream(t, v, doc)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestBase64BinaryEnumerationValueSpace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="B64Enum">
    <xs:restriction base="xs:base64Binary">
      <xs:enumeration value="AQID"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="B64Enum"/>
</xs:schema>`

	doc := `<root>A Q I D</root>`

	v := mustNewValidator(t, schemaXML)
	violations := validateStream(t, v, doc)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestDurationFixedValueValueSpace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:duration" fixed="P1D"/>
</xs:schema>`

	doc := `<root>PT24H</root>`

	v := mustNewValidator(t, schemaXML)
	violations := validateStream(t, v, doc)
	if hasViolationCode(violations, errors.ErrElementFixedValue) {
		t.Fatalf("expected fixed duration values to match, got %v", violations)
	}
}

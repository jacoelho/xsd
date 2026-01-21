package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestTimeMidnightFixedMatches24(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:time"
           xmlns:tns="urn:time"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:time" fixed="00:00:00"/>
</xs:schema>`

	docXML := `<tns:root xmlns:tns="urn:time">24:00:00</tns:root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestTimeMidnightEnumerationMatches24(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:time"
           xmlns:tns="urn:time"
           elementFormDefault="qualified">
  <xs:simpleType name="TimeEnum">
    <xs:restriction base="xs:time">
      <xs:enumeration value="00:00:00"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:TimeEnum"/>
</xs:schema>`

	docXML := `<tns:root xmlns:tns="urn:time">24:00:00</tns:root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

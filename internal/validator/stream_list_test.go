package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestStreamListEnumerationWhitespaceCollapse(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="IntList">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="IntListEnum">
    <xs:restriction base="IntList">
      <xs:enumeration value="1 2 3"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="IntListEnum"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))

	validDoc := "<root>1\t2\n3</root>"
	violations, err := v.ValidateStream(strings.NewReader(validDoc))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}

	invalidDoc := "<root>1 2 4</root>"
	violations, err = v.ValidateStream(strings.NewReader(invalidDoc))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations for enumeration mismatch")
	}
}

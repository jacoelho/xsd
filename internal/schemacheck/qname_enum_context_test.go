package schemacheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestQNameEnumerationContextMissingPrefix(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test">
  <xs:simpleType name="QNameEnum">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="p:val"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	if errs := ValidateStructure(schema); len(errs) == 0 {
		t.Fatalf("expected schema validation errors")
	}
}

func TestQNameEnumerationContextDefaultNamespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="urn:test"
           targetNamespace="urn:test">
  <xs:simpleType name="QNameEnum">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="val"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	if errs := ValidateStructure(schema); len(errs) != 0 {
		t.Fatalf("expected schema to be valid, got %v", errs)
	}
}

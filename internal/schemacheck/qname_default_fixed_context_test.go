package schemacheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestDefaultFixedQNameContextMissingPrefix(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:QName" default="p:val"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateStructure(schema)
	if len(errs) == 0 {
		t.Fatalf("expected schema validation errors")
	}
}

func TestFixedNotationContextMissingPrefix(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:notation name="note" public="pub"/>
  <xs:element name="root" type="xs:NOTATION" fixed="p:note"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateStructure(schema)
	if len(errs) == 0 {
		t.Fatalf("expected schema validation errors")
	}
}

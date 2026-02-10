package runtimeassemble

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestBuildSchemaMissingAttributeRefFails(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute ref="missingAttr"/>
  </xs:complexType>
  <xs:element name="root" type="T"/>
</xs:schema>`

	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	if _, err := buildSchemaForTest(sch, BuildConfig{}); err == nil || !strings.Contains(err.Error(), "attribute ref") {
		t.Fatalf("expected missing attribute ref error, got %v", err)
	}
}

func TestBuildSchemaMissingAttributeTypeFails(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" type="MissingType"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute ref="a"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if _, err := buildSchemaForTest(sch, BuildConfig{}); err == nil {
		t.Fatalf("expected missing attribute type to fail build")
	}
}

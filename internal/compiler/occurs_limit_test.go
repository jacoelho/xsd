package compiler

import (
	"strings"
	"testing"
)

func TestBuildSchemaOccursLimitError(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" maxOccurs="2"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	parsed := mustResolveSchema(t, schemaXML)
	_, err := buildSchemaForTest(parsed, BuildConfig{MaxOccursLimit: 1})
	if err == nil {
		t.Fatalf("expected maxOccurs limit error")
	}
	if !strings.Contains(err.Error(), "SCHEMA_OCCURS_TOO_LARGE") {
		t.Fatalf("expected SCHEMA_OCCURS_TOO_LARGE, got %v", err)
	}
}

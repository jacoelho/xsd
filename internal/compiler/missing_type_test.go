package compiler

import "testing"

func TestBuildSchemaMissingElementTypeFails(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="MissingType"/>
</xs:schema>`

	docs, err := parseDocumentSet(schemaXML)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if _, err := buildSchemaForTest(docs, BuildConfig{}); err == nil {
		t.Fatalf("expected missing element type to fail build")
	}
}

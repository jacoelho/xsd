package runtimebuild

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestBuildSchemaMissingElementTypeFails(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="MissingType"/>
</xs:schema>`

	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	sch.Phase = parser.PhaseResolved
	sch.HasPlaceholders = false
	if _, err := BuildSchema(sch, BuildConfig{}); err == nil {
		t.Fatalf("expected missing element type to fail build")
	}
}

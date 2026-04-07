package semantics

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestResolveAndValidateSchemaNil(t *testing.T) {
	validationErrs, err := ResolveAndValidateSchema(nil)
	if err == nil {
		t.Fatalf("expected nil schema error")
	}
	if len(validationErrs) != 0 {
		t.Fatalf("expected no validation errors, got %v", validationErrs)
	}
}

func TestResolveAndValidateSchemaValidSchema(t *testing.T) {
	const schemaXML = `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:ok"
           xmlns:tns="urn:ok"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	validationErrs, err := ResolveAndValidateSchema(sch)
	if err != nil {
		t.Fatalf("ResolveAndValidateSchema error = %v", err)
	}
	if len(validationErrs) != 0 {
		t.Fatalf("ResolveAndValidateSchema validation errors = %v", validationErrs)
	}
}

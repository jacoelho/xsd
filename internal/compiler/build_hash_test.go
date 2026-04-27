package compiler

import (
	"strings"
	"testing"
)

func TestBuildHashDeterministic(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	first := mustResolveSchema(t, schemaXML)
	rt1, err := buildSchemaForTest(first, BuildConfig{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	if rt1.BuildHashValue() == 0 {
		t.Fatalf("expected non-zero build hash")
	}

	second := mustResolveSchema(t, schemaXML)
	rt2, err := buildSchemaForTest(second, BuildConfig{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	if rt1.BuildHashValue() != rt2.BuildHashValue() {
		t.Fatalf("build hash mismatch: %d vs %d", rt1.BuildHashValue(), rt2.BuildHashValue())
	}

	changedXML := strings.Replace(schemaXML, "root", "root2", 1)
	third := mustResolveSchema(t, changedXML)
	rt3, err := buildSchemaForTest(third, BuildConfig{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	if rt1.BuildHashValue() == rt3.BuildHashValue() {
		t.Fatalf("expected build hash to change when schema changes")
	}
}

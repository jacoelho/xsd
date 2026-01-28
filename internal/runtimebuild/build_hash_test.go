package runtimebuild

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestBuildHashDeterministic(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:hash"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	first, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	rt1, err := BuildSchema(first, BuildConfig{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	if rt1.BuildHash == 0 {
		t.Fatalf("expected non-zero build hash")
	}

	second, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	rt2, err := BuildSchema(second, BuildConfig{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	if rt1.BuildHash != rt2.BuildHash {
		t.Fatalf("build hash mismatch: %d vs %d", rt1.BuildHash, rt2.BuildHash)
	}

	changedXML := strings.Replace(schemaXML, "root", "root2", 1)
	third, err := parser.Parse(strings.NewReader(changedXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	rt3, err := BuildSchema(third, BuildConfig{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	if rt1.BuildHash == rt3.BuildHash {
		t.Fatalf("expected build hash to change when schema changes")
	}
}

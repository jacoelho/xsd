package runtimebuild

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestDeterministicBuild(t *testing.T) {
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
	digest1 := rt1.CanonicalDigest()

	second, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	rt2, err := BuildSchema(second, BuildConfig{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	digest2 := rt2.CanonicalDigest()
	if digest1 != digest2 {
		t.Fatalf("digest mismatch")
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
	digest3 := rt3.CanonicalDigest()
	if digest1 == digest3 {
		t.Fatalf("expected digest to change when schema changes")
	}
}

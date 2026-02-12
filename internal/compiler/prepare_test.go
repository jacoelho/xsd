package compiler_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compiler"
	parser "github.com/jacoelho/xsd/internal/parser"
)

func TestPrepareParityWithPrepareOwned(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="T">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="b" type="xs:int"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`

	parsedCloned, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	parsedOwned, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse owned schema: %v", err)
	}

	preparedCloned, err := compiler.Prepare(parsedCloned)
	if err != nil {
		t.Fatalf("prepare cloned: %v", err)
	}
	preparedOwned, err := compiler.PrepareOwned(parsedOwned)
	if err != nil {
		t.Fatalf("prepare owned: %v", err)
	}

	cfg := compiler.BuildConfig{MaxDFAStates: 2048, MaxOccursLimit: 2048}
	runtimeCloned, err := preparedCloned.Build(cfg)
	if err != nil {
		t.Fatalf("build runtime cloned: %v", err)
	}
	runtimeOwned, err := preparedOwned.Build(cfg)
	if err != nil {
		t.Fatalf("build runtime owned: %v", err)
	}

	if runtimeOwned.BuildHash != runtimeCloned.BuildHash {
		t.Fatalf("build hash mismatch: owned=%x cloned=%x", runtimeOwned.BuildHash, runtimeCloned.BuildHash)
	}
	if got, want := runtimeOwned.CanonicalDigest(), runtimeCloned.CanonicalDigest(); got != want {
		t.Fatalf("canonical digest mismatch: owned=%x cloned=%x", got, want)
	}
}

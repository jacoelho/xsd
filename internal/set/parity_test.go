package set_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/objects"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/set"
)

func TestPrepareParsedParityWithPrepareParsedOwned(t *testing.T) {
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

	preparedCloned, err := set.PrepareParsed(parsedCloned)
	if err != nil {
		t.Fatalf("prepare parsed: %v", err)
	}
	preparedOwned, err := set.PrepareParsedOwned(parsedOwned)
	if err != nil {
		t.Fatalf("prepare parsed owned: %v", err)
	}

	if got, want := qNamesSignature(slices.Collect(preparedOwned.GlobalElementOrderSeq())), qNamesSignature(slices.Collect(preparedCloned.GlobalElementOrderSeq())); got != want {
		t.Fatalf("global element order mismatch: owned=%v cloned=%v", got, want)
	}

	cfg := set.CompileConfig{MaxDFAStates: 2048, MaxOccursLimit: 2048}
	runtimeCloned, err := preparedCloned.BuildRuntime(cfg)
	if err != nil {
		t.Fatalf("build runtime cloned: %v", err)
	}
	runtimeOwned, err := preparedOwned.BuildRuntime(cfg)
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

func qNamesSignature(items []objects.QName) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Namespace+"|"+item.Local)
	}
	return strings.Join(parts, ";")
}

package pipeline

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestPrepareBuildsSemanticArtifacts(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	prepared, err := Prepare(sch)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared == nil {
		t.Fatal("Prepare() returned nil")
	}
	if prepared.Schema != sch {
		t.Fatal("prepared schema pointer changed")
	}
	if prepared.Registry == nil {
		t.Fatal("prepared registry is nil")
	}
	if prepared.Refs == nil {
		t.Fatal("prepared references are nil")
	}
	if prepared.Ancestors == nil {
		t.Fatal("prepared ancestors are nil")
	}
}

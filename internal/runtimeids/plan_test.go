package runtimeids

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
	"github.com/jacoelho/xsd/internal/schemaprep"
)

func TestBuildDeterministicIDs(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:tns="urn:test">
  <xs:simpleType name="A"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:element name="root" type="tns:A"/>
  <xs:attribute name="att" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}
	if err := schemaprep.ResolveAndValidateOwned(sch); err != nil {
		t.Fatalf("ResolveAndValidateOwned() error = %v", err)
	}
	reg, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs() error = %v", err)
	}
	plan, err := Build(reg)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(plan.BuiltinTypeIDs) == 0 {
		t.Fatal("Build() builtin map is empty")
	}
	if got := plan.BuiltinTypeIDs[model.TypeNameAnyType]; got == 0 {
		t.Fatal("Build() anyType id missing")
	}
	if len(plan.TypeIDs) != len(reg.TypeOrder) {
		t.Fatalf("Build() type ids len = %d, want %d", len(plan.TypeIDs), len(reg.TypeOrder))
	}
	if len(plan.ElementIDs) != len(reg.ElementOrder) {
		t.Fatalf("Build() element ids len = %d, want %d", len(plan.ElementIDs), len(reg.ElementOrder))
	}
	if len(plan.AttributeIDs) != len(reg.AttributeOrder) {
		t.Fatalf("Build() attribute ids len = %d, want %d", len(plan.AttributeIDs), len(reg.AttributeOrder))
	}
}

func TestBuiltinTypeNamesCopies(t *testing.T) {
	first := BuiltinTypeNames()
	second := BuiltinTypeNames()
	if len(first) == 0 {
		t.Fatal("BuiltinTypeNames() empty")
	}
	if !slices.Equal(first, second) {
		t.Fatalf("BuiltinTypeNames() mismatch: %v vs %v", first, second)
	}
	first[0] = model.TypeNameString
	third := BuiltinTypeNames()
	if third[0] != model.TypeNameAnyType {
		t.Fatalf("BuiltinTypeNames() returned shared backing slice")
	}
}

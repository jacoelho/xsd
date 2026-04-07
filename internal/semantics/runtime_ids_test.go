package semantics_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

func TestBuildRuntimeIDPlanDeterministicOrdering(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="child" type="xs:string"/>
    </xs:sequence>
    <xs:attribute name="baseAttr" type="xs:int"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:element name="tail" type="xs:int"/>
        </xs:sequence>
        <xs:attribute name="derivedAttr" type="xs:string"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	reg1 := mustAssignedRegistry(t, schemaXML)
	reg2 := mustAssignedRegistry(t, schemaXML)

	plan1, err := semantics.BuildRuntimeIDPlan(reg1)
	if err != nil {
		t.Fatalf("semantics.BuildRuntimeIDPlan(reg1) error = %v", err)
	}
	plan2, err := semantics.BuildRuntimeIDPlan(reg2)
	if err != nil {
		t.Fatalf("semantics.BuildRuntimeIDPlan(reg2) error = %v", err)
	}

	if len(plan1.BuiltinTypeNames) == 0 || len(plan1.BuiltinTypeNames) != len(plan1.BuiltinTypeIDs) {
		t.Fatalf("builtin plan mismatch: names=%d ids=%d", len(plan1.BuiltinTypeNames), len(plan1.BuiltinTypeIDs))
	}
	for i, name := range plan1.BuiltinTypeNames {
		if plan1.BuiltinTypeIDs[name] != plan2.BuiltinTypeIDs[name] {
			t.Fatalf("builtin %s id mismatch: %d vs %d", name, plan1.BuiltinTypeIDs[name], plan2.BuiltinTypeIDs[name])
		}
		if i > 0 && plan1.BuiltinTypeIDs[name] <= plan1.BuiltinTypeIDs[plan1.BuiltinTypeNames[i-1]] {
			t.Fatalf("builtin order not strictly increasing at %s", name)
		}
	}

	assertTypeOrder := func(plan *semantics.RuntimeIDPlan, reg *analysis.Registry) {
		for i, entry := range reg.TypeOrder {
			got := plan.TypeIDs[entry.ID]
			if got == 0 {
				t.Fatalf("type %s missing runtime id", entry.QName)
			}
			if i > 0 && got <= plan.TypeIDs[reg.TypeOrder[i-1].ID] {
				t.Fatalf("type runtime order not increasing at %s", entry.QName)
			}
		}
	}
	assertElementOrder := func(plan *semantics.RuntimeIDPlan, reg *analysis.Registry) {
		for i, entry := range reg.ElementOrder {
			got := plan.ElementIDs[entry.ID]
			if got == 0 {
				t.Fatalf("element %s missing runtime id", entry.QName)
			}
			if i > 0 && got <= plan.ElementIDs[reg.ElementOrder[i-1].ID] {
				t.Fatalf("element runtime order not increasing at %s", entry.QName)
			}
		}
	}
	assertAttributeOrder := func(plan *semantics.RuntimeIDPlan, reg *analysis.Registry) {
		for i, entry := range reg.AttributeOrder {
			got := plan.AttributeIDs[entry.ID]
			if got == 0 {
				t.Fatalf("attribute %s missing runtime id", entry.QName)
			}
			if i > 0 && got <= plan.AttributeIDs[reg.AttributeOrder[i-1].ID] {
				t.Fatalf("attribute runtime order not increasing at %s", entry.QName)
			}
		}
	}

	assertTypeOrder(plan1, reg1)
	assertTypeOrder(plan2, reg2)
	assertElementOrder(plan1, reg1)
	assertElementOrder(plan2, reg2)
	assertAttributeOrder(plan1, reg1)
	assertAttributeOrder(plan2, reg2)
}

func mustAssignedRegistry(t *testing.T, schemaXML string) *analysis.Registry {
	t.Helper()
	prepared, err := compiler.Prepare(mustParsedSchema(t, schemaXML))
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	reg, err := analysis.AssignIDs(prepared.Schema())
	if err != nil {
		t.Fatalf("AssignIDs() error = %v", err)
	}
	return reg
}

func mustParsedSchema(t *testing.T, schemaXML string) *parser.Schema {
	t.Helper()
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return sch
}

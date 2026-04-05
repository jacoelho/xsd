package semantics_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

func TestCompileWithComplexTypePlanPreservesEffectiveSemantics(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:plan"
           targetNamespace="urn:plan"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="baseChild" type="xs:string"/>
    </xs:sequence>
    <xs:attribute name="baseAttr" type="xs:string" use="required"/>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base">
        <xs:sequence>
          <xs:element name="tail" type="xs:int"/>
        </xs:sequence>
        <xs:attribute name="extra" type="xs:int"/>
        <xs:anyAttribute processContents="lax"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="MeasureType">
    <xs:simpleContent>
      <xs:extension base="xs:double"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="LengthType">
    <xs:simpleContent>
      <xs:restriction base="tns:MeasureType"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	sch, reg, refs := mustSemanticsInputs(t, schemaXML)
	ctx, err := semantics.Build(sch, reg, refs)
	if err != nil {
		t.Fatalf("semantics.Build() error = %v", err)
	}
	complexTypes, err := ctx.ComplexTypes()
	if err != nil {
		t.Fatalf("ComplexTypes() error = %v", err)
	}
	validators, err := semantics.CompileWithComplexTypePlan(sch, reg, complexTypes)
	if err != nil {
		t.Fatalf("CompileWithComplexTypePlan() error = %v", err)
	}
	if validators.ComplexTypes != complexTypes {
		t.Fatal("compiled validators did not retain the supplied complex-type plan")
	}

	derivedType, ok := sch.TypeDefs[model.QName{Namespace: "urn:plan", Local: "Derived"}]
	if !ok {
		t.Fatal("missing Derived type")
	}
	derived, ok := model.AsComplexType(derivedType)
	if !ok || derived == nil {
		t.Fatalf("Derived type = %T, want *model.ComplexType", derivedType)
	}
	entry, ok := validators.ComplexTypes.Entry(derived)
	if !ok {
		t.Fatal("missing complex-type entry for Derived")
	}
	group, ok := entry.Content.(*model.ModelGroup)
	if !ok || len(group.Particles) != 2 {
		t.Fatalf("Derived content = %#v, want 2-particle sequence", entry.Content)
	}
	if len(entry.Attributes) != 2 {
		t.Fatalf("Derived attributes = %d, want 2", len(entry.Attributes))
	}
	if entry.Attributes[0].Name.Local != "baseAttr" || entry.Attributes[1].Name.Local != "extra" {
		t.Fatalf("Derived attribute order = [%s %s], want [baseAttr extra]", entry.Attributes[0].Name.Local, entry.Attributes[1].Name.Local)
	}
	if entry.Wildcard == nil {
		t.Fatal("Derived wildcard = nil, want extension anyAttribute")
	}

	lengthType, ok := sch.TypeDefs[model.QName{Namespace: "urn:plan", Local: "LengthType"}]
	if !ok {
		t.Fatal("missing LengthType")
	}
	lengthCT, ok := model.AsComplexType(lengthType)
	if !ok || lengthCT == nil {
		t.Fatalf("LengthType = %T, want *model.ComplexType", lengthType)
	}
	entry, ok = validators.ComplexTypes.Entry(lengthCT)
	if !ok {
		t.Fatal("missing complex-type entry for LengthType")
	}
	if entry.SimpleTextType == nil {
		t.Fatal("LengthType simple text type = nil")
	}
	primitive := entry.SimpleTextType.PrimitiveType()
	if primitive == nil || primitive.Name().Local != "double" {
		t.Fatalf("LengthType primitive text type = %v, want xs:double", primitive)
	}
}

func mustSemanticsInputs(t *testing.T, schemaXML string) (*parser.Schema, *analysis.Registry, *analysis.ResolvedReferences) {
	t.Helper()
	prepared, err := compiler.Prepare(mustSemanticsParsedSchema(t, schemaXML))
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	sch := prepared.Schema()
	reg, err := analysis.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs() error = %v", err)
	}
	refs, err := analysis.ResolveReferences(sch, reg)
	if err != nil {
		t.Fatalf("ResolveReferences() error = %v", err)
	}
	return sch, reg, refs
}

func mustSemanticsParsedSchema(t *testing.T, schemaXML string) *parser.Schema {
	t.Helper()
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return sch
}

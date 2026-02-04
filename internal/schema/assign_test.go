package schema_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
)

func mustResolveSchema(t *testing.T, schemaXML string) *parser.Schema {
	t.Helper()
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if errs := schemacheck.ValidateStructure(sch); len(errs) != 0 {
		t.Fatalf("ValidateStructure errors = %v", errs)
	}
	if err := schema.MarkSemantic(sch); err != nil {
		t.Fatalf("MarkSemantic error = %v", err)
	}
	if err := resolver.ResolveTypeReferences(sch); err != nil {
		t.Fatalf("ResolveTypeReferences error = %v", err)
	}
	if errs := resolver.ValidateReferences(sch); len(errs) != 0 {
		t.Fatalf("ValidateReferences errors = %v", errs)
	}
	parser.UpdatePlaceholderState(sch)
	if err := schema.MarkResolved(sch); err != nil {
		t.Fatalf("MarkResolved error = %v", err)
	}
	return sch
}

func findLocalElement(t *testing.T, group *types.ModelGroup, local string) *types.ElementDecl {
	t.Helper()
	for _, particle := range group.Particles {
		decl, ok := particle.(*types.ElementDecl)
		if !ok {
			continue
		}
		if decl.Name.Local == local {
			return decl
		}
	}
	t.Fatalf("local element %q not found", local)
	return nil
}

func findAttribute(t *testing.T, attrs []*types.AttributeDecl, local string) *types.AttributeDecl {
	t.Helper()
	for _, attr := range attrs {
		if attr.Name.Local == local {
			return attr
		}
	}
	t.Fatalf("attribute %q not found", local)
	return nil
}

func TestDeterministicIDs(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:ids"
           xmlns:tns="urn:ids"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="first">
          <xs:simpleType>
            <xs:restriction base="xs:string"/>
          </xs:simpleType>
        </xs:element>
        <xs:element name="second" type="xs:string"/>
      </xs:sequence>
      <xs:attribute name="attrInline">
        <xs:simpleType>
          <xs:restriction base="xs:string"/>
        </xs:simpleType>
      </xs:attribute>
    </xs:complexType>
  </xs:element>
  <xs:complexType name="T">
    <xs:sequence>
      <xs:element name="nested" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:attribute name="gAttr">
    <xs:simpleType>
      <xs:restriction base="xs:string"/>
    </xs:simpleType>
  </xs:attribute>
  <xs:attributeGroup name="AG">
    <xs:attribute name="agAttr" type="xs:string"/>
  </xs:attributeGroup>
</xs:schema>`

	sch := mustResolveSchema(t, schemaXML)
	reg, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}

	root := sch.ElementDecls[types.QName{Namespace: "urn:ids", Local: "root"}]
	if root == nil {
		t.Fatalf("root element not found")
	}
	rootCT, ok := root.Type.(*types.ComplexType)
	if !ok {
		t.Fatalf("root type = %T, want *types.ComplexType", root.Type)
	}
	rootContent, ok := rootCT.Content().(*types.ElementContent)
	if !ok {
		t.Fatalf("root content = %T, want *types.ElementContent", rootCT.Content())
	}
	rootGroup, ok := rootContent.Particle.(*types.ModelGroup)
	if !ok {
		t.Fatalf("root particle = %T, want *types.ModelGroup", rootContent.Particle)
	}
	first := findLocalElement(t, rootGroup, "first")
	second := findLocalElement(t, rootGroup, "second")
	attrInline := findAttribute(t, rootCT.Attributes(), "attrInline")

	typeQName := types.QName{Namespace: "urn:ids", Local: "T"}
	globalType, ok := sch.TypeDefs[typeQName].(*types.ComplexType)
	if !ok {
		t.Fatalf("global type T not found")
	}
	globalContent, ok := globalType.Content().(*types.ElementContent)
	if !ok {
		t.Fatalf("global type content = %T, want *types.ElementContent", globalType.Content())
	}
	globalGroup, ok := globalContent.Particle.(*types.ModelGroup)
	if !ok {
		t.Fatalf("global type particle = %T, want *types.ModelGroup", globalContent.Particle)
	}
	nested := findLocalElement(t, globalGroup, "nested")

	globalAttr := sch.AttributeDecls[types.QName{Namespace: "urn:ids", Local: "gAttr"}]
	if globalAttr == nil {
		t.Fatalf("global attribute gAttr not found")
	}
	attrGroup := sch.AttributeGroups[types.QName{Namespace: "urn:ids", Local: "AG"}]
	if attrGroup == nil {
		t.Fatalf("attributeGroup AG not found")
	}
	if len(attrGroup.Attributes) != 1 {
		t.Fatalf("attributeGroup AG attributes = %d, want 1", len(attrGroup.Attributes))
	}
	agAttr := attrGroup.Attributes[0]

	wantElements := []struct {
		decl   *types.ElementDecl
		global bool
	}{
		{root, true},
		{first, false},
		{second, false},
		{nested, false},
	}
	if len(reg.ElementOrder) != len(wantElements) {
		t.Fatalf("element order length = %d, want %d", len(reg.ElementOrder), len(wantElements))
	}
	for i, want := range wantElements {
		got := reg.ElementOrder[i]
		if got.Decl != want.decl || got.Global != want.global {
			t.Fatalf("element[%d] = (%p,%v), want (%p,%v)", i, got.Decl, got.Global, want.decl, want.global)
		}
	}

	firstType, ok := first.Type.(*types.SimpleType)
	if !ok {
		t.Fatalf("first type = %T, want *types.SimpleType", first.Type)
	}
	attrInlineType, ok := attrInline.Type.(*types.SimpleType)
	if !ok {
		t.Fatalf("attrInline type = %T, want *types.SimpleType", attrInline.Type)
	}
	globalAttrType, ok := globalAttr.Type.(*types.SimpleType)
	if !ok {
		t.Fatalf("global attribute type = %T, want *types.SimpleType", globalAttr.Type)
	}

	wantTypes := []struct {
		typ    types.Type
		global bool
	}{
		{rootCT, false},
		{firstType, false},
		{attrInlineType, false},
		{globalType, true},
		{globalAttrType, false},
	}
	if len(reg.TypeOrder) != len(wantTypes) {
		t.Fatalf("type order length = %d, want %d", len(reg.TypeOrder), len(wantTypes))
	}
	for i, want := range wantTypes {
		got := reg.TypeOrder[i]
		if got.Type != want.typ || got.Global != want.global {
			t.Fatalf("type[%d] = (%p,%v), want (%p,%v)", i, got.Type, got.Global, want.typ, want.global)
		}
	}

	wantAttrs := []struct {
		decl   *types.AttributeDecl
		global bool
	}{
		{globalAttr, true},
		{agAttr, false},
	}
	if len(reg.AttributeOrder) != len(wantAttrs) {
		t.Fatalf("attribute order length = %d, want %d", len(reg.AttributeOrder), len(wantAttrs))
	}
	for i, want := range wantAttrs {
		got := reg.AttributeOrder[i]
		if got.Decl != want.decl || got.Global != want.global {
			t.Fatalf("attribute[%d] = (%p,%v), want (%p,%v)", i, got.Decl, got.Global, want.decl, want.global)
		}
	}

	if _, ok := reg.LocalAttributes[attrInline]; ok {
		t.Fatalf("expected local attribute attrInline to be excluded from ID assignment")
	}

	reg2, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs (second) error = %v", err)
	}
	if !equalElemOrder(reg.ElementOrder, reg2.ElementOrder) {
		t.Fatalf("element order differs across runs")
	}
	if !equalTypeOrder(reg.TypeOrder, reg2.TypeOrder) {
		t.Fatalf("type order differs across runs")
	}
	if !equalAttrOrder(reg.AttributeOrder, reg2.AttributeOrder) {
		t.Fatalf("attribute order differs across runs")
	}
}

func TestAssignIDs_AllowsSharedLocalElement(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:shared"
           xmlns:tns="urn:shared"
           elementFormDefault="qualified">
  <xs:element name="A">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:element name="B">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	sch := mustResolveSchema(t, schemaXML)
	elemA := sch.ElementDecls[types.QName{Namespace: "urn:shared", Local: "A"}]
	elemB := sch.ElementDecls[types.QName{Namespace: "urn:shared", Local: "B"}]
	if elemA == nil || elemB == nil {
		t.Fatalf("elements A/B not found")
	}

	groupA := elementSequenceGroup(t, elemA)
	groupB := elementSequenceGroup(t, elemB)
	if len(groupA.Particles) == 0 || len(groupB.Particles) == 0 {
		t.Fatalf("expected non-empty model groups")
	}

	rowA := findLocalElement(t, groupA, "row")
	rowB := findLocalElement(t, groupB, "row")
	if rowA == rowB {
		t.Fatalf("expected distinct local element declarations")
	}
	groupB.Particles[0] = rowA

	if _, err := schema.AssignIDs(sch); err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}
}

func elementSequenceGroup(t *testing.T, decl *types.ElementDecl) *types.ModelGroup {
	t.Helper()
	ct, ok := decl.Type.(*types.ComplexType)
	if !ok {
		t.Fatalf("element type = %T, want *types.ComplexType", decl.Type)
	}
	content, ok := ct.Content().(*types.ElementContent)
	if !ok {
		t.Fatalf("element content = %T, want *types.ElementContent", ct.Content())
	}
	group, ok := content.Particle.(*types.ModelGroup)
	if !ok {
		t.Fatalf("element particle = %T, want *types.ModelGroup", content.Particle)
	}
	return group
}

func equalElemOrder(a, b []schema.ElementEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Decl != b[i].Decl || a[i].Global != b[i].Global {
			return false
		}
	}
	return true
}

func equalTypeOrder(a, b []schema.TypeEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Type != b[i].Type || a[i].Global != b[i].Global {
			return false
		}
	}
	return true
}

func equalAttrOrder(a, b []schema.AttributeEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Decl != b[i].Decl || a[i].Global != b[i].Global {
			return false
		}
	}
	return true
}

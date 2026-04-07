package analysis_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func mustResolveSchema(t *testing.T, schemaXML string) *parser.Schema {
	t.Helper()
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	prepared, err := compiler.Prepare(sch)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	return prepared.Schema()
}

func findLocalElement(t *testing.T, group *model.ModelGroup, local string) *model.ElementDecl {
	t.Helper()
	for _, particle := range group.Particles {
		decl, ok := particle.(*model.ElementDecl)
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

func findAttribute(t *testing.T, attrs []*model.AttributeDecl, local string) *model.AttributeDecl {
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
	reg, err := analysis.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}

	root := requireElement(t, sch, "urn:ids", "root")
	rootCT := requireComplexType(t, root.Type, "root type")
	rootGroup := requireElementContentGroup(t, rootCT.Content(), "root")
	first := findLocalElement(t, rootGroup, "first")
	second := findLocalElement(t, rootGroup, "second")
	attrInline := findAttribute(t, rootCT.Attributes(), "attrInline")

	typeQName := model.QName{Namespace: "urn:ids", Local: "T"}
	globalType := requireComplexType(t, sch.TypeDefs[typeQName], "global type T")
	globalGroup := requireElementContentGroup(t, globalType.Content(), "global type T")
	nested := findLocalElement(t, globalGroup, "nested")

	globalAttr := requireAttributeDecl(t, sch, "urn:ids", "gAttr")
	attrGroup := requireAttributeGroup(t, sch, "urn:ids", "AG")
	requireLen(t, "attributeGroup AG attributes", len(attrGroup.Attributes), 1)
	agAttr := attrGroup.Attributes[0]

	assertElementOrder(t, reg, []struct {
		decl   *model.ElementDecl
		global bool
	}{
		{root, true},
		{first, false},
		{second, false},
		{nested, false},
	})

	firstType := requireSimpleType(t, first.Type, "first type")
	attrInlineType := requireSimpleType(t, attrInline.Type, "attrInline type")
	globalAttrType := requireSimpleType(t, globalAttr.Type, "global attribute type")

	assertTypeOrder(t, reg, []struct {
		typ    model.Type
		global bool
	}{
		{rootCT, false},
		{firstType, false},
		{attrInlineType, false},
		{globalType, true},
		{globalAttrType, false},
	})

	assertAttributeOrder(t, reg, []struct {
		decl   *model.AttributeDecl
		global bool
	}{
		{globalAttr, true},
		{agAttr, false},
	})

	if _, ok := reg.LookupLocalAttributeID(attrInline); ok {
		t.Fatalf("expected local attribute attrInline to be excluded from ID assignment")
	}

	reg2, err := analysis.AssignIDs(sch)
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

func requireElement(t *testing.T, sch *parser.Schema, namespace, local string) *model.ElementDecl {
	t.Helper()
	decl := sch.ElementDecls[model.QName{Namespace: namespace, Local: local}]
	if decl == nil {
		t.Fatalf("element %s not found", local)
	}
	return decl
}

func requireAttributeDecl(t *testing.T, sch *parser.Schema, namespace, local string) *model.AttributeDecl {
	t.Helper()
	decl := sch.AttributeDecls[model.QName{Namespace: namespace, Local: local}]
	if decl == nil {
		t.Fatalf("attribute %s not found", local)
	}
	return decl
}

func requireAttributeGroup(t *testing.T, sch *parser.Schema, namespace, local string) *model.AttributeGroup {
	t.Helper()
	group := sch.AttributeGroups[model.QName{Namespace: namespace, Local: local}]
	if group == nil {
		t.Fatalf("attributeGroup %s not found", local)
	}
	return group
}

func requireComplexType(t *testing.T, typ model.Type, label string) *model.ComplexType {
	t.Helper()
	ct, ok := typ.(*model.ComplexType)
	if !ok {
		t.Fatalf("%s = %T, want *model.ComplexType", label, typ)
	}
	return ct
}

func requireSimpleType(t *testing.T, typ model.Type, label string) *model.SimpleType {
	t.Helper()
	st, ok := typ.(*model.SimpleType)
	if !ok {
		t.Fatalf("%s = %T, want *model.SimpleType", label, typ)
	}
	return st
}

func requireElementContentGroup(t *testing.T, content model.Content, label string) *model.ModelGroup {
	t.Helper()
	elementContent, ok := content.(*model.ElementContent)
	if !ok {
		t.Fatalf("%s content = %T, want *model.ElementContent", label, content)
	}
	group, ok := elementContent.Particle.(*model.ModelGroup)
	if !ok {
		t.Fatalf("%s particle = %T, want *model.ModelGroup", label, elementContent.Particle)
	}
	return group
}

func requireLen(t *testing.T, label string, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %d, want %d", label, got, want)
	}
}

func assertElementOrder(t *testing.T, reg *analysis.Registry, want []struct {
	decl   *model.ElementDecl
	global bool
}) {
	t.Helper()
	requireLen(t, "element order length", len(reg.ElementOrder), len(want))
	for i, item := range want {
		got := reg.ElementOrder[i]
		if got.Decl != item.decl || got.Global != item.global {
			t.Fatalf("element[%d] = (%p,%v), want (%p,%v)", i, got.Decl, got.Global, item.decl, item.global)
		}
	}
}

func assertTypeOrder(t *testing.T, reg *analysis.Registry, want []struct {
	typ    model.Type
	global bool
}) {
	t.Helper()
	requireLen(t, "type order length", len(reg.TypeOrder), len(want))
	for i, item := range want {
		got := reg.TypeOrder[i]
		if got.Type != item.typ || got.Global != item.global {
			t.Fatalf("type[%d] = (%p,%v), want (%p,%v)", i, got.Type, got.Global, item.typ, item.global)
		}
	}
}

func assertAttributeOrder(t *testing.T, reg *analysis.Registry, want []struct {
	decl   *model.AttributeDecl
	global bool
}) {
	t.Helper()
	requireLen(t, "attribute order length", len(reg.AttributeOrder), len(want))
	for i, item := range want {
		got := reg.AttributeOrder[i]
		if got.Decl != item.decl || got.Global != item.global {
			t.Fatalf("attribute[%d] = (%p,%v), want (%p,%v)", i, got.Decl, got.Global, item.decl, item.global)
		}
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
	elemA := sch.ElementDecls[model.QName{Namespace: "urn:shared", Local: "A"}]
	elemB := sch.ElementDecls[model.QName{Namespace: "urn:shared", Local: "B"}]
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

	if _, err := analysis.AssignIDs(sch); err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}
}

func elementSequenceGroup(t *testing.T, decl *model.ElementDecl) *model.ModelGroup {
	t.Helper()
	ct, ok := decl.Type.(*model.ComplexType)
	if !ok {
		t.Fatalf("element type = %T, want *model.ComplexType", decl.Type)
	}
	content, ok := ct.Content().(*model.ElementContent)
	if !ok {
		t.Fatalf("element content = %T, want *model.ElementContent", ct.Content())
	}
	group, ok := content.Particle.(*model.ModelGroup)
	if !ok {
		t.Fatalf("element particle = %T, want *model.ModelGroup", content.Particle)
	}
	return group
}

func equalElemOrder(a, b []analysis.ElementEntry) bool {
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

func equalTypeOrder(a, b []analysis.TypeEntry) bool {
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

func equalAttrOrder(a, b []analysis.AttributeEntry) bool {
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

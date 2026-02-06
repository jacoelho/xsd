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

func findElementRef(t *testing.T, group *types.ModelGroup) *types.ElementDecl {
	t.Helper()
	for _, particle := range group.Particles {
		decl, ok := particle.(*types.ElementDecl)
		if !ok {
			continue
		}
		if decl.IsReference {
			return decl
		}
	}
	t.Fatalf("element reference not found")
	return nil
}

func findGroupRef(t *testing.T, group *types.ModelGroup) *types.GroupRef {
	t.Helper()
	for _, particle := range group.Particles {
		ref, ok := particle.(*types.GroupRef)
		if ok {
			return ref
		}
	}
	t.Fatalf("group reference not found")
	return nil
}

func findAttributeRef(t *testing.T, attrs []*types.AttributeDecl) *types.AttributeDecl {
	t.Helper()
	for _, attr := range attrs {
		if attr.IsReference {
			return attr
		}
	}
	t.Fatalf("attribute reference not found")
	return nil
}

func TestReferenceResolution(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:ref"
           xmlns:tns="urn:ref"
           elementFormDefault="qualified">
  <xs:element name="root" type="tns:T"/>
  <xs:complexType name="T">
    <xs:sequence>
      <xs:group ref="tns:G"/>
      <xs:element ref="tns:leaf"/>
    </xs:sequence>
    <xs:attribute ref="tns:ga"/>
    <xs:attributeGroup ref="tns:AG"/>
  </xs:complexType>
  <xs:element name="leaf" type="xs:string"/>
  <xs:attribute name="ga" type="xs:string"/>
  <xs:attributeGroup name="AG">
    <xs:attribute name="agAttr" type="xs:string"/>
  </xs:attributeGroup>
  <xs:group name="G">
    <xs:sequence>
      <xs:element name="inside" type="xs:string"/>
    </xs:sequence>
  </xs:group>
</xs:schema>`

	sch := mustResolveSchema(t, schemaXML)
	registry, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}

	refs, err := schema.ResolveReferences(sch, registry)
	if err != nil {
		t.Fatalf("ResolveReferences error = %v", err)
	}

	rootQName := types.QName{Namespace: "urn:ref", Local: "root"}
	root := sch.ElementDecls[rootQName]
	if root == nil {
		t.Fatalf("root element not found")
	}
	if _, ok := root.Type.(*types.ComplexType); !ok {
		t.Fatalf("root type = %T, want *types.ComplexType", root.Type)
	}

	ctQName := types.QName{Namespace: "urn:ref", Local: "T"}
	ct, ok := sch.TypeDefs[ctQName].(*types.ComplexType)
	if !ok {
		t.Fatalf("type T not found")
	}
	content, ok := ct.Content().(*types.ElementContent)
	if !ok {
		t.Fatalf("type T content = %T, want *types.ElementContent", ct.Content())
	}
	group, ok := content.Particle.(*types.ModelGroup)
	if !ok {
		t.Fatalf("type T particle = %T, want *types.ModelGroup", content.Particle)
	}

	elemRef := findElementRef(t, group)
	groupRef := findGroupRef(t, group)
	attrRef := findAttributeRef(t, ct.Attributes())

	leafID := registry.Elements[types.QName{Namespace: "urn:ref", Local: "leaf"}]
	if refs.ElementRefs[elemRef] != leafID {
		t.Fatalf("element ref ID = %d, want %d", refs.ElementRefs[elemRef], leafID)
	}

	attrID := registry.Attributes[types.QName{Namespace: "urn:ref", Local: "ga"}]
	if refs.AttributeRefs[attrRef] != attrID {
		t.Fatalf("attribute ref ID = %d, want %d", refs.AttributeRefs[attrRef], attrID)
	}

	groupQName := types.QName{Namespace: "urn:ref", Local: "G"}
	if refs.GroupRefs[groupRef] != sch.Groups[groupQName] {
		t.Fatalf("group ref resolved to unexpected target")
	}
}

func TestReferenceResolutionMissing(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:missing"
           xmlns:tns="urn:missing"
           elementFormDefault="qualified">
  <xs:element name="root" type="tns:Missing"/>
</xs:schema>`

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
	if errs := resolver.ValidateReferences(sch); len(errs) == 0 {
		t.Fatalf("expected missing type to fail reference validation")
	}
}

func TestReferenceResolutionRecursiveType(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:recursive"
           xmlns:tns="urn:recursive">
  <xs:element name="node" type="tns:NodeType"/>
  <xs:complexType name="NodeType">
    <xs:sequence>
      <xs:element name="node" type="tns:NodeType" minOccurs="0"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	sch := mustResolveSchema(t, schemaXML)
	registry, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}

	if _, err := schema.ResolveReferences(sch, registry); err != nil {
		t.Fatalf("ResolveReferences error = %v", err)
	}
}

func TestReferenceResolutionMissingSubstitutionGroupHead(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:missing"
           xmlns:tns="urn:missing"
           elementFormDefault="qualified">
  <xs:element name="member" type="xs:string" substitutionGroup="tns:missingHead"/>
</xs:schema>`

	sch := mustParsedResolved(t, schemaXML)
	registry, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}
	if _, err := schema.ResolveReferences(sch, registry); err == nil {
		t.Fatalf("expected missing substitutionGroup head to fail reference resolution")
	}
}

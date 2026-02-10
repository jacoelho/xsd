package semantic_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	schema "github.com/jacoelho/xsd/internal/semantic"
	schemacheck "github.com/jacoelho/xsd/internal/semanticcheck"
	resolver "github.com/jacoelho/xsd/internal/semanticresolve"
)

func findElementRef(t *testing.T, group *model.ModelGroup) *model.ElementDecl {
	t.Helper()
	for _, particle := range group.Particles {
		decl, ok := particle.(*model.ElementDecl)
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

func findGroupRef(t *testing.T, group *model.ModelGroup) *model.GroupRef {
	t.Helper()
	for _, particle := range group.Particles {
		ref, ok := particle.(*model.GroupRef)
		if ok {
			return ref
		}
	}
	t.Fatalf("group reference not found")
	return nil
}

func findAttributeRef(t *testing.T, attrs []*model.AttributeDecl) *model.AttributeDecl {
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

	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if errs := schemacheck.ValidateStructure(sch); len(errs) != 0 {
		t.Fatalf("ValidateStructure errors = %v", errs)
	}
	if err := resolver.NewResolver(sch).Resolve(); err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if errs := resolver.ValidateReferences(sch); len(errs) != 0 {
		t.Fatalf("ValidateReferences errors = %v", errs)
	}
	registry, err := schema.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs error = %v", err)
	}

	refs, err := schema.ResolveReferences(sch, registry)
	if err != nil {
		t.Fatalf("ResolveReferences error = %v", err)
	}

	rootQName := model.QName{Namespace: "urn:ref", Local: "root"}
	root := sch.ElementDecls[rootQName]
	if root == nil {
		t.Fatalf("root element not found")
	}
	if _, ok := root.Type.(*model.ComplexType); !ok {
		t.Fatalf("root type = %T, want *model.ComplexType", root.Type)
	}

	ctQName := model.QName{Namespace: "urn:ref", Local: "T"}
	ct, ok := sch.TypeDefs[ctQName].(*model.ComplexType)
	if !ok {
		t.Fatalf("type T not found")
	}
	content, ok := ct.Content().(*model.ElementContent)
	if !ok {
		t.Fatalf("type T content = %T, want *model.ElementContent", ct.Content())
	}
	group, ok := content.Particle.(*model.ModelGroup)
	if !ok {
		t.Fatalf("type T particle = %T, want *model.ModelGroup", content.Particle)
	}

	elemRef := findElementRef(t, group)
	groupRef := findGroupRef(t, group)
	attrRef := findAttributeRef(t, ct.Attributes())

	leafID := registry.Elements[model.QName{Namespace: "urn:ref", Local: "leaf"}]
	if refs.ElementRefs[elemRef.Name] != leafID {
		t.Fatalf("element ref ID = %d, want %d", refs.ElementRefs[elemRef.Name], leafID)
	}

	attrID := registry.Attributes[model.QName{Namespace: "urn:ref", Local: "ga"}]
	if refs.AttributeRefs[attrRef.Name] != attrID {
		t.Fatalf("attribute ref ID = %d, want %d", refs.AttributeRefs[attrRef.Name], attrID)
	}

	groupQName := model.QName{Namespace: "urn:ref", Local: "G"}
	if refs.GroupRefs[groupRef.RefQName] != groupQName {
		t.Fatalf("group ref resolved to unexpected target: %s", refs.GroupRefs[groupRef.RefQName])
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

func TestResolvedReferencesStableAcrossEquivalentClones(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:stable"
           xmlns:tns="urn:stable"
           elementFormDefault="qualified">
  <xs:element name="root" type="tns:T"/>
  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="tns:leaf"/>
    </xs:sequence>
    <xs:attribute ref="tns:a"/>
  </xs:complexType>
  <xs:element name="leaf" type="xs:string"/>
  <xs:attribute name="a" type="xs:string"/>
  <xs:group name="G">
    <xs:sequence>
      <xs:element name="inside" type="xs:string"/>
    </xs:sequence>
  </xs:group>
</xs:schema>`

	original := mustResolveSchema(t, schemaXML)
	regA, err := schema.AssignIDs(original)
	if err != nil {
		t.Fatalf("AssignIDs(original) error = %v", err)
	}
	refsA, err := schema.ResolveReferences(original, regA)
	if err != nil {
		t.Fatalf("ResolveReferences(original) error = %v", err)
	}

	cloned, err := loadmerge.CloneSchemaDeep(original)
	if err != nil {
		t.Fatalf("CloneSchemaDeep() error = %v", err)
	}
	regB, err := schema.AssignIDs(cloned)
	if err != nil {
		t.Fatalf("AssignIDs(clone) error = %v", err)
	}
	refsB, err := schema.ResolveReferences(cloned, regB)
	if err != nil {
		t.Fatalf("ResolveReferences(clone) error = %v", err)
	}
	if !reflect.DeepEqual(refsA, refsB) {
		t.Fatalf("resolved references mismatch across equivalent clones:\nA=%+v\nB=%+v", refsA, refsB)
	}
}

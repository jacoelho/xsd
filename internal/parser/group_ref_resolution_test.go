package parser

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestGroupRefUsesDefaultNamespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:group name="string">
    <xs:sequence>
      <xs:element name="e" type="xs:string"/>
    </xs:sequence>
  </xs:group>
  <xs:complexType name="ct">
    <xs:sequence>
      <xs:group ref="string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	ct, ok := schema.TypeDefs[model.QName{Namespace: "urn:t", Local: "ct"}].(*model.ComplexType)
	if !ok {
		t.Fatalf("complexType ct not found")
	}

	content, ok := ct.Content().(*model.ElementContent)
	if !ok || content.Particle == nil {
		t.Fatalf("expected element content with particle")
	}

	var refQName model.QName
	err = model.WalkParticles([]model.Particle{content.Particle}, model.ParticleHandlers{
		OnGroupRef: func(gr *model.GroupRef) error {
			refQName = gr.RefQName
			return nil
		},
	})
	if err != nil {
		t.Fatalf("walk particles: %v", err)
	}
	if refQName.Namespace != "urn:t" || refQName.Local != "string" {
		t.Fatalf("group ref QName = %v, want {urn:t}string", refQName)
	}
}

func TestAttributeGroupRefUsesDefaultNamespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="urn:t"
           targetNamespace="urn:t"
           elementFormDefault="qualified">
  <xs:attributeGroup name="string">
    <xs:attribute name="a" type="xs:string"/>
  </xs:attributeGroup>
  <xs:complexType name="ct">
    <xs:attributeGroup ref="string"/>
  </xs:complexType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	ct, ok := schema.TypeDefs[model.QName{Namespace: "urn:t", Local: "ct"}].(*model.ComplexType)
	if !ok {
		t.Fatalf("complexType ct not found")
	}
	if len(ct.AttrGroups) != 1 {
		t.Fatalf("expected one attributeGroup ref, got %d", len(ct.AttrGroups))
	}
	refQName := ct.AttrGroups[0]
	if refQName.Namespace != "urn:t" || refQName.Local != "string" {
		t.Fatalf("attributeGroup ref QName = %v, want {urn:t}string", refQName)
	}
}

package semanticcheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestAttributeUseProhibitedDisallowsDefault(t *testing.T) {
	qname := model.QName{Namespace: "urn:test", Local: "a"}
	decl := &model.AttributeDecl{
		Name:       qname,
		Use:        model.Prohibited,
		HasDefault: true,
		Default:    "d",
	}
	schema := &parser.Schema{
		TargetNamespace: "urn:test",
		AttributeDecls: map[model.QName]*model.AttributeDecl{
			qname: decl,
		},
	}

	errs := ValidateStructure(schema)
	if len(errs) == 0 {
		t.Fatalf("expected schema validation errors")
	}
	if !strings.Contains(errs[0].Error(), "use='prohibited'") {
		t.Fatalf("expected prohibited-use error, got %v", errs[0])
	}
}

func TestAttributeUseProhibitedAllowsFixed(t *testing.T) {
	qname := model.QName{Namespace: "urn:test", Local: "a"}
	decl := &model.AttributeDecl{
		Name:     qname,
		Use:      model.Prohibited,
		HasFixed: true,
		Fixed:    "x",
	}
	schema := &parser.Schema{
		TargetNamespace: "urn:test",
		AttributeDecls: map[model.QName]*model.AttributeDecl{
			qname: decl,
		},
	}

	if errs := ValidateStructure(schema); len(errs) != 0 {
		t.Fatalf("expected schema to be valid, got %v", errs)
	}
}

func TestMergeAttributesFromTypeForValidationNilContent(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:test"

	attr := &model.AttributeDecl{
		Name: model.QName{Local: "a"},
	}
	ct := &model.ComplexType{}
	ct.SetAttributes([]*model.AttributeDecl{attr})

	attrMap := make(map[model.QName]*model.AttributeDecl)
	mergeAttributesFromTypeForValidation(schema, ct, attrMap)

	if len(attrMap) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(attrMap))
	}
}

func TestAttributeDefaultQNameContextMissingPrefix(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test">
  <xs:attribute name="attr" type="xs:QName" default="p:val"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateStructure(schema)
	if len(errs) == 0 {
		t.Fatalf("expected schema validation errors")
	}
}

func TestAttributeDefaultIDDerivedTypeRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test">
  <xs:simpleType name="myID">
    <xs:restriction base="xs:ID"/>
  </xs:simpleType>
  <xs:attribute name="attr" type="tns:myID" default="x"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateStructure(schema)
	if len(errs) == 0 {
		t.Fatalf("expected schema validation errors")
	}
}

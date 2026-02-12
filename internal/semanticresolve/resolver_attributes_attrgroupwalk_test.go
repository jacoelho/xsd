package semanticresolve

import (
	"strings"
	"testing"

	parser "github.com/jacoelho/xsd/internal/parser"
	model "github.com/jacoelho/xsd/internal/types"
)

func TestResolverAttributeGroupCycleReturnsCycleError(t *testing.T) {
	schema := parser.NewSchema()
	a := model.QName{Namespace: "urn:test", Local: "A"}
	b := model.QName{Namespace: "urn:test", Local: "B"}

	schema.AttributeGroups[a] = &model.AttributeGroup{Name: a, AttrGroups: []model.QName{b}}
	schema.AttributeGroups[b] = &model.AttributeGroup{Name: b, AttrGroups: []model.QName{a}}

	err := NewResolver(schema).Resolve()
	if err == nil {
		t.Fatalf("expected cycle error")
	}
	if !IsCycleError(err) {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestResolverComplexTypeReportsMissingNestedAttributeGroup(t *testing.T) {
	schema := parser.NewSchema()
	typeQName := model.QName{Namespace: "urn:test", Local: "T"}
	rootGroup := model.QName{Namespace: "urn:test", Local: "AG"}
	missingGroup := model.QName{Namespace: "urn:test", Local: "Missing"}

	ct := model.NewComplexType(typeQName, "urn:test")
	ct.SetContent(&model.EmptyContent{})
	ct.AttrGroups = []model.QName{rootGroup}
	schema.TypeDefs[typeQName] = ct
	schema.AttributeGroups[rootGroup] = &model.AttributeGroup{
		Name:       rootGroup,
		AttrGroups: []model.QName{missingGroup},
	}

	err := NewResolver(schema).Resolve()
	if err == nil {
		t.Fatalf("expected missing attributeGroup error")
	}
	if !strings.Contains(err.Error(), rootGroup.String()) {
		t.Fatalf("error %q does not mention root group %s", err, rootGroup)
	}
	if !strings.Contains(err.Error(), missingGroup.String()) {
		t.Fatalf("error %q does not mention missing group %s", err, missingGroup)
	}
}

func TestValidateReferencesAttributeGroupCycle(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test">
  <xs:attributeGroup name="A">
    <xs:attributeGroup ref="tns:B"/>
  </xs:attributeGroup>
  <xs:attributeGroup name="B">
    <xs:attributeGroup ref="tns:A"/>
  </xs:attributeGroup>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	errs := ValidateReferences(schema)
	if len(errs) == 0 {
		t.Fatalf("expected cycle validation error")
	}
	if !containsCycleError(errs) {
		t.Fatalf("expected cycle error in %v", errs)
	}
}

func containsCycleError(errs []error) bool {
	for _, err := range errs {
		if err != nil && IsCycleError(err) {
			return true
		}
	}
	return false
}

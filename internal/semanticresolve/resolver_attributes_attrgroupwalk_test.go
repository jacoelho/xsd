package semanticresolve

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestResolverAttributeGroupCycleReturnsCycleError(t *testing.T) {
	schema := parser.NewSchema()
	a := types.QName{Namespace: "urn:test", Local: "A"}
	b := types.QName{Namespace: "urn:test", Local: "B"}

	schema.AttributeGroups[a] = &types.AttributeGroup{Name: a, AttrGroups: []types.QName{b}}
	schema.AttributeGroups[b] = &types.AttributeGroup{Name: b, AttrGroups: []types.QName{a}}

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
	typeQName := types.QName{Namespace: "urn:test", Local: "T"}
	rootGroup := types.QName{Namespace: "urn:test", Local: "AG"}
	missingGroup := types.QName{Namespace: "urn:test", Local: "Missing"}

	ct := types.NewComplexType(typeQName, "urn:test")
	ct.SetContent(&types.EmptyContent{})
	ct.AttrGroups = []types.QName{rootGroup}
	schema.TypeDefs[typeQName] = ct
	schema.AttributeGroups[rootGroup] = &types.AttributeGroup{
		Name:       rootGroup,
		AttrGroups: []types.QName{missingGroup},
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

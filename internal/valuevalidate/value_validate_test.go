package valuevalidate_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemaprep"
	"github.com/jacoelho/xsd/internal/valuevalidate"
)

func TestValidateDefaultOrFixedResolvedUnionAllowsIDMember(t *testing.T) {
	schema := mustResolvedSchema(t, `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" fixed="a">
    <xs:simpleType>
      <xs:union memberTypes="xs:gYearMonth xs:ID xs:long"/>
    </xs:simpleType>
  </xs:element>
</xs:schema>`)

	root := schema.ElementDecls[model.QName{Local: "root"}]
	if root == nil {
		t.Fatal("missing root element")
	}
	if err := valuevalidate.ValidateDefaultOrFixedResolved(schema, root.Fixed, root.Type, root.FixedContext, valuevalidate.IDPolicyDisallow); err != nil {
		t.Fatalf("ValidateDefaultOrFixedResolved() error = %v", err)
	}
}

func TestValidateDefaultOrFixedResolvedDisallowsDerivedID(t *testing.T) {
	schema := mustResolvedSchema(t, `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test">
  <xs:simpleType name="MyID">
    <xs:restriction base="xs:ID"/>
  </xs:simpleType>
</xs:schema>`)

	typ, ok := schema.TypeDefs[model.QName{Namespace: "urn:test", Local: "MyID"}]
	if !ok {
		t.Fatal("missing MyID type")
	}
	err := valuevalidate.ValidateDefaultOrFixedResolved(schema, "abc", typ, nil, valuevalidate.IDPolicyDisallow)
	if err == nil {
		t.Fatal("ValidateDefaultOrFixedResolved() expected error")
	}
}

func TestValidateDefaultOrFixedResolvedRejectsListItemBuiltinID(t *testing.T) {
	list, err := model.NewListSimpleType(model.QName{Namespace: "urn:test", Local: "IDs"}, "urn:test", &model.ListType{
		ItemType: model.QName{Namespace: model.XSDNamespace, Local: "ID"},
	}, nil)
	if err != nil {
		t.Fatalf("NewListSimpleType() error = %v", err)
	}

	err = valuevalidate.ValidateDefaultOrFixedResolved(nil, "abc", list, nil, valuevalidate.IDPolicyDisallow)
	if err == nil || !strings.Contains(err.Error(), "cannot have default or fixed values") {
		t.Fatalf("ValidateDefaultOrFixedResolved() error = %v, want ID policy error", err)
	}
}

func TestValidateDefaultOrFixedResolvedRejectsPlaceholderUnionMember(t *testing.T) {
	missing := model.NewPlaceholderSimpleType(model.QName{Namespace: "urn:test", Local: "MissingType"})
	union := &model.SimpleType{
		QName:       model.QName{Namespace: "urn:test", Local: "BrokenUnion"},
		Union:       &model.UnionType{},
		MemberTypes: []model.Type{missing},
	}

	err := valuevalidate.ValidateDefaultOrFixedResolved(nil, "abc", union, nil, valuevalidate.IDPolicyDisallow)
	if err == nil || !strings.Contains(err.Error(), "not resolved") {
		t.Fatalf("ValidateDefaultOrFixedResolved() error = %v, want unresolved type error", err)
	}
}

func TestValidateWithFacetsAllowsPlaceholderUnionMember(t *testing.T) {
	missing := model.NewPlaceholderSimpleType(model.QName{Namespace: "urn:test", Local: "MissingType"})
	union := &model.SimpleType{
		QName:       model.QName{Namespace: "urn:test", Local: "BrokenUnion"},
		Union:       &model.UnionType{},
		MemberTypes: []model.Type{missing},
	}

	if err := valuevalidate.ValidateWithFacets(nil, "abc", union, nil, nil); err != nil {
		t.Fatalf("ValidateWithFacets() error = %v, want nil", err)
	}
}

func TestValidateWithFacetsRequiresQNameContext(t *testing.T) {
	qnameType := builtins.Get(builtins.TypeNameQName)
	if qnameType == nil {
		t.Fatal("missing QName builtin")
	}
	err := valuevalidate.ValidateWithFacets(nil, "p:name", qnameType, nil, nil)
	if err == nil {
		t.Fatal("ValidateWithFacets() expected QName context error")
	}
}

func TestValidateDefaultOrFixedResolvedRejectsDerivedQNameWithUnboundPrefix(t *testing.T) {
	typ, err := model.NewAtomicSimpleType(model.QName{Namespace: "urn:test", Local: "QNameAlias"}, "urn:test", &model.Restriction{
		Base: model.QName{Namespace: model.XSDNamespace, Local: "QName"},
	})
	if err != nil {
		t.Fatalf("NewAtomicSimpleType() error = %v", err)
	}

	err = valuevalidate.ValidateDefaultOrFixedResolved(nil, "p:name", typ, nil, valuevalidate.IDPolicyDisallow)
	if err == nil || !strings.Contains(err.Error(), "prefix p not found") {
		t.Fatalf("ValidateDefaultOrFixedResolved() error = %v, want unbound prefix error", err)
	}
}

func TestValidateDefaultOrFixedResolvedRejectsEmptyUnion(t *testing.T) {
	union := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "BrokenUnion"},
		Union: &model.UnionType{},
	}

	err := valuevalidate.ValidateDefaultOrFixedResolved(nil, "abc", union, nil, valuevalidate.IDPolicyDisallow)
	if err == nil || !strings.Contains(err.Error(), "union has no member types") {
		t.Fatalf("ValidateDefaultOrFixedResolved() error = %v, want union member error", err)
	}
}

func TestValidateWithFacetsRejectsEmptyUnion(t *testing.T) {
	union := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "BrokenUnion"},
		Union: &model.UnionType{},
	}

	err := valuevalidate.ValidateWithFacets(nil, "abc", union, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "union has no member types") {
		t.Fatalf("ValidateWithFacets() error = %v, want union member error", err)
	}
}

func TestValidateDefaultOrFixedResolvedRejectsMissingListItemType(t *testing.T) {
	list := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "BrokenList"},
		List:  &model.ListType{},
	}

	err := valuevalidate.ValidateDefaultOrFixedResolved(nil, "1 2", list, nil, valuevalidate.IDPolicyDisallow)
	if err == nil || !strings.Contains(err.Error(), "list item type is missing") {
		t.Fatalf("ValidateDefaultOrFixedResolved() error = %v, want list item error", err)
	}
}

func TestValidateWithFacetsRejectsMissingListItemType(t *testing.T) {
	list := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "BrokenList"},
		List:  &model.ListType{},
	}

	err := valuevalidate.ValidateWithFacets(nil, "1 2", list, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "list item type is missing") {
		t.Fatalf("ValidateWithFacets() error = %v, want list item error", err)
	}
}

func mustResolvedSchema(t *testing.T, schemaXML string) *parser.Schema {
	t.Helper()
	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	resolved, err := schemaprep.ResolveAndValidate(sch)
	if err != nil {
		t.Fatalf("ResolveAndValidate() error = %v", err)
	}
	return resolved
}

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

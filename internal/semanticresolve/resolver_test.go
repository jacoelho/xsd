package semanticresolve

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	model "github.com/jacoelho/xsd/internal/types"
)

var w3cSchemaFixtures = map[string]string{
	"sunData/combined/xsd024/xsd024.xsdmod": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:xsd024"
           xmlns:tns="urn:xsd024"
           elementFormDefault="qualified">
  <xs:group name="g">
    <xs:sequence>
      <xs:element name="value" type="xs:string"/>
    </xs:sequence>
  </xs:group>
  <xs:attributeGroup name="attrs">
    <xs:attribute name="code" type="xs:string"/>
  </xs:attributeGroup>
  <xs:complexType name="complexType">
    <xs:sequence>
      <xs:group ref="tns:g"/>
    </xs:sequence>
    <xs:attributeGroup ref="tns:attrs"/>
  </xs:complexType>
</xs:schema>`,
	"sunData/CType/pSubstitutions/pSubstitutions00101m/pSubstitutions00101m.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:substitutions"
           xmlns:tns="urn:substitutions">
  <xs:complexType name="A"/>
  <xs:complexType name="B">
    <xs:complexContent>
      <xs:extension base="tns:A"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="C">
    <xs:complexContent>
      <xs:extension base="tns:A"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`,
	"saxonData/Complex/unique001.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:unique"
           xmlns:tns="urn:unique"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="id" type="xs:string"/>
    </xs:complexType>
    <xs:unique name="test">
      <xs:selector xpath="."/>
      <xs:field xpath="@id"/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
	"saxonData/Missing/missing006.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:missing-list"
           xmlns:tns="urn:missing-list">
  <xs:simpleType name="brokenList">
    <xs:list itemType="tns:missingItem"/>
  </xs:simpleType>
</xs:schema>`,
	"saxonData/Missing/missing004.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:missing-base"
           xmlns:tns="urn:missing-base">
  <xs:simpleType name="brokenRestriction">
    <xs:restriction base="tns:missingBase"/>
  </xs:simpleType>
</xs:schema>`,
	"sunData/combined/xsd010/xsd010.e.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:subst-cycle"
           xmlns:tns="urn:subst-cycle">
  <xs:element name="A" type="xs:string" abstract="true" substitutionGroup="tns:B"/>
  <xs:element name="B" type="xs:string" abstract="true" substitutionGroup="tns:A"/>
</xs:schema>`,
	"sunData/IdConstrDefs/name/name00101m/name00101m2.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:dup-id"
           xmlns:tns="urn:dup-id"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="duplicate">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:key name="duplicate">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
	"sunData/AttrDecl/AD_valConstr/AD_valConstr00101m/AD_valConstr00101m.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attr-ref"
           xmlns:tns="urn:attr-ref">
  <xs:attribute name="a" type="xs:string"/>
  <xs:complexType name="withAttr">
    <xs:attribute ref="tns:a"/>
  </xs:complexType>
  <xs:element name="root" type="tns:withAttr"/>
</xs:schema>`,
	"sunData/combined/identity/IdentityTestSuite/001/test.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:keyref"
           xmlns:tns="urn:keyref"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:string"/>
            <xs:attribute name="ref" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="itemRef" refer="tns:itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@ref"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`,
	"sunData/combined/xsd001/xsd001.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="mytype">
    <xs:restriction base="xs:string">
      <xs:minLength value="4"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="value" type="mytype"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
	"msData/identityConstraint/idK015.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:inline-union"
           xmlns:tns="urn:inline-union"
           elementFormDefault="qualified">
  <xs:element name="uid">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="pid">
          <xs:simpleType>
            <xs:union>
              <xs:simpleType>
                <xs:restriction base="xs:string"/>
              </xs:simpleType>
              <xs:simpleType>
                <xs:restriction base="xs:int"/>
              </xs:simpleType>
            </xs:union>
          </xs:simpleType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
	"sunData/SType/ST_final/ST_final00101m/ST_final00101m1.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="noRestriction">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="finalRestriction" final="restriction">
    <xs:restriction base="noRestriction"/>
  </xs:simpleType>
  <xs:simpleType name="derivedRestriction">
    <xs:restriction base="finalRestriction"/>
  </xs:simpleType>
</xs:schema>`,
	"sunData/SType/ST_final/ST_final00102m/ST_final00102m1.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="baseItem" final="list">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="badList">
    <xs:list itemType="baseItem"/>
  </xs:simpleType>
</xs:schema>`,
	"sunData/SType/ST_final/ST_final00103m/ST_final00103m1.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="baseMember" final="union">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="badUnion">
    <xs:union memberTypes="baseMember"/>
  </xs:simpleType>
</xs:schema>`,
	"saxonData/Missing/missing001.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:missing">
</xs:schema>`,
	"saxonData/Simple/simple085.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="myUnion">
    <xs:union memberTypes="xs:int xs:boolean"/>
  </xs:simpleType>
</xs:schema>`,
	"ibmData/instance_invalid/S3_3_4/s3_3_4ii08.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="listOfIDs">
    <xs:list itemType="xs:ID"/>
  </xs:simpleType>
</xs:schema>`,
	"sunData/CType/baseTD/baseTD00101m/baseTD00101m1.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:base-td"
           xmlns:tns="urn:base-td">
  <xs:complexType name="Test2">
    <xs:simpleContent>
      <xs:extension base="xs:int"/>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`,
	"sunData/combined/006/test.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:derivation"
           xmlns:tns="urn:derivation">
  <xs:complexType name="B"/>
  <xs:complexType name="C">
    <xs:complexContent>
      <xs:extension base="tns:B"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="Drr">
    <xs:complexContent>
      <xs:extension base="tns:C"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`,
	"sunData/ElemDecl/valueConstraint/valueConstraint00101m/valueConstraint00101m1.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:value-constraint"
           xmlns:tns="urn:value-constraint">
  <xs:element name="root" type="xs:int" default="42"/>
</xs:schema>`,
	"sunData/ElemDecl/valueConstraint/valueConstraint00101m/valueConstraint00101m2.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:value-constraint"
           xmlns:tns="urn:value-constraint">
  <xs:element name="root" type="xs:int" default="abc"/>
</xs:schema>`,
	"sunData/ElemDecl/substGroupExclusions/substGrpExcl00202m/substGrpExcl00202m2.xsd": `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:subst-final"
           xmlns:tns="urn:subst-final">
  <xs:complexType name="Base"/>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="head" type="tns:Base" final="extension"/>
  <xs:element name="member" type="tns:Derived" substitutionGroup="tns:head"/>
</xs:schema>`,
}

func parseW3CSchema(t *testing.T, relPath string) *parser.Schema {
	t.Helper()

	schemaText, ok := w3cSchemaFixtures[relPath]
	if !ok {
		t.Fatalf("schema fixture not defined: %s", relPath)
	}

	schema, err := parser.Parse(strings.NewReader(schemaText))
	if err != nil {
		t.Fatalf("parse schema fixture %s: %v", relPath, err)
	}
	return schema
}

func resolveW3CSchema(t *testing.T, relPath string) *parser.Schema {
	t.Helper()

	schema := parseW3CSchema(t, relPath)
	resolver := NewResolver(schema)
	if err := resolver.Resolve(); err != nil {
		t.Fatalf("resolve schema %s: %v", relPath, err)
	}
	return schema
}

func requireNoReferenceErrors(t *testing.T, schema *parser.Schema) {
	t.Helper()

	if errs := ValidateReferences(schema); len(errs) > 0 {
		t.Fatalf("validate references: %v", errs[0])
	}
}

func requireReferenceErrorContains(t *testing.T, schema *parser.Schema, substr string) {
	t.Helper()

	errs := ValidateReferences(schema)
	if len(errs) == 0 {
		t.Fatalf("expected reference error containing %q", substr)
	}
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), substr) {
			return
		}
	}
	t.Fatalf("expected reference error containing %q, got %v", substr, errs[0])
}

func requireReferenceErrorNotContains(t *testing.T, schema *parser.Schema, substr string) {
	t.Helper()

	errs := ValidateReferences(schema)
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), substr) {
			t.Fatalf("unexpected reference error containing %q: %v", substr, err)
		}
	}
}

func TestResolveW3CGroupAndAttributeGroup(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/combined/xsd024/xsd024.xsdmod")
	requireNoReferenceErrors(t, schema)

	ctQName := model.QName{Namespace: schema.TargetNamespace, Local: "complexType"}
	ct, ok := schema.TypeDefs[ctQName].(*model.ComplexType)
	if !ok || ct == nil {
		t.Fatalf("expected complexType to be a complex type")
	}

	groupQName := model.QName{Namespace: schema.TargetNamespace, Local: "g"}
	if _, ok := schema.Groups[groupQName]; !ok {
		t.Fatalf("expected group %s in schema", groupQName)
	}
	if len(ct.AttrGroups) != 1 {
		t.Fatalf("expected 1 attribute group reference, got %d", len(ct.AttrGroups))
	}
}

func TestResolveW3CComplexTypeBases(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/CType/pSubstitutions/pSubstitutions00101m/pSubstitutions00101m.xsd")
	requireNoReferenceErrors(t, schema)

	baseQName := model.QName{Namespace: schema.TargetNamespace, Local: "A"}
	for _, local := range []string{"B", "C"} {
		ct, ok := schema.TypeDefs[model.QName{Namespace: schema.TargetNamespace, Local: local}].(*model.ComplexType)
		if !ok || ct == nil {
			t.Fatalf("expected %s to be a complex type", local)
		}
		if ct.ResolvedBase == nil {
			t.Fatalf("expected %s to resolve base type", local)
		}
		if ct.ResolvedBase.Name() != baseQName {
			t.Fatalf("expected %s base %s, got %s", local, baseQName, ct.ResolvedBase.Name())
		}
	}
}

func TestResolveAnyTypeUsesBuiltin(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test">
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="xs:anyType"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	res := NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}

	derivedQName := model.QName{Namespace: schema.TargetNamespace, Local: "Derived"}
	derived, ok := schema.TypeDefs[derivedQName].(*model.ComplexType)
	if !ok || derived == nil {
		t.Fatalf("expected Derived to be a complex type")
	}
	if derived.ResolvedBase == nil {
		t.Fatalf("expected Derived base type to be resolved")
	}
	builtinAny := builtins.Get(model.TypeNameAnyType)
	if builtinAny == nil {
		t.Fatalf("expected builtin xs:anyType")
	}
	if derived.ResolvedBase != builtinAny {
		t.Fatalf("expected anyType base to use builtin instance, got %T", derived.ResolvedBase)
	}
	if !model.IsDerivedFrom(derived, builtinAny) {
		t.Fatalf("expected Derived to be derived from builtin anyType")
	}
}

func TestResolveSimpleTypeRestrictionInheritsUnionMembers(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:string xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="R">
    <xs:restriction base="tns:U"/>
  </xs:simpleType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	resolver := NewResolver(schema)
	if err := resolver.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}

	baseQName := model.QName{Namespace: schema.TargetNamespace, Local: "U"}
	derivedQName := model.QName{Namespace: schema.TargetNamespace, Local: "R"}
	base, ok := schema.TypeDefs[baseQName].(*model.SimpleType)
	if !ok || base == nil {
		t.Fatalf("expected base union type U")
	}
	derived, ok := schema.TypeDefs[derivedQName].(*model.SimpleType)
	if !ok || derived == nil {
		t.Fatalf("expected derived type R")
	}
	if len(base.MemberTypes) == 0 {
		t.Fatalf("expected base union member types to be resolved")
	}
	if len(derived.MemberTypes) != len(base.MemberTypes) {
		t.Fatalf("derived member types = %d, want %d", len(derived.MemberTypes), len(base.MemberTypes))
	}
}

func TestResolveW3CUniqueConstraints(t *testing.T) {
	schema := resolveW3CSchema(t, "saxonData/Complex/unique001.xsd")
	requireNoReferenceErrors(t, schema)

	rootQName := model.QName{Namespace: schema.TargetNamespace, Local: "root"}
	root := schema.ElementDecls[rootQName]
	if root == nil {
		t.Fatalf("expected root element declaration")
	}
	if len(root.Constraints) != 1 {
		t.Fatalf("expected 1 identity constraint, got %d", len(root.Constraints))
	}
	if root.Constraints[0].Name != "test" {
		t.Fatalf("expected constraint name 'test', got %q", root.Constraints[0].Name)
	}
}

func TestResolveW3CMissingListItemType(t *testing.T) {
	schema := parseW3CSchema(t, "saxonData/Missing/missing006.xsd")
	if err := NewResolver(schema).Resolve(); err == nil {
		t.Fatalf("expected missing list item type to fail resolution")
	}
}

func TestResolveW3CMissingSimpleTypeBase(t *testing.T) {
	schema := parseW3CSchema(t, "saxonData/Missing/missing004.xsd")
	if err := NewResolver(schema).Resolve(); err == nil {
		t.Fatalf("expected error resolving missing base type")
	}
}

func TestValidateReferencesCyclicSubstitutionGroups(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/combined/xsd010/xsd010.e.xsd")
	requireReferenceErrorContains(t, schema, "cyclic substitution group")
}

func TestValidateReferencesSubstitutionGroupExplicitAnyType(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test">
  <xs:element name="head" type="xs:string" abstract="true"/>
  <xs:element name="member" type="xs:anyType" substitutionGroup="tns:head"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	res := NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}
	requireReferenceErrorContains(t, schema, "not derived from substitution group head type")
}

func TestValidateReferencesMissingSubstitutionGroupHead(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test">
  <xs:element name="member" type="xs:string" substitutionGroup="tns:missing"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if err := NewResolver(schema).Resolve(); err != nil {
		t.Fatalf("resolve type references: %v", err)
	}
	requireReferenceErrorContains(t, schema, "substitutionGroup")
	requireReferenceErrorContains(t, schema, "does not exist")
	requireReferenceErrorNotContains(t, schema, "cyclic substitution group")
}

func TestValidateReferencesListDefaultRejectsNonXMLWhitespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:list"
           xmlns:tns="urn:list">
  <xs:simpleType name="listType">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:listType" default="1` + "\u00A0" + `2"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	res := NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}
	requireReferenceErrorContains(t, schema, "invalid default value")
}

func TestValidateReferencesLocalKeyrefContext(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:keyref"
           xmlns:tns="urn:keyref"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="id" type="xs:string"/>
            </xs:sequence>
          </xs:complexType>
          <xs:key name="k">
            <xs:selector xpath="."/>
            <xs:field xpath="tns:id"/>
          </xs:key>
          <xs:keyref name="kr" refer="tns:k">
            <xs:selector xpath="."/>
            <xs:field xpath="tns:id"/>
          </xs:keyref>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if err := NewResolver(schema).Resolve(); err != nil {
		t.Fatalf("resolve type references: %v", err)
	}
	if errs := ValidateReferences(schema); len(errs) > 0 {
		t.Fatalf("validate references: %v", errs[0])
	}
}

func TestValidateReferencesDefaultFacetViolations(t *testing.T) {
	tests := []struct {
		name   string
		schema string
	}{
		{
			name: "enumeration",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:enum"
           xmlns:tns="urn:enum">
  <xs:simpleType name="EnumType">
    <xs:restriction base="xs:string">
      <xs:enumeration value="A"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:EnumType" default="B"/>
</xs:schema>`,
		},
		{
			name: "list minLength",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:listmin"
           xmlns:tns="urn:listmin">
  <xs:simpleType name="IntList">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="IntListMin2">
    <xs:restriction base="tns:IntList">
      <xs:minLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:IntListMin2" default="1"/>
</xs:schema>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.Parse(strings.NewReader(tt.schema))
			if err != nil {
				t.Fatalf("parse schema: %v", err)
			}
			res := NewResolver(schema)
			if err := res.Resolve(); err != nil {
				t.Fatalf("resolve schema: %v", err)
			}
			requireReferenceErrorContains(t, schema, "invalid default value")
		})
	}
}

func TestValidateReferencesUnionFieldIncompatibleTypesAllowed(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:union"
           xmlns:tns="urn:union"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:int"/>
        <xs:element name="b" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="unionKey">
      <xs:selector xpath="."/>
      <xs:field xpath="tns:a | tns:b"/>
    </xs:key>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	res := NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}
	requireNoReferenceErrors(t, schema)
}

func TestValidateReferencesDuplicateIdentityConstraints(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/IdConstrDefs/name/name00101m/name00101m2.xsd")
	requireReferenceErrorContains(t, schema, "not unique")
}

func TestValidateReferencesAttributeReferences(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/AttrDecl/AD_valConstr/AD_valConstr00101m/AD_valConstr00101m.xsd")
	requireNoReferenceErrors(t, schema)
}

func TestValidateReferencesAttributeRefIgnoresDefaultNamespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="urn:tns"
           targetNamespace="urn:tns"
           elementFormDefault="qualified">
  <xs:import schemaLocation="no-ns.xsd"/>
  <xs:attribute name="a" type="xs:string"/>
  <xs:complexType name="t">
    <xs:attribute ref="a"/>
  </xs:complexType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	requireReferenceErrorContains(t, schema, "attribute reference")
}

func TestValidateReferencesAttributeRefNoTargetNamespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="urn:tns">
  <xs:attribute name="a" type="xs:string"/>
  <xs:complexType name="t">
    <xs:attribute ref="a"/>
  </xs:complexType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	requireNoReferenceErrors(t, schema)
}

func TestValidateReferencesAttributeRefDefaultAgainstFixed(t *testing.T) {
	tests := []struct {
		name     string
		defaultV string
	}{
		{name: "default matches fixed", defaultV: "1"},
		{name: "default differs from fixed", defaultV: "2"},
	}

	for _, tt := range tests {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attr"
           xmlns:tns="urn:attr"
           elementFormDefault="qualified">
  <xs:attribute name="a" type="xs:string" fixed="1"/>
  <xs:complexType name="t">
    <xs:attribute ref="tns:a" default="` + tt.defaultV + `"/>
  </xs:complexType>
</xs:schema>`

		schema, err := parser.Parse(strings.NewReader(schemaXML))
		if err != nil {
			t.Fatalf("parse schema: %v", err)
		}

		requireReferenceErrorContains(t, schema, "default")
	}
}

func TestValidateReferencesAttributeRefFixedMatches(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attr"
           xmlns:tns="urn:attr"
           elementFormDefault="qualified">
  <xs:attribute name="a" type="xs:string" fixed="1"/>
  <xs:complexType name="t">
    <xs:attribute ref="tns:a" fixed="1"/>
  </xs:complexType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	requireNoReferenceErrors(t, schema)
}

func TestValidateReferencesAttributeRefFixedQNameValueSpace(t *testing.T) {
	equivalent := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attr"
           xmlns:tns="urn:attr"
           xmlns:p="urn:a"
           elementFormDefault="qualified">
  <xs:attribute name="a" type="xs:QName" fixed="p:code"/>
  <xs:complexType name="t" xmlns:q="urn:a">
    <xs:attribute ref="tns:a" fixed="q:code"/>
  </xs:complexType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(equivalent))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	requireNoReferenceErrors(t, schema)

	mismatch := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attr"
           xmlns:tns="urn:attr"
           xmlns:p="urn:a"
           elementFormDefault="qualified">
  <xs:attribute name="a" type="xs:QName" fixed="p:code"/>
  <xs:complexType name="t" xmlns:p="urn:b">
    <xs:attribute ref="tns:a" fixed="p:code"/>
  </xs:complexType>
</xs:schema>`

	schema, err = parser.Parse(strings.NewReader(mismatch))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	requireReferenceErrorContains(t, schema, "fixed value")
}

func TestValidateReferencesAttributeRefFixedWhitespaceValueSpace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:attr"
           xmlns:tns="urn:attr"
           elementFormDefault="qualified">
  <xs:attribute name="a" type="xs:token" fixed="a   b"/>
  <xs:complexType name="t">
    <xs:attribute ref="tns:a" fixed="a b"/>
  </xs:complexType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	requireNoReferenceErrors(t, schema)
}

func TestValidateReferencesKeyref(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/combined/identity/IdentityTestSuite/001/test.xsd")
	requireNoReferenceErrors(t, schema)
}

func TestValidateReferencesInlineTypes(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/combined/xsd001/xsd001.xsd")
	requireNoReferenceErrors(t, schema)
}

func TestResolveW3CInlineUnionAnonymousTypes(t *testing.T) {
	schema := resolveW3CSchema(t, "msData/identityConstraint/idK015.xsd")
	requireNoReferenceErrors(t, schema)

	uidQName := model.QName{Namespace: schema.TargetNamespace, Local: "uid"}
	uid := schema.ElementDecls[uidQName]
	if uid == nil {
		t.Fatalf("expected uid element declaration")
	}
	ct, ok := uid.Type.(*model.ComplexType)
	if !ok || ct == nil {
		t.Fatalf("expected uid to have a complex type")
	}

	particle := traversal.GetContentParticle(ct.Content())
	if particle == nil {
		t.Fatalf("expected uid content particle")
	}
	var pid *model.ElementDecl
	for _, elem := range traversal.CollectFromParticlesWithVisited([]model.Particle{particle}, nil, func(p model.Particle) (*model.ElementDecl, bool) {
		elem, ok := p.(*model.ElementDecl)
		return elem, ok
	}) {
		if elem.Name.Local == "pid" {
			pid = elem
			break
		}
	}
	if pid == nil {
		t.Fatalf("expected pid element in uid content")
	}
	st, ok := pid.Type.(*model.SimpleType)
	if !ok || st == nil {
		t.Fatalf("expected pid to have a simple type")
	}
	if st.Union == nil || len(st.Union.InlineTypes) == 0 {
		t.Fatalf("expected pid to use union with inline member types")
	}
	for i, inline := range st.Union.InlineTypes {
		if inline.ResolvedBase == nil {
			t.Fatalf("expected union inline member %d to resolve base type", i)
		}
	}
}

func TestResolveUnionRestrictionMemberTypes(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="BaseUnion">
    <xs:union memberTypes="xs:int xs:boolean"/>
  </xs:simpleType>
  <xs:simpleType name="RestrictedUnion">
    <xs:restriction base="BaseUnion">
      <xs:pattern value="a+"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	resolver := NewResolver(schema)
	if err := resolver.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}

	base, ok := schema.TypeDefs[model.QName{Local: "BaseUnion"}].(*model.SimpleType)
	if !ok || base == nil {
		t.Fatalf("expected BaseUnion simple type")
	}
	if base.Variety() != model.UnionVariety {
		t.Fatalf("expected BaseUnion to be a union type")
	}
	if len(base.MemberTypes) != 2 {
		t.Fatalf("expected BaseUnion to have 2 member types, got %d", len(base.MemberTypes))
	}

	restricted, ok := schema.TypeDefs[model.QName{Local: "RestrictedUnion"}].(*model.SimpleType)
	if !ok || restricted == nil {
		t.Fatalf("expected RestrictedUnion simple type")
	}
	if restricted.Variety() != model.UnionVariety {
		t.Fatalf("expected RestrictedUnion to be a union type")
	}
	if len(restricted.MemberTypes) != len(base.MemberTypes) {
		t.Fatalf("expected RestrictedUnion to inherit %d member types, got %d", len(base.MemberTypes), len(restricted.MemberTypes))
	}
	for i, member := range restricted.MemberTypes {
		if member == nil || member.Name() != base.MemberTypes[i].Name() {
			t.Fatalf("member type %d = %v, want %v", i, member, base.MemberTypes[i].Name())
		}
	}
}

func TestValidateUnionRestrictionDefaultValue(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="BaseUnion">
    <xs:union memberTypes="xs:int xs:boolean"/>
  </xs:simpleType>
  <xs:simpleType name="RestrictedUnion">
    <xs:restriction base="BaseUnion">
      <xs:enumeration value="1"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="RestrictedUnion" default="1"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	resolver := NewResolver(schema)
	if err := resolver.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}

	if errs := ValidateReferences(schema); len(errs) > 0 {
		t.Fatalf("validate references: %v", errs[0])
	}
}

package resolver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
)

func parseW3CSchema(t *testing.T, relPath string) *parser.Schema {
	t.Helper()

	schemaPath := filepath.Join("..", "..", "testdata", "xsdtests", filepath.FromSlash(relPath))
	file, err := os.Open(schemaPath)
	if err != nil {
		t.Fatalf("open schema %s: %v", schemaPath, err)
	}
	t.Cleanup(func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close schema %s: %v", schemaPath, closeErr)
		}
	})

	schema, err := parser.Parse(file)
	if err != nil {
		t.Fatalf("parse schema %s: %v", schemaPath, err)
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

func resolveW3CTypeReferences(t *testing.T, relPath string) *parser.Schema {
	t.Helper()

	schema := parseW3CSchema(t, relPath)
	if err := ResolveTypeReferences(schema); err != nil {
		t.Fatalf("resolve type references %s: %v", relPath, err)
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

func TestResolveW3CGroupAndAttributeGroup(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/combined/xsd024/xsd024.xsdmod")
	requireNoReferenceErrors(t, schema)

	ct, ok := schema.TypeDefs[types.QName{Local: "complexType"}].(*types.ComplexType)
	if !ok || ct == nil {
		t.Fatalf("expected complexType to be a complex type")
	}

	var refQName types.QName
	if err := schemacheck.WalkContentParticles(ct.Content(), func(p types.Particle) error {
		if ref, ok := p.(*types.GroupRef); ok {
			refQName = ref.RefQName
		}
		return nil
	}); err != nil {
		t.Fatalf("walk content particles: %v", err)
	}

	if refQName.IsZero() {
		t.Fatalf("expected group reference in complexType content")
	}
	if _, ok := schema.Groups[refQName]; !ok {
		t.Fatalf("group reference %s not found in schema groups", refQName)
	}
	if len(ct.AttrGroups) != 1 {
		t.Fatalf("expected 1 attribute group reference, got %d", len(ct.AttrGroups))
	}
}

func TestResolveW3CComplexTypeBases(t *testing.T) {
	schema := resolveW3CSchema(t, "sunData/CType/pSubstitutions/pSubstitutions00101m/pSubstitutions00101m.xsd")
	requireNoReferenceErrors(t, schema)

	baseQName := types.QName{Namespace: schema.TargetNamespace, Local: "A"}
	for _, local := range []string{"B", "C"} {
		ct, ok := schema.TypeDefs[types.QName{Namespace: schema.TargetNamespace, Local: local}].(*types.ComplexType)
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

func TestResolveW3CUniqueConstraints(t *testing.T) {
	schema := resolveW3CSchema(t, "saxonData/Complex/unique001.xsd")
	requireNoReferenceErrors(t, schema)

	root := schema.ElementDecls[types.QName{Local: "root"}]
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
	schema := resolveW3CTypeReferences(t, "saxonData/Missing/missing006.xsd")

	st, ok := schema.TypeDefs[types.QName{Local: "list"}].(*types.SimpleType)
	if !ok || st == nil {
		t.Fatalf("expected list to be a simple type")
	}
	if st.ItemType == nil {
		t.Fatalf("expected list item type to be set")
	}
	if st.ItemType.Name().Local != "absent" {
		t.Fatalf("expected list item type name 'absent', got %s", st.ItemType.Name())
	}
	if st.WhiteSpace() != types.WhiteSpaceCollapse {
		t.Fatalf("expected list whiteSpace collapse, got %v", st.WhiteSpace())
	}
}

func TestResolveW3CMissingSimpleTypeBase(t *testing.T) {
	schema := parseW3CSchema(t, "saxonData/Missing/missing004.xsd")
	if err := ResolveTypeReferences(schema); err == nil {
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

	uid := schema.ElementDecls[types.QName{Local: "uid"}]
	if uid == nil {
		t.Fatalf("expected uid element declaration")
	}
	ct, ok := uid.Type.(*types.ComplexType)
	if !ok || ct == nil {
		t.Fatalf("expected uid to have a complex type")
	}

	particle := schemacheck.GetContentParticle(ct.Content())
	if particle == nil {
		t.Fatalf("expected uid content particle")
	}
	var pid *types.ElementDecl
	for _, elem := range schemacheck.CollectElements(particle) {
		if elem.Name.Local == "pid" {
			pid = elem
			break
		}
	}
	if pid == nil {
		t.Fatalf("expected pid element in uid content")
	}
	st, ok := pid.Type.(*types.SimpleType)
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

	base, ok := schema.TypeDefs[types.QName{Local: "BaseUnion"}].(*types.SimpleType)
	if !ok || base == nil {
		t.Fatalf("expected BaseUnion simple type")
	}
	if base.Variety() != types.UnionVariety {
		t.Fatalf("expected BaseUnion to be a union type")
	}
	if len(base.MemberTypes) != 2 {
		t.Fatalf("expected BaseUnion to have 2 member types, got %d", len(base.MemberTypes))
	}

	restricted, ok := schema.TypeDefs[types.QName{Local: "RestrictedUnion"}].(*types.SimpleType)
	if !ok || restricted == nil {
		t.Fatalf("expected RestrictedUnion simple type")
	}
	if restricted.Variety() != types.UnionVariety {
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

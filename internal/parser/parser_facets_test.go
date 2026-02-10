package parser

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestParser_UsesFacetConstructors(t *testing.T) {
	// test that parser uses type-safe facet constructors when base type is available
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com"
           xmlns:tns="http://example.com">
  <xs:simpleType name="restrictedDecimal">
    <xs:restriction base="xs:decimal">
      <xs:minInclusive value="10.5"/>
      <xs:maxInclusive value="100.0"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := model.QName{
		Namespace: "http://example.com",
		Local:     "restrictedDecimal",
	}
	typeDef, ok := schema.TypeDefs[qname]
	if !ok {
		t.Fatal("restrictedDecimal type not found")
	}

	st, ok := typeDef.(*model.SimpleType)
	if !ok {
		t.Fatal("type is not a SimpleType")
	}

	if st.Restriction == nil {
		t.Fatal("Restriction is nil")
	}

	// check that facets are Facet instances (from constructors)
	foundMinInclusive := false
	foundMaxInclusive := false

	for _, f := range st.Restriction.Facets {
		if facet, ok := f.(model.Facet); ok {
			name := facet.Name()
			switch name {
			case "minInclusive":
				foundMinInclusive = true
			case "maxInclusive":
				foundMaxInclusive = true
			}
		}
	}

	if !foundMinInclusive {
		t.Error("minInclusive facet should be created using constructor (Facet)")
	}
	if !foundMaxInclusive {
		t.Error("maxInclusive facet should be created using constructor (Facet)")
	}
}

func TestParser_UsesFacetConstructors_Integer(t *testing.T) {
	// test with integer base type
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com"
           xmlns:tns="http://example.com">
  <xs:simpleType name="restrictedInteger">
    <xs:restriction base="xs:integer">
      <xs:minInclusive value="0"/>
      <xs:maxInclusive value="100"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := model.QName{
		Namespace: "http://example.com",
		Local:     "restrictedInteger",
	}
	typeDef, ok := schema.TypeDefs[qname]
	if !ok {
		t.Fatal("restrictedInteger type not found")
	}

	st, ok := typeDef.(*model.SimpleType)
	if !ok {
		t.Fatal("type is not a SimpleType")
	}

	for _, f := range st.Restriction.Facets {
		if facet, ok := f.(model.Facet); ok {
			if facet.Name() == "minInclusive" || facet.Name() == "maxInclusive" {
				// good - using constructor
				return
			}
		}
	}

	t.Error("Range facets should be created using constructors (Facet)")
}

func TestParser_FallbackToStringFacets_UserDefinedType(t *testing.T) {
	// test that parser falls back to string-based facets when base type is not available
	// (user-defined types defined later in the schema)
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com"
           xmlns:tns="http://example.com">
  <xs:simpleType name="restrictedType">
    <xs:restriction base="tns:baseType">
      <xs:minInclusive value="10"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="baseType">
    <xs:restriction base="xs:decimal"/>
  </xs:simpleType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// during parsing, baseType might not be available yet, so facets won't be created
	// after semantic resolution in the loader, facets should be created.
	// for now, we just verify it doesn't crash and the type is parsed correctly
	qname := model.QName{
		Namespace: "http://example.com",
		Local:     "restrictedType",
	}
	typeDef, ok := schema.TypeDefs[qname]
	if !ok {
		t.Fatal("restrictedType not found")
	}

	st, ok := typeDef.(*model.SimpleType)
	if !ok {
		t.Fatal("type is not a SimpleType")
	}

	// facets may not be created during parsing if baseType is not available
	// they will be created during semantic resolution in the loader.
	// so we just verify the type structure is correct
	if st.Restriction == nil {
		t.Error("Restriction should not be nil")
	}
}

func TestParser_RestrictionWithInlineSimpleType(t *testing.T) {
	// test restriction without base attribute but with inline simpleType child
	// this is a valid XSD pattern where the base type is defined inline
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com"
           xmlns:tns="http://example.com">
  <xs:simpleType name="type_c">
    <xs:restriction>
      <xs:simpleType>
        <xs:restriction base="xs:string">
          <xs:whiteSpace value="preserve"/>
        </xs:restriction>
      </xs:simpleType>
      <xs:whiteSpace value="replace"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := model.QName{
		Namespace: "http://example.com",
		Local:     "type_c",
	}
	typeDef, ok := schema.TypeDefs[qname]
	if !ok {
		t.Fatal("type_c not found")
	}

	st, ok := typeDef.(*model.SimpleType)
	if !ok {
		t.Fatal("type is not a SimpleType")
	}

	if st.Restriction == nil {
		t.Fatal("Restriction is nil")
	}

	// restriction.Base should be zero (empty) since it's an inline base
	if !st.Restriction.Base.IsZero() {
		t.Errorf("Restriction.Base should be zero for inline simpleType base, got %v", st.Restriction.Base)
	}

	// parse phase keeps inline base symbolic in Restriction.SimpleType.
	if st.ResolvedBase != nil {
		t.Fatal("ResolvedBase should remain unset during parse phase")
	}

	// verify the inline base type is a SimpleType
	baseST := st.Restriction.SimpleType
	if baseST == nil {
		t.Fatal("Restriction.SimpleType should be present")
	}

	// the inline base should have its own restriction with base="xs:string"
	if baseST.Restriction == nil {
		t.Fatal("Inline base type should have a restriction")
	}
	if baseST.Restriction.Base.IsZero() {
		t.Error("Inline base type's restriction should have a base QName")
	}
	if baseST.Restriction.Base.Local != "string" || baseST.Restriction.Base.Namespace != model.XSDNamespace {
		t.Errorf("Inline base type should restrict xs:string, got %v", baseST.Restriction.Base)
	}

	// verify whiteSpace facet is set on the outer type (value="replace")
	if st.WhiteSpace() != model.WhiteSpaceReplace {
		t.Errorf("WhiteSpace should be Replace, got %v", st.WhiteSpace())
	}
}

func TestParser_RestrictionWithInlineSimpleTypeUnion(t *testing.T) {
	// test restriction with inline simpleType containing a union
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com"
           xmlns:tns="http://example.com"
           elementFormDefault="qualified">
  <xs:simpleType name="st.unionType">
    <xs:restriction>
      <xs:simpleType>
        <xs:union memberTypes="xs:string xs:integer"/>
      </xs:simpleType>
      <xs:enumeration value="a"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	schema, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	qname := model.QName{
		Namespace: "http://example.com",
		Local:     "st.unionType",
	}
	typeDef, ok := schema.TypeDefs[qname]
	if !ok {
		t.Fatal("st.unionType not found")
	}

	st, ok := typeDef.(*model.SimpleType)
	if !ok {
		t.Fatal("type is not a SimpleType")
	}

	if st.Restriction == nil {
		t.Fatal("Restriction is nil")
	}

	// restriction.Base should be zero (empty) since it's an inline base
	if !st.Restriction.Base.IsZero() {
		t.Errorf("Restriction.Base should be zero for inline simpleType base, got %v", st.Restriction.Base)
	}

	// parse phase keeps inline base symbolic in Restriction.SimpleType.
	if st.ResolvedBase != nil {
		t.Fatal("ResolvedBase should remain unset during parse phase")
	}

	// verify the inline base type is a SimpleType with Union variety
	baseST := st.Restriction.SimpleType
	if baseST == nil {
		t.Fatal("Restriction.SimpleType should be present")
	}

	if baseST.Variety() != model.UnionVariety {
		t.Errorf("Inline base type should have Union variety, got %v", baseST.Variety())
	}

	if baseST.Union == nil {
		t.Fatal("Inline base type should have a Union")
	}

	// verify enumeration facet is present
	foundEnum := false
	for _, f := range st.Restriction.Facets {
		if facet, ok := f.(model.Facet); ok {
			if facet.Name() == "enumeration" {
				foundEnum = true
				enum, ok := facet.(*model.Enumeration)
				if !ok {
					t.Fatal("enumeration facet should be *model.Enumeration")
				}
				values := enum.Values()
				if len(values) != 1 || values[0] != "a" {
					t.Errorf("enumeration should have value 'a', got %v", values)
				}
				break
			}
		}
	}
	if !foundEnum {
		t.Error("enumeration facet should be present")
	}
}

func TestParser_RestrictionCannotHaveBothBaseAndSimpleType(t *testing.T) {
	// test that restriction cannot have both base attribute and inline simpleType child
	// (per XSD spec: "Either the base attribute or the simpleType child must be present, but not both")
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com"
           xmlns:tns="http://example.com">
  <xs:simpleType name="invalidType">
    <xs:restriction base="xs:string">
      <xs:simpleType>
        <xs:restriction base="xs:string"/>
      </xs:simpleType>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	_, err := Parse(strings.NewReader(schemaXML))
	if err == nil {
		t.Fatal("Parse() should have returned an error for restriction with both base and simpleType")
	}
	if !strings.Contains(err.Error(), "cannot have both base attribute and inline simpleType child") {
		t.Errorf("Error message should mention both base and simpleType, got: %v", err)
	}
}

package schemaast

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestParseDocumentSimpleTypeDecls(t *testing.T) {
	doc := parseSchemaDocumentForTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:maxLength value="10" fixed="true"/>
      <xs:enumeration value="A"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="CodeList">
    <xs:list itemType="tns:Code"/>
  </xs:simpleType>
  <xs:simpleType name="CodeUnion">
    <xs:union memberTypes="xs:int tns:Code">
      <xs:simpleType>
        <xs:restriction base="xs:string"/>
      </xs:simpleType>
    </xs:union>
  </xs:simpleType>
</xs:schema>`)

	if len(doc.Decls) != 3 {
		t.Fatalf("decl count = %d, want 3", len(doc.Decls))
	}
	code := doc.Decls[0].SimpleType
	if code == nil || code.Kind != SimpleDerivationRestriction {
		t.Fatalf("Code kind = %#v, want restriction", code)
	}
	if code.Base != (QName{Namespace: XSDNamespace, Local: "string"}) {
		t.Fatalf("Code base = %v, want xs:string", code.Base)
	}
	if len(code.Facets) != 2 {
		t.Fatalf("Code facets = %d, want 2", len(code.Facets))
	}
	if code.Facets[0].Name != "maxLength" || code.Facets[0].Lexical != "10" || !code.Facets[0].Fixed {
		t.Fatalf("maxLength facet = %#v", code.Facets[0])
	}

	list := doc.Decls[1].SimpleType
	if list.Kind != SimpleDerivationList {
		t.Fatalf("CodeList kind = %v, want list", list.Kind)
	}
	if list.ItemType != (QName{Namespace: "urn:test", Local: "Code"}) {
		t.Fatalf("CodeList item = %v, want tns:Code", list.ItemType)
	}

	union := doc.Decls[2].SimpleType
	if union.Kind != SimpleDerivationUnion {
		t.Fatalf("CodeUnion kind = %v, want union", union.Kind)
	}
	wantMembers := []QName{{Namespace: XSDNamespace, Local: "int"}, {Namespace: "urn:test", Local: "Code"}}
	if !reflect.DeepEqual(union.MemberTypes, wantMembers) {
		t.Fatalf("CodeUnion members = %v, want %v", union.MemberTypes, wantMembers)
	}
	if len(union.InlineMembers) != 1 {
		t.Fatalf("CodeUnion inline members = %d, want 1", len(union.InlineMembers))
	}
}

func TestParseDocumentComplexTypeDecls(t *testing.T) {
	doc := parseSchemaDocumentForTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:complexType name="Person">
    <xs:sequence>
      <xs:element name="id" type="xs:string" minOccurs="0" maxOccurs="unbounded"/>
      <xs:element name="nested">
        <xs:complexType>
          <xs:sequence>
            <xs:element name="child" type="xs:int"/>
          </xs:sequence>
        </xs:complexType>
      </xs:element>
      <xs:any namespace="##other" processContents="lax" minOccurs="0"/>
    </xs:sequence>
    <xs:attribute name="status" type="xs:string" default="ok"/>
    <xs:attributeGroup ref="tns:commonAttrs"/>
    <xs:anyAttribute namespace="##other" processContents="skip"/>
  </xs:complexType>
</xs:schema>`)

	ct := doc.Decls[0].ComplexType
	if ct == nil {
		t.Fatal("complex type is nil")
	}
	if ct.Name != (QName{Namespace: "urn:test", Local: "Person"}) {
		t.Fatalf("complex name = %v", ct.Name)
	}
	if ct.Particle == nil || ct.Particle.Kind != ParticleSequence || len(ct.Particle.Children) != 3 {
		t.Fatalf("particle = %#v, want sequence with 3 children", ct.Particle)
	}
	first := ct.Particle.Children[0].Element
	if first == nil || first.Name.Local != "id" || first.Type.Name != (QName{Namespace: XSDNamespace, Local: "string"}) {
		t.Fatalf("first element = %#v", first)
	}
	if !first.MaxOccurs.IsUnbounded() {
		t.Fatalf("first maxOccurs unbounded = false")
	}
	nested := ct.Particle.Children[1].Element
	if nested == nil || nested.Type.Complex == nil || nested.Type.Complex.Particle == nil {
		t.Fatalf("nested element = %#v", nested)
	}
	wildcard := ct.Particle.Children[2].Wildcard
	if wildcard == nil || wildcard.Namespace != NSCOther || wildcard.ProcessContents != Lax {
		t.Fatalf("wildcard = %#v", wildcard)
	}
	if len(ct.Attributes) != 1 || ct.Attributes[0].Attribute == nil {
		t.Fatalf("attributes = %#v", ct.Attributes)
	}
	attr := ct.Attributes[0].Attribute
	if attr.Name.Local != "status" || attr.Use != Optional || !attr.Default.Present || attr.Default.Lexical != "ok" {
		t.Fatalf("attribute = %#v", attr)
	}
	if len(ct.AttributeGroups) != 1 || ct.AttributeGroups[0] != (QName{Namespace: "urn:test", Local: "commonAttrs"}) {
		t.Fatalf("attribute groups = %v", ct.AttributeGroups)
	}
	if ct.AnyAttribute == nil || ct.AnyAttribute.Namespace != NSCOther || ct.AnyAttribute.ProcessContents != Skip {
		t.Fatalf("anyAttribute = %#v", ct.AnyAttribute)
	}
}

func TestParseDocumentSimpleContentRestrictionInlineSimpleType(t *testing.T) {
	doc := parseSchemaDocumentForTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test">
  <xs:complexType name="Restricted">
    <xs:simpleContent>
      <xs:restriction base="xs:decimal">
        <xs:simpleType>
          <xs:restriction base="xs:integer"/>
        </xs:simpleType>
        <xs:maxInclusive value="16"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`)

	ct := doc.Decls[0].ComplexType
	if ct == nil {
		t.Fatal("complex type is nil")
	}
	if ct.Content != ComplexContentSimple || ct.Derivation != ComplexDerivationRestriction {
		t.Fatalf("content = %v derivation = %v, want simple restriction", ct.Content, ct.Derivation)
	}
	if ct.SimpleType == nil {
		t.Fatal("nested simpleType is nil")
	}
	if ct.SimpleType.Base != (QName{Namespace: XSDNamespace, Local: "integer"}) {
		t.Fatalf("nested simpleType base = %v, want xs:integer", ct.SimpleType.Base)
	}
	if len(ct.SimpleFacets) != 1 || ct.SimpleFacets[0].Name != "maxInclusive" || ct.SimpleFacets[0].Lexical != "16" {
		t.Fatalf("simple facets = %#v", ct.SimpleFacets)
	}
}

func TestParseDocumentRejectsInvalidBooleanAttributes(t *testing.T) {
	tests := []struct {
		name string
		xsd  string
		want string
	}{
		{
			name: "complex abstract",
			xsd: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T" abstract="-1"/>
</xs:schema>`,
			want: "invalid abstract attribute value '-1': must be 'true', 'false', '1', or '0'",
		},
		{
			name: "complex mixed",
			xsd: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T" mixed="-1"/>
</xs:schema>`,
			want: "invalid mixed attribute value '-1': must be 'true', 'false', '1', or '0'",
		},
		{
			name: "element nillable",
			xsd: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="e" nillable="-1"/>
</xs:schema>`,
			want: "invalid nillable attribute value '-1': must be 'true', 'false', '1', or '0'",
		},
		{
			name: "facet fixed",
			xsd: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="S">
    <xs:restriction base="xs:string">
      <xs:length value="1" fixed="-1"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`,
			want: "invalid fixed attribute value '-1': must be 'true', 'false', '1', or '0'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDocumentWithImportsOptions(strings.NewReader(tt.xsd))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
			}
		})
	}
}

func TestParseDocumentRejectsInvalidTopLevelElementAttribute(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="e" nullable="true"/>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "invalid attribute 'nullable' on top-level element") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsNamedInlineTypes(t *testing.T) {
	tests := []struct {
		name string
		xsd  string
		want string
	}{
		{
			name: "inline simple type",
			xsd: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="e">
    <xs:simpleType name="S">
      <xs:restriction base="xs:string"/>
    </xs:simpleType>
  </xs:element>
</xs:schema>`,
			want: "inline simpleType cannot have 'name' attribute",
		},
		{
			name: "inline complex type",
			xsd: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="e">
    <xs:complexType name="T"/>
  </xs:element>
</xs:schema>`,
			want: "inline complexType cannot have 'name' attribute",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDocumentWithImportsOptions(strings.NewReader(tt.xsd))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
			}
		})
	}
}

func TestParseDocumentRejectsSimpleContentExtensionParticle(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G">
    <xs:sequence>
      <xs:element name="e" type="xs:string"/>
    </xs:sequence>
  </xs:group>
  <xs:complexType name="T">
    <xs:simpleContent>
      <xs:extension base="xs:string">
        <xs:group ref="G"/>
      </xs:extension>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "simpleContent extension has unexpected child element 'group'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentIdentityDecls(t *testing.T) {
	doc := parseSchemaDocumentForTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:element name="root">
    <xs:key name="rootKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="rootRef" refer="tns:rootKey">
      <xs:selector xpath="tns:ref"/>
      <xs:field xpath="@id"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`)

	elem := doc.Decls[0].Element
	if elem == nil {
		t.Fatal("element is nil")
	}
	if len(elem.Identity) != 2 {
		t.Fatalf("identity count = %d, want 2", len(elem.Identity))
	}
	key := elem.Identity[0]
	if key.Kind != IdentityKey || key.Selector != "tns:item" || !reflect.DeepEqual(key.Fields, []string{"@id"}) {
		t.Fatalf("key = %#v", key)
	}
	keyref := elem.Identity[1]
	if keyref.Kind != IdentityKeyref || keyref.Refer != (QName{Namespace: "urn:test", Local: "rootKey"}) {
		t.Fatalf("keyref = %#v", keyref)
	}
	if !contextHasBinding(doc.NamespaceContexts[key.NamespaceContextID], "tns", "urn:test") {
		t.Fatalf("identity namespace context lacks tns binding: %#v", doc.NamespaceContexts[key.NamespaceContextID])
	}
}

func TestParseDocumentRejectsLateIdentityAnnotation(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="people">
    <xs:unique name="UNIQUENESS">
      <xs:selector xpath="./person"/>
      <xs:field xpath="."/>
      <xs:annotation/>
    </xs:unique>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "identity constraint \"UNIQUENESS\": annotation must appear before selector and field") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleFieldAnnotations(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:unique name="u">
      <xs:selector xpath="*"/>
      <xs:field xpath="@id">
        <xs:annotation/>
        <xs:annotation/>
      </xs:field>
    </xs:unique>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "field: at most one annotation is allowed") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsReferOnUnique(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:unique name="u" refer="k">
      <xs:selector xpath="*"/>
      <xs:field xpath="@id"/>
    </xs:unique>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "'refer' attribute is only allowed on keyref constraints") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsKeyrefMissingRefer(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:keyref name="kr">
      <xs:selector xpath="*"/>
      <xs:field xpath="@id"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "keyref missing refer attribute") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsIdentityMissingFields(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:unique name="u">
      <xs:selector xpath="*"/>
    </xs:unique>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "identity constraint missing fields") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsElementTypeAfterIdentity(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:unique name="u">
      <xs:selector xpath="*"/>
      <xs:field xpath="@id"/>
    </xs:unique>
    <xs:complexType/>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "element type definition must precede identity constraints") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleListAnnotations(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="IDs">
    <xs:list itemType="xs:int">
      <xs:annotation/>
      <xs:annotation/>
    </xs:list>
  </xs:simpleType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "at most one annotation element is allowed") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsFacetNonAnnotationChild(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="foo">
    <xs:restriction base="xs:string">
      <xs:enumeration value="1 2">
        <xs:notation name="jpeg" public="image/jpeg" system="viewer.exe"/>
      </xs:enumeration>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "enumeration: unexpected child element 'notation'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsUnknownRestrictionFacet(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="foo">
    <xs:restriction base="xs:string">
      <xs:notation name="jpeg" public="image/jpeg" system="viewer.exe"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "parse facets: unknown or invalid facet 'notation' (not a valid XSD 1.0 facet)") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleAttributeGroupAnnotations(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:attributeGroup name="G">
    <xs:annotation/>
    <xs:annotation/>
    <xs:attribute name="a" type="xs:string"/>
  </xs:attributeGroup>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attributeGroup 'G': at most one annotation is allowed") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsAttributeGroupRefChildren(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attributeGroup name="G"/>
  <xs:complexType name="T">
    <xs:attributeGroup ref="G">
      <xs:attribute name="a" type="xs:string"/>
    </xs:attributeGroup>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attributeGroup: unexpected child element 'attribute'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleAttributeAnnotations(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a">
    <xs:annotation/>
    <xs:annotation/>
  </xs:attribute>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "parse attribute: at most one annotation element is allowed") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleAttributeSimpleTypes(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="a">
      <xs:simpleType>
        <xs:restriction base="xs:string"/>
      </xs:simpleType>
      <xs:simpleType>
        <xs:restriction base="xs:string"/>
      </xs:simpleType>
    </xs:attribute>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attribute cannot have multiple simpleType children") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsRestrictionMultipleSimpleTypes(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T">
    <xs:restriction>
      <xs:simpleType>
        <xs:restriction base="xs:string"/>
      </xs:simpleType>
      <xs:simpleType>
        <xs:restriction base="xs:integer"/>
      </xs:simpleType>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "restriction cannot have multiple simpleType children") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsRestrictionBaseAndInlineSimpleType(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T">
    <xs:restriction base="xs:string">
      <xs:simpleType>
        <xs:restriction base="xs:string"/>
      </xs:simpleType>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "restriction cannot have both base attribute and inline simpleType child") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsListMultipleSimpleTypes(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T">
    <xs:list>
      <xs:simpleType>
        <xs:restriction base="xs:string"/>
      </xs:simpleType>
      <xs:simpleType>
        <xs:restriction base="xs:integer"/>
      </xs:simpleType>
    </xs:list>
  </xs:simpleType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "list cannot have multiple simpleType children") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsUnionWithoutMembers(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T">
    <xs:union memberTypes=""/>
  </xs:simpleType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "union memberTypes attribute cannot be empty") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsSimpleTypeWithoutDerivation(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T">
    <xs:annotation/>
  </xs:simpleType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "simpleType must have exactly one derivation child") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsEmptySchemaFormDefaults(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "elementFormDefault attribute cannot be empty") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}

	_, err = ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" attributeFormDefault="">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attributeFormDefault attribute cannot be empty") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsUnexpectedAnyAttribute(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:a="urn:a">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##other" processContents="lax" minOccurs="1" maxOccurs="2" a:foreign="ok" b="bad"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "any: unexpected attribute 'b'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsUnexpectedAnyAttributeOnAnyAttribute(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:anyAttribute namespace="##other" b="bad"/>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "anyAttribute: unexpected attribute 'b'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsEmptyOccursAttributes(t *testing.T) {
	tests := []struct {
		name string
		xsd  string
		want string
	}{
		{
			name: "element minOccurs",
			xsd: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" minOccurs=""/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			want: "minOccurs attribute cannot be empty",
		},
		{
			name: "wildcard maxOccurs",
			xsd: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any maxOccurs=""/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			want: "maxOccurs attribute cannot be empty",
		},
		{
			name: "wildcard processContents",
			xsd: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any processContents=""/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			want: "processContents attribute cannot be empty",
		},
		{
			name: "model group maxOccurs",
			xsd: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence maxOccurs="">
        <xs:element name="child"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			want: "maxOccurs attribute cannot be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDocumentWithImportsOptions(strings.NewReader(tt.xsd))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseDocumentWithImportsOptions() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseDocumentRejectsAttributeNameAndRef(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:attribute name="global" type="xs:string"/>
  <xs:complexType name="T">
    <xs:attribute name="a" ref="tns:global"/>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attribute cannot have both 'name' and 'ref' attributes") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsElementWithoutNameOrRef(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:all>
      <xs:element/>
    </xs:all>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "element must have either 'name' or 'ref' attribute") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsAttributeWithoutNameOrRef(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute/>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attribute must have either 'name' or 'ref' attribute") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsEmptyAttributeRef(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute ref=""/>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attribute ref attribute cannot be empty") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsXMLNSAttributeName(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="xmlns"/>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attribute name cannot be 'xmlns'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsEmptyAttributeName(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name=""/>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attribute name attribute cannot be empty") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsEmptyElementName(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name=""/>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "element name attribute cannot be empty") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsAbsoluteIdentitySelector(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:unique name="u">
      <xs:selector xpath="/item"/>
      <xs:field xpath="@id"/>
    </xs:unique>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "selector xpath must be a relative path") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsIdentityFieldFunction(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:key name="k">
      <xs:selector xpath=".//item"/>
      <xs:field xpath="document('')"/>
    </xs:key>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "field xpath cannot use functions or parentheses") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsAttributeRefInlineSimpleType(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:tns="urn:test" targetNamespace="urn:test">
  <xs:attribute name="global" type="xs:string"/>
  <xs:complexType name="T">
    <xs:attribute ref="tns:global">
      <xs:simpleType>
        <xs:restriction base="xs:string"/>
      </xs:simpleType>
    </xs:attribute>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attribute reference cannot have inline simpleType") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsEmptyAttributeForm(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="a" form=""/>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "invalid form attribute value '': must be 'qualified' or 'unqualified'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsEmptyAttributeUse(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="a" use=""/>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "invalid use attribute value '': must be 'optional', 'prohibited', or 'required'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsUnexpectedAttributeDeclarationAttribute(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="a" value="string"/>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "invalid attribute 'value' on <attribute> element") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleGroupAnnotations(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:group name="G">
    <xs:annotation/>
    <xs:annotation/>
    <xs:sequence/>
  </xs:group>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "group 'G': at most one annotation is allowed") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleTopLevelGroupModels(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:group name="G">
    <xs:sequence/>
    <xs:choice/>
  </xs:group>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "group 'G': exactly one model group (all, choice, or sequence) is allowed") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleNotationAnnotations(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:notation name="png" public="image/png">
    <xs:annotation/>
    <xs:annotation/>
  </xs:notation>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "notation 'png': at most one annotation is allowed") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsInvalidNotationAttribute(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation foo="bar" name="jpeg" public="image/jpeg" system="viewer.exe"/>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "notation: unexpected attribute 'foo'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsNotationCharacterData(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="jpeg" public="image/jpeg" system="viewer.exe">Some Text</xs:notation>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "notation must not contain character data") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsNotationNonXSDChild(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="jpeg" public="image/jpeg" system="viewer.exe">
    <a>
      <b/>
    </a>
  </xs:notation>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "notation 'jpeg': unexpected child element 'a'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleWildcardAnnotations(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:complexType name="Root">
    <xs:anyAttribute>
      <xs:annotation/>
      <xs:annotation/>
    </xs:anyAttribute>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "anyAttribute: at most one annotation is allowed") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsDuplicateSchemaIDs(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="price">
    <xs:complexType>
      <xs:simpleContent id="anID">
        <xs:extension id="anID" base="xs:decimal"/>
      </xs:simpleContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "extension element has duplicate id attribute 'anID' (already used by simpleContent)") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsQNameLikeDeclarationName(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:simpleType name="tns:T">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "invalid type name 'tns:T': must be a valid NCName") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsRequiredAttributeDefault(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:string" use="required" default="x"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "attribute with use='required' cannot have default value") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsLateComplexTypeAnnotation(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:complexType name="T">
    <xs:all/>
    <xs:annotation/>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "complexType: annotation must appear before other elements") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsComplexTypeAttributeBeforeSimpleContent(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:attribute name="a"/>
    <xs:simpleContent>
      <xs:extension base="xs:string"/>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "complexType: simpleContent must be the only content model") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsInvalidSimpleContentRestrictionChildAsFacet(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G">
    <xs:sequence/>
  </xs:group>
  <xs:complexType name="T">
    <xs:simpleContent>
      <xs:restriction base="xs:string">
        <xs:group ref="G"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "parse facets: unknown or invalid facet 'group' (not a valid XSD 1.0 facet)") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsSimpleContentExtensionFacet(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:simpleContent>
      <xs:extension base="xs:string">
        <xs:length value="5"/>
      </xs:extension>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "simpleContent extension has unexpected child element 'length'") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentAllowsSimpleContentRestrictionAnnotation(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:simpleContent>
      <xs:extension base="xs:string"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="T">
    <xs:simpleContent>
      <xs:restriction base="Base">
        <xs:annotation/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`))
	if err != nil {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleComplexContentAnnotations(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:complexType name="Base"/>
  <xs:complexType name="T">
    <xs:complexContent>
      <xs:annotation/>
      <xs:annotation/>
      <xs:extension base="Base"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "complexContent: at most one annotation is allowed") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsMultipleModelGroupAnnotations(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:complexType name="T">
    <xs:choice>
      <xs:annotation/>
      <xs:annotation/>
      <xs:element name="a"/>
    </xs:choice>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "choice: at most one annotation is allowed") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsInvalidModelGroupAttribute(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:all name="bad">
      <xs:element name="a"/>
    </xs:all>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "invalid attribute 'name' on <all> (only id, minOccurs, maxOccurs allowed)") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsAllGroupReference(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:group name="G">
    <xs:all>
      <xs:group>
        <xs:sequence/>
      </xs:group>
    </xs:all>
  </xs:group>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "xs:all cannot contain group references") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsNestedAllGroup(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T">
    <xs:choice>
      <xs:all>
        <xs:element name="a"/>
      </xs:all>
    </xs:choice>
  </xs:complexType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "xs:all cannot be nested inside choice") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsTopLevelGroupParticleOccurs(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:group name="aa">
    <xs:choice maxOccurs="2">
      <xs:element name="a"/>
    </xs:choice>
  </xs:group>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "group 'aa' must have minOccurs='1' and maxOccurs='1' (got minOccurs=1, maxOccurs=2)") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsTopLevelGroupOccursAttribute(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:group name="aa" minOccurs="1">
    <xs:sequence/>
  </xs:group>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "invalid attribute 'minOccurs' on top-level group (only id, name allowed)") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsLateElementAnnotation(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root">
    <xs:complexType>
      <xs:all/>
    </xs:complexType>
    <xs:annotation/>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "element: annotation must appear before other elements") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsElementNameAndRef(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root"/>
  <xs:element name="parent">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" ref="root"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "element cannot have both 'name' and 'ref' attributes") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsElementRefBlock(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root"/>
  <xs:element name="parent">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="root" block="extension"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "invalid attribute 'block' on element reference") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsElementRefInlineType(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root"/>
  <xs:element name="parent">
    <xs:complexType>
      <xs:all>
        <xs:element ref="root">
          <xs:complexType/>
        </xs:element>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "element reference cannot have inline complexType") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsPrefixedSchemaAttribute(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xs:targetNamespace="urn:test">
  <xs:element name="root"/>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "schema attribute 'targetNamespace' on <schema> must be unprefixed") {
		t.Fatalf("ParseDocumentWithImportsOptionsWithPool() error = %v", err)
	}
}

func TestParseDocumentRejectsTopLevelAttributeForm(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:attribute name="id" type="xs:string" form="qualified"/>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "top-level attribute cannot have 'form' attribute") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsTopLevelElementForm(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root" type="xs:string" form="qualified"/>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "top-level element cannot have 'form' attribute") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsElementTypeAndInlineType(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:complexType name="T"/>
  <xs:element name="root" type="T">
    <xs:complexType/>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "element cannot have both 'type' attribute and inline type") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsElementSimpleTypeAndComplexType(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root">
    <xs:simpleType>
      <xs:restriction base="xs:string"/>
    </xs:simpleType>
    <xs:complexType/>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "element cannot have more than one inline type definition") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsElementComplexTypeAndSimpleType(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root">
    <xs:complexType/>
    <xs:simpleType>
      <xs:restriction base="xs:string"/>
    </xs:simpleType>
  </xs:element>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "element cannot have more than one inline type definition") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentRejectsNonPositiveTotalDigits(t *testing.T) {
	_, err := ParseDocumentWithImportsOptions(strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T">
    <xs:restriction base="xs:decimal">
      <xs:totalDigits value="0"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`))
	if err == nil || !strings.Contains(err.Error(), "totalDigits value must be positive, got 0") {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
}

func TestParseDocumentDeclGraphContainsNoGraphTypes(t *testing.T) {
	doc := parseSchemaDocumentForTest(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	assertNoGraphTypeRefs(t, reflect.TypeOf(*doc), map[reflect.Type]bool{})
}

func TestLoadDocumentSetFSOrderAndLexicalChameleonRemap(t *testing.T) {
	fsys := fstest.MapFS{
		"root.xsd": {Data: []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root">
  <xs:include schemaLocation="common.xsd"/>
  <xs:include schemaLocation="common.xsd"/>
  <xs:import namespace="urn:dep" schemaLocation="dep.xsd"/>
  <xs:element name="root" type="Included"/>
</xs:schema>`)},
		"common.xsd": {Data: []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Included"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:element name="includedRoot" type="Included"/>
  <xs:attributeGroup name="attrs">
    <xs:attribute name="localAttr" type="Included"/>
  </xs:attributeGroup>
</xs:schema>`)},
		"dep.xsd": {Data: []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:dep">
  <xs:element name="depRoot" type="xs:string"/>
</xs:schema>`)},
	}

	docs, err := LoadDocumentSetFS(fsys, "root.xsd")
	if err != nil {
		t.Fatalf("LoadDocumentSetFS() error = %v", err)
	}
	if len(docs.Documents) != 3 {
		t.Fatalf("document count = %d, want 3", len(docs.Documents))
	}
	wantLocations := []string{"root.xsd", "common.xsd", "dep.xsd"}
	for i, want := range wantLocations {
		if docs.Documents[i].Location != want {
			t.Fatalf("document %d location = %q, want %q", i, docs.Documents[i].Location, want)
		}
	}
	common := docs.Documents[1]
	if common.TargetNamespace != "urn:root" {
		t.Fatalf("common target namespace = %q, want urn:root", common.TargetNamespace)
	}
	if !contextHasBinding(common.NamespaceContexts[0], "", "urn:root") {
		t.Fatalf("common namespace context lacks remapped default binding: %#v", common.NamespaceContexts[0])
	}
	if common.Decls[0].Name != (QName{Namespace: "urn:root", Local: "Included"}) {
		t.Fatalf("included simple type name = %v", common.Decls[0].Name)
	}
	includedElem := common.Decls[1].Element
	if includedElem.Type.Name != (QName{Namespace: "urn:root", Local: "Included"}) {
		t.Fatalf("included element type = %v, want remapped Included", includedElem.Type.Name)
	}
	attrGroup := common.Decls[2].AttributeGroup
	if attrGroup == nil || len(attrGroup.Attributes) != 1 || attrGroup.Attributes[0].Attribute == nil {
		t.Fatalf("included attribute group = %#v", attrGroup)
	}
	localAttr := attrGroup.Attributes[0].Attribute
	if localAttr.Name != (QName{Local: "localAttr"}) {
		t.Fatalf("local attribute name = %v, want unqualified localAttr", localAttr.Name)
	}
	if localAttr.Type.Name != (QName{Namespace: "urn:root", Local: "Included"}) {
		t.Fatalf("local attribute type = %v, want remapped Included", localAttr.Type.Name)
	}
}

func parseSchemaDocumentForTest(t *testing.T, schema string) *SchemaDocument {
	t.Helper()
	result, err := ParseDocumentWithImportsOptions(strings.NewReader(schema))
	if err != nil {
		t.Fatalf("ParseDocumentWithImportsOptions() error = %v", err)
	}
	if result.Document == nil {
		t.Fatal("ParseDocumentWithImportsOptions() returned nil document")
	}
	return result.Document
}

func contextHasBinding(ctx NamespaceContext, prefix string, uri NamespaceURI) bool {
	for _, binding := range ctx.Bindings {
		if binding.Prefix == prefix && binding.URI == uri {
			return true
		}
	}
	return false
}

func assertNoGraphTypeRefs(t *testing.T, typ reflect.Type, seen map[reflect.Type]bool) {
	t.Helper()
	if typ == nil {
		return
	}
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	if seen[typ] {
		return
	}
	seen[typ] = true
	switch typ.Kind() {
	case reflect.Interface:
		t.Fatalf("parse-only document contains interface type %s", typ)
	case reflect.Struct:
		for i := range typ.NumField() {
			assertNoGraphTypeRefs(t, typ.Field(i).Type, seen)
		}
	}
}

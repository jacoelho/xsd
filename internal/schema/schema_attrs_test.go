package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func testRawNodeWithLocalAttrs(local string, attrs []string) *schemaSyntaxNode {
	rawAttrs := make([]xmlstream.Attr, len(attrs))
	for i, attr := range attrs {
		rawAttrs[i] = testRawAttr("", attr, "")
	}
	return testXSDRawNode(local, rawAttrs)
}

func TestReferenceAttributesRejectedAtStartInDocumentOrder(t *testing.T) {
	t.Parallel()
	invalid := func(name, body, msg string, line, col int) schemaStartFixture {
		return schemaStartFixture{name: name, body: body, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: msg, wantLine: line, wantCol: col}
	}
	for _, fixture := range []schemaStartFixture{
		{name: "element ref accepts occurrence attrs", body: `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element id="i" ref="Target" minOccurs="0" maxOccurs="2"/>
    </xs:sequence>
  </xs:complexType>`},
		{name: "attribute ref accepts value constraint attrs", body: `  <xs:complexType name="T">
    <xs:attribute id="i" ref="Target" use="optional" default="x"/>
  </xs:complexType>`},
		{name: "group ref accepts occurrence attrs", body: `  <xs:complexType name="T">
    <xs:sequence>
      <xs:group id="i" ref="Target" minOccurs="0" maxOccurs="2"/>
    </xs:sequence>
  </xs:complexType>`},
		{name: "foreign attributes are not reference attributes", body: `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="Target" f:type="xs:string" f:form=""/>
    </xs:sequence>
  </xs:complexType>`},
		invalid("element ref type", `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="Target" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>`, `element ref cannot have attribute "type"`, 4, 7),
		invalid("element ref first forbidden attribute wins", `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="Target" form="" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>`, `element ref cannot have attribute "form"`, 4, 7),
		invalid("element ref unknown attribute before forbidden one", `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element foo="1" ref="Target" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>`, `"element" cannot have attribute "foo"`, 4, 7),
		invalid("element ref attribute before invalid child", `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="Target" nillable="true">
        <xs:complexType/>
      </xs:element>
    </xs:sequence>
  </xs:complexType>`, `element ref cannot have attribute "nillable"`, 4, 7),
		invalid("attribute ref name before form", `  <xs:complexType name="T">
    <xs:attribute ref="Target" name="Local" form=""/>
  </xs:complexType>`, `attribute ref cannot have attribute "name"`, 3, 5),
		invalid("attribute ref form before name", `  <xs:complexType name="T">
    <xs:attribute ref="Target" form="" name="Local"/>
  </xs:complexType>`, `attribute ref cannot have attribute "form"`, 3, 5),
		invalid("attribute ref attribute before invalid child", `  <xs:complexType name="T">
    <xs:attribute ref="Target" type="xs:string">
      <xs:simpleType/>
    </xs:attribute>
  </xs:complexType>`, `attribute ref cannot have attribute "type"`, 3, 5),
		invalid("nested group name before invalid child", `  <xs:complexType name="T">
    <xs:sequence>
      <xs:group ref="Target" name="g">
        <xs:sequence/>
      </xs:group>
    </xs:sequence>
  </xs:complexType>`, `group cannot have attribute "name"`, 4, 7),
		invalid("attributeGroup use name before invalid child", `  <xs:complexType name="T">
    <xs:attributeGroup ref="Target" name="g">
      <xs:attribute name="a"/>
    </xs:attributeGroup>
  </xs:complexType>`, `attributeGroup use cannot have attribute "name"`, 3, 5),
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSchemaStartFixture(t, fixture)
			assertSchemaStartFixture(t, err, fixture)
		})
	}
}

func TestCheckRawSchemaAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		element     string
		attrs       []string
		wantMessage string
	}{
		{
			name:    "schema accepts document attributes",
			element: "schema",
			attrs:   []string{"id", "targetNamespace", "version", "finalDefault", "blockDefault", "attributeFormDefault", "elementFormDefault"},
		},
		{
			name:        "schema rejects unrelated attribute",
			element:     "schema",
			attrs:       []string{"type"},
			wantMessage: "\"schema\" cannot have attribute \"type\"",
		},
		{
			name:    "element accepts declaration and occurrence attributes",
			element: "element",
			attrs:   []string{"id", "name", "ref", "type", "substitutionGroup", "nillable", "default", "fixed", "form", "block", "final", "abstract", "minOccurs", "maxOccurs"},
		},
		{
			name:        "attribute rejects nillable",
			element:     "attribute",
			attrs:       []string{"name", "nillable"},
			wantMessage: "\"attribute\" cannot have attribute \"nillable\"",
		},
		{
			name:    "ordinary facet accepts fixed",
			element: "maxLength",
			attrs:   []string{"id", "value", "fixed"},
		},
		{
			name:        "pattern rejects fixed",
			element:     "pattern",
			attrs:       []string{"value", "fixed"},
			wantMessage: "\"pattern\" cannot have attribute \"fixed\"",
		},
		{
			name:        "enumeration rejects fixed",
			element:     "enumeration",
			attrs:       []string{"value", "fixed"},
			wantMessage: "\"enumeration\" cannot have attribute \"fixed\"",
		},
		{
			name:        "facet rejects unknown attribute",
			element:     "maxLength",
			attrs:       []string{"value", "ref"},
			wantMessage: "\"maxLength\" cannot have attribute \"ref\"",
		},
		{
			name:    "unknown xsd element remains permissive",
			element: "futureElement",
			attrs:   []string{"futureAttr"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := checkElementChildAttributes(testRawNodeWithLocalAttrs(tt.element, tt.attrs))
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("checkSchemaAttributesSyntax() error = %v", err)
			}
		})
	}
}

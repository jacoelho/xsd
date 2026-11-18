package schema

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func testRawNode(name xml.Name, attrs []xmlstream.Attr) *schemaSyntaxNode {
	// Test attributes are owned (NewAttr), so a zero Reader never materializes.
	normalized, err := schemaAttributesStream(new(xmlstream.Reader), name, attrs)
	if err != nil {
		panic(err)
	}
	return &schemaSyntaxNode{
		Name:  name,
		Attrs: normalized,
	}
}

func testXSDRawNode(local string, attrs []xmlstream.Attr) *schemaSyntaxNode {
	return testRawNode(xml.Name{Space: vocab.XSDNamespaceURI, Local: local}, attrs)
}

func testRawAttr(namespace, local, value string) xmlstream.Attr {
	return xmlstream.NewAttr(xml.Name{Space: namespace, Local: local}, value)
}

type schemaStartFixture struct {
	name       string
	body       string
	rootAttrs  string
	wantOpaque bool
	wantCat    xsderrors.Category
	wantCode   xsderrors.Code
	wantMsg    string
	wantLine   int
	wantCol    int
}

func parseSchemaStartFixture(t *testing.T, fixture schemaStartFixture) (*schemaDocument, error) {
	t.Helper()
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	input := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:f="urn:foreign"` + fixture.rootAttrs + `>` + "\n" + fixture.body + "\n</xs:schema>"
	return parseSchemaDocument("start.xsd", "start.xsd", strings.NewReader(input), limits, new(xmlstream.Reader), testSchemaAdmission(limits))
}

func assertSchemaStartFixture(t *testing.T, err error, fixture schemaStartFixture) {
	t.Helper()
	if fixture.wantMsg == "" {
		if err != nil {
			t.Fatalf("parseSchemaDocument() error = %v", err)
		}
		return
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want structured diagnostic", err)
	}
	if diagnostic.Category() != fixture.wantCat || diagnostic.Code() != fixture.wantCode || diagnostic.Message() != fixture.wantMsg {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diagnostic.Category(), diagnostic.Code(), diagnostic.Message(), fixture.wantCat, fixture.wantCode, fixture.wantMsg)
	}
	if diagnostic.Path() != "start.xsd" || diagnostic.Line() != fixture.wantLine || diagnostic.Column() != fixture.wantCol {
		t.Fatalf("location = (%q, %d, %d), want (start.xsd, %d, %d)", diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), fixture.wantLine, fixture.wantCol)
	}
}

func TestCheckTopLevelSchemaChild(t *testing.T) {
	t.Parallel()
	for _, fixture := range []schemaStartFixture{
		{name: "annotation", body: `  <xs:annotation/>`},
		{name: "include", body: `  <xs:include schemaLocation="other.xsd"/>`},
		{name: "notation", body: `  <xs:notation name="N" public=""/>`},
		{name: "simple type named", body: `  <xs:simpleType name="T"><xs:restriction base="xs:string"/></xs:simpleType>`},
		{name: "complex type named", body: `  <xs:complexType name="T"/>`},
		{name: "attributeGroup named", body: `  <xs:attributeGroup name="Attrs"/>`},
		{name: "unknown child", body: `  <xs:selector xpath="."/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "identity constraint must be inside element", wantLine: 2, wantCol: 3},
		{name: "invalid child", body: `  <xs:foo/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "invalid top-level schema child \"foo\"", wantLine: 2, wantCol: 3},
		{name: "simple type missing name", body: `  <xs:simpleType/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaReference, wantMsg: "top-level \"simpleType\" missing name", wantLine: 2, wantCol: 3},
		{name: "attributeGroup ref", body: `  <xs:attributeGroup name="Attrs" ref="Other"/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level \"attributeGroup\" cannot have \"ref\"", wantLine: 2, wantCol: 3},
		{name: "group minOccurs", body: `  <xs:group name="Group" minOccurs=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level \"group\" cannot have \"minOccurs\"", wantLine: 2, wantCol: 3},
		{name: "attribute form", body: `  <xs:attribute name="Attr" form=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level \"attribute\" cannot have \"form\"", wantLine: 2, wantCol: 3},
		{name: "element ref", body: `  <xs:element name="Root" ref="Other"/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level \"element\" cannot have \"ref\"", wantLine: 2, wantCol: 3},
		{name: "group: first forbidden attribute in document order wins over missing name", body: `  <xs:group maxOccurs="" minOccurs="" ref=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level \"group\" cannot have \"maxOccurs\"", wantLine: 2, wantCol: 3},
		{name: "attribute: first forbidden attribute in document order wins over missing name", body: `  <xs:attribute use="" form="" ref=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level \"attribute\" cannot have \"use\"", wantLine: 2, wantCol: 3},
		{name: "element: first forbidden attribute in document order wins over missing name", body: `  <xs:element maxOccurs="" minOccurs="" form="" ref=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level \"element\" cannot have \"maxOccurs\"", wantLine: 2, wantCol: 3},
		{name: "forbidden attr wins before later empty name", body: `  <xs:element ref="" name=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level \"element\" cannot have \"ref\"", wantLine: 2, wantCol: 3},
		{name: "forbidden attr wins before later whitespace name", body: `  <xs:group minOccurs="" name=" "/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level \"group\" cannot have \"minOccurs\"", wantLine: 2, wantCol: 3},
		{name: "empty name wins before later forbidden attr", body: `  <xs:element name="" ref=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "schema component name must be NCName", wantLine: 2, wantCol: 3},
		{name: "unknown attr wins before later invalid name", body: `  <xs:element foo="x" name="1"/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "\"element\" cannot have attribute \"foo\"", wantLine: 2, wantCol: 3},
		{name: "invalid xml:base wins before later unknown attr", body: `  <xs:element xml:base="%zz" foo="x" name="e"/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaReference, wantMsg: "invalid xml:base: invalid anyURI", wantLine: 2, wantCol: 3},
		{name: "unknown attr wins before later schema namespace attr", body: `  <xs:element foo="x" xs:bar="" name="e"/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "\"element\" cannot have attribute \"foo\"", wantLine: 2, wantCol: 3},
		{name: "reference attr wins before invalid name syntax", body: "  <xs:element name=\"x\"/>\n  <xs:complexType name=\"C\"><xs:sequence>\n    <xs:element ref=\"x\" name=\"a:b\"/>\n  </xs:sequence></xs:complexType>", wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "element ref cannot have attribute \"name\"", wantLine: 4, wantCol: 5},
		{name: "local definition name wins before its syntax", body: "  <xs:element name=\"x\">\n    <xs:complexType name=\"1\"/>\n  </xs:element>", wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "local complexType cannot have name", wantLine: 3, wantCol: 5},
		{name: "earlier parent text wins before child attr", body: "  <xs:complexType name=\"C\"><xs:sequence>text\n    <xs:element name=\"a\" foo=\"1\"/>\n  </xs:sequence></xs:complexType>", wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "xs:\"sequence\" cannot contain text", wantLine: 2, wantCol: 28},
		{name: "earlier parent text wins before foreign child", body: "  <xs:complexType name=\"C\"><xs:sequence>text\n    <f:other/>\n  </xs:sequence></xs:complexType>", wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "xs:\"sequence\" cannot contain text", wantLine: 2, wantCol: 28},
		{name: "foreign attributes do not create forbidden presence", body: `  <xs:element name="Root" f:ref="" f:form="" f:minOccurs="" f:maxOccurs=""/>`},
		{name: "start general attr wins before end text", body: `  <xs:element name="Root" foo="">text</xs:element>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "\"element\" cannot have attribute \"foo\"", wantLine: 2, wantCol: 3},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSchemaStartFixture(t, fixture)
			assertSchemaStartFixture(t, err, fixture)
		})
	}
}

func TestCheckUnsupportedSchemaNode(t *testing.T) {
	t.Parallel()
	for _, fixture := range []schemaStartFixture{
		{name: "appinfo under annotation skips subtree", wantOpaque: true, body: `  <xs:annotation id="payloadID">
    <xs:appinfo f:unknown="">
      <xs:redefine xs:foo="" id=""><f:payload/></xs:redefine>
      <xs:element id="payloadID" name="0"/>
    </xs:appinfo>
  </xs:annotation>`},
		{name: "non schema node", body: `  <f:other/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "foreign element \"other\" is not allowed in schema grammar", wantLine: 2, wantCol: 3},
		{name: "schema namespace attribute", rootAttrs: ` xs:foo=""`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "schema namespace attribute \"foo\" is not allowed", wantLine: 1, wantCol: 1},
		{name: "redefine", body: `  <xs:redefine/>`, wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedRedefine, wantMsg: "xs:redefine is not supported", wantLine: 2, wantCol: 3},
		{name: "top-level notation", body: `  <xs:notation name="N" public=""/>`},
		{name: "nested notation", body: `  <xs:element name="Root">
    <xs:notation name="N" public=""/>
  </xs:element>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "invalid element child \"notation\"", wantLine: 3, wantCol: 5},
		{name: "redefine wins before namespace attrs and names", body: `  <xs:redefine xs:foo="" id=""/>`, wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedRedefine, wantMsg: "xs:redefine is not supported", wantLine: 2, wantCol: 3},
		{name: "redefine wins before invalid name", body: `  <xs:redefine name=""/>`, wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedRedefine, wantMsg: "xs:redefine is not supported", wantLine: 2, wantCol: 3},
		{name: "redefine after declaration", body: "  <xs:element name=\"e\"/>\n  <xs:redefine schemaLocation=\"other.xsd\"/>", wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedRedefine, wantMsg: "xs:redefine is not supported", wantLine: 3, wantCol: 3},
		{name: "redefine in annotation", body: "  <xs:annotation>\n    <xs:redefine schemaLocation=\"other.xsd\"/>\n  </xs:annotation>", wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedRedefine, wantMsg: "xs:redefine is not supported", wantLine: 3, wantCol: 5},
		{name: "redefine in notation", body: "  <xs:notation name=\"N\" public=\"p\">\n    <xs:redefine/>\n  </xs:notation>", wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedRedefine, wantMsg: "xs:redefine is not supported", wantLine: 3, wantCol: 5},
		{name: "redefine in facet", body: "  <xs:simpleType name=\"S\"><xs:restriction base=\"xs:string\"><xs:length value=\"1\">\n    <xs:redefine/>\n  </xs:length></xs:restriction></xs:simpleType>", wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedRedefine, wantMsg: "xs:redefine is not supported", wantLine: 3, wantCol: 5},
		{name: "redefine in model group", body: "  <xs:complexType name=\"C\"><xs:sequence>\n    <xs:redefine/>\n  </xs:sequence></xs:complexType>", wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedRedefine, wantMsg: "xs:redefine is not supported", wantLine: 3, wantCol: 5},
		{name: "redefine wins over earlier text", body: "  <xs:complexType name=\"C\"><xs:sequence>text\n    <xs:redefine/>\n  </xs:sequence></xs:complexType>", wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedRedefine, wantMsg: "xs:redefine is not supported", wantLine: 3, wantCol: 5},
		{name: "foreign rejection wins before namespace attrs and names", body: `  <f:other xs:foo="" id=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "foreign element \"other\" is not allowed in schema grammar", wantLine: 2, wantCol: 3},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			doc, err := parseSchemaStartFixture(t, fixture)
			assertSchemaStartFixture(t, err, fixture)
			if fixture.wantOpaque {
				annotation := doc.root.firstXS(annotationChild)
				if annotation == nil || len(annotation.children) != 0 || len(doc.arena) != 2 {
					t.Fatalf("opaque payload retained grammar nodes: %#v", doc)
				}
			}
		})
	}
}

func TestSchemaIDsAreUniqueInTheDocument(t *testing.T) {
	t.Parallel()
	const invalidAttribute = xsderrors.CodeSchemaInvalidAttribute
	for _, fixture := range []schemaStartFixture{
		{name: "no ids", body: `  <xs:element name="a"/>`},
		{name: "distinct ids", rootAttrs: ` id="r"`, body: `  <xs:element name="a" id="a"/>` + "\n" + `  <xs:element name="b" id="b"/>`},
		{name: "ids in annotation payload are not schema ids", body: `  <xs:annotation id="a"><xs:appinfo><f:x id="a"/><f:x id="a"/></xs:appinfo></xs:annotation>`},
		{name: "second declaration of an id", body: `  <xs:element name="a" id="a"/>` + "\n" + `  <xs:element name="b" id="b"/>` + "\n" + `    <xs:element name="c" id="a"/>`,
			wantCat: xsderrors.CategorySchemaCompile, wantCode: invalidAttribute, wantMsg: `duplicate schema id "a"`, wantLine: 4, wantCol: 5},
		{name: "id of the root declared again", rootAttrs: ` id="r"`, body: `  <xs:element name="a" id="r"/>`,
			wantCat: xsderrors.CategorySchemaCompile, wantCode: invalidAttribute, wantMsg: `duplicate schema id "r"`, wantLine: 2, wantCol: 3},
		{name: "id declared again by a descendant", body: `  <xs:element name="a" id="a"><xs:complexType id="a"/></xs:element>`,
			wantCat: xsderrors.CategorySchemaCompile, wantCode: invalidAttribute, wantMsg: `duplicate schema id "a"`, wantLine: 2, wantCol: 31},
		{name: "attribute written before the duplicate id wins", body: `  <xs:element name="a" id="a"/>` + "\n" + `  <xs:element name="b" nillable="x" id="a"/>`,
			wantCat: xsderrors.CategorySchemaCompile, wantCode: invalidAttribute, wantMsg: `invalid boolean attribute "nillable"`, wantLine: 3, wantCol: 3},
		{name: "duplicate id wins over an attribute written after it", body: `  <xs:element name="a" id="a"/>` + "\n" + `  <xs:element name="b" id="a" nillable="x"/>`,
			wantCat: xsderrors.CategorySchemaCompile, wantCode: invalidAttribute, wantMsg: `duplicate schema id "a"`, wantLine: 3, wantCol: 3},
		{name: "duplicate id wins over a later element", body: `  <xs:element name="a" id="a"/>` + "\n" + `  <xs:element name="b" id="a"/>` + "\n" + `  <xs:foo/>`,
			wantCat: xsderrors.CategorySchemaCompile, wantCode: invalidAttribute, wantMsg: `duplicate schema id "a"`, wantLine: 3, wantCol: 3},
		{name: "earlier element wins over a duplicate id", body: `  <xs:element name="a" id="a"/>` + "\n" + `  <xs:foo/>` + "\n" + `  <xs:element name="b" id="a"/>`,
			wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: `invalid top-level schema child "foo"`, wantLine: 3, wantCol: 3},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSchemaStartFixture(t, fixture)
			assertSchemaStartFixture(t, err, fixture)
		})
	}
}

func TestValidateSchemaTargetNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		hasTarget bool
		target    string
		wantMsg   string
	}{
		{name: "missing"},
		{name: "non empty", hasTarget: true, target: "urn:test"},
		{name: "empty", hasTarget: true, wantMsg: "schema targetNamespace cannot be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateSchemaTargetNamespace(lexicalAttribute{Value: tt.target, Present: tt.hasTarget})
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateSchemaTargetNamespace() error = %v", err)
				}
				return
			}
			diag, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("ValidateSchemaTargetNamespace() error = %T %[1]v, want xsderrors.Error", err)
			}
			if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != xsderrors.CodeSchemaInvalidAttribute || diag.Message() != tt.wantMsg {
				t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category(), diag.Code(), diag.Message(), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute, tt.wantMsg)
			}
		})
	}
}

func TestValidateSchemaNameAttribute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, attr, value, wantMsg string
	}{
		{name: "valid id", attr: vocab.XSDAttrID, value: "id1"},
		{name: "valid name", attr: vocab.XSDAttrName, value: "root"},
		{name: "invalid id", attr: vocab.XSDAttrID, value: ":bad", wantMsg: "schema id must be NCName"},
		{name: "invalid name", attr: vocab.XSDAttrName, value: "0", wantMsg: "schema component name must be NCName"},
		{name: "other attribute", attr: vocab.XSDAttrType, value: "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateSchemaNameAttribute(tt.attr, tt.value)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("validateSchemaNameAttribute() error = %v", err)
				}
				return
			}
			expectInvalidAttributeMessage(t, err, tt.wantMsg)
		})
	}
}

func checkElementChildAttributes(n *schemaSyntaxNode) error {
	var values admittedValues
	var ids schemaIDs
	admission := testSchemaAdmission(defaultPatternLimits)
	return checkSchemaAttributesSyntax(n, schemaPlacementOther, startTagAttributes{values: &values, ids: &ids, set: &admission})
}

func TestSchemaAnnotationStartAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		local    string
		attrs    []xmlstream.Attr
		validate func(*schemaSyntaxNode) error
		wantMsg  string
	}{
		{
			name: "annotation allows id attr", local: annotationChild,
			attrs:    []xmlstream.Attr{testRawAttr("", vocab.XSDAttrID, "a1")},
			validate: checkElementChildAttributes,
		},
		{
			name: "annotation rejects local attr", local: annotationChild,
			attrs:    []xmlstream.Attr{testRawAttr("", "foo", "")},
			validate: checkElementChildAttributes, wantMsg: "\"annotation\" cannot have attribute \"foo\"",
		},
		{
			name: "documentation valid lang", local: vocab.XSDElemDocumentation,
			attrs:    []xmlstream.Attr{testRawAttr(vocab.XMLNamespaceURI, vocab.XMLAttrLang, "en-US")},
			validate: checkElementChildAttributes,
		},
		{
			name: "documentation invalid lang", local: vocab.XSDElemDocumentation,
			attrs:    []xmlstream.Attr{testRawAttr(vocab.XMLNamespaceURI, vocab.XMLAttrLang, " ")},
			validate: checkElementChildAttributes, wantMsg: "invalid xml:lang on xs:documentation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node := testXSDRawNode(tt.local, tt.attrs)
			node.doc = &schemaDocument{name: "annotation.xsd"}
			node.Line, node.Column = 3, 5
			err := tt.validate(node)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("start attribute check error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("start attribute check error = %T %[1]v, want structured diagnostic", err)
			}
			if issue.Category() != xsderrors.CategorySchemaCompile || issue.Code() != xsderrors.CodeSchemaInvalidAttribute || issue.Message() != tt.wantMsg {
				t.Fatalf("diagnostic = {%s %s %q}, want {%s %s %q}", issue.Category(), issue.Code(), issue.Message(), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute, tt.wantMsg)
			}
			if issue.Path() != "annotation.xsd" || issue.Line() != 3 || issue.Column() != 5 {
				t.Fatalf("location = {%q %d %d}, want {%q 3 5}", issue.Path(), issue.Line(), issue.Column(), "annotation.xsd")
			}
		})
	}
}

package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestTypedParserDoesNotRetainGenericSyntaxTree(t *testing.T) {
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseSchemaDocument("typed.xsd", "typed.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation id="outer">
    <xs:appinfo><vendor:payload xmlns:vendor="urn:vendor"><vendor:item/></vendor:payload></xs:appinfo>
    <xs:documentation xml:lang="en-US">text<xs:element id="outer" name="not a schema name"><xs:future/></xs:element></xs:documentation>
  </xs:annotation>
  <xs:simpleType name="value"><xs:restriction base="xs:string"><xs:minLength value="1"/></xs:restriction></xs:simpleType>
</xs:schema>`), limits, new(xmlstream.Reader))
	if err != nil {
		t.Fatal(err)
	}
	if doc.root == nil || len(doc.root.children) != 2 {
		t.Fatalf("root = %#v, want root with annotation and simpleType", doc.root)
	}
	var restriction *schemaNode
	for _, id := range doc.root.children {
		n := doc.node(id)
		if n != nil && n.local == simpleTypeChild {
			restriction = doc.node(n.children[0])
		}
	}
	if restriction == nil || restriction.local != restrictionChild {
		t.Fatalf("restriction node = %#v; want retained admitted XSD node", restriction)
	}
	if got := len(doc.arena); got != 5 {
		t.Fatalf("retained semantic nodes = %d, want root, annotation, simpleType, restriction, facet", got)
	}
	annotation := doc.node(doc.root.children[0])
	if annotation == nil || len(annotation.children) != 0 {
		t.Fatal("annotation payload was retained as a semantic child")
	}
}

func TestSchemaEndAdmissionDiagnosticPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		wantLine int
		wantCol  int
		wantCode xsderrors.Code
		wantMsg  string
	}{
		{
			name: "include text before invalid child",
			body: `  <xs:include>
    text
    <xs:attribute/>
  </xs:include>`,
			wantLine: 2, wantCol: 3, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:\"include\" cannot contain text",
		},
		{
			name: "import text before invalid child",
			body: `  <xs:import>
    text
    <xs:attribute/>
  </xs:import>`,
			wantLine: 2, wantCol: 3, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:\"import\" cannot contain text",
		},
		{
			name: "include invalid child before duplicate annotation",
			body: `  <xs:include>
    <xs:annotation/>
    <xs:attribute/>
    <xs:annotation/>
  </xs:include>`,
			wantLine: 4, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:\"include\" can contain only xs:annotation, not xs:\"attribute\"",
		},
		{
			name: "import invalid child before duplicate annotation",
			body: `  <xs:import>
    <xs:annotation/>
    <xs:attribute/>
    <xs:annotation/>
  </xs:import>`,
			wantLine: 4, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:\"import\" can contain only xs:annotation, not xs:\"attribute\"",
		},
		{
			name: "component duplicate annotation after nonannotation",
			body: `  <xs:complexType name="C">
    <xs:annotation/>
    <xs:attribute/>
    <xs:annotation/>
  </xs:complexType>`,
			wantLine: 5, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "schema component cannot contain multiple annotations",
		},
		{
			name: "component first late annotation before duplicate",
			body: `  <xs:complexType name="C">
    <xs:attribute/>
    <xs:annotation/>
    <xs:annotation/>
  </xs:complexType>`,
			wantLine: 4, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "\"complexType\" annotation must be first",
		},
		{
			name: "include duplicate annotation before invalid child",
			body: `  <xs:include>
    <xs:annotation/>
    <xs:annotation/>
    <xs:attribute/>
  </xs:include>`,
			wantLine: 4, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:\"include\" can contain at most one xs:annotation",
		},
		{
			name: "import duplicate annotation before invalid child",
			body: `  <xs:import>
    <xs:annotation/>
    <xs:annotation/>
    <xs:attribute/>
  </xs:import>`,
			wantLine: 4, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:\"import\" can contain at most one xs:annotation",
		},
		{
			name: "nested annotation source position",
			body: `  <xs:annotation>
    <xs:annotation/>
  </xs:annotation>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationPayloadOnly("annotation"),
		},
		{
			name:     "annotation attribute start admission",
			body:     `  <xs:annotation foo=""/>`,
			wantLine: 2, wantCol: 3, wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg: "\"annotation\" cannot have attribute \"foo\"",
		},
		{
			name: "include child grammar before documentation language",
			body: `  <xs:include>
    <xs:documentation xml:lang=" "/>
  </xs:include>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("include", "documentation"),
		},
		{
			name: "documentation language start admission",
			body: `  <xs:annotation>
    <xs:documentation xml:lang=" "/>
  </xs:annotation>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg: "invalid xml:lang on xs:documentation",
		},
		{
			name: "annotation child grammar before child attributes",
			body: `  <xs:annotation>
    <xs:element bogus="1"/>
  </xs:annotation>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationPayloadOnly("element"),
		},
		{
			name: "annotation child grammar before unsupported child",
			body: `  <xs:annotation>
    <xs:redefine schemaLocation="other.xsd"/>
  </xs:annotation>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationPayloadOnly("redefine"),
		},
		{
			name: "nested annotation before later forbidden sibling",
			body: `  <xs:annotation>
    <xs:annotation/>
    <xs:element name="e"/>
  </xs:annotation>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationPayloadOnly("annotation"),
		},
		{
			name: "nested annotation before its forbidden descendant",
			body: `  <xs:annotation>
    <xs:annotation>
      <xs:element name="e"/>
    </xs:annotation>
  </xs:annotation>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationPayloadOnly("annotation"),
		},
		{
			name: "annotation text before forbidden child",
			body: `  <xs:annotation>
    text
    <xs:element name="e"/>
  </xs:annotation>`,
			wantLine: 2, wantCol: 3, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:\"annotation\" cannot contain text",
		},
		{
			name:     "facet child grammar before unsupported child",
			body:     facetBody(`<xs:assert test="1"/>`),
			wantLine: 4, wantCol: 28, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("length", "assert"),
		},
		{
			name:     "facet child grammar before child attributes",
			body:     facetBody(`<xs:element bogus="1"/>`),
			wantLine: 4, wantCol: 28, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("length", "element"),
		},
		{
			name:     "facet child grammar before top-level-only child",
			body:     facetBody(`<xs:notation name="N" public="p"/>`),
			wantLine: 4, wantCol: 28, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("length", "notation"),
		},
		{
			name:     "facet forbidden child before its invalid descendant",
			body:     facetBody(`<xs:element name="e"><xs:bogus/></xs:element>`),
			wantLine: 4, wantCol: 28, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("length", "element"),
		},
		{
			name:     "facet bare documentation",
			body:     facetBody(`<xs:documentation/>`),
			wantLine: 4, wantCol: 28, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("length", "documentation"),
		},
		{
			name:     "facet duplicate annotation",
			body:     facetBody(`<xs:annotation/><xs:annotation/>`),
			wantLine: 4, wantCol: 44, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:\"length\" can contain at most one xs:annotation",
		},
		{
			name: "facet in simpleContent restriction",
			body: `  <xs:complexType name="C">
    <xs:simpleContent>
      <xs:restriction base="xs:string">
        <xs:enumeration value="a"><xs:element name="e"/></xs:enumeration>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>`,
			wantLine: 5, wantCol: 35, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("enumeration", "element"),
		},
		{
			name: "notation forbidden child",
			body: `  <xs:notation name="N" public="p">
    <xs:documentation/>
  </xs:notation>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("notation", "documentation"),
		},
		{
			name: "selector forbidden child",
			body: `  <xs:element name="r">
    <xs:key name="k">
      <xs:selector xpath="."><xs:field xpath="@a"/></xs:selector>
      <xs:field xpath="@a"/>
    </xs:key>
  </xs:element>`,
			wantLine: 4, wantCol: 30, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("selector", "field"),
		},
		{
			name: "field duplicate annotation",
			body: `  <xs:element name="r">
    <xs:key name="k">
      <xs:selector xpath="."/>
      <xs:field xpath="@a"><xs:annotation/><xs:annotation/></xs:field>
    </xs:key>
  </xs:element>`,
			wantLine: 5, wantCol: 44, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:\"field\" can contain at most one xs:annotation",
		},
		{
			name: "wildcard forbidden child",
			body: `  <xs:complexType name="C">
    <xs:sequence><xs:any><xs:element name="e"/></xs:any></xs:sequence>
  </xs:complexType>`,
			wantLine: 3, wantCol: 26, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("any", "element"),
		},
		{
			name: "attribute wildcard forbidden child",
			body: `  <xs:complexType name="C">
    <xs:anyAttribute><xs:attribute name="a"/></xs:anyAttribute>
  </xs:complexType>`,
			wantLine: 3, wantCol: 22, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("anyAttribute", "attribute"),
		},
		{
			name: "group reference forbidden child",
			body: `  <xs:complexType name="C">
    <xs:group ref="G"><xs:sequence/></xs:group>
  </xs:complexType>`,
			wantLine: 3, wantCol: 23, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("group", "sequence"),
		},
		{
			name: "attributeGroup reference forbidden child",
			body: `  <xs:complexType name="C">
    <xs:attributeGroup ref="G"><xs:attribute name="a"/></xs:attributeGroup>
  </xs:complexType>`,
			wantLine: 3, wantCol: 32, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: annotationOnly("attributeGroup", "attribute"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			limits, err := NormalizeOptions(Options{})
			if err != nil {
				t.Fatal(err)
			}
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + "\n" + tt.body + "\n</xs:schema>"
			_, err = parseSchemaDocument("admission.xsd", "admission.xsd", strings.NewReader(schema), limits, new(xmlstream.Reader))
			issue, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("parseSchemaDocument() error = %T %[1]v, want structured diagnostic", err)
			}
			if issue.Category() != xsderrors.CategorySchemaCompile || issue.Code() != tt.wantCode || issue.Message() != tt.wantMsg {
				t.Fatalf("diagnostic = {%s %s %q}, want {%s %s %q}", issue.Category(), issue.Code(), issue.Message(), xsderrors.CategorySchemaCompile, tt.wantCode, tt.wantMsg)
			}
			if issue.Path() != "admission.xsd" || issue.Line() != tt.wantLine || issue.Column() != tt.wantCol {
				t.Fatalf("location = {%q %d %d}, want {%q %d %d}", issue.Path(), issue.Line(), issue.Column(), "admission.xsd", tt.wantLine, tt.wantCol)
			}
		})
	}
}

func TestDocumentationLanguageOutsideAnnotation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		lang     string
		wantCode xsderrors.Code
		wantMsg  string
	}{
		{
			name: "invalid language before complexType child grammar", lang: " ",
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:  "invalid xml:lang on xs:documentation",
		},
		{
			name: "valid language reaches complexType child grammar", lang: "en-US",
			wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg:  "invalid complexType child \"documentation\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="C">
    <xs:documentation xml:lang="` + tt.lang + `"/>
  </xs:complexType>
</xs:schema>`
			_, err := Compile(Options{}, []source.Source{source.Bytes("language.xsd", []byte(schema))})
			issue, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("Compile() error = %T %[1]v, want structured diagnostic", err)
			}
			if issue.Category() != xsderrors.CategorySchemaCompile || issue.Code() != tt.wantCode || issue.Message() != tt.wantMsg {
				t.Fatalf("diagnostic = {%s %s %q}, want {%s %s %q}", issue.Category(), issue.Code(), issue.Message(), xsderrors.CategorySchemaCompile, tt.wantCode, tt.wantMsg)
			}
			if issue.Path() != "language.xsd" || issue.Line() != 3 || issue.Column() != 5 {
				t.Fatalf("location = {%q %d %d}, want {%q 3 5}", issue.Path(), issue.Line(), issue.Column(), "language.xsd")
			}
		})
	}
}

func annotationOnly(parent, child string) string {
	return "xs:\"" + parent + "\" can contain only xs:annotation, not xs:\"" + child + "\""
}

func annotationPayloadOnly(child string) string {
	return "xs:\"annotation\" can contain only xs:appinfo and xs:documentation, not xs:\"" + child + "\""
}

func facetBody(children string) string {
	return `  <xs:simpleType name="T">
    <xs:restriction base="xs:string">
      <xs:length value="1">` + children + `</xs:length>
    </xs:restriction>
  </xs:simpleType>`
}

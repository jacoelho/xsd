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
			wantMsg: "xs:include cannot contain text",
		},
		{
			name: "import text before invalid child",
			body: `  <xs:import>
    text
    <xs:attribute/>
  </xs:import>`,
			wantLine: 2, wantCol: 3, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:import cannot contain text",
		},
		{
			name: "include invalid child before duplicate annotation",
			body: `  <xs:include>
    <xs:annotation/>
    <xs:attribute/>
    <xs:annotation/>
  </xs:include>`,
			wantLine: 4, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "include can contain only annotation",
		},
		{
			name: "import invalid child before duplicate annotation",
			body: `  <xs:import>
    <xs:annotation/>
    <xs:attribute/>
    <xs:annotation/>
  </xs:import>`,
			wantLine: 4, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "import can contain only annotation",
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
			wantMsg: "complexType annotation must be first",
		},
		{
			name: "include duplicate annotation before invalid child",
			body: `  <xs:include>
    <xs:annotation/>
    <xs:annotation/>
    <xs:attribute/>
  </xs:include>`,
			wantLine: 4, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "include can contain at most one annotation",
		},
		{
			name: "import duplicate annotation before invalid child",
			body: `  <xs:import>
    <xs:annotation/>
    <xs:annotation/>
    <xs:attribute/>
  </xs:import>`,
			wantLine: 4, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "import can contain at most one annotation",
		},
		{
			name: "nested annotation source position",
			body: `  <xs:annotation>
    <xs:annotation/>
  </xs:annotation>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg: "xs:annotation cannot contain xs:annotation",
		},
		{
			name:     "annotation attribute start admission",
			body:     `  <xs:annotation foo=""/>`,
			wantLine: 2, wantCol: 3, wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg: "annotation cannot have attribute foo",
		},
		{
			name: "documentation language before include child grammar",
			body: `  <xs:include>
    <xs:documentation xml:lang=" "/>
  </xs:include>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg: "invalid xml:lang on xs:documentation",
		},
		{
			name: "documentation language start admission",
			body: `  <xs:annotation>
    <xs:documentation xml:lang=" "/>
  </xs:annotation>`,
			wantLine: 3, wantCol: 5, wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg: "invalid xml:lang on xs:documentation",
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
			name: "invalid language before notation child grammar", lang: " ",
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:  "invalid xml:lang on xs:documentation",
		},
		{
			name: "valid language reaches notation child grammar", lang: "en-US",
			wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg:  "notation can contain only annotation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="N" public="">
    <xs:documentation xml:lang="` + tt.lang + `"/>
  </xs:notation>
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

func TestNonopaqueDocumentationEndAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		children string
		wantMsg  string
	}{
		{
			name: "text and repeated annotations exempt from component grammar",
			children: `      <xs:annotation/>
      <xs:annotation/>`,
		},
		{
			name: "retained child still validates its own annotations",
			children: `      <xs:annotation/>
      <xs:annotation>
        <xs:annotation/>
      </xs:annotation>`,
			wantMsg: "xs:annotation cannot contain xs:annotation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			limits, err := NormalizeOptions(Options{})
			if err != nil {
				t.Fatal(err)
			}
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="N" public="">
    <xs:documentation xml:lang="en-US">
      text
` + tt.children + `
    </xs:documentation>
  </xs:notation>
</xs:schema>`
			_, err = parseSchemaDocument("documentation.xsd", "documentation.xsd", strings.NewReader(schema), limits, new(xmlstream.Reader))
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("parseSchemaDocument() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("parseSchemaDocument() error = %T %[1]v, want structured diagnostic", err)
			}
			if issue.Category() != xsderrors.CategorySchemaCompile || issue.Code() != xsderrors.CodeSchemaContentModel || issue.Message() != tt.wantMsg {
				t.Fatalf("diagnostic = {%s %s %q}, want {%s %s %q}", issue.Category(), issue.Code(), issue.Message(), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel, tt.wantMsg)
			}
			if issue.Path() != "documentation.xsd" || issue.Line() != 7 || issue.Column() != 9 {
				t.Fatalf("location = {%q %d %d}, want {%q 7 9}", issue.Path(), issue.Line(), issue.Column(), "documentation.xsd")
			}
		})
	}
}

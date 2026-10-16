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
	return &schemaSyntaxNode{
		Name:  name,
		Attrs: schemaAttributesStream(name, attrs),
	}
}

func testXSDRawNode(local string, attrs []xmlstream.Attr) *schemaSyntaxNode {
	return testRawNode(xml.Name{Space: vocab.XSDNamespaceURI, Local: local}, attrs)
}

func testRawAttr(namespace, local, value string) xmlstream.Attr {
	return xmlstream.Attr{Name: xml.Name{Space: namespace, Local: local}, Value: value}
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
	return parseSchemaDocument("start.xsd", "start.xsd", strings.NewReader(input), limits, new(xmlstream.Reader))
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
		{name: "simple type named", body: `  <xs:simpleType name="T"/>`},
		{name: "complex type named", body: `  <xs:complexType name="T"/>`},
		{name: "attributeGroup named", body: `  <xs:attributeGroup name="Attrs"/>`},
		{name: "unknown child", body: `  <xs:selector xpath="."/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "identity constraint must be inside element", wantLine: 2, wantCol: 3},
		{name: "invalid child", body: `  <xs:foo/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "invalid top-level schema child foo", wantLine: 2, wantCol: 3},
		{name: "simple type missing name", body: `  <xs:simpleType/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaReference, wantMsg: "top-level simpleType missing name", wantLine: 2, wantCol: 3},
		{name: "attributeGroup ref", body: `  <xs:attributeGroup name="Attrs" ref="Other"/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level attributeGroup cannot have ref", wantLine: 2, wantCol: 3},
		{name: "group minOccurs", body: `  <xs:group name="Group" minOccurs=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level group cannot have minOccurs", wantLine: 2, wantCol: 3},
		{name: "attribute form", body: `  <xs:attribute name="Attr" form=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level attribute cannot have form", wantLine: 2, wantCol: 3},
		{name: "element ref", body: `  <xs:element name="Root" ref="Other"/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level element cannot have ref", wantLine: 2, wantCol: 3},
		{name: "group ref wins reverse order and missing name", body: `  <xs:group maxOccurs="" minOccurs="" ref=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level group cannot have ref", wantLine: 2, wantCol: 3},
		{name: "group min wins reverse order and missing name", body: `  <xs:group maxOccurs="" minOccurs=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level group cannot have minOccurs", wantLine: 2, wantCol: 3},
		{name: "attribute ref wins reverse order and missing name", body: `  <xs:attribute use="" form="" ref=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level attribute cannot have ref", wantLine: 2, wantCol: 3},
		{name: "attribute form wins reverse order and missing name", body: `  <xs:attribute use="" form=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level attribute cannot have form", wantLine: 2, wantCol: 3},
		{name: "element ref wins reverse order and missing name", body: `  <xs:element maxOccurs="" minOccurs="" form="" ref=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level element cannot have ref", wantLine: 2, wantCol: 3},
		{name: "element form wins reverse order and missing name", body: `  <xs:element maxOccurs="" minOccurs="" form=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "top-level element cannot have form", wantLine: 2, wantCol: 3},
		{name: "empty name wins before forbidden attr", body: `  <xs:element ref="" name=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "schema component name must be NCName", wantLine: 2, wantCol: 3},
		{name: "whitespace name wins before forbidden attr", body: `  <xs:group minOccurs="" name=" "/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "schema component name must be NCName", wantLine: 2, wantCol: 3},
		{name: "foreign attributes do not create forbidden presence", body: `  <xs:element name="Root" f:ref="" f:form="" f:minOccurs="" f:maxOccurs=""/>`},
		{name: "start general attr wins before end text", body: `  <xs:element name="Root" foo="">text</xs:element>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "element cannot have attribute foo", wantLine: 2, wantCol: 3},
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
		{name: "non schema node", body: `  <f:other/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "foreign element other is not allowed in schema grammar", wantLine: 2, wantCol: 3},
		{name: "schema namespace attribute", rootAttrs: ` xs:foo=""`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "schema namespace attribute foo is not allowed", wantLine: 1, wantCol: 1},
		{name: "redefine", body: `  <xs:redefine/>`, wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedRedefine, wantMsg: "xs:redefine is not supported", wantLine: 2, wantCol: 3},
		{name: "top-level notation", body: `  <xs:notation name="N" public=""/>`},
		{name: "nested notation", body: `  <xs:element name="Root">
    <xs:notation name="N" public=""/>
  </xs:element>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "xs:notation must be a top-level schema child", wantLine: 3, wantCol: 5},
		{name: "xsd 1.1 element", body: `  <xs:assert/>`, wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedXSD11, wantMsg: "XSD 1.1 feature assert is not supported", wantLine: 2, wantCol: 3},
		{name: "xsd 1.1 wildcard attribute", body: `  <xs:anyAttribute notNamespace=""/>`, wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedXSD11, wantMsg: "XSD 1.1 wildcard attribute notNamespace is not supported", wantLine: 2, wantCol: 3},
		{name: "schema namespace wildcard attribute reports invalid attribute first", body: `  <xs:any xs:notQName=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "schema namespace attribute notQName is not allowed", wantLine: 2, wantCol: 3},
		{name: "foreign rejection wins before namespace attrs and names", body: `  <f:other xs:foo="" id=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaContentModel, wantMsg: "foreign element other is not allowed in schema grammar", wantLine: 2, wantCol: 3},
		{name: "namespace attr wins before names and unsupported", body: `  <xs:assert xs:foo="" id=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "schema namespace attribute foo is not allowed", wantLine: 2, wantCol: 3},
		{name: "invalid name wins before unsupported", body: `  <xs:assert name=""/>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "schema component name must be NCName", wantLine: 2, wantCol: 3},
		{name: "unsupported wins before general attrs and text", body: `  <xs:assert foo="">text</xs:assert>`, wantCat: xsderrors.CategoryUnsupported, wantCode: xsderrors.CodeUnsupportedXSD11, wantMsg: "XSD 1.1 feature assert is not supported", wantLine: 2, wantCol: 3},
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

func TestValidateSchemaIDs(t *testing.T) {
	t.Parallel()
	doc := &schemaDocument{name: "ids.xsd"}
	first := &schemaNode{doc: doc, local: vocab.XSDElemAnnotation, line: 2, column: 3}
	second := &schemaNode{doc: doc, local: vocab.XSDElemAnnotation, line: 3, column: 5}
	third := &schemaNode{doc: doc, local: vocab.XSDElemAnnotation, line: 4, column: 7}
	for _, test := range []struct {
		name          string
		ids           []typedSchemaIDRecord
		wantDuplicate bool
	}{
		{name: "no IDs"},
		{name: "distinct", ids: []typedSchemaIDRecord{{node: first, value: "a"}, {node: second, value: "b"}}},
		{name: "first duplicate", ids: []typedSchemaIDRecord{{node: first, value: "a"}, {node: second, value: "b"}, {node: third, value: "a"}, {node: second, value: "b"}}, wantDuplicate: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			state := typedSchemaParseState{ids: test.ids}
			err := state.validateIDs()
			if !test.wantDuplicate {
				if err != nil {
					t.Fatalf("validateIDs() error = %v", err)
				}
				return
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("error = %T %[1]v, want structured diagnostic", err)
			}
			if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaInvalidAttribute || diagnostic.Message() != "duplicate schema id a" || diagnostic.Path() != "ids.xsd" || diagnostic.Line() != 4 || diagnostic.Column() != 7 || diagnostic.Cause() != nil {
				t.Fatalf("duplicate diagnostic = %#v", diagnostic)
			}
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

			err := ValidateSchemaTargetNamespace(LexicalAttribute{Value: tt.target, Present: tt.hasTarget})
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

func TestValidateSchemaNodeNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		node    SchemaNodeNames
		wantMsg string
	}{
		{name: "non schema node", node: SchemaNodeNames{Local: elementChild, HasName: true, Name: "0"}},
		{name: "valid schema names", node: SchemaNodeNames{Local: elementChild, XSD: true, HasID: true, ID: "id1", HasName: true, Name: "root"}},
		{
			name:    "invalid id",
			node:    SchemaNodeNames{Local: elementChild, XSD: true, HasID: true, ID: ":bad"},
			wantMsg: "schema id must be NCName",
		},
		{
			name:    "invalid name",
			node:    SchemaNodeNames{Local: elementChild, XSD: true, HasName: true, Name: "0"},
			wantMsg: "schema component name must be NCName",
		},
		{
			name: "top-level attribute group name",
			node: SchemaNodeNames{
				Local:       attributeGroup,
				ParentLocal: vocab.XSDElemSchema,
				Name:        "attrs",
				XSD:         true,
				ParentXSD:   true,
				HasName:     true,
			},
		},
		{
			name: "nested attribute group name",
			node: SchemaNodeNames{
				Local:       attributeGroup,
				ParentLocal: complexTypeChild,
				Name:        "attrs",
				XSD:         true,
				ParentXSD:   true,
				HasName:     true,
			},
			wantMsg: "attributeGroup use cannot have name",
		},
		{
			name: "invalid nested attribute group name wins",
			node: SchemaNodeNames{
				Local:       attributeGroup,
				ParentLocal: complexTypeChild,
				Name:        "0",
				XSD:         true,
				ParentXSD:   true,
				HasName:     true,
			},
			wantMsg: "schema component name must be NCName",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSchemaNodeNames(tt.node)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateSchemaNodeNames() error = %v", err)
				}
				return
			}
			expectInvalidAttributeMessage(t, err, tt.wantMsg)
		})
	}
}

func TestCheckSchemaAnnotationNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		local    string
		children []annotationChildFixture
		wantLine int
		wantMsg  string
	}{
		{
			name: "annotation rejects nested annotation", local: annotationChild,
			children: []annotationChildFixture{{local: annotationChild}},
			wantLine: 100, wantMsg: "xs:annotation cannot contain xs:annotation",
		},
		{
			name: "schema allows multiple top-level annotations", local: vocab.XSDElemSchema,
			children: []annotationChildFixture{{local: annotationChild}, {local: annotationChild}},
		},
		{
			name: "component rejects duplicate annotation", local: elementChild,
			children: []annotationChildFixture{{local: annotationChild}, {local: annotationChild}},
			wantLine: 101, wantMsg: "schema component cannot contain multiple annotations",
		},
		{
			name: "component rejects late annotation", local: complexTypeChild,
			children: []annotationChildFixture{{local: attributeChild}, {local: annotationChild}},
			wantLine: 101, wantMsg: "complexType annotation must be first",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := annotationGrammarFixture(t, tt.local, tt.children)
			err := checkSchemaAnnotationNode(parent)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("checkSchemaAnnotationNode() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("checkSchemaAnnotationNode() error = %T %[1]v, want structured diagnostic", err)
			}
			if issue.Category() != xsderrors.CategorySchemaCompile || issue.Code() != xsderrors.CodeSchemaContentModel || issue.Message() != tt.wantMsg {
				t.Fatalf("diagnostic = {%s %s %q}, want {%s %s %q}", issue.Category(), issue.Code(), issue.Message(), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel, tt.wantMsg)
			}
			if issue.Path() != "annotation.xsd" || issue.Line() != tt.wantLine || issue.Column() != 7 {
				t.Fatalf("location = {%q %d %d}, want {%q %d %d}", issue.Path(), issue.Line(), issue.Column(), "annotation.xsd", tt.wantLine, 7)
			}
		})
	}
}

type annotationChildFixture struct {
	local string
}

func annotationGrammarFixture(t *testing.T, local string, children []annotationChildFixture) *schemaNode {
	t.Helper()
	doc := &schemaDocument{name: "annotation.xsd"}
	parent := &schemaNode{doc: doc, local: local, line: 1, column: 3}
	doc.arena = append(doc.arena, parent)
	for i, child := range children {
		id, err := schemaNodeIDFor(len(doc.arena))
		if err != nil {
			t.Fatal(err)
		}
		node := &schemaNode{doc: doc, id: id, local: child.local, line: 100 + i, column: 7}
		doc.arena = append(doc.arena, node)
		parent.children = append(parent.children, node.id)
	}
	return parent
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
			attrs:    []xmlstream.Attr{testRawAttr("", vocab.XSDAttrID, "")},
			validate: checkSchemaAttributesSyntax,
		},
		{
			name: "annotation rejects local attr", local: annotationChild,
			attrs:    []xmlstream.Attr{testRawAttr("", "foo", "")},
			validate: checkSchemaAttributesSyntax, wantMsg: "annotation cannot have attribute foo",
		},
		{
			name: "documentation valid lang", local: vocab.XSDElemDocumentation,
			attrs:    []xmlstream.Attr{testRawAttr(vocab.XMLNamespaceURI, vocab.XMLAttrLang, "en-US")},
			validate: checkAnnotationEnvelopeStart,
		},
		{
			name: "documentation invalid lang", local: vocab.XSDElemDocumentation,
			attrs:    []xmlstream.Attr{testRawAttr(vocab.XMLNamespaceURI, vocab.XMLAttrLang, " ")},
			validate: checkAnnotationEnvelopeStart, wantMsg: "invalid xml:lang on xs:documentation",
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

func TestCheckNotationDeclaration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		hasText   bool
		children  []notationChildFixture
		hasName   bool
		hasPublic bool
		hasSystem bool
		wantIndex int
		wantCode  xsderrors.Code
		wantMsg   string
	}{
		{name: "public", hasName: true, hasPublic: true},
		{name: "system", hasName: true, hasSystem: true},
		{name: "annotation", children: []notationChildFixture{{local: annotationChild}}, hasName: true, hasPublic: true},
		{
			name:      "text",
			hasText:   true,
			hasName:   true,
			hasPublic: true,
			wantIndex: -1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "notation can contain only annotation",
		},
		{
			name:      "xsd child",
			children:  []notationChildFixture{{local: "element"}},
			hasName:   true,
			hasPublic: true,
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "notation can contain only annotation",
		},
		{
			name:      "other nonannotation child",
			children:  []notationChildFixture{{local: simpleTypeChild}},
			hasName:   true,
			hasPublic: true,
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "notation can contain only annotation",
		},
		{
			name:      "missing name",
			hasPublic: true,
			wantIndex: -1,
			wantCode:  xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:   "notation missing name",
		},
		{
			name:      "missing public and system",
			hasName:   true,
			wantIndex: -1,
			wantCode:  xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:   "notation requires public or system",
		},
		{
			name:      "text before invalid child and missing attributes",
			hasText:   true,
			children:  []notationChildFixture{{local: elementChild}},
			wantIndex: -1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "notation can contain only annotation",
		},
		{
			name:      "nonannotation child before missing name",
			children:  []notationChildFixture{{local: elementChild}},
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "notation can contain only annotation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := notationGrammarFixture(t, tt.children)
			parent.hasNonWhitespaceText = tt.hasText
			parent.semantic.kind = &schemaNotationSource{
				Global: schemaGlobalSource{Name: LexicalAttribute{Present: tt.hasName}},
				Public: LexicalAttribute{Present: tt.hasPublic},
				System: LexicalAttribute{Present: tt.hasSystem},
			}
			err := checkNotationDeclaration(parent)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("checkNotationDeclaration() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("checkNotationDeclaration() error = %T %[1]v, want structured diagnostic", err)
			}
			target := parent
			if tt.wantIndex >= 0 {
				target = parent.doc.node(parent.children[tt.wantIndex])
			}
			if issue.Category() != xsderrors.CategorySchemaCompile || issue.Code() != tt.wantCode || issue.Message() != tt.wantMsg {
				t.Fatalf("diagnostic = {%s %s %q}, want {%s %s %q}", issue.Category(), issue.Code(), issue.Message(), xsderrors.CategorySchemaCompile, tt.wantCode, tt.wantMsg)
			}
			if issue.Path() != "notation.xsd" || issue.Line() != target.line || issue.Column() != target.column {
				t.Fatalf("location = {%q %d %d}, want {%q %d %d}", issue.Path(), issue.Line(), issue.Column(), "notation.xsd", target.line, target.column)
			}
		})
	}
}

type notationChildFixture struct {
	local string
}

func notationGrammarFixture(t *testing.T, children []notationChildFixture) *schemaNode {
	t.Helper()
	doc := &schemaDocument{name: "notation.xsd"}
	parent := &schemaNode{doc: doc, local: notationChild, line: 1, column: 3}
	doc.arena = append(doc.arena, parent)
	for i, child := range children {
		id, err := schemaNodeIDFor(len(doc.arena))
		if err != nil {
			t.Fatal(err)
		}
		node := &schemaNode{doc: doc, id: id, local: child.local, line: 100 + i, column: 7}
		doc.arena = append(doc.arena, node)
		parent.children = append(parent.children, node.id)
	}
	return parent
}

package compile

import (
	"encoding/xml"
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func testRawNode(local string, xsd bool, attrs []xml.Attr, children ...*rawNode) *rawNode {
	space := ""
	if xsd {
		space = vocab.XSDNamespaceURI
	}
	return &rawNode{
		Name:     xml.Name{Space: space, Local: local},
		Attr:     attrs,
		Children: children,
	}
}

func testRawAttr(namespace, local, value string) xml.Attr {
	return xml.Attr{Name: xml.Name{Space: namespace, Local: local}, Value: value}
}

func TestValidateTopLevelSchemaChild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		child    TopLevelSchemaChild
		wantCode xsderrors.Code
		wantMsg  string
	}{
		{name: "annotation", child: TopLevelSchemaChild{Local: annotationChild}},
		{name: "include", child: TopLevelSchemaChild{Local: includeChild}},
		{name: "notation", child: TopLevelSchemaChild{Local: notationChild}},
		{name: "simple type named", child: TopLevelSchemaChild{Local: simpleTypeChild, HasName: true}},
		{name: "complex type named", child: TopLevelSchemaChild{Local: complexTypeChild, HasName: true}},
		{name: "attributeGroup named", child: TopLevelSchemaChild{Local: attributeGroup, HasName: true}},
		{
			name:     "unknown child",
			child:    TopLevelSchemaChild{Local: "selector"},
			wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg:  "identity constraint must be inside element",
		},
		{
			name:     "invalid child",
			child:    TopLevelSchemaChild{Local: "foo"},
			wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg:  "invalid top-level schema child foo",
		},
		{
			name:     "simple type missing name",
			child:    TopLevelSchemaChild{Local: simpleTypeChild},
			wantCode: xsderrors.CodeSchemaReference,
			wantMsg:  "top-level simpleType missing name",
		},
		{
			name:     "attributeGroup ref",
			child:    TopLevelSchemaChild{Local: attributeGroup, HasName: true, HasRef: true},
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:  "top-level attributeGroup cannot have ref",
		},
		{
			name:     "group minOccurs",
			child:    TopLevelSchemaChild{Local: groupChild, HasName: true, HasMinOccurs: true},
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:  "top-level group cannot have minOccurs",
		},
		{
			name:     "attribute form",
			child:    TopLevelSchemaChild{Local: attributeChild, HasName: true, HasForm: true},
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:  "top-level attribute cannot have form",
		},
		{
			name:     "element ref",
			child:    TopLevelSchemaChild{Local: elementChild, HasName: true, HasRef: true},
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:  "top-level element cannot have ref",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTopLevelSchemaChild(tt.child)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateTopLevelSchemaChild() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*TopLevelSchemaChildError](err)
			if !ok {
				t.Fatalf("ValidateTopLevelSchemaChild() error = %T %[1]v, want TopLevelSchemaChildError", err)
			}
			if issue.Code != tt.wantCode || issue.Message != tt.wantMsg {
				t.Fatalf("TopLevelSchemaChildError = {Code:%s Message:%q}, want {%s %q}", issue.Code, issue.Message, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func TestCheckUnsupportedSchemaNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		node     *rawNode
		parent   *rawNode
		wantSkip bool
		wantCat  xsderrors.Category
		wantCode xsderrors.Code
		wantMsg  string
	}{
		{
			name: "appinfo under annotation skips subtree",
			node: testRawNode(vocab.XSDElemAppinfo, true, []xml.Attr{
				testRawAttr("urn:foreign", "unknown", ""),
			}),
			parent:   testRawNode(annotationChild, true, nil),
			wantSkip: true,
		},
		{
			name:     "non schema node",
			node:     testRawNode("other", false, nil),
			wantCat:  xsderrors.CategorySchemaCompile,
			wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg:  "foreign element other is not allowed in schema grammar",
		},
		{
			name: "schema namespace attribute",
			node: testRawNode(vocab.XSDElemSchema, true, []xml.Attr{
				testRawAttr(vocab.XSDNamespaceURI, "foo", ""),
			}),
			wantCat:  xsderrors.CategorySchemaCompile,
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:  "schema namespace attribute foo is not allowed",
		},
		{
			name:     "redefine",
			node:     testRawNode(redefineChild, true, nil),
			parent:   testRawNode(vocab.XSDElemSchema, true, nil),
			wantCat:  xsderrors.CategoryUnsupported,
			wantCode: xsderrors.CodeUnsupportedRedefine,
			wantMsg:  "xs:redefine is not supported",
		},
		{
			name:   "top-level notation",
			node:   testRawNode(notationChild, true, nil),
			parent: testRawNode(vocab.XSDElemSchema, true, nil),
		},
		{
			name:     "nested notation",
			node:     testRawNode(notationChild, true, nil),
			parent:   testRawNode(elementChild, true, nil),
			wantCat:  xsderrors.CategorySchemaCompile,
			wantCode: xsderrors.CodeSchemaContentModel,
			wantMsg:  "xs:notation must be a top-level schema child",
		},
		{
			name:     "xsd 1.1 element",
			node:     testRawNode("assert", true, nil),
			wantCat:  xsderrors.CategoryUnsupported,
			wantCode: xsderrors.CodeUnsupportedXSD11,
			wantMsg:  "XSD 1.1 feature assert is not supported",
		},
		{
			name: "xsd 1.1 wildcard attribute",
			node: testRawNode(anyAttribute, true, []xml.Attr{
				testRawAttr("", vocab.XSDAttrNotNamespace, ""),
			}),
			wantCat:  xsderrors.CategoryUnsupported,
			wantCode: xsderrors.CodeUnsupportedXSD11,
			wantMsg:  "XSD 1.1 wildcard attribute notNamespace is not supported",
		},
		{
			name: "schema namespace wildcard attribute reports invalid attribute first",
			node: testRawNode(anyChild, true, []xml.Attr{
				testRawAttr(vocab.XSDNamespaceURI, vocab.XSDAttrNotQName, ""),
			}),
			wantCat:  xsderrors.CategorySchemaCompile,
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:  "schema namespace attribute notQName is not allowed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			skip, err := checkUnsupportedSchemaNode(tt.node, tt.parent)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("checkUnsupportedSchemaNode() error = %v", err)
				}
				if skip != tt.wantSkip {
					t.Fatalf("skip = %v, want %v", skip, tt.wantSkip)
				}
				return
			}
			diag, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("checkUnsupportedSchemaNode() error = %T %[1]v, want xsderrors.Error", err)
			}
			if diag.Category != tt.wantCat || diag.Code != tt.wantCode || diag.Message != tt.wantMsg {
				t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category, diag.Code, diag.Message, tt.wantCat, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func TestValidateSchemaIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ids       []SchemaID
		wantIndex int
		wantMsg   string
	}{
		{name: "empty"},
		{name: "distinct", ids: []SchemaID{{Value: "a"}, {Value: "b"}}},
		{
			name:      "duplicate",
			ids:       []SchemaID{{Value: "a"}, {Value: "b"}, {Value: "a"}},
			wantIndex: 2,
			wantMsg:   "duplicate schema id a",
		},
		{
			name:      "empty value duplicate",
			ids:       []SchemaID{{Value: ""}, {Value: ""}},
			wantIndex: 1,
			wantMsg:   "duplicate schema id ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSchemaIDs(tt.ids)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateSchemaIDs() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*SchemaIDError](err)
			if !ok {
				t.Fatalf("ValidateSchemaIDs() error = %T %[1]v, want SchemaIDError", err)
			}
			if issue.Index != tt.wantIndex || issue.Code != xsderrors.CodeSchemaInvalidAttribute || issue.Message != tt.wantMsg {
				t.Fatalf("SchemaIDError = {Index:%d Code:%s Message:%q}, want {%d %s %q}", issue.Index, issue.Code, issue.Message, tt.wantIndex, xsderrors.CodeSchemaInvalidAttribute, tt.wantMsg)
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

			err := ValidateSchemaTargetNamespace(tt.hasTarget, tt.target)
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
			if diag.Category != xsderrors.CategorySchemaCompile || diag.Code != xsderrors.CodeSchemaInvalidAttribute || diag.Message != tt.wantMsg {
				t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category, diag.Code, diag.Message, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute, tt.wantMsg)
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

func TestValidateRawSchemaAnnotationNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		node      *rawNode
		wantSkip  bool
		wantIndex int
		wantCode  xsderrors.Code
		wantMsg   string
	}{
		{name: "non schema node", node: testRawNode(elementChild, false, nil)},
		{name: "appinfo skips subtree", node: testRawNode(vocab.XSDElemAppinfo, true, nil), wantSkip: true},
		{
			name: "documentation valid lang skips subtree",
			node: testRawNode(vocab.XSDElemDocumentation, true, []xml.Attr{
				testRawAttr(vocab.XMLNamespaceURI, vocab.XMLAttrLang, "en-US"),
			}),
			wantSkip: true,
		},
		{
			name: "documentation invalid lang",
			node: testRawNode(vocab.XSDElemDocumentation, true, []xml.Attr{
				testRawAttr(vocab.XMLNamespaceURI, vocab.XMLAttrLang, " "),
			}),
			wantIndex: -1,
			wantCode:  xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:   "invalid xml:lang on xs:documentation",
		},
		{
			name: "annotation allows id attr",
			node: testRawNode(annotationChild, true, []xml.Attr{
				testRawAttr("", vocab.XSDAttrID, ""),
			}),
		},
		{
			name: "annotation rejects local attr",
			node: testRawNode(annotationChild, true, []xml.Attr{
				testRawAttr("", "foo", ""),
			}),
			wantIndex: -1,
			wantCode:  xsderrors.CodeSchemaInvalidAttribute,
			wantMsg:   "attribute foo cannot appear on xs:annotation",
		},
		{
			name:      "annotation rejects nested annotation",
			node:      testRawNode(annotationChild, true, nil, testRawNode(annotationChild, true, nil)),
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "xs:annotation cannot contain xs:annotation",
		},
		{
			name: "schema allows multiple top-level annotations",
			node: testRawNode(vocab.XSDElemSchema, true, nil,
				testRawNode(annotationChild, true, nil),
				testRawNode(annotationChild, true, nil),
			),
		},
		{
			name: "component rejects duplicate annotation",
			node: testRawNode(elementChild, true, nil,
				testRawNode(annotationChild, true, nil),
				testRawNode(annotationChild, true, nil),
			),
			wantIndex: 1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "schema component cannot contain multiple annotations",
		},
		{
			name: "component rejects late annotation",
			node: testRawNode(complexTypeChild, true, nil,
				testRawNode(attributeChild, true, nil),
				testRawNode(annotationChild, true, nil),
			),
			wantIndex: 1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "complexType annotation must be first",
		},
		{
			name: "non xsd child ignored for placement",
			node: testRawNode(complexTypeChild, true, nil,
				testRawNode("foreign", false, nil),
				testRawNode(annotationChild, true, nil),
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			action, err := validateRawSchemaAnnotationNode(tt.node)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("validateRawSchemaAnnotationNode() error = %v", err)
				}
				if action.SkipChildren != tt.wantSkip {
					t.Fatalf("SkipChildren = %v, want %v", action.SkipChildren, tt.wantSkip)
				}
				return
			}
			issue, ok := errors.AsType[*SchemaAnnotationSyntaxError](err)
			if !ok {
				t.Fatalf("validateRawSchemaAnnotationNode() error = %T %[1]v, want SchemaAnnotationSyntaxError", err)
			}
			if issue.Index != tt.wantIndex || issue.Code != tt.wantCode || issue.Message != tt.wantMsg {
				t.Fatalf("SchemaAnnotationSyntaxError = {Index:%d Code:%s Message:%q}, want {%d %s %q}", issue.Index, issue.Code, issue.Message, tt.wantIndex, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func TestValidateNotationDeclaration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      string
		children  []NotationChild
		hasName   bool
		hasPublic bool
		hasSystem bool
		wantIndex int
		wantCode  xsderrors.Code
		wantMsg   string
	}{
		{name: "public", hasName: true, hasPublic: true},
		{name: "system", hasName: true, hasSystem: true},
		{name: "annotation", children: []NotationChild{{Local: annotationChild, XSD: true}}, hasName: true, hasPublic: true},
		{
			name:      "text",
			text:      "x",
			hasName:   true,
			hasPublic: true,
			wantIndex: -1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "notation can contain only annotation",
		},
		{
			name:      "xsd child",
			children:  []NotationChild{{Local: "element", XSD: true}},
			hasName:   true,
			hasPublic: true,
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "notation can contain only annotation",
		},
		{
			name:      "non xsd child",
			children:  []NotationChild{{Local: "other"}},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateNotationDeclaration(tt.text, tt.children, tt.hasName, tt.hasPublic, tt.hasSystem)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateNotationDeclaration() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*NotationSyntaxError](err)
			if !ok {
				t.Fatalf("ValidateNotationDeclaration() error = %T %[1]v, want NotationSyntaxError", err)
			}
			if issue.Index != tt.wantIndex || issue.Code != tt.wantCode || issue.Message != tt.wantMsg {
				t.Fatalf("NotationSyntaxError = {Index:%d Code:%s Message:%q}, want {%d %s %q}", issue.Index, issue.Code, issue.Message, tt.wantIndex, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

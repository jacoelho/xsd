package schema

import (
	"errors"
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

func canonicalAttributeFixture(local string, semantic schemaSemanticKind, attrs []string) *schemaNode {
	var mask uint64
	for _, attr := range attrs {
		mask |= typedAttributeMaskFor(attr)
	}
	return &schemaNode{
		doc: &schemaDocument{name: "attributes.xsd"}, local: local,
		semantic: schemaSemanticSource{kind: semantic}, attributeMask: mask, line: 13, column: 5,
	}
}

func assertCanonicalAttributeDiagnostic(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("attribute admission error = %v", err)
		}
		return
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want structured diagnostic", err)
	}
	if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaInvalidAttribute || diagnostic.Message() != want {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diagnostic.Category(), diagnostic.Code(), diagnostic.Message(), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute, want)
	}
	if diagnostic.Path() != "attributes.xsd" || diagnostic.Line() != 13 || diagnostic.Column() != 5 {
		t.Fatalf("location = (%q, %d, %d), want (attributes.xsd, 13, 5)", diagnostic.Path(), diagnostic.Line(), diagnostic.Column())
	}
}

func TestCheckReferenceAttributes(t *testing.T) {
	t.Parallel()
	reference := schemaQNameAttribute{Lexical: LexicalAttribute{Value: "Target", Present: true}}
	for _, fixture := range []struct {
		name        string
		check       func(*schemaNode) error
		local       string
		source      schemaSemanticKind
		attrs       []string
		wantMessage string
	}{
		{
			name: "element ref accepts occurrence attrs", check: checkElementRefAttributes, local: elementChild,
			source: &schemaElementSource{Ref: reference, MinOccurs: LexicalAttribute{Present: true}, MaxOccurs: LexicalAttribute{Present: true}},
			attrs:  []string{"id", "ref", "minOccurs", "maxOccurs"},
		},
		{
			name: "element ref rejects type", check: checkElementRefAttributes, local: elementChild,
			source: &schemaElementSource{Ref: reference, Type: schemaQNameAttribute{Lexical: LexicalAttribute{Value: "xs:string", Present: true}}},
			attrs:  []string{"ref", "type"}, wantMessage: "element ref cannot have attribute type",
		},
		{
			name: "attribute ref accepts value constraint attrs", check: checkAttributeRefAttributes, local: attributeChild,
			source: &schemaAttributeSource{Ref: reference, Use: LexicalAttribute{Present: true}, Default: LexicalAttribute{Present: true}, Fixed: LexicalAttribute{Present: true}},
			attrs:  []string{"id", "ref", "use", "default", "fixed"},
		},
		{
			name: "attribute ref rejects form", check: checkAttributeRefAttributes, local: attributeChild,
			source: &schemaAttributeSource{Ref: reference, Form: LexicalAttribute{Present: true}},
			attrs:  []string{"ref", "form"}, wantMessage: "attribute ref cannot have attribute form",
		},
		{
			name: "group occurrence accepts occurrence attrs", check: checkGroupOccurrenceAttributes, local: groupChild,
			source: &schemaGroupSource{Ref: reference, MinOccurs: LexicalAttribute{Present: true}, MaxOccurs: LexicalAttribute{Present: true}},
			attrs:  []string{"id", "ref", "minOccurs", "maxOccurs"},
		},
		{
			name: "group occurrence rejects name", check: checkGroupOccurrenceAttributes, local: groupChild,
			source: &schemaGroupSource{Global: schemaGlobalSource{Name: LexicalAttribute{Present: true}}, Ref: reference},
			attrs:  []string{"ref", "name"}, wantMessage: "group cannot have attribute name",
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			node := canonicalAttributeFixture(fixture.local, fixture.source, fixture.attrs)
			assertCanonicalAttributeDiagnostic(t, fixture.check(node), fixture.wantMessage)
		})
	}
}

func TestReferenceAttributeDiagnosticPriority(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		start schemaStartFixture
		path  []string
		check func(*schemaNode) error
	}{
		{
			start: schemaStartFixture{name: "element type wins reverse XML order", body: `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="Target" form="" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "element ref cannot have attribute type", wantLine: 4, wantCol: 7},
			path: []string{complexTypeChild, sequenceChild, elementChild}, check: checkElementRefAttributes,
		},
		{
			start: schemaStartFixture{name: "element type wins matching XML order", body: `  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="Target" type="xs:string" form=""/>
    </xs:sequence>
  </xs:complexType>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "element ref cannot have attribute type", wantLine: 4, wantCol: 7},
			path: []string{complexTypeChild, sequenceChild, elementChild}, check: checkElementRefAttributes,
		},
		{
			start: schemaStartFixture{name: "attribute name wins reverse XML order", body: `  <xs:complexType name="T">
    <xs:attribute ref="Target" form="" name="Local"/>
  </xs:complexType>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "attribute ref cannot have attribute name", wantLine: 3, wantCol: 5},
			path: []string{complexTypeChild, attributeChild}, check: checkAttributeRefAttributes,
		},
		{
			start: schemaStartFixture{name: "attribute name wins matching XML order", body: `  <xs:complexType name="T">
    <xs:attribute ref="Target" name="Local" form=""/>
  </xs:complexType>`, wantCat: xsderrors.CategorySchemaCompile, wantCode: xsderrors.CodeSchemaInvalidAttribute, wantMsg: "attribute ref cannot have attribute name", wantLine: 3, wantCol: 5},
			path: []string{complexTypeChild, attributeChild}, check: checkAttributeRefAttributes,
		},
	} {
		t.Run(fixture.start.name, func(t *testing.T) {
			t.Parallel()
			doc, err := parseSchemaStartFixture(t, fixture.start)
			if err != nil {
				t.Fatal(err)
			}
			node := doc.root
			for _, local := range fixture.path {
				node = node.firstXS(local)
			}
			if node == nil {
				t.Fatal("missing reference node")
			}
			assertSchemaStartFixture(t, fixture.check(node), fixture.start)
		})
	}
}

func TestReferenceAdmissionFiltersForeignAttributes(t *testing.T) {
	t.Parallel()
	doc, err := parseSchemaStartFixture(t, schemaStartFixture{body: `  <xs:element name="Root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="Target" f:type="xs:string" f:form=""/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>`})
	if err != nil {
		t.Fatal(err)
	}
	node := doc.root.firstXS(elementChild).firstXS(complexTypeChild).firstXS(sequenceChild).firstXS(elementChild)
	if node == nil {
		t.Fatal("missing reference element")
	}
	if err := checkElementRefAttributes(node); err != nil {
		t.Fatalf("foreign attributes changed reference admission: %v", err)
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
			wantMessage: "schema cannot have attribute type",
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
			wantMessage: "attribute cannot have attribute nillable",
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
			wantMessage: "pattern cannot have attribute fixed",
		},
		{
			name:        "enumeration rejects fixed",
			element:     "enumeration",
			attrs:       []string{"value", "fixed"},
			wantMessage: "enumeration cannot have attribute fixed",
		},
		{
			name:        "facet rejects unknown attribute",
			element:     "maxLength",
			attrs:       []string{"value", "ref"},
			wantMessage: "maxLength cannot have attribute ref",
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

			err := checkSchemaAttributesSyntax(testRawNodeWithLocalAttrs(tt.element, tt.attrs))
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

func TestCheckLocalElementAttributes(t *testing.T) {
	t.Parallel()
	present := LexicalAttribute{Present: true}
	name := schemaGlobalSource{Name: present}
	for _, fixture := range []struct {
		name        string
		source      *schemaElementSource
		attrs       []string
		wantMessage string
	}{
		{name: "accepts ordinary local attrs", source: &schemaElementSource{Global: name, Type: schemaQNameAttribute{Lexical: present}, Form: present}, attrs: []string{"id", "name", "type", "form"}},
		{name: "rejects abstract", source: &schemaElementSource{Global: name, Abstract: present}, attrs: []string{"name", "abstract"}, wantMessage: "local element cannot have abstract"},
		{name: "rejects final", source: &schemaElementSource{Global: name, Final: present}, attrs: []string{"name", "final"}, wantMessage: "local element cannot have final"},
		{name: "rejects substitution group", source: &schemaElementSource{Global: name, SubstitutionGroup: schemaQNameAttribute{Lexical: present}}, attrs: []string{"name", "substitutionGroup"}, wantMessage: "local element cannot have substitutionGroup"},
		{name: "uses fixed forbidden priority", source: &schemaElementSource{Global: name, Abstract: present, Final: present, SubstitutionGroup: schemaQNameAttribute{Lexical: present}}, attrs: []string{"name", "substitutionGroup", "final", "abstract"}, wantMessage: "local element cannot have abstract"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			node := canonicalAttributeFixture(elementChild, fixture.source, fixture.attrs)
			assertCanonicalAttributeDiagnostic(t, checkLocalElementAttributes(node), fixture.wantMessage)
		})
	}
}

func TestCheckLocalTypeAttributes(t *testing.T) {
	t.Parallel()
	present := LexicalAttribute{Present: true}
	for _, fixture := range []struct {
		name        string
		check       func(*schemaNode) error
		local       string
		source      schemaSemanticKind
		attrs       []string
		wantMessage string
	}{
		{name: "simple type accepts anonymous id", check: checkLocalSimpleTypeAttributes, local: simpleTypeChild, source: &schemaSimpleTypeSource{}, attrs: []string{"id"}},
		{name: "simple type rejects name", check: checkLocalSimpleTypeAttributes, local: simpleTypeChild, source: &schemaSimpleTypeSource{Global: schemaGlobalSource{Name: present}}, attrs: []string{"id", "name"}, wantMessage: "local simpleType cannot have name"},
		{name: "simple type rejects final", check: checkLocalSimpleTypeAttributes, local: simpleTypeChild, source: &schemaSimpleTypeSource{Final: present}, attrs: []string{"id", "final"}, wantMessage: "local simpleType cannot have final"},
		{name: "complex type accepts anonymous attrs", check: checkLocalComplexTypeAttributes, local: complexTypeChild, source: &schemaComplexTypeSource{Mixed: present}, attrs: []string{"id", "mixed"}},
		{name: "complex type rejects name", check: checkLocalComplexTypeAttributes, local: complexTypeChild, source: &schemaComplexTypeSource{Global: schemaGlobalSource{Name: present}}, attrs: []string{"id", "name"}, wantMessage: "local complexType cannot have name"},
		{name: "complex type rejects abstract", check: checkLocalComplexTypeAttributes, local: complexTypeChild, source: &schemaComplexTypeSource{Abstract: present}, attrs: []string{"id", "abstract"}, wantMessage: "local complexType cannot have abstract"},
		{name: "complex type rejects block", check: checkLocalComplexTypeAttributes, local: complexTypeChild, source: &schemaComplexTypeSource{Block: present}, attrs: []string{"id", "block"}, wantMessage: "local complexType cannot have block"},
		{name: "complex type rejects final", check: checkLocalComplexTypeAttributes, local: complexTypeChild, source: &schemaComplexTypeSource{Final: present}, attrs: []string{"id", "final"}, wantMessage: "local complexType cannot have final"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			node := canonicalAttributeFixture(fixture.local, fixture.source, fixture.attrs)
			assertCanonicalAttributeDiagnostic(t, fixture.check(node), fixture.wantMessage)
		})
	}
}

func TestValidateUseSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		validate    func() error
		wantMessage string
	}{
		{name: "local element name", validate: func() error {
			return ValidateLocalElementSource(NameReferenceSource{Name: LexicalAttribute{Present: true}})
		}},
		{name: "local element ref", validate: func() error {
			return ValidateLocalElementSource(NameReferenceSource{Reference: LexicalAttribute{Present: true}})
		}},
		{
			name:        "local element missing",
			validate:    func() error { return ValidateLocalElementSource(NameReferenceSource{}) },
			wantMessage: "local element missing name or ref",
		},
		{name: "attribute name", validate: func() error {
			return ValidateAttributeUseSource(NameReferenceSource{Name: LexicalAttribute{Present: true}})
		}},
		{name: "attribute ref", validate: func() error {
			return ValidateAttributeUseSource(NameReferenceSource{Reference: LexicalAttribute{Present: true}})
		}},
		{
			name:        "attribute missing",
			validate:    func() error { return ValidateAttributeUseSource(NameReferenceSource{}) },
			wantMessage: "attribute missing name or ref",
		},
		{name: "attribute group ref", validate: func() error { return ValidateAttributeGroupUseSource(LexicalAttribute{Present: true}) }},
		{
			name:        "attribute group missing",
			validate:    func() error { return ValidateAttributeGroupUseSource(LexicalAttribute{}) },
			wantMessage: "attributeGroup use missing ref",
		},
		{name: "group ref", validate: func() error { return ValidateGroupUseSource(LexicalAttribute{Present: true}) }},
		{
			name:        "group missing",
			validate:    func() error { return ValidateGroupUseSource(LexicalAttribute{}) },
			wantMessage: "group use missing ref",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate()
			if tt.wantMessage == "" {
				if err != nil {
					t.Fatalf("validate source error = %v", err)
				}
				return
			}
			expectSchemaReferenceMessage(t, err, tt.wantMessage)
		})
	}
}

func expectSchemaReferenceMessage(t *testing.T, err error, message string) {
	t.Helper()
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want xsderrors.Error", err)
	}
	if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != xsderrors.CodeSchemaReference || diag.Message() != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category(), diag.Code(), diag.Message(), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaReference, message)
	}
}

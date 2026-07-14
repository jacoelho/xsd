package compile

import (
	"encoding/xml"
	"errors"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func testRawNodeWithLocalAttrs(local string, attrs []string) *rawNode {
	rawAttrs := make([]xml.Attr, len(attrs))
	for i, attr := range attrs {
		rawAttrs[i] = testRawAttr("", attr, "")
	}
	return testRawNode(local, true, rawAttrs)
}

func TestCheckReferenceAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		validate    func(*rawNode) error
		local       string
		attrs       []string
		wantMessage string
	}{
		{
			name:     "element ref accepts occurrence attrs",
			validate: checkElementRefAttributes,
			local:    elementChild,
			attrs:    []string{"id", "ref", "minOccurs", "maxOccurs"},
		},
		{
			name:        "element ref rejects type",
			validate:    checkElementRefAttributes,
			local:       elementChild,
			attrs:       []string{"ref", "type"},
			wantMessage: "element ref cannot have attribute type",
		},
		{
			name:     "attribute ref accepts value constraint attrs",
			validate: checkAttributeRefAttributes,
			local:    attributeChild,
			attrs:    []string{"id", "ref", "use", "default", "fixed"},
		},
		{
			name:        "attribute ref rejects form",
			validate:    checkAttributeRefAttributes,
			local:       attributeChild,
			attrs:       []string{"ref", "form"},
			wantMessage: "attribute ref cannot have attribute form",
		},
		{
			name:     "group occurrence accepts occurrence attrs",
			validate: checkGroupOccurrenceAttributes,
			local:    groupChild,
			attrs:    []string{"id", "ref", "minOccurs", "maxOccurs"},
		},
		{
			name:        "group occurrence rejects name",
			validate:    checkGroupOccurrenceAttributes,
			local:       groupChild,
			attrs:       []string{"ref", "name"},
			wantMessage: "group cannot have attribute name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate(testRawNodeWithLocalAttrs(tt.local, tt.attrs))
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("validate attrs error = %v", err)
			}
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

			err := checkRawSchemaAttributes(testRawNodeWithLocalAttrs(tt.element, tt.attrs))
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("checkRawSchemaAttributes() error = %v", err)
			}
		})
	}
}

func TestCheckLocalElementAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		attrs       []string
		wantMessage string
	}{
		{name: "accepts ordinary local attrs", attrs: []string{"id", "name", "type", "form"}},
		{name: "rejects abstract", attrs: []string{"name", "abstract"}, wantMessage: "local element cannot have abstract"},
		{name: "rejects final", attrs: []string{"name", "final"}, wantMessage: "local element cannot have final"},
		{name: "rejects substitution group", attrs: []string{"name", "substitutionGroup"}, wantMessage: "local element cannot have substitutionGroup"},
		{
			name:        "uses fixed forbidden priority",
			attrs:       []string{"name", "substitutionGroup", "final", "abstract"},
			wantMessage: "local element cannot have abstract",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := checkLocalElementAttributes(testRawNodeWithLocalAttrs(elementChild, tt.attrs))
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("checkLocalElementAttributes() error = %v", err)
			}
		})
	}
}

func TestCheckLocalTypeAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		validate    func(*rawNode) error
		local       string
		attrs       []string
		wantMessage string
	}{
		{
			name:     "simple type accepts anonymous id",
			validate: checkLocalSimpleTypeAttributes,
			local:    simpleTypeChild,
			attrs:    []string{"id"},
		},
		{
			name:        "simple type rejects name",
			validate:    checkLocalSimpleTypeAttributes,
			local:       simpleTypeChild,
			attrs:       []string{"id", "name"},
			wantMessage: "local simpleType cannot have name",
		},
		{
			name:        "simple type rejects final",
			validate:    checkLocalSimpleTypeAttributes,
			local:       simpleTypeChild,
			attrs:       []string{"id", "final"},
			wantMessage: "local simpleType cannot have final",
		},
		{
			name:     "complex type accepts anonymous attrs",
			validate: checkLocalComplexTypeAttributes,
			local:    complexTypeChild,
			attrs:    []string{"id", "mixed"},
		},
		{
			name:        "complex type rejects name",
			validate:    checkLocalComplexTypeAttributes,
			local:       complexTypeChild,
			attrs:       []string{"id", "name"},
			wantMessage: "local complexType cannot have name",
		},
		{
			name:        "complex type rejects abstract",
			validate:    checkLocalComplexTypeAttributes,
			local:       complexTypeChild,
			attrs:       []string{"id", "abstract"},
			wantMessage: "local complexType cannot have abstract",
		},
		{
			name:        "complex type rejects block",
			validate:    checkLocalComplexTypeAttributes,
			local:       complexTypeChild,
			attrs:       []string{"id", "block"},
			wantMessage: "local complexType cannot have block",
		},
		{
			name:        "complex type rejects final",
			validate:    checkLocalComplexTypeAttributes,
			local:       complexTypeChild,
			attrs:       []string{"id", "final"},
			wantMessage: "local complexType cannot have final",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate(testRawNodeWithLocalAttrs(tt.local, tt.attrs))
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("validate local type attrs error = %v", err)
			}
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
		{name: "local element name", validate: func() error { return ValidateLocalElementSource(true, false) }},
		{name: "local element ref", validate: func() error { return ValidateLocalElementSource(false, true) }},
		{
			name:        "local element missing",
			validate:    func() error { return ValidateLocalElementSource(false, false) },
			wantMessage: "local element missing name or ref",
		},
		{name: "attribute name", validate: func() error { return ValidateAttributeUseSource(true, false) }},
		{name: "attribute ref", validate: func() error { return ValidateAttributeUseSource(false, true) }},
		{
			name:        "attribute missing",
			validate:    func() error { return ValidateAttributeUseSource(false, false) },
			wantMessage: "attribute missing name or ref",
		},
		{name: "attribute group ref", validate: func() error { return ValidateAttributeGroupUseSource(true) }},
		{
			name:        "attribute group missing",
			validate:    func() error { return ValidateAttributeGroupUseSource(false) },
			wantMessage: "attributeGroup use missing ref",
		},
		{name: "group ref", validate: func() error { return ValidateGroupUseSource(true) }},
		{
			name:        "group missing",
			validate:    func() error { return ValidateGroupUseSource(false) },
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
	if diag.Category != xsderrors.CategorySchemaCompile || diag.Code != xsderrors.CodeSchemaReference || diag.Message != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category, diag.Code, diag.Message, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaReference, message)
	}
}

package schema

import (
	"encoding/xml"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateAttributeTypeSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		hasAttr     bool
		hasChild    bool
		wantMessage string
	}{
		{name: "neither"},
		{name: "type attr", hasAttr: true},
		{name: "simple child", hasChild: true},
		{name: "both", hasAttr: true, hasChild: true, wantMessage: "attribute cannot have both type and simpleType"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeTypeSource(AttributeTypeSource{
				Type:               LexicalAttribute{Present: tt.hasAttr},
				HasSimpleTypeChild: tt.hasChild,
			})
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("ValidateAttributeTypeSource() error = %v", err)
			}
		})
	}
}

func TestValidateSimpleRestrictionTypeSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		hasAttr     bool
		hasChild    bool
		wantCode    xsderrors.Code
		wantMessage string
	}{
		{name: "neither", wantCode: xsderrors.CodeSchemaReference, wantMessage: "simple restriction missing base"},
		{name: "base attr", hasAttr: true},
		{name: "simple child", hasChild: true},
		{name: "both", hasAttr: true, hasChild: true, wantCode: xsderrors.CodeSchemaContentModel, wantMessage: "restriction cannot have both base and simpleType"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleRestrictionTypeSource(SimpleRestrictionTypeSource{
				Base:               LexicalAttribute{Present: tt.hasAttr},
				HasSimpleTypeChild: tt.hasChild,
			})
			if tt.wantMessage != "" {
				expectXSDMessage(t, err, tt.wantCode, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("ValidateSimpleRestrictionTypeSource() error = %v", err)
			}
		})
	}
}

func TestValidateSimpleListItemTypeSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		hasAttr     bool
		hasChild    bool
		wantCode    xsderrors.Code
		wantMessage string
	}{
		{name: "neither", wantCode: xsderrors.CodeSchemaReference, wantMessage: "list missing item type"},
		{name: "itemType attr", hasAttr: true},
		{name: "simple child", hasChild: true},
		{name: "both", hasAttr: true, hasChild: true, wantCode: xsderrors.CodeSchemaContentModel, wantMessage: "list cannot have both itemType and simpleType"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleListItemTypeSource(SimpleListItemTypeSource{
				ItemType:           LexicalAttribute{Present: tt.hasAttr},
				HasSimpleTypeChild: tt.hasChild,
			})
			if tt.wantMessage != "" {
				expectXSDMessage(t, err, tt.wantCode, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("ValidateSimpleListItemTypeSource() error = %v", err)
			}
		})
	}
}

func TestUnionMemberSourceAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		memberTypes        string
		hasMemberTypes     bool
		hasSimpleTypeChild bool
		wantValue          string
		wantNames          []xml.Name
		wantMessage        string
	}{
		{
			name:        "missing",
			wantMessage: "union missing member types",
		},
		{
			name:               "anonymous child",
			hasSimpleTypeChild: true,
		},
		{
			name:               "empty attribute with anonymous child",
			memberTypes:        " \t\n\r ",
			hasMemberTypes:     true,
			hasSimpleTypeChild: true,
		},
		{
			name:           "tokens",
			memberTypes:    " xs:string\tp:local\nother\r\nlast ",
			hasMemberTypes: true,
			wantValue:      "xs:string p:local other last",
			wantNames: []xml.Name{
				{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"},
				{Space: "urn:member", Local: "local"},
				{Local: "other"},
				{Local: "last"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			limits, err := NormalizeOptions(Options{})
			if err != nil {
				t.Fatal(err)
			}
			input := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:member">` + "\n  " + `<xs:simpleType name="union">` + "\n    <xs:union"
			if tt.hasMemberTypes {
				input += ` memberTypes="` + tt.memberTypes + `"`
			}
			input += ">"
			if tt.hasSimpleTypeChild {
				input += `<xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType>`
			}
			input += "</xs:union>\n  </xs:simpleType>\n</xs:schema>"
			doc, err := parseSchemaDocument("union.xsd", "union.xsd", strings.NewReader(input), limits, new(xmlstream.Reader))
			if err != nil {
				t.Fatalf("parseSchemaDocument() error = %v", err)
			}
			union := doc.root.firstXS("simpleType").firstXS("union")
			members, ok := union.semantic.derivationMemberTypes()
			if !ok {
				t.Fatal("union has no admitted member source")
			}
			if members.Lexical.Present != tt.hasMemberTypes || members.Lexical.Value != tt.wantValue || !slices.Equal(members.Names, tt.wantNames) {
				t.Fatalf("admitted members = %+v, want present %v value %q names %#v", members, tt.hasMemberTypes, tt.wantValue, tt.wantNames)
			}
			err = validateUnionMemberSource(union, len(members.Names), len(schemaSimpleTypeChildren(union)))
			if tt.wantMessage != "" {
				diagnostic, ok := errors.AsType[*xsderrors.Error](err)
				if !ok {
					t.Fatalf("validateUnionMemberSource() error = %T %[1]v, want xsderrors.Error", err)
				}
				if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaReference || diagnostic.Message() != tt.wantMessage {
					t.Fatalf("diagnostic = (%s, %s, %q), want (schema_compile, schema_reference, %q)", diagnostic.Category(), diagnostic.Code(), diagnostic.Message(), tt.wantMessage)
				}
				if diagnostic.Path() != "union.xsd" || diagnostic.Line() != 3 || diagnostic.Column() != 5 {
					t.Fatalf("location = {%q %d %d}, want {union.xsd 3 5}", diagnostic.Path(), diagnostic.Line(), diagnostic.Column())
				}
				return
			}
			if err != nil {
				t.Fatalf("validateUnionMemberSource() error = %v", err)
			}
		})
	}
}

func expectXSDMessage(t *testing.T, err error, code xsderrors.Code, message string) {
	t.Helper()
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want xsderrors.Error", err)
	}
	if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != code || diag.Message() != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)",
			diag.Category(), diag.Code(), diag.Message(), xsderrors.CategorySchemaCompile, code, message)
	}
}

func TestCompiledListItemQueryRejectsInvalidRoots(t *testing.T) {
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	err = c.loadOwned([]source.Source{source.Bytes("query.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="inner"><xs:union memberTypes="xs:string xs:int"/></xs:simpleType>
  <xs:simpleType name="outer"><xs:union memberTypes="inner inner xs:boolean"/></xs:simpleType>
</xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.index(); err != nil {
		t.Fatal(err)
	}
	if err := c.compileGlobals(); err != nil {
		t.Fatal(err)
	}
	q, ok := c.rt.Names.LookupQName("", "outer")
	if !ok {
		t.Fatal("compiled outer type missing")
	}
	id, ok := c.rt.GlobalTypes[q].Simple()
	if !ok {
		t.Fatal("outer is not a simple type")
	}
	if c.simpleListItemReachesList(id) {
		t.Fatal("compiled scalar union unexpectedly contains a list")
	}
	for _, invalid := range []SimpleTypeID{NoSimpleType, SimpleTypeID(1_000_000)} {
		if c.simpleListItemReachesList(invalid) {
			t.Fatalf("list-item query(%d) = true for invalid root", invalid)
		}
	}
}

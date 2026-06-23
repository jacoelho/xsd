package compile

import (
	"errors"
	"slices"
	"testing"

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

			err := ValidateAttributeTypeSource(tt.hasAttr, tt.hasChild)
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

			err := ValidateSimpleRestrictionTypeSource(tt.hasAttr, tt.hasChild)
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

			err := ValidateSimpleListItemTypeSource(tt.hasAttr, tt.hasChild)
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

func TestParseUnionMemberTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		memberTypes        string
		hasMemberTypes     bool
		hasSimpleTypeChild bool
		want               []string
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
			want:           []string{"xs:string", "p:local", "other", "last"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseUnionMemberTypes(tt.memberTypes, tt.hasMemberTypes, tt.hasSimpleTypeChild)
			if tt.wantMessage != "" {
				diag, ok := errors.AsType[*xsderrors.Error](err)
				if !ok {
					t.Fatalf("ParseUnionMemberTypes() error = %T %[1]v, want xsderrors.Error", err)
				}
				if diag.Code != xsderrors.CodeSchemaReference || diag.Message != tt.wantMessage {
					t.Fatalf("diagnostic = (%s, %q), want (%s, %q)", diag.Code, diag.Message, xsderrors.CodeSchemaReference, tt.wantMessage)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseUnionMemberTypes() error = %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("ParseUnionMemberTypes() = %#v, want %#v", got, tt.want)
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
	if diag.Category != xsderrors.CategorySchemaCompile || diag.Code != code || diag.Message != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)",
			diag.Category, diag.Code, diag.Message, xsderrors.CategorySchemaCompile, code, message)
	}
}

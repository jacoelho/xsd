package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseBooleanAttr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		attr        booleanAttr
		want        bool
		wantMessage string
	}{
		{
			name: "missing uses default",
			attr: booleanAttr{Default: true},
			want: true,
		},
		{
			name: "true literal",
			attr: booleanAttr{Name: "mixed", Value: "\ttrue\n", HasValue: true},
			want: true,
		},
		{
			name: "one literal",
			attr: booleanAttr{Name: "mixed", Value: "1", HasValue: true},
			want: true,
		},
		{
			name: "false literal",
			attr: booleanAttr{Name: "mixed", Value: "\rfalse ", HasValue: true},
		},
		{
			name: "zero literal",
			attr: booleanAttr{Name: "mixed", Value: "0", HasValue: true},
		},
		{
			name:        "invalid",
			attr:        booleanAttr{Name: "mixed", Value: "yes", HasValue: true},
			wantMessage: "invalid boolean attribute \"mixed\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseBooleanAttr(tt.attr)
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("ParseBooleanAttr() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseBooleanAttr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFormDefaultAttr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		attr        formAttr
		want        bool
		wantMessage string
	}{
		{
			name: "missing uses zero default",
			attr: formAttr{Name: "elementFormDefault"},
		},
		{
			name: "missing uses explicit default",
			attr: formAttr{Name: "elementFormDefault", DefaultQualified: true},
			want: true,
		},
		{
			name: "qualified",
			attr: formAttr{Name: "elementFormDefault", Value: "qualified", HasValue: true},
			want: true,
		},
		{
			name: "unqualified",
			attr: formAttr{Name: "elementFormDefault", Value: "unqualified", HasValue: true},
		},
		{
			name:        "invalid",
			attr:        formAttr{Name: "elementFormDefault", Value: "local", HasValue: true},
			wantMessage: "invalid \"elementFormDefault\" value \"local\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseFormDefaultAttr(tt.attr)
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("ParseFormDefaultAttr() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseFormDefaultAttr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSchemaDefaults(t *testing.T) {
	t.Parallel()

	t.Run("zero defaults", func(t *testing.T) {
		t.Parallel()
		got, err := parseSchemaDefaults(schemaDocumentSource{})
		if err != nil || got != (schemaDefaults{}) {
			t.Fatalf("ParseSchemaDefaults() = %+v, %v, want zero defaults", got, err)
		}
	})

	t.Run("valid defaults", func(t *testing.T) {
		t.Parallel()

		got, err := parseSchemaDefaults(schemaDocumentSource{
			TargetNamespace:      lexicalAttribute{Value: "urn:test", Present: true},
			BlockDefault:         lexicalAttribute{Value: "#all"},
			FinalDefault:         lexicalAttribute{Value: "extension restriction"},
			ElementFormDefault:   lexicalAttribute{Value: "qualified", Present: true},
			AttributeFormDefault: lexicalAttribute{Value: "unqualified", Present: true},
		})
		if err != nil {
			t.Fatalf("ParseSchemaDefaults() error = %v", err)
		}
		if got.TargetNamespace != "urn:test" ||
			got.BlockDefault != derivationBlockDefaultMask ||
			got.FinalDefault != DerivationExtension|DerivationRestriction ||
			!got.ElementQualified ||
			got.AttributeQualified {
			t.Fatalf("ParseSchemaDefaults() = %+v, want parsed defaults", got)
		}
	})

	tests := []struct {
		name        string
		attrs       schemaDocumentSource
		wantMessage string
	}{
		{
			name:        "empty target namespace",
			attrs:       schemaDocumentSource{TargetNamespace: lexicalAttribute{Present: true}},
			wantMessage: "schema targetNamespace cannot be empty",
		},
		{
			name:        "invalid block default",
			attrs:       schemaDocumentSource{BlockDefault: lexicalAttribute{Value: "list"}},
			wantMessage: "schema blockDefault cannot contain \"list\"",
		},
		{
			name:        "invalid final default",
			attrs:       schemaDocumentSource{FinalDefault: lexicalAttribute{Value: "#all extension"}},
			wantMessage: "schema finalDefault cannot combine #all with other values",
		},
		{
			name:        "invalid element form default",
			attrs:       schemaDocumentSource{ElementFormDefault: lexicalAttribute{Value: "local", Present: true}},
			wantMessage: "invalid \"elementFormDefault\" value \"local\"",
		},
		{
			name:        "invalid attribute form default",
			attrs:       schemaDocumentSource{AttributeFormDefault: lexicalAttribute{Value: "local", Present: true}},
			wantMessage: "invalid \"attributeFormDefault\" value \"local\"",
		},
		{
			name: "target namespace before block",
			attrs: schemaDocumentSource{
				TargetNamespace: lexicalAttribute{Present: true},
				BlockDefault:    lexicalAttribute{Value: "list"},
			},
			wantMessage: "schema targetNamespace cannot be empty",
		},
		{
			name: "block before final",
			attrs: schemaDocumentSource{
				BlockDefault: lexicalAttribute{Value: "list"},
				FinalDefault: lexicalAttribute{Value: "#all extension"},
			},
			wantMessage: "schema blockDefault cannot contain \"list\"",
		},
		{
			name: "element form before attribute form",
			attrs: schemaDocumentSource{
				ElementFormDefault:   lexicalAttribute{Value: "local", Present: true},
				AttributeFormDefault: lexicalAttribute{Value: "local", Present: true},
			},
			wantMessage: "invalid \"elementFormDefault\" value \"local\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseSchemaDefaults(tt.attrs)
			expectInvalidAttributeMessage(t, err, tt.wantMessage)
		})
	}
}

func TestParseLocalFormAttrs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		parse       func(formAttr) (bool, error)
		attr        formAttr
		want        bool
		wantMessage string
	}{
		{
			name:  "element missing uses default",
			parse: parseElementFormAttr,
			attr:  formAttr{DefaultQualified: true},
			want:  true,
		},
		{
			name:  "attribute qualified",
			parse: parseAttributeFormAttr,
			attr:  formAttr{Value: "qualified", HasValue: true},
			want:  true,
		},
		{
			name:  "element unqualified",
			parse: parseElementFormAttr,
			attr:  formAttr{Value: "unqualified", HasValue: true, DefaultQualified: true},
		},
		{
			name:        "invalid element",
			parse:       parseElementFormAttr,
			attr:        formAttr{Value: "local", HasValue: true},
			wantMessage: "invalid element form value \"local\"",
		},
		{
			name:        "invalid attribute",
			parse:       parseAttributeFormAttr,
			attr:        formAttr{Value: "local", HasValue: true},
			wantMessage: "invalid attribute form \"local\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.parse(tt.attr)
			if tt.wantMessage != "" {
				expectInvalidAttributeMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("parse form attr error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parse form attr = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDocumentDefaultsRequireTypedSource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		root *schemaNode
	}{
		{name: "missing root"},
		{name: "missing source", root: &schemaNode{}},
		{name: "wrong source", root: &schemaNode{semantic: schemaSemanticSource{kind: &schemaElementSource{}}}},
		{name: "nil document source", root: &schemaNode{semantic: schemaSemanticSource{kind: (*schemaDocumentSource)(nil)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantMessage := "schema defaults require a parsed root"
			var wantPath string
			var wantLine, wantColumn int
			if tt.root != nil {
				wantMessage = "schema defaults require a typed document source"
				wantPath, wantLine, wantColumn = "defaults.xsd", 3, 5
				tt.root.doc = &schemaDocument{name: wantPath}
				tt.root.line, tt.root.column = wantLine, wantColumn
			}
			got, err := parseDocumentDefaultsNode(tt.root)
			issue, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || got != (schemaDefaults{}) {
				t.Fatalf("parseDocumentDefaultsNode() = %+v, %v, want zero defaults and invariant error", got, err)
			}
			if issue.Category() != xsderrors.CategoryInternal || issue.Code() != xsderrors.CodeInternalInvariant || issue.Message() != wantMessage {
				t.Fatalf("diagnostic = {%s %s %q}, want internal invariant %q", issue.Category(), issue.Code(), issue.Message(), wantMessage)
			}
			if issue.Path() != wantPath || issue.Line() != wantLine || issue.Column() != wantColumn {
				t.Fatalf("location = {%q %d %d}, want {%q %d %d}", issue.Path(), issue.Line(), issue.Column(), wantPath, wantLine, wantColumn)
			}
		})
	}
}

func expectInvalidAttributeMessage(t *testing.T, err error, message string) {
	t.Helper()
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want xsderrors.Error", err)
	}
	if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != xsderrors.CodeSchemaInvalidAttribute || diag.Message() != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category(), diag.Code(), diag.Message(), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute, message)
	}
}

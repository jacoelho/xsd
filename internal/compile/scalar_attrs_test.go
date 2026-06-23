package compile

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseBooleanAttr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		attr        BooleanAttr
		want        bool
		wantMessage string
	}{
		{
			name: "missing uses default",
			attr: BooleanAttr{Default: true},
			want: true,
		},
		{
			name: "true literal",
			attr: BooleanAttr{Name: "mixed", Value: "\ttrue\n", HasValue: true},
			want: true,
		},
		{
			name: "one literal",
			attr: BooleanAttr{Name: "mixed", Value: "1", HasValue: true},
			want: true,
		},
		{
			name: "false literal",
			attr: BooleanAttr{Name: "mixed", Value: "\rfalse ", HasValue: true},
		},
		{
			name: "zero literal",
			attr: BooleanAttr{Name: "mixed", Value: "0", HasValue: true},
		},
		{
			name:        "invalid",
			attr:        BooleanAttr{Name: "mixed", Value: "yes", HasValue: true},
			wantMessage: "invalid boolean attribute mixed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseBooleanAttr(tt.attr)
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
		attr        FormAttr
		want        bool
		wantMessage string
	}{
		{
			name: "missing uses zero default",
			attr: FormAttr{Name: "elementFormDefault"},
		},
		{
			name: "missing uses explicit default",
			attr: FormAttr{Name: "elementFormDefault", DefaultQualified: true},
			want: true,
		},
		{
			name: "qualified",
			attr: FormAttr{Name: "elementFormDefault", Value: "qualified", HasValue: true},
			want: true,
		},
		{
			name: "unqualified",
			attr: FormAttr{Name: "elementFormDefault", Value: "unqualified", HasValue: true},
		},
		{
			name:        "invalid",
			attr:        FormAttr{Name: "elementFormDefault", Value: "local", HasValue: true},
			wantMessage: "invalid elementFormDefault value local",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseFormDefaultAttr(tt.attr)
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

	t.Run("valid defaults", func(t *testing.T) {
		t.Parallel()

		got, err := ParseSchemaDefaults(SchemaDefaultAttrs{
			TargetNamespace:         "urn:test",
			BlockDefault:            "#all",
			FinalDefault:            "extension restriction",
			ElementFormDefault:      "qualified",
			AttributeFormDefault:    "unqualified",
			HasTargetNamespace:      true,
			HasElementFormDefault:   true,
			HasAttributeFormDefault: true,
		})
		if err != nil {
			t.Fatalf("ParseSchemaDefaults() error = %v", err)
		}
		if got.TargetNamespace != "urn:test" ||
			got.BlockDefault != runtime.DerivationBlockDefaultMask ||
			got.FinalDefault != runtime.DerivationExtension|runtime.DerivationRestriction ||
			!got.ElementQualified ||
			got.AttributeQualified {
			t.Fatalf("ParseSchemaDefaults() = %+v, want parsed defaults", got)
		}
	})

	tests := []struct {
		name        string
		attrs       SchemaDefaultAttrs
		wantMessage string
	}{
		{
			name:        "empty target namespace",
			attrs:       SchemaDefaultAttrs{HasTargetNamespace: true},
			wantMessage: "schema targetNamespace cannot be empty",
		},
		{
			name:        "invalid block default",
			attrs:       SchemaDefaultAttrs{BlockDefault: "list"},
			wantMessage: "schema blockDefault cannot contain list",
		},
		{
			name:        "invalid final default",
			attrs:       SchemaDefaultAttrs{FinalDefault: "#all extension"},
			wantMessage: "schema finalDefault cannot combine #all with other values",
		},
		{
			name:        "invalid element form default",
			attrs:       SchemaDefaultAttrs{ElementFormDefault: "local", HasElementFormDefault: true},
			wantMessage: "invalid elementFormDefault value local",
		},
		{
			name:        "invalid attribute form default",
			attrs:       SchemaDefaultAttrs{AttributeFormDefault: "local", HasAttributeFormDefault: true},
			wantMessage: "invalid attributeFormDefault value local",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseSchemaDefaults(tt.attrs)
			expectInvalidAttributeMessage(t, err, tt.wantMessage)
		})
	}
}

func TestParseLocalFormAttrs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		parse       func(FormAttr) (bool, error)
		attr        FormAttr
		want        bool
		wantMessage string
	}{
		{
			name:  "element missing uses default",
			parse: ParseElementFormAttr,
			attr:  FormAttr{DefaultQualified: true},
			want:  true,
		},
		{
			name:  "attribute qualified",
			parse: ParseAttributeFormAttr,
			attr:  FormAttr{Value: "qualified", HasValue: true},
			want:  true,
		},
		{
			name:  "element unqualified",
			parse: ParseElementFormAttr,
			attr:  FormAttr{Value: "unqualified", HasValue: true, DefaultQualified: true},
		},
		{
			name:        "invalid element",
			parse:       ParseElementFormAttr,
			attr:        FormAttr{Value: "local", HasValue: true},
			wantMessage: "invalid element form value local",
		},
		{
			name:        "invalid attribute",
			parse:       ParseAttributeFormAttr,
			attr:        FormAttr{Value: "local", HasValue: true},
			wantMessage: "invalid attribute form local",
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

func expectInvalidAttributeMessage(t *testing.T, err error, message string) {
	t.Helper()
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want xsderrors.Error", err)
	}
	if diag.Category != xsderrors.CategorySchemaCompile || diag.Code != xsderrors.CodeSchemaInvalidAttribute || diag.Message != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category, diag.Code, diag.Message, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute, message)
	}
}

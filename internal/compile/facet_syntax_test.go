package compile

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateFacetSource(t *testing.T) {
	t.Parallel()

	stringType := FacetSource{
		Variety:   runtime.SimpleVarietyAtomic,
		Primitive: runtime.PrimitiveString,
	}
	booleanType := FacetSource{
		Variety:   runtime.SimpleVarietyAtomic,
		Primitive: runtime.PrimitiveBoolean,
	}

	tests := []struct {
		name        string
		source      FacetSource
		wantCompile bool
		wantMessage string
	}{
		{
			name:   "skips non xsd non facet child",
			source: FacetSource{Local: "other"},
		},
		{
			name:        "rejects unsupported xsd facet",
			source:      FacetSource{Local: "other", InXSDNamespace: true},
			wantMessage: "unsupported facet other",
		},
		{
			name:        "rejects missing value",
			source:      FacetSource{Local: "length", InXSDNamespace: true, Variety: stringType.Variety, Primitive: stringType.Primitive},
			wantMessage: "length missing value",
		},
		{
			name:        "rejects facet not allowed for type",
			source:      FacetSource{Local: "length", InXSDNamespace: true, HasValue: true, Variety: booleanType.Variety, Primitive: booleanType.Primitive},
			wantMessage: "facet length is not allowed",
		},
		{
			name:        "admits allowed facet",
			source:      FacetSource{Local: "length", InXSDNamespace: true, HasValue: true, Variety: stringType.Variety, Primitive: stringType.Primitive},
			wantCompile: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ValidateFacetSource(tt.source)
			if tt.wantMessage != "" {
				expectSchemaFacetMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("ValidateFacetSource() error = %v", err)
			}
			if got != tt.wantCompile {
				t.Fatalf("ValidateFacetSource() = %v, want %v", got, tt.wantCompile)
			}
		})
	}
}

func TestIsFacetLocal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		local string
		want  bool
	}{
		{local: "length", want: true},
		{local: "minLength", want: true},
		{local: "maxLength", want: true},
		{local: "totalDigits", want: true},
		{local: "fractionDigits", want: true},
		{local: "minInclusive", want: true},
		{local: "maxInclusive", want: true},
		{local: "minExclusive", want: true},
		{local: "maxExclusive", want: true},
		{local: "enumeration", want: true},
		{local: "pattern", want: true},
		{local: "whiteSpace", want: true},
		{local: "attribute"},
	}
	for _, tt := range tests {
		t.Run(tt.local, func(t *testing.T) {
			t.Parallel()

			if got := IsFacetLocal(tt.local); got != tt.want {
				t.Fatalf("IsFacetLocal(%q) = %v, want %v", tt.local, got, tt.want)
			}
		})
	}
}

func TestParseWhitespaceFacetValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		value       string
		base        runtime.WhitespaceMode
		want        runtime.WhitespaceMode
		wantMessage string
	}{
		{name: "preserve", value: "preserve", base: runtime.WhitespacePreserve, want: runtime.WhitespacePreserve},
		{name: "replace", value: "replace", base: runtime.WhitespacePreserve, want: runtime.WhitespaceReplace},
		{name: "collapse", value: "collapse", base: runtime.WhitespaceReplace, want: runtime.WhitespaceCollapse},
		{name: "invalid lexical", value: "trim", base: runtime.WhitespacePreserve, wantMessage: "invalid whiteSpace facet trim"},
		{name: "looser", value: "replace", base: runtime.WhitespaceCollapse, wantMessage: "whiteSpace cannot loosen base whiteSpace"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseWhitespaceFacetValue(tt.value, tt.base)
			if tt.wantMessage != "" {
				expectSchemaFacetMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("ParseWhitespaceFacetValue() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseWhitespaceFacetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func expectSchemaFacetMessage(t *testing.T, err error, message string) {
	t.Helper()
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want xsderrors.Error", err)
	}
	if diag.Category != xsderrors.CategorySchemaCompile || diag.Code != xsderrors.CodeSchemaFacet || diag.Message != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category, diag.Code, diag.Message, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaFacet, message)
	}
}

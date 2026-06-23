package compile

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseSizeFacetValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		facet string
		value string
		want  uint32
		code  xsderrors.Code
		msg   string
	}{
		{name: "plain", facet: "length", value: "7", want: 7},
		{name: "plus", facet: "length", value: "+1", want: 1},
		{name: "negative zero", facet: "length", value: "-0", want: 0},
		{name: "leading zeros", facet: "minLength", value: "00012", want: 12},
		{name: "max uint32", facet: "maxLength", value: "4294967295", want: 4294967295},
		{
			name:  "empty",
			facet: "length",
			value: "",
			code:  xsderrors.CodeSchemaFacet,
			msg:   "invalid length facet ",
		},
		{
			name:  "sign only",
			facet: "length",
			value: "-",
			code:  xsderrors.CodeSchemaFacet,
			msg:   "invalid length facet -",
		},
		{
			name:  "text",
			facet: "fractionDigits",
			value: "1x",
			code:  xsderrors.CodeSchemaFacet,
			msg:   "invalid fractionDigits facet 1x",
		},
		{
			name:  "negative non-zero",
			facet: "length",
			value: "-1",
			code:  xsderrors.CodeSchemaFacet,
			msg:   "invalid length facet -1",
		},
		{
			name:  "totalDigits zero",
			facet: "totalDigits",
			value: "0",
			code:  xsderrors.CodeSchemaFacet,
			msg:   "totalDigits must be positive",
		},
		{
			name:  "totalDigits negative zero",
			facet: "totalDigits",
			value: "-0",
			code:  xsderrors.CodeSchemaFacet,
			msg:   "totalDigits must be positive",
		},
		{
			name:  "overflow",
			facet: "maxLength",
			value: "4294967296",
			code:  xsderrors.CodeSchemaLimit,
			msg:   "maxLength facet exceeds uint32 limit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseSizeFacetValue(tt.facet, tt.value)
			if tt.code == "" {
				if err != nil {
					t.Fatalf("ParseSizeFacetValue() error = %v", err)
				}
				if got != tt.want {
					t.Fatalf("ParseSizeFacetValue() = %d, want %d", got, tt.want)
				}
				return
			}
			var xerr *xsderrors.Error
			if !errors.As(err, &xerr) {
				t.Fatalf("ParseSizeFacetValue() error = %v, want *xsderrors.Error", err)
			}
			if xerr.Code != tt.code {
				t.Fatalf("ParseSizeFacetValue() code = %s, want %s", xerr.Code, tt.code)
			}
			if xerr.Message != tt.msg {
				t.Fatalf("ParseSizeFacetValue() message = %q, want %q", xerr.Message, tt.msg)
			}
		})
	}
}

func TestValidateCompiledFacets(t *testing.T) {
	t.Parallel()

	base := runtime.SimpleType{
		Variety:   runtime.SimpleVarietyAtomic,
		Primitive: runtime.PrimitiveString,
	}
	if err := ValidateCompiledFacets(base, base, runtime.OrderedFacetStep{}); err != nil {
		t.Fatalf("ValidateCompiledFacets(valid) error = %v", err)
	}

	minLength := uint32(2)
	maxLength := uint32(1)
	st := base
	st.Facets.MinLength = minLength
	st.Facets.MaxLength = maxLength
	st.Facets.Present = runtime.FacetMinLength | runtime.FacetMaxLength
	err := ValidateCompiledFacets(st, base, runtime.OrderedFacetStep{})
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("ValidateCompiledFacets(invalid) error = %v, want *xsderrors.Error", err)
	}
	if xerr.Code != xsderrors.CodeSchemaFacet {
		t.Fatalf("ValidateCompiledFacets(invalid) code = %s, want %s", xerr.Code, xsderrors.CodeSchemaFacet)
	}
	if xerr.Message != "minLength cannot exceed maxLength" {
		t.Fatalf("ValidateCompiledFacets(invalid) message = %q, want minLength cannot exceed maxLength", xerr.Message)
	}
}

func TestFacetValueError(t *testing.T) {
	t.Parallel()

	if err := FacetValueError("bad", nil); err != nil {
		t.Fatalf("FacetValueError(nil) error = %v", err)
	}
	unsupported := xsderrors.Unsupported(xsderrors.CodeUnsupportedRegex, "unsupported regex")
	if err := FacetValueError("bad", unsupported); !errors.Is(err, unsupported) {
		t.Fatalf("FacetValueError(unsupported) = %v, want original unsupported error", err)
	}
	err := FacetValueError("bad", fmt.Errorf("runtime reject"))
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("FacetValueError(reject) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaFacet {
		t.Fatalf("diagnostic = %s/%s, want schema compile facet", xerr.Category, xerr.Code)
	}
	if xerr.Message != "invalid facet value bad" {
		t.Fatalf("message = %q, want invalid facet value", xerr.Message)
	}
}

func TestDeclarationValueConstraintError(t *testing.T) {
	t.Parallel()

	if err := DeclarationValueConstraintError("element fixed", "p:e", nil); err != nil {
		t.Fatalf("DeclarationValueConstraintError(nil) error = %v", err)
	}
	unsupported := xsderrors.Unsupported(xsderrors.CodeUnsupportedRegex, "unsupported regex")
	if err := DeclarationValueConstraintError("element fixed", "p:e", unsupported); !errors.Is(err, unsupported) {
		t.Fatalf("DeclarationValueConstraintError(unsupported) = %v, want original unsupported error", err)
	}
	err := DeclarationValueConstraintError("element fixed", "p:e", fmt.Errorf("runtime reject"))
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("DeclarationValueConstraintError(reject) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaFacet {
		t.Fatalf("diagnostic = %s/%s, want schema compile facet", xerr.Category, xerr.Code)
	}
	if xerr.Message != "invalid element fixed value for p:e" {
		t.Fatalf("message = %q, want invalid value constraint", xerr.Message)
	}
}

func TestElementValueConstraintTypeError(t *testing.T) {
	t.Parallel()

	if err := ElementValueConstraintTypeError(nil); err != nil {
		t.Fatalf("ElementValueConstraintTypeError(nil) error = %v", err)
	}
	err := ElementValueConstraintTypeError(fmt.Errorf("owner reject"))
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("ElementValueConstraintTypeError(reject) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaInvalidAttribute {
		t.Fatalf("diagnostic = %s/%s, want schema compile invalid attribute", xerr.Category, xerr.Code)
	}
	if xerr.Message != "owner reject" {
		t.Fatalf("message = %q, want runtime message", xerr.Message)
	}
}

func TestElementValueConstraintRuntimeError(t *testing.T) {
	t.Parallel()

	if err := ElementValueConstraintRuntimeError(nil); err != nil {
		t.Fatalf("ElementValueConstraintRuntimeError(nil) error = %v", err)
	}

	err := ElementValueConstraintRuntimeError(runtime.ErrBareNotationValueConstraint)
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("ElementValueConstraintRuntimeError(bare notation) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaFacet {
		t.Fatalf("bare notation diagnostic = %s/%s, want schema compile facet", xerr.Category, xerr.Code)
	}
	if xerr.Message != runtime.ErrBareNotationValueConstraint.Error() {
		t.Fatalf("bare notation message = %q, want runtime message", xerr.Message)
	}

	err = ElementValueConstraintRuntimeError(fmt.Errorf("runtime reject"))
	if !errors.As(err, &xerr) {
		t.Fatalf("ElementValueConstraintRuntimeError(reject) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaInvalidAttribute {
		t.Fatalf("runtime reject diagnostic = %s/%s, want schema compile invalid attribute", xerr.Category, xerr.Code)
	}
	if xerr.Message != "runtime reject" {
		t.Fatalf("runtime reject message = %q, want runtime message", xerr.Message)
	}
}

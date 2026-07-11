package compile

import (
	"encoding/xml"
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestDerivedSimpleTypeSharesImmutableInheritedStorage(t *testing.T) {
	t.Parallel()

	members := []runtime.SimpleTypeID{1, 2}
	enumeration := []runtime.CompiledLiteral{{Canonical: "inherited"}}
	base := runtime.SimpleType{
		Union:      members,
		Base:       runtime.NoSimpleType,
		ListItem:   runtime.NoSimpleType,
		Variety:    runtime.SimpleVarietyUnion,
		Primitive:  runtime.PrimitiveString,
		Whitespace: runtime.WhitespaceCollapse,
		Facets: runtime.FacetSet{
			Enumeration: enumeration,
			Present:     runtime.FacetEnumeration,
		},
	}
	derived := derivedSimpleType(base, 0, runtime.QName{})
	if &derived.Union[0] != &base.Union[0] {
		t.Fatal("derived restriction cloned immutable union storage")
	}
	if &derived.Facets.Enumeration[0] != &base.Facets.Enumeration[0] {
		t.Fatal("derived restriction cloned immutable enumeration storage")
	}

	c := compiler{rt: runtime.SchemaBuild{SimpleTypes: []runtime.SimpleType{base}}}
	pattern := &rawNode{
		Name: xml.Name{Space: runtime.XSDNamespaceURI, Local: vocab.XSDFacetPattern},
		Attr: []xml.Attr{{Name: xml.Name{Local: vocab.XSDAttrValue}, Value: "[A-Z]+"}},
	}
	if err := c.compileFacets(&rawNode{Children: []*rawNode{pattern}}, &derived, 0, 0); err != nil {
		t.Fatalf("compileFacets() error = %v", err)
	}
	if &derived.Union[0] != &base.Union[0] {
		t.Fatal("facet compilation detached immutable union storage")
	}
	if &base.Facets.Enumeration[0] != &enumeration[0] || base.Facets.Present != runtime.FacetEnumeration {
		t.Fatal("facet compilation mutated base facets")
	}
	if derived.Facets.Present&runtime.FacetPattern == 0 {
		t.Fatal("pattern facet was not compiled")
	}
}

func TestCompiledFacetStatePreservesUnmodifiedEnumeration(t *testing.T) {
	t.Parallel()

	enumeration := []runtime.CompiledLiteral{{Canonical: "a"}}
	base := runtime.SimpleType{Facets: runtime.FacetSet{
		Enumeration: enumeration,
		Present:     runtime.FacetEnumeration,
	}}

	derived := base
	var scalarStep compiledFacetState
	scalarStep.beginStep(&derived)
	scalarStep.apply(&derived)
	if &derived.Facets.Enumeration[0] != &base.Facets.Enumeration[0] {
		t.Fatal("unmodified enumeration storage was cloned")
	}
}

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

package schema

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateFacetSource(t *testing.T) {
	t.Parallel()

	stringType := facetSource{
		Variety:   valuepkg.Atomic,
		Primitive: valuepkg.PrimitiveString,
	}
	booleanType := facetSource{
		Variety:   valuepkg.Atomic,
		Primitive: valuepkg.PrimitiveBoolean,
	}

	tests := []struct {
		name        string
		source      facetSource
		want        valuepkg.FacetMask
		wantMessage string
	}{
		{
			name:        "rejects facet not allowed for type",
			source:      facetSource{Local: "length", Variety: booleanType.Variety, Primitive: booleanType.Primitive},
			wantMessage: "facet length is not allowed",
		},
		{
			name:   "admits allowed facet",
			source: facetSource{Local: "length", Variety: stringType.Variety, Primitive: stringType.Primitive},
			want:   valuepkg.FacetLength,
		},
		{
			name:   "admits another allowed facet",
			source: facetSource{Local: "pattern", Variety: booleanType.Variety, Primitive: booleanType.Primitive},
			want:   valuepkg.FacetPattern,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := validateFacetSource(tt.source)
			if tt.wantMessage != "" {
				expectSchemaFacetMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("validateFacetSource() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("validateFacetSource() = %v, want %v", got, tt.want)
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

			if got := isFacetLocal(tt.local); got != tt.want {
				t.Fatalf("IsFacetLocal(%q) = %v, want %v", tt.local, got, tt.want)
			}
		})
	}
}

func TestAdmitFacetValueDecidesOnlyTypeIndependentValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		element  string
		value    string
		want     facetValue
		wantCode xsderrors.Code
	}{
		{element: "length", value: " +07 ", want: facetValue{size: 7}},
		{element: "fractionDigits", value: "0", want: facetValue{}},
		{element: "totalDigits", value: "0", wantCode: xsderrors.CodeSchemaFacet},
		{element: "maxLength", value: "4294967295", want: facetValue{size: 4294967295}},
		{element: "maxLength", value: "4294967296", wantCode: xsderrors.CodeSchemaLimit},
		{element: "minLength", value: "-1", wantCode: xsderrors.CodeSchemaFacet},
		{element: "whiteSpace", value: " replace ", want: facetValue{whitespace: valuepkg.WhitespaceReplace}},
		{element: "whiteSpace", value: "trim", wantCode: xsderrors.CodeSchemaFacet},
		{element: "minInclusive", value: "not a number", want: facetValue{}},
		{element: "enumeration", value: "", want: facetValue{}},
		{element: "pattern", value: "[a", wantCode: xsderrors.CodeSchemaFacet},
		{element: "pattern", value: "(a{1000}){1000}", wantCode: xsderrors.CodeSchemaLimit},
	}
	for _, tt := range tests {
		t.Run(tt.element+"="+tt.value, func(t *testing.T) {
			t.Parallel()

			got, err := admitFacetValue(tt.element, tt.value, testSchemaPatterns(defaultPatternLimits))
			if tt.wantCode != "" {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, tt.wantCode)
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("admitFacetValue(%q, %q) = %+v, %v, want %+v", tt.element, tt.value, got, err, tt.want)
			}
		})
	}
}

func TestParseWhitespaceFacetValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		value       string
		want        valuepkg.WhitespaceMode
		wantMessage string
	}{
		{name: "preserve", value: "preserve", want: valuepkg.WhitespacePreserve},
		{name: "replace", value: "replace", want: valuepkg.WhitespaceReplace},
		{name: "collapse", value: "collapse", want: valuepkg.WhitespaceCollapse},
		{name: "collapsed lexical", value: " \tcollapse\n", want: valuepkg.WhitespaceCollapse},
		{name: "invalid lexical", value: "trim", wantMessage: "invalid whiteSpace facet \"trim\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseWhitespaceFacetValue(tt.value)
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
	if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != xsderrors.CodeSchemaFacet || diag.Message() != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category(), diag.Code(), diag.Message(), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaFacet, message)
	}
}

func BenchmarkSizeFacetAdmission(b *testing.B) {
	cases := []struct {
		name, text string
		want       uint32
	}{{"Short", "8", 8}, {"MinusZero", "-" + strings.Repeat("0", 4096), 0}}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			patterns := testSchemaPatterns(defaultPatternLimits)
			pre, err := admitFacetValue("maxLength", c.text, patterns)
			if err != nil || pre.size != c.want {
				b.Fatalf("preflight %v %v", pre, err)
			}
			var last facetValue
			b.ReportAllocs()
			for b.Loop() {
				var err error
				last, err = admitFacetValue("maxLength", c.text, patterns)
				if err != nil {
					b.Fatal(err)
				}
			}
			if last.size != c.want {
				b.Fatalf("postflight %v", last)
			}
		})
	}
}

func TestFacetFixedIsAdmittedInEitherAttributeOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		element  string
		attrs    []schemaAttribute
		want     facetValue
		wantCode xsderrors.Code
	}{
		{name: "fixed before value", element: "maxLength", attrs: []schemaAttribute{testAttr("fixed", "true"), testAttr("value", "3")}, want: facetValue{size: 3, fixed: true}},
		{name: "fixed after value", element: "maxLength", attrs: []schemaAttribute{testAttr("value", "3"), testAttr("fixed", " 1 ")}, want: facetValue{size: 3, fixed: true}},
		{name: "fixed false", element: "maxLength", attrs: []schemaAttribute{testAttr("fixed", "false"), testAttr("value", "3")}, want: facetValue{size: 3}},
		{name: "maxLength fixed not boolean", element: "maxLength", attrs: []schemaAttribute{testAttr("fixed", "x")}, wantCode: xsderrors.CodeSchemaInvalidAttribute},
		{name: "whiteSpace fixed not boolean", element: "whiteSpace", attrs: []schemaAttribute{testAttr("fixed", "x")}, wantCode: xsderrors.CodeSchemaInvalidAttribute},
		{name: "element fixed is a value constraint", element: "element", attrs: []schemaAttribute{testAttr("fixed", "x")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var values admittedValues
			admitted := startTagAttributes{values: &values}
			var err error
			for _, attr := range tt.attrs {
				if err = admitParsedAttributeValue(tt.element, attr, admitted); err != nil {
					break
				}
			}
			if tt.wantCode != "" {
				d, ok := errors.AsType[*xsderrors.Error](err)
				if !ok || d.Code() != tt.wantCode || d.Message() != `invalid boolean attribute "fixed"` {
					t.Fatalf("admission error = %v, want %s invalid boolean attribute \"fixed\"", err, tt.wantCode)
				}
				return
			}
			if err != nil || values.facet != tt.want {
				t.Fatalf("admitted facet = %+v, %v, want %+v", values.facet, err, tt.want)
			}
		})
	}
}

func testAttr(local, value string) schemaAttribute {
	return schemaAttribute{Name: xml.Name{Local: local}, Value: value}
}

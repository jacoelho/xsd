package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/diagnostictext"
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
		{name: "collapsed whitespace", facet: "fractionDigits", value: " \t2\n", want: 2},
		{name: "max uint32", facet: "maxLength", value: "4294967295", want: 4294967295},
		{
			name:  "empty",
			facet: "length",
			value: "",
			code:  xsderrors.CodeSchemaFacet,
			msg:   "invalid length facet " + diagnostictext.Quote(""),
		},
		{
			name:  "sign only",
			facet: "length",
			value: "-",
			code:  xsderrors.CodeSchemaFacet,
			msg:   "invalid length facet " + diagnostictext.Quote("-"),
		},
		{
			name:  "text",
			facet: "fractionDigits",
			value: "1x",
			code:  xsderrors.CodeSchemaFacet,
			msg:   `invalid fractionDigits facet "1x"`,
		},
		{
			name:  "negative non-zero",
			facet: "length",
			value: "-1",
			code:  xsderrors.CodeSchemaFacet,
			msg:   "invalid length facet " + diagnostictext.Quote("-1"),
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
		{name: "long negative zero", facet: "length", value: "-" + strings.Repeat("0", 1024), want: 0},
		{name: "overflow malformed tail", facet: "maxLength", value: strings.Repeat("9", 256) + "x", code: xsderrors.CodeSchemaFacet, msg: "invalid maxLength facet " + diagnostictext.Quote(strings.Repeat("9", 256)+"x")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseSizeFacetValue(tt.facet, tt.value)
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
			if xerr.Code() != tt.code {
				t.Fatalf("ParseSizeFacetValue() code = %s, want %s", xerr.Code(), tt.code)
			}
			if xerr.Message() != tt.msg {
				t.Fatalf("ParseSizeFacetValue() message = %q, want %q", xerr.Message(), tt.msg)
			}
		})
	}
}

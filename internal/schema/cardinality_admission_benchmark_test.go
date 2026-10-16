package schema_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/xsderrors"
)

func occurrence286Error(tb testing.TB, err error, code xsderrors.Code, message string) {
	tb.Helper()
	d, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || d.Category() != xsderrors.CategorySchemaCompile || d.Code() != code || d.Message() != message {
		tb.Fatalf("unexpected diagnostic: %v", err)
	}
}

func BenchmarkOccurrenceAdmission(b *testing.B) {
	zeros := strings.Repeat("0", 4096)
	cases := []struct {
		name, min, max string
		want           schema.Occurrence
		message        string
	}{
		{"Short", "+2", "+4", schema.Occurrence{Min: 2, Max: 4}, ""},
		{"LongZeros", "+" + zeros + "1", "+" + zeros + "4", schema.Occurrence{Min: 1, Max: 4}, ""},
		{"AllZero", zeros, zeros, schema.Occurrence{}, ""},
		{"OverflowMalformed", "1", "42949672960x", schema.Occurrence{}, "invalid maxOccurs 42949672960x"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			attrs := schema.OccurrenceAttrs{MinOccurs: c.min, MaxOccurs: c.max, HasMinOccurs: true, HasMaxOccurs: true}
			var last schema.Occurrence
			pre, err := schema.ParseOccurrence(attrs, schema.Limits{})
			if c.message == "" {
				if err != nil || pre != c.want {
					b.Fatalf("preflight %v %v", pre, err)
				}
			} else {
				occurrence286Error(b, err, xsderrors.CodeSchemaOccurrence, c.message)
			}
			b.ReportAllocs()
			for b.Loop() {
				var err error
				last, err = schema.ParseOccurrence(attrs, schema.Limits{})
				if c.message == "" {
					if err != nil {
						b.Fatal(err)
					}
				} else {
					d, ok := errors.AsType[*xsderrors.Error](err)
					if !ok || d.Category() != xsderrors.CategorySchemaCompile || d.Code() != xsderrors.CodeSchemaOccurrence || d.Message() != c.message {
						b.Fatalf("unexpected failure: %v", err)
					}
				}
			}
			if last != c.want {
				b.Fatalf("postflight %v", last)
			}
		})
	}
}

func BenchmarkSizeFacetAdmission(b *testing.B) {
	cases := []struct {
		name, text string
		want       uint32
	}{{"Short", "8", 8}, {"MinusZero", "-" + strings.Repeat("0", 4096), 0}}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			pre, err := schema.ParseSizeFacetValue("maxLength", c.text)
			if err != nil || pre != c.want {
				b.Fatalf("preflight %v %v", pre, err)
			}
			var last uint32
			b.ReportAllocs()
			for b.Loop() {
				var err error
				last, err = schema.ParseSizeFacetValue("maxLength", c.text)
				if err != nil {
					b.Fatal(err)
				}
			}
			if last != c.want {
				b.Fatalf("postflight %v", last)
			}
		})
	}
}

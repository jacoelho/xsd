package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseOccurrence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		attrs    OccurrenceAttrs
		limits   Limits
		want     Occurrence
		wantCode xsderrors.Code
		wantText string
	}{
		{
			name: "defaults",
			want: Occurrence{Min: 1, Max: 1},
		},
		{
			name:  "trims XML whitespace and plus",
			attrs: OccurrenceAttrs{MinOccurs: "\t+0002\n", MaxOccurs: "\r0003 ", HasMinOccurs: true, HasMaxOccurs: true},
			want:  Occurrence{Min: 2, Max: 3},
		},
		{
			name:  "zero",
			attrs: OccurrenceAttrs{MinOccurs: "0", MaxOccurs: "0", HasMinOccurs: true, HasMaxOccurs: true},
			want:  Occurrence{},
		},
		{
			name:  "unbounded",
			attrs: OccurrenceAttrs{MinOccurs: "2", MaxOccurs: " unbounded\t", HasMinOccurs: true, HasMaxOccurs: true},
			want:  Occurrence{Min: 2, Unbounded: true},
		},
		{
			name:     "invalid min",
			attrs:    OccurrenceAttrs{MinOccurs: "x", HasMinOccurs: true},
			wantCode: xsderrors.CodeSchemaOccurrence,
			wantText: "invalid minOccurs x",
		},
		{
			name:     "invalid max",
			attrs:    OccurrenceAttrs{MaxOccurs: "1.5", HasMaxOccurs: true},
			wantCode: xsderrors.CodeSchemaOccurrence,
			wantText: "invalid maxOccurs 1.5",
		},
		{
			name:     "max less than min",
			attrs:    OccurrenceAttrs{MinOccurs: "2", MaxOccurs: "1", HasMinOccurs: true, HasMaxOccurs: true},
			wantCode: xsderrors.CodeSchemaOccurrence,
			wantText: "maxOccurs is less than minOccurs",
		},
		{
			name:     "min over uint32",
			attrs:    OccurrenceAttrs{MinOccurs: "4294967296", HasMinOccurs: true},
			wantCode: xsderrors.CodeSchemaLimit,
			wantText: "minOccurs exceeds uint32 limit",
		},
		{
			name:     "max over uint32",
			attrs:    OccurrenceAttrs{MaxOccurs: "4294967296", HasMaxOccurs: true},
			wantCode: xsderrors.CodeSchemaLimit,
			wantText: "maxOccurs exceeds uint32 limit",
		},
		{
			name:     "max over configured limit",
			attrs:    OccurrenceAttrs{MaxOccurs: "11", HasMaxOccurs: true},
			limits:   Limits{MaxFiniteOccurs: 10},
			wantCode: xsderrors.CodeSchemaLimit,
			wantText: "maxOccurs exceeds configured limit",
		},
		{
			name:   "max equals configured limit",
			attrs:  OccurrenceAttrs{MaxOccurs: "10", HasMaxOccurs: true},
			limits: Limits{MaxFiniteOccurs: 10},
			want:   Occurrence{Min: 1, Max: 10},
		},
		{
			name:  "max uint32 accepted",
			attrs: OccurrenceAttrs{MaxOccurs: "4294967295", HasMaxOccurs: true},
			want:  Occurrence{Min: 1, Max: ^uint32(0)},
		},
		{name: "long zero prefixes", attrs: OccurrenceAttrs{MinOccurs: strings.Repeat("0", 1024) + "2", MaxOccurs: strings.Repeat("0", 1024) + "3", HasMinOccurs: true, HasMaxOccurs: true}, want: Occurrence{Min: 2, Max: 3}},
		{name: "long all zero", attrs: OccurrenceAttrs{MinOccurs: strings.Repeat("0", 1024), MaxOccurs: strings.Repeat("0", 1024), HasMinOccurs: true, HasMaxOccurs: true}},
		{name: "negative zero min", attrs: OccurrenceAttrs{MinOccurs: "-0", HasMinOccurs: true}, want: Occurrence{Max: 1}},
		{name: "negative zero max below default min", attrs: OccurrenceAttrs{MaxOccurs: "-0", HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "maxOccurs is less than minOccurs"},
		{name: "negative zero bounds", attrs: OccurrenceAttrs{MinOccurs: "-0", MaxOccurs: "-0", HasMinOccurs: true, HasMaxOccurs: true}},
		{name: "long negative zero with whitespace", attrs: OccurrenceAttrs{MinOccurs: "\t-" + strings.Repeat("0", 1024) + "\n", MaxOccurs: "\r-" + strings.Repeat("0", 1024) + " ", HasMinOccurs: true, HasMaxOccurs: true}},
		{name: "negative min", attrs: OccurrenceAttrs{MinOccurs: "-1", HasMinOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid minOccurs -1"},
		{name: "negative max", attrs: OccurrenceAttrs{MinOccurs: "0", MaxOccurs: "-1", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid maxOccurs -1"},
		{name: "negative zero malformed suffix", attrs: OccurrenceAttrs{MinOccurs: "-0x", HasMinOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid minOccurs -0x"},
		{name: "negative zero decimal", attrs: OccurrenceAttrs{MinOccurs: "0", MaxOccurs: "-0.0", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid maxOccurs -0.0"},
		{name: "sign only", attrs: OccurrenceAttrs{MinOccurs: "-", HasMinOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid minOccurs -"},
		{name: "huge magnitude", attrs: OccurrenceAttrs{MaxOccurs: strings.Repeat("9", 256), HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaLimit, wantText: "maxOccurs exceeds uint32 limit"},
		{name: "overflow malformed tail", attrs: OccurrenceAttrs{MaxOccurs: strings.Repeat("9", 256) + "x", HasMaxOccurs: true}, limits: Limits{MaxFiniteOccurs: 10}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid maxOccurs " + strings.Repeat("9", 256) + "x"},
		{name: "minimum lexical error first", attrs: OccurrenceAttrs{MinOccurs: "x", MaxOccurs: "y", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid minOccurs x"},
		{name: "minimum limit error first", attrs: OccurrenceAttrs{MinOccurs: "4294967296", MaxOccurs: "y", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaLimit, wantText: "minOccurs exceeds uint32 limit"},
		{name: "configured cap before ordering", attrs: OccurrenceAttrs{MinOccurs: "12", MaxOccurs: "11", HasMinOccurs: true, HasMaxOccurs: true}, limits: Limits{MaxFiniteOccurs: 10}, wantCode: xsderrors.CodeSchemaLimit, wantText: "maxOccurs exceeds configured limit"},
		{name: "uint32 cap with large configured cap", attrs: OccurrenceAttrs{MaxOccurs: "4294967296", HasMaxOccurs: true}, limits: Limits{MaxFiniteOccurs: ^uint64(0)}, wantCode: xsderrors.CodeSchemaLimit, wantText: "maxOccurs exceeds uint32 limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseOccurrence(tt.attrs, tt.limits)
			if tt.wantCode != "" {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, tt.wantCode)
				if !strings.Contains(err.Error(), tt.wantText) {
					t.Fatalf("ParseOccurrence() error = %v, want text %q", err, tt.wantText)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOccurrence() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseOccurrence() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMaxOccurrenceKindInvariant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		maximum  maxOccurrence
		want     Occurrence
		wantText string
	}{
		{
			name:    "finite",
			maximum: maxOccurrence{value: 3, kind: maxOccurrenceFinite},
			want:    Occurrence{Min: 2, Max: 3},
		},
		{
			name:    "unbounded",
			maximum: maxOccurrence{kind: maxOccurrenceUnbounded},
			want:    Occurrence{Min: 2, Unbounded: true},
		},
		{
			name:     "invalid",
			maximum:  maxOccurrence{kind: maxOccurrenceInvalid},
			wantText: "maxOccurs parser returned an invalid kind",
		},
		{
			name:     "unknown",
			maximum:  maxOccurrence{kind: maxOccurrenceKind(99)},
			wantText: "maxOccurs parser returned an unknown kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.maximum.withMinimum(2)
			if tt.wantText != "" {
				expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
				if !strings.Contains(err.Error(), tt.wantText) {
					t.Fatalf("maxOccurrence.withMinimum() error = %v, want text %q", err, tt.wantText)
				}
				return
			}
			if err != nil {
				t.Fatalf("maxOccurrence.withMinimum() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("maxOccurrence.withMinimum() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateAllModelOccurrence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		occurs   Occurrence
		wantCode xsderrors.Code
	}{
		{name: "required", occurs: Occurrence{Min: 1, Max: 1}},
		{name: "optional", occurs: Occurrence{Min: 0, Max: 1}},
		{name: "zero max", occurs: Occurrence{}, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "min greater than one", occurs: Occurrence{Min: 2, Max: 2}, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "max greater than one", occurs: Occurrence{Min: 0, Max: 2}, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "unbounded", occurs: Occurrence{Min: 0, Unbounded: true}, wantCode: xsderrors.CodeSchemaOccurrence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAllModelOccurrence(tt.occurs)
			if tt.wantCode != "" {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, tt.wantCode)
				if !strings.Contains(err.Error(), "xs:all occurrence must be zero or one") {
					t.Fatalf("ValidateAllModelOccurrence() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateAllModelOccurrence() error = %v", err)
			}
		})
	}
}

func assertOccurrenceBenchmarkDiagnostic(tb testing.TB, err error, code xsderrors.Code, message string) {
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
		want           Occurrence
		message        string
	}{
		{"Short", "+2", "+4", Occurrence{Min: 2, Max: 4}, ""},
		{"LongZeros", "+" + zeros + "1", "+" + zeros + "4", Occurrence{Min: 1, Max: 4}, ""},
		{"AllZero", zeros, zeros, Occurrence{}, ""},
		{"OverflowMalformed", "1", "42949672960x", Occurrence{}, "invalid maxOccurs 42949672960x"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			attrs := OccurrenceAttrs{MinOccurs: c.min, MaxOccurs: c.max, HasMinOccurs: true, HasMaxOccurs: true}
			var last Occurrence
			pre, err := ParseOccurrence(attrs, Limits{})
			if c.message == "" {
				if err != nil || pre != c.want {
					b.Fatalf("preflight %v %v", pre, err)
				}
			} else {
				assertOccurrenceBenchmarkDiagnostic(b, err, xsderrors.CodeSchemaOccurrence, c.message)
			}
			b.ReportAllocs()
			for b.Loop() {
				var err error
				last, err = ParseOccurrence(attrs, Limits{})
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

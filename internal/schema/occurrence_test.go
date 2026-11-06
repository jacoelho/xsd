package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseOccurrence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		attrs    rawOccurrence
		limits   Limits
		want     occurrence
		wantCode xsderrors.Code
		wantText string
	}{
		{
			name: "defaults",
			want: occurrence{Min: 1, Max: 1},
		},
		{
			name:  "trims XML whitespace and plus",
			attrs: rawOccurrence{MinOccurs: "\t+0002\n", MaxOccurs: "\r0003 ", HasMinOccurs: true, HasMaxOccurs: true},
			want:  occurrence{Min: 2, Max: 3},
		},
		{
			name:  "zero",
			attrs: rawOccurrence{MinOccurs: "0", MaxOccurs: "0", HasMinOccurs: true, HasMaxOccurs: true},
			want:  occurrence{},
		},
		{
			name:  "unbounded",
			attrs: rawOccurrence{MinOccurs: "2", MaxOccurs: " unbounded\t", HasMinOccurs: true, HasMaxOccurs: true},
			want:  occurrence{Min: 2, Unbounded: true},
		},
		{
			name:     "invalid min",
			attrs:    rawOccurrence{MinOccurs: "x", HasMinOccurs: true},
			wantCode: xsderrors.CodeSchemaOccurrence,
			wantText: `invalid minOccurs "x"`,
		},
		{
			name:     "invalid max",
			attrs:    rawOccurrence{MaxOccurs: "1.5", HasMaxOccurs: true},
			wantCode: xsderrors.CodeSchemaOccurrence,
			wantText: "invalid maxOccurs " + diagnostictext.Quote("1.5"),
		},
		{
			name:     "max less than min",
			attrs:    rawOccurrence{MinOccurs: "2", MaxOccurs: "1", HasMinOccurs: true, HasMaxOccurs: true},
			wantCode: xsderrors.CodeSchemaOccurrence,
			wantText: "maxOccurs is less than minOccurs",
		},
		{
			name:     "min over uint32",
			attrs:    rawOccurrence{MinOccurs: "4294967296", HasMinOccurs: true},
			wantCode: xsderrors.CodeSchemaLimit,
			wantText: "minOccurs exceeds uint32 limit",
		},
		{
			name:     "max over uint32",
			attrs:    rawOccurrence{MaxOccurs: "4294967296", HasMaxOccurs: true},
			wantCode: xsderrors.CodeSchemaLimit,
			wantText: "maxOccurs exceeds uint32 limit",
		},
		{
			name:     "max over configured limit",
			attrs:    rawOccurrence{MaxOccurs: "11", HasMaxOccurs: true},
			limits:   Limits{MaxFiniteOccurs: 10},
			wantCode: xsderrors.CodeSchemaLimit,
			wantText: "maxOccurs exceeds configured limit",
		},
		{
			name:   "max equals configured limit",
			attrs:  rawOccurrence{MaxOccurs: "10", HasMaxOccurs: true},
			limits: Limits{MaxFiniteOccurs: 10},
			want:   occurrence{Min: 1, Max: 10},
		},
		{
			name:  "max uint32 accepted",
			attrs: rawOccurrence{MaxOccurs: "4294967295", HasMaxOccurs: true},
			want:  occurrence{Min: 1, Max: ^uint32(0)},
		},
		{name: "long zero prefixes", attrs: rawOccurrence{MinOccurs: strings.Repeat("0", 1024) + "2", MaxOccurs: strings.Repeat("0", 1024) + "3", HasMinOccurs: true, HasMaxOccurs: true}, want: occurrence{Min: 2, Max: 3}},
		{name: "long all zero", attrs: rawOccurrence{MinOccurs: strings.Repeat("0", 1024), MaxOccurs: strings.Repeat("0", 1024), HasMinOccurs: true, HasMaxOccurs: true}},
		{name: "negative zero min", attrs: rawOccurrence{MinOccurs: "-0", HasMinOccurs: true}, want: occurrence{Max: 1}},
		{name: "negative zero max below default min", attrs: rawOccurrence{MaxOccurs: "-0", HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "maxOccurs is less than minOccurs"},
		{name: "negative zero bounds", attrs: rawOccurrence{MinOccurs: "-0", MaxOccurs: "-0", HasMinOccurs: true, HasMaxOccurs: true}},
		{name: "long negative zero with whitespace", attrs: rawOccurrence{MinOccurs: "\t-" + strings.Repeat("0", 1024) + "\n", MaxOccurs: "\r-" + strings.Repeat("0", 1024) + " ", HasMinOccurs: true, HasMaxOccurs: true}},
		{name: "negative min", attrs: rawOccurrence{MinOccurs: "-1", HasMinOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid minOccurs " + diagnostictext.Quote("-1")},
		{name: "negative max", attrs: rawOccurrence{MinOccurs: "0", MaxOccurs: "-1", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid maxOccurs " + diagnostictext.Quote("-1")},
		{name: "negative zero malformed suffix", attrs: rawOccurrence{MinOccurs: "-0x", HasMinOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid minOccurs " + diagnostictext.Quote("-0x")},
		{name: "negative zero decimal", attrs: rawOccurrence{MinOccurs: "0", MaxOccurs: "-0.0", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid maxOccurs " + diagnostictext.Quote("-0.0")},
		{name: "sign only", attrs: rawOccurrence{MinOccurs: "-", HasMinOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid minOccurs " + diagnostictext.Quote("-")},
		{name: "huge magnitude", attrs: rawOccurrence{MaxOccurs: strings.Repeat("9", 256), HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaLimit, wantText: "maxOccurs exceeds uint32 limit"},
		{name: "overflow malformed tail", attrs: rawOccurrence{MaxOccurs: strings.Repeat("9", 256) + "x", HasMaxOccurs: true}, limits: Limits{MaxFiniteOccurs: 10}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid maxOccurs " + diagnostictext.Quote(strings.Repeat("9", 256)+"x")},
		{name: "minimum lexical error first", attrs: rawOccurrence{MinOccurs: "x", MaxOccurs: "y", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "invalid minOccurs " + diagnostictext.Quote("x")},
		{name: "minimum limit error first", attrs: rawOccurrence{MinOccurs: "4294967296", MaxOccurs: "y", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaLimit, wantText: "minOccurs exceeds uint32 limit"},
		{name: "configured cap before ordering", attrs: rawOccurrence{MinOccurs: "12", MaxOccurs: "11", HasMinOccurs: true, HasMaxOccurs: true}, limits: Limits{MaxFiniteOccurs: 10}, wantCode: xsderrors.CodeSchemaLimit, wantText: "maxOccurs exceeds configured limit"},
		{name: "uint32 cap with large configured cap", attrs: rawOccurrence{MaxOccurs: "4294967296", HasMaxOccurs: true}, limits: Limits{MaxFiniteOccurs: ^uint64(0)}, wantCode: xsderrors.CodeSchemaLimit, wantText: "maxOccurs exceeds uint32 limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseOccurrence(tt.attrs, tt.limits)
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
		want     occurrence
		wantText string
	}{
		{
			name:    "finite",
			maximum: maxOccurrence{value: 3, kind: maxOccurrenceFinite},
			want:    occurrence{Min: 2, Max: 3},
		},
		{
			name:    "unbounded",
			maximum: maxOccurrence{kind: maxOccurrenceUnbounded},
			want:    occurrence{Min: 2, Unbounded: true},
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
		occurs   occurrence
		wantCode xsderrors.Code
	}{
		{name: "required", occurs: occurrence{Min: 1, Max: 1}},
		{name: "optional", occurs: occurrence{Min: 0, Max: 1}},
		{name: "zero max", occurs: occurrence{}, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "min greater than one", occurs: occurrence{Min: 2, Max: 2}, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "max greater than one", occurs: occurrence{Min: 0, Max: 2}, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "unbounded", occurs: occurrence{Min: 0, Unbounded: true}, wantCode: xsderrors.CodeSchemaOccurrence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAllModelOccurrence(tt.occurs)
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
		want           occurrence
		message        string
	}{
		{"Short", "+2", "+4", occurrence{Min: 2, Max: 4}, ""},
		{"LongZeros", "+" + zeros + "1", "+" + zeros + "4", occurrence{Min: 1, Max: 4}, ""},
		{"AllZero", zeros, zeros, occurrence{}, ""},
		{"OverflowMalformed", "1", "42949672960x", occurrence{}, `invalid maxOccurs "42949672960x"`},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			attrs := rawOccurrence{MinOccurs: c.min, MaxOccurs: c.max, HasMinOccurs: true, HasMaxOccurs: true}
			var last occurrence
			pre, err := parseOccurrence(attrs, Limits{})
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
				last, err = parseOccurrence(attrs, Limits{})
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

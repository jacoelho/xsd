package compile

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseOccurrence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		attrs    OccurrenceAttrs
		limits   Limits
		want     runtime.Occurrence
		wantCode xsderrors.Code
		wantText string
	}{
		{
			name: "defaults",
			want: runtime.Occurrence{Min: 1, Max: 1},
		},
		{
			name:  "trims XML whitespace and plus",
			attrs: OccurrenceAttrs{MinOccurs: "\t+0002\n", MaxOccurs: "\r0003 ", HasMinOccurs: true, HasMaxOccurs: true},
			want:  runtime.Occurrence{Min: 2, Max: 3},
		},
		{
			name:  "zero",
			attrs: OccurrenceAttrs{MinOccurs: "0", MaxOccurs: "0", HasMinOccurs: true, HasMaxOccurs: true},
			want:  runtime.Occurrence{},
		},
		{
			name:  "unbounded",
			attrs: OccurrenceAttrs{MinOccurs: "2", MaxOccurs: " unbounded\t", HasMinOccurs: true, HasMaxOccurs: true},
			want:  runtime.Occurrence{Min: 2, Unbounded: true},
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
			want:   runtime.Occurrence{Min: 1, Max: 10},
		},
		{
			name:  "max uint32 accepted",
			attrs: OccurrenceAttrs{MaxOccurs: "4294967295", HasMaxOccurs: true},
			want:  runtime.Occurrence{Min: 1, Max: ^uint32(0)},
		},
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

func TestValidateAllModelOccurrence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		occurs   runtime.Occurrence
		wantCode xsderrors.Code
	}{
		{name: "required", occurs: runtime.Occurrence{Min: 1, Max: 1}},
		{name: "optional", occurs: runtime.Occurrence{Min: 0, Max: 1}},
		{name: "zero max", occurs: runtime.Occurrence{}, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "min greater than one", occurs: runtime.Occurrence{Min: 2, Max: 2}, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "max greater than one", occurs: runtime.Occurrence{Min: 0, Max: 2}, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "unbounded", occurs: runtime.Occurrence{Min: 0, Unbounded: true}, wantCode: xsderrors.CodeSchemaOccurrence},
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

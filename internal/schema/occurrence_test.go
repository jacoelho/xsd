package schema

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// occurrenceTag is the minOccurs and maxOccurs of one start tag, written in
// that order.
type occurrenceTag struct {
	MinOccurs    string
	MaxOccurs    string
	HasMinOccurs bool
	HasMaxOccurs bool
}

func admitOccurrenceTag(element, parent string, tag occurrenceTag, limits Limits) (occurrence, error) {
	admission := newOccurrenceAdmission(xml.Name{Space: vocab.XSDNamespaceURI, Local: element}, xml.Name{Space: vocab.XSDNamespaceURI, Local: parent}, limits.MaxFiniteOccurs)
	if tag.HasMinOccurs {
		if err := admission.admitAttribute(vocab.XSDAttrMinOccurs, tag.MinOccurs); err != nil {
			return occurrence{}, err
		}
	}
	if tag.HasMaxOccurs {
		if err := admission.admitAttribute(vocab.XSDAttrMaxOccurs, tag.MaxOccurs); err != nil {
			return occurrence{}, err
		}
	}
	return admission.complete()
}

func TestOccurrenceAdmissionOfParticle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		attrs    occurrenceTag
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
			attrs: occurrenceTag{MinOccurs: "\t+0002\n", MaxOccurs: "\r0003 ", HasMinOccurs: true, HasMaxOccurs: true},
			want:  occurrence{Min: 2, Max: 3},
		},
		{
			name:  "zero",
			attrs: occurrenceTag{MinOccurs: "0", MaxOccurs: "0", HasMinOccurs: true, HasMaxOccurs: true},
			want:  occurrence{},
		},
		{
			name:  "unbounded",
			attrs: occurrenceTag{MinOccurs: "2", MaxOccurs: " unbounded\t", HasMinOccurs: true, HasMaxOccurs: true},
			want:  occurrence{Min: 2, Unbounded: true},
		},
		{
			name:     "max less than min",
			attrs:    occurrenceTag{MinOccurs: "2", MaxOccurs: "1", HasMinOccurs: true, HasMaxOccurs: true},
			wantCode: xsderrors.CodeSchemaOccurrence,
			wantText: "maxOccurs is less than minOccurs",
		},
		{
			name:     "min over uint32",
			attrs:    occurrenceTag{MinOccurs: "4294967296", HasMinOccurs: true},
			wantCode: xsderrors.CodeSchemaLimit,
			wantText: "minOccurs exceeds uint32 limit",
		},
		{
			name:     "max over uint32",
			attrs:    occurrenceTag{MaxOccurs: "4294967296", HasMaxOccurs: true},
			wantCode: xsderrors.CodeSchemaLimit,
			wantText: "maxOccurs exceeds uint32 limit",
		},
		{
			name:     "max over configured limit",
			attrs:    occurrenceTag{MaxOccurs: "11", HasMaxOccurs: true},
			limits:   Limits{MaxFiniteOccurs: 10},
			wantCode: xsderrors.CodeSchemaLimit,
			wantText: "maxOccurs exceeds configured limit",
		},
		{
			name:   "max equals configured limit",
			attrs:  occurrenceTag{MaxOccurs: "10", HasMaxOccurs: true},
			limits: Limits{MaxFiniteOccurs: 10},
			want:   occurrence{Min: 1, Max: 10},
		},
		{
			name:  "max uint32 accepted",
			attrs: occurrenceTag{MaxOccurs: "4294967295", HasMaxOccurs: true},
			want:  occurrence{Min: 1, Max: ^uint32(0)},
		},
		{name: "long zero prefixes", attrs: occurrenceTag{MinOccurs: strings.Repeat("0", 1024) + "2", MaxOccurs: strings.Repeat("0", 1024) + "3", HasMinOccurs: true, HasMaxOccurs: true}, want: occurrence{Min: 2, Max: 3}},
		{name: "long all zero", attrs: occurrenceTag{MinOccurs: strings.Repeat("0", 1024), MaxOccurs: strings.Repeat("0", 1024), HasMinOccurs: true, HasMaxOccurs: true}},
		{name: "negative zero min", attrs: occurrenceTag{MinOccurs: "-0", HasMinOccurs: true}, want: occurrence{Max: 1}},
		{name: "negative zero max below default min", attrs: occurrenceTag{MaxOccurs: "-0", HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "maxOccurs is less than minOccurs"},
		{name: "negative zero bounds", attrs: occurrenceTag{MinOccurs: "-0", MaxOccurs: "-0", HasMinOccurs: true, HasMaxOccurs: true}},
		{name: "long negative zero with whitespace", attrs: occurrenceTag{MinOccurs: "\t-" + strings.Repeat("0", 1024) + "\n", MaxOccurs: "\r-" + strings.Repeat("0", 1024) + " ", HasMinOccurs: true, HasMaxOccurs: true}},
		{name: "huge magnitude", attrs: occurrenceTag{MaxOccurs: strings.Repeat("9", 256), HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaLimit, wantText: "maxOccurs exceeds uint32 limit"},
		{name: "minimum limit error first", attrs: occurrenceTag{MinOccurs: "4294967296", MaxOccurs: "4294967297", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaLimit, wantText: "minOccurs exceeds uint32 limit"},
		{name: "configured cap before ordering", attrs: occurrenceTag{MinOccurs: "12", MaxOccurs: "11", HasMinOccurs: true, HasMaxOccurs: true}, limits: Limits{MaxFiniteOccurs: 10}, wantCode: xsderrors.CodeSchemaLimit, wantText: "maxOccurs exceeds configured limit"},
		{name: "uint32 cap with large configured cap", attrs: occurrenceTag{MaxOccurs: "4294967296", HasMaxOccurs: true}, limits: Limits{MaxFiniteOccurs: ^uint64(0)}, wantCode: xsderrors.CodeSchemaLimit, wantText: "maxOccurs exceeds uint32 limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := admitOccurrenceTag(vocab.XSDElemElement, vocab.XSDElemSequence, tt.attrs, tt.limits)
			if tt.wantCode != "" {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, tt.wantCode)
				if !strings.Contains(err.Error(), tt.wantText) {
					t.Fatalf("occurrence admission error = %v, want text %q", err, tt.wantText)
				}
				return
			}
			if err != nil {
				t.Fatalf("occurrence admission error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("occurrence admission = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestOccurrenceAdmissionOfAllGroupAndMember(t *testing.T) {
	t.Parallel()

	const (
		groupText  = "xs:all occurrence must be zero or one"
		memberText = "xs:all particles cannot repeat"
	)
	tests := []struct {
		name     string
		element  string
		parent   string
		attrs    occurrenceTag
		limits   Limits
		want     occurrence
		wantCode xsderrors.Code
		wantText string
	}{
		{name: "all defaults", element: "all", parent: "complexType", want: occurrence{Min: 1, Max: 1}},
		{name: "all optional", element: "all", parent: "complexType", attrs: occurrenceTag{MinOccurs: "0", HasMinOccurs: true}, want: occurrence{Max: 1}},
		{name: "all minOccurs two", element: "all", parent: "complexType", attrs: occurrenceTag{MinOccurs: "2", HasMinOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: groupText},
		{name: "all maxOccurs zero", element: "all", parent: "complexType", attrs: occurrenceTag{MinOccurs: "0", MaxOccurs: "0", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: groupText},
		{name: "all maxOccurs unbounded", element: "all", parent: "complexType", attrs: occurrenceTag{MaxOccurs: "unbounded", HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: groupText},
		{name: "all minOccurs above uint32 is a limit", element: "all", parent: "complexType", attrs: occurrenceTag{MinOccurs: "4294967296", HasMinOccurs: true}, wantCode: xsderrors.CodeSchemaLimit, wantText: "minOccurs exceeds uint32 limit"},
		{name: "all maxOccurs above configured limit is a limit", element: "all", parent: "complexType", attrs: occurrenceTag{MaxOccurs: "3", HasMaxOccurs: true}, limits: Limits{MaxFiniteOccurs: 2}, wantCode: xsderrors.CodeSchemaLimit, wantText: "maxOccurs exceeds configured limit"},
		{name: "all minOccurs range before maxOccurs limit", element: "all", parent: "complexType", attrs: occurrenceTag{MinOccurs: "2", MaxOccurs: "4294967296", HasMinOccurs: true, HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: groupText},
		{name: "member optional", element: "element", parent: "all", attrs: occurrenceTag{MinOccurs: "0", MaxOccurs: "1", HasMinOccurs: true, HasMaxOccurs: true}, want: occurrence{Max: 1}},
		{name: "member absent", element: "element", parent: "all", attrs: occurrenceTag{MinOccurs: "0", MaxOccurs: "0", HasMinOccurs: true, HasMaxOccurs: true}, want: occurrence{}},
		{name: "member maxOccurs two", element: "element", parent: "all", attrs: occurrenceTag{MaxOccurs: "2", HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: memberText},
		{name: "member maxOccurs unbounded", element: "element", parent: "all", attrs: occurrenceTag{MaxOccurs: "unbounded", HasMaxOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: memberText},
		{name: "member minOccurs two is a relation", element: "element", parent: "all", attrs: occurrenceTag{MinOccurs: "2", HasMinOccurs: true}, wantCode: xsderrors.CodeSchemaOccurrence, wantText: "maxOccurs is less than minOccurs"},
		{name: "element of a sequence repeats", element: "element", parent: "sequence", attrs: occurrenceTag{MaxOccurs: "2", HasMaxOccurs: true}, want: occurrence{Min: 1, Max: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := admitOccurrenceTag(tt.element, tt.parent, tt.attrs, tt.limits)
			if tt.wantCode != "" {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, tt.wantCode)
				if !strings.Contains(err.Error(), tt.wantText) {
					t.Fatalf("occurrence admission error = %v, want text %q", err, tt.wantText)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("occurrence admission = %+v, %v, want %+v", got, err, tt.want)
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

func TestOccurrenceLexicalForm(t *testing.T) {
	t.Parallel()

	overflow := strings.Repeat("9", 256)
	tests := []struct {
		name          string
		attr          string
		value         string
		wantCount     uint64
		wantUnbounded bool
		wantInvalid   bool
	}{
		{name: "minOccurs count", attr: "minOccurs", value: "\t+0002\n", wantCount: 2},
		{name: "minOccurs negative zero", attr: "minOccurs", value: "-0"},
		{name: "minOccurs word", attr: "minOccurs", value: "x", wantInvalid: true},
		{name: "minOccurs unbounded", attr: "minOccurs", value: "unbounded", wantInvalid: true},
		{name: "minOccurs negative", attr: "minOccurs", value: "-1", wantInvalid: true},
		{name: "minOccurs negative zero with suffix", attr: "minOccurs", value: "-0x", wantInvalid: true},
		{name: "minOccurs sign only", attr: "minOccurs", value: "-", wantInvalid: true},
		{name: "minOccurs empty", attr: "minOccurs", value: "", wantInvalid: true},
		{name: "maxOccurs count", attr: "maxOccurs", value: "\r0003 ", wantCount: 3},
		{name: "maxOccurs unbounded", attr: "maxOccurs", value: " unbounded\t", wantUnbounded: true},
		{name: "maxOccurs decimal", attr: "maxOccurs", value: "1.5", wantInvalid: true},
		{name: "maxOccurs negative", attr: "maxOccurs", value: "-1", wantInvalid: true},
		{name: "maxOccurs negative zero decimal", attr: "maxOccurs", value: "-0.0", wantInvalid: true},
		{name: "maxOccurs overflow with malformed tail", attr: "maxOccurs", value: overflow + "x", wantInvalid: true},
		{name: "maxOccurs capitalised unbounded", attr: "maxOccurs", value: "Unbounded", wantInvalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got maxOccursLexical
			var err error
			if tt.attr == "minOccurs" {
				got.count, err = parseMinOccursLexical(tt.value)
			} else {
				got, err = parseMaxOccursLexical(tt.value)
			}
			if tt.wantInvalid {
				assertOccurrenceDiagnostic(t, err, "invalid "+tt.attr+" "+diagnostictext.Quote(tt.value))
				return
			}
			if err != nil {
				t.Fatalf("%s %q error = %v", tt.attr, tt.value, err)
			}
			if want := (maxOccursLexical{count: tt.wantCount, unbounded: tt.wantUnbounded}); got != want {
				t.Fatalf("%s %q = %+v, want %+v", tt.attr, tt.value, got, want)
			}
		})
	}
}

func TestOccurrenceOverflowIsLexicallyValid(t *testing.T) {
	t.Parallel()

	overflow := strings.Repeat("9", 256)
	minimum, err := parseMinOccursLexical(overflow)
	if err != nil || minimum <= maxUint32Value {
		t.Fatalf("minOccurs overflow = %d, %v, want a count above uint32", minimum, err)
	}
	maximum, err := parseMaxOccursLexical(overflow)
	if err != nil || maximum.unbounded || maximum.count <= maxUint32Value {
		t.Fatalf("maxOccurs overflow = %+v, %v, want a count above uint32", maximum, err)
	}
}

func assertOccurrenceDiagnostic(tb testing.TB, err error, message string) {
	tb.Helper()
	d, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || d.Category() != xsderrors.CategorySchemaCompile || d.Code() != xsderrors.CodeSchemaOccurrence || d.Message() != message {
		tb.Fatalf("diagnostic = %v, want schema.occurrence %q", err, message)
	}
}

func BenchmarkOccurrenceAdmission(b *testing.B) {
	zeros := strings.Repeat("0", 4096)
	cases := []struct {
		name, min, max string
		want           occurrence
	}{
		{"Short", "+2", "+4", occurrence{Min: 2, Max: 4}},
		{"LongZeros", "+" + zeros + "1", "+" + zeros + "4", occurrence{Min: 1, Max: 4}},
		{"AllZero", zeros, zeros, occurrence{}},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			attrs := occurrenceTag{MinOccurs: c.min, MaxOccurs: c.max, HasMinOccurs: true, HasMaxOccurs: true}
			var last occurrence
			pre, err := admitOccurrenceTag(vocab.XSDElemElement, vocab.XSDElemSequence, attrs, Limits{})
			if err != nil || pre != c.want {
				b.Fatalf("preflight %v %v", pre, err)
			}
			b.ReportAllocs()
			for b.Loop() {
				var err error
				last, err = admitOccurrenceTag(vocab.XSDElemElement, vocab.XSDElemSequence, attrs, Limits{})
				if err != nil {
					b.Fatal(err)
				}
			}
			if last != c.want {
				b.Fatalf("postflight %v", last)
			}
		})
	}
	b.Run("OverflowMalformed", func(b *testing.B) {
		const value = "42949672960x"
		const message = `invalid maxOccurs "42949672960x"`
		_, err := parseMaxOccursLexical(value)
		assertOccurrenceDiagnostic(b, err, message)
		b.ReportAllocs()
		for b.Loop() {
			_, err = parseMaxOccursLexical(value)
			if err == nil {
				b.Fatal("malformed maxOccurs was accepted")
			}
		}
		assertOccurrenceDiagnostic(b, err, message)
	})
}

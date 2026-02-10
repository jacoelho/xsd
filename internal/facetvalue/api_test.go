package facetvalue

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
)

type testFacet struct {
	name       string
	validateFn func(value model.TypedValue, baseType model.Type) error
}

func (f testFacet) Name() string {
	return f.name
}

func (f testFacet) Validate(value model.TypedValue, baseType model.Type) error {
	if f.validateFn == nil {
		return nil
	}
	return f.validateFn(value, baseType)
}

type testLexicalFacet struct {
	testFacet
	validateLexicalFn func(lexical string, baseType model.Type) error
}

func (f testLexicalFacet) ValidateLexical(lexical string, baseType model.Type) error {
	if f.validateLexicalFn == nil {
		return nil
	}
	return f.validateLexicalFn(lexical, baseType)
}

func TestValuesEqual_DecimalAndInteger(t *testing.T) {
	t.Parallel()

	decimalType, err := builtins.NewSimpleType(builtins.TypeNameDecimal)
	if err != nil {
		t.Fatalf("NewSimpleType(decimal) error = %v", err)
	}
	integerType, err := builtins.NewSimpleType(builtins.TypeNameInteger)
	if err != nil {
		t.Fatalf("NewSimpleType(integer) error = %v", err)
	}

	left, err := decimalType.ParseValue("1.0")
	if err != nil {
		t.Fatalf("ParseValue(left) error = %v", err)
	}
	right, err := integerType.ParseValue("1")
	if err != nil {
		t.Fatalf("ParseValue(right) error = %v", err)
	}

	if !ValuesEqual(left, right) {
		t.Fatal("ValuesEqual() = false, want true")
	}
}

func TestValuesEqual_FloatNaN(t *testing.T) {
	t.Parallel()

	floatType, err := builtins.NewSimpleType(builtins.TypeNameFloat)
	if err != nil {
		t.Fatalf("NewSimpleType(float) error = %v", err)
	}

	left, err := floatType.ParseValue("NaN")
	if err != nil {
		t.Fatalf("ParseValue(left) error = %v", err)
	}
	right, err := floatType.ParseValue("NaN")
	if err != nil {
		t.Fatalf("ParseValue(right) error = %v", err)
	}

	if !ValuesEqual(left, right) {
		t.Fatal("ValuesEqual(NaN, NaN) = false, want true")
	}
}

func TestTypedValueForFacet_FallbackStringValue(t *testing.T) {
	t.Parallel()

	baseType := builtins.Get(builtins.TypeNameInteger)
	if baseType == nil {
		t.Fatal("missing builtin integer type")
	}

	got := TypedValueForFacet("not-an-integer", baseType)
	if got == nil {
		t.Fatal("TypedValueForFacet() returned nil")
	}
	if _, ok := got.(*model.StringTypedValue); !ok {
		t.Fatalf("TypedValueForFacet() type = %T, want *model.StringTypedValue", got)
	}
}

func TestValidateApplicability_ListRangeFacetRejected(t *testing.T) {
	t.Parallel()

	listType := builtins.Get(builtins.TypeNameNMTOKENS)
	if listType == nil {
		t.Fatal("missing builtin NMTOKENS type")
	}

	err := ValidateApplicability("minInclusive", listType, listType.Name())
	if err == nil {
		t.Fatal("expected list-type applicability error")
	}
	if !strings.Contains(err.Error(), "not applicable to list type") {
		t.Fatalf("error = %q, want substring %q", err.Error(), "not applicable to list type")
	}
}

func TestParseDurationToTimeDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr string
	}{
		{
			name:  "day hour minute and fractional seconds",
			input: "P1DT2H3M4.5S",
			want:  26*time.Hour + 3*time.Minute + 4500*time.Millisecond,
		},
		{
			name:  "negative duration",
			input: "-PT1S",
			want:  -1 * time.Second,
		},
		{
			name:    "month-based duration rejected",
			input:   "P1M",
			wantErr: "years or months",
		},
		{
			name:    "second overflow rejected",
			input:   "PT999999999999999999999S",
			wantErr: "second value too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDurationToTimeDuration(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseDurationToTimeDuration(%q) expected error, got nil", tt.input)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseDurationToTimeDuration(%q) error = %v, want substring %q", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDurationToTimeDuration(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseDurationToTimeDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewMinInclusive_UnresolvedPrimitiveReturnsDeferredSentinel(t *testing.T) {
	t.Parallel()

	unknown := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "Unknown"},
		Restriction: &model.Restriction{
			Base: model.QName{Namespace: "urn:external", Local: "ExternalBase"},
		},
	}

	_, err := NewMinInclusive("1", unknown)
	if err == nil {
		t.Fatal("expected unresolved primitive error")
	}
	if !errors.Is(err, model.ErrCannotDeterminePrimitiveType) {
		t.Fatalf("error = %v, want errors.Is(..., model.ErrCannotDeterminePrimitiveType)", err)
	}
}

func TestNewMinInclusive_ReturnsModelRangeFacet(t *testing.T) {
	t.Parallel()

	baseType := builtins.Get(builtins.TypeNameInteger)
	if baseType == nil {
		t.Fatal("missing builtin integer type")
	}

	facet, err := NewMinInclusive("1", baseType)
	if err != nil {
		t.Fatalf("NewMinInclusive() error = %v", err)
	}
	if facet.Name() != "minInclusive" {
		t.Fatalf("facet.Name() = %q, want %q", facet.Name(), "minInclusive")
	}
	if _, ok := facet.(model.LexicalFacet); !ok {
		t.Fatalf("facet type = %T, want model.LexicalFacet", facet)
	}
	if _, ok := facet.(*model.RangeFacet); !ok {
		t.Fatalf("facet type = %T, want *model.RangeFacet", facet)
	}
}

func TestApplyStopsOnFirstError(t *testing.T) {
	t.Parallel()

	calls := 0
	wantErr := errors.New("boom")
	facets := []model.Facet{
		testFacet{
			name: "ok",
			validateFn: func(model.TypedValue, model.Type) error {
				calls++
				return nil
			},
		},
		testFacet{
			name: "bad",
			validateFn: func(model.TypedValue, model.Type) error {
				calls++
				return wantErr
			},
		},
		testFacet{
			name: "later",
			validateFn: func(model.TypedValue, model.Type) error {
				calls++
				return nil
			},
		},
	}

	err := Apply(nil, facets, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Apply() error = %v, want %v", err, wantErr)
	}
	if calls != 2 {
		t.Fatalf("Apply() calls = %d, want 2", calls)
	}
}

func TestValidateWrapsLexicalErrors(t *testing.T) {
	t.Parallel()

	typedValidated := false
	facets := []model.Facet{
		testLexicalFacet{
			testFacet: testFacet{
				name: "lex",
				validateFn: func(model.TypedValue, model.Type) error {
					typedValidated = true
					return nil
				},
			},
			validateLexicalFn: func(string, model.Type) error {
				return errors.New("bad lexical")
			},
		},
	}

	err := Validate("v", nil, facets, nil)
	if err == nil {
		t.Fatal("expected lexical validation error")
	}
	if !strings.Contains(err.Error(), "facet 'lex' violation: bad lexical") {
		t.Fatalf("error = %v", err)
	}
	if typedValidated {
		t.Fatal("typed validation should not run for lexical-only errors")
	}
}

func TestValidateBuildsTypedValueOnce(t *testing.T) {
	t.Parallel()

	baseType := builtins.Get(builtins.TypeNameInteger)
	if baseType == nil {
		t.Fatal("missing builtin integer type")
	}

	var firstTyped model.TypedValue
	facets := []model.Facet{
		testFacet{
			name: "first",
			validateFn: func(value model.TypedValue, _ model.Type) error {
				firstTyped = value
				return nil
			},
		},
		testFacet{
			name: "second",
			validateFn: func(value model.TypedValue, _ model.Type) error {
				if firstTyped == nil {
					return errors.New("first facet received nil typed value")
				}
				if value != firstTyped {
					return errors.New("typed value created more than once")
				}
				return nil
			},
		},
	}

	if err := Validate("not-an-integer", baseType, facets, nil); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if firstTyped == nil {
		t.Fatal("first facet did not receive typed value")
	}
}

func TestValidateKeepsQNameEnumerationErrorsUnwrapped(t *testing.T) {
	t.Parallel()

	qnameType := builtins.Get(builtins.TypeNameQName)
	if qnameType == nil {
		t.Fatal("missing builtin QName type")
	}

	enumFacet := model.NewEnumeration([]string{"ns:allowed"})
	err := Validate("ns:value", qnameType, []model.Facet{enumFacet}, nil)
	if err == nil {
		t.Fatal("expected QName enumeration error")
	}
	if !strings.Contains(err.Error(), "namespace context unavailable for QName/NOTATION enumeration") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "facet 'enumeration' violation") {
		t.Fatalf("QName enumeration error was unexpectedly wrapped: %v", err)
	}
}

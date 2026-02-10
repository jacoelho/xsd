package typefacet

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
)

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

	got, err := ParseDurationToTimeDuration("P1DT2H3M4.5S")
	if err != nil {
		t.Fatalf("ParseDurationToTimeDuration() error = %v", err)
	}
	want := 26*time.Hour + 3*time.Minute + 4500*time.Millisecond
	if got != want {
		t.Fatalf("ParseDurationToTimeDuration() = %v, want %v", got, want)
	}

	if _, err := ParseDurationToTimeDuration("P1M"); err == nil {
		t.Fatal("expected error for month-based duration")
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

func TestNewMinInclusive_ReturnsTypeFacetOwnedRangeFacet(t *testing.T) {
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
	if _, ok := facet.(*model.RangeFacet); ok {
		t.Fatalf("facet type = %T, want typefacet-owned implementation", facet)
	}
}

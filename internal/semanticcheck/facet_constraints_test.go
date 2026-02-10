package semanticcheck

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	facetengine "github.com/jacoelho/xsd/internal/schemafacet"
)

func TestCompareGYearValues(t *testing.T) {
	bt := builtins.Get(model.TypeNameGYear)
	if bt == nil {
		t.Fatal("builtin.Get(\"gYear\") returned nil")
	}

	result, err := facetengine.CompareFacetValues("2002", "1998", bt)
	if err != nil || result != 1 {
		t.Errorf("facetengine.CompareFacetValues(\"2002\", \"1998\", gYear) = %d, %v; want 1, nil", result, err)
	}
}

func TestGYearTimezoneEquivalence(t *testing.T) {
	// Test that "2000Z" and "2000+00:00" are considered equal in value space
	bt := builtins.Get(model.TypeNameGYear)
	if bt == nil {
		t.Fatal("builtin.Get(\"gYear\") returned nil")
	}

	result, err := facetengine.CompareFacetValues("2000Z", "2000+00:00", bt)
	if err != nil {
		t.Fatalf("compareFacetValues failed: %v", err)
	}
	if result != 0 {
		t.Errorf("expected 0 (equal), got %d", result)
	}

	// Test reverse order
	result, err = facetengine.CompareFacetValues("2000+00:00", "2000Z", bt)
	if err != nil {
		t.Fatalf("compareFacetValues failed: %v", err)
	}
	if result != 0 {
		t.Errorf("expected 0 (equal), got %d", result)
	}
}

func TestValidateRangeFacetsGYear(t *testing.T) {
	minInclusive := "2002"
	maxInclusive := "1998"
	bt := builtins.Get(model.TypeNameGYear)

	err := facetengine.ValidateRangeConsistency(nil, nil, &minInclusive, &maxInclusive, bt, bt.Name())
	if err == nil {
		t.Error("validateRangeFacets should return error for minInclusive > maxInclusive")
	}
}

func TestValidateRangeFacetsDateTimeTimezoneDefiniteOrder(t *testing.T) {
	minInclusive := "2000-01-01T00:00:00Z"
	maxInclusive := "1999-12-31T00:00:00"
	bt := builtins.Get(model.TypeNameDateTime)

	err := facetengine.ValidateRangeConsistency(nil, nil, &minInclusive, &maxInclusive, bt, bt.Name())
	if err == nil {
		t.Fatal("expected timezone mismatch comparison to detect minInclusive > maxInclusive")
	}
}

func TestValidateRangeFacetsDateTimeTimezoneIndeterminate(t *testing.T) {
	minInclusive := "2000-01-01T12:00:00Z"
	maxInclusive := "2000-01-01T12:00:00"
	bt := builtins.Get(model.TypeNameDateTime)

	err := facetengine.ValidateRangeConsistency(nil, nil, &minInclusive, &maxInclusive, bt, bt.Name())
	if err != nil {
		t.Fatalf("expected indeterminate timezone comparison to be ignored, got %v", err)
	}
}

func TestValidateRangeFacetsLargeIntegerPrecision(t *testing.T) {
	minInclusive := "9007199254740993"
	maxInclusive := "9007199254740992"
	bt := builtins.Get(model.TypeNameInteger)

	err := facetengine.ValidateRangeConsistency(nil, nil, &minInclusive, &maxInclusive, bt, bt.Name())
	if err == nil {
		t.Fatal("expected range facet comparison to detect minInclusive > maxInclusive")
	}
}

func TestValidateRangeFacetsDecimalPrecision(t *testing.T) {
	minInclusive := "0.1234567890123456789012345678901"
	maxInclusive := "0.1234567890123456789012345678900"
	bt := builtins.Get(model.TypeNameDecimal)

	err := facetengine.ValidateRangeConsistency(nil, nil, &minInclusive, &maxInclusive, bt, bt.Name())
	if err == nil {
		t.Fatal("expected range facet comparison to detect minInclusive > maxInclusive")
	}
}

func TestValidateRangeFacetsFloatNaNNotComparable(t *testing.T) {
	minInclusive := "NaN"
	maxInclusive := "1.0"
	bt := builtins.Get(model.TypeNameFloat)

	err := facetengine.ValidateRangeConsistency(nil, nil, &minInclusive, &maxInclusive, bt, bt.Name())
	if err != nil {
		t.Fatalf("expected NaN range comparison to be ignored, got %v", err)
	}
}

func TestCompareFloatFacetValuesNaN(t *testing.T) {
	if _, err := facetengine.CompareFloatFacetValues("NaN", "1.0"); !errors.Is(err, errFloatNotComparable) {
		t.Fatalf("expected errFloatNotComparable, got %v", err)
	}
	cmp, err := facetengine.CompareFloatFacetValues("NaN", "NaN")
	if err != nil {
		t.Fatalf("compareFloatFacetValues NaN error = %v", err)
	}
	if cmp != 0 {
		t.Fatalf("compareFloatFacetValues NaN cmp = %d, want 0", cmp)
	}
}

func TestValidateDurationRangeFacetsInvalidLexical(t *testing.T) {
	minExclusive := "P"
	maxExclusive := "P1D"

	if err := facetengine.ValidateDurationRangeConsistency(&minExclusive, &maxExclusive, nil, nil); err == nil {
		t.Fatal("expected invalid duration lexical value to return error")
	}
}

func TestValidatePatternFacetSyntax(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		valid   bool
	}{
		{
			name:    "valid pattern",
			pattern: `\d{3}-\d{3}-\d{4}`,
			valid:   true,
		},
		{
			name:    "invalid Unicode property escape",
			pattern: `\p{IsCJKSymbolsandPunctuation}?`,
			valid:   false,
		},
		{
			name:    "invalid anchor escape sequence",
			pattern: `\z`,
			valid:   false, // \z is not valid XSD 1.0 syntax (Perl anchor, not XSD)
		},
		{
			name:    "invalid unmatched bracket",
			pattern: `a[b`,
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patternFacet := &model.Pattern{Value: tt.pattern}
			baseQName := model.QName{Namespace: model.XSDNamespace, Local: "string"}

			baseType, err := builtins.NewSimpleType(model.TypeNameString)
			if err != nil {
				t.Fatalf("NewBuiltinSimpleType(string) failed: %v", err)
			}

			facetList := []model.Facet{patternFacet}
			err = ValidateFacetConstraints(nil, facetList, baseType, baseQName)
			if tt.valid && err != nil {
				t.Errorf("Pattern %q should be valid but got error: %v", tt.pattern, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("Pattern %q should be invalid but validation passed", tt.pattern)
			}
			if !tt.valid && err != nil {
				if !strings.Contains(err.Error(), "pattern") {
					t.Errorf("Error should mention 'pattern', got: %v", err)
				}
			}
		})
	}
}

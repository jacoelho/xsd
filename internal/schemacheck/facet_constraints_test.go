package schemacheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestCompareGYearValues(t *testing.T) {
	bt := types.GetBuiltin(types.TypeNameGYear)
	if bt == nil {
		t.Fatal("builtin.Get(\"gYear\") returned nil")
	}

	result, err := compareFacetValues("2002", "1998", bt)
	if err != nil || result != 1 {
		t.Errorf("compareFacetValues(\"2002\", \"1998\", gYear) = %d, %v; want 1, nil", result, err)
	}
}

func TestGYearTimezoneEquivalence(t *testing.T) {
	// Test that "2000Z" and "2000+00:00" are considered equal in value space
	bt := types.GetBuiltin(types.TypeNameGYear)
	if bt == nil {
		t.Fatal("builtin.Get(\"gYear\") returned nil")
	}

	result, err := compareFacetValues("2000Z", "2000+00:00", bt)
	if err != nil {
		t.Fatalf("compareFacetValues failed: %v", err)
	}
	if result != 0 {
		t.Errorf("expected 0 (equal), got %d", result)
	}

	// Test reverse order
	result, err = compareFacetValues("2000+00:00", "2000Z", bt)
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
	bt := types.GetBuiltin(types.TypeNameGYear)

	err := validateRangeFacets(nil, nil, &minInclusive, &maxInclusive, bt, bt.Name())
	if err == nil {
		t.Error("validateRangeFacets should return error for minInclusive > maxInclusive")
	}
}

func TestValidateRangeFacetsLargeIntegerPrecision(t *testing.T) {
	minInclusive := "9007199254740993"
	maxInclusive := "9007199254740992"
	bt := types.GetBuiltin(types.TypeNameInteger)

	err := validateRangeFacets(nil, nil, &minInclusive, &maxInclusive, bt, bt.Name())
	if err == nil {
		t.Fatal("expected range facet comparison to detect minInclusive > maxInclusive")
	}
}

func TestValidateRangeFacetsDecimalPrecision(t *testing.T) {
	minInclusive := "0.1234567890123456789012345678901"
	maxInclusive := "0.1234567890123456789012345678900"
	bt := types.GetBuiltin(types.TypeNameDecimal)

	err := validateRangeFacets(nil, nil, &minInclusive, &maxInclusive, bt, bt.Name())
	if err == nil {
		t.Fatal("expected range facet comparison to detect minInclusive > maxInclusive")
	}
}

func TestValidateDurationRangeFacetsInvalidLexical(t *testing.T) {
	minExclusive := "P"
	maxExclusive := "P1D"

	if err := validateDurationRangeFacets(&minExclusive, &maxExclusive, nil, nil); err == nil {
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
			patternFacet := &types.Pattern{Value: tt.pattern}
			baseQName := types.QName{Namespace: types.XSDNamespace, Local: "string"}

			baseType, err := types.NewBuiltinSimpleType(types.TypeNameString)
			if err != nil {
				t.Fatalf("NewBuiltinSimpleType(string) failed: %v", err)
			}

			facetList := []types.Facet{patternFacet}
			err = validateFacetConstraints(nil, facetList, baseType, baseQName)
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

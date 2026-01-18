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

	result := compareNumericOrString("2002", "1998", "gYear", bt)
	if result != 1 {
		t.Errorf("compareNumericOrString(\"2002\", \"1998\", \"gYear\", bt) = %d, want 1", result)
	}

	result = compareNumericOrString("2002", "1998", "gYear", nil)
	if result != 1 {
		t.Errorf("compareNumericOrString(\"2002\", \"1998\", \"gYear\", nil) = %d, want 1", result)
	}
}

func TestValidateRangeFacetsGYear(t *testing.T) {
	minInclusive := "2002"
	maxInclusive := "1998"
	baseTypeName := "gYear"
	bt := types.GetBuiltin(types.TypeNameGYear)

	err := validateRangeFacets(nil, nil, &minInclusive, &maxInclusive, baseTypeName, bt)
	if err == nil {
		t.Error("validateRangeFacets should return error for minInclusive > maxInclusive")
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
			err = validateFacetConstraints(facetList, baseType, baseQName)
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

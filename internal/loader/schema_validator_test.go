package loader

import (
	"os"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/types"
)

func TestGYearMinInclusive003Schema(t *testing.T) {
	// Test loading the actual schema
	testDataDir := "../../testdata/xsdtests"
	if _, err := os.Stat(testDataDir); os.IsNotExist(err) {
		t.Skip("testdata directory not found")
	}

	cfg := Config{
		FS: os.DirFS(testDataDir),
	}
	l := NewLoader(cfg)
	schema, err := l.Load("msData/datatypes/Facets/Schemas/gYear_minInclusive003.xsd")
	if err != nil {
		// This should fail because the schema is invalid
		t.Logf("Schema loading failed as expected: %v", err)
		return
	}

	// If it loaded successfully, that's the bug - it should have failed
	t.Errorf("Schema loaded successfully but should have failed validation")

	// Let's check what we got
	if schema != nil {
		t.Logf("Schema loaded with %d type definitions", len(schema.TypeDefs))
	}
}

func TestCompareGYearValues(t *testing.T) {
	// Test that "2002" > "1998" returns 1 for gYear
	bt := types.GetBuiltin(types.TypeNameGYear)
	if bt == nil {
		t.Fatal("builtin.Get(\"gYear\") returned nil")
	}

	result := compareNumericOrString("2002", "1998", "gYear", bt)
	if result != 1 {
		t.Errorf("compareNumericOrString(\"2002\", \"1998\", \"gYear\", bt) = %d, want 1", result)
	}

	// Test with nil bt
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
	} else {
		t.Logf("Got expected error: %v", err)
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
			patternFacet := &facets.Pattern{Value: tt.pattern}
			baseQName := types.QName{Namespace: types.XSDNamespace, Local: "string"}

			bt := types.GetBuiltin(types.TypeNameString)
			var baseType types.Type
			if bt != nil {
				baseType = &types.SimpleType{
					QName: baseQName,
					// Variety set via SetVariety
				}
				baseType.(*types.SimpleType).MarkBuiltin()
			}

			facetList := []facets.Facet{patternFacet}
			err := validateFacetConstraints(facetList, baseType, baseQName)
			if tt.valid && err != nil {
				t.Errorf("Pattern %q should be valid but got error: %v", tt.pattern, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("Pattern %q should be invalid but validation passed", tt.pattern)
			}
			if !tt.valid && err != nil {
				// Verify the error mentions pattern
				if !strings.Contains(err.Error(), "pattern") {
					t.Errorf("Error should mention 'pattern', got: %v", err)
				}
			}
		})
	}
}

func TestInvalidPatternSchemas(t *testing.T) {
	testDataDir := "../../testdata/xsdtests"
	if _, err := os.Stat(testDataDir); os.IsNotExist(err) {
		t.Skip("testdata directory not found")
	}

	tests := []struct {
		name       string
		schemaPath string
		shouldFail bool
	}{
		{
			name:       "reM61 - invalid Unicode property",
			schemaPath: "msData/regex/reM61.xsd",
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				FS: os.DirFS(testDataDir),
			}
			l := NewLoader(cfg)
			_, err := l.Load(tt.schemaPath)

			if tt.shouldFail {
				if err == nil {
					t.Errorf("Schema %s should have failed validation but loaded successfully", tt.schemaPath)
				} else {
					// Schema failed to load - verify the error mentions pattern
					if !strings.Contains(err.Error(), "pattern") {
						t.Logf("Schema validation failed as expected, but error doesn't mention 'pattern': %v", err)
					} else {
						t.Logf("Schema validation correctly failed with pattern error: %v", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Schema %s should have loaded successfully but got error: %v", tt.schemaPath, err)
				}
			}
		})
	}
}

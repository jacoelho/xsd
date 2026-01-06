package loader

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// TestOrderedTypeFacetApplicability tests that range facets (minInclusive, maxInclusive, etc.)
// are correctly accepted for ordered types and rejected for unordered types
func TestOrderedTypeFacetApplicability(t *testing.T) {
	tests := []struct {
		name          string
		baseTypeName  string
		facetName     string
		facetValue    string
		shouldAccept  bool
		expectedError string
	}{
		// Numeric types (OrderedTotal) - should accept range facets
		{
			name:         "float with minInclusive",
			baseTypeName: "float",
			facetName:    "minInclusive",
			facetValue:   "0.0",
			shouldAccept: true,
		},
		{
			name:         "float with maxExclusive",
			baseTypeName: "float",
			facetName:    "maxExclusive",
			facetValue:   "100.0",
			shouldAccept: true,
		},
		{
			name:         "double with minInclusive",
			baseTypeName: "double",
			facetName:    "minInclusive",
			facetValue:   "0.0",
			shouldAccept: true,
		},
		{
			name:         "decimal with minInclusive",
			baseTypeName: "decimal",
			facetName:    "minInclusive",
			facetValue:   "0",
			shouldAccept: true,
		},
		{
			name:         "integer with minInclusive",
			baseTypeName: "integer",
			facetName:    "minInclusive",
			facetValue:   "0",
			shouldAccept: true,
		},
		{
			name:         "long with maxInclusive",
			baseTypeName: "long",
			facetName:    "maxInclusive",
			facetValue:   "100",
			shouldAccept: true,
		},
		{
			name:         "int with minExclusive",
			baseTypeName: "int",
			facetName:    "minExclusive",
			facetValue:   "0",
			shouldAccept: true,
		},

		// Date/time types (OrderedTotal) - should accept range facets
		{
			name:         "dateTime with minInclusive",
			baseTypeName: "dateTime",
			facetName:    "minInclusive",
			facetValue:   "2000-01-01T00:00:00",
			shouldAccept: true,
		},
		{
			name:         "date with maxInclusive",
			baseTypeName: "date",
			facetName:    "maxInclusive",
			facetValue:   "2020-12-31",
			shouldAccept: true,
		},
		{
			name:         "time with minInclusive",
			baseTypeName: "time",
			facetName:    "minInclusive",
			facetValue:   "00:00:00",
			shouldAccept: true,
		},
		{
			name:         "gYear with maxInclusive",
			baseTypeName: "gYear",
			facetName:    "maxInclusive",
			facetValue:   "2020",
			shouldAccept: true,
		},
		{
			name:         "gYearMonth with minInclusive",
			baseTypeName: "gYearMonth",
			facetName:    "minInclusive",
			facetValue:   "2000-01",
			shouldAccept: true,
		},
		{
			name:         "gMonthDay with maxInclusive",
			baseTypeName: "gMonthDay",
			facetName:    "maxInclusive",
			facetValue:   "--12-31",
			shouldAccept: true,
		},
		{
			name:         "gDay with minInclusive",
			baseTypeName: "gDay",
			facetName:    "minInclusive",
			facetValue:   "---01",
			shouldAccept: true,
		},
		{
			name:         "gMonth with maxInclusive",
			baseTypeName: "gMonth",
			facetName:    "maxInclusive",
			facetValue:   "--12",
			shouldAccept: true,
		},

		// Duration (OrderedPartial) - SHOULD accept range facets
		// According to XSD spec, duration is partially ordered (ordered=partial), and range facets ARE applicable
		{
			name:         "duration with minInclusive - should accept",
			baseTypeName: "duration",
			facetName:    "minInclusive",
			facetValue:   "P1D",
			shouldAccept: true,
		},
		{
			name:         "duration with maxInclusive - should accept",
			baseTypeName: "duration",
			facetName:    "maxInclusive",
			facetValue:   "P30D",
			shouldAccept: true,
		},
		{
			name:         "duration with minExclusive - should accept",
			baseTypeName: "duration",
			facetName:    "minExclusive",
			facetValue:   "P1D",
			shouldAccept: true,
		},
		{
			name:         "duration with maxExclusive - should accept",
			baseTypeName: "duration",
			facetName:    "maxExclusive",
			facetValue:   "P30D",
			shouldAccept: true,
		},

		// Unordered types (OrderedNone) - should reject range facets
		{
			name:          "string with minInclusive - should reject",
			baseTypeName:  "string",
			facetName:     "minInclusive",
			facetValue:    "a",
			shouldAccept:  false,
			expectedError: "facet minInclusive is only applicable to ordered types",
		},
		{
			name:          "boolean with maxInclusive - should reject",
			baseTypeName:  "boolean",
			facetName:     "maxInclusive",
			facetValue:    "true",
			shouldAccept:  false,
			expectedError: "facet maxInclusive is only applicable to ordered types",
		},
		{
			name:          "hexBinary with minExclusive - should reject",
			baseTypeName:  "hexBinary",
			facetName:     "minExclusive",
			facetValue:    "12",
			shouldAccept:  false,
			expectedError: "facet minExclusive is only applicable to ordered types",
		},
		{
			name:          "base64Binary with maxExclusive - should reject",
			baseTypeName:  "base64Binary",
			facetName:     "maxExclusive",
			facetValue:    "QQ==",
			shouldAccept:  false,
			expectedError: "facet maxExclusive is only applicable to ordered types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bt := types.GetBuiltin(types.TypeName(tt.baseTypeName))
			if bt == nil {
				t.Fatalf("Built-in type %s not found", tt.baseTypeName)
			}

			var facet facets.Facet
			var err error

			switch tt.facetName {
			case "minInclusive":
				facet, err = facets.NewMinInclusive(tt.facetValue, bt)
			case "maxInclusive":
				facet, err = facets.NewMaxInclusive(tt.facetValue, bt)
			case "minExclusive":
				facet, err = facets.NewMinExclusive(tt.facetValue, bt)
			case "maxExclusive":
				facet, err = facets.NewMaxExclusive(tt.facetValue, bt)
			default:
				t.Fatalf("Unknown facet name: %s", tt.facetName)
			}

			if err != nil {
				if tt.shouldAccept {
					t.Fatalf("Failed to create facet %s: %v", tt.facetName, err)
				}
				// If we expect rejection and facet creation failed, that's correct behavior
				// The constructor now validates applicability, so this is expected
				// For schema validation testing, create a mock facet that will be caught by validation
				facet = &mockRangeFacet{
					name:    tt.facetName,
					lexical: tt.facetValue,
				}
			}

			schema := &schema.Schema{
				TargetNamespace: "http://example.com",
				TypeDefs:        make(map[types.QName]types.Type),
			}

			simpleType := &types.SimpleType{
				QName: types.QName{
					Namespace: "http://example.com",
					Local:     "TestType",
				},
				Restriction: &types.Restriction{
					Base: types.QName{
						Namespace: types.XSDNamespace,
						Local:     tt.baseTypeName,
					},
					Facets: []any{facet},
				},
			}
			simpleType.ResolvedBase = bt
			simpleType.SetVariety(types.AtomicVariety)
			schema.TypeDefs[simpleType.QName] = simpleType

			errs := ValidateSchema(schema)

			// Check if we got the expected result
			if tt.shouldAccept {
				// Should not have errors about facet applicability
				for _, err := range errs {
					if err != nil {
						errStr := err.Error()
						if strings.Contains(errStr, "only applicable to ordered types") {
							t.Errorf("Facet %s should be accepted for %s, but got error: %v", tt.facetName, tt.baseTypeName, err)
						}
					}
				}
			} else {
				// Should have an error about facet applicability
				foundError := false
				for _, err := range errs {
					if err != nil {
						errStr := err.Error()
						if strings.Contains(errStr, "only applicable to ordered types") {
							foundError = true
							break
						}
					}
				}
				if !foundError {
					t.Errorf("Facet %s should be rejected for %s, but no error was found. Errors: %v", tt.facetName, tt.baseTypeName, errs)
				}
			}
		})
	}
}

// mockRangeFacet is a mock facet that implements range facet interfaces
// Used for testing validation of inapplicable facets
type mockRangeFacet struct {
	name    string
	lexical string
}

func (m *mockRangeFacet) Name() string {
	return m.name
}

func (m *mockRangeFacet) GetLexical() string {
	return m.lexical
}

func (m *mockRangeFacet) Validate(value types.TypedValue, baseType types.Type) error {
	return nil // Not used for applicability testing
}

package validator

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

func TestValidateFacetsPolicy(t *testing.T) {
	pattern := &types.Pattern{Value: "a+"}
	if err := pattern.ValidateSyntax(); err != nil {
		t.Fatalf("ValidateSyntax: %v", err)
	}

	stringType := types.GetBuiltin(types.TypeNameString)
	ct := &grammar.CompiledType{Original: stringType}

	tests := []struct {
		name           string
		policy         errorPolicy
		wantOK         bool
		wantViolations int
	}{
		{
			name:           "report policy",
			policy:         errorPolicyReport,
			wantOK:         false,
			wantViolations: 1,
		},
		{
			name:           "suppress policy",
			policy:         errorPolicySuppress,
			wantOK:         false,
			wantViolations: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, violations := validateFacets(&facetValidationInput{
				data: &facetValidationData{
					value:  "bbb",
					facets: []types.Facet{pattern},
				},
				typ:      stringType,
				compiled: ct,
				context: &facetValidationContext{
					path: func() string { return "/" },
				},
				policy: tt.policy,
			})
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if len(violations) != tt.wantViolations {
				t.Fatalf("violations = %d, want %d", len(violations), tt.wantViolations)
			}
			if tt.policy == errorPolicyReport && len(violations) == 1 && violations[0].Code != string(errors.ErrFacetViolation) {
				t.Fatalf("violation code = %q, want %q", violations[0].Code, errors.ErrFacetViolation)
			}
		})
	}
}

func TestValidateFacetsMakeViolation(t *testing.T) {
	pattern := &types.Pattern{Value: "a+"}
	if err := pattern.ValidateSyntax(); err != nil {
		t.Fatalf("ValidateSyntax: %v", err)
	}

	stringType := types.GetBuiltin(types.TypeNameString)
	ct := &grammar.CompiledType{Original: stringType}

	ok, violations := validateFacets(&facetValidationInput{
		data: &facetValidationData{
			value:  "bbb",
			facets: []types.Facet{pattern},
		},
		typ:      stringType,
		compiled: ct,
		context: &facetValidationContext{
			path: func() string { return "/" },
			callbacks: &facetValidationCallbacks{
				makeViolation: func(err error) errors.Validation {
					return errors.NewValidation(errors.ErrFacetViolation, "custom", "/")
				},
			},
		},
		policy: errorPolicyReport,
	})
	if ok {
		t.Fatal("expected facets to fail")
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Message != "custom" {
		t.Fatalf("message = %q, want %q", violations[0].Message, "custom")
	}
}

func TestValidateFacetsQNameEnum(t *testing.T) {
	enum := &types.Enumeration{Values: []string{"p:q"}}
	qnameType := types.GetBuiltin(types.TypeNameQName)
	ct := &grammar.CompiledType{
		Original:              qnameType,
		IsQNameOrNotationType: true,
	}

	ok, violations := validateFacets(&facetValidationInput{
		data: &facetValidationData{
			value:  "p:q",
			facets: []types.Facet{enum},
		},
		typ:      qnameType,
		compiled: ct,
		context: &facetValidationContext{
			path: func() string { return "/" },
			callbacks: &facetValidationCallbacks{
				validateQNameEnum: func(string, *types.Enumeration) error {
					return fmt.Errorf("bad enum")
				},
			},
		},
		policy: errorPolicyReport,
	})
	if ok {
		t.Fatal("expected QName enumeration to fail")
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}

func TestValidateFacetsTypedFacet(t *testing.T) {
	decimalType := types.GetBuiltin(types.TypeNameDecimal)
	minInclusive, err := types.NewMinInclusive("10", decimalType)
	if err != nil {
		t.Fatalf("NewMinInclusive: %v", err)
	}
	ct := &grammar.CompiledType{Original: decimalType}

	ok, violations := validateFacets(&facetValidationInput{
		data: &facetValidationData{
			value:  "9",
			facets: []types.Facet{minInclusive},
		},
		typ:      decimalType,
		compiled: ct,
		context: &facetValidationContext{
			path: func() string { return "/" },
		},
		policy: errorPolicyReport,
	})
	if ok {
		t.Fatal("expected minInclusive to fail")
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}

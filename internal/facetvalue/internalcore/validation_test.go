package internalcore

import (
	"errors"
	"strings"
	"testing"
)

func TestApplyFacetsStopsOnFirstError(t *testing.T) {
	t.Parallel()

	calls := 0
	wantErr := errors.New("boom")
	err := ApplyFacets("value", []any{"ok", "bad", "later"}, nil, ApplyFacetOps{
		ValidateFacet: func(facet any, _ any, _ any) error {
			calls++
			if facet == "bad" {
				return wantErr
			}
			return nil
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ApplyFacets() error = %v, want %v", err, wantErr)
	}
	if calls != 2 {
		t.Fatalf("ApplyFacets() calls = %d, want 2", calls)
	}
}

func TestValidateValueAgainstFacetsWrapsLexicalErrors(t *testing.T) {
	t.Parallel()

	typedCreated := false
	err := ValidateValueAgainstFacets("v", nil, []any{"lex"}, nil, ValidateFacetOps{
		FacetName:             func(facet any) string { return facet.(string) },
		ShouldSkipLengthFacet: func(any, any) bool { return false },
		IsQNameOrNotationType: func(any) bool { return false },
		IsListTypeForFacetValidation: func(any) bool {
			return false
		},
		ValidateQNameEnumerationLexical: func(any, string, any, map[string]string) (bool, error) {
			return false, nil
		},
		ValidateLexicalFacet: func(facet any, _ string, _ any) (bool, error) {
			if facet == "lex" {
				return true, errors.New("bad lexical")
			}
			return false, nil
		},
		TypedValueForFacet: func(string, any) any {
			typedCreated = true
			return struct{}{}
		},
		ValidateFacet: func(any, any, any) error { return nil },
	})
	if err == nil {
		t.Fatalf("expected lexical validation error")
	}
	if !strings.Contains(err.Error(), "facet 'lex' violation: bad lexical") {
		t.Fatalf("error = %v", err)
	}
	if typedCreated {
		t.Fatalf("typed value should not be created for lexical-only errors")
	}
}

func TestValidateValueAgainstFacetsBuildsTypedValueOnce(t *testing.T) {
	t.Parallel()

	typedCreations := 0
	validations := 0
	err := ValidateValueAgainstFacets("v", nil, []any{"a", "b"}, nil, ValidateFacetOps{
		FacetName: func(facet any) string { return facet.(string) },
		ShouldSkipLengthFacet: func(any, any) bool {
			return false
		},
		IsQNameOrNotationType: func(any) bool { return false },
		IsListTypeForFacetValidation: func(any) bool {
			return false
		},
		ValidateQNameEnumerationLexical: func(any, string, any, map[string]string) (bool, error) {
			return false, nil
		},
		ValidateLexicalFacet: func(any, string, any) (bool, error) {
			return false, nil
		},
		TypedValueForFacet: func(string, any) any {
			typedCreations++
			return struct{}{}
		},
		ValidateFacet: func(any, any, any) error {
			validations++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ValidateValueAgainstFacets() error = %v", err)
	}
	if typedCreations != 1 {
		t.Fatalf("typed value creations = %d, want 1", typedCreations)
	}
	if validations != 2 {
		t.Fatalf("facet validations = %d, want 2", validations)
	}
}

func TestValidateValueAgainstFacetsKeepsQNameEnumerationErrorsUnwrapped(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("qname mismatch")
	err := ValidateValueAgainstFacets("v", nil, []any{"enum"}, nil, ValidateFacetOps{
		FacetName:             func(facet any) string { return facet.(string) },
		ShouldSkipLengthFacet: func(any, any) bool { return false },
		IsQNameOrNotationType: func(any) bool { return true },
		IsListTypeForFacetValidation: func(any) bool {
			return false
		},
		ValidateQNameEnumerationLexical: func(facet any, _ string, _ any, _ map[string]string) (bool, error) {
			if facet == "enum" {
				return true, wantErr
			}
			return false, nil
		},
		ValidateLexicalFacet: func(any, string, any) (bool, error) { return false, nil },
		TypedValueForFacet:   func(string, any) any { return struct{}{} },
		ValidateFacet:        func(any, any, any) error { return nil },
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ValidateValueAgainstFacets() error = %v, want %v", err, wantErr)
	}
	if strings.Contains(err.Error(), "facet 'enum' violation") {
		t.Fatalf("qname enumeration error was unexpectedly wrapped: %v", err)
	}
}

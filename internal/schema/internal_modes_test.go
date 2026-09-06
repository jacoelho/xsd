package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompileModeValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func() error
		text string
	}{
		{name: "invalid complex type scope", call: func() error { return validateComplexTypeScope(complexTypeScopeInvalid) }, text: "invalid complex type compilation scope"},
		{name: "unknown complex type scope", call: func() error { return validateComplexTypeScope(complexTypeScope(99)) }, text: "unknown complex type compilation scope"},
		{name: "invalid facet child mode", call: func() error { return validateFacetChildMode(facetChildModeInvalid) }, text: "invalid facet child mode"},
		{name: "unknown facet child mode", call: func() error { return validateFacetChildMode(facetChildMode(99)) }, text: "unknown facet child mode"},
		{name: "invalid facet fixedness", call: func() error { return validateFacetFixedness(facetFixednessInvalid) }, text: "facet fixedness is invalid"},
		{name: "unknown facet fixedness", call: func() error { return validateFacetFixedness(facetFixedness(99)) }, text: "facet fixedness is unknown"},
		{
			name: "unknown schema source requirement",
			call: func() error {
				_, err := classifySchemaAcquireResult("schema.xsd", source.ReadResult{Err: errors.New("read failed")}, 1, 1, schemaSourceRequirement(99))
				return err
			},
			text: "unknown schema source requirement",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.call()
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if !strings.Contains(err.Error(), tt.text) {
				t.Fatalf("mode validator error = %v, want text %q", err, tt.text)
			}
		})
	}
}

func TestCompileKindPanics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func()
	}{
		{name: "invalid simple derivation", call: func() { simpleDerivationOrder(simpleDerivationInvalid) }},
		{name: "unknown simple derivation", call: func() { simpleDerivationOrder(simpleDerivationKind(99)) }},
		{name: "unknown DFA repeat counter", call: func() { dfaRepeatCounter{kind: dfaRepeatCounterKind(99)}.active() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				if recover() == nil {
					t.Fatal("operation did not panic")
				}
			}()
			tt.call()
		})
	}
}

func TestModelTextKindInvariant(t *testing.T) {
	t.Parallel()

	for _, kind := range []modelTextKind{modelTextInvalid, modelTextKind(99)} {
		_, err := modelWithTextKind(nil, nil, 0, kind)
		expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	}
}

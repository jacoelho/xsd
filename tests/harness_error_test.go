package tests_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestExpectedFailureRejectsInternalErrors(t *testing.T) {
	invariant := xsderrors.InternalInvariant("corrupted content model")
	deep := invariant
	for range 300 {
		deep = fmt.Errorf("context: %w", deep)
	}
	for _, phase := range []struct {
		name      string
		semantic  error
		code      xsderrors.Code
		withCause error
		causeCode xsderrors.Code
	}{
		{
			name:      "compile",
			semantic:  xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "invalid model"),
			code:      xsderrors.CodeSchemaContentModel,
			withCause: xsderrors.SchemaParse(xsderrors.CodeSchemaXML, "schema error", invariant),
			causeCode: xsderrors.CodeSchemaXML,
		},
		{
			name:      "validation",
			semantic:  xsderrors.Validation(xsderrors.CodeValidationContent, "invalid content", nil),
			code:      xsderrors.CodeValidationContent,
			withCause: xsderrors.Validation(xsderrors.CodeValidationContent, "content error", invariant),
			causeCode: xsderrors.CodeValidationContent,
		},
	} {
		t.Run(phase.name, func(t *testing.T) {
			for _, failure := range []struct {
				name string
				err  error
				code xsderrors.Code
			}{
				{"internal", invariant, xsderrors.CodeInternalInvariant},
				{"wrapped", fmt.Errorf("context: %w", invariant), xsderrors.CodeInternalInvariant},
				{"deeply wrapped", deep, xsderrors.CodeInternalInvariant},
				{"structured cause", phase.withCause, phase.causeCode},
				{"diagnostic aggregate", xsderrors.NewErrors(phase.semantic, invariant), phase.code},
				{"joined errors", errors.Join(phase.semantic, invariant), phase.code},
				{"unsupported cause", xsderrors.Unsupported(xsderrors.CodeUnsupportedRegex, "unsupported pattern", invariant), xsderrors.CodeUnsupportedRegex},
				{"unsupported sibling", errors.Join(xsderrors.Unsupported(xsderrors.CodeUnsupportedRegex, "unsupported pattern", nil), invariant), xsderrors.CodeUnsupportedRegex},
			} {
				t.Run(failure.name, func(t *testing.T) {
					for _, code := range []string{"", string(failure.code)} {
						if err := checkExpectedFailure(failure.err, code); !errors.Is(err, invariant) {
							t.Errorf("checkExpectedFailure(%v, %q) = %v, want internal failure", failure.err, code, err)
						}
					}
				})
			}
		})
	}
}

func TestExpectedFailurePreservesCorpusOutcomes(t *testing.T) {
	for _, outcome := range []struct {
		name string
		err  error
		code xsderrors.Code
	}{
		{"invalid schema", xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "invalid model"), xsderrors.CodeSchemaContentModel},
		{"invalid instance", xsderrors.Validation(xsderrors.CodeValidationContent, "invalid content", nil), xsderrors.CodeValidationContent},
		{"malformed schema", xsderrors.SchemaParse(xsderrors.CodeSchemaXML, "malformed schema", errors.New("unexpected end")), xsderrors.CodeSchemaXML},
		{"malformed XML", xsderrors.Validation(xsderrors.CodeValidationXML, "malformed XML", errors.New("unexpected end")), xsderrors.CodeValidationXML},
		{"schema limit", xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema limit"), xsderrors.CodeSchemaLimit},
		{"validation limit", xsderrors.Validation(xsderrors.CodeValidationLimit, "validation limit", nil), xsderrors.CodeValidationLimit},
		{"unsupported", xsderrors.Unsupported(xsderrors.CodeUnsupportedRegex, "unsupported pattern", nil), xsderrors.CodeUnsupportedRegex},
	} {
		t.Run(outcome.name, func(t *testing.T) {
			for _, code := range []string{"", string(outcome.code)} {
				if err := checkExpectedFailure(outcome.err, code); err != nil {
					t.Errorf("checkExpectedFailure(%v, %q) = %v", outcome.err, code, err)
				}
			}
			if err := checkExpectedFailure(outcome.err, "different.code"); err == nil {
				t.Error("checkExpectedFailure() accepted a mismatched expected error code")
			}
		})
	}
	if err := checkExpectedFailure(nil, ""); err == nil {
		t.Error("checkExpectedFailure() accepted successful assessment")
	}
}

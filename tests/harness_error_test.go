package tests_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestExpectedFailureRejectsInternalErrors(t *testing.T) {
	invariant := xsderrors.InternalInvariant("corrupted content model", nil)
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
			semantic:  xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "invalid model", nil),
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
		{"invalid schema", xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "invalid model", nil), xsderrors.CodeSchemaContentModel},
		{"invalid instance", xsderrors.Validation(xsderrors.CodeValidationContent, "invalid content", nil), xsderrors.CodeValidationContent},
		{"malformed schema", xsderrors.SchemaParse(xsderrors.CodeSchemaXML, "malformed schema", errors.New("unexpected end")), xsderrors.CodeSchemaXML},
		{"malformed XML", xsderrors.Validation(xsderrors.CodeValidationXML, "malformed XML", errors.New("unexpected end")), xsderrors.CodeValidationXML},
		{"schema limit", xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema limit", nil), xsderrors.CodeSchemaLimit},
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

func TestDiagnosticLocationsRejectUnlocatedDiagnostics(t *testing.T) {
	located := func(err error) error { return xsderrors.WithLocation("a.xsd", 3, 7, err) }
	compileDiagnostic := xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "UPA violation", nil)
	parseDiagnostic := xsderrors.SchemaParse(xsderrors.CodeSchemaXML, "invalid schema XML", nil)
	unsupported := xsderrors.Unsupported(xsderrors.CodeUnsupportedDTD, "", nil)
	validation := xsderrors.Validation(xsderrors.CodeValidationContent, "invalid content", nil)
	for _, test := range []struct {
		name       string
		err        error
		categories []xsderrors.Category
		wantErr    bool
	}{
		{"compile success", nil, compileLocatedCategories, false},
		{"located compile", located(compileDiagnostic), compileLocatedCategories, false},
		{"located parse", located(parseDiagnostic), compileLocatedCategories, false},
		{"located unsupported", located(unsupported), compileLocatedCategories, false},
		{"unlocated compile", compileDiagnostic, compileLocatedCategories, true},
		{"unlocated parse", parseDiagnostic, compileLocatedCategories, true},
		{"unlocated unsupported at compile", unsupported, compileLocatedCategories, true},
		{"path without line", xsderrors.WithLocation("a.xsd", 0, 0, compileDiagnostic), compileLocatedCategories, true},
		{"line without path", xsderrors.WithLocation("", 3, 7, compileDiagnostic), compileLocatedCategories, true},
		{"unlocated branch of an aggregate", xsderrors.NewErrors(located(compileDiagnostic), compileDiagnostic), compileLocatedCategories, true},
		{"unlocated wrapped", fmt.Errorf("context: %w", compileDiagnostic), compileLocatedCategories, true},
		{"internal is not a document diagnostic", xsderrors.InternalInvariant("corrupted", nil), compileLocatedCategories, false},
		{"located validation", located(validation), instanceLocatedCategories, false},
		{"unlocated validation", validation, instanceLocatedCategories, true},
		{"unlocated unsupported at validation", unsupported, instanceLocatedCategories, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := checkDiagnosticLocations(test.err, test.categories)
			if (err != nil) != test.wantErr {
				t.Fatalf("checkDiagnosticLocations(%v) = %v, want error %t", test.err, err, test.wantErr)
			}
		})
	}
}

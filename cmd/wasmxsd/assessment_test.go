package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestConclusiveAssessmentRequiresOnlySemanticDiagnostics(t *testing.T) {
	semantic := xsderrors.Validation(xsderrors.CodeValidationFacet, "invalid value", errors.New("lexical failure"))
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"semantic with lexical cause", semantic, true},
		{"wrapped semantic", fmt.Errorf("document: %w", semantic), true},
		{"multiple semantic", xsderrors.NewErrors(semantic, xsderrors.Validation(xsderrors.CodeValidationText, "text", nil)), true},
		{"semantic root", xsderrors.Validation(xsderrors.CodeValidationRoot, "root", nil), true},
		{"semantic element", xsderrors.Validation(xsderrors.CodeValidationElement, "element", nil), true},
		{"semantic attribute", xsderrors.Validation(xsderrors.CodeValidationAttribute, "attribute", nil), true},
		{"semantic text", xsderrors.Validation(xsderrors.CodeValidationText, "text", nil), true},
		{"semantic type", xsderrors.Validation(xsderrors.CodeValidationType, "type", nil), true},
		{"semantic facet", xsderrors.Validation(xsderrors.CodeValidationFacet, "facet", nil), true},
		{"semantic content", xsderrors.Validation(xsderrors.CodeValidationContent, "content", nil), true},
		{"semantic nil", xsderrors.Validation(xsderrors.CodeValidationNil, "nil", nil), true},
		{"semantic identity", xsderrors.Validation(xsderrors.CodeValidationIdentity, "identity", nil), true},
		{"XML", xsderrors.Validation(xsderrors.CodeValidationXML, "malformed", nil), false},
		{"limit", xsderrors.Validation(xsderrors.CodeValidationLimit, "limit", nil), false},
		{"option", xsderrors.Validation(xsderrors.CodeValidationOption, "option", nil), false},
		{"session", xsderrors.Validation(xsderrors.CodeValidationSession, "overlap", nil), false},
		{"unsupported", xsderrors.Unsupported(xsderrors.CodeUnsupportedDTD, "DTD", nil), false},
		{"internal", xsderrors.InternalInvariant("broken invariant"), false},
		{"unknown", errors.New("unknown failure"), false},
		{"semantic plus XML", errors.Join(semantic, xsderrors.Validation(xsderrors.CodeValidationXML, "malformed", nil)), false},
		{"semantic plus limit", xsderrors.NewErrors(semantic, xsderrors.Validation(xsderrors.CodeValidationLimit, "limit", nil)), false},
		{"semantic plus unknown", errors.Join(semantic, errors.New("unknown failure")), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isConclusiveValidation(tc.err); got != tc.want {
				t.Fatalf("conclusive = %v, want %v for %v", got, tc.want, tc.err)
			}
		})
	}
}
